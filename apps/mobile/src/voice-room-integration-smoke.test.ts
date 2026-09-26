import test from 'node:test';
import assert from 'node:assert/strict';

// tsx 以 CJS 输出 .ts（顶层 await 不可用），在各测试体内动态导入（先例：voice-dictation-integration-smoke.test.ts）。
const loadMod = () => import('./voice-room-integration-smoke.ts');
const env = (): Record<string, string | undefined> => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip and a private/localhost host is invalid, never a pass', async () => {
  const { voiceRoomIntegrationConfig } = await loadMod();
  const missing = voiceRoomIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(missing.enabled, false);
  assert.equal(missing.disposition, 'skip');
  for (const host of ['http://insecure.example', 'https://localhost', 'https://127.0.0.1', 'https://10.0.0.5', 'https://169.254.1.2']) {
    const invalid = voiceRoomIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host,
      WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: ['short-lived', 'secret'].join('-'),
    });
    assert.equal(invalid.enabled, false, `${host} 不得成为被授权目标`);
    assert.equal(invalid.disposition, 'invalid');
  }
});

test('live voice room end to end: open → turn → confirm steer → leave settled → resume new session (opt-in)', async (t) => {
  const { emitVoiceRoomIntegrationEvidence, runVoiceRoomIntegration, voiceRoomIntegrationConfig } = await loadMod();
  const config = voiceRoomIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runVoiceRoomIntegration(config);
  const emitted: string[] = [];
  emitVoiceRoomIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).sessionOpened, evidence.sessionOpened);
  assert.equal(evidence.sessionOpened, true, '真实授权会话已开启');
  assert.equal(
    evidence.turn === 'transcribed' || evidence.turn === 'charging-unconfigured' || evidence.turn === 'failed',
    true,
    '转写结果如实记录（未配置语音计价的部署如实记 charging-unconfigured，不伪造）',
  );
  assert.equal(evidence.rawAudioDiscarded, true, 'AC1：每轮原始音频处置回调真实发生');
  assert.equal(evidence.voiceNeverDecided, true, 'AC2：语音通道全程零决定能力（确认只产 steer 且句柄无决定方法）');
  if (evidence.turn === 'transcribed') {
    assert.equal(evidence.disconnect, 'ended-settled', 'AC1：leave 的服务端结算如实落到 settled');
    assert.equal(evidence.resume, 'new-session', 'AC1：恢复以新会话开启');
  }
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|email/i, '证据不含凭据');
});

test('the integration runner is total: a failing transport still yields evidence, not a rejection', async () => {
  const { runVoiceRoomIntegration } = await loadMod();
  const evidence = await runVoiceRoomIntegration({ enabled: true, deploymentOrigin: 'https://weknora.invalid.test', email: 'nobody@example.test', password: 'wrong' });
  assert.equal(typeof evidence.errorReason, 'string');
  assert.equal(evidence.sessionOpened, false, '登录失败路径：会话未开，字段如实为 false 而非伪造 true');
});
