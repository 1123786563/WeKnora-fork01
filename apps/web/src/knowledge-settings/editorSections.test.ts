import assert from 'node:assert/strict';
import test from 'node:test';
import {
  CHUNKING_LANGUAGE_VALUES,
  CHUNKING_SEPARATOR_VALUES,
  CHUNKING_STRATEGY_VALUES,
  CHILD_CHUNK_SIZE_RANGE,
  CHUNK_OVERLAP_RANGE,
  CHUNK_SIZE_RANGE,
  PARENT_CHUNK_SIZE_RANGE,
  QUESTION_COUNT_RANGE,
  TOKEN_LIMIT_RANGE,
  clampQuestionCount,
  filterKnowledgeSettingsModels,
  isChunkOverlapTooHigh,
  isChunkingAdvancedDisabled,
  isKnowledgeSettingsModelAvailable,
  type KnowledgeSettingsModelOption,
} from './editorSections.ts';

// Vue contract references:
// - frontend/src/components/modelSelectorFilter.ts (filterModelsByType)
// - frontend/src/utils/modelDefaults.ts (!model.status || model.status === 'active')
// - frontend/src/views/knowledge/settings/KBChunkingSettings.vue (ranges/options)
// - frontend/src/views/knowledge/settings/KBAdvancedSettings.vue (question count bounds)

test('model catalogue filter mirrors the Vue ModelSelector type filter and drops unavailable models', () => {
  const models: KnowledgeSettingsModelOption[] = [
    { id: 'llm-1', name: 'gpt-x', displayName: 'GPT X', type: 'KnowledgeQA', source: 'remote' },
    { id: 'llm-2', name: 'qwen', displayName: '', type: 'KnowledgeQA', source: 'local' },
    { id: 'llm-dead', name: 'dead', displayName: 'Dead', type: 'KnowledgeQA', source: 'local' },
    { id: 'embed-1', name: 'bge', displayName: 'BGE', type: 'Embedding', source: 'local' },
  ];
  // Mark llm-dead unavailable: Vue modelDefaults treats any status other than
  // 'active' (or missing) as not usable.
  (models[2] as { status?: string }).status = 'unavailable';

  const llm = filterKnowledgeSettingsModels(models, 'KnowledgeQA');
  assert.deepEqual(llm.map((model) => model.id), ['llm-1', 'llm-2'], 'unavailable models must be excluded, other types ignored');
  assert.equal(llm[0]!.displayName, 'GPT X');
  assert.equal(llm[1]!.displayName, 'qwen', 'display name falls back to the raw model name');

  assert.deepEqual(filterKnowledgeSettingsModels(models, 'Embedding').map((model) => model.id), ['embed-1']);
  assert.deepEqual(filterKnowledgeSettingsModels([], 'KnowledgeQA'), []);
});

test('model availability follows the Vue modelDefaults rule (no status or active is usable)', () => {
  assert.equal(isKnowledgeSettingsModelAvailable({}), true);
  assert.equal(isKnowledgeSettingsModelAvailable({ status: 'active' }), true);
  assert.equal(isKnowledgeSettingsModelAvailable({ status: 'unavailable' }), false);
  assert.equal(isKnowledgeSettingsModelAvailable({ status: 'downloading' }), false);
});

test('chunking ranges match the Vue KBChunkingSettings slider bounds', () => {
  assert.deepEqual(CHUNK_SIZE_RANGE, { min: 100, max: 4000, step: 50 });
  assert.deepEqual(CHUNK_OVERLAP_RANGE, { min: 0, max: 500, step: 20 });
  assert.deepEqual(PARENT_CHUNK_SIZE_RANGE, { min: 512, max: 8192, step: 64 });
  assert.deepEqual(CHILD_CHUNK_SIZE_RANGE, { min: 64, max: 2048, step: 32 });
  assert.deepEqual(TOKEN_LIMIT_RANGE, { min: 0, max: 8192, step: 64 });
  assert.deepEqual(QUESTION_COUNT_RANGE, { min: 1, max: 10, step: 1 });
});

test('chunking option vocabularies match the Vue selects', () => {
  assert.deepEqual([...CHUNKING_STRATEGY_VALUES], ['auto', 'heading', 'heuristic', 'legacy']);
  assert.deepEqual([...CHUNKING_SEPARATOR_VALUES], ['\n\n', '\n', '。', '！', '？', '；', ';', ' ']);
  assert.deepEqual([...CHUNKING_LANGUAGE_VALUES], ['de', 'en', 'zh']);
});

test('overlap warning reproduces the Vue overlapTooHigh computed', () => {
  assert.equal(isChunkOverlapTooHigh(700, 90), false);
  assert.equal(isChunkOverlapTooHigh(700, 0), false, 'zero overlap never warns');
  assert.equal(isChunkOverlapTooHigh(700, 350), true, 'overlap equal to half the chunk size warns');
  assert.equal(isChunkOverlapTooHigh(700, 400), true);
  assert.equal(isChunkOverlapTooHigh(100, 50), true);
});

test('legacy strategy disables the adaptive token/language controls like the Vue panel', () => {
  assert.equal(isChunkingAdvancedDisabled('legacy'), true);
  assert.equal(isChunkingAdvancedDisabled('auto'), false);
  assert.equal(isChunkingAdvancedDisabled(''), false);
});

test('question count clamps to the Vue input-number bounds', () => {
  assert.equal(clampQuestionCount(5), 5);
  assert.equal(clampQuestionCount(0), 1);
  assert.equal(clampQuestionCount(99), 10);
  assert.equal(clampQuestionCount(Number.NaN), 1);
});
