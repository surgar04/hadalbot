package mihoyo

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/surgar04/hadalbot/pkg/gateway"
)

func mockedClient(t *testing.T, handler http.HandlerFunc, retries int) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	g, e := gateway.New(gateway.Options{Services: map[string]gateway.Limits{"mihoyo": {Concurrency: 4, QueueSize: 4, Origins: []string{server.URL}}}, MaxRetries: retries})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(g.Close)
	overrides := map[string]string{}
	for _, origin := range Origins() {
		overrides[origin] = server.URL
	}
	c, e := New(g, Options{OriginOverrides: overrides})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestAppLoginExchangeBindingAndAccountIsolation(t *testing.T) {
	var roleCalls atomic.Int64
	c := mockedClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/account/ma-passport/api/appLoginByPassword":
			if r.Header.Get("Cookie") != "" {
				t.Error("新登录不应带入另一个账号Cookie")
			}
			if r.Header.Get("x-rpc-device_id") != "app-device" {
				t.Error("App设备上下文错误")
			}
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				t.Error("登录正文无效")
			}
			for _, key := range []string{"account", "password"} {
				value, _ := body[key].(string)
				if len(value) != 344 || strings.Contains(value, "plaintext") {
					t.Error("未按协议RSA加密")
				}
			}
			_, _ = io.WriteString(w, `{"retcode":0,"message":"OK","data":{"token":{"token_type":2,"token":"new-stoken"},"user_info":{"aid":"123","mid":"mid"}}}`)
		case "/account/auth/api/getCookieAccountInfoBySToken":
			if !strings.Contains(r.Header.Get("Cookie"), "stuid=123") {
				t.Error("交换凭证账号错误")
			}
			_, _ = io.WriteString(w, `{"retcode":0,"message":"OK","data":{"uid":"123","cookie_token":"new-cookie-token"}}`)
		case "/account/auth/api/getLTokenBySToken":
			_, _ = io.WriteString(w, `{"retcode":0,"message":"OK","data":{"ltoken":"new-ltoken"}}`)
		case "/binding/api/getUserGameRolesByCookie":
			roleCalls.Add(1)
			_, _ = fmt.Fprintf(w, `{"retcode":0,"message":"OK","data":{"list":[{"game_uid":"777","region":"os_asia","nickname":"name","level":60,"game_biz":%q}]}}`, r.URL.Query().Get("game_biz"))
		default:
			t.Errorf("出现意外请求: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}, 0)
	old := c.ForAccount(Credentials{Cookie: Secret("ltuid=other-account; ltoken=old-token"), DeviceID: "old-device"})
	result, e := old.LoginApp(context.Background(), " plaintext-account ", "plaintext-password", Device{ID: "device", AppDeviceID: "app-device"}, nil)
	if e != nil || !result.SessionComplete {
		t.Fatalf("登录交换失败: %v", e)
	}
	cookies, e := ParseCookie(result.Cookie)
	if e != nil || cookies["ltuid"] != "123" || cookies["ltoken"] != "new-ltoken" {
		t.Fatal("交换结果不完整")
	}
	binding, e := c.BindCookie(context.Background(), Credentials{Cookie: Secret("Cookie: " + result.Cookie.Reveal()), DeviceID: "device"})
	if e != nil || binding.AccountID != "123" || len(binding.Roles) != 3 || roleCalls.Load() != 3 {
		t.Fatalf("绑定验证错误: %v", e)
	}
	if _, e = c.BindCookie(context.Background(), Credentials{Cookie: Secret("ltuid_v2=one; stuid=two; stoken=x; ltoken=y"), DeviceID: "device"}); e == nil {
		t.Fatal("不应接受不同账号的混合Cookie")
	}
	if old.core.options.Credentials.Cookie.Reveal() != "ltuid=other-account; ltoken=old-token" {
		t.Fatal("登录修改了原账号实例")
	}
}
func TestProtocolBusinessChallengeAndIncompleteWebSession(t *testing.T) {
	var mode atomic.Int32
	c := mockedClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case 0:
			w.Header().Set("x-rpc-aigis", `{"session_id":"challenge","data":"{}"}`)
			_, _ = io.WriteString(w, `{"retcode":-3101,"message":"captcha"}`)
		case 1:
			_, _ = io.WriteString(w, `{"retcode":0,"message":"OK","data":{}}`)
		case 2:
			_, _ = io.WriteString(w, `<html>not-json</html>`)
		case 3:
			_, _ = io.WriteString(w, `{"retcode":-100,"message":"private-cookie-value","data":{}}`)
		}
	}, 0)
	_, e := c.Invoke(context.Background(), "mihoyo-passport.webLoginByPassword", Parameters{Body: struct{}{}})
	var challenge *ChallengeError
	if !errors.As(e, &challenge) || challenge.Kind != "x-rpc-aigis" {
		t.Fatalf("未明确返回挑战: %v", e)
	}
	mode.Store(1)
	result, e := c.LoginWeb(context.Background(), "account", "password", Device{ID: "device", FP: "fp"}, nil)
	if e != nil || result.SessionComplete || result.Cookie != "" {
		t.Fatalf("Web POST成功不能冒充绑定成功: %v", e)
	}
	mode.Store(2)
	if _, e = c.Invoke(context.Background(), "mihoyo-passport.verifyLToken", Parameters{}); !errors.Is(e, ErrInvalidReply) {
		t.Fatal("非JSON不能当作retcode0")
	}
	mode.Store(3)
	_, e = c.Invoke(context.Background(), "mihoyo-passport.verifyLToken", Parameters{})
	var business *BusinessError
	if !errors.As(e, &business) || strings.Contains(e.Error(), "private-cookie-value") {
		t.Fatal("业务错误分类或脱敏错误")
	}
}
func TestDSWireInputAndWriteGETNeverRetried(t *testing.T) {
	var calls atomic.Int64
	c := mockedClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.Contains(r.URL.Path, "webExchangeCdkey") {
			w.WriteHeader(503)
			return
		}
		body, _ := io.ReadAll(r.Body)
		source := "salt=salt&t=1&r=abc123&b=" + string(body) + "&q=" + r.URL.RawQuery
		sum := md5.Sum([]byte(source))
		expected := "1,abc123," + hex.EncodeToString(sum[:])
		if r.Header.Get("DS") != expected {
			t.Error("签名与实际发送的query/body不一致")
		}
		_, _ = io.WriteString(w, `{"retcode":0,"message":"OK","data":{}}`)
	}, 2)
	_, e := c.Invoke(context.Background(), "starrail-app.getStarRailAppIndex", Parameters{Query: url.Values{"role_id": []string{"a & b"}, "server": []string{"os_asia"}}}, WithDS2Signer(func(_ context.Context, input SigningInput) (string, error) {
		return DS2("salt", 1, "abc123", input.RawQuery, input.Body)
	}))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Invoke(context.Background(), "starrail-app.getStarRailAppIndex", Parameters{}); !errors.Is(e, ErrDS2Required) {
		t.Fatal("未知App DS2不应猜测或发出请求")
	}
	before := calls.Load()
	_, e = c.Invoke(context.Background(), "genshin-redeem.redeemCode", Parameters{Query: url.Values{"cdkey": []string{"test"}}})
	if e == nil || calls.Load()-before != 1 {
		t.Fatal("GET兑换码请求不得重试")
	}
}
func TestCookieConflictsAndAigisEncoding(t *testing.T) {
	if _, e := ParseCookie(Secret("a=1; a=2")); e == nil {
		t.Fatal("冲突Cookie应拒绝")
	}
	if _, e := ParseCookie(Secret("a=b\r\nx=1")); e == nil {
		t.Fatal("Cookie注入应拒绝")
	}
	header, e := (AigisSolution{Version: 3, SessionID: "session", Challenge: "challenge", Validate: "validate", Seccode: "seccode"}).Encode()
	if e != nil || !strings.HasPrefix(header, "session;") {
		t.Fatal(e)
	}
	if _, e = (AigisSolution{Version: 4, SessionID: "session"}).Encode(); e == nil {
		t.Fatal("不完整结果不得提交")
	}
}
