import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// String-based path math avoids the DOM-lib `URL` vs `node:url` `URL` type clash
// that Expo's tsconfig base (lib includes DOM) would otherwise raise（app-smoke 同注释）。
const here = dirname(fileURLToPath(import.meta.url));
const workspaceRoot = resolve(here, '..', '..', '..');

test('app.json declares the Android release surface: package, scheme, versionCode and runtime permissions', () => {
  const config = JSON.parse(readFileSync(resolve(here, '..', 'app.json'), 'utf8')) as {
    expo: { scheme?: string; android?: { package?: string; versionCode?: number; permissions?: string[] } };
  };
  assert.equal(config.expo.android?.package, 'com.weknora.mobile', 'android applicationId 显式声明（验收标准 1 的包身份）');
  assert.equal(config.expo.scheme, 'weknora', 'weknora:// 深链 scheme——auth-return 与通知深链的系统返回通道');
  assert.equal(typeof config.expo.android?.versionCode, 'number', 'versionCode 显式声明（Release 递增基线，prebuild 写入 build.gradle）');
  const permissions = config.expo.android?.permissions ?? [];
  for (const required of ['INTERNET', 'VIBRATE', 'RECORD_AUDIO', 'POST_NOTIFICATIONS']) {
    assert.equal(permissions.includes(required), true, `android.permissions 必须显式包含 ${required}——库 manifest 在 Gradle 期才合并，显式声明才进 prebuild 产物（差异记录 6）`);
  }
});

test('the native workflow prerequisites are declared dependencies (mic / network / file cleanup)', () => {
  const pkg = JSON.parse(readFileSync(resolve(here, '..', 'package.json'), 'utf8')) as { dependencies?: Record<string, string> };
  const expected: Array<readonly [string, string]> = [
    ['expo-audio', '~55.0.18'],
    ['expo-network', '~55.0.18'],
    ['expo-file-system', '~55.0.26'],
  ];
  for (const [name, version] of expected) {
    assert.equal(pkg.dependencies?.[name], version, `${name}@${version} 必须声明（版本源 node_modules/expo/bundledNativeModules.json，差异记录 7）`);
  }
});

test('eas.json provides an installable release path (preview apk) and the store path (production aab)', () => {
  const eas = JSON.parse(readFileSync(resolve(here, '..', 'eas.json'), 'utf8')) as {
    build?: Record<string, { distribution?: string; android?: { buildType?: string } }>;
  };
  assert.equal(eas.build?.preview?.android?.buildType, 'apk', '验收路径：preview 产直接可安装的 Release APK');
  assert.equal(eas.build?.preview?.distribution, 'internal', 'preview 走内部分发（真机安装验收）');
  assert.equal(eas.build?.production?.android?.buildType, 'app-bundle', '商店路径：production 产 AAB');
});

test('the generated android project stays out of version control (CNG, same as ios/)', () => {
  // --quiet（不带 -v）：负模式匹配（根 !apps/mobile/**）不算被忽略 → exit 1；
  // git 2.54 实测 -v 模式对负模式匹配也返回 0，会使 RED 阶段本测试假绿。
  const result = spawnSync('git', ['check-ignore', '--quiet', 'apps/mobile/android/build.gradle'], {
    cwd: workspaceRoot,
    encoding: 'utf8',
  });
  assert.equal(result.status, 0, `apps/mobile/android/ 必须被忽略（根 !apps/mobile/** 再包含之下需要更深规则）；exit=${result.status}`);
});
