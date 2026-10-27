package pluginhost

import (
	"context"
	"encoding/json"
	"github.com/surgar04/hadalbot/internal/config"
	"github.com/surgar04/hadalbot/internal/httpserver"
	"github.com/surgar04/hadalbot/internal/integrations"
	"github.com/surgar04/hadalbot/internal/modules"
	"github.com/surgar04/hadalbot/internal/testutil"
	"github.com/surgar04/hadalbot/pkg/plugin"
	"github.com/surgar04/hadalbot/pkg/qianxun"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type noChecks struct{}

func (noChecks) Check(context.Context) map[string]string { return map[string]string{} }
func TestDisabledHostHasNoManagerOrDirectoryRead(t *testing.T) {
	c := testutil.Config(t)
	c.Plugins.Directory = "Z:/not-exists"
	r, e := New(c, nil, slog.Default())
	if e != nil || r.Manager != nil {
		t.Fatalf("默认不应初始化运行资源: %v", e)
	}
	if e = r.Start(context.Background(), nil); e != nil {
		t.Fatal(e)
	}
	if e = r.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestGoPluginWiresUpstreamAndControlAuthorization(t *testing.T) {
	calls := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("wxid") != "event-bot" {
			t.Error("机器人上下文丢失")
		}
		var p struct {
			Type string
			Data map[string]string
		}
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p.Type != "sendText" || p.Data["wxid"] != "sender" || p.Data["msg"] != "go pong" {
			t.Error("上游正文不符")
		}
		_, _ = io.WriteString(w, `{"code":200,"result":{}}`)
	}))
	defer up.Close()
	c := testutil.Config(t)
	c.HTTP.APIKey = "test-control-key"
	c.Plugins.Enabled = true
	c.Plugins.EnabledIDs = []string{"example-go"}
	c.Plugins.Grants = map[string][]string{"example-go": {"qx.sendText"}}
	c.Qianxun.Enabled = true
	c.Qianxun.BaseURL = up.URL
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	u, e := integrations.New(c, log)
	if e != nil {
		t.Fatal(e)
	}
	defer u.Close(context.Background())
	host, e := New(c, u, log)
	if e != nil {
		t.Fatal(e)
	}
	defer host.Close(context.Background())
	if e = host.Start(context.Background(), nil); e != nil {
		t.Fatal(e)
	}
	event, e := qianxun.DecodeEvent([]byte(`{"event":10009,"wxid":"event-bot","data":{"type":"recvMsg","wxid":"event-bot","data":{"fromWxid":"sender","msgId":"1","msg":"go ping","msgType":1}}}`))
	if e != nil {
		t.Fatal(e)
	}
	if e = host.Manager.Dispatch(context.Background(), event); e != nil || calls != 1 {
		t.Fatalf("Go插件未完成实际HTTP适配: calls=%d err=%v", calls, e)
	}
	reg := modules.NewRegistry()
	if e = reg.Register(host); e != nil {
		t.Fatal(e)
	}
	router := httpserver.New(c, log, noChecks{}, &httpserver.State{}, reg)
	for _, tc := range []struct {
		key, origin string
		want        int
	}{{"", "", 401}, {c.HTTP.APIKey, "https://untrusted", 403}, {c.HTTP.APIKey, "", 200}} {
		req := httptest.NewRequest("POST", "/api/v1/plugins/example-go/disable", nil)
		if tc.key != "" {
			req.Header.Set("X-API-Key", tc.key)
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("控制鉴权 got=%d want=%d", w.Code, tc.want)
		}
	}
	if host.Manager.Status()[0].State != plugin.Stopped {
		t.Fatal(host.Manager.Status())
	}
}
func TestDiscoveryPreservesLegacyMetadataAndRejectsEscape(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "example")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	manifest := `{"id":"example","name":"旧插件","version":"1.0.0","features":["测试"],"commands":[{"pattern":"ping","description":"pong"}],"configSchema":{"enabled":{"type":"boolean"}},"renderTemplates":{"directory":"resources"}}`
	if e := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "index.ts"), []byte("export default {name:'example'}"), 0600); e != nil {
		t.Fatal(e)
	}
	spec, e := Discover(root, 8)
	if e != nil || len(spec) != 1 {
		t.Fatalf("发现失败: %v", e)
	}
	if spec[0].Manifest.Runtime != "typescript" || len(spec[0].Manifest.ConfigSchema) == 0 || len(spec[0].Manifest.RenderTemplates) == 0 {
		t.Fatal("旧清单信息丢失")
	}
	bad := strings.Replace(manifest, `"version":"1.0.0"`, `"version":"1.0.0","entry":"../outside.ts"`, 1)
	if e = os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(bad), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Discover(root, 8); e == nil {
		t.Fatal("逃逸入口应在发现时拒绝")
	}
}
func TestYAMLPluginConfiguration(t *testing.T) {
	testutil.CleanEnvironment(t)
	p := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "plugins:\n  enabled: true\n  enabled_ids: [example-go]\n  grants:\n    example-go: [qx.sendText]\n  config:\n    example-go:\n      label: 测试\n"
	if e := os.WriteFile(p, []byte(yaml), 0600); e != nil {
		t.Fatal(e)
	}
	c, e := config.Load(p)
	if e != nil {
		t.Fatal(e)
	}
	if c.Plugins.Grants["example-go"][0] != "qx.sendText" || c.Plugins.Config["example-go"]["label"] != "测试" {
		t.Fatal("嵌套插件配置丢失")
	}
}
