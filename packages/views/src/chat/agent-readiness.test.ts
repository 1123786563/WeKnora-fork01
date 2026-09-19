import assert from 'node:assert/strict';
import test from 'node:test';

import {
  agentHasConfiguredChatModel,
  agentRequiresRerankModel,
  getAgentNotReadyReasonKeys,
  resolveAgentNotReadyHighlight,
  resolveAgentNotReadySection,
} from './agent-readiness.ts';

// Upstream agent-readiness.ts contract: an agent is selectable only when it
// explicitly references a usable chat model; smart-reasoning agents also need
// a rerank model whenever knowledge_search can run.

const MODELS = [
  { id: 'chat-1', type: 'KnowledgeQA' },
  { id: 'rerank-1', type: 'Rerank' },
  { id: 'embed-1', type: 'Embedding' },
];

test('chat model readiness requires an explicit, existing KnowledgeQA model', () => {
  assert.equal(agentHasConfiguredChatModel({ model_id: 'chat-1' }, MODELS), true);
  assert.equal(agentHasConfiguredChatModel({ model_id: 'embed-1' }, MODELS), false, 'wrong type does not count');
  assert.equal(agentHasConfiguredChatModel({ model_id: 'missing' }, MODELS), false);
  assert.equal(agentHasConfiguredChatModel({ model_id: '  ' }, MODELS), false, 'whitespace-only id');
  assert.equal(agentHasConfiguredChatModel(undefined, MODELS), false, 'no config at all');
  assert.equal(agentHasConfiguredChatModel({ model_id: 'chat-1' }, []), false, 'no models loaded yet');
});

test('rerank is required exactly when knowledge_search can run', () => {
  assert.equal(agentRequiresRerankModel(undefined), false);
  assert.equal(agentRequiresRerankModel({ kb_selection_mode: 'none' }), false);
  assert.equal(agentRequiresRerankModel({ kb_selection_mode: 'all' }), true, 'empty allowed_tools falls back to the default toolset');
  assert.equal(agentRequiresRerankModel({ kb_selection_mode: 'all', allowed_tools: ['knowledge_search'] }), true);
  assert.equal(agentRequiresRerankModel({ kb_selection_mode: 'all', allowed_tools: ['web_search'] }), false);
});

test('not-ready keys combine chat model and rerank gates for agent mode', () => {
  assert.deepEqual(
    getAgentNotReadyReasonKeys({ agent_mode: 'smart-reasoning' }, MODELS, { isAgentMode: true }),
    ['summary_model', 'rerank_model'],
  );
  assert.deepEqual(
    getAgentNotReadyReasonKeys({ model_id: 'chat-1' }, MODELS, { isAgentMode: false }),
    [],
    'quick-answer agents do not need a rerank model',
  );
  assert.deepEqual(
    getAgentNotReadyReasonKeys(
      { agent_mode: 'smart-reasoning', model_id: 'chat-1', kb_selection_mode: 'all', rerank_model_id: 'rerank-1' },
      MODELS,
      { isAgentMode: true },
    ),
    [],
  );
  assert.deepEqual(
    getAgentNotReadyReasonKeys(
      { agent_mode: 'smart-reasoning', model_id: 'chat-1', kb_selection_mode: 'all', rerank_model_id: 'embed-1' },
      MODELS,
      { isAgentMode: true },
    ),
    ['rerank_model'],
    'a configured model of the wrong type is still not ready',
  );
});

test('reasons map to the model editor section with the first missing item highlighted', () => {
  assert.equal(resolveAgentNotReadySection(['summary_model']), 'model');
  assert.equal(resolveAgentNotReadySection(['rerank_model']), 'model');
  assert.equal(resolveAgentNotReadySection([]), 'model');
  assert.equal(resolveAgentNotReadyHighlight(['summary_model', 'rerank_model']), 'summary_model');
  assert.equal(resolveAgentNotReadyHighlight([]), undefined);
});

// R484 D14 — shared-agent branches ported from the Vue judgement source
// (frontend/src/utils/agent-readiness.ts getAgentNotReadyReasonKeys): for a
// shared agent the models live in the source tenant, so a non-empty
// model/rerank id is accepted without local existence validation.
test('shared agents pass readiness with non-empty model ids (Vue isSharedAgent branch)', () => {
  assert.deepEqual(
    getAgentNotReadyReasonKeys(
      { agent_mode: 'smart-reasoning', model_id: 'remote-model', kb_selection_mode: 'all', rerank_model_id: 'remote-rerank' },
      [],
      { isAgentMode: true, isSharedAgent: true },
    ),
    [],
    'remote ids are trusted without local model lookup',
  );
  assert.deepEqual(
    getAgentNotReadyReasonKeys(
      { agent_mode: 'smart-reasoning', model_id: '', kb_selection_mode: 'all', rerank_model_id: 'remote-rerank' },
      MODELS,
      { isAgentMode: true, isSharedAgent: true },
    ),
    ['summary_model'],
    'an empty model id is still not ready even for shared agents',
  );
  assert.deepEqual(
    getAgentNotReadyReasonKeys(
      { agent_mode: 'smart-reasoning', model_id: 'chat-1', kb_selection_mode: 'all', rerank_model_id: '' },
      MODELS,
      { isAgentMode: true, isSharedAgent: true },
    ),
    ['rerank_model'],
    'an empty rerank id is still not ready even for shared agents',
  );
});
