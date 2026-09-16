import test from 'node:test';
import assert from 'node:assert/strict';
import { connectWithBackoff, PaseoPersonalNodeTransport, PersonalNodeConnector, validateNodeGrant } from './node-connector.ts';

test('old epoch and expired grants cannot operate a node', () => {
  const grant = { targetID: 'n', epoch: 1, expiresAt: 100, operations: ['start'] };
  assert.throws(() => validateNodeGrant(grant, { targetID: 'n', epoch: 2 }, 'start', 50), /NODE_GRANT/);
  assert.throws(() => validateNodeGrant(grant, { targetID: 'n', epoch: 1 }, 'start', 101), /NODE_GRANT/);
  assert.throws(() => validateNodeGrant(grant, { targetID: 'n', epoch: 1 }, 'stop', 50), /NODE_GRANT/);
});

test('connector rejects revoked registrations and closes transport', async () => {
  let closed = 0;
  const client = {
    async createChallenge() { return { challenge_id: 'c1', runtime_id: 'r', external_target_id: 'x', public_key: 'pk', nonce: 'nonce', expires_at: new Date(2_000).toISOString() }; },
    async complete() { return { registration: { id: 'n', runtime_id: 'r', external_target_id: 'x', public_key: 'fp', credential_version: 1, state: 'active' as const }, grant: { targetID: 'n', epoch: 1, expiresAt: 1_000, operations: ['start'] } }; },
    async revoke() {},
  };
  const connector = new PersonalNodeConnector(client, { async send() { return { accepted: true }; }, async close() { closed += 1; } }, { now: () => 500 });
  await connector.enroll({ runtime_id: 'r', external_target_id: 'x', public_key: 'pk', signature: 'sig', idempotency_key: 'i1' });
  assert.deepEqual(await connector.send({ operation: 'start', payload: {} }), { accepted: true });
  await connector.revoke();
  assert.equal(closed, 1);
  await assert.rejects(() => connector.send({ operation: 'start', payload: {} }), /NODE_REVOKED/);
});

test('outbound registration retries with bounded backoff', async () => {
  let attempts = 0;
  const result = await connectWithBackoff(async () => { attempts += 1; if (attempts < 3) throw new Error('offline'); return 'connected'; }, { backoffMs: [0, 0], sleep: async () => {} });
  assert.equal(result, 'connected');
  assert.equal(attempts, 3);
});

test('configured Paseo transport sends ack, heartbeat, logs, and rotates credentials', async () => {
  const calls: Array<{ path: string; auth?: string }> = [];
  const transport = new PaseoPersonalNodeTransport({
    baseURL: 'https://bridge.example.test',
    allowedOrigins: ['https://bridge.example.test'],
    bearer: 'old',
    fetchImpl: async (input, init) => {
      const url = String(input);
      calls.push({ path: new URL(url).pathname, auth: (init?.headers as Record<string, string>)?.authorization });
      return new Response(url.endsWith('/commands') ? JSON.stringify({ accepted: true, commandID: 'ack-1' }) : JSON.stringify({ ok: true }), { status: 200, headers: { 'content-type': 'application/json' } });
    },
  });
  assert.deepEqual(await transport.send({ operation: 'start', payload: { prompt: 'hi' } }, { targetID: 't', epoch: 1, expiresAt: 9_999, operations: ['start'] }), { accepted: true, commandID: 'ack-1' });
  await transport.heartbeat({ nodeID: 'n', credentialVersion: 1, at: 1 });
  await transport.appendLog({ level: 'info', message: 'connected', at: 1 });
  await transport.rotateCredential('new');
  await transport.close();
  assert.deepEqual(calls.map(call => call.path), ['/v1/commands', '/v1/heartbeat', '/v1/logs', '/v1/credentials/rotate', '/v1/close']);
  assert.equal(calls[3].auth, 'Bearer old');
  assert.throws(() => new PaseoPersonalNodeTransport({ baseURL: 'https://evil.example.test', allowedOrigins: ['https://bridge.example.test'] }), /PASEO_ENDPOINT_FORBIDDEN/);
  assert.throws(() => new PaseoPersonalNodeTransport({ baseURL: 'http://daemon.example.test', allowedOrigins: ['http://daemon.example.test'] }), /PASEO_ENDPOINT_FORBIDDEN/);
});
