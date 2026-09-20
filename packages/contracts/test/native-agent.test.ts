import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import {
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

  assert.equal(parsed.last_event_id.sequence, 9007199254740993n);
  assert.equal(parsed.events[0]?.tenant_id, '9007199254740993');
  assert.equal(parsed.events[4]?.payload.usage?.prompt_tokens, '9007199254740993');
  assert.deepEqual(parsed.events[3], (wire.events as unknown[])[3]);
  assert.deepEqual(parsed.pending, wire.pending);
  assert.deepEqual(parsed.command_errors, wire.command_errors);
  assert.deepEqual(parsed.archive, wire.archive);
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
