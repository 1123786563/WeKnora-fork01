import test from 'node:test';
import assert from 'node:assert/strict';
import {
  blindPushIntegrationConfig,
  emitBlindPushIntegrationEvidence,
  runBlindPushIntegration,
} from './blind-push-integration-smoke.ts';

/**
 * Opt-in 真实 HTTP 检查（#67 AC1 客户端观察 + AC2——真实 JSON transport + 真实 Runtime +
 * 真实服务端注册面/inbox 读）。无回退凭据：
 *
 * WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
 * WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
 * WEKNORA_MOBILE_TEST_PASSWORD=<short-lived-secret> \
 * [WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID=enterprise:acme] \
 * pnpm --filter @weknora/mobile test
 *
 * 占位 token 不是可用凭据（无 APNs/FCM 真实效力）；凭据一律来自环境变量。
 */
test('real HTTP official/enterprise registration isolation and foreground sync through the wire', async (t) => {
  const config = blindPushIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`BLIND_PUSH_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`BLIND_PUSH_HTTP_INVALID: ${config.reason}`);
    return;
  }
  const evidence = await runBlindPushIntegration(config);
  emitBlindPushIntegrationEvidence(evidence, (record) => t.diagnostic(record));
  assert.equal(evidence.officialRegister, 'registered', 'the official two-step registration must bind');
  assert.equal(evidence.foregroundSync, 'synced', 'foreground authoritative sync works with no push involvement');
  if (config.enterpriseAppId !== undefined) {
    assert.equal(evidence.enterpriseRegister, 'registered', 'the declared enterprise app must register under its own identity');
    assert.equal(evidence.isolation, 'isolated', 'official and enterprise registrations must not mix');
  } else {
    assert.ok(
      evidence.enterpriseRegister === 'rejected-by-policy' || evidence.enterpriseRegister === 'registered',
      'an undeclared probe is either policy-rejected (400) or registered on a deployment that allows it—both are honest observations',
    );
  }
});

test('blind-push config reuses the device-inbox credentials and host guard', () => {
  assert.deepEqual(
    blindPushIntegrationConfig({}),
    { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' },
  );
  const withCredentials = {
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  };
  for (const url of ['https://127.0.0.1', 'https://10.0.0.2', 'https://localhost']) {
    const config = blindPushIntegrationConfig({ ...withCredentials, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: url });
    assert.equal(config.enabled, false, `${url} must not enable a real HTTP run`);
    assert.equal(config.disposition, 'invalid');
  }
  const enabled = blindPushIntegrationConfig(withCredentials);
  assert.equal(enabled.enabled, true);
  const enterprise = blindPushIntegrationConfig({ ...withCredentials, WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID: 'enterprise:acme' });
  assert.equal(enterprise.enabled, true);
  if (enterprise.enabled) assert.equal(enterprise.enterpriseAppId, 'enterprise:acme');
  const malformed = blindPushIntegrationConfig({ ...withCredentials, WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID: 'enterprise:Acme' });
  assert.equal(malformed.enabled, true, 'a malformed enterprise id degrades to official-only probing, still enabled');
});

test('emitted evidence carries no credential fields', () => {
  const emitted: string[] = [];
  emitBlindPushIntegrationEvidence({
    deploymentOrigin: 'https://deployment.example',
    officialRegister: 'registered',
    enterpriseRegister: 'rejected-by-policy',
    isolation: 'unverified',
    foregroundSync: 'synced',
    inboxUnreadCount: 0,
    commandTimestamp: '2026-09-24T00:00:00.000Z',
  }, (record) => emitted.push(record));
  assert.equal(emitted.length, 1);
  assert.doesNotMatch(emitted[0]!, /short-lived-secret|mobile-test@|password|Bearer\s/i, 'evidence must stay credential-free');
});
