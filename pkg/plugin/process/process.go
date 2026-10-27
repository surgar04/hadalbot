// Package process 为TS兼容worker提供有界双向RPC。每个启用的插件独立进程，禁用时不驻留Node。
// 这提供故障隔离而非恶意代码安全沙箱；插件仍需可信发布和操作系统权限隔离。
package process

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/surgar04/hadalbot/pkg/plugin"
	"github.com/surgar04/hadalbot/pkg/qianxun"
)

//go:embed worker/*.mjs
var workerFiles embed.FS
var ErrWorker = errors.New("插件worker已退出、超时或协议无效")

// Options由可信宿主配置，插件清单不能覆盖Node可执行文件或注入任意启动参数。
type Options struct {
	Node          string
	Directory     string
	Entry         string
	HeapMB        int
	MaxFrameBytes int
	MaxPending    int
	BotWXID       string
	Budget        *Budget
}
type rpcError struct {
	Code string `json:"code"`
}
type frame struct {
	ID     string          `json:"id"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}
type pendingCall struct {
	result chan frame
	ctx    context.Context
}

// Worker实现Go插件钩子；工厂每次必须创建新Worker，Init之前不会解析/导入TS。
type Worker struct {
	manifest    plugin.Manifest
	options     Options
	mu          sync.Mutex
	writeMu     sync.Mutex
	pending     map[string]pendingCall
	input       io.WriteCloser
	cmd         *exec.Cmd
	stopped     chan struct{}
	done        chan struct{}
	stopOnce    sync.Once
	host        *plugin.Context
	hostSlots   chan struct{}
	calls       sync.WaitGroup
	sequence    atomic.Uint64
	temporary   string
	hooks       map[string]bool
	releaseOnce sync.Once
	acquired    bool
}

func New(m plugin.Manifest, o Options) (*Worker, error) {
	m.Runtime = "typescript"
	normalized, e := plugin.Normalize(m)
	if e != nil {
		return nil, e
	}
	if o.Node == "" {
		o.Node = "node"
	}
	if o.Budget == nil {
		o.Budget = defaultBudget
	}
	if o.Entry == "" {
		o.Entry = m.Entry
	}
	if o.Entry == "" {
		o.Entry = "index.ts"
	}
	if o.HeapMB == 0 {
		o.HeapMB = 128
	}
	if o.MaxFrameBytes == 0 {
		o.MaxFrameBytes = 1 << 20
	}
	if o.MaxPending == 0 {
		o.MaxPending = 32
	}
	if o.HeapMB < 64 || o.HeapMB > 1024 || o.MaxFrameBytes < 4096 || o.MaxFrameBytes > 2<<20 || o.MaxPending < 1 || o.MaxPending > 128 {
		return nil, errors.New("TS worker预算无效")
	}
	root, e := filepath.Abs(o.Directory)
	if e != nil {
		return nil, errors.New("插件目录无效")
	}
	root, e = filepath.EvalSymlinks(root)
	if e != nil {
		return nil, errors.New("插件目录不存在")
	}
	if filepath.IsAbs(o.Entry) {
		return nil, errors.New("插件入口必须是相对路径")
	}
	entry, e := filepath.EvalSymlinks(filepath.Join(root, o.Entry))
	if e != nil || !inside(root, entry) {
		return nil, errors.New("插件入口不存在或逃逸目录")
	}
	info, e := os.Stat(entry)
	if e != nil || !info.Mode().IsRegular() {
		return nil, errors.New("插件入口不是普通文件")
	}
	o.Directory = root
	o.Entry = entry
	return &Worker{manifest: normalized, options: o, pending: map[string]pendingCall{}, stopped: make(chan struct{}), done: make(chan struct{}), hostSlots: make(chan struct{}, 8), hooks: map[string]bool{}}, nil
}
func inside(root, path string) bool {
	rel, e := filepath.Rel(root, path)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func (w *Worker) Manifest() plugin.Manifest { return w.manifest }
func (w *Worker) Init(ctx context.Context, host *plugin.Context) error {
	w.host = host
	if err := w.options.Budget.acquire(); err != nil {
		return err
	}
	w.acquired = true
	temporary, e := os.MkdirTemp("", "hadalbot-ts-")
	if e != nil {
		w.releaseBudget()
		return errors.New("无法创建worker临时目录")
	}
	w.temporary = temporary
	for _, name := range []string{"worker.mjs", "register.mjs", "loader.mjs", "base.mjs", "events.mjs", "message.mjs", "api.mjs", "index.mjs"} {
		b, _ := workerFiles.ReadFile("worker/" + name)
		if e = os.WriteFile(filepath.Join(temporary, name), b, 0600); e != nil {
			w.removeTemporary()
			return errors.New("无法准备worker")
		}
	}
	importPath := filepath.ToSlash(filepath.Join(temporary, "register.mjs"))
	if !strings.HasPrefix(importPath, "/") {
		importPath = "/" + importPath
	}
	registration := (&url.URL{Scheme: "file", Path: importPath}).String()
	cmd := exec.Command(w.options.Node, "--max-old-space-size="+strconv.Itoa(w.options.HeapMB), "--import", registration, filepath.Join(temporary, "worker.mjs"), w.options.Entry, w.manifest.ID, strconv.Itoa(w.options.MaxFrameBytes))
	cmd.Dir = w.options.Directory
	cmd.Env = environment()
	cmd.Env = append(cmd.Env, "HADALBOT_PLUGIN_ROOT="+w.options.Directory, "HADALBOT_PLUGIN_ID="+w.manifest.ID)
	cmd.Stderr = io.Discard
	hideWindow(cmd)
	output, e := cmd.StdoutPipe()
	if e != nil {
		w.removeTemporary()
		return ErrWorker
	}
	input, e := cmd.StdinPipe()
	if e != nil {
		_ = output.Close()
		w.removeTemporary()
		return ErrWorker
	}
	w.input = input
	w.cmd = cmd
	if e = cmd.Start(); e != nil {
		_ = input.Close()
		_ = output.Close()
		w.removeTemporary()
		return errors.New("TS worker启动失败，请检查Node24及入口依赖")
	}
	// read结束即切断全部pending；Wait仅有一个调用者，确保进程和临时资源最终释放。
	go func() {
		w.read(output)
		w.terminate()
		_ = cmd.Wait()
		w.calls.Wait()
		w.removeTemporary()
		close(w.done)
	}()
	var result json.RawMessage
	result, e = w.call(plugin.WithAccount(ctx, w.options.BotWXID), "init", map[string]any{"protocol": plugin.APIVersion, "config": json.RawMessage(host.Config()), "botWxid": w.options.BotWXID, "capabilities": w.manifest.Capabilities})
	if e == nil {
		var hello struct {
			Hooks []string `json:"hooks"`
		}
		if json.Unmarshal(result, &hello) != nil {
			return ErrWorker
		}
		for _, hook := range hello.Hooks {
			w.hooks[hook] = true
		}
	}
	return e
}
func environment() []string {
	var out []string
	for _, key := range []string{"PATH", "SystemRoot", "SystemDrive", "TEMP", "TMP", "HOME", "USERPROFILE", "LANG", "TZ"} {
		if v, ok := os.LookupEnv(key); ok {
			out = append(out, key+"="+v)
		}
	}
	return out
}
func (w *Worker) removeTemporary() {
	defer w.releaseBudget()
	if w.temporary != "" && strings.HasPrefix(filepath.Base(w.temporary), "hadalbot-ts-") && inside(os.TempDir(), w.temporary) {
		_ = os.RemoveAll(w.temporary)
	}
}
func (w *Worker) releaseBudget() {
	w.releaseOnce.Do(func() {
		if w.acquired {
			<-w.options.Budget.slots
		}
	})
}
func (w *Worker) terminate() {
	w.stopOnce.Do(func() {
		close(w.stopped)
		if w.input != nil {
			_ = w.input.Close()
		}
		if w.cmd != nil && w.cmd.Process != nil {
			_ = w.cmd.Process.Kill()
		}
	})
}
func (w *Worker) send(f frame) error {
	if len(f.Result) > w.options.MaxFrameBytes || len(f.Params) > w.options.MaxFrameBytes {
		return ErrWorker
	}
	body, e := json.Marshal(f)
	if e != nil || len(body) > w.options.MaxFrameBytes {
		return ErrWorker
	}
	// 即使收到没有关联请求的异常帧，回包也必须有写入期限。
	deadline := time.AfterFunc(time.Duration(w.manifest.Limits.HookTimeoutMS)*time.Millisecond, w.terminate)
	defer deadline.Stop()
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	select {
	case <-w.stopped:
		return ErrWorker
	default:
	}
	if w.input == nil {
		return ErrWorker
	}
	if _, e = w.input.Write(append(body, '\n')); e != nil {
		return ErrWorker
	}
	return nil
}
func (w *Worker) read(output io.Reader) {
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), w.options.MaxFrameBytes+1)
	for scanner.Scan() {
		var f frame
		if len(scanner.Bytes()) > w.options.MaxFrameBytes || json.Unmarshal(scanner.Bytes(), &f) != nil || f.ID == "" {
			return
		}
		if f.Method != "" {
			if f.Method != "host.call" || !strings.HasPrefix(f.ID, "n:") {
				return
			}
			select {
			case w.hostSlots <- struct{}{}:
				w.calls.Add(1)
				go w.hostCall(f)
			default:
				_ = w.send(frame{ID: f.ID, Error: &rpcError{Code: "busy"}})
			}
			continue
		}
		w.mu.Lock()
		pending, ok := w.pending[f.ID]
		if ok {
			delete(w.pending, f.ID)
		}
		w.mu.Unlock()
		if ok {
			pending.result <- f
		}
	}
}
func (w *Worker) hostCall(f frame) {
	defer w.calls.Done()
	defer func() { <-w.hostSlots }()
	defer func() {
		if recover() != nil {
			_ = w.send(frame{ID: f.ID, Error: &rpcError{Code: "host_failure"}})
		}
	}()
	var p struct {
		Method string          `json:"method"`
		Args   json.RawMessage `json:"args"`
		Parent string          `json:"parent"`
	}
	if json.Unmarshal(f.Params, &p) != nil {
		_ = w.send(frame{ID: f.ID, Error: &rpcError{Code: "invalid_params"}})
		return
	}
	if p.Method == "log.write" {
		var log struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}
		if json.Unmarshal(p.Args, &log) == nil && len(log.Message) <= 2048 {
			level := slog.LevelInfo
			if log.Level == "warn" {
				level = slog.LevelWarn
			}
			if log.Level == "error" {
				level = slog.LevelError
			}
			w.host.Log.Log(w.host.Scope.Context(), level, log.Message)
		}
		_ = w.send(frame{ID: f.ID, Result: json.RawMessage(`null`)})
		return
	}
	w.mu.Lock()
	parent, ok := w.pending[p.Parent]
	w.mu.Unlock()
	if !ok {
		_ = w.send(frame{ID: f.ID, Error: &rpcError{Code: "orphan_call"}})
		return
	}
	ctx, cancel := context.WithTimeout(parent.ctx, time.Duration(w.manifest.Limits.HookTimeoutMS)*time.Millisecond)
	defer cancel()
	stop := context.AfterFunc(ctx, w.terminate)
	defer stop()
	result, err := w.host.Call(ctx, p.Method, p.Args)
	if len(result) > w.options.MaxFrameBytes-256 {
		result = nil
		err = plugin.ErrTooLarge
	}
	reply := frame{ID: f.ID, Result: result}
	if err != nil {
		reply.Result = nil
		reply.Error = &rpcError{Code: "host_failure"}
	}
	_ = w.send(reply)
}
func (w *Worker) call(parent context.Context, method string, params any) (json.RawMessage, error) {
	timeout := w.manifest.Limits.HookTimeoutMS
	if method == "init" || method == "ready" || method == "dispose" {
		timeout = w.manifest.Limits.LifecycleTimeoutMS
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	id := "g:" + strconv.FormatUint(w.sequence.Add(1), 10)
	body, e := json.Marshal(params)
	if e != nil {
		return nil, ErrWorker
	}
	if len(body) > w.options.MaxFrameBytes-256 {
		return nil, plugin.ErrTooLarge
	}
	ch := make(chan frame, 1)
	w.mu.Lock()
	if len(w.pending) >= w.options.MaxPending {
		w.mu.Unlock()
		return nil, plugin.ErrBusy
	}
	w.pending[id] = pendingCall{result: ch, ctx: ctx}
	w.mu.Unlock()
	defer func() { w.mu.Lock(); delete(w.pending, id); w.mu.Unlock() }()
	// stdin可能被失控worker停止读取；ctx回调直接Kill，保证Write不会无限阻塞。
	stop := context.AfterFunc(ctx, w.terminate)
	defer stop()
	if e = w.send(frame{ID: id, Method: method, Params: body}); e != nil {
		w.terminate()
		return nil, e
	}
	select {
	case <-ctx.Done():
		w.terminate()
		return nil, ctx.Err()
	case <-w.stopped:
		return nil, ErrWorker
	case f := <-ch:
		if f.Error != nil {
			return nil, ErrWorker
		}
		return f.Result, nil
	}
}
func (w *Worker) Ready(ctx context.Context, _ *plugin.Context) error {
	if !w.hooks["onReady"] {
		return nil
	}
	_, e := w.call(ctx, "ready", nil)
	return e
}
func (w *Worker) ObserveMessage(ctx context.Context, m plugin.Message, _ *plugin.Context) error {
	if !w.hooks["observeMessage"] {
		return nil
	}
	_, e := w.call(ctx, "observe", messageWire(m))
	return e
}
func (w *Worker) OnEvent(ctx context.Context, e qianxun.Event, _ *plugin.Context) (bool, error) {
	if !w.hooks["onEvent"] {
		return false, nil
	}
	raw := legacyEvent(e)
	b, err := w.call(ctx, "event", raw)
	var handled bool
	_ = json.Unmarshal(b, &handled)
	return handled, err
}
func (w *Worker) OnMessage(ctx context.Context, m plugin.Message, _ *plugin.Context) (bool, error) {
	if !w.hooks["onMessage"] {
		return false, nil
	}
	b, e := w.call(ctx, "message", messageWire(m))
	var handled bool
	_ = json.Unmarshal(b, &handled)
	return handled, e
}
func (w *Worker) Dispose(ctx context.Context) error {
	if w.cmd == nil || w.cmd.Process == nil {
		w.removeTemporary()
		return nil
	}
	select {
	case <-w.done:
		return nil
	default:
	}
	_, err := w.call(ctx, "dispose", nil)
	w.terminate()
	select {
	case <-w.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (w *Worker) Healthy() bool {
	select {
	case <-w.stopped:
		return false
	default:
		return true
	}
}

// legacyEvent为直连模式补旧框架外壳；框架原文保持完全不变。
func legacyEvent(e qianxun.Event) json.RawMessage {
	if e.Mode == "framework" {
		return e.Raw
	}
	body, _ := json.Marshal(map[string]any{"event": e.ID, "wxid": e.WXID, "data": map[string]any{"type": e.Kind, "wxid": e.WXID, "port": e.Port, "pid": e.PID, "data": e.Payload}})
	return body
}

// 仅传一份事件原文和精确ID，避免正文、raw、event在IPC中重复三次。
func messageWire(m plugin.Message) any {
	if e, err := qianxun.DecodeEvent(m.Event); err == nil {
		m.Event = legacyEvent(e)
	}
	return struct {
		Event json.RawMessage `json:"event"`
		MsgID string          `json:"msgId"`
	}{Event: m.Event, MsgID: m.MsgID}
}
