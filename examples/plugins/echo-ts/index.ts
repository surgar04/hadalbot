// 使用旧插件对象契约，不依赖旧机器人进程。能力还需由Go宿主配置显式授予。
import type { Plugin } from '../../../sdks/typescript/index.js';
const plugin: Plugin = {
 name:'echo-ts',
 init(ctx) { ctx.log.info('TS兼容示例已初始化'); },
 async onMessage(message,ctx) {
  if(message.content!=='ts ping')return false;
  const target=message.source==='group'?message.groupWxid:message.senderWxid;
  if(!target)return false;
  await ctx.sendText(target,'ts pong');
  return true;
 },
 dispose() { /* 清理由插件直接创建的定时器/套接字；worker退出是最后的故障回收手段。 */ },
};
export default plugin;
