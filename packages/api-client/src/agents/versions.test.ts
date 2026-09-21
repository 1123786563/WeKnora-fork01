import assert from 'node:assert/strict';
import test from 'node:test';
import type { ClientRequest } from '../client.ts';
import { createAgentVersionsApi } from './versions.ts';

const version = { id: 'version-1', agent_id: 'agent/1', version_number: 2, source_sha256: 'a'.repeat(64), frozen_by: 'user-1', created_at: '2026-09-21T00:00:00Z' };

test('freezes and reads AgentVersions through the registered routes', async () => {
  const calls: ClientRequest[] = [];
  const api = createAgentVersionsApi(async (call) => { calls.push(call); return { success: true, data: version }; });
  await api.freezeVersion('agent/1');
  await api.getVersion('agent/1', 'version 1');
  assert.deepEqual(calls, [
    { method: 'POST', path: '/api/v1/agents/agent%2F1/versions' },
    { method: 'GET', path: '/api/v1/agents/agent%2F1/versions/version%201' },
  ]);
});

test('rejects malformed AgentVersion responses', async () => {
  const api = createAgentVersionsApi(async () => ({ success: true, data: { ...version, source_sha256: '' } }));
  await assert.rejects(api.freezeVersion('agent-1'));
});
