// Package testutil 隔离测试配置，防止开发终端里的生产环境变量影响单元测试。
package testutil

import (
	"os"
	"strings"
	"testing"

	"github.com/surgar04/hadalbot/internal/config"
)

func CleanEnvironment(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "HADALBOT_") && !strings.HasPrefix(name, "HADALBOT_TEST_") {
			t.Setenv(name, "")
		}
	}
}
func Config(t *testing.T) config.Config {
	t.Helper()
	CleanEnvironment(t)
	c, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return c
}
