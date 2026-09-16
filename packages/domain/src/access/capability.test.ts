import assert from 'node:assert/strict';
import test from 'node:test';

import { isCapabilitySupported, normalizeCapabilityMap } from './capability.ts';

test('normalizes auth/me capability descriptors without failing open on malformed entries', () => {
  assert.deepEqual(normalizeCapabilityMap({
    organizations: { supported: false, reason: 'lite' },
    agents: true,
    malformed: { supported: 'yes' },
  }), {
    organizations: { supported: false, reason: 'lite' },
    agents: { supported: true },
  });
});

test('keeps legacy capabilities visible unless explicitly disabled and hides Lite organizations', () => {
  assert.equal(isCapabilitySupported({}, 'agents'), true);
  assert.equal(isCapabilitySupported({ organizations: { supported: true } }, 'organizations', { edition: 'lite' }), false);
  assert.equal(isCapabilitySupported({}, 'settings.sandbox.docker'), false);
});
