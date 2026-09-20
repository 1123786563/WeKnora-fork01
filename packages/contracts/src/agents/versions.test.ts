import assert from 'node:assert/strict';
import test from 'node:test';
import { ContractError } from '../index.ts';
import { parseAgentVersion, parseAgentVersionListResponse } from './versions.ts';

const version = { id: 'version-1', agent_id: 'agent-1', version_number: 2, source_sha256: 'a'.repeat(64), frozen_by: 'user-1', created_at: '2026-09-21T00:00:00Z' };

test('parses an AgentVersion with required immutable identifiers and digest', () => {
  assert.deepEqual(parseAgentVersion({ success: true, data: version }), version);
  assert.equal(parseAgentVersion({ success: true, data: { ...version, frozen_by: '' } }).frozen_by, '');
});

test('rejects an AgentVersion without its source digest', () => {
  assert.throws(() => parseAgentVersion({ success: true, data: { ...version, source_sha256: '' } }), ContractError);
});

test('parses the version list envelope and rejects malformed envelopes', () => {
  assert.deepEqual(parseAgentVersionListResponse({ success: true, data: [version] }), [version]);
  assert.throws(() => parseAgentVersionListResponse({ success: true, data: version }), ContractError);
  assert.throws(() => parseAgentVersion({ success: false, data: version }), ContractError);
});
