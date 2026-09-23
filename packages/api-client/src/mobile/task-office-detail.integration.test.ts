import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskDetailIntegrationEvidence, runTaskDetailIntegration, taskDetailIntegrationConfig } from '../../../../apps/mobile/src/task-detail-integration-smoke.ts';

/**
 * Opt-in real HTTP check（AC3 端到端）。无回退凭据、无 mock：
 *
 * WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
 * WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
 * WEKNORA_MOBILE_TEST_PASSWORD=<short-lived-secret> \
 * pnpm exec tsx --test packages/api-client/src/mobile/task-office-detail.integration.test.ts
 */
test('real HTTP task detail hydrates, streams and resyncs through the task office', async (t) => {
  const config = taskDetailIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`TASK_DETAIL_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`TASK_DETAIL_HTTP_INVALID: ${config.reason}`);
    return;
  }
  const evidence = await runTaskDetailIntegration(config);
  emitTaskDetailIntegrationEvidence(evidence, (record) => t.diagnostic(record));
  assert.equal(evidence.opened, 'hydrated', 'opening a real task must hydrate end to end');
  assert.equal(evidence.connection !== undefined && evidence.connection !== 'interrupted', true, 'a healthy deployment ends live or drained');
  assert.equal(typeof evidence.timelineEntries, 'number');
  assert.equal(evidence.resync, 'resynced', 'the explicit resync path must work against the real backend');
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|token|email/i);
});

test('a path-bearing deployment URL is invalid, not skippable', () => {
  const probeSecret = ['short-lived', 'secret'].join('-');
  const config = taskDetailIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example/api/v1',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: probeSecret,
  });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'invalid');
});
