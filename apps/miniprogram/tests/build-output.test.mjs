import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { resolve, relative } from 'node:path';

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

// ---- F1（评审第 1 轮）：主包体积与拷贝范围 ----
// 微信单包/主包上限 2MB：把整个 tdesign-miniprogram/miniprogram_dist 拷进主包会超限
// （实测 2285KB）；拷贝必须收窄到 button 的运行时闭包并排除纯类型文件。

function walkFiles(root) {
  const out = [];
  const visit = dir => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const p = resolve(dir, entry.name);
      if (entry.isDirectory()) visit(p); else out.push(p);
    }
  };
  visit(root);
  return out;
}

test('F1: dist/npm/tdesign 不含 .d.ts 纯类型文件', { skip: skipReason }, () => {
  const dts = walkFiles(resolve(dist, 'npm/tdesign')).filter(p => p.endsWith('.d.ts'));
  assert.deepEqual(dts, [], `.d.ts 不应进入构建产物（发现 ${dts.length} 个，如 ${dts[0] ?? '无'}）`);
});

test('F1: 主包（dist 去分包目录）体积必须低于微信 2MB 上限', { skip: skipReason }, () => {
  const appJson = JSON.parse(readFileSync(resolve(dist, 'app.json'), 'utf8'));
  const subRoots = (appJson.subpackages ?? appJson.subPackages ?? []).map(s => resolve(dist, s.root));
  const isSub = p => subRoots.some(r => p === r || p.startsWith(r + resolve('/')));
  let main = 0;
  for (const p of walkFiles(dist)) if (!isSub(p)) main += statSync(p).size;
  const limit = 2 * 1024 * 1024;
  assert.ok(main < limit, `主包 ${(main / 1024).toFixed(0)}KB 超过微信 2MB 上限（${(limit / 1024).toFixed(0)}KB）`);
});

test('F1: tdesign 拷贝闭包完整（button 及其 usingComponents 传递依赖 + common 运行时）', { skip: skipReason }, () => {
  const tdesignDir = resolve(dist, 'npm/tdesign');
  // button.json 的传递依赖必须在产物中（DevTools 加载 button 时按 json 递归解析）
  const queue = ['button/button'];
  const seen = new Set();
  while (queue.length) {
    const stem = queue.pop();
    if (seen.has(stem)) continue;
    seen.add(stem);
    const jsonPath = resolve(tdesignDir, `${stem}.json`);
    assert.ok(existsSync(jsonPath), `闭包组件 ${stem}.json 必须存在`);
    const cfg = JSON.parse(readFileSync(jsonPath, 'utf8'));
    for (const ref of Object.values(cfg.usingComponents ?? {})) {
      queue.push(relative(resolve(tdesignDir), resolve(tdesignDir, stem, '..', ref)));
    }
    for (const ext of ['.js', '.wxml', '.wxss']) {
      assert.ok(existsSync(resolve(tdesignDir, stem + ext)), `闭包组件文件 ${stem}${ext} 必须存在`);
    }
  }
  // button.js 的 common 运行时依赖（../common/config、../common/src/index 等）
  const buttonJs = readFileSync(resolve(tdesignDir, 'button/button.js'), 'utf8');
  for (const m of buttonJs.matchAll(/from"(\.\.\/common[^"]*)"/g)) {
    const dep = resolve(tdesignDir, 'button', m[1].endsWith('.js') ? m[1] : m[1] + '.js');
    assert.ok(existsSync(dep), `button 运行时依赖 ${m[1]} 必须存在`);
  }
  // TDesign 组件 JS 以裸模块名 require 依赖（如 require("tslib")）；微信按向上查找
  // miniprogram_npm/<name> 解析。miniprogram_dist/miniprogram_npm 必须一并拷入，否则
  // 组件模块初始化抛 module not defined → 页面 "has not been registered yet" 白屏
  // （第 2 轮实测定位：npm/tdesign/button/tslib.js is not defined, require args is 'tslib'）。
  assert.ok(existsSync(resolve(tdesignDir, 'miniprogram_npm/tslib/index.js')), 'miniprogram_npm/tslib 必须随闭包拷入');
  // 收窄：未使用的组件目录不得拷入（防回归到全量拷贝）
  const dirs = readdirSync(tdesignDir, { withFileTypes: true }).filter(e => e.isDirectory()).map(e => e.name).sort();
  assert.deepEqual(dirs, ['button', 'common', 'icon', 'loading', 'miniprogram_npm'].sort(), 'tdesign 拷贝范围必须收窄到 button 闭包 + miniprogram_npm');
});
