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
  assert.equal(evidence.tenantSwitch, config.switchTenantId ? 'switched' : 'skipped');
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
    tenantSwitch: 'skipped',
    commandTimestamp: '2026-09-21T00:00:00.000Z',
  }, (record) => emitted.push(record));

  assert.equal(emitted.length, 1);
  assert.deepEqual(JSON.parse(emitted[0]!), {
    deploymentOrigin: 'https://deployment.example',
    clientProtocol: 3,
    capabilityMode: 'incompatible',
    identity: 'absent',
    outcome: 'not-authorized',
    tenantSwitch: 'skipped',
    commandTimestamp: '2026-09-21T00:00:00.000Z',
  });
  assert.doesNotMatch(emitted[0]!, /short-lived-secret|password|token|email/i);
});

/** Not a credential: the config validator only requires a non-empty password value. */
const nonEmptyPassword = process.env.WEKNORA_MOBILE_TEST_PASSWORD ?? 'x';

test('integration config rejects a non-numeric switch tenant id as invalid rather than skippable', () => {
  const config = mobileRuntimeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: nonEmptyPassword,
    WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID: 'abc',
  });

  assert.deepEqual(config, {
    enabled: false,
    disposition: 'invalid',
    reason: 'WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID must be a positive integer tenant id',
  });
});

test('integration config rejects loopback, private, and reserved deployment hosts', () => {
  const rejected = [
    'https://localhost',
    'https://sub.localhost',
    'https://127.0.0.1',
    'https://10.0.0.2',
    'https://172.16.0.9',
    'https://192.168.1.10',
    'https://169.254.1.1',
    'https://0.0.0.0',
    'https://240.0.0.1',
    // IPv6 loopback, unspecified, IPv4-mapped loopback/private, IPv4-compatible private.
    'https://[::1]',
    'https://[::]',
    'https://[::ffff:127.0.0.1]',
    'https://[::ffff:7f00:1]',
    'https://[::ffff:10.0.0.2]',
    'https://[::10.0.0.2]',
    // Link-local fe80::/10 and zone-scoped link-local.
    'https://[fe80::1]',
    'https://[febf::1]',
    // ULA-private fc00::/7 (both fc00:: and fd00:: halves).
    'https://[fc00::1]',
    'https://[fd12:3456:789a::1]',
    // Multicast, documentation, Teredo, discard-only, and non-global-unicast space.
    'https://[ff02::1]',
    'https://[2001:db8::1]',
    'https://[2001::1]',
    'https://[100::1]',
    'https://[4000::1]',
    // 6to4 embedding a private IPv4 host (172.16.0.9 = ac10:9).
    'https://[2002:ac10:9::1]',
  ];
  for (const url of rejected) {
    const config = mobileRuntimeIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: url,
      WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: nonEmptyPassword,
    });

    assert.equal(config.enabled, false, `${url} must not enable a real HTTP run`);
    assert.equal(config.disposition, 'invalid', `${url} is invalid, not skippable`);
  }
});

test('integration config keeps a public IPv6 deployment host eligible for a real HTTP run', () => {
  const config = mobileRuntimeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://[2606:4700::6810:84e5]',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: nonEmptyPassword,
  });

  assert.equal(config.enabled, true, 'a global-unicast IPv6 literal must not be over-blocked');
});
