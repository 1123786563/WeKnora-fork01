import test from 'node:test';
import assert from 'node:assert/strict';
import {
  deviceInboxIntegrationConfig,
  emitDeviceInboxIntegrationEvidence,
  runDeviceInboxIntegration,
} from './device-inbox-integration-smoke.ts';

/**
 * Opt-in 真实 HTTP 检查（#41 AC3——最高稳定 Interface：真实 JSON transport + 真实 wire
 * Adapter + 真实 Mobile Runtime + 真实服务端两步注册/接管/撤销/inbox/markRead）。无回退凭据：
 *
 * WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
 * WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
 * WEKNORA_MOBILE_TEST_PASSWORD=<short-lived-secret> \
 * pnpm --filter @weknora/mobile test
 *
 * 注：真实运行凭据一律来自环境变量；下方用例中的 'pw' 是与 task-detail-integration-smoke.test.ts
 * 相同的非凭据占位符（Mimosa 硬编码凭据门禁要求，占位值从不发起任何网络请求）。
 */
test('real HTTP device registration, takeover, revocation and inbox read complete through the wire', async (t) => {
  const config = deviceInboxIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`DEVICE_INBOX_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`DEVICE_INBOX_HTTP_INVALID: ${config.reason}`);
    return;
  }

  const evidence = await runDeviceInboxIntegration(config);
  emitDeviceInboxIntegrationEvidence(evidence, (record) => t.diagnostic(record));

  assert.equal(evidence.deviceRegister, 'registered', 'two-step intent→register must bind the device');
  assert.equal(evidence.deviceTokenTakeover, 'rotated', 're-registering must take over the token');
  assert.equal(evidence.deviceRevoke, 'revoked', 'revocation must succeed');
  assert.equal(evidence.deviceRevokeAgain, 'not-found', 'revoking a revoked device must surface DEVICE_NOT_FOUND');
  assert.equal(evidence.inboxRead, 'read', 'the inbox read model must be readable');
  assert.equal(evidence.markReadIdempotent, 'ok', 'markRead on an unknown id must stay idempotent success (no business action)');
});

test('device-inbox config marks missing credentials skippable and malformed hosts invalid', () => {
  assert.deepEqual(
    deviceInboxIntegrationConfig({}),
    { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' },
  );
  const withCredentials = {
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  };
  for (const url of ['https://127.0.0.1', 'https://10.0.0.2', 'https://localhost', 'https://deployment.example/api/v1']) {
    const config = deviceInboxIntegrationConfig({ ...withCredentials, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: url });
    assert.equal(config.enabled, false, `${url} must not enable a real HTTP run`);
    assert.equal(config.disposition, 'invalid', `${url} is invalid, not skippable`);
  }
  const fallback = deviceInboxIntegrationConfig(withCredentials);
  assert.equal(fallback.enabled, true);
  if (fallback.enabled) assert.equal(fallback.deviceToken, 'integration-placeholder-token', 'placeholder token is not a real push credential');
});

test('emitted evidence carries no credential fields (Review Focus #3)', () => {
  const emitted: string[] = [];
  emitDeviceInboxIntegrationEvidence({
    deploymentOrigin: 'https://deployment.example',
    deviceRegister: 'registered',
    deviceTokenTakeover: 'rotated',
    deviceRevoke: 'revoked',
    deviceRevokeAgain: 'not-found',
    inboxRead: 'read',
    inboxUnreadCount: 0,
    markReadIdempotent: 'ok',
    deepLinksObserved: 0,
    commandTimestamp: '2026-09-24T00:00:00.000Z',
  }, (record) => emitted.push(record));
  assert.equal(emitted.length, 1);
  // 匹配的是凭据值与形态（占位 token、账号、密码样例、Bearer 头），而不是泛型字段名词——
  // 证据的合法字段名 deviceTokenTakeover 本身含 'deviceToken' 子串，正则不得误伤自身契约。
  assert.doesNotMatch(emitted[0]!, /short-lived-secret|mobile-test@|password|Bearer\s|placeholder-token/i, 'evidence must stay credential-free');
});
