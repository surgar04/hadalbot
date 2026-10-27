package qianxun

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/surgar04/hadalbot/pkg/gateway"
)

// 按官方完整目录逐项调用实际生成的方法，而非只测一个任意type的通用POST。
func TestEveryOfficialHTTPContract(t *testing.T) {
	if len(Operations()) != 99 {
		t.Fatalf("官方HTTP契约数量变化，需要重新核对: %d", len(Operations()))
	}
	for _, op := range Operations() {
		t.Run(op.Mode+"/"+op.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expected := "/qianxun/httpapi"
				if op.Mode == "direct" {
					expected = "/wechat/httpapi"
				}
				if op.Method == "GET" {
					expected += "/file"
				}
				if r.Method != op.Method || r.URL.Path != expected {
					t.Errorf("错误路由: %s %s", r.Method, r.URL.Path)
				}
				if r.URL.Query().Get("safekey") != "key +&=?" {
					t.Error("safekey编码错误")
				}
				if op.Mode == "framework" && r.URL.Query().Get("wxid") != "bot-account" {
					t.Error("账号参数缺失")
				}
				if op.Mode == "direct" && r.URL.Query().Has("wxid") {
					t.Error("直接客户端模式不应带框架wxid查询参数")
				}
				if op.Method == "GET" {
					if r.URL.Query().Get("path") != "C:\\图片\\a&b.png" {
						t.Error("文件路径编码错误")
					}
					_, _ = io.WriteString(w, "image")
					return
				}
				var body struct {
					Type string          `json:"type"`
					Data json.RawMessage `json:"data"`
				}
				if json.NewDecoder(r.Body).Decode(&body) != nil || body.Type != op.Name {
					t.Error("请求外壳错误")
				}
				var got, want map[string]any
				_ = json.Unmarshal(body.Data, &got)
				_ = json.Unmarshal(op.RequestExample, &want)
				for key, value := range want {
					if !reflect.DeepEqual(got[key], value) {
						t.Errorf("请求字段%s与官方示例不一致: got=%v want=%v", key, got[key], value)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(op.ResponseExample)
			}))
			defer server.Close()
			g, e := gateway.New(gateway.Options{Services: map[string]gateway.Limits{"qianxun": {Concurrency: 2, QueueSize: 2, Origins: []string{server.URL}}}})
			if e != nil {
				t.Fatal(e)
			}
			defer g.Close()
			cfg := Config{BaseURL: server.URL, SafeKey: "key +&=?", WXID: "bot-account"}
			var client any
			if op.Mode == "framework" {
				client, e = NewFramework(g, cfg)
			} else {
				client, e = NewDirect(g, cfg)
			}
			if e != nil {
				t.Fatal(e)
			}
			if op.Method == "GET" {
				var result strings.Builder
				var err error
				if c, ok := client.(*FrameworkClient); ok {
					_, err = c.DownloadFileTo(context.Background(), "C:\\图片\\a&b.png", &result)
				} else {
					_, err = client.(*DirectClient).DownloadFileTo(context.Background(), "C:\\图片\\a&b.png", &result)
				}
				if err != nil || result.String() != "image" {
					t.Fatal(err)
				}
				return
			}
			method := reflect.ValueOf(client).MethodByName(op.GoMethod)
			if !method.IsValid() {
				t.Fatalf("缺少强类型方法: %s", op.GoMethod)
			}
			argument := reflect.New(method.Type().In(1))
			if e = json.Unmarshal(op.RequestExample, argument.Interface()); e != nil {
				t.Fatalf("官方请求不能解码: %v", e)
			}
			result := method.CallSlice([]reflect.Value{reflect.ValueOf(context.Background()), argument.Elem(), reflect.ValueOf([]CallOption{})})
			if !result[1].IsNil() {
				t.Fatalf("官方响应不兼容: %v", result[1].Interface())
			}
		})
	}
}
func TestBusinessAndProtocolErrorsAreDistinct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":403,"msg":"sensitive-value","result":null}`)
	}))
	defer server.Close()
	g, e := gateway.New(gateway.Options{Services: map[string]gateway.Limits{"qianxun": {Concurrency: 1, Origins: []string{server.URL}}}})
	if e != nil {
		t.Fatal(e)
	}
	defer g.Close()
	c, e := NewFramework(g, Config{BaseURL: server.URL})
	if e != nil {
		t.Fatal(e)
	}
	reply, e := c.Call(context.Background(), "sendText", map[string]string{"wxid": "target", "msg": "hello"})
	var business *BusinessError
	if !errors.As(e, &business) || reply.Code != 403 || strings.Contains(e.Error(), "sensitive-value") {
		t.Fatalf("业务错误语义或脱敏失败: %v", e)
	}
	if _, e = c.Call(context.Background(), "arbitrary-write", nil); !errors.Is(e, ErrUnsupported) {
		t.Fatal("未知接口不应发出")
	}
}
