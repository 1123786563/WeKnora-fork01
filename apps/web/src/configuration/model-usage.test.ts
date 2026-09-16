import assert from 'node:assert/strict';
import test from 'node:test';
import { modelInUseDetails, modelUsageBindingLabel, parseModelUsageDetails } from './model-usage.ts';

test('parses structured model usage details and keeps totals above listed rows', () => {
  const details = parseModelUsageDetails({
    knowledge_bases: [{ id: 'kb-1', name: 'Docs', bindings: ['embedding_model'] }],
    agents: [{ id: 'agent-1', name: 'Research', bindings: ['chat_model'] }],
    long_term_memory: { bindings: ['follow_up_model'] },
    knowledge_base_total: 3,
    agent_total: 2,
  });
  assert.deepEqual(details, {
    knowledge_bases: [{ id: 'kb-1', name: 'Docs', bindings: ['embedding_model'] }],
    agents: [{ id: 'agent-1', name: 'Research', bindings: ['chat_model'] }],
    long_term_memory: { bindings: ['follow_up_model'] },
    knowledge_base_total: 3,
    agent_total: 2,
  });
});

test('extracts model-in-use details from the shared ApiError shape', () => {
  assert.deepEqual(modelInUseDetails({ code: 2300, details: {
    knowledge_bases: [], agents: [], long_term_memory: { bindings: ['vlm_model'] },
  } }), {
    knowledge_bases: [], agents: [], long_term_memory: { bindings: ['vlm_model'] },
    knowledge_base_total: 0, agent_total: 0,
  });
  assert.equal(modelInUseDetails({ code: 1000, details: {} }), null);
});

test('uses a safe fallback label for a future model binding', () => {
  assert.equal(modelUsageBindingLabel('chat_model'), 'chat model');
  assert.equal(modelUsageBindingLabel('future_binding'), 'future binding');
});
