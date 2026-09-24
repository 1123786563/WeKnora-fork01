import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskStartIntegrationEvidence, runTaskStartIntegration, taskStartIntegrationConfig } from './task-start-integration-smoke.ts';

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip, never fake a pass', () => {
  // 不修改 process.env（同进程泄漏会让 live 用例在环境齐备时也永远 skip）：
  // 用剔除该变量的 env 快照验证 skip 语义。
  const config = taskStartIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
  const invalid = taskStartIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://insecure.example', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
});

test('live end-to-end task creation through the highest stable interface (opt-in)', async (t) => {
  const config = taskStartIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runTaskStartIntegration(config);
  const emitted: string[] = [];
  emitTaskStartIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).start, evidence.start);
  assert.equal(evidence.start, 'admitted', 'a live deployment with one agent must admit the goal');
  assert.equal(evidence.repeatSubmitSameRequest, 'no-second-dispatch', 'AC1: the same intent never dispatches twice');
  assert.notEqual(evidence.runVisibleInTasks, false, 'when the list is reachable, the created run must be observable in it');
});

test('the integration runner is total: a failing transport still yields evidence, not a rejection', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'task-start-integration-smoke.ts'), 'utf8');
  // 结构级断言（对照 batch2 Task 9 的先例模式）：主流程包 try/catch/finally。
  assert.match(source, /finally\s*\{/);
  assert.match(source, /catch \(error\)/);
  // B3-F45：第二次同 ID 重入必须在 try 内且 catch 记 failed + errorReason（不裸 await）。
  assert.match(source, /try \{\s*\n\s*(?:const second = )?await office\.start\(goal, \{ requestId: first\.requestId \}\)/, '重入受 try 保护');
  assert.match(source, /repeatSubmitSameRequest = 'failed'/, '重入失败也落证据字段');
});
