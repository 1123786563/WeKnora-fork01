// R465-A2 — deployment-capability gating for the React command palette.
// Vue contract:
//   • frontend/src/config/deploymentCapabilities.ts
//     isDeploymentCapabilitySupported(): fail-open semantics — a missing or
//     failed capability probe keeps entries VISIBLE; only an explicit
//     `supported: false` hides them. The `organizations` key additionally
//     returns false on lite deployments (liteMode flag or edition 'lite').
//   • frontend/src/components/GlobalCommandPalette.vue passes
//     `agentsEnabled: deploymentCapabilities.isSupported('agents')` into
//     useCmdkSearch (agent search group) and filters the open-agents /
//     open-organizations quick actions through the same store.
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isPaletteCapabilitySupported,
  paletteAccessFromCapabilities,
  type PaletteDeploymentCapabilities,
} from './deployment-capabilities.ts';

const caps = (entries: Record<string, { supported: boolean }>): PaletteDeploymentCapabilities => ({
  edition: '',
  capabilities: entries,
});

test('missing capability entries stay visible (fail-open like Vue)', () => {
  assert.equal(isPaletteCapabilitySupported(caps({}), 'agents'), true);
  assert.equal(isPaletteCapabilitySupported({ edition: '', capabilities: {} }, 'organizations'), true);
});

test('an explicit supported:false hides the entry', () => {
  assert.equal(isPaletteCapabilitySupported(caps({ agents: { supported: false } }), 'agents'), false);
  assert.equal(isPaletteCapabilitySupported(caps({ organizations: { supported: false } }), 'organizations'), false);
});

test('organizations is forced off for lite deployments even when the probe says supported', () => {
  const probed = caps({ organizations: { supported: true } });
  assert.equal(isPaletteCapabilitySupported(probed, 'organizations', { liteMode: true }), false);
  assert.equal(isPaletteCapabilitySupported({ ...probed, edition: 'Lite' }, 'organizations'), false);
  assert.equal(isPaletteCapabilitySupported({ ...probed, edition: 'lite' }, 'organizations'), false);
  // Non-lite editions honor the probe result.
  assert.equal(isPaletteCapabilitySupported({ ...probed, edition: 'pro' }, 'organizations'), true);
});

test('agents is NOT lite-gated (only the probe result decides)', () => {
  const probed = caps({ agents: { supported: true } });
  assert.equal(isPaletteCapabilitySupported(probed, 'agents', { liteMode: true }), true);
});

test('paletteAccessFromCapabilities: null probe fail-opens both entries', () => {
  assert.deepEqual(
    paletteAccessFromCapabilities(null, { liteMode: false, isAdmin: false }),
    { canOpenAgents: true, canOpenOrganizations: false },
  );
});

test('paletteAccessFromCapabilities: organizations needs admin role AND the capability', () => {
  const probed = caps({ agents: { supported: false }, organizations: { supported: true } });
  assert.deepEqual(
    paletteAccessFromCapabilities(probed, { liteMode: false, isAdmin: false }),
    { canOpenAgents: false, canOpenOrganizations: false },
  );
  assert.deepEqual(
    paletteAccessFromCapabilities(probed, { liteMode: false, isAdmin: true }),
    { canOpenAgents: false, canOpenOrganizations: true },
  );
  assert.deepEqual(
    paletteAccessFromCapabilities(probed, { liteMode: true, isAdmin: true }),
    { canOpenAgents: false, canOpenOrganizations: false },
  );
});
