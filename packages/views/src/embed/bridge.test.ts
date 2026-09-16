import assert from 'node:assert/strict';
import test from 'node:test';

import { createEmbedBridgeGuard, EMBED_HOST_SOURCE } from './bridge.ts';

test('pins the first valid parent origin and rejects other windows/origins', () => {
  const guard = createEmbedBridgeGuard('https://known.example');
  assert.equal(guard.accept({ sourceIsParent: true, origin: 'https://known.example', data: { source: EMBED_HOST_SOURCE, type: 'provide_token' } }), true);
  assert.equal(guard.accept({ sourceIsParent: false, origin: 'https://known.example', data: { source: EMBED_HOST_SOURCE, type: 'provide_token' } }), false);
  assert.equal(guard.accept({ sourceIsParent: true, origin: 'https://evil.example', data: { source: EMBED_HOST_SOURCE, type: 'provide_token' } }), false);
  assert.equal(guard.targetOrigin(), 'https://known.example');
  assert.equal(guard.canSendSensitive(), true);
});

test('trust-on-first-use only accepts host-tagged messages and keeps sensitive output blocked before pinning', () => {
  const guard = createEmbedBridgeGuard();
  assert.equal(guard.canSendSensitive(), false);
  assert.equal(guard.accept({ sourceIsParent: true, origin: 'null', data: { source: EMBED_HOST_SOURCE, type: 'set_context' } }), false);
  assert.equal(guard.accept({ sourceIsParent: true, origin: 'https://host.example', data: { source: 'other', type: 'set_context' } }), false);
  assert.equal(guard.accept({ sourceIsParent: true, origin: 'https://host.example', data: { source: EMBED_HOST_SOURCE, type: 'set_context' } }), true);
  assert.equal(guard.targetOrigin(), 'https://host.example');
  assert.equal(guard.canSendSensitive(), true);
});
