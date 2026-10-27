// Package example 演示原生Go插件；默认只登记不启用，不会自行发消息。
package example

import (
	"context"
	"encoding/json"
	"github.com/surgar04/hadalbot/pkg/plugin"
)

type Plugin struct{}

func (Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{ID: "example-go", Name: "Go插件示例", Version: "1.0.0", Capabilities: []string{"qx.sendText"}, Description: "收到go ping时回复go pong"}
}
func (Plugin) OnMessage(ctx context.Context, m plugin.Message, c *plugin.Context) (bool, error) {
	if m.Content != "go ping" {
		return false, nil
	}
	target := m.SenderWXID
	if m.Source == "group" {
		target = m.GroupWXID
	}
	args, _ := json.Marshal(map[string]string{"wxid": target, "msg": "go pong"})
	_, err := c.Call(ctx, "qx.sendText", args)
	return true, err
}
