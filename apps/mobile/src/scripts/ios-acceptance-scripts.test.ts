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
  assert.match(script, /expo prebuild -p ios --clean --no-install/, '必须清除 SDK55 遗留 ios/ 再生成 SDK57 工程');
  assert.doesNotMatch(script, /ios-xcode27\.js|重放入库 plugin/);
  const prebuildAt = script.indexOf('expo prebuild -p ios --clean --no-install');
  const contractAt = script.indexOf('pnpm exec tsx scripts/verify-ios-scene-project.ts');
  const podAt = script.search(/^pod install$/m);
  const buildAt = script.search(/^xcodebuild /m);
  assert.ok(podAt >= 0, '必须执行 pod install');
  assert.ok(contractAt > prebuildAt && podAt > contractAt, '生成工程契约必须在 clean prebuild 后、Pods 前执行');
  assert.ok(buildAt > podAt, 'xcodebuild 必须在 pod install 之后（rm -rf build 连带删 codegen 产物，B4 实测教训）');
  assert.match(script, /-configuration Release/, '证据口径是 Release 包，不是 Debug');
  assert.match(script, /-sdk iphonesimulator/, '模拟器 SDK 口径');
  assert.match(script, /-derivedDataPath build/, '产物路径固定，验收脚本才能找到 WeKnora.app');
  assert.match(script, /babel-preset-expo/, 'pnpm 工作区下 babel-preset-expo 链接守卫（B3 实测缺失即 Metro 打包失败）');
  assert.match(script, /Release-iphonesimulator\/WeKnora\.app/, '脚本末尾必须解析出 .app 产物路径');
  const guardCmdAt = script.search(/^pnpm exec tsx scripts\/verify-ios-native-deps\.ts$/m);
  const podCmdAt = script.search(/^pod install$/m);
  const xcodebuildCmdAt = script.search(/^xcodebuild -workspace /m);
  assert.ok(podCmdAt >= 0 && guardCmdAt > podCmdAt && xcodebuildCmdAt > guardCmdAt, 'B5 复验 important 发现：原生依赖漂移守卫必须夹在 pod install 与 xcodebuild 之间——旧 Podfile.lock 缺 expo-audio/expo-network 时裸 xcodebuild 静默产出缺模块的包');
  assert.match(script, /watchman watch-project/, 'B5 复验 minor 发现：watchman 预热缓解主仓库 .worktrees 初始 crawl 挤压构建预算');
  assert.match(script, /verify-ios-framework-closure\.py" "\$APP" "\$IOS\/Podfile\.properties\.json"/, 'Release must invoke the tested checker with generated mode properties');
});

test('the acceptance probe script installs before probing and records the no-credential paths', () => {
  const script = scriptOf('ios-acceptance-run.sh');
  assert.match(script, /set -euo pipefail/);
  const installAt = script.indexOf('simctl install');
  const launchAt = script.indexOf('simctl launch');
  assert.ok(installAt >= 0 && launchAt > installAt, '必须先 install 再 launch——证据来自安装包，不是浏览器 prototype（AC1）');
  assert.match(script, /recordVideo/, '冷启动必须有录屏证据（AC2 cold-start）');
  assert.match(script, /simctl uninstall/, '冷启动探针前必须干净卸载（首装冷启动口径）');
  assert.match(script, /weknora:\/\/tasks\/detail/, '未授权深链 fail-closed 探针必须包含详情深链（越权面最强探针）');
  assert.match(script, /simctl push/, '未授权推送投递探针（AC2 撤销/越权面）');
  assert.match(script, /revoke microphone/, '麦克风拒权探针（AC2 permission-denied）');
  assert.match(script, /NSException\|SIGTRAP/, '日志崩溃筛查必须覆盖 NSException/SIGTRAP 口径（B4 同款）');
});

test('the unauthorized push probe tolerates system rejection and archives the outcome honestly (Task 6 live-run fix)', () => {
  const script = scriptOf('ios-acceptance-run.sh');
  assert.match(
    script,
    /if ! xcrun simctl push/,
    '未授权源上系统拒绝投递（Task 6 实跑：UNErrorDomain 2003），set -e 不得在此中止整条管线——崩溃由日志门判定，不由投递成败判定',
  );
  assert.match(script, /push-error\.txt/, '投递拒绝的错误输出必须留档 push-error.txt（诚实证据，不得静默吞掉）');
  assert.match(script, /PUSH_DELIVERY_REJECTED/, '投递被拒必须打印显式标记，供验收报告如实引用');
  assert.match(
    script,
    /06-push-unauthorized-fail-closed\.png/,
    '截图名不得预设「已送达」——包外无法证实应用侧是否收到帧，可证事实是 fail-closed 存活',
  );
});
