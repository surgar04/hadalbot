package plugin

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Scope统一拥有插件任务和清理动作。后台任务只有通过Go/Every登记才受宿主管理。
// Go原生代码能自行启动goroutine，因此这是协作契约，并非针对恶意代码的沙箱。
type Scope struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	mu                    sync.Mutex
	closed                bool
	tasks                 int
	failures              uint64
	maxTasks, maxCleanups int
	cleanups              []func(context.Context) error
	idle                  chan struct{}
}

func newScope(parent context.Context, l Limits) *Scope {
	ctx, cancel := context.WithCancel(parent)
	idle := make(chan struct{})
	close(idle)
	return &Scope{ctx: ctx, cancel: cancel, maxTasks: l.MaxTasks, maxCleanups: l.MaxCleanups, idle: idle}
}
func (s *Scope) Context() context.Context { return s.ctx }

// Go创建受预算控制的后台任务；返回错误时不会启动goroutine。任务panic会被隔离，不输出敏感panic值。
func (s *Scope) Go(task func(context.Context)) error {
	if task == nil {
		return errors.New("插件任务不能为空")
	}
	s.mu.Lock()
	if s.closed || s.ctx.Err() != nil {
		s.mu.Unlock()
		return ErrNotReady
	}
	if s.tasks >= s.maxTasks {
		s.mu.Unlock()
		return ErrBusy
	}
	if s.tasks == 0 {
		s.idle = make(chan struct{})
	}
	s.tasks++
	s.mu.Unlock()
	go func() {
		defer func() {
			panicked := recover() != nil
			s.mu.Lock()
			if panicked {
				s.failures++
			}
			s.tasks--
			if s.tasks == 0 {
				close(s.idle)
			}
			s.mu.Unlock()
		}()
		task(s.ctx)
	}()
	return nil
}

// Every使用一个受管任务串行执行定时器，不为每次tick再开goroutine；慢任务不会堆积。
func (s *Scope) Every(interval time.Duration, task func(context.Context)) error {
	if interval < 10*time.Millisecond || task == nil {
		return errors.New("定时器间隔或任务无效")
	}
	return s.Go(func(ctx context.Context) {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				task(ctx)
			}
		}
	})
}
func (s *Scope) Defer(cleanup func(context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cleanup == nil {
		return errors.New("清理函数不能为空")
	}
	if s.closed {
		return ErrNotReady
	}
	if len(s.cleanups) >= s.maxCleanups {
		return ErrBusy
	}
	s.cleanups = append(s.cleanups, cleanup)
	return nil
}
func (s *Scope) count() int           { s.mu.Lock(); defer s.mu.Unlock(); return s.tasks }
func (s *Scope) failureCount() uint64 { s.mu.Lock(); defer s.mu.Unlock(); return s.failures }

// close先取消并等待后台任务，再逆序释放资源；清理函数同样必须遵守context。
func (s *Scope) close(ctx context.Context) error {
	if err := s.quiesce(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	cleanup := s.cleanups
	s.cleanups = nil
	s.mu.Unlock()
	var result error
	for i := len(cleanup) - 1; i >= 0; i-- {
		err := guard(func() error { return cleanup[i](ctx) })
		result = errors.Join(result, err)
	}
	return result
}
func (s *Scope) quiesce(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	idle := s.idle
	s.mu.Unlock()
	select {
	case <-idle:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

type hookError struct{ cause error }

func (e *hookError) Error() string        { return ErrHook.Error() }
func (e *hookError) Unwrap() error        { return e.cause }
func (e *hookError) Is(target error) bool { return target == ErrHook }
func guard(fn func() error) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrHook
		}
	}()
	if e := fn(); e != nil {
		return &hookError{cause: e}
	}
	return nil
}
