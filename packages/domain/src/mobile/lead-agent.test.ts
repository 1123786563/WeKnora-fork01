import test from 'node:test';
import assert from 'node:assert/strict';
import { recommendLeadAgent } from './lead-agent.ts';
import type { AgentOption } from './agent-options.ts';

function agent(id: string, overrides: Partial<AgentOption> = {}): AgentOption {
  return {
    id,
    name: `Agent ${id}`,
    summary: '',
    kind: 'general',
    capability: { state: 'supported', reason: '' },
    ...overrides,
  };
}

test('recommends the first supported general agent (universal New entry default lead)', () => {
  const coding = agent('a-coding', { kind: 'coding' });
  const general = agent('a-general', { kind: 'general' });
  const later = agent('a-later', { kind: 'general' });
  const recommendation = recommendLeadAgent([coding, general, later]);
  assert.deepEqual(recommendation, { agent: general, basis: 'kind-general' });
});

test('falls back to the first supported agent of any kind when no general agent is supported', () => {
  const analysis = agent('a-analysis', { kind: 'analysis' });
  const recommendation = recommendLeadAgent([analysis, agent('a-coding', { kind: 'coding' })]);
  assert.deepEqual(recommendation, { agent: analysis, basis: 'first-supported' });
});

test('unavailable and forbidden agents are never recommended; absence is explicit', () => {
  const unavailable = agent('a-1', { capability: { state: 'unavailable', reason: 'capability_not_reported' } });
  const forbidden = agent('a-2', { capability: { state: 'forbidden', reason: 'policy' } });
  assert.deepEqual(recommendLeadAgent([unavailable, forbidden]), { agent: undefined, reason: 'no_supported_agent' });
  assert.deepEqual(recommendLeadAgent([]), { agent: undefined, reason: 'no_supported_agent' });
});
