import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { taskBudgetIntegrationConfig } from './task-budget-integration-smoke.ts';

const here = dirname(fileURLToPath(import.meta.url));

test('taskBudgetIntegrationConfig skips without credentials and never fabricates enabled', () => {
  const config = taskBudgetIntegrationConfig({});
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
});

test('taskBudgetIntegrationConfig validates the HTTPS origin and host line', () => {
  for (const url of ['http://weknora.example.test', 'https://user:pw@weknora.example.test', 'https://127.0.0.1:8080', 'https://weknora.example.test/path']) {
    const config = taskBudgetIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: url,
      WEKNORA_MOBILE_TEST_EMAIL: 'e@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: 'p',
    });
    assert.equal(config.enabled, false, url);
    assert.equal(config.disposition, 'invalid', url);
  }
});

test('the extend arm is opt-in via WEKNORA_MOBILE_TEST_EXTEND_BUDGET=1 with a positive credit amount', () => {
  const config = taskBudgetIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.test',
    WEKNORA_MOBILE_TEST_EMAIL: 'e@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
  });
  assert.equal(config.enabled, true);
  assert.equal(config.extendBudget, false);
  const extended = taskBudgetIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.test',
    WEKNORA_MOBILE_TEST_EMAIL: 'e@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
    WEKNORA_MOBILE_TEST_EXTEND_BUDGET: '1',
    WEKNORA_MOBILE_TEST_EXTEND_CREDITS: '5',
  });
  assert.equal(extended.enabled, true);
  assert.ok(extended.enabled);
  assert.equal((extended as { extendBudget: boolean }).extendBudget, true);
  assert.equal((extended as { extendCredits: number }).extendCredits, 5);
});

test('runTaskBudgetIntegration records the paused projection as a count, not an Array.isArray tautology', () => {
  // 终审修复 #3：parseTaskBudgetFacts 已保证 pausedRunIds 是数组，Array.isArray(facts.pausedRunIds)
  // 是同义反复；证据契约以计数承载「达限暂停清单非空/为空」的语义。
  const source = readFileSync(join(here, 'task-budget-integration-smoke.ts'), 'utf8');
  assert.match(source, /pausedRunCount\?: number/, '证据契约以计数承载暂停清单非空/为空的语义');
  assert.match(source, /evidence\.pausedRunCount = facts\.pausedRunIds\.length/, '运行体以 facts.pausedRunIds.length 投影计数');
  assert.doesNotMatch(source, /Array\.isArray\(facts\.pausedRunIds\)/, '不得复述 parse 已保证的数组类型');
});

// 终审修复 #1：预算真环境集成证据挂进测试套件的 opt-in 钩子（与 material-integration-smoke
// 同一范式）——具备 WEKNORA_MOBILE_TEST_* 环境的常规 `pnpm --filter @weknora/mobile test`
// 运行会自动产出预算端到端证据；无环境时如实 skip，不冒充、不伪造。
test('runTaskBudgetIntegration executes against a real deployment when credentials are provided', { skip: Object.keys(process.env).some((key) => key === 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL') === false }, async () => {
  const { runTaskBudgetIntegration } = await import('./task-budget-integration-smoke.ts');
  const config = taskBudgetIntegrationConfig(process.env as Record<string, string | undefined>);
  if (config.enabled === false) { assert.ok(true, '凭据不完整或主机不被允许时如实跳过，不伪造通过'); return; }
  const evidence = await runTaskBudgetIntegration(config);
  assert.equal(typeof evidence.commandTimestamp, 'string');
  if (evidence.budgetFacts === 'read') {
    assert.ok(evidence.remainingConsistent === true, '真实 wire 上的四数字必须算术自洽');
    assert.equal(typeof evidence.pausedRunCount, 'number', '达限暂停清单必须以计数投影');
    if (evidence.extend === 'extended') {
      assert.ok(evidence.replayNeverDoubled === true, `同键幂等重放不得加倍（limitRaisedBy=${evidence.limitRaisedBy}）`);
    }
  }
});
