import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import {
  NATIVE_AGENT_EVENT_KINDS,
  parseLastEventID,
  parseNativeAgentFixture,
  parseNativeEvent,
  parseSequence,
} from '../src/chat/native.ts';
import { ContractError } from '../src/index.ts';

const fixturePath = new URL('../../../tests/native-agent/wire-v1.json', import.meta.url);

async function fixture(): Promise<Record<string, unknown>> {
  return JSON.parse(await readFile(fixturePath, 'utf8')) as Record<string, unknown>;
}

test('parses the canonical v1 fixture without losing decimal counters or public wire fields', async () => {
  const wire = await fixture();
  const parsed = parseNativeAgentFixture(wire);

  assert.equal(parsed.last_event_id.sequence, 13n);
  assert.equal(parsed.events[0]?.tenant_id, '9007199254740993');
  const usage = parsed.events.find((event): event is Extract<typeof event, { kind: 'usage.observed' }> => event.kind === 'usage.observed');
  assert.equal(usage?.payload.usage.prompt_tokens, '9007199254740993');
  assert.deepEqual(parsed.events[3], (wire.events as unknown[])[3]);
  assert.deepEqual(parsed.pending, wire.pending);
  assert.deepEqual(parsed.command_errors, wire.command_errors);
  assert.deepEqual(parsed.archive, wire.archive);
});

test('fixture contains exactly the authoritative twelve public event kinds', async () => {
  const wire = await fixture();
  const kinds = new Set((wire.events as Record<string, unknown>[]).map((event) => event.kind));
  assert.deepEqual([...kinds].sort(), [...NATIVE_AGENT_EVENT_KINDS].sort());
  assert.equal(kinds.size, 12);
});

test('preserves sequences above JavaScript safe integer range', () => {
  assert.equal(parseSequence('9007199254740993'), 9007199254740993n);
  for (const value of ['0', '-1', '1.1', '01', '9223372036854775808']) {
    assert.throws(() => parseSequence(value), (error: unknown) => error instanceof ContractError && error.path === 'seq');
  }
});

test('rejects incompatible protocol and schema versions', async () => {
  const wire = await fixture();
  const event = (wire.events as Record<string, unknown>[])[0]!;
  assert.throws(() => parseNativeEvent({ ...event, protocol: 'weknora.agent.v2' }), /protocol/);
  assert.throws(() => parseNativeEvent({ ...event, schema_version: 2 }), /schema_version/);
});

test('rejects malformed native events before a reducer can consume them', async () => {
  const wire = await fixture();
  const event = (wire.events as Record<string, unknown>[])[0]!;
  assert.throws(() => parseNativeEvent({ ...event, run_id: '' }), /run_id/);
  assert.throws(() => parseNativeEvent({ ...event, seq: '1.1' }), /seq/);
  assert.throws(() => parseNativeEvent({ ...event, kind: 'sdk.internal' }), /kind/);
});

test('rejects private and unknown fields from every public event projection', async () => {
  const wire = await fixture();
  const waiting = (wire.events as Record<string, unknown>[])[0]!;
  assert.throws(() => parseNativeEvent({ ...waiting, payload: { ...(waiting.payload as Record<string, unknown>), status: 'sdk.internal' } }), /payload.status/);
  assert.throws(() => parseNativeEvent({ ...waiting, payload: { ...(waiting.payload as Record<string, unknown>), provider_receipt: 'private' } }), /payload.provider_receipt/);
  for (const event of wire.events as Record<string, unknown>[]) {
    assert.throws(() => parseNativeEvent({ ...event, payload: { ...(event.payload as Record<string, unknown>), private_field: true } }), /payload.private_field/);
  }
});

test('requires a complete public decision reference and a decimal artifact size', () => {
  const decision = {
    protocol: 'weknora.agent.v1', schema_version: 1, event_id: 'decision', tenant_id: '1', session_id: 'session', run_id: 'run', attempt_id: 'attempt', seq: '1', kind: 'decision.required',
    payload: { call_id: 'call', plan_version: 1, args_hash: 'sha256:args', expires_at: '2026-09-20T12:00:00Z', wait_kind: 'tool_approval' },
  };
  assert.throws(() => parseNativeEvent(decision), /payload.pending/);
  const artifact = {
    protocol: 'weknora.agent.v1', schema_version: 1, event_id: 'artifact', tenant_id: '1', session_id: 'session', run_id: 'run', seq: '2', kind: 'artifact.available',
    payload: { artifact: { id: 'a', media_type: 'text/plain', sha256: 'sha256:a', size_bytes: 42 } },
  };
  assert.throws(() => parseNativeEvent(artifact), /size_bytes/);
});

test('validates the scoped v1 Last-Event-ID representation', () => {
  assert.deepEqual(parseLastEventID('v1:cnVuLTE:7'), { run_id: 'run-1', sequence: 7n });
  for (const value of ['v2:cnVuLTE:7', 'v1:not-base64!:7', 'v1:cnVuLTE:0', 'v1:cnVuLTE:7:extra']) {
    assert.throws(() => parseLastEventID(value), /last_event_id/);
  }
});

test('rejects OAuth details that could contain a redirect credential or unredacted parameters', async () => {
  const wire = await fixture();
  const pending = wire.pending as Record<string, unknown>;
  assert.throws(
    () => parseNativeAgentFixture({ ...wire, pending: { ...pending, oauth: { ...(pending.oauth as Record<string, unknown>), authorization_url: 'https://provider.example/token' } } }),
    /pending.oauth.authorization_url/,
  );
  assert.throws(
    () => parseNativeAgentFixture({ ...wire, pending: { ...pending, redacted_paths: [], redacted_args: { token: 'secret' } } }),
    /pending.redacted_args/,
  );
});
