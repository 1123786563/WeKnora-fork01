import test from 'node:test';
import assert from 'node:assert/strict';
import { deliveryIntegrationConfig } from './delivery-integration-smoke.ts';

test('config is skipped without env and rejected on private hosts', () => {
  assert.deepEqual(deliveryIntegrationConfig({}), { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' });
  const rejected = deliveryIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://127.0.0.1:8080',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
  });
  assert.equal(rejected.enabled, false);
  assert.equal(rejected.disposition, 'invalid');
});

test('config accepts a public https origin and evidence helpers are exported', async () => {
  const smoke = await import('./delivery-integration-smoke.ts');
  const config = smoke.deliveryIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.com',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
  });
  assert.equal(config.enabled, true);
  assert.equal(typeof smoke.emitDeliveryIntegrationEvidence, 'function');
  assert.equal(typeof smoke.runDeliveryIntegration, 'function');
});

test('the recover flag is opt-in and defaults to off', () => {
  const base = {
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.com',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c',
    WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  };
  const off = deliveryIntegrationConfig(base);
  assert.equal(off.enabled === true && off.recover === false, true);
  const on = deliveryIntegrationConfig({ ...base, WEKNORA_MOBILE_TEST_DELIVERY_RECOVER: '1' });
  assert.equal(on.enabled === true && on.recover === true, true);
  assert.equal(deliveryIntegrationConfig({}).enabled, false);
});
