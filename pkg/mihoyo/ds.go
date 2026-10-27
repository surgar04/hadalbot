package mihoyo

import (
	"crypto/md5" // 协议要求MD5；不是用于保存密码或替代现代安全摘要。
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// 以下salt为旧客户端公开常量，可能随上游版本失效；不是用户账号凭证。
const LegacyHSRSalt = "6s25p5ox5y14umn1p61aqyyvbvvl3lrt"
const LegacyOverseasSalt = "6cqshh5dhw73bzxn20oexa9k516chk7s"

func nonce() (string, error) {
	alphabet := "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		n, e := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if e != nil {
			return "", e
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), nil
}
func validNonce(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789", c) {
			return false
		}
	}
	return true
}

// DS1 构造不包含query/body的旧版海外签名，方便使用固定时间和nonce进行契约回归。
func DS1(salt string, timestamp int64, random string) (string, error) {
	if salt == "" || timestamp < 1 || !validNonce(random) {
		return "", errors.New("DS1签名输入无效")
	}
	source := fmt.Sprintf("salt=%s&t=%d&r=%s", salt, timestamp, random)
	digest := md5.Sum([]byte(source))
	return fmt.Sprintf("%d,%s,%s", timestamp, random, hex.EncodeToString(digest[:])), nil
}

// DS2 对实际将发送的正文及RawQuery逐字节签名，不能在签名后重排参数。
// 该函数不会猜测App当前salt；调用方只应使用已核实的协议常量。
func DS2(salt string, timestamp int64, random, query string, body []byte) (string, error) {
	if salt == "" || timestamp < 1 || !validNonce(random) {
		return "", errors.New("DS2签名输入无效")
	}
	source := fmt.Sprintf("salt=%s&t=%d&r=%s&b=%s&q=%s", salt, timestamp, random, body, query)
	digest := md5.Sum([]byte(source))
	return fmt.Sprintf("%d,%s,%s", timestamp, random, hex.EncodeToString(digest[:])), nil
}

// RandomDS1 使用加密安全随机数和当前Unix秒数，为一次请求创建DS1。
func RandomDS1(salt string) (string, error) {
	r, e := nonce()
	if e != nil {
		return "", e
	}
	return DS1(salt, time.Now().Unix(), r)
}

// RandomDS2 使用加密安全nonce，为已固定的HTTP正文与查询串签名。
func RandomDS2(salt, query string, body []byte) (string, error) {
	r, e := nonce()
	if e != nil {
		return "", e
	}
	return DS2(salt, time.Now().Unix(), r, query, body)
}
