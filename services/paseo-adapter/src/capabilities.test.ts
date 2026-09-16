import test from 'node:test';
import assert from 'node:assert/strict';

import {
  probeCapabilities,
  requireCore,
  type PaseoCapabilities,
} from './capabilities.ts';

test('daemon without observable execution is not an admitted target', () => {
  const capabilities: PaseoCapabilities = {
    create: true,
    observe: false,
    events: true,
    cancel: true,
    approval: false,
    steer: false,
    exportArtifact: false,
    lookupByRequest: false,
  };

  assert.throws(() => requireCore(capabilities), /PASEO_CORE_UNAVAILABLE/);
});

test('capability probe preserves supported, unavailable, and forbidden states', () => {
  assert.deepEqual(
    probeCapabilities({
      create: { state: 'supported', method: 'PaseoApi.agents.create' },
      observe: { state: 'supported', method: 'PaseoAgentHandle.waitForFinish' },
      events: { state: 'supported', method: 'PaseoAgentHandle.subscribe' },
      cancel: { state: 'supported', method: 'DaemonClient.cancelAgent' },
      approval: { state: 'forbidden', reason: 'daemon policy forbids interactive approval' },
      steer: { state: 'unavailable', reason: 'SDK does not expose steering' },
      exportArtifact: { state: 'unavailable', reason: 'artifact export is a product concern' },
      lookupByRequest: { state: 'unavailable', reason: 'daemon has no request lookup endpoint' },
    }),
    {
      create: { state: 'supported', method: 'PaseoApi.agents.create' },
      observe: { state: 'supported', method: 'PaseoAgentHandle.waitForFinish' },
      events: { state: 'supported', method: 'PaseoAgentHandle.subscribe' },
      cancel: { state: 'supported', method: 'DaemonClient.cancelAgent' },
      approval: { state: 'forbidden', reason: 'daemon policy forbids interactive approval' },
      steer: { state: 'unavailable', reason: 'SDK does not expose steering' },
      exportArtifact: { state: 'unavailable', reason: 'artifact export is a product concern' },
      lookupByRequest: { state: 'unavailable', reason: 'daemon has no request lookup endpoint' },
    },
  );
});

test('core requirement rejects every missing core capability', () => {
  for (const key of ['create', 'observe', 'events', 'cancel'] as const) {
    const capabilities: PaseoCapabilities = {
      create: true,
      observe: true,
      events: true,
      cancel: true,
      approval: false,
      steer: false,
      exportArtifact: false,
      lookupByRequest: false,
    };
    capabilities[key] = false;
    assert.throws(() => requireCore(capabilities), /PASEO_CORE_UNAVAILABLE/);
  }
});
