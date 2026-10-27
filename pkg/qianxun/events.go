package qianxun

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
)

// EventID 为千寻框架事件编号；直接微信模式没有该字段，由解码器保留原始Kind并推导可对应编号。
type EventID int

const (
	EventAny          EventID = -1    // EventAny 用于订阅所有事件，包括未来新增的未知事件。
	Heartbeat         EventID = 10    // Heartbeat 是WebSocket应用层心跳，不代表微信消息。
	InjectSuccess     EventID = 10000 // InjectSuccess 允许WXID为空，此时微信可能尚未登录。
	Transfer          EventID = 10006 // Transfer 为转账状态事件，不等同于二维码收款。
	Payment           EventID = 10007 // Payment 为支付消息，保留原始消息体。
	GroupMessage      EventID = 10008 // GroupMessage 为收到的群聊消息。
	PrivateMessage    EventID = 10009 // PrivateMessage 为私聊/公众号消息。
	SelfMessage       EventID = 10010 // SelfMessage 为账号自己发出的消息。
	FriendRequest     EventID = 10011 // FriendRequest 为好友验证请求，接受操作必须由业务明确决定。
	Revoke            EventID = 10013 // Revoke 为消息撤回，不应当作普通消息重新执行命令。
	AccountChange     EventID = 10014 // AccountChange 的payload直接是账号对象，不套通用data.data包装。
	QRPayment         EventID = 10015 // QRPayment 为二维码收款事件。
	GroupMemberChange EventID = 10016 // GroupMemberChange 为成员变动事件。
	AuthExpired       EventID = 99999 // AuthExpired 后其他事件可能停止，业务应报告授权状态。
)

// Event 同时支持框架数字事件和直接客户端字符串事件，Raw保留所有未知字段。
// 消息正文、签名和账号信息可能敏感，不应直接记录整个Event或Raw。
type Event struct {
	ID            EventID
	Kind          string
	Mode          string
	WXID          string
	Port          Scalar
	PID           Scalar
	Timestamp     Scalar
	Flag          string
	Description   string
	Payload       json.RawMessage
	Raw           json.RawMessage
	Known         bool
	retainedBytes int64
	messageID     Scalar
	conversation  string
}

var eventNames = map[EventID]string{Heartbeat: "heartbeat", InjectSuccess: "injectSuccess", Transfer: "transPay", Payment: "recvMsg", GroupMessage: "recvMsg", PrivateMessage: "recvMsg", SelfMessage: "recvMsg", FriendRequest: "friendReq", Revoke: "revokeMsg", AccountChange: "accountChange", QRPayment: "qrPay", GroupMemberChange: "groupMemberChanges", AuthExpired: "authExpire"}

// DecodeEvent 校验单个JSON对象并归一化外壳；不解码为map[string]float64，避免长消息ID失真。
// 未知事件会保留并标记Known=false；注入成功、心跳和授权事件不强制要求非空WXID。
func DecodeEvent(raw []byte) (Event, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return Event{}, errors.New("千寻事件必须是JSON对象")
	}
	var e Event
	e.Raw = bytes.Clone(raw)
	if event, ok := fields["event"]; ok {
		var id Scalar
		if json.Unmarshal(event, &id) != nil {
			return Event{}, errors.New("千寻事件编号无效")
		}
		n, err := strconv.Atoi(string(id))
		if err != nil || n < 0 {
			return Event{}, errors.New("千寻事件编号无效")
		}
		e.ID = EventID(n)
		e.Kind, e.Known = eventNames[e.ID]
		e.Mode = "framework"
		if e.ID == Heartbeat {
			return e, nil
		}
		_ = json.Unmarshal(fields["wxid"], &e.WXID)
		if !e.Known {
			e.Kind = "unknown"
			e.Payload = fields["data"]
			return e, nil
		}
		if e.ID == AccountChange {
			e.Payload = fields["data"]
			var account AccountChangeData
			if json.Unmarshal(e.Payload, &account) != nil {
				return Event{}, errors.New("账号变动事件数据无效")
			}
			if e.WXID == "" {
				e.WXID = account.WXID
			}
			e.Port = account.Port
			return e, nil
		}
		if data, ok := fields["data"]; ok && !bytes.Equal(data, []byte("null")) {
			if err := decodeWrapper(data, &e); err != nil {
				return Event{}, err
			}
		} else if e.Known && e.ID != AuthExpired {
			return Event{}, errors.New("千寻事件缺少data包装")
		}
	} else {
		e.Mode = "direct"
		if err := decodeWrapper(raw, &e); err != nil {
			return Event{}, err
		}
		switch e.Kind {
		case "injectSuccess":
			e.ID = InjectSuccess
		case "loginSuccess":
			e.ID = AccountChange
		case "recvMsg":
			var m MessageData
			if json.Unmarshal(e.Payload, &m) != nil {
				return Event{}, errors.New("消息事件数据无效")
			}
			if m.FromWXID == "" || m.MsgID == "" {
				return Event{}, errors.New("消息事件缺少会话或消息ID")
			}
			e.messageID = m.MsgID
			e.conversation = m.FromWXID
			e.ID = PrivateMessage
			if m.FromType == "2" {
				e.ID = GroupMessage
			}
			if m.MsgSource == "1" {
				e.ID = SelfMessage
			}
		case "transPay":
			e.ID = Transfer
		case "revokeMsg":
			e.ID = Revoke
		case "friendReq":
			e.ID = FriendRequest
		case "qrPay":
			e.ID = QRPayment
		case "groupMemberChanges":
			e.ID = GroupMemberChange
		case "authExpire":
			e.ID = AuthExpired
		}
		e.Known = e.ID != 0
	}
	if (e.ID == GroupMessage || e.ID == PrivateMessage || e.ID == SelfMessage) && e.messageID == "" {
		var m MessageData
		if json.Unmarshal(e.Payload, &m) != nil || m.FromWXID == "" || m.MsgID == "" {
			return Event{}, errors.New("消息事件缺少会话或消息ID")
		}
		e.messageID = m.MsgID
		e.conversation = m.FromWXID
	}
	return e, nil
}
func decodeWrapper(raw []byte, e *Event) error {
	var w struct {
		Type        Scalar          `json:"type"`
		Description string          `json:"des"`
		Data        json.RawMessage `json:"data"`
		WXID        string          `json:"wxid"`
		Port        Scalar          `json:"port"`
		PID         Scalar          `json:"pid"`
		Timestamp   Scalar          `json:"timestamp"`
		Flag        string          `json:"flag"`
	}
	if json.Unmarshal(raw, &w) != nil {
		return errors.New("千寻事件包装字段无效")
	}
	if e.Kind == "" {
		e.Kind = string(w.Type)
	} else if w.Type != "" && e.Kind != string(w.Type) {
		return errors.New("千寻事件编号与类型不一致")
	}
	if e.Kind == "" {
		return errors.New("千寻事件缺少类型")
	}
	if e.WXID != "" && w.WXID != "" && e.WXID != w.WXID {
		return errors.New("千寻事件账号不一致")
	}
	if e.WXID == "" {
		e.WXID = w.WXID
	}
	e.Port = w.Port
	e.PID = w.PID
	e.Timestamp = w.Timestamp
	e.Flag = w.Flag
	e.Description = w.Description
	e.Payload = w.Data
	return nil
}

// PayloadAs 将已保留的事件业务体解码成调用方选定的强类型，不丢弃Event.Raw。
func PayloadAs[T any](e Event) (T, error) {
	var out T
	err := json.Unmarshal(e.Payload, &out)
	return out, err
}

// MessageData 对应recvMsg的消息体。所有标识/数字兼容字符串和数字，保留整数精度。
type MessageData struct {
	TimeStamp     Scalar   `json:"timeStamp"`     // TimeStamp 为上游消息时间；不要与入站接收时间混淆。
	FromType      Scalar   `json:"fromType"`      // FromType：1私聊、2群聊、3公众号。
	MsgType       Scalar   `json:"msgType"`       // MsgType 为微信消息类型，未知类型仍应保留。
	MsgSource     Scalar   `json:"msgSource"`     // MsgSource：0接收、1自己发送。
	FromWXID      string   `json:"fromWxid"`      // FromWXID 是会话ID，群聊时为群ID。
	FinalFromWXID string   `json:"finalFromWxid"` // FinalFromWXID 是实际发送者ID，不能以昵称替代。
	AtWXIDList    []string `json:"atWxidList"`    // AtWXIDList 保留真正被艾特的账号列表。
	Silence       Scalar   `json:"silence"`       // Silence 表示免打扰状态。
	MemberCount   Scalar   `json:"membercount"`   // MemberCount 是事件快照中的成员数量。
	Signature     string   `json:"signature"`     // Signature 可能包含消息签名，不写入普通日志。
	Msg           string   `json:"msg"`           // Msg 为原文/XML/媒体描述，不在SDK中擅自清理或截断。
	MsgID         Scalar   `json:"msgId"`         // MsgID 用于消息识别，不能转成float64。
	SendID        Scalar   `json:"sendId"`        // SendID 为发送标识，可在部分事件中为空。
}

// InjectSuccessData 在登录前即可收到，因此Port/PID可存在而账号为空。
type InjectSuccessData struct {
	Port Scalar `json:"port"`
	PID  Scalar `json:"pid"`
}

// AccountData 为登录成功/账号上线的账号快照，离线时大部分字段可能缺失。
type AccountData struct {
	WXID      string `json:"wxid"`
	WXNum     string `json:"wxNum"`
	Nick      string `json:"nick"`
	Device    string `json:"device"`
	Phone     string `json:"phone"`
	AvatarURL string `json:"avatarUrl"`
	Country   string `json:"country"`
	Province  string `json:"province"`
	City      string `json:"city"`
	Email     string `json:"email"`
	QQ        Scalar `json:"qq"`
	Sign      string `json:"sign"`
}

// AccountChangeData 是框架10014事件的直接payload；Type为1上线、0离线。
type AccountChangeData struct {
	AccountData
	Type Scalar `json:"type"`
	Port Scalar `json:"port"`
}

// TransferData 保留转账状态与订单标识；SDK不会自动确认或退还款项。
type TransferData struct {
	FromWXID      string `json:"fromWxid"`
	MsgSource     Scalar `json:"msgSource"`
	TransType     Scalar `json:"transType"`
	Money         Scalar `json:"money"`
	Memo          string `json:"memo"`
	TransferID    string `json:"transferid"`
	TransactionID string `json:"transcationid"`
	InvalidTime   Scalar `json:"invalidtime"`
	MsgID         Scalar `json:"msgId"`
}

// FriendRequestData 是验证请求快照，V3/V4应仅交给明确授权的好友操作。
type FriendRequestData struct {
	AccountData
	NickBrief    string `json:"nickBrief"`
	NickWhole    string `json:"nickWhole"`
	V3           string `json:"v3"`
	V4           string `json:"v4"`
	AvatarMinURL string `json:"avatarMinUrl"`
	AvatarMaxURL string `json:"avatarMaxUrl"`
	Sex          Scalar `json:"sex"`
	Content      string `json:"content"`
	Scene        Scalar `json:"scene"`
	ShareWXID    string `json:"shareWxid"`
	ShareNick    string `json:"shareNick"`
	GroupWXID    string `json:"groupWxid"`
	MsgID        Scalar `json:"msgId"`
}

// QRPaymentData 是二维码收款快照，与普通转账字段不同。
type QRPaymentData struct {
	FromWXID     string `json:"fromWxid"`
	FromNickname string `json:"fromNickName"`
	MsgSource    Scalar `json:"msgSource"`
	Money        Scalar `json:"money"`
	TransferID   string `json:"transferid"`
	InvalidTime  Scalar `json:"invalidtime"`
}

// GroupMemberChangeData 保留群成员变动类型；未知EventType仍应交给业务识别。
type GroupMemberChangeData struct {
	TimeStamp     Scalar `json:"timeStamp"`
	FromWXID      string `json:"fromWxid"`
	FinalFromWXID string `json:"finalFromWxid"`
	EventType     Scalar `json:"eventType"`
	InviterWXID   string `json:"inviterWxid"`
}

// LoginSuccessData 是直接客户端模式登录成功的账号快照。
type LoginSuccessData = AccountData

// RevokeData 沿用消息字段子集，原始撤回通知始终保留在Event.Raw。
type RevokeData = MessageData

// PaymentData 是支付事件中的消息数据；不能因缺少普通聊天msgId而丢弃支付通知。
type PaymentData = MessageData

// AuthExpiredData 只描述授权状态，不会触发自动购买、续费或其他副作用。
type AuthExpiredData struct {
	Nick       string `json:"nick"`
	WXID       string `json:"wxid"`
	WXNum      string `json:"wxNum"`
	ExpireTime string `json:"expireTime"`
	Msg        string `json:"msg"`
}
