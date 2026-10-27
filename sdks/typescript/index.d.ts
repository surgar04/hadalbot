/** TS兼容契约。类型声明不执行旧机器人代码；运行时能力仍由Go宿主逐次授权。 */
export interface QxReply<T = unknown> { code: number; msg?: string; result: T; wxid?: string }
/** 原始事件保留未知字段；业务应检查event编号再读取data。 */
export interface QxEvent { event: number; wxid?: string; data?: unknown; [field: string]: unknown }
export interface ParsedMessage {
 source: 'private' | 'group' | 'unknown';
 content: string;
 senderWxid: string;
 targetWxid: string;
 groupWxid?: string;
 atWxidList: string[];
 /** 始终使用字符串，禁止转成Number后作为去重键。 */
 msgId: string;
 msgType: number;
 raw: Record<string, unknown>;
 event: QxEvent;
}
export interface QxAPI {
 sendText(data: {wxid: string; msg: string}): Promise<QxReply>;
 /** 实际type对应的能力必须获批，例如qx.sendText；不能仅授权一个任意request入口。 */
 request<T = unknown>(type: string, data: unknown): Promise<QxReply<T>>;
}
/** 同步查询依赖宿主permissions.snapshot适配器；未配置时运行时明确报错。 */
export interface PermissionQueries {
 getLevel(wxid: string, groupWxid?: string): 'root' | 'super' | 'delegate' | 'none';
 isRoot(wxid: string): boolean;
 isSuper(wxid: string): boolean;
 isSuperOnly(wxid: string): boolean;
 isDelegate(wxid: string, groupWxid: string): boolean;
 isGroupBotEnabled(groupWxid: string): boolean;
 hasSpecialPermission(wxid: string, groupWxid?: string): boolean;
 check(context: {wxid: string; groupWxid?: string}): {level: string; hasSpecial: boolean};
}
export interface PluginContext {
 readonly api: QxAPI;
 readonly botWxid: string;
 readonly config: Readonly<Record<string, unknown>>;
 readonly perms: PermissionQueries;
 /** API调用必须发生在当前存活钩子内，并await完成。 */
 sendText(wxid: string, msg: string): Promise<QxReply>;
 readonly apis: {
  get<T>(service: string): T;
  getDynamic<T>(service: string): T;
  facade<T>(service: string): T;
  call<T = unknown>(service: string, method: string, ...args: unknown[]): Promise<T>;
  callDynamic<T = unknown>(service: string, method: string, args?: unknown[]): Promise<T>;
 };
 readonly log: { info(message: string, meta?: Record<string, unknown>): void; warn(message: string, meta?: Record<string, unknown>): void; error(message: string, meta?: Record<string, unknown>): void };
}
/** 与旧Plugin对象生命周期兼容；可直接default导出对象或实例，禁止导出尚未实例化的构造器。 */
export interface Plugin {
 name: string;
 description?: string;
 enabled?: boolean;
 runtime?: 'typescript';
 init?(context: PluginContext): void | Promise<void>;
 onReady?(context: PluginContext): void | Promise<void>;
 observeMessage?(message: ParsedMessage, context: PluginContext): void | Promise<void>;
 onEvent?(event: QxEvent, context: PluginContext): boolean | void | Promise<boolean | void>;
 onMessage?(message: ParsedMessage, context: PluginContext): boolean | void | Promise<boolean | void>;
 dispose?(): void | Promise<void>;
}
