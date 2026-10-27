package storage

import (
	"context"
	"github.com/surgar04/hadalbot/internal/testutil"
	"testing"
)

func TestOptionalDependenciesDisabled(t *testing.T) {
	c := testutil.Config(t)
	s, err := Open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, status := range s.Check(context.Background()) {
		if status != "disabled" {
			t.Fatal(status)
		}
	}
}
