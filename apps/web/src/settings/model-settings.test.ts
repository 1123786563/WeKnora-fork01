import assert from 'node:assert/strict';
import test from 'node:test';

import {
  DEFAULT_MODEL_CONTEXT_WINDOW,
  FALLBACK_MODEL_PROVIDERS,
  backendModelType,
  baseUrlPlaceholderKey,
  createModelTranslator,
  customHeadersMap,
  defaultThinkingControl,
  effectiveContextWindow,
  fallbackProviderOptions,
  formatContextWindow,
  isDefaultContextWindow,
  modelDraftFromRecord,
  modelFieldErrorKey,
  modelNamePlaceholderKey,
  modelPayload,
  modelSupportsThinking,
  modelValidationErrorKey,
  modelType,
  newModelDraft,
  providerDefaultUrl,
  providerText,
  resolveThinkingControl,
  signedRerankProvider,
  storedThinkingControl,
  subsectionToFilter,
  validateModelDraft,
  type ModelDraft,
} from './model-settings.ts';

// ModelEditorDialog.vue rules (lines 907-946): per-field messages with the
// exact Vue i18n copy, triggered on blur.
test('field-level validators reuse the Vue ModelEditorDialog rules and copy', () => {
  assert.equal(modelFieldErrorKey('name', { ...baseDraft, name: '' }), 'model.editor.validation.modelNameRequired');
  assert.equal(modelFieldErrorKey('name', { ...baseDraft, name: '   ' }), 'model.editor.validation.modelNameEmpty');
  assert.equal(modelFieldErrorKey('name', { ...baseDraft, name: 'x'.repeat(101) }), 'model.editor.validation.modelNameMax');
  assert.equal(modelFieldErrorKey('name', baseDraft), null);
  assert.equal(modelFieldErrorKey('name', { ...baseDraft, name: 'ok'.padEnd(100, 'x') }), null);
  assert.equal(modelFieldErrorKey('baseUrl', { ...baseDraft, baseUrl: '' }), 'model.editor.validation.baseUrlRequired');
  assert.equal(modelFieldErrorKey('baseUrl', { ...baseDraft, baseUrl: '   ' }), 'model.editor.validation.baseUrlEmpty');
  assert.equal(modelFieldErrorKey('baseUrl', { ...baseDraft, baseUrl: 'not-a-url' }), 'model.editor.validation.baseUrlInvalid');
  assert.equal(modelFieldErrorKey('baseUrl', baseDraft), null);
});

// ModelSettings.vue watches uiStore.settingsInitialSubSection (lines 329-337)
// and maps it onto the type tab filter.
test('subsection deep links map onto model type tabs and ignore unknown values', () => {
  assert.equal(subsectionToFilter('chat'), 'chat');
  assert.equal(subsectionToFilter('embedding'), 'embedding');
  assert.equal(subsectionToFilter('rerank'), 'rerank');
  assert.equal(subsectionToFilter('vllm'), 'vllm');
  assert.equal(subsectionToFilter('asr'), 'asr');
  assert.equal(subsectionToFilter('models'), null);
  assert.equal(subsectionToFilter(undefined), null);
});

const baseDraft: ModelDraft = {
  name: 'text-embedding-3-small', displayName: '', type: 'embedding', source: 'remote', provider: 'openai',
  baseUrl: 'https://api.openai.com/v1', dimension: 1536, supportsDimensionOverride: false, supportsVision: false,
  contextWindow: '', maxConcurrency: '', thinkingControl: '', customHeaders: [], apiKey: '', appSecret: '',
  lkeapRegion: 'ap-guangzhou',
};

test('model type mapping uses the backend vocabulary (ModelSettings.vue backendTypeToModelType)', () => {
  assert.equal(modelType({ type: 'EmbeddingModel' }), 'embedding');
  assert.equal(modelType({ type: 'KnowledgeQA' }), 'chat');
  assert.equal(modelType({ type: 'Rerank' }), 'rerank');
  assert.equal(modelType({ type: 'VLLM' }), 'vllm');
  assert.equal(modelType({ type: 'ASR' }), 'asr');
  assert.equal(backendModelType('vllm'), 'VLLM');
});

// ModelSettings.vue handleModelSave lines 585-655.
test('model payload builds fresh Vue-shaped parameters and isolates credentials to create', () => {
  assert.deepEqual(modelPayload(baseDraft), {
    name: 'text-embedding-3-small', display_name: '', description: '', type: 'Embedding', source: 'remote',
    parameters: {
      base_url: 'https://api.openai.com/v1', provider: 'openai',
      embedding_parameters: { dimension: 1536, truncate_prompt_tokens: 0, supports_dimension_override: false },
    },
  });
  const created = modelPayload({
    ...baseDraft, apiKey: '  sk-secret  ', appSecret: ' app-secret ',
  });
  assert.equal((created.parameters as Record<string, unknown>).api_key, 'sk-secret');
  assert.equal((created.parameters as Record<string, unknown>).app_secret, 'app-secret');
  const edited = modelPayload({ ...baseDraft, id: 'model-1', apiKey: 'sk-secret', appSecret: 'app-secret' });
  const editedParameters = edited.parameters as Record<string, unknown>;
  assert.ok(!('api_key' in editedParameters));
  assert.ok(!('app_secret' in editedParameters));
});

// ModelEditorDialog.vue checkRemoteAPI + ModelSettings.vue customHeadersMap loop:
// only rows where both key and value trim to non-empty are sent.
test('custom headers drop empty rows in payload and map helpers', () => {
  const payload = modelPayload({
    ...baseDraft,
    customHeaders: [
      { key: 'X-Trace', value: 'abc' },
      { key: '  ', value: 'no-key' },
      { key: 'X-Empty', value: '' },
    ],
  });
  assert.deepEqual((payload.parameters as Record<string, unknown>).custom_headers, { 'X-Trace': 'abc' });
  assert.deepEqual(customHeadersMap([{ key: 'A', value: '1' }, { key: 'A ', value: ' 2 ' }]), { A: '2' });
});

// ModelSettings.vue line 606-618: lkeap rerank region and chat thinking control
// share the extra_config object; region is rerank-only, thinking is chat+remote-only.
test('extra_config carries lkeap rerank region and chat thinking control like Vue', () => {
  const rerank = modelPayload({
    ...baseDraft, type: 'rerank', provider: 'lkeap', dimension: '', source: 'remote', lkeapRegion: 'ap-shanghai',
  });
  assert.deepEqual((rerank.parameters as Record<string, unknown>).extra_config, { region: 'ap-shanghai' });
  const volcengine = modelPayload({
    ...baseDraft, type: 'rerank', provider: 'volcengine', dimension: '',
  });
  assert.ok(!('extra_config' in (volcengine.parameters as Record<string, unknown>)));

  const chat = modelPayload({ ...baseDraft, type: 'chat', dimension: '', thinkingControl: 'thinking_type' });
  assert.deepEqual((chat.parameters as Record<string, unknown>).extra_config, { thinking_control: 'thinking_type' });
  const localChat = modelPayload({ ...baseDraft, type: 'chat', dimension: '', source: 'local', thinkingControl: 'thinking_type' });
  assert.ok(!('extra_config' in (localChat.parameters as Record<string, unknown>)));
});

// ModelSettings.vue lines 640-653: vllm is always multimodal, chat follows the
// toggle, context window needs >= 1024 tokens, concurrency only for governed types.
test('payload applies per-type vision, context window and concurrency rules', () => {
  const vision = (draft: ModelDraft) => (modelPayload(draft).parameters as Record<string, unknown>);
  assert.equal(vision({ ...baseDraft, type: 'vllm', dimension: '' }).supports_vision, true);
  assert.equal(vision({ ...baseDraft, type: 'chat', dimension: '', supportsVision: true }).supports_vision, true);
  assert.equal(vision({ ...baseDraft, type: 'chat', dimension: '', supportsVision: false }).supports_vision, false);
  assert.ok(!('supports_vision' in vision({ ...baseDraft, type: 'rerank', dimension: '' })));

  const contextWindow = (value: number | '') => (modelPayload({ ...baseDraft, type: 'chat', dimension: '', contextWindow: value }).parameters as Record<string, unknown>).context_window;
  assert.equal(contextWindow(200000), 200000);
  assert.equal(contextWindow(1023.6), undefined);
  assert.equal(contextWindow(''), undefined);

  const concurrency = (type: ModelDraft['type'], value: number | '') => (modelPayload({ ...baseDraft, type, dimension: '', maxConcurrency: value }).parameters as Record<string, unknown>).max_concurrency;
  assert.equal(concurrency('chat', 5), 5);
  assert.equal(concurrency('embedding', 3), 3);
  assert.equal(concurrency('vllm', 2), 2);
  assert.equal(concurrency('rerank', 5), undefined);
  assert.equal(concurrency('asr', 5), undefined);
  assert.equal(concurrency('chat', 0), undefined);

  // Rerank stays remote-only even if a draft slips through with source local.
  assert.equal(modelPayload({ ...baseDraft, type: 'rerank', source: 'local', dimension: '' }).source, 'remote');
});

test('embedding parameters record dimension override state (ModelSettings.vue line 633-639)', () => {
  const parameters = modelPayload({ ...baseDraft, supportsDimensionOverride: true }).parameters as Record<string, unknown>;
  assert.deepEqual(parameters.embedding_parameters, { dimension: 1536, truncate_prompt_tokens: 0, supports_dimension_override: true });
});

// ModelSettings.vue handleModelSave lines 549-583 — order matters for the toast copy.
test('validation mirrors Vue submit gating including display name and weknoracloud base URL', () => {
  assert.deepEqual(validateModelDraft({ ...baseDraft, name: '', baseUrl: '' }), ['nameRequired', 'baseUrlRequired']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, name: 'x'.repeat(101) }), ['nameTooLong']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, displayName: 'y'.repeat(101) }), ['displayNameTooLong']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, baseUrl: 'not-a-url' }), ['baseUrlInvalid']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, baseUrl: '' }), ['baseUrlRequired']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, dimension: 64 }), ['dimensionInvalid']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, dimension: 1536 }), []);
  // Vue's parent handler requires a base URL for every remote source — the
  // weknoracloud exemption only lives in the editor dialog's own gating.
  assert.deepEqual(validateModelDraft({ ...baseDraft, provider: 'weknoracloud', baseUrl: '' }), ['baseUrlRequired']);
  assert.deepEqual(validateModelDraft({ ...baseDraft, source: 'local', baseUrl: '' }), []);
  assert.equal(modelValidationErrorKey('nameRequired'), 'modelSettings.toasts.nameRequired');
});

// frontend/src/utils/thinkingControl.ts defaultThinkingControl.
test('thinking control defaults follow provider and model name like Vue', () => {
  assert.equal(defaultThinkingControl('aliyun', 'qwen3-8b'), 'enable_thinking');
  assert.equal(defaultThinkingControl('aliyun', 'deepseek-v3'), 'none');
  assert.equal(defaultThinkingControl('lkeap', 'deepseek-r1'), 'none');
  assert.equal(defaultThinkingControl('lkeap', 'deepseek-v3'), 'thinking_type');
  assert.equal(defaultThinkingControl('lkeap', ''), 'thinking_type');
  assert.equal(defaultThinkingControl('generic', ''), 'chat_template_kwargs');
  assert.equal(defaultThinkingControl('nvidia', 'x'), 'chat_template_kwargs');
  assert.equal(defaultThinkingControl('litellm', 'x'), 'chat_template_kwargs');
  assert.equal(defaultThinkingControl('volcengine', 'doubao-seed-rerank'), 'thinking_type');
  assert.equal(defaultThinkingControl('openai', 'gpt-4'), 'none');
  assert.equal(defaultThinkingControl('weknoracloud', 'x'), 'none');

  assert.equal(resolveThinkingControl('enable_thinking', 'openai', 'gpt'), 'enable_thinking');
  assert.equal(resolveThinkingControl(' bogus ', 'aliyun', 'qwen3'), 'enable_thinking');
  assert.equal(resolveThinkingControl(undefined, 'openai', 'gpt'), 'none');
});

// frontend/src/utils/thinkingControl.ts modelSupportsThinking.
test('modelSupportsThinking gates the debug thinking toggle to chat models that send thinking params', () => {
  const modelOf = (provider: string, thinkingControl?: string, source = 'remote') => ({
    type: 'KnowledgeQA', source, name: 'qwen3',
    parameters: { provider, extra_config: thinkingControl ? { thinking_control: thinkingControl } : {} },
  });
  assert.equal(modelSupportsThinking(modelOf('aliyun', undefined, 'remote') as never), true);
  assert.equal(modelSupportsThinking(modelOf('openai', undefined) as never), false);
  assert.equal(modelSupportsThinking(modelOf('openai', 'enable_thinking') as never), true);
  assert.equal(modelSupportsThinking(modelOf('aliyun', undefined, 'local') as never), false);
  assert.equal(modelSupportsThinking({ ...modelOf('openai'), type: 'Embedding' } as never), false);
});

// frontend/src/utils/contextWindow.ts.
test('context window formatting matches the Vue token helpers', () => {
  assert.equal(DEFAULT_MODEL_CONTEXT_WINDOW, 200000);
  assert.equal(formatContextWindow(128000), '128K');
  assert.equal(formatContextWindow(200000), '200K');
  assert.equal(formatContextWindow(1048576), '1M');
  assert.equal(formatContextWindow(undefined), '200K');
  assert.equal(formatContextWindow(0), '200K');
  assert.equal(formatContextWindow(1300), '1300');
  assert.equal(effectiveContextWindow(512), 512);
  assert.equal(isDefaultContextWindow(undefined), true);
  assert.equal(isDefaultContextWindow(512), false);
});

// ModelEditorDialog.vue fallbackProviderOptions + providerOptions computed.
test('fallback provider catalogue filters by model type and resolves localized labels', () => {
  const t = createModelTranslator('zh-CN');
  const chat = fallbackProviderOptions(t, 'chat');
  assert.ok(chat.some((option) => option.value === 'openai' && option.label === 'OpenAI'));
  assert.ok(chat.some((option) => option.value === 'generic' && option.label === '自定义 (OpenAI兼容接口)'));
  const embedding = fallbackProviderOptions(t, 'embedding');
  assert.ok(!embedding.some((option) => option.value === 'volcengine'));
  assert.ok(embedding.some((option) => option.value === 'jina'));
  const rerank = fallbackProviderOptions(t, 'rerank');
  // Vue's fallback marks openai as rerank-capable in defaultUrls but not in
  // modelTypes, so the rerank list starts at aliyun (ModelEditorDialog.vue line 517).
  assert.deepEqual(rerank.map((option) => option.value), ['aliyun', 'siliconflow', 'jina', 'nvidia', 'generic']);

  const openai = FALLBACK_MODEL_PROVIDERS.find((option) => option.value === 'openai')!;
  assert.equal(providerDefaultUrl(openai.defaultUrls, 'chat'), 'https://api.openai.com/v1');
  assert.equal(providerDefaultUrl(openai.defaultUrls, 'asr'), 'https://api.openai.com/v1');
  assert.equal(providerDefaultUrl({}, 'chat'), '');
  // API text stays as the fallback when no translation exists.
  assert.equal(providerText(t, 'made-up-provider', 'label', 'From API'), 'From API');
  assert.equal(providerText(t, 'openai', 'description', 'From API') !== 'From API', true);
});

test('signed rerank providers swap credential labels (ModelEditorDialog.vue isSignedRerank)', () => {
  assert.equal(signedRerankProvider('rerank', 'lkeap'), 'lkeap');
  assert.equal(signedRerankProvider('rerank', 'volcengine'), 'volcengine');
  assert.equal(signedRerankProvider('rerank', 'openai'), null);
  assert.equal(signedRerankProvider('chat', 'lkeap'), null);
});

// ModelEditorDialog.vue getModelNamePlaceholder / getBaseUrlPlaceholder.
test('model name and base URL placeholders key off type and source', () => {
  assert.equal(modelNamePlaceholderKey('chat', 'remote'), 'model.editor.modelNamePlaceholder.remote');
  assert.equal(modelNamePlaceholderKey('chat', 'local'), 'model.editor.modelNamePlaceholder.local');
  assert.equal(modelNamePlaceholderKey('vllm', 'local'), 'model.editor.modelNamePlaceholder.localVllm');
  assert.equal(modelNamePlaceholderKey('vllm', 'remote'), 'model.editor.modelNamePlaceholder.remoteVllm');
  assert.equal(modelNamePlaceholderKey('asr', 'remote'), 'model.editor.modelNamePlaceholder.remoteAsr');
  assert.equal(baseUrlPlaceholderKey('chat'), 'model.editor.baseUrlPlaceholder');
  assert.equal(baseUrlPlaceholderKey('vllm'), 'model.editor.baseUrlPlaceholderVllm');
  assert.equal(baseUrlPlaceholderKey('asr'), 'model.editor.baseUrlPlaceholderAsr');
});

// ModelEditorDialog.vue resetForm + applyThinkingControlFromModelData + convertToLegacyFormat.
test('draft prefill restores Vue legacy fields including region and thinking resolution', () => {
  const draft = newModelDraft();
  assert.equal(draft.lkeapRegion, 'ap-guangzhou');
  assert.equal(draft.thinkingControl, 'chat_template_kwargs');
  assert.equal(draft.source, 'remote');

  const record = {
    id: 'm-1', name: 'qwen3', display_name: 'Qwen', type: 'KnowledgeQA', source: 'remote',
    parameters: {
      provider: 'aliyun', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
      supports_vision: true, context_window: 131072, max_concurrency: 4,
      custom_headers: { 'X-Trace': 'abc' },
      extra_config: { thinking_control: 'bogus' },
    },
    credentials: { api_key: { configured: true } },
  } as never;
  const prefilled = modelDraftFromRecord(record);
  assert.equal(prefilled.thinkingControl, 'enable_thinking');
  assert.equal(prefilled.supportsVision, true);
  assert.equal(prefilled.contextWindow, 131072);
  assert.equal(prefilled.maxConcurrency, 4);
  assert.deepEqual(prefilled.customHeaders, [{ key: 'X-Trace', value: 'abc' }]);
  assert.deepEqual(prefilled.credentials, { api_key: { configured: true } });
  assert.equal(storedThinkingControl(record), 'bogus');

  const rerankRecord = {
    id: 'm-2', name: 'lke-reranker-base', type: 'Rerank', source: 'remote',
    parameters: { provider: 'lkeap', extra_config: { region: 'ap-shanghai' } },
  } as never;
  const rerankDraft = modelDraftFromRecord(rerankRecord);
  assert.equal(rerankDraft.type, 'rerank');
  assert.equal(rerankDraft.lkeapRegion, 'ap-shanghai');
  assert.equal(rerankDraft.thinkingControl, '');

  const embeddingRecord = {
    id: 'm-3', name: 'bge', type: 'Embedding', source: 'local',
    parameters: { embedding_parameters: { dimension: 1024, supports_dimension_override: true } },
  } as never;
  const embeddingDraft = modelDraftFromRecord(embeddingRecord);
  assert.equal(embeddingDraft.dimension, 1024);
  assert.equal(embeddingDraft.supportsDimensionOverride, true);
  assert.equal(embeddingDraft.source, 'local');
});

test('translator layers shared i18n over the verbatim Vue locale tables', () => {
  const zh = createModelTranslator('zh-CN');
  assert.equal(zh('model.editor.addTitle'), '添加模型');
  assert.equal(zh('model.editor.dimensionDetected', { value: 1024 }), '检测成功，向量维度：1024');
  assert.equal(zh('modelSettings.confirmDelete', { name: 'bge' }), '确定删除模型「bge」吗？');
  // Keys already migrated into @weknora/i18n resolve there first.
  assert.equal(zh('modelSettings.title'), '模型配置');
  const en = createModelTranslator('en-US');
  assert.equal(en('model.editor.ollamaNotSupportRerank'), 'Ollama does not support ReRank models, please use a remote API instead');
  const ru = createModelTranslator('ru-RU');
  assert.equal(ru('model.editor.downloadLabel', { keyword: 'llama3' }).includes('llama3'), true);
  assert.equal(zh('model.editor.missing-key'), 'model.editor.missing-key');
});

test('formatModelSize renders Vue GB/MB labels', async () => {
  const { formatModelSize } = await import('./model-settings.ts');
  assert.equal(formatModelSize(undefined), '');
  assert.equal(formatModelSize(0), '');
  assert.equal(formatModelSize(512 * 1024 * 1024), '512 MB');
  assert.equal(formatModelSize(2.5 * 1024 * 1024 * 1024), '2.5 GB');
});
