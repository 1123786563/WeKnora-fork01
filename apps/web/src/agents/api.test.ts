import assert from 'node:assert/strict';
import test from 'node:test';
import { loadAgents, saveAgent, type AgentRequestClient } from './api.ts';

test('loads list response and does not turn an empty list into an error', async () => {
  const calls: unknown[] = [];
  const client: AgentRequestClient = { request: async (input) => { calls.push(input); return { data: [] }; } };
  assert.deepEqual(await loadAgents(client), { status: 'success', items: [] });
  assert.deepEqual(calls, [{ method: 'GET', path: '/api/v1/agents' }]);
});

test('preserves backend error code for permission and transport feedback', async () => {
  const client: AgentRequestClient = { request: async () => { throw Object.assign(new Error('Forbidden'), { code: 'FORBIDDEN' }); } };
  assert.deepEqual(await loadAgents(client), { status: 'error', code: 'FORBIDDEN', message: 'Forbidden' });
});

test('sends the complete editable payload and reports mutation errors', async () => {
  const calls: unknown[] = [];
  const client: AgentRequestClient = { request: async (input) => { calls.push(input); return { data: { id: 'a-1', name: 'Updated', is_builtin: false } }; } };
  const result = await saveAgent(client, 'a-1', { name: 'Updated', description: 'd', config: { agent_mode: 'quick-answer' } });
  assert.deepEqual(result, { status: 'success', item: { id: 'a-1', name: 'Updated', is_builtin: false } });
  assert.deepEqual(calls, [{ method: 'PUT', path: '/api/v1/agents/a-1', body: { name: 'Updated', description: 'd', config: { agent_mode: 'quick-answer' } } }]);
});
