import test from 'node:test';
import assert from 'node:assert/strict';

import {
  createPinnedPaseoClient,
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
      cancel: { state: 'unavailable', reason: 'DaemonClient.cancelAgent is an internal SDK surface' },
      approval: { state: 'forbidden', reason: 'daemon policy forbids interactive approval' },
      steer: { state: 'unavailable', reason: 'SDK does not expose steering' },
      exportArtifact: { state: 'unavailable', reason: 'artifact export is a product concern' },
      lookupByRequest: { state: 'unavailable', reason: 'daemon has no request lookup endpoint' },
    }),
    {
      create: { state: 'supported', method: 'PaseoApi.agents.create' },
      observe: { state: 'supported', method: 'PaseoAgentHandle.waitForFinish' },
      events: { state: 'supported', method: 'PaseoAgentHandle.subscribe' },
      cancel: { state: 'unavailable', reason: 'DaemonClient.cancelAgent is an internal SDK surface' },
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

test('internal cancel cannot satisfy core admission', () => {
  const probed = probeCapabilities({
    create: { state: 'supported', method: 'PaseoApi.agents.create' },
    observe: { state: 'supported', method: 'PaseoAgentHandle.waitForFinish' },
    events: { state: 'supported', method: 'PaseoAgentHandle.subscribe' },
    cancel: { state: 'supported', method: 'DaemonClient.cancelAgent' },
    approval: { state: 'unavailable', reason: 'no public approval operation' },
    steer: { state: 'unavailable', reason: 'no public steer operation' },
    exportArtifact: { state: 'unavailable', reason: 'product boundary' },
    lookupByRequest: { state: 'unavailable', reason: 'no public lookup operation' },
  });

  assert.equal(probed.cancel.state, 'unavailable');
  assert.throws(() => requireCore({
    create: true,
    observe: true,
    events: true,
    cancel: false,
    approval: false,
    steer: false,
    exportArtifact: false,
    lookupByRequest: false,
  }), /PASEO_CORE_UNAVAILABLE:cancel/);
});

test('pinned SDK exposes only the public create, observe, and event mapping', async () => {
  const client = createPinnedPaseoClient({
    url: 'ws://127.0.0.1:1/ws',
    reconnect: { enabled: false },
  });
  assert.equal(typeof client.agents.create, 'function');
  const agent = client.agents.ref('probe-agent');
  assert.equal(typeof agent.waitForFinish, 'function');
  assert.equal(typeof agent.subscribe, 'function');
  assert.equal('cancel' in agent, false);
  await client.close();
});
