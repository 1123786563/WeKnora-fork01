import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// D1/D2 的缺陷都在官方构建产物里（docs/plans/issue-140/task-6-live-validation.md）：
// D1：artifact 页 JSON 的 t-button 引用被 Taro 解析成含 node_modules 段的 /npm/.pnpm/... 路径，
//     而 dist 从不产出对应组件文件，DevTools 报 getAppJSON error 白屏；
// D2：base.wxml 的 tmpl_0_t-button 模板不带 bindtap，onTap 事件链断裂。
// 这里对 build:weapp 的真实输出做最小断言。单测覆盖边界声明：本文件只验证“产物内容”，
// 不验证 DevTools 运行时行为——真实判据是 DevTools GUI 复验。
// 若 dist/ 不存在则跳过（pnpm build:weapp 之后再跑）。

const appDir = resolve(import.meta.dirname, '..');
const dist = resolve(appDir, 'dist');
const skipReason = existsSync(resolve(dist, 'app.json')) ? false : 'dist/ 不存在——先运行 pnpm build:weapp';

test('D1: artifact 页组件引用不含 node_modules 段且在 dist 内可解析', { skip: skipReason }, () => {
  const pageJson = JSON.parse(readFileSync(resolve(dist, 'subpackages/execution/artifact/index.json'), 'utf8'));
  const ref = pageJson.usingComponents?.['t-button'];
  assert.ok(typeof ref === 'string' && ref.length > 0, 't-button 组件引用必须存在');
  assert.ok(!ref.includes('node_modules'), `引用不得含 node_modules 段（当前：${ref}）`);
  assert.ok(!ref.includes('.pnpm'), `引用不得含 .pnpm 段（当前：${ref}）`);
  const base = resolve(dist, ref.replace(/^\//, ''));
  for (const ext of ['.wxml', '.js', '.json', '.wxss']) {
    assert.ok(existsSync(base + ext), `DevTools 需要的组件文件必须存在于构建产物：${ref}${ext}`);
  }
});

test('D2: base.wxml 的 t-button 模板必须绑定 tap 事件与真实属性', { skip: skipReason }, () => {
  const wxml = readFileSync(resolve(dist, 'base.wxml'), 'utf8');
  const m = wxml.match(/<template name="tmpl_0_t-button">[\s\S]*?<\/template>/);
  assert.ok(m, 'base.wxml 必须包含 tmpl_0_t-button 模板');
  const tmpl = m[0];
  assert.match(tmpl, /bindtap="eh"/, 't-button 模板必须把 tap 绑定到 Taro 事件通道 eh');
  for (const attr of ['block', 'disabled', 'loading', 'customStyle']) {
    assert.match(tmpl, new RegExp(`${attr}="\\{\\{i\\.${attr}\\}\\}"`), `t-button 模板必须绑定 ${attr} 属性`);
  }
});
