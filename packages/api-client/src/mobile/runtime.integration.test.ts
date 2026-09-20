import test from 'node:test';
import assert from 'node:assert/strict';
import { mobileRuntimeIntegrationConfig, runMobileRuntimeIntegration } from '../../../../apps/mobile/src/runtime-integration-smoke.ts';

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
    t.skip(`MOBILE_RUNTIME_HTTP_SKIPPED: ${config.reason}`);
    return;
  }

  const evidence = await runMobileRuntimeIntegration(config);

  assert.equal(evidence.outcome, 'authorized');
  assert.equal(evidence.capabilityMode, 'compatible');
  assert.equal(evidence.identity, 'present', 'real /auth/me must produce stable user and tenant identities');
  t.diagnostic(JSON.stringify(evidence));
});
