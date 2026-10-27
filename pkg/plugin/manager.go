package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/surgar04/hadalbot/pkg/qianxun"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Options注入宿主服务和权限策略；MaxPlugins限制清单、路由及常驻实例的总量。
type Options struct {
	MaxPlugins     int
	Logger         *slog.Logger
	Caller         Caller
	Policy         Policy
	StartupTimeout time.Duration // StartupTimeout为全部插件启动及一次回滚分别设置总预算，默认30秒。
}
type entry struct {
	manifest                  Manifest
	factory                   Factory
	config                    json.RawMessage
	enabled                   bool
	mu                        sync.Mutex
	state                     State
	instance                  Plugin
	context                   *Context
	admitted, executing       chan struct{}
	idle                      chan struct{}
	inFlight                  int
	calls, failures, rejected uint64
}

func (e *entry) ready() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state == Ready {
		if h, ok := e.instance.(HealthChecker); ok && !h.Healthy() {
			e.state = Failed
			if e.context != nil {
				e.context.Scope.cancel()
			}
		}
	}
	return e.state == Ready
}

// Manager控制面串行变更，数据面只短暂读取快照；执行钩子时不持有全局锁。
// 同一插件默认串行；提高Concurrency后插件必须自行保护内部可变状态。
type Manager struct {
	options         Options
	control         chan struct{}
	mu              sync.RWMutex
	entries         map[string]*entry
	order           []*entry
	started, closed bool
	ctx             context.Context
	cancel          context.CancelFunc
}

func New(o Options) (*Manager, error) {
	if o.StartupTimeout == 0 {
		o.StartupTimeout = 30 * time.Second
	}
	if o.StartupTimeout < time.Millisecond || o.StartupTimeout > 5*time.Minute {
		return nil, errors.New("插件启动总预算无效")
	}
	if o.MaxPlugins == 0 {
		o.MaxPlugins = 128
	}
	if o.MaxPlugins < 1 || o.MaxPlugins > 1024 {
		return nil, errors.New("插件总量预算无效")
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{options: o, control: make(chan struct{}, 1), entries: map[string]*entry{}, ctx: ctx, cancel: cancel}, nil
}
func (m *Manager) lock(ctx context.Context) error {
	select {
	case m.control <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (m *Manager) unlock() { <-m.control }

// Register仅登记工厂，不加载资源、不启动进程。依赖图在Start前冻结，便于确定性审计。
func (m *Manager) Register(manifest Manifest, factory Factory, config json.RawMessage, enabled bool) error {
	normalized, err := Normalize(manifest)
	if err != nil {
		return err
	}
	if factory == nil {
		return errors.New("插件工厂不能为空")
	}
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	if len(config) > 256<<10 || !json.Valid(config) {
		return errors.New("插件配置无效或过大")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	if m.started {
		return errors.New("启动后不能修改插件注册表，请在重启宿主前登记工厂")
	}
	if len(m.entries) >= m.options.MaxPlugins {
		return ErrBusy
	}
	if _, ok := m.entries[normalized.ID]; ok {
		return errors.New("插件ID重复")
	}
	idle := make(chan struct{})
	close(idle)
	m.entries[normalized.ID] = &entry{manifest: normalized, factory: factory, config: append(json.RawMessage(nil), config...), enabled: enabled, state: Registered, idle: idle, admitted: make(chan struct{}, normalized.Limits.Concurrency+normalized.Limits.Queue), executing: make(chan struct{}, normalized.Limits.Concurrency)}
	return nil
}

// plan使用拓扑排序，优先级仅打破无依赖节点间的顺序；拒绝缺失依赖、版本冲突和循环。
func (m *Manager) plan() ([]*entry, error) {
	indegree := map[string]int{}
	children := map[string][]string{}
	for id, e := range m.entries {
		indegree[id] = len(e.manifest.Dependencies)
		for dep, rule := range e.manifest.Dependencies {
			d, ok := m.entries[dep]
			if !ok || !matches(d.manifest.Version, rule) {
				return nil, fmt.Errorf("插件依赖缺失或版本不匹配: %s", id)
			}
			children[dep] = append(children[dep], id)
		}
	}
	result := make([]*entry, 0, len(m.entries))
	for len(result) < len(m.entries) {
		var candidates []string
		for id, n := range indegree {
			if n == 0 {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) == 0 {
			return nil, errors.New("插件依赖存在循环")
		}
		sort.Slice(candidates, func(i, j int) bool {
			a, b := m.entries[candidates[i]].manifest, m.entries[candidates[j]].manifest
			if a.Priority != b.Priority {
				return a.Priority > b.Priority
			}
			return a.ID < b.ID
		})
		id := candidates[0]
		delete(indegree, id)
		result = append(result, m.entries[id])
		for _, child := range children[id] {
			indegree[child]--
		}
	}
	return result, nil
}

// Start依次Init、Ready，失败时逆序回滚本次已启动实例。Ready表示宿主依赖已装配，不表示微信已经登录。
func (m *Manager) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, m.options.StartupTimeout)
	defer cancel()
	if err := m.lock(ctx); err != nil {
		return err
	}
	defer m.unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	if m.started {
		m.mu.Unlock()
		return errors.New("插件系统已经启动")
	}
	order, err := m.plan()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	m.order = order
	m.started = true
	m.mu.Unlock()
	var started []*entry
	for _, e := range order {
		if !e.enabled {
			e.mu.Lock()
			e.state = Disabled
			e.mu.Unlock()
			continue
		}
		if err = m.start(ctx, e); err != nil {
			rollback, finish := context.WithTimeout(context.Background(), m.options.StartupTimeout)
			for i := len(started) - 1; i >= 0; i-- {
				cleanup, cancel := context.WithTimeout(rollback, started[i].manifest.Limits.lifecycleTimeout())
				_ = m.stop(cleanup, started[i])
				cancel()
			}
			finish()
			return err
		}
		started = append(started, e)
	}
	return nil
}
func (m *Manager) dependenciesReady(e *entry) bool {
	for id := range e.manifest.Dependencies {
		d := m.entries[id]
		ready := d.ready()
		if !ready {
			return false
		}
	}
	return true
}
func (m *Manager) start(ctx context.Context, e *entry) error {
	if !m.dependenciesReady(e) {
		return fmt.Errorf("插件依赖尚未就绪: %s", e.manifest.ID)
	}
	e.mu.Lock()
	if e.state == Ready {
		e.mu.Unlock()
		return nil
	}
	if e.inFlight != 0 || (e.context != nil && e.context.Scope.count() != 0) {
		e.mu.Unlock()
		return ErrBusy
	}
	e.state = Loading
	scope := newScope(m.ctx, e.manifest.Limits)
	caps := map[string]bool{}
	for _, c := range e.manifest.Capabilities {
		caps[c] = true
	}
	pc := &Context{ID: e.manifest.ID, Log: m.options.Logger.With("plugin", e.manifest.ID), Scope: scope, config: e.config, capabilities: caps, caller: m.options.Caller, policy: m.options.Policy}
	e.context = pc
	e.mu.Unlock()
	var instance Plugin
	err := guard(func() error {
		instance = e.factory()
		if instance == nil {
			return ErrHook
		}
		meta := instance.Manifest()
		if meta.ID != e.manifest.ID || meta.Version != e.manifest.Version {
			return ErrHook
		}
		return nil
	})
	e.mu.Lock()
	e.instance = instance
	e.mu.Unlock()
	if err == nil {
		if h, ok := instance.(Initializer); ok {
			err = lifecycle(ctx, scope, e.manifest.Limits, func(c context.Context) error { return h.Init(c, pc) })
		}
	}
	if err == nil {
		e.mu.Lock()
		e.state = Initialized
		e.mu.Unlock()
		if h, ok := instance.(Readier); ok {
			err = lifecycle(ctx, scope, e.manifest.Limits, func(c context.Context) error { return h.Ready(c, pc) })
		}
	}
	if err != nil {
		e.mu.Lock()
		e.state = Failed
		e.failures++
		e.mu.Unlock()
		cleanup, cancel := context.WithTimeout(context.Background(), e.manifest.Limits.lifecycleTimeout())
		defer cancel()
		_ = m.stop(cleanup, e)
		e.mu.Lock()
		e.state = Failed
		e.mu.Unlock()
		return fmt.Errorf("插件启动失败: %s: %w", e.manifest.ID, err)
	}
	e.mu.Lock()
	e.state = Ready
	e.enabled = true
	e.mu.Unlock()
	return nil
}
func lifecycle(parent context.Context, s *Scope, l Limits, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, l.lifecycleTimeout())
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	if err := ctx.Err(); err != nil {
		return err
	}
	err := guard(func() error { return fn(ctx) })
	if err == nil {
		err = ctx.Err()
	}
	return err
}

// Enable重新创建实例；Disable不自动停用依赖它的插件，以免隐式扩大操作范围。
func (m *Manager) Enable(ctx context.Context, id string) error {
	if err := m.lock(ctx); err != nil {
		return err
	}
	defer m.unlock()
	e, err := m.lookup(id)
	if err != nil {
		return err
	}
	_ = e.ready()
	e.mu.Lock()
	if e.inFlight != 0 {
		e.mu.Unlock()
		return ErrBusy
	}
	needsCleanup := e.context != nil && e.state != Ready
	e.mu.Unlock()
	if needsCleanup {
		if err = m.stop(ctx, e); err != nil {
			return err
		}
	}
	return m.start(ctx, e)
}
func (m *Manager) Disable(ctx context.Context, id string) error {
	if err := m.lock(ctx); err != nil {
		return err
	}
	defer m.unlock()
	e, err := m.lookup(id)
	if err != nil {
		return err
	}
	for _, d := range m.snapshot() {
		if _, ok := d.manifest.Dependencies[id]; ok {
			d.mu.Lock()
			active := d.state == Ready
			d.mu.Unlock()
			if active {
				return fmt.Errorf("仍有就绪插件依赖该插件: %s", d.manifest.ID)
			}
		}
	}
	return m.stop(ctx, e)
}
func (m *Manager) lookup(id string) (*entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, ErrClosed
	}
	if !m.started {
		return nil, ErrNotReady
	}
	e := m.entries[id]
	if e == nil {
		return nil, errors.New("插件未注册")
	}
	return e, nil
}
func (m *Manager) stop(ctx context.Context, e *entry) error {
	e.mu.Lock()
	if e.context == nil {
		e.state = Stopped
		e.enabled = false
		e.mu.Unlock()
		return nil
	}
	e.state = Draining
	idle := e.idle
	pc, instance := e.context, e.instance
	e.mu.Unlock()
	select {
	case <-idle:
	case <-ctx.Done():
		pc.Scope.cancel()
		e.mu.Lock()
		e.state = Failed
		e.mu.Unlock()
		return ctx.Err()
	}
	// 先停止后台任务，再调用Dispose，最后清理登记资源；各阶段使用调用方剩余预算。
	pc.Scope.cancel()
	err := pc.Scope.quiesce(ctx)
	if err != nil {
		e.mu.Lock()
		e.state = Failed
		e.mu.Unlock()
		return err
	}
	if d, ok := instance.(Disposer); ok {
		cleanup, cancel := context.WithTimeout(ctx, e.manifest.Limits.lifecycleTimeout())
		err = errors.Join(err, guard(func() error { return d.Dispose(cleanup) }))
		cancel()
	}
	err = errors.Join(err, pc.Scope.close(ctx))
	e.mu.Lock()
	if err == nil {
		e.state = Stopped
		e.instance = nil
		e.context = nil
	} else {
		e.state = Failed
	}
	e.enabled = false
	e.mu.Unlock()
	return err
}

// Close停止新分发，按反向依赖顺序排空；多次调用可继续清理之前超时的实例。
func (m *Manager) Close(ctx context.Context) error {
	if err := m.lock(ctx); err != nil {
		return err
	}
	defer m.unlock()
	m.mu.Lock()
	m.closed = true
	order := append([]*entry(nil), m.order...)
	m.mu.Unlock()
	var result error
	for i := len(order) - 1; i >= 0; i-- {
		result = errors.Join(result, m.stop(ctx, order[i]))
	}
	m.cancel()
	return result
}
func (m *Manager) snapshot() []*entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil
	}
	return append([]*entry(nil), m.order...)
}
func (m *Manager) Status() []Status {
	m.mu.RLock()
	entries := make([]*entry, 0, len(m.entries))
	for _, e := range m.entries {
		entries = append(entries, e)
	}
	m.mu.RUnlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].manifest.ID < entries[j].manifest.ID })
	out := make([]Status, 0, len(entries))
	for _, e := range entries {
		ready := e.ready()
		e.mu.Lock()
		s := Status{ID: e.manifest.ID, Version: e.manifest.Version, Runtime: e.manifest.Runtime, State: e.state, InFlight: e.inFlight, Calls: e.calls, Failures: e.failures, Rejected: e.rejected}
		if e.context != nil {
			s.Tasks = e.context.Scope.count()
			s.TaskFailures = e.context.Scope.failureCount()
		}
		e.mu.Unlock()
		if ready && !m.dependenciesReady(e) {
			s.State = Blocked
		}
		out = append(out, s)
	}
	return out
}

// invoke先抢占总名额，再等待执行位；队列满直接背压，不创建等待goroutine。
func (m *Manager) invoke(parent context.Context, e *entry, fn func(context.Context, Plugin, *Context) error) error {
	e.mu.Lock()
	if e.state != Ready {
		e.mu.Unlock()
		return nil
	}
	select {
	case e.admitted <- struct{}{}:
	default:
		e.rejected++
		e.mu.Unlock()
		return ErrBusy
	}
	if e.inFlight == 0 {
		e.idle = make(chan struct{})
	}
	e.inFlight++
	pc, p := e.context, e.instance
	e.mu.Unlock()
	defer func() {
		<-e.admitted
		e.mu.Lock()
		e.inFlight--
		if e.inFlight == 0 {
			close(e.idle)
		}
		e.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(parent, e.manifest.Limits.hookTimeout())
	defer cancel()
	stop := context.AfterFunc(pc.Scope.ctx, cancel)
	defer stop()
	select {
	case e.executing <- struct{}{}:
		defer func() { <-e.executing }()
	case <-ctx.Done():
		return ctx.Err()
	}
	e.mu.Lock()
	ready := e.state == Ready
	if ready {
		e.calls++
	}
	e.mu.Unlock()
	if !ready {
		return ErrNotReady
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err := guard(func() error { return fn(ctx, p, pc) })
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		e.mu.Lock()
		e.failures++
		if h, ok := p.(HealthChecker); ok && !h.Healthy() {
			e.state = Failed
			pc.Scope.cancel()
		}
		e.mu.Unlock()
	}
	return err
}

// Dispatch遵循旧协议：全部观察者先执行，之后onEvent拦截链，最后onMessage拦截链。
// 观察者失败不会阻断其他插件；错误仍返回给事件总线计数。任何队列满都会显式返回错误。
func (m *Manager) Dispatch(ctx context.Context, event qianxun.Event) error {
	m.mu.RLock()
	closed, started := m.closed, m.started
	m.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	if !started {
		return ErrNotReady
	}
	ctx = WithAccount(ctx, event.WXID)
	order := m.snapshot()
	message, err := ParseMessage(event)
	if err != nil {
		return err
	}
	allowed := make([]*entry, 0, len(order))
	for _, e := range order {
		if e.ready() && m.dependenciesReady(e) && (m.options.Policy == nil || m.options.Policy.AllowEvent(ctx, e.manifest.ID, event, message)) {
			allowed = append(allowed, e)
		}
	}
	var result error
	if message != nil {
		for _, e := range allowed {
			err = m.invoke(ctx, e, func(c context.Context, p Plugin, pc *Context) error {
				if h, ok := p.(Observer); ok {
					return h.ObserveMessage(c, *message, pc)
				}
				return nil
			})
			result = errors.Join(result, err)
		}
	}
	for _, e := range allowed {
		handled := false
		err = m.invoke(ctx, e, func(c context.Context, p Plugin, pc *Context) error {
			if h, ok := p.(EventHandler); ok {
				var callErr error
				handled, callErr = h.OnEvent(c, event, pc)
				return callErr
			}
			return nil
		})
		result = errors.Join(result, err)
		if err != nil || handled {
			return result
		}
	}
	if message != nil {
		for _, e := range allowed {
			handled := false
			err = m.invoke(ctx, e, func(c context.Context, p Plugin, pc *Context) error {
				if h, ok := p.(MessageHandler); ok {
					var callErr error
					handled, callErr = h.OnMessage(c, *message, pc)
					return callErr
				}
				return nil
			})
			result = errors.Join(result, err)
			if err != nil || handled {
				return result
			}
		}
	}
	return result
}

// ParseMessage保持旧ParsedMessage字段及字符串消息ID，同时兼容千寻直连模式。
func ParseMessage(e qianxun.Event) (*Message, error) {
	if e.ID != qianxun.GroupMessage && e.ID != qianxun.PrivateMessage {
		return nil, nil
	}
	data, err := qianxun.PayloadAs[qianxun.MessageData](e)
	if err != nil {
		return nil, errors.New("插件消息数据无效")
	}
	typ, _ := strconv.Atoi(data.MsgType.String())
	m := &Message{Source: "private", Content: data.Msg, SenderWXID: data.FromWXID, TargetWXID: e.WXID, AtWXIDList: append([]string{}, data.AtWXIDList...), MsgID: data.MsgID.String(), MsgType: typ, Raw: e.Payload, Event: e.Raw}
	if e.ID == qianxun.GroupMessage {
		m.Source = "group"
		m.GroupWXID = data.FromWXID
		m.SenderWXID = data.FinalFromWXID
	}
	return m, nil
}
