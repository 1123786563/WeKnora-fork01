import test from 'node:test';
import assert from 'node:assert/strict';
import { emitMobileRuntimeIntegrationEvidence, mobileRuntimeIntegrationConfig, runMobileRuntimeIntegration } from '../../../../apps/mobile/src/runtime-integration-smoke.ts';

/**
 * Opt-in real HTTP check.  It never supplies fallback credentials or a mock:
 *
 * WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
 * WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
 * WEKNORA_MOBILE_TEST_PASSWORD=short-lived-secret \
 * pnpm exec tsx --test packages/api-client/src/mobile/runtime.integration.test.ts
 *
 * Evidence is emitted as JSON by this test and contains only: deployment
 * origin, client protocol, capability mode, whether stable user/tenant IDs
 * were present, outcome, and command timestamp. Tokens and credentials are
 * never emitted.
 */
test('real HTTP login reaches identity, capabilities, and an authorized Runtime surface', async (t) => {
  const config = mobileRuntimeIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`MOBILE_RUNTIME_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`MOBILE_RUNTIME_HTTP_INVALID: ${config.reason}`);
    return;
  }

  const evidence = await runMobileRuntimeIntegration(config);
  emitMobileRuntimeIntegrationEvidence(evidence, (record) => t.diagnostic(record));

  assert.equal(evidence.outcome, 'authorized');
  assert.equal(evidence.capabilityMode, 'compatible');
  assert.equal(evidence.identity, 'present', 'real /auth/me must produce stable user and tenant identities');
});

test('integration config marks a path-bearing deployment URL invalid rather than skippable', () => {
  const config = mobileRuntimeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example/api/v1',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'short-lived-secret',
  });

  assert.deepEqual(config, {
    enabled: false,
    disposition: 'invalid',
    reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin',
  });
});

test('non-authorized evidence is emitted before assertions without credential fields', () => {
  const emitted: string[] = [];
  emitMobileRuntimeIntegrationEvidence({
    deploymentOrigin: 'https://deployment.example',
    clientProtocol: 3,
    capabilityMode: 'incompatible',
    identity: 'absent',
    outcome: 'not-authorized',
    commandTimestamp: '2026-09-21T00:00:00.000Z',
  }, (record) => emitted.push(record));

  assert.equal(emitted.length, 1);
  assert.deepEqual(JSON.parse(emitted[0]!), {
    deploymentOrigin: 'https://deployment.example',
    clientProtocol: 3,
    capabilityMode: 'incompatible',
    identity: 'absent',
    outcome: 'not-authorized',
    commandTimestamp: '2026-09-21T00:00:00.000Z',
  });
  assert.doesNotMatch(emitted[0]!, /short-lived-secret|password|token|email/i);
});
