import test from 'node:test';
import assert from 'node:assert/strict';
import { legacyTasksIntegrationConfig, runLegacyTasksIntegration } from './legacy-tasks-integration-smoke.ts';

const env = () => ({ ...process.env }) as Record<string, string | undefined>;

test('the legacy smoke stays opt-in: missing credentials skip, malformed origin is rejected', () => {
  assert.equal(legacyTasksIntegrationConfig({}).enabled, false);
  assert.equal(legacyTasksIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.test' }).enabled, false);
  const invalid = legacyTasksIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://weknora.example.test',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'x',
  });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.enabled === false && invalid.disposition, 'invalid');
});

test('the legacy smoke rejects loopback, private and reserved deployment hosts as invalid', () => {
  for (const host of ['https://localhost', 'https://127.0.0.1:8080', 'https://10.0.0.5', 'https://192.168.1.4', 'https://[fe80::1]', 'https://169.254.1.1']) {
    const verdict = legacyTasksIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host,
      WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
    });
    assert.equal(verdict.enabled, false, host);
    assert.equal(verdict.enabled === false && verdict.disposition, 'invalid', host);
  }
  const verdict = legacyTasksIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.org',
    WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  });
  assert.equal(verdict.enabled, true);
});

test('the legacy smoke runs against a real deployment when credentials are supplied', async (t) => {
  const config = legacyTasksIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(`missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD: ${config.enabled === false ? config.disposition : ''}`);
    return;
  }
  const evidence = await runLegacyTasksIntegration(config);
  assert.equal(evidence.legacyList, 'loaded', 'the legacy projection must load over the real wire');
  assert.equal(evidence.probeCreated, true, 'the probe session must have been created');
  assert.ok(['submitted', 'unavailable', 'failed'].includes(evidence.followUp), 'follow-up outcome is recorded honestly');
  assert.ok(['loaded', 'unavailable', 'failed'].includes(evidence.history));
  assert.ok(typeof evidence.commandTimestamp === 'string' && evidence.commandTimestamp !== '');
});
