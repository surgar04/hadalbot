// Package modules 定义编译期业务模块边界，避免跨模块读取私有存储。
package modules

import (
	"fmt"
	"sort"

	"github.com/go-chi/chi/v5"
)

type Module interface {
	ID() string
	Mount(chi.Router)
}
type Registry struct{ modules map[string]Module }

func NewRegistry() *Registry { return &Registry{modules: make(map[string]Module)} }
func (r *Registry) Register(m Module) error {
	if m == nil || m.ID() == "" {
		return fmt.Errorf("模块必须提供 ID")
	}
	if _, ok := r.modules[m.ID()]; ok {
		return fmt.Errorf("模块重复: %s", m.ID())
	}
	r.modules[m.ID()] = m
	return nil
}
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.modules))
	for id := range r.modules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func (r *Registry) Mount(router chi.Router) {
	for _, id := range r.IDs() {
		r.modules[id].Mount(router)
	}
}
