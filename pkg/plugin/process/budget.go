package process

import (
	"errors"
	"github.com/surgar04/hadalbot/pkg/plugin"
)

// Budget限制所有TS worker的同时存活数；等待释放前不启动新进程。
// 堆上限另由HeapMB控制，二者不能当作操作系统RSS硬限制。
type Budget struct{ slots chan struct{} }

func NewBudget(max int) (*Budget, error) {
	if max < 1 || max > 16 {
		return nil, errors.New("TS进程总量预算必须为1至16")
	}
	return &Budget{slots: make(chan struct{}, max)}, nil
}
func (b *Budget) acquire() error {
	select {
	case b.slots <- struct{}{}:
		return nil
	default:
		return plugin.ErrBusy
	}
}

var defaultBudget, _ = NewBudget(4)
