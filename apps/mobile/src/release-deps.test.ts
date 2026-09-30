import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

// T39（#69）：安装包验收的两个原生前置。测试读仓库事实（package.json/app.json/composition.ts
// 源级），依赖缺席时组合根的两个惰性 Adapter 永远 fail closed → 语音权限与离线草稿验收不可达。
// 路径解析沿用仓库既有模式（network-status.test.ts 等）：fileURLToPath 只吃 string，
// 绕开全局 URL 与 node:url.URL 的类型冲突，否则 tsc --noEmit 不过。
const here = dirname(fileURLToPath(import.meta.url));
const readJson = (relative: string): unknown =>
  JSON.parse(readFileSync(join(here, relative), 'utf8'));

test('release package ships the native voice and network adapters (T39 #69)', () => {
  const pkg = readJson('../package.json') as { dependencies?: Record<string, string> };
  assert.match(
    pkg.dependencies?.['expo-audio'] ?? '',
    /^~57\./,
    'expo-audio 缺席 → createNativeDictationCaptureIfAvailable 恒 undefined，包上无麦克风入口（#56 留给本 Issue 的构建前置）',
  );
  assert.match(
    pkg.dependencies?.['expo-network'] ?? '',
    /^~57\./,
    'expo-network 缺席 → createNativeNetworkStatusIfAvailable 恒 undefined，离线门透传（#40 留给本 Issue 的构建前置）',
  );
});

test('the composition root wiring for both adapters stays intact (guard, not duplication)', () => {
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /createNativeDictationCaptureIfAvailable\(\)/);
  assert.match(composition, /createNativeNetworkStatusIfAvailable\(\)/);
});

test('the iOS build declares the microphone permission via the expo-audio config plugin', () => {
  const app = readJson('../app.json') as { expo?: { plugins?: unknown[] } };
  const entry = app.expo?.plugins?.find((plugin) => Array.isArray(plugin) && plugin[0] === 'expo-audio');
  assert.ok(Array.isArray(entry), 'expo-audio config plugin 缺席 → prebuild 出的 Info.plist 不含 NSMicrophoneUsageDescription，拒权路径无从谈起');
  const props = entry[1] as { microphonePermission?: unknown };
  assert.equal(typeof props.microphonePermission, 'string');
  assert.ok((props.microphonePermission as string).trim().length > 0, '权限文案不得为空串');
});
