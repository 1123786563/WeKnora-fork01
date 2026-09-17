import test from 'node:test';
import assert from 'node:assert/strict';
// W37 carry-forward ①: the compatibility window must be reachable through the
// package surface mobile actually imports (`@weknora/domain/mobile`).
// Before this wiring the re-export was missing, so every consumer would have
// had to reach into the file path directly — the W36 review recorded that as
// an unreleased gap (protocolMode existed with zero production consumers).
import { protocolMode, clientGate, CLIENT_PROTOCOL_VERSION } from './index.ts';

test('protocolMode is re-exported through the mobile domain surface', () => {
  assert.equal(protocolMode(1, 2, 3), 'upgrade_required');
  assert.equal(protocolMode(4, 2, 3), 'server_upgrade_required');
  assert.equal(protocolMode(3, 2, 3), 'full');
});

test('clientGate and the build protocol generation are re-exported for the app handshake', () => {
  const verdict = clientGate(CLIENT_PROTOCOL_VERSION, { protocol_minimum: 2, protocol_maximum: 3 });
  assert.equal(verdict.mode, 'full');
  assert.equal(verdict.controlCommandsAllowed, true);
});
