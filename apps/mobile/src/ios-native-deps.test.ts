import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  installedPodNames,
  nativePodGaps,
  parseAutolinkingResolution,
  podsSectionOf,
} from './ios-native-deps.ts';
import { verifyFromInputs } from './ios-native-deps-cli.ts';

// B5 复验 important 发现的定向单测：旧 Podfile.lock 缺 expo-audio/expo-network 原生 pod 时，
// xcodebuild 增量构建仍 SUCCEEDED、产物静默缺模块（2026-09-28 复验实证）。这里的守卫必须
// 在构建前把该缺口点名出来。

/** 真实 autolinking 输出的最小节选（形状取自 expo 55 的 resolve --platform ios --json）。 */
const RESOLUTION_JSON = JSON.stringify({
  extraDependencies: [],
  coreFeatures: [],
  modules: [
    {
      packageName: 'expo',
      pods: [{ podName: 'Expo', podspecDir: '/x/node_modules/expo' }],
      swiftModuleNames: ['Expo'],
      modules: [],
      appDelegateSubscribers: [],
      reactDelegateHandlers: [],
      debugOnly: false,
      packageVersion: '55.0.31',
    },
    {
      packageName: 'expo-audio',
      pods: [{ podName: 'ExpoAudio', podspecDir: '/x/node_modules/expo-audio/ios' }],
      swiftModuleNames: ['ExpoAudio'],
      modules: [{ name: 'Audio', class: 'AudioModule' }],
      appDelegateSubscribers: [],
      reactDelegateHandlers: [],
      debugOnly: false,
      packageVersion: '55.0.18',
    },
    {
      packageName: 'expo-network',
      pods: [{ podName: 'ExpoNetwork', podspecDir: '/x/node_modules/expo-network/ios' }],
      swiftModuleNames: ['ExpoNetwork'],
      modules: [{ name: 'Network', class: 'NetworkModule' }],
      appDelegateSubscribers: [],
      reactDelegateHandlers: [],
      debugOnly: false,
      packageVersion: '55.0.18',
    },
    {
      packageName: 'expo-modules-core',
      pods: [
        { podName: 'ExpoModulesCore', podspecDir: '/x/node_modules/expo-modules-core' },
        { podName: 'ExpoModulesJSI', podspecDir: '/x/node_modules/expo-modules-core/jsi' },
      ],
      swiftModuleNames: ['ExpoModulesCore'],
      modules: [],
      appDelegateSubscribers: [],
      reactDelegateHandlers: [],
      debugOnly: false,
      packageVersion: '55.0.31',
    },
  ],
});

/** 完整 lock 的 PODS 节选（行格式取自本仓库 apps/mobile/ios/Podfile.lock 实测：
 *  顶层条目两空格缩进、每个已装 pod 都有顶层行（如 ExpoModulesJSI 在 :82）、
 *  传递依赖四空格缩进、subspec 带引号（真实 lock 有 98 行 `  - "X/y (…)"` 形态）。 */
const COMPLETE_LOCK = `PODS:
  - Expo (55.0.31):
    - ExpoModulesCore (55.0.31)
  - ExpoAudio (55.0.18):
    - Expo (55.0.31)
  - ExpoModulesCore (55.0.31):
    - ExpoModulesJSI (55.0.31)
  - ExpoModulesJSI (55.0.31):
    - hermes-engine (1.0)
  - "ReactCommon/callinvoker (0.83.10)":
    - React-callinvoker (0.83.10)
  - ExpoNetwork (55.0.18):
    - Expo (55.0.31)

DEPENDENCIES:
  - "ExpoAudio (from \`../node_modules/expo-audio/ios\`)"
  - "ExpoNetwork (from \`../node_modules/expo-network/ios\`)"

SPEC CHECKSUMS:
  ExpoAudio: 507b44eb640d1e0e49d55f38d0b153d3b1fd0f2b
  ExpoNetwork: c41f5f750450edadc4724dbf424c3bf224f171e2

PODFILE CHECKSUM: d2c00c8ee4664eba15fe8fb557223a4ed2879dc1

COCOAPODS: 1.17.0
`;

/** B5 陷阱现场：pre-B5 的旧 lock——没有 ExpoAudio/ExpoNetwork（却有其它 pod），xcodebuild 照样成功。 */
const STALE_LOCK = `PODS:
  - Expo (55.0.31):
    - ExpoModulesCore (55.0.31)
  - ExpoModulesCore (55.0.31):
    - ExpoModulesJSI (55.0.31)
  - ExpoModulesJSI (55.0.31):
    - hermes-engine (1.0)
  - "ReactCommon/callinvoker (0.83.10)":
    - React-callinvoker (0.83.10)

DEPENDENCIES:
  - "Expo (from \`../node_modules/expo\`)"

SPEC CHECKSUMS:
  Expo: 639993dd1c89a04b0f8441f9c17b84f5b2a2d1c7

PODFILE CHECKSUM: 0000000000000000000000000000000000000000

COCOAPODS: 1.17.0
`;

test('parseAutolinkingResolution keeps package -> pod(s), drops pod-less modules and junk', () => {
  const resolution = parseAutolinkingResolution(RESOLUTION_JSON);
  assert.deepEqual(
    resolution.map((module) => [module.packageName, [...module.pods]]),
    [
      ['expo', ['Expo']],
      ['expo-audio', ['ExpoAudio']],
      ['expo-network', ['ExpoNetwork']],
      ['expo-modules-core', ['ExpoModulesCore', 'ExpoModulesJSI']],
    ],
  );
  // pods 为空（纯 JS 包）与 packageName 缺失的条目不产出（不误报纯 JS 依赖）。
  assert.deepEqual(
    parseAutolinkingResolution(
      JSON.stringify({ modules: [{ packageName: 'expo-auth-session', pods: [] }, { pods: [{ podName: 'X' }] }] }),
    ),
    [],
  );
});

test('nativePodGaps names exactly the B5 trap: stale lock missing ExpoAudio/ExpoNetwork', () => {
  const gaps = nativePodGaps(parseAutolinkingResolution(RESOLUTION_JSON), STALE_LOCK);
  assert.deepEqual(
    gaps.map((gap) => `${gap.packageName} -> ${gap.pod}`),
    ['expo-audio -> ExpoAudio', 'expo-network -> ExpoNetwork'],
  );
});

test('nativePodGaps passes on the complete lock (PODS entries, subspecs and all)', () => {
  assert.deepEqual(nativePodGaps(parseAutolinkingResolution(RESOLUTION_JSON), COMPLETE_LOCK), []);
});

test('PODS-section-only semantics: DEPENDENCIES/SPEC CHECKSUMS mentions do not count as installed', () => {
  // 反向陷阱：lock 只在 DEPENDENCIES/SPEC CHECKSUMS 里出现 ExpoAudio（PODS 节没有）
  // = 声明过 ≠ 装上了，必须仍报缺口。
  const decoy = STALE_LOCK.replace(
    'DEPENDENCIES:\n  - "Expo (from `../node_modules/expo`)"',
    'DEPENDENCIES:\n  - "ExpoAudio (from `../node_modules/expo-audio/ios`)"',
  ).replace('  Expo: 63999', '  ExpoAudio: 507b44eb640d1e0e49d55f38d0b153d3b1fd0f2b\n  Expo: 63999');
  const gaps = nativePodGaps(parseAutolinkingResolution(RESOLUTION_JSON), decoy);
  assert.ok(gaps.some((gap) => gap.pod === 'ExpoAudio'), 'DEPENDENCIES decoy must not satisfy the guard');
});

test('installedPodNames reads top-level entries; subspec-only mention does not count as the base pod', () => {
  const names = installedPodNames(podsSectionOf(COMPLETE_LOCK));
  assert.ok(names.has('ExpoAudio'));
  assert.ok(names.has('ExpoNetwork'));
  assert.ok(names.has('ExpoModulesJSI'), 'top-level ExpoModulesJSI entry (real lock :82 shape) must register');
  // subspec 条目（带引号的 "X/y (…)"）只登记全名 X/y，不得让基础 pod X 误判为已安装：
  const subspecOnly = 'PODS:\n  - "ExpoModulesJSI/ExpoModulesJSIPlaceholder (55.0.31)":\n\nDEPENDENCIES:\n';
  assert.ok(!installedPodNames(subspecOnly).has('ExpoModulesJSI'), 'quoted subspec line leaked as base pod');
});

test('podsSectionOf tolerates a lock with no DEPENDENCIES section (whole file is the section)', () => {
  const section = podsSectionOf('PODS:\n  - Expo (55.0.31)\n');
  assert.match(section, /- Expo \(55\.0\.31\)/);
});

test('verifyFromInputs (CLI core) blocks the B5 trap and passes the healthy tree', () => {
  assert.equal(verifyFromInputs(RESOLUTION_JSON, STALE_LOCK).gaps.length, 2);
  assert.equal(verifyFromInputs(RESOLUTION_JSON, COMPLETE_LOCK).gaps.length, 0);
  assert.equal(verifyFromInputs(RESOLUTION_JSON, COMPLETE_LOCK).resolutionCount, 4);
});
