/**
 * @module plugins/example
 * 示例插件，演示最小 Plugin 实现。
 *
 * 响应私聊或群聊中的 `ping` 命令并回复 `pong`。
 */
import type { Plugin } from '@/types/plugin.js';
import type { ParsedMessage } from '@/types/message.js';

const examplePlugin: Plugin = {
  name: 'example',
  description: '示例插件 - 响应 ping 命令',
  enabled: true,

  init(ctx) {
    ctx.log.info('example 插件已初始化');
  },

  async onMessage(message: ParsedMessage, ctx) {
    const content = message.content.trim();
    if (content !== 'ping') {
      return false;
    }

    const target = message.source === 'group' ? message.groupWxid : message.senderWxid;
    if (!target) {
      return false;
    }

    await ctx.sendText(target, 'pong');
    ctx.log.info('已响应 ping 命令', { target, source: message.source });
    return true;
  },
};

export default examplePlugin;
