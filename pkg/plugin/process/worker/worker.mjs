// 双向逐行JSON协议。stdout只写协议帧，插件console输出丢弃，避免破坏帧边界或泄露凭证。
import { pathToFileURL } from 'node:url';
import { AsyncLocalStorage } from 'node:async_hooks';
import { parseMessageEvent } from './message.mjs';
const local = new AsyncLocalStorage();
const [entry, expectedID, maximum] = process.argv.slice(2);
const maxBytes = Number(maximum), pending = new Map();
let sequence = 0, instance, context, initialized = false, draining = false, active = 0;
const write = process.stdout.write.bind(process.stdout);
const discard = () => {};
console.log = console.info = console.warn = console.error = console.debug = discard;
function send(frame) {
  const line = JSON.stringify(frame) + '\n';
  if (Buffer.byteLength(line) > maxBytes) process.exit(72);
  // stdout缓冲预算固定；慢宿主触发退出，由宿主明确记录worker失败。
  if (process.stdout.writableLength + Buffer.byteLength(line) > maxBytes * 2) process.exit(73);
  write(line);
}
function call(method, args) {
  if (pending.size >= 32 || draining) return Promise.reject(new Error('HOST_BUSY'));
  const id = `n:${++sequence}`;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => { pending.delete(id); reject(new Error('HOST_TIMEOUT')); }, 30000);
    pending.set(id, { resolve, reject, timer });
    send({ id, method: 'host.call', params: { method, args: args ?? {}, parent: local.getStore()?.id } });
  });
}
function facade(service) {
  return new Proxy(Object.create(null), { get(_target, method) {
    if (method === 'then') return undefined;
    if(service === 'qx' && method === 'request') return (type, data) => call(`qx.${type}`,data);
    return args => call(`${service}.${String(method)}`, args);
  } });
}
// 同步权限检查读取当前钩子的宿主快照；没有适配器时明确报错，不能用Promise伪装布尔值。
function permissions() {
  const snapshot=local.getStore()?.permissions;
  if(!snapshot)throw Error('UNSUPPORTED_SYNC_PERMISSION_ADAPTER');
  const level=(wxid,group)=>snapshot.rootUsers?.includes(wxid)?'root':snapshot.superUsers?.includes(wxid)?'super':snapshot.groupDelegates?.[group]?.includes(wxid)?'delegate':'none';
  return {getLevel:level,isRoot:wxid=>level(wxid)==='root',isSuper:wxid=>['root','super'].includes(level(wxid)),
    isSuperOnly:wxid=>level(wxid)==='super',isDelegate:(wxid,group)=>level(wxid,group)==='delegate',
    isGroupBotEnabled:group=>!snapshot.disabledGroups?.includes(group),
    check:({wxid,groupWxid})=>({level:level(wxid,groupWxid),hasSpecial:level(wxid,groupWxid)!=='none'}),
    hasSpecialPermission:(wxid,group)=>level(wxid,group)!=='none'};
}
let capabilities=new Set();
async function request(frame) {
  try {
    let result;
    if (frame.method === 'init') {
      if (initialized || frame.params?.protocol !== 1) throw new Error('PROTOCOL_MISMATCH');
      const major = Number(process.versions.node.split('.')[0]);
      if (major < 24) throw new Error('NODE24_REQUIRED');
      const module = await import(pathToFileURL(entry).href);
      instance = module.default ?? module.plugin;
      if (!instance || instance.name !== expectedID || instance.enabled === false) throw new Error('INVALID_PLUGIN_EXPORT');
      const api = facade('qx');
      capabilities=new Set(frame.params.capabilities??[]);
      const invoke = (service, method, ...args) => call(`${service}.${method}`, args.length === 1 ? args[0] : args);
      context = Object.freeze({ api, get botWxid(){return local.getStore()?.wxid ?? frame.params.botWxid;},
        apis: Object.freeze({ facade, get:facade, getDynamic:facade, call:invoke,
          callDynamic:(service,method,args=[])=>invoke(service,method,...args) }),
        get perms(){return permissions();},
        sendText: (wxid, msg) => api.sendText({ wxid, msg }),
        config: Object.freeze(frame.params.config ?? {}),
        log: Object.freeze(Object.fromEntries(['info','warn','error'].map(level=>[level,message=>{void call('log.write',{level,message:String(message).slice(0,2048)}).catch(discard);}]))),
      });
      initialized = true;
      if(capabilities.has('permissions.snapshot'))local.getStore().permissions=await call('permissions.snapshot',{});
      await instance.init?.(context);
      result = { hooks: ['onReady','observeMessage','onEvent','onMessage','dispose'].filter(hook=>typeof instance[hook]==='function') };
    } else {
      if (!initialized) throw new Error('NOT_INITIALIZED');
      if(frame.method!=='dispose'&&capabilities.has('permissions.snapshot'))local.getStore().permissions=await call('permissions.snapshot',{});
      let message;
      if(frame.method==='observe'||frame.method==='message'){
        message=parseMessageEvent(frame.params.event);
        message.msgId=frame.params.msgId; // 使用Go保留的字符串，避免JS数值精度损失。
      }
      switch (frame.method) {
        case 'ready': await instance.onReady?.(context); break;
        case 'observe': await instance.observeMessage?.(message, context); break;
        case 'event': result = (await instance.onEvent?.(frame.params, context)) === true; break;
        case 'message': result = (await instance.onMessage?.(message, context)) === true; break;
        case 'dispose': await instance.dispose?.(); draining = true; break;
        default: throw new Error('UNKNOWN_HOOK');
      }
    }
    send({ id: frame.id, result: result ?? null });
  } catch (_error) {
    // 不把异常堆栈、路径或上游正文跨进程带入管理API和日志。
    send({ id: frame.id, error: { code: 'hook_failure' } });
  }
}
let buffered = Buffer.alloc(0);
process.stdin.on('data', chunk => {
  buffered = Buffer.concat([buffered, chunk]);
  for (;;) {
    const end = buffered.indexOf(10);
    if (end < 0) break;
    if (end > maxBytes) process.exit(74);
    const line = buffered.subarray(0, end); buffered = buffered.subarray(end + 1);
    let frame;
    try { frame = JSON.parse(line); } catch { process.exit(75); }
    if (!frame || typeof frame.id !== 'string') process.exit(76);
    if (!frame.method) {
      const item = pending.get(frame.id);
      if (item) { pending.delete(frame.id); clearTimeout(item.timer); frame.error ? item.reject(new Error('HOST_FAILURE')) : item.resolve(frame.result); }
      continue;
    }
    if (!frame.id.startsWith('g:') || draining || active >= 64) process.exit(77);
    active++;
    const wxid=frame.method==='message'||frame.method==='observe'?frame.params?.event?.wxid:frame.params?.wxid;
    void local.run({id:frame.id,wxid},()=>request(frame)).finally(() => active--);
  }
  if (buffered.length > maxBytes) process.exit(78);
});
process.stdin.on('end', () => process.exit(0));
