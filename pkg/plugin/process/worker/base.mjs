/** 兼容旧PluginBase的可选钩子；不混入旧进程的数据库或API Hub实例。 */
export class PluginBase {
  enabled = true;
  init(_ctx) {}
  onReady(_ctx) {}
  observeMessage(_message, _ctx) {}
  onEvent(_event, _ctx) { return false; }
  onMessage(_message, _ctx) { return false; }
  dispose() {}
}
