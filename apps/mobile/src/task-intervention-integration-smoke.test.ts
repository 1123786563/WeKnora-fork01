import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskInterventionIntegrationEvidence, runTaskInterventionIntegration, taskInterventionIntegrationConfig } from './task-intervention-integration-smoke.ts';

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip, never fake a pass', () => {
  const config = taskInterventionIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
  const invalid = taskInterventionIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://insecure.example', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
});

test('live intervention through the highest stable interface (opt-in)', async (t) => {
  const config = taskInterventionIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runTaskInterventionIntegration(config);
  const emitted: string[] = [];
  emitTaskInterventionIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).start, evidence.start);
  assert.notEqual(evidence.steer, 'failed');
  assert.notEqual(evidence.stop, 'failed');
  assert.notEqual(evidence.queueNext, 'failed');
});

test('the integration runner is total: failures still yield evidence, never a rejection', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'task-intervention-integration-smoke.ts'), 'utf8');
  assert.match(source, /catch \(error\)/);
  assert.match(source, /finally\s*\{/);
  // 修复轮 F9-1：queue-next 投递结果 unknown 时，绑定的旧 Run 恒为终态，观察它无法揭示
  // 新 Run 的重准入命运——源码不得把 unknown 经 resync 洗白成 admitted（绝不伪造通过）。
  assert.doesNotMatch(source, /outcome === 'unknown'[\s\S]{0,400}TERMINAL_STATUSES\.has\(reconciled\.runStatus\)[\s\S]{0,80}\? 'admitted'/, 'queue-next unknown 不得经旧 Run 终态洗白成 admitted');
  // 修复轮 F9-2：stop 投递结果 unknown 且核对未观察到 canceled 时，按 resolveUnknownStop
  // 的快照规则（task-intent.ts：非 canceled ⇒ 取消 CAS 不可能已落地 ⇒ 未落地），如实落
  // requested-only，绝不捏造一次从未观察到的 CAS 拒绝（conflict）。
  assert.doesNotMatch(source, /'unknown-then-reconciled' : 'conflict'/, 'stop unknown 核对未 canceled 不得捏造 conflict');
  assert.match(source, /'unknown-then-reconciled' : 'requested-only'/, 'stop unknown 核对未 canceled 如实落 requested-only');
});
