package qianxun

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

var ErrEventBusy = errors.New("千寻事件分片队列已满")
var ErrEventBusClosed = errors.New("千寻事件分发器已关闭")
var ErrNoSubscribers = errors.New("当前事件没有已注册处理器")

// Handler 按分片同步调用，必须尊重ctx取消；不得阻塞等待同一分片中稍后的事件。
// 业务错误由OnError接收，不自动重试可能发送消息、收款等非幂等行为。
type Handler func(context.Context, Event) error

// BusOptions 为事件系统分配固定资源预算；未启用时不创建实例，也不启动goroutine。
type BusOptions struct {
	Shards         int
	QueuePerShard  int
	HandlerTimeout time.Duration
	DedupCapacity  int
	DedupTTL       time.Duration
	MaxHandlers    int
	// MaxBufferedBytes 同时限制排队和处理中事件的原文/业务体字节，避免大消息撑爆内存。
	MaxBufferedBytes int64
	// OnError 只能做短时、非阻塞的错误记录，不能记录整个事件或原始正文。
	OnError func(EventID, error)
}
type subscription struct {
	id      EventID
	handler Handler
}
type dedupRecord struct {
	key   [32]byte
	until time.Time
}

// EventStats 表示传输层处理结果，不等于业务数据已经持久化。
type EventStats struct {
	Accepted      uint64 `json:"accepted"`
	Duplicates    uint64 `json:"duplicates"`
	Processed     uint64 `json:"processed"`
	Failed        uint64 `json:"failed"`
	Rejected      uint64 `json:"rejected"`
	Queued        int    `json:"queued"`
	BufferedBytes int64  `json:"buffered_bytes"`
}

// EventBus 提供固定分片、同会话顺序和有限去重窗口。
// 它是进程内队列：HTTP应答只表示成功接收入队，不提供进程崩溃后的持久化交付保证。
type EventBus struct {
	options                                           BusOptions
	queues                                            []chan Event
	ctx                                               context.Context
	cancel                                            context.CancelFunc
	done                                              chan struct{}
	mu                                                sync.RWMutex
	closing                                           bool
	handlers                                          []subscription
	dedup                                             map[[32]byte]time.Time
	ring                                              []dedupRecord
	cursor                                            int
	accepted, duplicates, processed, failed, rejected atomic.Uint64
	buffered                                          atomic.Int64
}

// NewEventBus 创建固定数量的worker，不按群、账号或消息无限创建goroutine。
func NewEventBus(o BusOptions) (*EventBus, error) {
	if o.Shards == 0 {
		o.Shards = 4
	}
	if o.QueuePerShard == 0 {
		o.QueuePerShard = 16
	}
	if o.HandlerTimeout == 0 {
		o.HandlerTimeout = 5 * time.Second
	}
	if o.DedupCapacity == 0 {
		o.DedupCapacity = 4096
	}
	if o.DedupTTL == 0 {
		o.DedupTTL = 5 * time.Minute
	}
	if o.MaxHandlers == 0 {
		o.MaxHandlers = 128
	}
	if o.MaxBufferedBytes == 0 {
		o.MaxBufferedBytes = 32 << 20
	}
	if o.Shards < 1 || o.Shards > 64 || o.QueuePerShard < 1 || o.QueuePerShard > 4096 || o.HandlerTimeout < 0 || o.DedupCapacity < 1 || o.DedupCapacity > 65536 || o.DedupTTL < 0 || o.MaxHandlers < 1 || o.MaxHandlers > 4096 || o.MaxBufferedBytes < 1 || o.MaxBufferedBytes > 256<<20 {
		return nil, errors.New("事件分发资源预算无效")
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &EventBus{options: o, ctx: ctx, cancel: cancel, done: make(chan struct{}), dedup: make(map[[32]byte]time.Time, o.DedupCapacity), ring: make([]dedupRecord, o.DedupCapacity), queues: make([]chan Event, o.Shards)}
	var workers sync.WaitGroup
	for i := range b.queues {
		b.queues[i] = make(chan Event, o.QueuePerShard)
		workers.Add(1)
		go func(queue <-chan Event) {
			defer workers.Done()
			for e := range queue {
				if b.ctx.Err() != nil {
					b.failed.Add(1)
					b.buffered.Add(-e.retainedBytes)
					continue
				}
				b.dispatch(e)
				b.buffered.Add(-e.retainedBytes)
			}
		}(b.queues[i])
	}
	go func() { workers.Wait(); close(b.done) }()
	return b, nil
}

// Subscribe 注册处理器并返回取消订阅函数。最多注册MaxHandlers个处理器，重复业务应自行避免重复注册。
func (b *EventBus) Subscribe(id EventID, h Handler) (func(), error) {
	if h == nil {
		return nil, errors.New("事件处理器不能为空")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closing {
		return nil, ErrEventBusClosed
	}
	index := -1
	for i, s := range b.handlers {
		if s.handler == nil {
			index = i
			break
		}
	}
	if index < 0 {
		if len(b.handlers) >= b.options.MaxHandlers {
			return nil, errors.New("事件处理器数量已达上限")
		}
		index = len(b.handlers)
		b.handlers = append(b.handlers, subscription{})
	}
	b.handlers[index] = subscription{id: id, handler: h}
	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if index < len(b.handlers) {
				b.handlers[index].handler = nil
			}
		})
	}, nil
}

// TryPublish 只尝试一次入队，满载立即返回ErrEventBusy；不会静默丢弃并回复成功。
// 返回duplicate=true表示相同消息已被接收，无须再次执行；仅消息事件按账号/会话/ID去重。
// 发布成功即转交字节切片所有权，调用方和处理器均不得修改Raw/Payload；需要修改时请先复制。
func (b *EventBus) TryPublish(e Event) (duplicate bool, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closing {
		return false, ErrEventBusClosed
	}
	subscribed := false
	for _, s := range b.handlers {
		if s.handler != nil && (s.id == EventAny || s.id == e.ID) {
			subscribed = true
			break
		}
	}
	if !subscribed {
		b.rejected.Add(1)
		return false, ErrNoSubscribers
	}
	now := time.Now()
	key, hasKey := messageKey(e)
	if hasKey {
		if until, ok := b.dedup[key]; ok && now.Before(until) {
			b.duplicates.Add(1)
			return true, nil
		}
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(e.WXID))
	_, _ = h.Write([]byte{0})
	if m, ok := message(e); ok {
		_, _ = h.Write([]byte(m.FromWXID))
	}
	queue := b.queues[h.Sum64()%uint64(len(b.queues))]
	retained := int64(len(e.Raw) + len(e.Payload) + len(e.WXID) + len(e.Kind) + len(e.Description) + len(e.Flag) + len(e.Port) + len(e.PID) + len(e.Timestamp) + 256)
	if retained > b.options.MaxBufferedBytes-b.buffered.Load() {
		b.rejected.Add(1)
		return false, ErrEventBusy
	}
	e.retainedBytes = retained
	b.buffered.Add(retained)
	select {
	case queue <- e:
	default:
		b.buffered.Add(-retained)
		b.rejected.Add(1)
		return false, ErrEventBusy
	}
	if hasKey {
		old := b.ring[b.cursor]
		if expiry, exists := b.dedup[old.key]; exists && expiry.Equal(old.until) {
			delete(b.dedup, old.key)
		}
		until := now.Add(b.options.DedupTTL)
		b.dedup[key] = until
		b.ring[b.cursor] = dedupRecord{key: key, until: until}
		b.cursor = (b.cursor + 1) % len(b.ring)
	}
	b.accepted.Add(1)
	return false, nil
}
func message(e Event) (MessageData, bool) {
	if e.ID != GroupMessage && e.ID != PrivateMessage && e.ID != SelfMessage {
		return MessageData{}, false
	}
	if e.messageID != "" {
		return MessageData{MsgID: e.messageID, FromWXID: e.conversation}, true
	}
	var m MessageData
	if json.Unmarshal(e.Payload, &m) != nil || m.MsgID == "" {
		return m, false
	}
	return m, true
}
func messageKey(e Event) ([32]byte, bool) {
	m, ok := message(e)
	if !ok {
		return [32]byte{}, false
	}
	return sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00%s", e.ID, e.WXID, m.FromWXID, m.MsgID))), true
}
func (b *EventBus) dispatch(e Event) {
	b.mu.RLock()
	handlers := make([]Handler, 0, len(b.handlers))
	for _, s := range b.handlers {
		if s.handler != nil && (s.id == EventAny || s.id == e.ID) {
			handlers = append(handlers, s.handler)
		}
	}
	b.mu.RUnlock()
	failed := false
	for _, h := range handlers {
		ctx, cancel := context.WithTimeout(b.ctx, b.options.HandlerTimeout)
		err := invoke(ctx, h, e)
		cancel()
		if err != nil {
			failed = true
			if b.options.OnError != nil {
				func() { defer func() { _ = recover() }(); b.options.OnError(e.ID, err) }()
			}
		}
	}
	if failed {
		b.failed.Add(1)
	} else {
		b.processed.Add(1)
	}
}
func invoke(ctx context.Context, h Handler, e Event) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("事件处理器发生panic")
		}
	}()
	if err = ctx.Err(); err != nil {
		return err
	}
	err = h(ctx, e)
	if err == nil {
		err = ctx.Err()
	}
	return err
}

// Close 停止接收并等待队列处理完毕；ctx超时后取消剩余任务。
// Go无法强制终止忽略ctx的处理器，因此业务必须遵守Handler的取消约定。
func (b *EventBus) Close(ctx context.Context) error {
	b.mu.Lock()
	if !b.closing {
		b.closing = true
		for _, q := range b.queues {
			close(q)
		}
	}
	b.mu.Unlock()
	select {
	case <-b.done:
		b.cancel()
		return nil
	case <-ctx.Done():
		b.cancel()
		return ctx.Err()
	}
}

// Snapshot 返回有界队列统计，不输出消息内容或账号标识。
func (b *EventBus) Snapshot() EventStats {
	out := EventStats{Accepted: b.accepted.Load(), Duplicates: b.duplicates.Load(), Processed: b.processed.Load(), Failed: b.failed.Load(), Rejected: b.rejected.Load()}
	for _, q := range b.queues {
		out.Queued += len(q)
	}
	out.BufferedBytes = b.buffered.Load()
	return out
}
