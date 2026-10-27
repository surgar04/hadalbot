package mihoyo

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// 登录公钥来自旧项目已核对的HoYoLAB网页实现；是公开协议材料，不是用户凭证。
const loginPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA4PMS2JVMwBsOIrYWRluY
wEiFZL7Aphtm9z5Eu/anzJ09nB00uhW+ScrDWFECPwpQto/GlOJYCUwVM/raQpAj
/xvcjK5tNVzzK94mhk+j9RiQ+aWHaTXmOgurhxSp3YbwlRDvOgcq5yPiTz0+kSeK
ZJcGeJ95bvJ+hJ/UMP0Zx2qB5PElZmiKvfiNqVUk8A8oxLJdBB5eCpqWV6CUqDKQ
KSQP4sM0mZvQ1Sr4UcACVcYgYnCbTZMWhJTWkrNXqI8TMomekgny3y+d6NX/cFa6
6jozFIF4HCX5aW8bp8C8vq2tFvFbleQ/Q3CU56EWWKMrOcpmFtRmC18s9biZBVR/
8QIDAQAB
-----END PUBLIC KEY-----`

var publicKey = sync.OnceValues(func() (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(loginPublicKey))
	if block == nil {
		return nil, errors.New("登录公钥格式无效")
	}
	key, e := x509.ParsePKIXPublicKey(block.Bytes)
	if e != nil {
		return nil, errors.New("登录公钥无法解析")
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("登录公钥不是RSA")
	}
	return rsaKey, nil
})

// EncryptCredential 执行上游要求的RSA PKCS#1 v1.5加密并Base64编码，不保存明文。
// account应由调用方去首尾空格；password必须原样保留，不能TrimSpace。
func EncryptCredential(value string) (string, error) {
	key, e := publicKey()
	if e != nil {
		return "", e
	}
	if value == "" || len([]byte(value)) > key.Size()-11 {
		return "", errors.New("登录字段为空或超过RSA协议长度")
	}
	//nolint:staticcheck // 上游JSEncrypt协议固定PKCS#1 v1.5，改成OAEP会破坏互通；这里只做公钥加密，不提供解密服务。
	cipher, e := rsa.EncryptPKCS1v15(rand.Reader, key, []byte(value))
	if e != nil {
		return "", errors.New("登录字段加密失败")
	}
	return base64.StdEncoding.EncodeToString(cipher), nil
}

// Device 是调用方为一个账号会话保留的设备上下文，不能伪造为固定全局指纹。
type Device struct {
	ID          string
	FP          string
	AppDeviceID string
	UserAgent   string
	SDKVersion  string
}

// LoginData 保留登录票据及用户信息；token字段属于敏感内容，禁止对外输出。
type LoginData struct {
	Token struct {
		TokenType int    `json:"token_type"`
		Token     Secret `json:"token"`
	} `json:"token"`
	UserInfo               map[string]json.RawMessage `json:"user_info"`
	ExtUserInfo            map[string]json.RawMessage `json:"ext_user_info"`
	ReactivateActionTicket Secret                     `json:"reactivate_action_ticket"`
}

// LoginResult 只是登录API结果，SessionComplete为false时不能当作绑定成功。
type LoginResult struct {
	Data            LoginData `json:"-"`
	Cookie          Secret    `json:"-"`
	SessionComplete bool      `json:"session_complete"`
}

// STokenSession 是明确属于一个用户的长期授权材料；不会由SDK全局缓存。
type STokenSession struct {
	SToken Secret `json:"-"`
	UID    string `json:"-"`
	MID    string `json:"-"`
}

// AigisSolution 描述调用方在官方验证页面完成后得到的v3/v4结果。
// SDK只编码已有结果，不请求付费打码服务、不自动破解或重放其他账号挑战。
type AigisSolution struct {
	Version       int
	SessionID     string
	Challenge     string
	Validate      string
	Seccode       string
	CaptchaID     string
	LotNumber     string
	PassToken     Secret
	GenTime       string
	CaptchaOutput Secret
}

// Encode 将结果编码为上游要求的session_id;base64(JSON)格式，不自动重试使用一次性结果。
func (s AigisSolution) Encode() (string, error) {
	if s.SessionID == "" || strings.ContainsAny(s.SessionID, ";\r\n") {
		return "", errors.New("Aigis会话ID无效")
	}
	var payload any
	switch s.Version {
	case 3:
		if s.Challenge == "" || s.Validate == "" || s.Seccode == "" {
			return "", errors.New("人机验证v3结果不完整")
		}
		payload = struct {
			Challenge string `json:"geetest_challenge"`
			Validate  string `json:"geetest_validate"`
			Seccode   string `json:"geetest_seccode"`
		}{s.Challenge, s.Validate, s.Seccode}
	case 4:
		if s.CaptchaID == "" || s.LotNumber == "" || s.PassToken == "" || s.GenTime == "" || s.CaptchaOutput == "" {
			return "", errors.New("人机验证v4结果不完整")
		}
		info, _ := json.Marshal(map[string]string{"session_id": s.SessionID})
		payload = struct {
			CaptchaID     string `json:"captcha_id"`
			LotNumber     string `json:"lot_number"`
			PassToken     string `json:"pass_token"`
			GenTime       string `json:"gen_time"`
			CaptchaOutput string `json:"captcha_output"`
			UserInfo      string `json:"userInfo"`
		}{s.CaptchaID, s.LotNumber, s.PassToken.Reveal(), s.GenTime, s.CaptchaOutput.Reveal(), string(info)}
	default:
		return "", errors.New("Aigis版本必须是3或4")
	}
	encoded, e := json.Marshal(payload)
	if e != nil {
		return "", e
	}
	return s.SessionID + ";" + base64.StdEncoding.EncodeToString(encoded), nil
}

func deviceHeaders(d Device, web bool) (http.Header, error) {
	if d.ID == "" {
		return nil, errors.New("登录需要当前会话的设备ID")
	}
	h := http.Header{}
	h.Set("x-rpc-device_id", d.ID)
	h.Set("x-rpc-language", "zh-cn")
	if web {
		if d.FP == "" {
			return nil, errors.New("网页登录需要已获取的设备指纹")
		}
		h.Set("x-rpc-app_id", "c9oqaq3s3gu8")
		h.Set("x-rpc-client_type", "4")
		h.Set("x-rpc-device_fp", d.FP)
		h.Set("Origin", "https://account.hoyolab.com")
		h.Set("Referer", "https://account.hoyolab.com/")
		h.Set("x-rpc-domain_redirect", "true")
		h.Set("x-rpc-game_biz", "bbs_oversea")
		h.Set("x-rpc-age_gate", "true")
		h.Set("x-rpc-age_gate_br", "true")
		h.Set("x-rpc-age_gate_eu", "true")
		h.Set("x-rpc-aigis_v4", "true")
		version := d.SDKVersion
		if version == "" {
			version = "2.56.0"
		}
		h.Set("x-rpc-sdk_version", version)
	} else {
		h.Set("x-rpc-app_id", "ddxf6vlr1reo")
		h.Set("x-rpc-client_type", "3")
		id := d.AppDeviceID
		if id == "" {
			id = d.ID
		}
		h.Set("x-rpc-device_id", id)
		h.Set("User-Agent", "HYPContainer/1.1.4.133")
	}
	if d.UserAgent != "" {
		h.Set("User-Agent", d.UserAgent)
	}
	return h, nil
}

// LoginApp 显式请求App密码登录，拿到SToken后才交换会话；遇到验证要求立即交还ChallengeError。
// 一次调用只尝试一个上游登录端点，不在失败后盲目切换登录方式或重复提交密码。
func (c *Client) LoginApp(ctx context.Context, account, password string, device Device, solution *AigisSolution, options ...CallOption) (LoginResult, error) {
	return c.login(ctx, account, password, device, solution, false, options...)
}

// LoginWeb 请求网页密码登录；只有真实返回了完整Cookie时SessionComplete才为true。
// POST成功不等于已建立跨域网页会话，缺失会话时需调用方继续明确的CrossLogin流程。
func (c *Client) LoginWeb(ctx context.Context, account, password string, device Device, solution *AigisSolution, options ...CallOption) (LoginResult, error) {
	return c.login(ctx, account, password, device, solution, true, options...)
}
func (c *Client) login(ctx context.Context, account, password string, d Device, solution *AigisSolution, web bool, options ...CallOption) (LoginResult, error) {
	var out LoginResult
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	accountCipher, e := EncryptCredential(strings.TrimSpace(account))
	if e != nil {
		return out, e
	}
	passwordCipher, e := EncryptCredential(password)
	if e != nil {
		return out, e
	}
	headers, e := deviceHeaders(d, web)
	if e != nil {
		return out, e
	}
	if solution != nil {
		value, err := solution.Encode()
		if err != nil {
			return out, err
		}
		headers.Set("x-rpc-aigis", value)
	}
	endpoint := "hoyoverse-app-auth.appLoginByPassword"
	tokenType := 2
	if web {
		endpoint = "mihoyo-passport.webLoginByPassword"
		tokenType = 6
	}
	opts := append(append([]CallOption(nil), options...), WithHeaders(headers))
	// 新登录不得带入原实例的其他账号Cookie；设备和结果只归本次显式登录所有。
	anonymous := c.ForAccount(Credentials{})
	raw, e := anonymous.Invoke(ctx, endpoint, Parameters{Body: map[string]any{"account": accountCipher, "password": passwordCipher, "token_type": tokenType}}, opts...)
	if e != nil {
		return out, e
	}
	reply, e := Decode[LoginData](raw)
	if e != nil {
		return out, e
	}
	out.Data = reply.Data
	if web {
		cookie := readSetCookies(reply.Headers)
		out.Cookie = cookie
		values, _ := ParseCookie(cookie)
		out.SessionComplete = values["account_id_v2"] != "" && (values["ltoken_v2"] != "" || values["cookie_token_v2"] != "")
		return out, nil
	}
	uid := rawString(reply.Data.UserInfo["aid"])
	mid := rawString(reply.Data.UserInfo["mid"])
	if uid == "" || reply.Data.Token.Token == "" {
		return out, errors.New("App登录未返回SToken及账号ID，不能视为绑定完成")
	}
	cookie, e := c.ExchangeSToken(ctx, STokenSession{SToken: reply.Data.Token.Token, UID: uid, MID: mid}, d)
	if e != nil {
		return out, e
	}
	out.Cookie = cookie
	out.SessionComplete = true
	return out, nil
}
func rawString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// ExchangeSToken 在同一账号下换取CookieToken和LToken，只有两步都成功才返回新的完整Cookie。
// 不修改旧凭证、不落盘、不自动绑定到微信ID，也不对POST交换操作自动重试。
func (c *Client) ExchangeSToken(ctx context.Context, session STokenSession, d Device) (Secret, error) {
	if session.SToken == "" || session.UID == "" || d.ID == "" {
		return "", errors.New("SToken交换需要明确账号及设备")
	}
	values := map[string]string{"stoken": session.SToken.Reveal(), "stuid": session.UID}
	if session.MID != "" {
		values["mid"] = session.MID
	}
	cookie, e := formatCookies(values)
	if e != nil {
		return "", e
	}
	scoped := c.ForAccount(Credentials{Cookie: cookie, DeviceID: d.ID, DeviceFP: d.FP})
	headers := http.Header{"X-Rpc-Client_type": []string{"5"}, "X-Rpc-App_version": []string{"2.54.0"}}
	body := map[string]string{"stoken": session.SToken.Reveal(), "uid": session.UID}
	r, e := scoped.Invoke(ctx, "hoyoverse-token-exchange.getCookieAccountInfoBySToken", Parameters{Body: body}, WithHeaders(headers))
	if e != nil {
		return "", e
	}
	var first struct {
		UID   json.RawMessage `json:"uid"`
		Token string          `json:"cookie_token"`
	}
	if json.Unmarshal(r.Data, &first) != nil || first.Token == "" {
		return "", errors.New("换取CookieToken的响应不完整")
	}
	uid := rawString(first.UID)
	if uid != "" && uid != session.UID {
		return "", errors.New("Token交换响应账号不一致")
	}
	r, e = scoped.Invoke(ctx, "hoyoverse-token-exchange.getLTokenBySToken", Parameters{Body: body}, WithHeaders(headers))
	if e != nil {
		return "", e
	}
	var second struct {
		Token string `json:"ltoken"`
	}
	if json.Unmarshal(r.Data, &second) != nil || second.Token == "" {
		return "", errors.New("换取LToken的响应不完整")
	}
	values["account_id"] = session.UID
	values["cookie_token"] = first.Token
	values["ltuid"] = session.UID
	values["ltoken"] = second.Token
	return formatCookies(values)
}

// ParseCookie 按首个等号切分，保留Token内的等号；拒绝换行、过大内容和冲突的重复名称。
func ParseCookie(cookie Secret) (map[string]string, error) {
	if strings.ContainsAny(cookie.Reveal(), "\r\n") {
		return nil, errors.New("Cookie不能包含换行")
	}
	s := strings.TrimSpace(cookie.Reveal())
	if prefix, value, ok := strings.Cut(s, ":"); ok && strings.EqualFold(strings.TrimSpace(prefix), "cookie") {
		s = strings.TrimSpace(value)
	}
	if len(s) > 16<<10 || strings.ContainsAny(s, "\r\n") {
		return nil, errors.New("Cookie格式无效")
	}
	out := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || name == "" {
			if strings.TrimSpace(part) != "" {
				return nil, errors.New("Cookie字段无效")
			}
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if err := (&http.Cookie{Name: name, Value: value}).Valid(); err != nil {
			return nil, errors.New("Cookie字段格式无效")
		}
		if previous, exists := out[name]; exists && previous != value {
			return nil, errors.New("Cookie包含冲突的重复字段")
		}
		out[name] = value
	}
	return out, nil
}
func formatCookies(values map[string]string) (Secret, error) {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, k := range keys {
		v := values[k]
		if strings.ContainsAny(k, "=;\r\n \t") || strings.ContainsAny(v, ";\r\n") {
			return "", errors.New("Cookie字段包含非法字符")
		}
		if out.Len() > 0 {
			out.WriteString("; ")
		}
		out.WriteString(k + "=" + v)
	}
	return Secret(out.String()), nil
}

// MergeCookies 仅用于同一个明确账号的会话更新；调用者负责确认来源响应属于该账号。
func MergeCookies(existing, addition Secret) (Secret, error) {
	values, e := ParseCookie(existing)
	if e != nil {
		return "", e
	}
	extra, e := ParseCookie(addition)
	if e != nil {
		return "", e
	}
	for k, v := range extra {
		values[k] = v
	}
	return formatCookies(values)
}
func readSetCookies(headers http.Header) Secret {
	response := http.Response{Header: headers}
	values := map[string]string{}
	for _, cookie := range response.Cookies() {
		values[cookie.Name] = cookie.Value
	}
	out, _ := formatCookies(values)
	return out
}

// GameRole 是绑定过程发现的账号角色，不包含Cookie或Token。
type GameRole struct {
	GameBiz  string `json:"game_biz"`
	UID      string `json:"uid"`
	Region   string `json:"region"`
	Nickname string `json:"nickname"`
	Level    string `json:"level"`
}

// Binding 是已核对会话的绑定材料；Credentials只能交给所属Provider的受控加密存储。
type Binding struct {
	AccountID   string          `json:"account_id"`
	Roles       []GameRole      `json:"roles"`
	UserInfo    json.RawMessage `json:"-"`
	Credentials Credentials     `json:"-"`
}

// BindCookie 验证登录材料并读取角色；不保存数据库、不生成公开绑定页，也不把账号票据当游戏客户端登录凭据。
func (c *Client) BindCookie(ctx context.Context, credentials Credentials) (Binding, error) {
	var out Binding
	cookies, e := ParseCookie(credentials.Cookie)
	if e != nil {
		return out, e
	}
	first := func(names ...string) string {
		for _, n := range names {
			if v := cookies[n]; v != "" {
				return v
			}
		}
		return ""
	}
	uid := first("ltuid_v2", "ltuid", "account_id_v2", "account_id", "stuid")
	if uid == "" || first("ltoken_v2", "ltoken", "cookie_token_v2", "cookie_token") == "" {
		return out, errors.New("绑定需要账号ID及LToken或CookieToken")
	}
	if credentials.DeviceID == "" {
		return out, errors.New("绑定必须提供账号会话的设备ID")
	}
	for _, key := range []string{"ltuid_v2", "ltuid", "account_id_v2", "account_id", "stuid"} {
		if value := cookies[key]; value != "" && value != uid {
			return out, errors.New("Cookie中存在多个不同账号ID，不能混合绑定")
		}
	}
	normalized, e := formatCookies(cookies)
	if e != nil {
		return out, e
	}
	credentials.Cookie = normalized
	scoped := c.ForAccount(credentials)
	appSession := first("stoken", "stoken_v2") != "" && cookies["stuid"] != ""
	if !appSession {
		endpoint := "mihoyo-passport.verifyLToken"
		if first("ltoken_v2", "ltoken") == "" {
			endpoint = "passport-extra.verifyCookieToken"
		}
		reply, err := scoped.Invoke(ctx, endpoint, Parameters{Body: struct{}{}})
		if err != nil {
			return out, err
		}
		out.UserInfo = reply.Data
		var data struct {
			UserInfo map[string]json.RawMessage `json:"user_info"`
		}
		if json.Unmarshal(reply.Data, &data) != nil {
			return out, ErrInvalidReply
		}
		verified := rawString(data.UserInfo["aid"])
		if verified != "" && verified != uid {
			return out, errors.New("已验证会话与Cookie中的账号不一致")
		}
	}
	out.AccountID = uid
	out.Credentials = credentials
	out.Roles = make([]GameRole, 0)
	// 通过按游戏角色接口真实验证Cookie；不会仅凭客户端提交的UID就宣布绑定成功。
	for _, biz := range []string{"hk4e_global", "hkrpg_global", "nap_global"} {
		reply, err := scoped.Invoke(ctx, "hoyoverse-game-roles.getUserGameRolesByCookie", Parameters{Query: url.Values{"game_biz": []string{biz}}})
		if err != nil {
			return Binding{}, err
		}
		var data struct {
			List []struct {
				GameUID  json.RawMessage `json:"game_uid"`
				Region   string          `json:"region"`
				Nickname string          `json:"nickname"`
				Level    json.RawMessage `json:"level"`
				GameBiz  string          `json:"game_biz"`
			} `json:"list"`
		}
		if json.Unmarshal(reply.Data, &data) != nil {
			return Binding{}, ErrInvalidReply
		}
		for _, r := range data.List {
			gameBiz := r.GameBiz
			if gameBiz == "" {
				gameBiz = biz
			}
			out.Roles = append(out.Roles, GameRole{GameBiz: gameBiz, UID: rawString(r.GameUID), Region: r.Region, Nickname: r.Nickname, Level: rawString(r.Level)})
		}
	}
	return out, nil
}

// RefreshCredentials 使用当前账号已有SToken续期，返回新值而不原地修改、写库或调度后台任务。
// 只有CookieToken和LToken都交换成功后才合并，避免中途失败破坏仍可用的旧会话。
func (c *Client) RefreshCredentials(ctx context.Context, current Credentials) (Credentials, error) {
	values, e := ParseCookie(current.Cookie)
	if e != nil {
		return Credentials{}, e
	}
	get := func(names ...string) string {
		for _, k := range names {
			if values[k] != "" {
				return values[k]
			}
		}
		return ""
	}
	token := get("stoken", "stoken_v2")
	uid := get("stuid", "ltuid", "account_id", "ltuid_v2", "account_id_v2")
	if token == "" || uid == "" {
		return Credentials{}, errors.New("当前会话没有可续期的SToken与账号ID")
	}
	for _, k := range []string{"ltuid", "ltuid_v2", "account_id", "account_id_v2", "stuid"} {
		if values[k] != "" && values[k] != uid {
			return Credentials{}, errors.New("续期会话的账号ID不一致")
		}
	}
	cookie, e := c.ExchangeSToken(ctx, STokenSession{SToken: Secret(token), UID: uid, MID: values["mid"]}, Device{ID: current.DeviceID, FP: current.DeviceFP})
	if e != nil {
		return Credentials{}, e
	}
	merged, e := MergeCookies(current.Cookie, cookie)
	if e != nil {
		return Credentials{}, e
	}
	out := current
	out.Cookie = merged
	return out, nil
}
