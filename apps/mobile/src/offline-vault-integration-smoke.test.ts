import test from 'node:test';
import assert from 'node:assert/strict';
import { emitOfflineVaultIntegrationEvidence, offlineVaultIntegrationConfig, runOfflineVaultIntegration } from './offline-vault-integration-smoke.ts';

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip, never fake a pass', () => {
  const config = offlineVaultIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
  const invalid = offlineVaultIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://insecure.example', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
  const loopback = offlineVaultIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://127.0.0.1', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(loopback.enabled, false);
  assert.equal(loopback.disposition, 'invalid');
});

test('live end-to-end offline cache, gate and confirmation through the highest stable interface (opt-in)', async (t) => {
  const config = offlineVaultIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runOfflineVaultIntegration(config);
  const emitted: string[] = [];
  emitOfflineVaultIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).onlineStart, evidence.onlineStart);
  assert.equal(evidence.onlineStart, 'admitted', 'a live deployment with one agent must admit the online goal');
  assert.deepEqual(evidence.offlineBlockedActions.sort(), ['approval', 'budget', 'external-action', 'run'], 'AC2: all four dangerous action kinds are refused offline');
  assert.equal(evidence.offlineDraftRoundTrip, true, 'offline drafts round-trip through the scoped vault');
  assert.equal(evidence.offlineDetailView, 'projected', 'AC1: the task detail degrades to the encrypted projection while offline');
  assert.equal(evidence.ciphertextProjectionRows, true, 'vault rows must not contain plaintext task ids');
  assert.equal(evidence.onlineResync, 'recovered', 'an explicit resync recovers the authoritative view once back online');
});

test('the integration runner is total: a failing transport still yields evidence, not a rejection', async () => {
  // RFC 2606 保留 TLD（.invalid）永不解析：等价 transport 层故障，不触达任何真实部署。
  const config = offlineVaultIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://integration-smoke-unreachable.invalid',
    WEKNORA_MOBILE_TEST_EMAIL: 'e',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
  });
  assert.equal(config.enabled, true);
  const evidence = await runOfflineVaultIntegration(config); // 行为断言：await 不 reject
  assert.equal(evidence.deploymentOrigin, 'https://integration-smoke-unreachable.invalid');
  assert.equal(evidence.onlineStart, 'failed');
  assert.equal(typeof evidence.errorReason, 'string');
  const emitted: string[] = [];
  emitOfflineVaultIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).onlineStart, 'failed');
});
