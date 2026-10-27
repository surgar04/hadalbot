package modules

import (
	"github.com/go-chi/chi/v5"
	"testing"
)

type module string

func (m module) ID() string     { return string(m) }
func (module) Mount(chi.Router) {}
func TestRejectDuplicateModuleAndStableOrder(t *testing.T) {
	r := NewRegistry()
	for _, id := range []string{"mihoyo", "passport"} {
		if err := r.Register(module(id)); err != nil {
			t.Fatal(err)
		}
	}
	if r.Register(module("mihoyo")) == nil {
		t.Fatal("未拒绝重复模块")
	}
	if r.Register(nil) == nil {
		t.Fatal("未拒绝空模块")
	}
	ids := r.IDs()
	if len(ids) != 2 || ids[0] != "mihoyo" || ids[1] != "passport" {
		t.Fatal(ids)
	}
}
