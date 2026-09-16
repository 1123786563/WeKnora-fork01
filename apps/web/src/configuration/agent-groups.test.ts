import assert from 'node:assert/strict';
import test from 'node:test';

import { agentGroupOf, filterAgentsByQuery, groupAgents, canManageAgent } from './agent-groups.ts';

// Vue baseline: frontend/src/views/agent/AgentList.vue 73-155 — the list is
// grouped into builtin / created-by-me / shared with collapsible headers and
// counts, filtered by a search box over name and description.

const agent = (overrides: Record<string, unknown>) => ({ id: 'a', name: 'Agent', ...overrides });

test('builtin agents always group first, regardless of creator', () => {
  assert.equal(agentGroupOf(agent({ is_builtin: true, created_by: 'u1' }), 'u1'), 'builtin');
  assert.equal(agentGroupOf(agent({ is_builtin: true, created_by: '' }), ''), 'builtin');
});

test('own agents group as mine via created_by', () => {
  assert.equal(agentGroupOf(agent({ is_builtin: false, created_by: 'u1' }), 'u1'), 'mine');
  assert.equal(agentGroupOf(agent({ created_by: 'u1' }), 'u1'), 'mine');
});

test('non-builtin agents from others are shared', () => {
  assert.equal(agentGroupOf(agent({ is_builtin: false, created_by: 'u2' }), 'u1'), 'shared');
  assert.equal(agentGroupOf(agent({ created_by: '' }), 'u1'), 'shared');
  assert.equal(agentGroupOf(agent({ created_by: 'u2' }), ''), 'shared', 'unknown current user cannot claim agents');
});

test('groupAgents partitions and orders builtin, mine, shared', () => {
  const agents = [
    agent({ id: '1', created_by: 'u2' }),
    agent({ id: '2', is_builtin: true }),
    agent({ id: '3', created_by: 'u1' }),
    agent({ id: '4', is_builtin: true }),
  ];
  const groups = groupAgents(agents, 'u1');
  assert.deepEqual(groups.builtin.map((item) => item.id), ['2', '4']);
  assert.deepEqual(groups.mine.map((item) => item.id), ['3']);
  assert.deepEqual(groups.shared.map((item) => item.id), ['1']);
});

test('search filters case-insensitively over name and description', () => {
  const agents = [
    agent({ id: 'a', name: 'Support bot', description: 'answers tickets' }),
    agent({ id: 'b', name: 'Wiki agent', description: 'searches docs' }),
  ];
  assert.deepEqual(filterAgentsByQuery(agents, 'SUPPORT').map((item) => item.id), ['a']);
  assert.deepEqual(filterAgentsByQuery(agents, 'docs').map((item) => item.id), ['b']);
  assert.equal(filterAgentsByQuery(agents, '  ').length, 2, 'blank query keeps everything');
  assert.equal(filterAgentsByQuery(agents, 'nomatch').length, 0);
});

test('matches Vue write permissions for built-in and shared agents', () => {
  assert.equal(canManageAgent(agent({ is_builtin: true, created_by: 'u1' })), false);
  assert.equal(canManageAgent(agent({ created_by: 'u2', permission: 'viewer' })), false);
  assert.equal(canManageAgent(agent({ created_by: 'u2', permission: 'editor' })), true);
  assert.equal(canManageAgent(agent({ created_by: 'u1' })), true);
});
