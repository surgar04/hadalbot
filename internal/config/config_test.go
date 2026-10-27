package config_test

import (
	"github.com/surgar04/hadalbot/internal/config"
	"github.com/surgar04/hadalbot/internal/testutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndEnvironmentOverride(t *testing.T) {
	testutil.CleanEnvironment(t)
	t.Setenv("HADALBOT_HTTP_ADDRESS", "127.0.0.1:19888")
	c, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTP.Address != "127.0.0.1:19888" || c.Database.Enabled || c.Redis.Enabled {
		t.Fatalf("配置不符合预期: address=%s", c.HTTP.Address)
	}
}
func TestYAMLPrecedenceAndUnknownFields(t *testing.T) {
	testutil.CleanEnvironment(t)
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("http:\n  address: 127.0.0.1:19999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HADALBOT_HTTP_ADDRESS", "127.0.0.1:19888")
	c, err := config.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTP.Address != "127.0.0.1:19888" {
		t.Fatal("环境变量未覆盖文件")
	}
	if err := os.WriteFile(file, []byte("unknown_option: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = config.Load(file); err == nil {
		t.Fatal("不应接受未知配置字段")
	}
}
func TestProductionAndExposedListenerRequireKey(t *testing.T) {
	c := testutil.Config(t)
	for _, address := range []string{"0.0.0.0:18878", "[::]:18878"} {
		c.HTTP.Address = address
		if c.Validate() == nil {
			t.Fatalf("不应接受未授权公网监听: %s", address)
		}
	}
	c.HTTP.Address = "127.0.0.1:18878"
	c.Environment = "production"
	if c.Validate() == nil {
		t.Fatal("生产环境必须设置密钥")
	}
	c.HTTP.APIKey = "12345678901234567890123456789012"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestMissingExplicitConfigFails(t *testing.T) {
	testutil.CleanEnvironment(t)
	if _, err := config.Load(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("显式指定不存在的配置必须失败")
	}
}
