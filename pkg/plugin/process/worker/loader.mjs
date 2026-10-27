import { readFile, realpath } from 'node:fs/promises';
import { stripTypeScriptTypes } from 'node:module';
import { pathToFileURL, fileURLToPath } from 'node:url';
import path from 'node:path';
const root = process.env.HADALBOT_PLUGIN_ROOT;
const id = process.env.HADALBOT_PLUGIN_ID;
const aliases = new Map([
  ['@/bot/plugin-base.js', 'base.mjs'],
  ['@/api/types/events.js', 'events.mjs'],
  ['@/types/message.js', 'message.mjs'],
  ['@/api/types/api.js', 'api.mjs'],
  ['@/api/types/index.js', 'index.mjs'],
]);
export async function resolve(specifier, context, next) {
  if (aliases.has(specifier)) return { url: new URL(aliases.get(specifier), import.meta.url).href, shortCircuit: true };
  if (specifier.startsWith(`@/plugins/${id}/`)) {
    specifier = pathToFileURL(path.join(root, specifier.slice(`@/plugins/${id}/`.length))).href;
  } else if (specifier.startsWith('@/')) {
    // 未迁移的Prisma、渲染、Provider等绝不能偷偷导入旧工程，也不能返回假成功。
    throw new Error('UNSUPPORTED_LEGACY_MODULE');
  }
  try { return await next(specifier, context); }
  catch (error) {
    if (error.code !== 'ERR_MODULE_NOT_FOUND' || !specifier.endsWith('.js')) throw error;
    return next(specifier.slice(0, -3) + '.ts', context);
  }
}
export async function load(url, context, next) {
  if (url.startsWith('file:') && url.endsWith('.ts')) {
    const filename = await realpath(fileURLToPath(url));
    const rel = path.relative(root, filename);
    if (rel === '..' || rel.startsWith('..' + path.sep) || path.isAbsolute(rel)) throw new Error('PLUGIN_MODULE_OUTSIDE_ROOT');
    // transform支持enum及参数属性；完整类型检查仍由插件作者的tsc负责。
    const source = stripTypeScriptTypes(await readFile(filename, 'utf8'), { mode: 'transform', sourceMap: true, sourceUrl: url });
    return { format: 'module', source, shortCircuit: true };
  }
  return next(url, context);
}
