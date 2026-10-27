// Package pluginhost负责插件发现、宿主能力适配和应用生命周期装配；SDK本身不读取全局配置。
package pluginhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/surgar04/hadalbot/internal/config"
	"github.com/surgar04/hadalbot/internal/httpserver"
	"github.com/surgar04/hadalbot/internal/integrations"
	"github.com/surgar04/hadalbot/internal/plugins/example"
	"github.com/surgar04/hadalbot/pkg/plugin"
	"github.com/surgar04/hadalbot/pkg/plugin/process"
	"github.com/surgar04/hadalbot/pkg/qianxun"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Runtime struct {
	Manager     *plugin.Manager
	enabled     bool
	unsubscribe func()
	control     chan struct{}
}

// New不启动插件；目录扫描仅在typescript明确启用时执行，不递归加载资源。
func New(c config.Config, upstreams *integrations.Runtime, log *slog.Logger) (*Runtime, error) {
	r := &Runtime{enabled: c.Plugins.Enabled, control: make(chan struct{}, 2)}
	if !r.enabled {
		return r, nil
	}
	caller := plugin.CallFunc(func(ctx context.Context, _ string, method string, args json.RawMessage) (json.RawMessage, error) {
		if !strings.HasPrefix(method, "qx.") {
			return nil, plugin.ErrDenied
		}
		operation := strings.TrimPrefix(method, "qx.")
		wxid := plugin.AccountWXID(ctx)
		if wxid == "" {
			wxid = c.Qianxun.WXID
		}
		var data any
		if json.Unmarshal(args, &data) != nil {
			return nil, errors.New("插件API参数无效")
		}
		if upstreams.Framework != nil {
			reply, err := upstreams.Framework.Call(ctx, operation, json.RawMessage(args), qianxun.WithWXID(wxid))
			return reply.Raw, err
		}
		if upstreams.Direct != nil {
			if c.Qianxun.WXID != "" && wxid != c.Qianxun.WXID {
				return nil, plugin.ErrDenied
			}
			reply, err := upstreams.Direct.Call(ctx, operation, json.RawMessage(args))
			return reply.Raw, err
		}
		return nil, errors.New("千寻上游未启用")
	})
	var err error
	r.Manager, err = plugin.New(plugin.Options{MaxPlugins: c.Plugins.MaxPlugins, Logger: log, Caller: caller})
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, id := range c.Plugins.EnabledIDs {
		if selected[id] {
			return nil, errors.New("重复的启用插件ID")
		}
		selected[id] = true
	}
	installed := map[string]bool{}
	register := func(meta plugin.Manifest, factory plugin.Factory) error {
		grants := c.Plugins.Grants[meta.ID]
		if len(meta.Capabilities) > 0 {
			declared := map[string]bool{}
			for _, v := range meta.Capabilities {
				declared[v] = true
			}
			var intersection []string
			for _, v := range grants {
				if !declared[v] {
					return fmt.Errorf("授权超出插件声明: %s", meta.ID)
				}
				intersection = append(intersection, v)
			}
			meta.Capabilities = intersection
		} else {
			meta.Capabilities = append([]string(nil), grants...)
		}
		body, e := json.Marshal(c.Plugins.Config[meta.ID])
		if e != nil {
			return errors.New("插件配置无法编码")
		}
		if string(body) == "null" {
			body = []byte(`{}`)
		}
		installed[meta.ID] = true
		return r.Manager.Register(meta, factory, body, selected[meta.ID])
	}
	native := example.Plugin{}
	if err = register(native.Manifest(), func() plugin.Plugin { return example.Plugin{} }); err != nil {
		return nil, err
	}
	if c.Plugins.TypeScript {
		budget, e := process.NewBudget(c.Plugins.MaxTSWorkers)
		if e != nil {
			return nil, e
		}
		specs, e := Discover(c.Plugins.Directory, c.Plugins.MaxPlugins)
		if e != nil {
			return nil, e
		}
		for _, spec := range specs {
			spec := spec
			meta := spec.Manifest
			factory := func() plugin.Plugin {
				worker, e := process.New(meta, process.Options{Node: c.Plugins.Node, Directory: spec.Directory, Entry: meta.Entry, HeapMB: c.Plugins.TSHeapMB, BotWXID: c.Qianxun.WXID, Budget: budget})
				if e != nil {
					panic("TS插件入口不可用")
				}
				return worker
			}
			if e = register(meta, factory); e != nil {
				return nil, e
			}
		}
	}
	for id := range selected {
		if !installed[id] {
			return nil, fmt.Errorf("启用列表中的插件未安装或运行时未开启: %s", id)
		}
	}
	return r, nil
}
func (r *Runtime) Start(ctx context.Context, events *qianxun.EventBus) error {
	if !r.enabled {
		return nil
	}
	if err := r.Manager.Start(ctx); err != nil {
		return err
	}
	if events != nil {
		var err error
		r.unsubscribe, err = events.Subscribe(qianxun.EventAny, r.Manager.Dispatch)
		return err
	}
	return nil
}
func (r *Runtime) Close(ctx context.Context) error {
	if r.unsubscribe != nil {
		r.unsubscribe()
	}
	if r.Manager != nil {
		return r.Manager.Close(ctx)
	}
	return nil
}
func (r *Runtime) ID() string { return "plugins" }
func (r *Runtime) Mount(router chi.Router) {
	router.Get("/plugins", func(w http.ResponseWriter, _ *http.Request) {
		status := []plugin.Status{}
		if r.Manager != nil {
			status = r.Manager.Status()
		}
		httpserver.JSON(w, 200, map[string]any{"enabled": r.enabled, "plugins": status})
	})
	for _, action := range []string{"enable", "disable"} {
		action := action
		router.Post("/plugins/{id}/"+action, func(w http.ResponseWriter, req *http.Request) {
			if req.Header.Get("Origin") != "" {
				httpserver.Error(w, req, 403, "origin_rejected", "插件控制不接受浏览器跨站请求")
				return
			}
			if r.Manager == nil {
				httpserver.Error(w, req, 409, "plugins_disabled", "插件系统未启用")
				return
			}
			select {
			case r.control <- struct{}{}:
				defer func() { <-r.control }()
			default:
				httpserver.Error(w, req, 503, "plugin_busy", "插件控制队列已满")
				return
			}
			ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
			defer cancel()
			var err error
			if action == "enable" {
				err = r.Manager.Enable(ctx, chi.URLParam(req, "id"))
			} else {
				err = r.Manager.Disable(ctx, chi.URLParam(req, "id"))
			}
			if err != nil {
				httpserver.Error(w, req, 409, "plugin_transition_failed", "插件状态转换失败，请核对依赖、预算及当前状态")
				return
			}
			httpserver.JSON(w, 200, map[string]bool{"ok": true})
		})
	}
}

type Discovered struct {
	Manifest  plugin.Manifest
	Directory string
}

// Discover兼容旧plugin.json，省略runtime视作typescript；限制读取大小并拒绝重复ID和入口越界。
func Discover(root string, max int) ([]Discovered, error) {
	if max < 1 || max > 1024 {
		return nil, errors.New("插件发现预算无效")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, errors.New("插件根目录无效")
	}
	dir, err := os.Open(absolute)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("无法读取插件目录")
	}
	defer dir.Close()
	entries, err := dir.ReadDir(max + 17)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, errors.New("插件目录枚举失败")
	}
	if len(entries) > max+16 {
		return nil, errors.New("插件根目录条目超过发现预算")
	}
	var out []Discovered
	ids := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" {
			continue
		}
		directory := filepath.Join(absolute, entry.Name())
		f, e := os.Open(filepath.Join(directory, "plugin.json"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, errors.New("无法读取插件清单")
		}
		b, e := io.ReadAll(io.LimitReader(f, 256<<10+1))
		_ = f.Close()
		if e != nil || len(b) > 256<<10 {
			return nil, errors.New("插件清单读取失败或过大")
		}
		var meta plugin.Manifest
		if json.Unmarshal(b, &meta) != nil {
			return nil, errors.New("插件清单JSON无效")
		}
		if meta.Runtime == "" {
			meta.Runtime = "typescript"
		}
		if meta.Runtime != "typescript" {
			return nil, errors.New("目录插件必须声明typescript运行时；Go使用编译注册")
		}
		if meta.ID != entry.Name() {
			return nil, errors.New("插件ID必须与目录名一致")
		}
		if meta.Entry == "" {
			for _, name := range []string{"index.ts", "index.js", "index.mjs", "index.cjs"} {
				if info, e := os.Stat(filepath.Join(directory, name)); e == nil && info.Mode().IsRegular() {
					meta.Entry = name
					break
				}
			}
		}
		if meta.Entry == "" {
			return nil, errors.New("插件缺少入口")
		}
		canonical, e := filepath.EvalSymlinks(directory)
		if e != nil {
			return nil, errors.New("插件目录无效")
		}
		resolved, e := filepath.EvalSymlinks(filepath.Join(canonical, meta.Entry))
		rel, relErr := filepath.Rel(canonical, resolved)
		if e != nil || relErr != nil || filepath.IsAbs(meta.Entry) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, errors.New("插件入口逃逸目录")
		}
		normalized, e := plugin.Normalize(meta)
		if e != nil {
			return nil, e
		}
		if ids[meta.ID] || len(out) >= max {
			return nil, errors.New("插件ID重复或数量超限")
		}
		ids[meta.ID] = true
		out = append(out, Discovered{Manifest: normalized, Directory: directory})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.ID < out[j].Manifest.ID })
	return out, nil
}
