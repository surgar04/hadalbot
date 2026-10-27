package mihoyo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/surgar04/hadalbot/pkg/gateway"
)

func fillValue(v reflect.Value) {
	if !v.CanSet() {
		return
	}
	if v.Type() == reflect.TypeOf(json.RawMessage{}) {
		v.SetBytes([]byte(`{}`))
		return
	}
	if v.Type() == reflect.TypeOf(json.Number("")) {
		v.SetString("1")
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString("fixture")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int64, reflect.Int:
		v.SetInt(1)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fillValue(v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fillValue(v.Field(i))
		}
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fillValue(v.Index(0))
	}
}

// 对目录中的每个具名方法做一次真实HTTP契约调用，不访问外部账号或游戏服务。
func TestEveryGeneratedUpstreamMethod(t *testing.T) {
	endpoints := Endpoints()
	if len(endpoints) != 179 {
		t.Fatalf("目录发生变化，请核对覆盖范围: %d", len(endpoints))
	}
	for _, endpoint := range endpoints {
		t.Run(endpoint.ID, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != endpoint.Method {
					t.Errorf("method=%s want=%s", r.Method, endpoint.Method)
				}
				if endpoint.RawResponse {
					if r.Header.Get("Cookie") != "" || r.Header.Get("x-rpc-device_id") != "" {
						t.Error("公开第三方上游不应携带账号信息")
					}
					_, _ = io.WriteString(w, `{"uid":"fixture"}`)
					return
				}
				if officialCredentialHost(endpoint.Origin) {
					if r.Header.Get("Cookie") != "ltuid=account; ltoken=token" {
						t.Error("账号Cookie丢失")
					}
				} else if r.Header.Get("Cookie") != "" {
					t.Error("公开上游不应收到HoYoLAB凭证")
				}
				if endpoint.Signature != "none" && r.Header.Get("DS") == "" {
					t.Error("签名缺失")
				}
				if r.Method == "POST" {
					b, e := io.ReadAll(r.Body)
					if e != nil || !json.Valid(b) {
						t.Error("正文不是有效JSON")
					}
				}
				_, _ = io.WriteString(w, `{"retcode":0,"message":"OK","data":{"fixture":true}}`)
			}))
			defer server.Close()
			g, e := gateway.New(gateway.Options{Services: map[string]gateway.Limits{"mihoyo": {Concurrency: 2, QueueSize: 2, Origins: []string{server.URL}}}})
			if e != nil {
				t.Fatal(e)
			}
			defer g.Close()
			overrides := map[string]string{}
			for _, origin := range endpoint.Origins {
				overrides[origin] = server.URL
			}
			c, e := New(g, Options{OriginOverrides: overrides, Credentials: Credentials{Cookie: Secret("ltuid=account; ltoken=token"), DeviceID: "device"}, AllowCandidates: true, DS2Signer: func(context.Context, SigningInput) (string, error) { return "verified-signer", nil }})
			if e != nil {
				t.Fatal(e)
			}
			api := reflect.ValueOf(c).Elem().FieldByName(endpoint.GoGroup)
			if !api.IsValid() {
				t.Fatalf("缺少分组%s", endpoint.GoGroup)
			}
			method := api.MethodByName(endpoint.GoMethod)
			if !method.IsValid() {
				t.Fatalf("缺少方法%s", endpoint.GoMethod)
			}
			argument := reflect.New(method.Type().In(1)).Elem()
			fillValue(argument)
			result := method.CallSlice([]reflect.Value{reflect.ValueOf(context.Background()), argument, reflect.ValueOf([]CallOption{})})
			if !result[1].IsNil() {
				t.Fatalf("封装调用失败: %v", result[1].Interface())
			}
		})
	}
}
func TestSecretFormatting(t *testing.T) {
	value := Secret("do-not-log-this")
	for _, got := range []string{fmt.Sprint(value), fmt.Sprintf("%#v", value)} {
		if got == value.Reveal() {
			t.Fatal("凭证格式化泄露")
		}
	}
	b, e := json.Marshal(Credentials{Cookie: value})
	if e != nil || string(b) != "{}" {
		t.Fatalf("凭证不应进入JSON: %s", b)
	}
}
