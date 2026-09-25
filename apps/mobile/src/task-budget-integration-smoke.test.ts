import test from 'node:test';
import assert from 'node:assert/strict';
import { taskBudgetIntegrationConfig } from './task-budget-integration-smoke.ts';

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
