import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

// 简报原文为 `srcRoot = dirname(file)/..; packageRoot = srcRoot/..`，会把 packageRoot
// 解析到 `packages/`（无 package.json，ENOENT）。最小修复：srcRoot 即本文件所在 src/，
// packageRoot 为其上一级包根 packages/mobile-core。
const srcRoot = path.dirname(fileURLToPath(import.meta.url));
const packageRoot = path.resolve(srcRoot, '..');

/**
 * Issue #68 AC1「业务 Module 不依赖 Expo 或 WeChat」的可执行门槛。
 * 业务 Module = mobile-core 六个深 Module（module-seams §3）。平台（Expo/RN/微信/Taro）
 * 只允许出现在 App Shell 的 Adapter 里；一旦有人把平台依赖引入本包，本测试立即失败。
 */
const FORBIDDEN_DEPENDENCIES = [/^expo($|\/)/, /^expo-/, /^@tarojs\//, /^react-native($|\/)/, /^weixin/, /^wechat/, /^wx-/];
// 修复轮 1（审查发现）：补 /^wechat/ 与 /^wx-/，对齐依赖门与本任务 Produces 契约
// （import/require 均不得含 wechat*/wx-*；计划 plan-t68.md:148 逐字代码漏此二形态，
// 由计划层同步修订，测试按契约先行补齐）。
const FORBIDDEN_IMPORTS = [/^expo($|\/)/, /^expo-/, /^@tarojs\//, /^react-native($|\/)/, /^weixin/, /^wechat/, /^wx-/, /^wx\//];

function tsFiles(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    const full = path.join(dir, name);
    if (statSync(full).isDirectory()) out.push(...tsFiles(full));
    else if (/\.ts$/.test(name)) out.push(full);
  }
  return out;
}

test('mobile-core declares no Expo/WeChat/Taro/React-Native dependency (issue #68 AC1 gate)', () => {
  const pkg = JSON.parse(readFileSync(path.join(packageRoot, 'package.json'), 'utf8')) as {
    dependencies?: Record<string, string>;
    devDependencies?: Record<string, string>;
  };
  const declared = [...Object.keys(pkg.dependencies ?? {}), ...Object.keys(pkg.devDependencies ?? {})];
  const offenders = declared.filter((name) => FORBIDDEN_DEPENDENCIES.some((pattern) => pattern.test(name)));
  assert.deepEqual(offenders, [], `业务 Module 不得声明平台依赖: ${offenders.join(', ')}`);
});

test('mobile-core source imports no platform module (issue #68 AC1 gate)', () => {
  const offenders: string[] = [];
  for (const file of tsFiles(srcRoot)) {
    const source = readFileSync(file, 'utf8');
    const specifiers = [
      ...source.matchAll(/from\s+['"]([^'"]+)['"]/g),
      // 副作用导入形态：import 后直接跟引号模块名（无 from、无括号）。缺失此条时，
      // Step 3 的 react-native 副作用导入探针不会触发失败（简报原文漏此形态，按 AC1 意图补齐）。
      ...source.matchAll(/import\s+['"]([^'"]+)['"]/g),
      ...source.matchAll(/import\s*\(\s*['"]([^'"]+)['"]\s*\)/g),
      ...source.matchAll(/require\s*\(\s*['"]([^'"]+)['"]\s*\)/g),
    ].map((match) => match[1]);
    for (const specifier of specifiers) {
      if (FORBIDDEN_IMPORTS.some((pattern) => pattern.test(specifier))) {
        offenders.push(`${path.relative(packageRoot, file)} -> ${specifier}`);
      }
    }
  }
  assert.deepEqual(offenders, [], `业务 Module 源码不得导入平台模块: ${offenders.join('; ')}`);
});
