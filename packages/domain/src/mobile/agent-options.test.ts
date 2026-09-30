import test from 'node:test';
import assert from 'node:assert/strict';
import {
  defaultAgent,
  filterAgents,
  selectAgent,
  toAgentOptions,
  type AgentOption,
} from './agent-options.ts';

test('toAgentOptions projects presentation-safe fields and defaults missing capability to unavailable', () => {
  const options = toAgentOptions([
    {
      id: 'builtin-quick-answer',
      name: 'Quick Answer',
      summary: 'Fast answers',
      kind: 'general',
      capability: { state: 'supported', reason: '' },
      config: { system_prompt: 'SECRET-PROMPT' },
    },
    { id: 'agent-2', name: 'No capability facts' },
    { name: 'missing id' },
  ]);

  assert.deepEqual(options, [
    { id: 'builtin-quick-answer', name: 'Quick Answer', summary: 'Fast answers', kind: 'general', capability: { state: 'supported', reason: '' } },
    { id: 'agent-2', name: 'No capability facts', summary: '', kind: 'general', capability: { state: 'unavailable', reason: 'capability_not_reported' } },
  ]);
  assert.equal(JSON.stringify(options).includes('SECRET-PROMPT'), false, 'raw config must not survive the projection');
});

test('selectAgent refuses revoked coding targets and never falls back silently', () => {
  const directory = {
    agents: toAgentOptions([
      { id: 'coder', name: 'Coder', kind: 'coding', authorizedTargetId: 'target-1', capability: { state: 'supported', reason: '' } },
      { id: 'helper', name: 'Helper', kind: 'general', capability: { state: 'supported', reason: '' } },
    ]),
  };

  assert.deepEqual(selectAgent(directory, 'coder', [{ id: 'target-1', revoked: true }]), { unavailableReason: 'driver_unavailable' });
  assert.equal(selectAgent(directory, 'coder', [{ id: 'target-1' }]).agent?.id, 'coder');
  assert.deepEqual(selectAgent(directory, 'missing'), { unavailableReason: 'agent_not_found' });
  assert.equal(defaultAgent(directory, [{ id: 'target-1', revoked: true }])?.id, 'helper');
});

test('filterAgents narrows display only and never changes capability verdicts', () => {
  const agents: AgentOption[] = [
    { id: 'a', name: 'Research', summary: 'deep research', kind: 'analysis', capability: { state: 'forbidden', reason: 'policy' } },
    { id: 'b', name: 'Writer', summary: '', kind: 'general', capability: { state: 'unavailable', reason: 'capability_not_reported' } },
  ];

  assert.deepEqual(filterAgents({ agents }, { kind: 'analysis' }).map((agent) => agent.id), ['a']);
  assert.deepEqual(filterAgents({ agents }, { keyword: 'wri' }).map((agent) => agent.id), ['b']);
  assert.deepEqual(filterAgents({ agents }, {}).map((agent) => agent.id), ['a', 'b']);
});
