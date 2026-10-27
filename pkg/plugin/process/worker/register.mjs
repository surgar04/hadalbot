// 自定义加载器只负责TS语法和明确的兼容别名，不启动旧机器人。
import { register } from 'node:module';
register('./loader.mjs', import.meta.url);
