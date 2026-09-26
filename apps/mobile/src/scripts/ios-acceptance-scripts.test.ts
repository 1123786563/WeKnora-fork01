import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

// T39（#69）：两条验收管线脚本的源级断言。脚本不入 node 测试运行时，这里钉的是
// B3/B4 三次实踩过的失败序（构建漂移、安装前启动、崩溃筛查口径）。
// 路径解析沿用仓库既有模式（release-deps.test.ts / network-status.test.ts）：
// fileURLToPath 只吃 string，绕开全局 URL 与 node:url.URL 的类型冲突，
// 否则 tsc --noEmit 不过——对计划逐字内容的唯一偏差，仅此机械改动。
const here = dirname(fileURLToPath(import.meta.url));
const scriptOf = (name: string): string =>
  readFileSync(join(here, '..', '..', 'scripts', name), 'utf8');

test('the release build script is a reproducible prebuild -> pods -> Release pipeline', () => {
  const script = scriptOf('ios-release-build.sh');
  assert.match(script, /set -euo pipefail/, '任何一步失败必须中止（验收产物不可半成品）');
  assert.match(script, /expo prebuild -p ios --no-install/, 'prebuild 重放入库 plugin（ios/ 不入库，漂移防护在这里）');
  const podAt = script.indexOf('pod install');
  const buildAt = script.indexOf('xcodebuild ');
  assert.ok(podAt >= 0, '必须执行 pod install');
  assert.ok(buildAt > podAt, 'xcodebuild 必须在 pod install 之后（rm -rf build 连带删 codegen 产物，B4 实测教训）');
  assert.match(script, /-configuration Release/, '证据口径是 Release 包，不是 Debug');
  assert.match(script, /-sdk iphonesimulator/, '模拟器 SDK 口径');
  assert.match(script, /-derivedDataPath build/, '产物路径固定，验收脚本才能找到 WeKnora.app');
  assert.match(script, /babel-preset-expo/, 'pnpm 工作区下 babel-preset-expo 链接守卫（B3 实测缺失即 Metro 打包失败）');
  assert.match(script, /Release-iphonesimulator\/WeKnora\.app/, '脚本末尾必须解析出 .app 产物路径');
});
