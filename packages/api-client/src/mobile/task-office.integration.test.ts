import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskOfficeIntegrationEvidence, runTaskOfficeIntegration, taskOfficeIntegrationConfig } from '../../../../apps/mobile/src/task-office-integration-smoke.ts';

/**
 * Opt-in real HTTP check（AC3 端到端）。无回退凭据、无 mock：
 *
 * WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
 * WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
 * WEKNORA_MOBILE_TEST_PASSWORD=<short-lived-secret> \
 * pnpm exec tsx --test packages/api-client/src/mobile/task-office.integration.test.ts
 *
 * 证据只包含：部署 origin、三段计数、搜索/归档回路结果与时间戳；绝不包含 token/凭据。
 */
test('real HTTP home, list, search and archive roundtrip through the task office', async (t) => {
  const config = taskOfficeIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`TASK_OFFICE_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`TASK_OFFICE_HTTP_INVALID: ${config.reason}`);
    return;
  }
  const evidence = await runTaskOfficeIntegration(config);
  emitTaskOfficeIntegrationEvidence(evidence, (record) => t.diagnostic(record));
  assert.equal(evidence.serverAuthBoundary, 'rejected', 'the unauthenticated server boundary must reject the direct request');
  assert.equal(evidence.unauthenticatedRead, 'rejected', 'the client-side scope lease must keep unauthenticated reads closed');
  assert.equal(evidence.unauthenticatedWrite, 'rejected', 'the client-side scope lease must keep unauthenticated writes closed');
  assert.equal(evidence.home, 'loaded', 'the aggregate overview must load end to end');
  assert.equal(evidence.sections !== 'unavailable', true);
  assert.notEqual(evidence.listSearch, 'failed');
  assert.notEqual(evidence.archiveRoundtrip, 'failed');
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|token|email/i);
});

test('a path-bearing deployment URL is invalid, not skippable', () => {
  // 占位哑值（运行时拼出 short-lived-secret），非真实凭据；仅用于填充本地 config 校验的 env 形参。
  const probeSecret = ['short-lived', 'secret'].join('-');
  const config = taskOfficeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example/api/v1',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: probeSecret,
  });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'invalid');
});
