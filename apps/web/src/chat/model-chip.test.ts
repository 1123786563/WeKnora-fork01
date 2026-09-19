import assert from 'node:assert/strict';
import test from 'node:test';

import {
  DEFAULT_MODEL_CONTEXT_WINDOW,
  formatContextWindow,
  isDefaultContextWindow,
  listChatModels,
  MODEL_CHIP_NOT_CONFIGURED,
  resolveChatModelChip,
} from './model-chip.ts';

// Vue parity pins for the composer model chip (R016). The Vue chat input
// renders the resolved chat model's display name plus a compact context-window
// suffix (frontend/src/components/Input-field.vue model-display computeds,
// lines 1067-1105) with the formatting of frontend/src/utils/contextWindow.ts.

test('context window formatting matches the Vue contextWindow util', () => {
  assert.equal(DEFAULT_MODEL_CONTEXT_WINDOW, 200000);
  // Missing/invalid values fall back to the 200K backend default.
  assert.equal(formatContextWindow(undefined), '200K');
  assert.equal(formatContextWindow(null), '200K');
  assert.equal(formatContextWindow(0), '200K');
  assert.equal(formatContextWindow(Number.NaN), '200K');
  assert.equal(formatContextWindow({} as unknown as number), '200K');
  // Explicit values compact to K/M (128000 -> 128K, 1048576 -> 1M).
  assert.equal(formatContextWindow(200000), '200K');
  assert.equal(formatContextWindow(128000), '128K');
  assert.equal(formatContextWindow(64000), '64K');
  assert.equal(formatContextWindow(1024), '1K');
  assert.equal(formatContextWindow(1048576), '1M');
  assert.equal(formatContextWindow(2097152), '2M');
  assert.equal(formatContextWindow(1536), '1536');
});

test('isDefaultContextWindow flags models without an explicit window', () => {
  assert.equal(isDefaultContextWindow(undefined), true);
  assert.equal(isDefaultContextWindow(null), true);
  assert.equal(isDefaultContextWindow(0), true);
  assert.equal(isDefaultContextWindow(200000), false);
  assert.equal(isDefaultContextWindow(128000), false);
});

test('chat model list keeps the Vue KnowledgeQA filter only', () => {
  const models = [
    { id: 'chat-1', type: 'KnowledgeQA' },
    { id: 'embed-1', type: 'BGE-M3' },
    { id: 'rerank-1', type: 'BGE-RERANKER-V2-M3' },
    { id: 'chat-2', type: 'KnowledgeQA' },
    { id: 'untyped-1' },
  ];
  assert.deepEqual(
    listChatModels(models).map((model) => model.id),
    ['chat-1', 'chat-2'],
  );
});

test('chip resolves display_name over name and appends the context spec', () => {
  const chip = resolveChatModelChip({
    models: [{ id: 'mock-stream-model', name: 'mock-stream-model', display_name: ' Mock Stream ', type: 'KnowledgeQA', parameters: { context_window: 128000 } }],
    selectedModelId: 'mock-stream-model',
  });
  assert.equal(chip.label, 'Mock Stream');
  assert.equal(chip.context, '128K');
  assert.equal(chip.isDefaultContext, false);
});

test('chip prefers the selected agent model over the first listed model', () => {
  const chip = resolveChatModelChip({
    models: [
      { id: 'first-model', name: 'first-model', type: 'KnowledgeQA' },
      { id: 'agent-model', name: 'agent-model', display_name: 'Agent Model', type: 'KnowledgeQA' },
    ],
    agentModelId: 'agent-model',
  });
  assert.equal(chip.label, 'Agent Model');
});

/*
 * R464 Vue selection-priority pins. Vue Input-field.vue resolves the chip from
 * selectedModelId, which is (a) seeded from the localStorage last pick
 * (ensureModelSelection), (b) only overridden by the agent-model watch when the
 * user has no differing explicit pick. The React chip must honor the same
 * pick → agent → first-model order instead of jumping straight to the agent
 * binding or the first row (the R463 mock/glm-5.3 header mismatch).
 */
test('chip honors the persisted model pick over the first listed model', () => {
  const chip = resolveChatModelChip({
    models: [
      { id: 'parity-llm-mock', name: 'parity-llm-mock', display_name: 'Mock LLM', type: 'KnowledgeQA' },
      { id: 'glm-5.3', name: 'glm-5.3', type: 'KnowledgeQA' },
    ],
    selectedModelId: 'glm-5.3',
  });
  assert.equal(chip.label, 'glm-5.3');
  assert.equal(chip.context, '200K');
  assert.equal(chip.isDefaultContext, true);
});

test('a differing user pick wins over the agent binding like the Vue agent-model watch', () => {
  const chip = resolveChatModelChip({
    models: [
      { id: 'first-model', name: 'first-model', type: 'KnowledgeQA' },
      { id: 'agent-model', name: 'agent-model', display_name: 'Agent Model', type: 'KnowledgeQA' },
      { id: 'glm-5.3', name: 'glm-5.3', type: 'KnowledgeQA' },
    ],
    agentModelId: 'agent-model',
    selectedModelId: 'glm-5.3',
  });
  assert.equal(chip.label, 'glm-5.3');
});

test('the agent binding wins when the pick matches it or is absent', () => {
  const models = [
    { id: 'first-model', name: 'first-model', type: 'KnowledgeQA' },
    { id: 'agent-model', name: 'agent-model', display_name: 'Agent Model', type: 'KnowledgeQA' },
  ];
  assert.equal(resolveChatModelChip({ models, agentModelId: 'agent-model', selectedModelId: 'agent-model' }).label, 'Agent Model');
  assert.equal(resolveChatModelChip({ models, agentModelId: 'agent-model' }).label, 'Agent Model');
});

test('a persisted pick missing from the list stays unconfigured like Vue', () => {
  const chip = resolveChatModelChip({
    models: [{ id: 'first-model', name: 'first-model', type: 'KnowledgeQA' }],
    selectedModelId: 'ghost-model',
    notConfiguredLabel: '未配置',
  });
  assert.equal(chip.label, '未配置');
  assert.equal(chip.context, '');
  assert.equal(chip.isDefaultContext, false);
});

test('without an agent binding or explicit pick the chip falls back to the first model (Vue ensureModelSelection)', () => {
  // Re-verified 2026-09-19 on a fresh origin: with an empty
  // weknora_last_chat_model_id on BOTH sides Vue shows the first chat model
  // with its 200K default context, not 未配置.
  const chip = resolveChatModelChip({
    models: [
      { id: 'first-model', name: 'first-model', type: 'KnowledgeQA', parameters: {} },
      { id: 'second-model', name: 'second-model', type: 'KnowledgeQA' },
    ],
  });
  assert.equal(chip.label, 'first-model');
  assert.equal(chip.context, '200K');
  assert.equal(chip.isDefaultContext, true);
});

test('an agent model missing from the list stays unconfigured like Vue', () => {
  // Vue Input-field.vue: selectedModel = find(selectedModelId); a bound id
  // that is not in the chat-model list renders input.notConfigured, it must
  // not silently fall back to the first model.
  const chip = resolveChatModelChip({
    models: [{ id: 'first-model', name: 'first-model', type: 'KnowledgeQA' }],
    agentModelId: 'missing-model',
    notConfiguredLabel: '未配置',
  });
  assert.equal(chip.label, '未配置');
  assert.equal(chip.context, '');
  assert.equal(chip.isDefaultContext, false);
});

test('an empty model list shows the localized unconfigured fallback', () => {
  const chip = resolveChatModelChip({ models: [], notConfiguredLabel: 'Not configured' });
  assert.equal(chip.label, 'Not configured');
  assert.equal(chip.context, '');
});

test('a model with no display name uses the localized unconfigured fallback', () => {
  const chip = resolveChatModelChip({ models: [{ id: 'm1', name: '', type: 'KnowledgeQA' }], notConfiguredLabel: '未配置' });
  assert.equal(chip.label, '未配置');
});

test('the unconfigured fallback carries the byte-exact Vue input.notConfigured strings', () => {
  assert.equal(MODEL_CHIP_NOT_CONFIGURED['zh-CN'], '未配置');
  assert.equal(MODEL_CHIP_NOT_CONFIGURED['en-US'], 'Not configured');
  assert.equal(MODEL_CHIP_NOT_CONFIGURED['ja-JP'], '未設定');
  assert.equal(MODEL_CHIP_NOT_CONFIGURED['ko-KR'], '구성되지 않음');
  assert.equal(MODEL_CHIP_NOT_CONFIGURED['ru-RU'], 'Не настроено');
});
