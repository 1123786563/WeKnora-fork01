import assert from 'node:assert/strict';
import test from 'node:test';

import { createEmbedBridge, mapEmbedError, type EmbedHostMessage } from './bridge.ts';

function message(origin: string, source: object, data: EmbedHostMessage): MessageEvent<EmbedHostMessage> {
  return { origin, source, data } as MessageEvent<EmbedHostMessage>;
}

test('pins the parent origin from the referrer and rejects other sources/origins', () => {
  const parent = {};
  const bridge = createEmbedBridge({ parentWindow: parent, referrer: 'https://host.example/page' });

  assert.equal(bridge.accept(message('https://host.example', parent, { source: 'weknora-host', type: 'token', token: 't' })), true);
  assert.equal(bridge.accept(message('https://evil.example', parent, { source: 'weknora-host', type: 'token', token: 't' })), false);
  assert.equal(bridge.accept(message('https://host.example', {}, { source: 'weknora-host', type: 'token', token: 't' })), false);
  assert.equal(bridge.targetOrigin(), 'https://host.example');
});

// Vue postToParent parity (frontend/src/api/embed/index.ts:479-495): the
// handshake (bootstrap_request/ready) may fall back to '*' when the parent
// origin is unknown so token handoff can still bootstrap, while sensitive
// payloads (conversation content) are dropped rather than broadcast.
test('falls back to * for handshake posts when the origin is unknown, never for sensitive posts', () => {
  const parent = {};
  const bridge = createEmbedBridge({ parentWindow: parent });
  const sent: Array<{ payload: unknown; targetOrigin: string }> = [];
  const post = (payload: unknown, targetOrigin: string) => sent.push({ payload, targetOrigin });

  assert.equal(bridge.post({ source: 'weknora-embed', type: 'bootstrap_request', channel_id: 'c1' }, post), true);
  assert.equal(bridge.post({ source: 'weknora-embed', type: 'message_sent', query: 'secret' }, post, { sensitive: true }), false);
  assert.deepEqual(sent, [
    { payload: { source: 'weknora-embed', type: 'bootstrap_request', channel_id: 'c1' }, targetOrigin: '*' },
  ]);
});

test('pins every post to the verified parent origin once known', () => {
  const parent = {};
  const bridge = createEmbedBridge({ parentWindow: parent });
  const sent: Array<{ payload: unknown; targetOrigin: string }> = [];
  const post = (payload: unknown, targetOrigin: string) => sent.push({ payload, targetOrigin });

  assert.equal(bridge.accept(message('https://host.example', parent, { source: 'weknora-host', type: 'context', context: {} })), true);
  assert.equal(bridge.post({ source: 'weknora-embed', type: 'ready' }, post), true);
  assert.equal(bridge.post({ source: 'weknora-embed', type: 'message_received', content: 'answer' }, post, { sensitive: true }), true);
  assert.deepEqual(sent, [
    { payload: { source: 'weknora-embed', type: 'ready' }, targetOrigin: 'https://host.example' },
    { payload: { source: 'weknora-embed', type: 'message_received', content: 'answer' }, targetOrigin: 'https://host.example' },
  ]);
});

test('rejects an invalid configured parent origin', () => {
  const bridge = createEmbedBridge({ parentWindow: {}, parentOrigin: '*' });
  assert.equal(bridge.targetOrigin(), null);
});

test('maps known Vue bridge failures to stable recoverable states', () => {
  assert.deepEqual(mapEmbedError(new Error('embed session exchange returned no token')), { kind: 'exchange', retryable: true });
  assert.deepEqual(mapEmbedError(new Error('channel disabled')), { kind: 'disabled', retryable: false });
  assert.deepEqual(mapEmbedError(new Error('failed to create session')), { kind: 'session', retryable: true });
  assert.deepEqual(mapEmbedError(new Error('attachment read failed')), { kind: 'attachment', retryable: true });
  assert.deepEqual(mapEmbedError(new Error('anything else')), { kind: 'unknown', retryable: true });
});
