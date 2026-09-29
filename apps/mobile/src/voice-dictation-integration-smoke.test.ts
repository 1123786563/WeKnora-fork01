import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import * as nodeModule from 'node:module';

// NewTaskScreen.tsx（GOAL_TEXT_MAX_LENGTH 源头）静态拉入 react-native 的 Flow 源码，
// esbuild 无法转换。与 app-smoke.test.tsx:10-45 相同的在库先例：把 react-native 解析为
// 惰性 CommonJS stub（真实文件，import 与转译后的 require 都能加载），再动态导入被测模块
// （静态导入会被提升、先于钩子注册执行，故必须动态导入；断言与 brief 逐字一致）。
type ResolveNext = (specifier: string, context: unknown) => unknown;
type ResolveHook = (specifier: string, context: unknown, nextResolve: ResolveNext) => unknown;
const moduleWithHooks = nodeModule as typeof nodeModule & {
  registerHooks?: (hooks: { resolve: ResolveHook }) => void;
};

const NATIVE_MODULE_STUBS: Record<string, string> = {
  'react-native': "module.exports = { View: 'View', Text: 'Text', TextInput: 'TextInput', Button: 'Button', ScrollView: 'ScrollView', Image: 'Image' }",
};
const stubDir = mkdtempSync(join(tmpdir(), 'weknora-mobile-stub-'));
const stubPath = (name: string): string => join(stubDir, `${name.replaceAll('/', '+')}.cjs`);
for (const [name, source] of Object.entries(NATIVE_MODULE_STUBS)) {
  writeFileSync(stubPath(name), source);
}
if (moduleWithHooks.registerHooks) {
  moduleWithHooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier in NATIVE_MODULE_STUBS
        ? { shortCircuit: true, url: pathToFileURL(stubPath(specifier)).href }
        : nextResolve(specifier, context),
  });
}
test.after(() => {
  rmSync(stubDir, { recursive: true, force: true });
});

// tsx 以 CJS 输出 .ts（顶层 await 不可用），故在各测试体内动态导入（先例：app-smoke.test.tsx）。
const loadMod = () => import('./voice-dictation-integration-smoke.ts');

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip and a private/localhost host is invalid, never a pass', async () => {
  const { voiceDictationIntegrationConfig } = await loadMod();
  const missing = voiceDictationIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(missing.enabled, false);
  assert.equal(missing.disposition, 'skip');
  for (const host of ['http://insecure.example', 'https://localhost', 'https://127.0.0.1', 'https://10.0.0.5']) {
    const invalid = voiceDictationIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host,
      WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: ['short-lived', 'secret'].join('-'),
    });
    assert.equal(invalid.enabled, false, `${host} 不得成为被授权目标`);
    assert.equal(invalid.disposition, 'invalid');
  }
});

test('live dictation end to end: cancel keeps the typed goal, confirm is explicit, nothing is ever submitted (opt-in)', async (t) => {
  const { emitVoiceDictationIntegrationEvidence, runVoiceDictationIntegration, voiceDictationIntegrationConfig } = await loadMod();
  const config = voiceDictationIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runVoiceDictationIntegration(config);
  const emitted: string[] = [];
  emitVoiceDictationIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).transcription, evidence.transcription);
  assert.equal(evidence.recordingCancelled, true, 'AC1：取消录音路径已真实执行');
  assert.equal(evidence.cancelKeptGoalText, true, 'AC1：取消不破坏手写目标文本');
  assert.equal(evidence.dictationNeverSubmitted, true, 'AC2：听写全程零 Task 提交');
  assert.ok(
    evidence.transcription === 'transcribed' || evidence.transcription === 'charging-unconfigured' || evidence.transcription === 'failed',
    '转写结果如实记录（未配置语音计价的部署如实记 charging-unconfigured，不伪造）',
  );
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|token|email/i, '证据不含凭据');
});

test('the integration runner is total: a failing transport still yields evidence, not a rejection', async () => {
  const { runVoiceDictationIntegration } = await loadMod();
  const evidence = await runVoiceDictationIntegration({ enabled: true, deploymentOrigin: 'https://weknora.invalid.test', email: 'nobody@example.test', password: 'wrong' });
  assert.equal(typeof evidence.errorReason, 'string');
  assert.equal(evidence.dictationNeverSubmitted, false, '登录失败路径：提交计数未被消费，字段如实为 false 而非伪造 true');
});
