package process

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/surgar04/hadalbot/pkg/plugin"
	"github.com/surgar04/hadalbot/pkg/qianxun"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func node(t *testing.T) string {
	t.Helper()
	p, e := exec.LookPath("node")
	if e != nil {
		t.Skip("未安装Node，TS真实进程测试未执行")
	}
	b, e := exec.Command(p, "--version").Output()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(string(b), "v24.") && !strings.HasPrefix(string(b), "v26.") {
		t.Skip("TS测试需要Node24以上")
	}
	return p
}
func setup(t *testing.T, dir, source string, caller plugin.Caller, timeout int) (*plugin.Manager, **Worker) {
	t.Helper()
	n := node(t)
	if dir == "" {
		dir = t.TempDir()
		if e := os.WriteFile(filepath.Join(dir, "index.ts"), []byte(source), 0600); e != nil {
			t.Fatal(e)
		}
	}
	meta := plugin.Manifest{ID: "example", Version: "1.0.0", Runtime: "typescript", Capabilities: []string{"qx.sendText"}, Limits: plugin.Limits{HookTimeoutMS: timeout, LifecycleTimeoutMS: 5000}}
	m, e := plugin.New(plugin.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Caller: caller})
	if e != nil {
		t.Fatal(e)
	}
	var worker *Worker
	if e = m.Register(meta, func() plugin.Plugin {
		var err error
		worker, err = New(meta, Options{Directory: dir, Node: n, BotWXID: "default-bot"})
		if err != nil {
			panic(err)
		}
		return worker
	}, nil, true); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = m.Close(ctx)
	})
	return m, &worker
}
func incoming(t *testing.T, bot string) qianxun.Event {
	t.Helper()
	e, err := qianxun.DecodeEvent([]byte(`{"event":10009,"wxid":"` + bot + `","data":{"type":"recvMsg","wxid":"` + bot + `","data":{"fromWxid":"sender","msgId":"184467440737095516151","msg":"ping","msgType":1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestOriginalLegacyPluginRunsUnmodified(t *testing.T) {
	dir, e := filepath.Abs("testdata/legacy")
	if e != nil {
		t.Fatal(e)
	}
	var mu sync.Mutex
	var calls []string
	caller := plugin.CallFunc(func(ctx context.Context, id, method string, args json.RawMessage) (json.RawMessage, error) {
		var p struct{ WXID, Msg string }
		if e := json.Unmarshal(args, &p); e != nil {
			return nil, e
		}
		if id != "example" || method != "qx.sendText" || p.WXID != "sender" || p.Msg != "pong" {
			t.Errorf("旧API协议错误: %s %s", id, method)
		}
		mu.Lock()
		calls = append(calls, plugin.AccountWXID(ctx))
		mu.Unlock()
		return json.RawMessage(`{"code":200,"result":{}}`), nil
	})
	m, w := setup(t, dir, "", caller, 1000)
	if e = m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	for _, bot := range []string{"robot-a", "robot-b"} {
		if e = m.Dispatch(context.Background(), incoming(t, bot)); e != nil {
			t.Fatal(e)
		}
	}
	mu.Lock()
	if strings.Join(calls, ",") != "robot-a,robot-b" {
		t.Fatal(calls)
	}
	mu.Unlock()
	if e = m.Disable(context.Background(), "example"); e != nil {
		t.Fatal(e)
	}
	select {
	case <-(*w).done:
	case <-time.After(time.Second):
		t.Fatal("子进程未退出")
	}
	if _, e = os.Stat((*w).temporary); !os.IsNotExist(e) {
		t.Fatal("临时资源未清理")
	}
}
func TestNativeTypeScriptTransformAndCompatibilityImports(t *testing.T) {
	src := `import {PluginBase} from '@/bot/plugin-base.js'; import {QxEventId} from '@/api/types/events.js'; enum Answer { Pong='pong' }; export default new class extends PluginBase {name='example'; async onMessage(message,ctx){if(QxEventId.PRIVATE_MSG!==10009||message.msgId!=='184467440737095516151')throw Error('bad');await ctx.api.request('sendText',{wxid:message.senderWxid,msg:Answer.Pong});return true}}`
	count := 0
	m, _ := setup(t, "", src, plugin.CallFunc(func(context.Context, string, string, json.RawMessage) (json.RawMessage, error) {
		count++
		return json.RawMessage(`{}`), nil
	}), 1000)
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := m.Dispatch(context.Background(), incoming(t, "robot")); e != nil || count != 1 {
		t.Fatalf("TS transform=%d err=%v", count, e)
	}
}
func TestWorkerTimeoutCrashAndInvalidProtocol(t *testing.T) {
	for _, source := range []string{`export default {name:'example',onMessage(){for(;;){}}}`, `export default {name:'example',onMessage(){process.exit(7)}}`, `export default {name:'example',onMessage(){process.stdout.write('x'.repeat(1100000));return true}}`} {
		t.Run(source[:35], func(t *testing.T) {
			m, w := setup(t, "", source, nil, 100)
			if e := m.Start(context.Background()); e != nil {
				t.Fatal(e)
			}
			start := time.Now()
			if e := m.Dispatch(context.Background(), incoming(t, "bot")); e == nil {
				t.Fatal("worker故障不能成功")
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("故障未在预算内返回")
			}
			select {
			case <-(*w).done:
			case <-time.After(2 * time.Second):
				t.Fatal("worker未被回收")
			}
			if s := m.Status()[0]; s.State != plugin.Failed {
				t.Fatal(s)
			}
		})
	}
}
func TestUnsupportedLegacyImportsFailAndEntryEscapeRejected(t *testing.T) {
	m, _ := setup(t, "", `import '@/database/document-store.js';export default {name:'example'}`, nil, 1000)
	if e := m.Start(context.Background()); e == nil {
		t.Fatal("未迁移服务不能假成功")
	}
	root := t.TempDir()
	_, e := New(plugin.Manifest{ID: "escape", Version: "1.0.0"}, Options{Directory: root, Entry: "../outside.ts"})
	if e == nil {
		t.Fatal("目录逃逸应拒绝")
	}
}
func TestUndeclaredCapabilityCannotReachHost(t *testing.T) {
	called := false
	m, _ := setup(t, "", `export default {name:'example',async onMessage(m,c){await c.api.delFriend({wxid:'victim'});return true}}`, plugin.CallFunc(func(context.Context, string, string, json.RawMessage) (json.RawMessage, error) {
		called = true
		return json.RawMessage(`{}`), nil
	}), 1000)
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := m.Dispatch(context.Background(), incoming(t, "bot")); !errors.Is(e, plugin.ErrHook) {
		t.Fatal(e)
	}
	if called {
		t.Fatal("未授权方法不应到达宿主服务")
	}
}
func TestSharedProcessBudgetAndRelease(t *testing.T) {
	budget, e := NewBudget(1)
	if e != nil {
		t.Fatal(e)
	}
	if e = budget.acquire(); e != nil {
		t.Fatal(e)
	}
	if e = budget.acquire(); !errors.Is(e, plugin.ErrBusy) {
		t.Fatal(e)
	}
	<-budget.slots
	if e = budget.acquire(); e != nil {
		t.Fatal("释放后应能重新占用")
	}
	<-budget.slots
}
