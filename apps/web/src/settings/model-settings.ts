import { formatMessage, type Locale, type MessageValues } from '@weknora/i18n';
import type { ModelConfiguration } from '@weknora/api-client';

export type ModelType = 'chat' | 'embedding' | 'rerank' | 'vllm' | 'asr';
export type ThinkingControlValue = 'none' | 'chat_template_kwargs' | 'enable_thinking' | 'thinking_type';

export interface CustomHeaderItem {
  key: string;
  value: string;
}

export interface ModelCredentialMeta {
  configured?: boolean;
}

export interface ModelDraft {
  id?: string;
  name: string;
  displayName: string;
  type: ModelType;
  source: 'remote' | 'local';
  provider: string;
  baseUrl: string;
  dimension: number | '';
  supportsDimensionOverride: boolean;
  supportsVision: boolean;
  contextWindow: number | '';
  maxConcurrency: number | '';
  thinkingControl: string;
  customHeaders: CustomHeaderItem[];
  apiKey: string;
  appSecret: string;
  lkeapRegion: string;
  credentials?: Record<string, ModelCredentialMeta>;
}

function record(value: unknown): Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

export function modelType(value: Pick<ModelConfiguration, 'type'>): ModelType {
  const valueType = String(value.type ?? '').toLowerCase();
  if (valueType.includes('embedding')) return 'embedding';
  if (valueType.includes('rerank')) return 'rerank';
  if (valueType.includes('vllm') || valueType.includes('vlm')) return 'vllm';
  if (valueType.includes('asr') || valueType.includes('speech')) return 'asr';
  return 'chat';
}

export function backendModelType(type: ModelType): 'KnowledgeQA' | 'Embedding' | 'Rerank' | 'VLLM' | 'ASR' {
  return ({ chat: 'KnowledgeQA', embedding: 'Embedding', rerank: 'Rerank', vllm: 'VLLM', asr: 'ASR' } as const)[type];
}

/*
 * Thinking-control resolution — ported verbatim from
 * frontend/src/utils/thinkingControl.ts (mirrors backend internal/models/provider).
 */
export function isQwenThinkingModel(modelName: string): boolean {
  const lower = modelName.trim().toLowerCase();
  return (
    lower.startsWith('qwen3')
    || lower.startsWith('qwen-plus')
    || lower.startsWith('qwen-max')
    || lower.startsWith('qwen-turbo')
  );
}

export function isLkeapDeepSeekR1Model(modelName: string): boolean {
  return modelName.toLowerCase().includes('deepseek-r1');
}

const THINKING_CONTROL_VALUES: ThinkingControlValue[] = [
  'none',
  'chat_template_kwargs',
  'enable_thinking',
  'thinking_type',
];

export function defaultThinkingControl(provider: string, modelName = ''): ThinkingControlValue {
  const p = provider.trim().toLowerCase();
  const model = modelName.trim();

  switch (p) {
    case 'aliyun':
      return isQwenThinkingModel(model) ? 'enable_thinking' : 'none';
    case 'lkeap':
      if (model && isLkeapDeepSeekR1Model(model)) return 'none';
      return 'thinking_type';
    case 'generic':
    case 'nvidia':
    case 'litellm':
      return 'chat_template_kwargs';
    case 'volcengine':
      return 'thinking_type';
    default:
      return 'none';
  }
}

export function resolveThinkingControl(saved: string | undefined, provider: string, modelName = ''): ThinkingControlValue {
  const v = saved?.trim().toLowerCase();
  if (THINKING_CONTROL_VALUES.includes(v as ThinkingControlValue)) {
    return v as ThinkingControlValue;
  }
  return defaultThinkingControl(provider, modelName);
}

export function modelSupportsThinking(model: {
  type: string;
  source: string;
  name: string;
  parameters: {
    provider?: string;
    extra_config?: { thinking_control?: string };
  };
}): boolean {
  if (model.type !== 'KnowledgeQA' || model.source !== 'remote') return false;
  return resolveThinkingControl(
    model.parameters.extra_config?.thinking_control,
    model.parameters.provider || '',
    model.name || '',
  ) !== 'none';
}

/*
 * Context-window helpers — ported verbatim from frontend/src/utils/contextWindow.ts
 * (matches internal/types.DefaultMaxContextTokens).
 */
export const DEFAULT_MODEL_CONTEXT_WINDOW = 200000;

export function modelHasContextWindow(type?: string): boolean {
  return type === 'KnowledgeQA' || type === 'VLLM' || type === 'chat' || type === 'vllm';
}

export function effectiveContextWindow(tokens?: number | null): number {
  if (typeof tokens === 'number' && tokens > 0) {
    return tokens;
  }
  return DEFAULT_MODEL_CONTEXT_WINDOW;
}

export function isDefaultContextWindow(tokens?: number | null): boolean {
  return !(typeof tokens === 'number' && tokens > 0);
}

export function formatTokenCount(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) {
    return '';
  }
  const n = Math.round(tokens);
  if (n >= 1_000_000 && n % 1_000_000 === 0) {
    return `${n / 1_000_000}M`;
  }
  if (n >= 1000 && n % 1000 === 0) {
    return `${n / 1000}K`;
  }
  if (n >= 1024 && n % 1024 === 0) {
    const k = n / 1024;
    if (k >= 1024 && k % 1024 === 0) {
      return `${k / 1024}M`;
    }
    return `${k}K`;
  }
  return String(n);
}

export function formatContextWindow(tokens?: number | null): string {
  return formatTokenCount(effectiveContextWindow(tokens));
}

/*
 * Provider catalogue — the fallback list from ModelEditorDialog.vue's
 * fallbackProviderOptions (labels/descriptions resolve through i18n so the
 * card list, editor dropdown and debug drawer stay in sync).
 */
export interface ModelProviderOption {
  value: string;
  label: string;
  description: string;
  defaultUrls: Record<string, string>;
  modelTypes: string[];
}

export const FALLBACK_MODEL_PROVIDERS: ReadonlyArray<{
  value: string;
  defaultUrls: Record<string, string>;
  modelTypes: ModelType[];
}> = [
  {
    value: 'openai',
    defaultUrls: {
      chat: 'https://api.openai.com/v1',
      embedding: 'https://api.openai.com/v1',
      rerank: 'https://api.openai.com/v1',
      vllm: 'https://api.openai.com/v1',
      asr: 'https://api.openai.com/v1',
    },
    modelTypes: ['chat', 'embedding', 'vllm', 'asr'],
  },
  {
    value: 'azure_openai',
    defaultUrls: {
      chat: 'https://{resource}.openai.azure.com',
      embedding: 'https://{resource}.openai.azure.com',
      vllm: 'https://{resource}.openai.azure.com',
      asr: 'https://{resource}.openai.azure.com',
    },
    modelTypes: ['chat', 'embedding', 'vllm', 'asr'],
  },
  {
    value: 'aliyun',
    defaultUrls: {
      chat: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
      embedding: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
      rerank: 'https://dashscope.aliyuncs.com/api/v1/services/rerank/text-rerank/text-rerank',
      vllm: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    },
    modelTypes: ['chat', 'embedding', 'rerank', 'vllm'],
  },
  {
    value: 'zhipu',
    defaultUrls: {
      chat: 'https://open.bigmodel.cn/api/paas/v4',
      embedding: 'https://open.bigmodel.cn/api/paas/v4/embeddings',
      vllm: 'https://open.bigmodel.cn/api/paas/v4',
    },
    modelTypes: ['chat', 'embedding', 'vllm'],
  },
  {
    value: 'openrouter',
    defaultUrls: {
      chat: 'https://openrouter.ai/api/v1',
      embedding: 'https://openrouter.ai/api/v1',
    },
    modelTypes: ['chat', 'embedding'],
  },
  {
    value: 'litellm',
    defaultUrls: {
      chat: 'http://your_litellm_proxy/v1',
      embedding: 'http://your_litellm_proxy/v1',
      vllm: 'http://your_litellm_proxy/v1',
    },
    modelTypes: ['chat', 'embedding', 'vllm'],
  },
  {
    value: 'requesty',
    defaultUrls: {
      chat: 'https://router.requesty.ai/v1',
      embedding: 'https://router.requesty.ai/v1',
    },
    modelTypes: ['chat', 'embedding'],
  },
  {
    value: 'gemini',
    defaultUrls: {
      chat: 'https://generativelanguage.googleapis.com/v1beta/openai',
      embedding: 'https://generativelanguage.googleapis.com/v1beta',
    },
    modelTypes: ['chat', 'embedding'],
  },
  {
    value: 'siliconflow',
    defaultUrls: {
      chat: 'https://api.siliconflow.cn/v1',
      embedding: 'https://api.siliconflow.cn/v1',
      rerank: 'https://api.siliconflow.cn/v1',
    },
    modelTypes: ['chat', 'embedding', 'rerank'],
  },
  {
    value: 'jina',
    defaultUrls: {
      embedding: 'https://api.jina.ai/v1',
      rerank: 'https://api.jina.ai/v1',
    },
    modelTypes: ['embedding', 'rerank'],
  },
  {
    value: 'nvidia',
    defaultUrls: {
      chat: 'https://integrate.api.nvidia.com/v1',
      embedding: 'https://integrate.api.nvidia.com/v1',
      rerank: 'https://ai.api.nvidia.com/v1/retrieval/nvidia/reranking',
      vllm: 'https://integrate.api.nvidia.com/v1',
    },
    modelTypes: ['chat', 'embedding', 'rerank', 'vllm'],
  },
  {
    value: 'novita',
    defaultUrls: {
      chat: 'https://api.novita.ai/openai/v1',
      embedding: 'https://api.novita.ai/openai/v1',
      vllm: 'https://api.novita.ai/openai/v1',
    },
    modelTypes: ['chat', 'embedding', 'vllm'],
  },
  {
    value: 'generic',
    defaultUrls: {},
    modelTypes: ['chat', 'embedding', 'rerank', 'vllm', 'asr'],
  },
];

export function providerLabelKey(provider: string): string {
  return `model.editor.providers.${provider}.label`;
}

export function providerDescriptionKey(provider: string): string {
  return `model.editor.providers.${provider}.description`;
}

/**
 * Renders the localized label/description for a provider id, falling back to
 * the API-provided text when no translation exists (ModelEditorDialog.vue
 * providerOptions computed: i18n text wins, API text is the fallback).
 */
export function providerText(t: (key: string) => string, provider: string, field: 'label' | 'description', fallback: string): string {
  const key = field === 'label' ? providerLabelKey(provider) : providerDescriptionKey(provider);
  const viaT = t(key);
  return viaT === key ? fallback : viaT;
}

export function fallbackProviderOptions(t: (key: string) => string, type: ModelType): ModelProviderOption[] {
  return FALLBACK_MODEL_PROVIDERS
    .filter((provider) => provider.modelTypes.includes(type))
    .map((provider) => ({
      value: provider.value,
      label: providerText(t, provider.value, 'label', provider.value),
      description: providerText(t, provider.value, 'description', ''),
      defaultUrls: provider.defaultUrls,
      modelTypes: provider.modelTypes,
    }));
}

export function providerDefaultUrl(defaultUrls: Record<string, string> | undefined, type: ModelType): string {
  const url = defaultUrls?.[type];
  return typeof url === 'string' && url ? url : '';
}

/** LKEAP / Volcengine rerank sign requests with AK/SK instead of an API key. */
export function signedRerankProvider(type: ModelType, provider: string): 'lkeap' | 'volcengine' | null {
  if (type !== 'rerank') return null;
  if (provider === 'lkeap') return 'lkeap';
  if (provider === 'volcengine') return 'volcengine';
  return null;
}

export function modelNamePlaceholderKey(type: ModelType, source: 'remote' | 'local'): string {
  if (type === 'vllm') return source === 'local' ? 'model.editor.modelNamePlaceholder.localVllm' : 'model.editor.modelNamePlaceholder.remoteVllm';
  if (type === 'asr') return 'model.editor.modelNamePlaceholder.remoteAsr';
  return source === 'local' ? 'model.editor.modelNamePlaceholder.local' : 'model.editor.modelNamePlaceholder.remote';
}

export function baseUrlPlaceholderKey(type: ModelType): string {
  if (type === 'vllm') return 'model.editor.baseUrlPlaceholderVllm';
  if (type === 'asr') return 'model.editor.baseUrlPlaceholderAsr';
  return 'model.editor.baseUrlPlaceholder';
}

export function customHeadersMap(headers: readonly CustomHeaderItem[]): Record<string, string> {
  const map: Record<string, string> = {};
  for (const item of headers) {
    const key = (item?.key ?? '').trim();
    const value = (item?.value ?? '').trim();
    if (key && value) map[key] = value;
  }
  return map;
}

export function storedThinkingControl(model: ModelConfiguration): string | undefined {
  const parameters = record((model as Record<string, unknown>).parameters);
  const extraConfig = record(parameters.extra_config);
  return typeof extraConfig.thinking_control === 'string' ? extraConfig.thinking_control : undefined;
}

export function modelDraftFromRecord(value: ModelConfiguration): ModelDraft {
  const row = value as Record<string, unknown>;
  const parameters = record(row.parameters);
  const embedding = record(parameters.embedding_parameters);
  const extraConfig = record(parameters.extra_config);
  const type = modelType(value);
  const provider = typeof parameters.provider === 'string' ? parameters.provider : 'generic';
  const source = row.source === 'local' ? 'local' : 'remote';
  return {
    id: value.id,
    name: value.name,
    displayName: typeof row.display_name === 'string' ? row.display_name : '',
    type,
    source,
    provider,
    baseUrl: typeof parameters.base_url === 'string' ? parameters.base_url : '',
    dimension: typeof embedding.dimension === 'number' ? embedding.dimension : '',
    supportsDimensionOverride: embedding.supports_dimension_override === true,
    supportsVision: parameters.supports_vision === true,
    contextWindow: typeof parameters.context_window === 'number' ? parameters.context_window : '',
    maxConcurrency: typeof parameters.max_concurrency === 'number' ? parameters.max_concurrency : '',
    thinkingControl: type === 'chat' && source === 'remote'
      ? resolveThinkingControl(typeof extraConfig.thinking_control === 'string' ? extraConfig.thinking_control : undefined, provider, value.name)
      : '',
    customHeaders: Object.entries(record(parameters.custom_headers))
      .filter(([, item]) => typeof item === 'string')
      .map(([key, item]) => ({ key, value: String(item) })),
    apiKey: '',
    appSecret: '',
    lkeapRegion: typeof extraConfig.region === 'string' && extraConfig.region ? extraConfig.region : 'ap-guangzhou',
    credentials: credentialMetaOf(row.credentials),
  };
}

function credentialMetaOf(value: unknown): Record<string, ModelCredentialMeta> | undefined {
  const meta = record(value);
  const entries = Object.entries(meta).flatMap(([field, item]) => {
    const configured = record(item).configured;
    return typeof configured === 'boolean' ? [[field, { configured }] as const] : [];
  });
  return entries.length > 0 ? Object.fromEntries(entries) : undefined;
}

export function newModelDraft(): ModelDraft {
  return {
    name: '',
    displayName: '',
    type: 'chat',
    source: 'remote',
    provider: 'generic',
    baseUrl: '',
    dimension: '',
    supportsDimensionOverride: false,
    supportsVision: false,
    contextWindow: '',
    maxConcurrency: '',
    thinkingControl: defaultThinkingControl('generic', ''),
    customHeaders: [],
    apiKey: '',
    appSecret: '',
    lkeapRegion: 'ap-guangzhou',
  };
}

export type ModelDraftValidationError =
  | 'nameRequired'
  | 'nameTooLong'
  | 'displayNameTooLong'
  | 'baseUrlRequired'
  | 'baseUrlInvalid'
  | 'dimensionInvalid';

/**
 * Mirrors the submit gating of ModelSettings.vue handleModelSave (lines 549-583):
 * name required and <=100, display name <=100, remote sources need a valid
 * base URL, embedding models need a 128-4096 dimension.
 */
export function validateModelDraft(draft: ModelDraft): ModelDraftValidationError[] {
  const errors: ModelDraftValidationError[] = [];
  if (!draft.name.trim()) errors.push('nameRequired');
  else if (draft.name.trim().length > 100) errors.push('nameTooLong');
  if (draft.displayName.trim().length > 100) errors.push('displayNameTooLong');
  if (draft.source === 'remote') {
    if (!draft.baseUrl.trim()) errors.push('baseUrlRequired');
    else {
      try { new URL(draft.baseUrl.trim()); } catch { errors.push('baseUrlInvalid'); }
    }
  }
  if (draft.type === 'embedding' && !(typeof draft.dimension === 'number' && draft.dimension >= 128 && draft.dimension <= 4096)) {
    errors.push('dimensionInvalid');
  }
  return errors;
}

/**
 * Per-field blur validation with the exact ModelEditorDialog.vue rules and
 * copy (lines 907-946): name required/empty/max-100 and base URL
 * required/empty/URL-format, each with its own message key so the error
 * renders under the field that owns it.
 */
export type ModelFieldKey = 'name' | 'baseUrl';
export function modelFieldErrorKey(field: ModelFieldKey, draft: ModelDraft): string | null {
  if (field === 'name') {
    const value = typeof draft.name === 'string' ? draft.name : '';
    if (value.length === 0) return 'model.editor.validation.modelNameRequired';
    if (!value.trim()) return 'model.editor.validation.modelNameEmpty';
    if (value.trim().length > 100) return 'model.editor.validation.modelNameMax';
    return null;
  }
  const value = typeof draft.baseUrl === 'string' ? draft.baseUrl : '';
  if (value.length === 0) return 'model.editor.validation.baseUrlRequired';
  if (!value.trim()) return 'model.editor.validation.baseUrlEmpty';
  try { new URL(value.trim()); } catch {
    return 'model.editor.validation.baseUrlInvalid';
  }
  return null;
}

/**
 * ModelSettings.vue watches uiStore.settingsInitialSubSection (lines 329-337)
 * and lands on the matching type tab; unknown values keep the Vue default of
 * showing every model.
 */
const MODEL_TAB_TYPES: ReadonlyArray<ModelType> = ["chat", "embedding", "rerank", "vllm", "asr"];
export function subsectionToFilter(value: string | null | undefined): ModelType | null {
  if (!value) return null;
  // Vue's knowledge-base model entry opens the KnowledgeQA model flow, while
  // ModelSettings.vue groups backend type KnowledgeQA under its chat tab.
  if (value === "knowledgeqa") return "chat";
  return (MODEL_TAB_TYPES as ReadonlyArray<string>).includes(value) ? value as ModelType : null;
}
export function modelValidationErrorKey(error: ModelDraftValidationError): string {
  return `modelSettings.toasts.${error}` as const;
}

/**
 * Builds the create/update payload exactly like ModelSettings.vue
 * handleModelSave (lines 585-655): a fresh parameters object (the backend
 * replaces parameters on update and preserves credentials server-side),
 * credentials only flow inside the create payload, empty custom-header rows
 * are dropped, and rerank stays remote-only.
 */
export function modelPayload(draft: ModelDraft): Record<string, unknown> {
  const isEdit = Boolean(draft.id);
  const parameters: Record<string, unknown> = {
    base_url: draft.baseUrl.trim(),
    provider: draft.provider || '',
  };
  const apiKey = draft.apiKey.trim();
  if (!isEdit && apiKey) parameters.api_key = apiKey;
  const appSecret = draft.appSecret.trim();
  if (!isEdit && appSecret) parameters.app_secret = appSecret;
  const extraConfig: Record<string, string> = {};
  if (draft.provider === 'lkeap' && draft.type === 'rerank') {
    extraConfig.region = (draft.lkeapRegion || 'ap-guangzhou').trim();
  }
  if (draft.type === 'chat' && draft.source === 'remote' && draft.thinkingControl) {
    extraConfig.thinking_control = draft.thinkingControl;
  }
  if (Object.keys(extraConfig).length > 0) parameters.extra_config = extraConfig;
  const headers = customHeadersMap(draft.customHeaders);
  if (Object.keys(headers).length > 0) parameters.custom_headers = headers;
  if (draft.type === 'embedding' && typeof draft.dimension === 'number') {
    parameters.embedding_parameters = {
      dimension: draft.dimension,
      truncate_prompt_tokens: 0,
      supports_dimension_override: draft.supportsDimensionOverride,
    };
  }
  if (draft.type === 'vllm') parameters.supports_vision = true;
  else if (draft.type === 'chat') parameters.supports_vision = draft.supportsVision;
  if ((draft.type === 'chat' || draft.type === 'vllm') && Number(draft.contextWindow) >= 1024) {
    parameters.context_window = Math.round(Number(draft.contextWindow));
  }
  if (['chat', 'embedding', 'vllm'].includes(draft.type) && Number(draft.maxConcurrency) > 0) {
    parameters.max_concurrency = Number(draft.maxConcurrency);
  }
  return {
    name: draft.name.trim(),
    display_name: draft.displayName.trim(),
    description: '',
    type: backendModelType(draft.type),
    source: draft.type === 'rerank' ? 'remote' : draft.source,
    parameters,
  };
}

export function formatModelSize(bytes: unknown): string {
  if (typeof bytes !== 'number' || !Number.isFinite(bytes) || bytes === 0) return '';
  const gb = bytes / (1024 * 1024 * 1024);
  return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / (1024 * 1024)).toFixed(0)} MB`;
}

export type ModelMessages = (key: string, values?: MessageValues) => string;

/**
 * Layered translator for the model settings surface: keys already migrated
 * into @weknora/i18n resolve there first; everything else falls back to the
 * verbatim Vue locale tables below (all five locales, byte-exact from
 * frontend/src/i18n/locales/*.ts). When the shared package gains a key, the
 * shared translation automatically wins.
 */
export function modelFormatMessage(locale: Locale, key: string, values?: MessageValues): string {
  const shared = formatMessage(locale, key);
  if (shared !== key) return values ? formatMessage(locale, key, values) : shared;
  const template = MODEL_EDITOR_MESSAGES[locale]?.[key] ?? MODEL_EDITOR_MESSAGES['zh-CN']?.[key];
  if (template === undefined) return key;
  if (!values) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) => (
    values[name] === undefined ? match : String(values[name])
  ));
}

export function createModelTranslator(locale: Locale): ModelMessages {
  return (key, values) => modelFormatMessage(locale, key, values);
}

/*
 * Verbatim locale tables for the model settings surface, extracted from
 * frontend/src/i18n/locales/{zh-CN,en-US,ja-JP,ko-KR,ru-RU}.ts:
 * the full model.editor.* subtree, model.modelName / model.searchPlaceholder,
 * the full modelSettings.* subtree, the common.* labels used here, and the
 * WeKnoraCloud credential hints. They only back keys that @weknora/i18n has
 * not migrated yet — modelFormatMessage prefers the shared package, so once
 * a key lands there the shared translation wins automatically.
 */
export const MODEL_EDITOR_MESSAGES: Record<Locale, Record<string, string>> = {
  "zh-CN": {
    "common.all": "全部",
    "common.cancel": "取消",
    "common.close": "关闭",
    "common.copied": "已复制",
    "common.copy": "复制",
    "common.delete": "删除",
    "common.edit": "编辑",
    "common.no": "否",
    "common.refresh": "刷新",
    "common.yes": "是",
    "model.editor.addTitle": "添加模型",
    "model.editor.apiKeyOptional": "API Key（可选）",
    "model.editor.apiKeyPlaceholder": "输入 API Key",
    "model.editor.baseUrlLabel": "Base URL",
    "model.editor.baseUrlPlaceholder": "例如：https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderAsr": "例如：https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderVllm": "例如：http://localhost:11434/v1",
    "model.editor.checkDimension": "检测维度",
    "model.editor.connectionConfigError": "连接失败，请检查配置",
    "model.editor.connectionFailed": "连接失败",
    "model.editor.connectionSuccess": "连接成功",
    "model.editor.contextWindowDefaultHint": "未设置，使用默认 {value}",
    "model.editor.contextWindowDesc": "该模型一次请求能容纳的 token 数。智能体压缩对话历史会按此上限工作。留空则使用默认 200000（200K）。请按厂商文档填写真实值，填大会导致压缩不触发、上游直接拒绝请求。",
    "model.editor.contextWindowLabel": "上下文窗口",
    "model.editor.contextWindowPlaceholder": "默认 {value}",
    "model.editor.contextWindowTokens": "{count} tokens",
    "model.editor.customHeadersAdd": "添加请求头",
    "model.editor.customHeadersDesc": "调用远程模型 API 时附加的 HTTP 请求头，常用于企业网关鉴权、链路追踪等场景；Authorization、Content-Type 等保留头会被自动忽略。",
    "model.editor.customHeadersKeyPlaceholder": "Header 名称",
    "model.editor.customHeadersLabel": "自定义请求头（可选）",
    "model.editor.customHeadersValuePlaceholder": "Header 值",
    "model.editor.description.asr": "配置用于语音识别和音频转录的语音转文本模型",
    "model.editor.description.chat": "配置用于对话的大语言模型",
    "model.editor.description.default": "配置模型信息",
    "model.editor.description.embedding": "配置用于文本向量化的嵌入模型",
    "model.editor.description.rerank": "配置用于结果重排序的模型",
    "model.editor.description.vllm": "配置用于视觉理解和多模态的视觉语言模型",
    "model.editor.dimensionDetected": "检测成功，向量维度：{value}",
    "model.editor.dimensionFailed": "检测失败，请手动输入维度",
    "model.editor.dimensionHint": "模型已选择，点击\"检测维度\"按钮自动获取向量维度",
    "model.editor.dimensionLabel": "向量维度",
    "model.editor.dimensionOverrideDesc": "仅在确认该模型支持 dimensions 参数时开启；默认只使用检测到的实际维度。",
    "model.editor.dimensionOverrideLabel": "自定义输出维度",
    "model.editor.dimensionPlaceholder": "例如：1536",
    "model.editor.displayNameDesc": "仅用于界面展示，实际调用仍使用上面的模型名称。",
    "model.editor.displayNameLabel": "显示名称（可选）",
    "model.editor.displayNamePlaceholder": "例如：客服问答模型",
    "model.editor.downloadCompleted": "{name} 下载完成",
    "model.editor.downloadFailed": "{name} 下载失败",
    "model.editor.downloadLabel": "下载: {keyword}",
    "model.editor.downloadStartFailed": "启动下载失败",
    "model.editor.downloadStarted": "开始下载 {name}",
    "model.editor.editTitle": "编辑模型",
    "model.editor.fillModelAndUrl": "请先填写模型标识和 Base URL",
    "model.editor.goToOllamaSettings": "查看设置",
    "model.editor.listRefreshed": "列表已刷新",
    "model.editor.lkeap.regionDesc": "RunRerank 支持 ap-beijing、ap-guangzhou 等，默认 ap-guangzhou",
    "model.editor.lkeap.regionLabel": "地域",
    "model.editor.lkeap.regionPlaceholder": "ap-guangzhou",
    "model.editor.lkeap.rerankCredentialHint": "Rerank 使用腾讯云 API 签名（非 OpenAI API Key）。请在云 API 密钥控制台创建 SecretId/SecretKey。",
    "model.editor.lkeap.secretIdLabel": "SecretId",
    "model.editor.lkeap.secretIdPlaceholder": "腾讯云 API 密钥 SecretId",
    "model.editor.lkeap.secretKeyLabel": "SecretKey",
    "model.editor.lkeap.secretKeyPlaceholder": "腾讯云 API 密钥 SecretKey",
    "model.editor.loadModelListFailed": "加载模型列表失败",
    "model.editor.maxConcurrencyDesc": "限制文档入库/富化等后台任务对该模型的并发调用数（按模型全副本共享）。0 或留空表示沿用全局默认；不影响交互式对话。",
    "model.editor.maxConcurrencyLabel": "后台并发上限",
    "model.editor.maxConcurrencyPlaceholder": "0 表示使用全局默认",
    "model.editor.modelNamePlaceholder.local": "例如：llama2:latest",
    "model.editor.modelNamePlaceholder.localVllm": "例如：llava:latest",
    "model.editor.modelNamePlaceholder.remote": "例如：gpt-4, claude-3-opus",
    "model.editor.modelNamePlaceholder.remoteAsr": "例如：whisper-1",
    "model.editor.modelNamePlaceholder.remoteVllm": "例如：gpt-4-vision-preview",
    "model.editor.ollamaNotSupportRerank": "Ollama 不支持 ReRank 模型，请使用远程接口配置",
    "model.editor.ollamaUnavailable": "Ollama服务不可用，无法选择本地模型",
    "model.editor.providerLabel": "服务商",
    "model.editor.providerPlaceholder": "选择模型服务商",
    "model.editor.providers.aliyun.description": "qwen-plus, tongyi-embedding-vision-plus, qwen3-rerank, etc.",
    "model.editor.providers.aliyun.label": "阿里云 DashScope",
    "model.editor.providers.anthropic.description": "Claude models via native Anthropic Messages API",
    "model.editor.providers.anthropic.label": "Anthropic",
    "model.editor.providers.azure_openai.description": "Microsoft Azure 上的 OpenAI 服务",
    "model.editor.providers.azure_openai.label": "Azure OpenAI",
    "model.editor.providers.deepseek.description": "deepseek-chat, deepseek-reasoner 等",
    "model.editor.providers.deepseek.label": "DeepSeek",
    "model.editor.providers.gemini.description": "gemini-3-flash-preview, gemini-2.5-pro 等",
    "model.editor.providers.gemini.label": "Google Gemini",
    "model.editor.providers.generic.description": "Generic API endpoint (OpenAI-compatible)",
    "model.editor.providers.generic.label": "自定义 (OpenAI兼容接口)",
    "model.editor.providers.gpustack.description": "Choose your deployed model on GPUStack",
    "model.editor.providers.gpustack.label": "GPUStack",
    "model.editor.providers.hunyuan.description": "hunyuan-pro, hunyuan-standard, hunyuan-embedding, etc.",
    "model.editor.providers.hunyuan.label": "腾讯混元 Hunyuan",
    "model.editor.providers.jina.description": "jina-clip-v1, jina-embeddings-v2-base-zh, etc.",
    "model.editor.providers.jina.label": "Jina",
    "model.editor.providers.litellm.description": "自托管代理，统一接入 OpenAI、Anthropic、Gemini、Bedrock 等 100+ 厂商。请将占位 URL 换成可访问地址；localhost 需加入 SSRF_WHITELIST。",
    "model.editor.providers.litellm.label": "LiteLLM",
    "model.editor.providers.lkeap.description": "DeepSeek-R1、DeepSeek-V3、lke-reranker-base 等",
    "model.editor.providers.lkeap.label": "腾讯云 LKEAP",
    "model.editor.providers.longcat.description": "LongCat-Flash-Chat, LongCat-Flash-Thinking, etc.",
    "model.editor.providers.longcat.label": "LongCat AI",
    "model.editor.providers.mimo.description": "mimo-v2-flash",
    "model.editor.providers.mimo.label": "小米 MiMo",
    "model.editor.providers.minimax.description": "MiniMax-M3, MiniMax-M2.7, MiniMax-M2.7-highspeed 等",
    "model.editor.providers.minimax.label": "MiniMax",
    "model.editor.providers.modelscope.description": "Qwen/Qwen3-8B, Qwen/Qwen3-Embedding-8B, etc.",
    "model.editor.providers.modelscope.label": "魔搭 ModelScope",
    "model.editor.providers.moonshot.description": "kimi-k2-turbo-preview, moonshot-v1-8k-vision-preview, etc.",
    "model.editor.providers.moonshot.label": "月之暗面 Moonshot",
    "model.editor.providers.novita.description": "moonshotai/kimi-k2.5, zai-org/glm-5, minimax/minimax-m2.7, qwen/qwen3-embedding-0.6b 等",
    "model.editor.providers.novita.label": "Novita AI",
    "model.editor.providers.nvidia.description": "deepseek-ai-deepseek-v3_1, nv-embed-v1, rerank-qa-mistral-4b, etc.",
    "model.editor.providers.nvidia.label": "NVIDIA",
    "model.editor.providers.openai.description": "gpt-5.2, gpt-5-mini, etc.",
    "model.editor.providers.openai.label": "OpenAI",
    "model.editor.providers.openrouter.description": "openai/gpt-5.2-chat, google/gemini-3-flash-preview, etc.",
    "model.editor.providers.openrouter.label": "OpenRouter",
    "model.editor.providers.qianfan.description": "ernie-5.0-thinking-preview, embedding-v1, bce-reranker-base, etc.",
    "model.editor.providers.qianfan.label": "百度千帆 Baidu Cloud",
    "model.editor.providers.qiniu.description": "deepseek/deepseek-v3.2-251201, z-ai/glm-4.7, etc.",
    "model.editor.providers.qiniu.label": "七牛云 Qiniu",
    "model.editor.providers.requesty.description": "openai/gpt-4o-mini, anthropic/claude-sonnet-4-5, etc.",
    "model.editor.providers.requesty.label": "Requesty",
    "model.editor.providers.siliconflow.description": "deepseek-ai/DeepSeek-V3.1, etc.",
    "model.editor.providers.siliconflow.label": "硅基流动 SiliconFlow",
    "model.editor.providers.volcengine.description": "doubao-1-5-pro-32k-250115, doubao-embedding-vision-250615, etc.",
    "model.editor.providers.volcengine.label": "火山引擎 Volcengine",
    "model.editor.providers.zhipu.description": "glm-4.7, embedding-3, rerank, etc.",
    "model.editor.providers.zhipu.label": "智谱 BigModel",
    "model.editor.refreshList": "刷新列表",
    "model.editor.remoteBaseUrlRequired": "Remote API 类型必须填写 Base URL",
    "model.editor.remoteDimensionDetected": "检测到向量维度：{value}",
    "model.editor.sectionAdvanced": "高级选项",
    "model.editor.sectionProvider": "接入配置",
    "model.editor.sectionSource": "模型来源",
    "model.editor.sectionType": "模型类型",
    "model.editor.sourceLabel": "模型来源",
    "model.editor.sourceLocal": "Ollama",
    "model.editor.sourceRemote": "API",
    "model.editor.supportsVisionDesc": "模型是否支持图片等多模态输入",
    "model.editor.supportsVisionLabel": "支持视觉/多模态",
    "model.editor.testConnection": "测试连接",
    "model.editor.testing": "测试中...",
    "model.editor.thinkingControl.chatTemplateKwargs.hint": "自定义 OpenAI 兼容、NVIDIA NIM、vLLM / 本地 Qwen 部署",
    "model.editor.thinkingControl.chatTemplateKwargs.label": "chat_template_kwargs",
    "model.editor.thinkingControl.enableThinking.hint": "阿里云 DashScope：qwen3、qwen-plus、qwen-max、qwen-turbo",
    "model.editor.thinkingControl.enableThinking.label": "enable_thinking",
    "model.editor.thinkingControl.none.hint": "智能体「思考模式」开关不生效，不会在请求中写入思考相关参数",
    "model.editor.thinkingControl.none.label": "不写入思考参数",
    "model.editor.thinkingControl.thinkingType.hint": "火山引擎 Ark；腾讯云 LKEAP（DeepSeek V3 等，选 LKEAP 时默认此项；R1 请改「不写入」）",
    "model.editor.thinkingControl.thinkingType.label": "thinking.type",
    "model.editor.thinkingControlDesc": "决定智能体「思考模式」开/关时如何写入 API。已尝试按厂商/模型预选，若与实际情况不符请按 API 文档手动修改；选「不写入」时，智能体「思考模式」开关不生效。",
    "model.editor.thinkingControlLabel": "思考模式参数格式",
    "model.editor.typeLabel": "模型类型",
    "model.editor.unsupportedModelType": "不支持的模型类型",
    "model.editor.validation.baseUrlEmpty": "Base URL 不能为空",
    "model.editor.validation.baseUrlInvalid": "Base URL 格式不正确，请输入有效的 URL",
    "model.editor.validation.baseUrlRequired": "请输入 Base URL",
    "model.editor.validation.modelNameEmpty": "模型名称不能为空",
    "model.editor.validation.modelNameMax": "模型名称不能超过100个字符",
    "model.editor.validation.modelNameRequired": "请输入模型名称",
    "model.editor.volcengine.accessKeyLabel": "Access Key ID",
    "model.editor.volcengine.accessKeyPlaceholder": "火山引擎访问密钥 Access Key ID",
    "model.editor.volcengine.rerankCredentialHint": "Rerank 使用 VikingDB AK/SK 签名（非方舟 API Key），模型建议填写 doubao-seed-rerank。",
    "model.editor.volcengine.secretKeyLabel": "Secret Access Key",
    "model.editor.volcengine.secretKeyPlaceholder": "火山引擎访问密钥 Secret Access Key",
    "model.modelName": "模型名称",
    "model.searchPlaceholder": "搜索模型...",
    "modelSettings.actions.addModel": "添加模型",
    "modelSettings.actions.debugModel": "模型测试",
    "modelSettings.asr.desc": "配置用于语音识别和音频转录的语音转文本模型（如 OpenAI Whisper）",
    "modelSettings.asr.empty": "暂无 ASR 语音模型",
    "modelSettings.asr.title": "ASR 语音模型",
    "modelSettings.builtinModels.description": "内置模型对所有空间可见，敏感信息会被隐藏，且不可编辑或删除。",
    "modelSettings.builtinModels.descriptionAdmin": "内置模型对所有空间可见。系统管理员可编辑配置和凭据；删除仍由部署配置管理。",
    "modelSettings.builtinModels.title": "内置模型",
    "modelSettings.builtinModels.viewGuide": "查看内置模型管理指南",
    "modelSettings.builtinTag": "内置",
    "modelSettings.chat.desc": "配置用于对话的大语言模型",
    "modelSettings.chat.empty": "暂无对话模型",
    "modelSettings.chat.title": "对话模型",
    "modelSettings.confirmDelete": "确定删除模型「{name}」吗？",
    "modelSettings.copySuffix": " 副本",
    "modelSettings.debug.audioFile": "音频文件",
    "modelSettings.debug.chooseFile": "选择文件",
    "modelSettings.debug.copyResult": "复制结果",
    "modelSettings.debug.description": "向已配置的模型发送真实请求，查看响应与耗时",
    "modelSettings.debug.documents": "候选文档",
    "modelSettings.debug.documentsHint": "每个非空行会作为一个独立文档发送给 ReRank 模型",
    "modelSettings.debug.documentsPlaceholder": "每行输入一个候选文档",
    "modelSettings.debug.embeddingInput": "待向量化文本",
    "modelSettings.debug.embeddingPlaceholder": "输入要生成 Embedding 的文本",
    "modelSettings.debug.failed": "调用失败",
    "modelSettings.debug.groupInput": "测试输入",
    "modelSettings.debug.groupModel": "选择模型",
    "modelSettings.debug.groupResult": "运行结果",
    "modelSettings.debug.history": "历史记录",
    "modelSettings.debug.imageFile": "图片文件",
    "modelSettings.debug.metrics.answerChars": "回答字符数",
    "modelSettings.debug.metrics.dimension": "向量维度",
    "modelSettings.debug.metrics.reasoningChars": "推理字符数",
    "modelSettings.debug.metrics.reasoningReturned": "返回推理内容",
    "modelSettings.debug.metrics.resultCount": "结果数量",
    "modelSettings.debug.metrics.segmentCount": "分段数量",
    "modelSettings.debug.metrics.textChars": "转写字符数",
    "modelSettings.debug.model": "模型",
    "modelSettings.debug.modelPlaceholder": "请选择要测试的模型",
    "modelSettings.debug.modelType": "模型类型",
    "modelSettings.debug.noModelsForType": "当前类型暂无已添加模型",
    "modelSettings.debug.parameters": "请求参数",
    "modelSettings.debug.query": "输入内容",
    "modelSettings.debug.queryPlaceholder": "输入要发送给模型的内容",
    "modelSettings.debug.rawResponse": "响应内容",
    "modelSettings.debug.requestFailed": "模型测试请求失败",
    "modelSettings.debug.requestPreview": "请求预览",
    "modelSettings.debug.run": "运行测试",
    "modelSettings.debug.runLabel": "第 {n} 次运行",
    "modelSettings.debug.success": "调用成功",
    "modelSettings.debug.systemPrompt": "System Prompt",
    "modelSettings.debug.systemPromptPlaceholder": "可选，输入系统提示词",
    "modelSettings.debug.thinkOff": "思考关闭",
    "modelSettings.debug.thinkOn": "思考开启",
    "modelSettings.debug.thinking": "思考模式",
    "modelSettings.debug.thinkingDesc": "仅对支持思考模式的模型生效",
    "modelSettings.debug.title": "模型测试",
    "modelSettings.debug.vlmPrompt": "图片提示词",
    "modelSettings.debug.vlmPromptPlaceholder": "例如：请详细描述这张图片",
    "modelSettings.description": "管理不同类型的 AI 模型，支持 Ollama 本地模型和远程 API",
    "modelSettings.embedding.desc": "配置用于文本向量化的嵌入模型",
    "modelSettings.embedding.empty": "暂无 Embedding 模型",
    "modelSettings.embedding.title": "Embedding 模型",
    "modelSettings.rerank.desc": "配置用于结果重排序的模型",
    "modelSettings.rerank.empty": "暂无 ReRank 模型",
    "modelSettings.rerank.title": "ReRank 模型",
    "modelSettings.source.custom": "自定义",
    "modelSettings.source.openaiCompatible": "OpenAI兼容",
    "modelSettings.source.remote": "Remote",
    "modelSettings.title": "模型配置",
    "modelSettings.toasts.added": "模型已添加",
    "modelSettings.toasts.baseUrlInvalid": "Base URL 格式不正确，请输入有效的 URL",
    "modelSettings.toasts.baseUrlRequired": "Remote API 类型必须填写 Base URL",
    "modelSettings.toasts.builtinCannotCopy": "内置模型不能复制",
    "modelSettings.toasts.builtinCannotDelete": "内置模型不能删除",
    "modelSettings.toasts.builtinCannotEdit": "内置模型不能编辑",
    "modelSettings.toasts.copied": "模型已复制",
    "modelSettings.toasts.copyFailed": "复制模型失败",
    "modelSettings.toasts.deleteFailed": "删除模型失败",
    "modelSettings.toasts.deleted": "模型已删除",
    "modelSettings.toasts.dimensionInvalid": "Embedding 模型必须填写有效的向量维度（128-4096）",
    "modelSettings.toasts.displayNameTooLong": "显示名称不能超过100个字符",
    "modelSettings.toasts.nameRequired": "模型名称不能为空",
    "modelSettings.toasts.nameTooLong": "模型名称不能超过100个字符",
    "modelSettings.toasts.saveFailed": "保存模型失败",
    "modelSettings.toasts.updated": "模型已更新",
    "modelSettings.typeShort.asr": "语音",
    "modelSettings.typeShort.chat": "对话",
    "modelSettings.typeShort.embedding": "Embedding",
    "modelSettings.typeShort.rerank": "ReRank",
    "modelSettings.typeShort.vllm": "视觉",
    "modelSettings.usage.agents": "智能体（{count}）",
    "modelSettings.usage.bindings.asr_model": "语音识别模型",
    "modelSettings.usage.bindings.chat_model": "对话模型",
    "modelSettings.usage.bindings.embedding_model": "Embedding 模型",
    "modelSettings.usage.bindings.extract_model": "记忆提取模型",
    "modelSettings.usage.bindings.follow_up_model": "追问模型",
    "modelSettings.usage.bindings.image_processing_model": "图片处理模型",
    "modelSettings.usage.bindings.query_understand_model": "问题理解模型",
    "modelSettings.usage.bindings.rerank_model": "重排序模型",
    "modelSettings.usage.bindings.summary_model": "摘要模型",
    "modelSettings.usage.bindings.unknown": "其他模型配置",
    "modelSettings.usage.bindings.vlm_model": "视觉理解模型",
    "modelSettings.usage.bindings.wiki_synthesis_model": "Wiki 综合模型",
    "modelSettings.usage.description": "模型「{name}」仍被以下配置引用。请先打开对应配置并更换模型，再重新删除。",
    "modelSettings.usage.knowledgeBases": "知识库（{count}）",
    "modelSettings.usage.longTermMemory": "长期记忆",
    "modelSettings.usage.openConfiguration": "打开配置",
    "modelSettings.usage.title": "模型无法删除",
    "modelSettings.usage.truncated": "仅显示前 {shown} 个，共 {total} 个",
    "modelSettings.vllm.desc": "配置用于视觉理解和多模态的视觉语言模型",
    "modelSettings.vllm.empty": "暂无 VLLM 视觉模型",
    "modelSettings.vllm.title": "VLLM 视觉模型",
    "settings.weknoraCloud.checkingStatus": "正在检查凭证状态...",
    "settings.weknoraCloud.credentialExpired": "凭证已失效，请重新配置。",
    "settings.weknoraCloud.credentialUnconfigured": "尚未配置 WeKnoraCloud 凭证，请先填写 APPID 和 APPSECRET。",
    "settings.weknoraCloud.goToSettings": "前往设置中配置",
    "settings.weknoraCloud.modelHintConfigured": "WeKnoraCloud 凭证已配置。支持的模型可参考",
    "settings.weknoraCloud.modelHintDocsLink": "接口文档",
  },
  "en-US": {
    "common.all": "All",
    "common.cancel": "Cancel",
    "common.close": "Close",
    "common.copied": "Copied",
    "common.copy": "Copy",
    "common.delete": "Delete",
    "common.edit": "Edit",
    "common.no": "No",
    "common.refresh": "Refresh",
    "common.yes": "Yes",
    "model.editor.addTitle": "Add Model",
    "model.editor.apiKeyOptional": "API Key (optional)",
    "model.editor.apiKeyPlaceholder": "Enter API Key",
    "model.editor.baseUrlLabel": "Base URL",
    "model.editor.baseUrlPlaceholder": "e.g. https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderAsr": "e.g. https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderVllm": "e.g. http://localhost:11434/v1",
    "model.editor.checkDimension": "Detect Dimension",
    "model.editor.connectionConfigError": "Connection failed, please check the configuration",
    "model.editor.connectionFailed": "Connection failed",
    "model.editor.connectionSuccess": "Connection succeeded",
    "model.editor.contextWindowDefaultHint": "Unset, using default {value}",
    "model.editor.contextWindowDesc": "How many tokens this model can take in one request. Agent history compaction uses this limit. Leave empty for the default 200000 (200K). Use the provider’s real window — a larger guess means compaction never fires and the provider rejects the request.",
    "model.editor.contextWindowLabel": "Context Window",
    "model.editor.contextWindowPlaceholder": "Default {value}",
    "model.editor.contextWindowTokens": "{count} tokens",
    "model.editor.customHeadersAdd": "Add Header",
    "model.editor.customHeadersDesc": "Extra HTTP headers appended to requests to the remote model API (e.g. for enterprise gateway auth or tracing). Reserved headers like Authorization / Content-Type are ignored.",
    "model.editor.customHeadersKeyPlaceholder": "Header name",
    "model.editor.customHeadersLabel": "Custom Request Headers (optional)",
    "model.editor.customHeadersValuePlaceholder": "Header value",
    "model.editor.description.asr": "Configure speech-to-text models for audio transcription",
    "model.editor.description.chat": "Configure large language models for conversations",
    "model.editor.description.default": "Configure model information",
    "model.editor.description.embedding": "Configure embedding models for text vectorization",
    "model.editor.description.rerank": "Configure models for result re-ranking",
    "model.editor.description.vllm": "Configure vision-language models for multimodal understanding",
    "model.editor.dimensionDetected": "Detection succeeded. Vector dimension: {value}",
    "model.editor.dimensionFailed": "Detection failed, please enter the dimension manually",
    "model.editor.dimensionHint": "Model selected. Click \"Detect Dimension\" to fetch the vector dimension automatically.",
    "model.editor.dimensionLabel": "Vector Dimension",
    "model.editor.dimensionOverrideDesc": "Enable only if the provider documentation says this model accepts a dimensions parameter.",
    "model.editor.dimensionOverrideLabel": "Custom Output Dimension",
    "model.editor.dimensionPlaceholder": "e.g. 1536",
    "model.editor.displayNameDesc": "Used only in the UI. Runtime calls still use the model name above.",
    "model.editor.displayNameLabel": "Display name (optional)",
    "model.editor.displayNamePlaceholder": "e.g. Support QA model",
    "model.editor.downloadCompleted": "{name} downloaded successfully",
    "model.editor.downloadFailed": "Failed to download {name}",
    "model.editor.downloadLabel": "Download: {keyword}",
    "model.editor.downloadStartFailed": "Failed to start download",
    "model.editor.downloadStarted": "Started downloading {name}",
    "model.editor.editTitle": "Edit Model",
    "model.editor.fillModelAndUrl": "Please fill in the model identifier and Base URL first",
    "model.editor.goToOllamaSettings": "Open Settings",
    "model.editor.listRefreshed": "List refreshed",
    "model.editor.lkeap.regionDesc": "RunRerank supports ap-beijing, ap-guangzhou, etc. Default: ap-guangzhou",
    "model.editor.lkeap.regionLabel": "Region",
    "model.editor.lkeap.regionPlaceholder": "ap-guangzhou",
    "model.editor.lkeap.rerankCredentialHint": "Rerank uses Tencent Cloud API signature (not the OpenAI-style LKEAP API key). Create SecretId/SecretKey in the CAM console.",
    "model.editor.lkeap.secretIdLabel": "SecretId",
    "model.editor.lkeap.secretIdPlaceholder": "Tencent Cloud API SecretId",
    "model.editor.lkeap.secretKeyLabel": "SecretKey",
    "model.editor.lkeap.secretKeyPlaceholder": "Tencent Cloud API SecretKey",
    "model.editor.loadModelListFailed": "Failed to load model list",
    "model.editor.maxConcurrencyDesc": "Caps concurrent background (ingestion/enrichment) calls to this model, shared per model across all replicas. 0 or empty falls back to the global default; interactive chat is never affected.",
    "model.editor.maxConcurrencyLabel": "Background concurrency limit",
    "model.editor.maxConcurrencyPlaceholder": "0 = use global default",
    "model.editor.modelNamePlaceholder.local": "e.g. llama2:latest",
    "model.editor.modelNamePlaceholder.localVllm": "e.g. llava:latest",
    "model.editor.modelNamePlaceholder.remote": "e.g. gpt-4, claude-3-opus",
    "model.editor.modelNamePlaceholder.remoteAsr": "e.g. whisper-1",
    "model.editor.modelNamePlaceholder.remoteVllm": "e.g. gpt-4-vision-preview",
    "model.editor.ollamaNotSupportRerank": "Ollama does not support ReRank models, please use a remote API instead",
    "model.editor.ollamaUnavailable": "Ollama service is unavailable, local models cannot be selected",
    "model.editor.providerLabel": "Provider",
    "model.editor.providerPlaceholder": "Select model provider",
    "model.editor.providers.aliyun.description": "qwen-plus, tongyi-embedding-vision-plus, qwen3-rerank, etc.",
    "model.editor.providers.aliyun.label": "Aliyun DashScope",
    "model.editor.providers.anthropic.description": "Claude models via native Anthropic Messages API",
    "model.editor.providers.anthropic.label": "Anthropic",
    "model.editor.providers.azure_openai.description": "OpenAI service hosted on Microsoft Azure",
    "model.editor.providers.azure_openai.label": "Azure OpenAI",
    "model.editor.providers.deepseek.description": "deepseek-chat, deepseek-reasoner, etc.",
    "model.editor.providers.deepseek.label": "DeepSeek",
    "model.editor.providers.gemini.description": "gemini-3-flash-preview, gemini-2.5-pro, etc.",
    "model.editor.providers.gemini.label": "Google Gemini",
    "model.editor.providers.generic.description": "Generic API endpoint",
    "model.editor.providers.generic.label": "Custom (OpenAI-compatible)",
    "model.editor.providers.gpustack.description": "Choose your deployed model on GPUStack",
    "model.editor.providers.gpustack.label": "GPUStack",
    "model.editor.providers.hunyuan.description": "hunyuan-pro, hunyuan-standard, hunyuan-embedding, etc.",
    "model.editor.providers.hunyuan.label": "Hunyuan",
    "model.editor.providers.jina.description": "jina-clip-v1, jina-embeddings-v2-base-zh, etc.",
    "model.editor.providers.jina.label": "Jina",
    "model.editor.providers.litellm.description": "Self-hosted proxy to 100+ providers (OpenAI, Anthropic, Gemini, Bedrock, etc.). Replace the placeholder URL; loopback hosts need SSRF_WHITELIST.",
    "model.editor.providers.litellm.label": "LiteLLM",
    "model.editor.providers.lkeap.description": "DeepSeek-R1, DeepSeek-V3, lke-reranker-base, etc.",
    "model.editor.providers.lkeap.label": "Tencent Cloud LKEAP",
    "model.editor.providers.longcat.description": "LongCat-Flash-Chat, LongCat-Flash-Thinking, etc.",
    "model.editor.providers.longcat.label": "LongCat AI",
    "model.editor.providers.mimo.description": "mimo-v2-flash",
    "model.editor.providers.mimo.label": "MiMo",
    "model.editor.providers.minimax.description": "MiniMax-M3, MiniMax-M2.7, MiniMax-M2.7-highspeed, etc.",
    "model.editor.providers.minimax.label": "MiniMax",
    "model.editor.providers.modelscope.description": "Qwen/Qwen3-8B, Qwen/Qwen3-Embedding-8B, etc.",
    "model.editor.providers.modelscope.label": "ModelScope",
    "model.editor.providers.moonshot.description": "kimi-k2-turbo-preview, moonshot-v1-8k-vision-preview, etc.",
    "model.editor.providers.moonshot.label": "Moonshot",
    "model.editor.providers.novita.description": "moonshotai/kimi-k2.5, zai-org/glm-5, minimax/minimax-m2.7, qwen/qwen3-embedding-0.6b, etc.",
    "model.editor.providers.novita.label": "Novita AI",
    "model.editor.providers.nvidia.description": "deepseek-ai-deepseek-v3_1, nv-embed-v1, rerank-qa-mistral-4b, etc.",
    "model.editor.providers.nvidia.label": "NVIDIA",
    "model.editor.providers.openai.description": "gpt-5.2, gpt-5-mini, etc.",
    "model.editor.providers.openai.label": "OpenAI",
    "model.editor.providers.openrouter.description": "openai/gpt-5.2-chat, google/gemini-3-flash-preview, etc.",
    "model.editor.providers.openrouter.label": "OpenRouter",
    "model.editor.providers.qianfan.description": "ernie-5.0-thinking-preview, embedding-v1, bce-reranker-base, etc.",
    "model.editor.providers.qianfan.label": "Baidu Qianfan",
    "model.editor.providers.qiniu.description": "deepseek/deepseek-v3.2-251201, z-ai/glm-4.7, etc.",
    "model.editor.providers.qiniu.label": "Qiniu Cloud",
    "model.editor.providers.requesty.description": "openai/gpt-4o-mini, anthropic/claude-sonnet-4-5, etc.",
    "model.editor.providers.requesty.label": "Requesty",
    "model.editor.providers.siliconflow.description": "deepseek-ai/DeepSeek-V3.1, etc.",
    "model.editor.providers.siliconflow.label": "SiliconFlow",
    "model.editor.providers.volcengine.description": "doubao-1-5-pro-32k-250115, doubao-embedding-vision-250615, etc.",
    "model.editor.providers.volcengine.label": "Volcengine",
    "model.editor.providers.zhipu.description": "glm-4.7, embedding-3, rerank, etc.",
    "model.editor.providers.zhipu.label": "Zhipu BigModel",
    "model.editor.refreshList": "Refresh List",
    "model.editor.remoteBaseUrlRequired": "Remote API type requires a Base URL",
    "model.editor.remoteDimensionDetected": "Detected vector dimension: {value}",
    "model.editor.sectionAdvanced": "Advanced Options",
    "model.editor.sectionProvider": "Provider Settings",
    "model.editor.sectionSource": "Source",
    "model.editor.sectionType": "Model Type",
    "model.editor.sourceLabel": "Model Source",
    "model.editor.sourceLocal": "Ollama",
    "model.editor.sourceRemote": "API",
    "model.editor.supportsVisionDesc": "Whether the model accepts image and multimodal input",
    "model.editor.supportsVisionLabel": "Supports Vision / Multimodal",
    "model.editor.testConnection": "Test Connection",
    "model.editor.testing": "Testing...",
    "model.editor.thinkingControl.chatTemplateKwargs.hint": "Custom OpenAI-compatible gateways, NVIDIA NIM, vLLM / local Qwen",
    "model.editor.thinkingControl.chatTemplateKwargs.label": "chat_template_kwargs",
    "model.editor.thinkingControl.enableThinking.hint": "Alibaba DashScope: qwen3, qwen-plus, qwen-max, qwen-turbo",
    "model.editor.thinkingControl.enableThinking.label": "enable_thinking",
    "model.editor.thinkingControl.none.hint": "Agent “Thinking mode” switch has no effect; thinking parameters are not sent in requests",
    "model.editor.thinkingControl.none.label": "Do not send thinking fields",
    "model.editor.thinkingControl.thinkingType.hint": "Volcengine Ark; Tencent LKEAP (DeepSeek V3, etc.; default for LKEAP; use “Do not send” for R1)",
    "model.editor.thinkingControl.thinkingType.label": "thinking.type",
    "model.editor.thinkingControlDesc": "Controls how the agent’s “Thinking mode” on/off switch is written to the API. We pre-select based on vendor/model when possible; change it to match your API docs. With “Do not send”, the agent Thinking mode switch has no effect.",
    "model.editor.thinkingControlLabel": "Thinking mode request format",
    "model.editor.typeLabel": "Model Type",
    "model.editor.unsupportedModelType": "Unsupported model type",
    "model.editor.validation.baseUrlEmpty": "Base URL cannot be empty",
    "model.editor.validation.baseUrlInvalid": "Invalid Base URL, please enter a valid URL",
    "model.editor.validation.baseUrlRequired": "Please enter the Base URL",
    "model.editor.validation.modelNameEmpty": "Model name cannot be empty",
    "model.editor.validation.modelNameMax": "Model name cannot exceed 100 characters",
    "model.editor.validation.modelNameRequired": "Please enter the model name",
    "model.editor.volcengine.accessKeyLabel": "Access Key ID",
    "model.editor.volcengine.accessKeyPlaceholder": "Volcengine Access Key ID",
    "model.editor.volcengine.rerankCredentialHint": "Rerank uses VikingDB AK/SK signing, not an Ark API key. Recommended model: doubao-seed-rerank.",
    "model.editor.volcengine.secretKeyLabel": "Secret Access Key",
    "model.editor.volcengine.secretKeyPlaceholder": "Volcengine Secret Access Key",
    "model.modelName": "Model Name",
    "model.searchPlaceholder": "Search models...",
    "modelSettings.actions.addModel": "Add Model",
    "modelSettings.actions.debugModel": "Model Test",
    "modelSettings.asr.desc": "Configure speech-to-text models for audio transcription (e.g. OpenAI Whisper)",
    "modelSettings.asr.empty": "No ASR models",
    "modelSettings.asr.title": "ASR Speech Models",
    "modelSettings.builtinModels.description": "Built-in models are visible to all workspaces. Sensitive information is hidden, and they cannot be edited or deleted.",
    "modelSettings.builtinModels.descriptionAdmin": "Built-in models are visible to all workspaces. System administrators can edit configuration and credentials; deletion remains deployment-managed.",
    "modelSettings.builtinModels.title": "Built-in Models",
    "modelSettings.builtinModels.viewGuide": "View Built-in Models Guide",
    "modelSettings.builtinTag": "Built-in",
    "modelSettings.chat.desc": "Configure large language models for chatting",
    "modelSettings.chat.empty": "No chat models",
    "modelSettings.chat.title": "Chat Models",
    "modelSettings.confirmDelete": "Delete model \"{name}\"?",
    "modelSettings.copySuffix": " Copy",
    "modelSettings.debug.audioFile": "Audio file",
    "modelSettings.debug.chooseFile": "Choose file",
    "modelSettings.debug.copyResult": "Copy result",
    "modelSettings.debug.description": "Send a real request to a saved model and inspect the response",
    "modelSettings.debug.documents": "Candidate documents",
    "modelSettings.debug.documentsHint": "Each non-empty line is sent as a separate document to the ReRank model",
    "modelSettings.debug.documentsPlaceholder": "Enter one candidate document per line",
    "modelSettings.debug.embeddingInput": "Text to embed",
    "modelSettings.debug.embeddingPlaceholder": "Enter text to generate an embedding",
    "modelSettings.debug.failed": "Request failed",
    "modelSettings.debug.groupInput": "Test input",
    "modelSettings.debug.groupModel": "Select model",
    "modelSettings.debug.groupResult": "Result",
    "modelSettings.debug.history": "History",
    "modelSettings.debug.imageFile": "Image file",
    "modelSettings.debug.metrics.answerChars": "Answer chars",
    "modelSettings.debug.metrics.dimension": "Dimensions",
    "modelSettings.debug.metrics.reasoningChars": "Reasoning chars",
    "modelSettings.debug.metrics.reasoningReturned": "Reasoning returned",
    "modelSettings.debug.metrics.resultCount": "Result count",
    "modelSettings.debug.metrics.segmentCount": "Segment count",
    "modelSettings.debug.metrics.textChars": "Transcript chars",
    "modelSettings.debug.model": "Model",
    "modelSettings.debug.modelPlaceholder": "Select a model to test",
    "modelSettings.debug.modelType": "Model type",
    "modelSettings.debug.noModelsForType": "No saved models of this type",
    "modelSettings.debug.parameters": "Request parameters",
    "modelSettings.debug.query": "Input",
    "modelSettings.debug.queryPlaceholder": "Enter the text to send to the model",
    "modelSettings.debug.rawResponse": "Response",
    "modelSettings.debug.requestFailed": "Model test request failed",
    "modelSettings.debug.requestPreview": "Request preview",
    "modelSettings.debug.run": "Run test",
    "modelSettings.debug.runLabel": "Run #{n}",
    "modelSettings.debug.success": "Request succeeded",
    "modelSettings.debug.systemPrompt": "System Prompt",
    "modelSettings.debug.systemPromptPlaceholder": "Optional system prompt",
    "modelSettings.debug.thinkOff": "Thinking off",
    "modelSettings.debug.thinkOn": "Thinking on",
    "modelSettings.debug.thinking": "Thinking mode",
    "modelSettings.debug.thinkingDesc": "Only applies to models that support thinking",
    "modelSettings.debug.title": "Model Test",
    "modelSettings.debug.vlmPrompt": "Image prompt",
    "modelSettings.debug.vlmPromptPlaceholder": "For example: Describe this image in detail",
    "modelSettings.description": "Manage different types of AI models, including local Ollama and remote APIs",
    "modelSettings.embedding.desc": "Configure embedding models for text vectorization",
    "modelSettings.embedding.empty": "No embedding models",
    "modelSettings.embedding.title": "Embedding Models",
    "modelSettings.rerank.desc": "Configure models for result re-ranking",
    "modelSettings.rerank.empty": "No re-rank models",
    "modelSettings.rerank.title": "ReRank Models",
    "modelSettings.source.custom": "Custom",
    "modelSettings.source.openaiCompatible": "OpenAI-compatible",
    "modelSettings.source.remote": "Remote",
    "modelSettings.title": "Model Settings",
    "modelSettings.toasts.added": "Model added",
    "modelSettings.toasts.baseUrlInvalid": "Invalid Base URL, please enter a valid URL",
    "modelSettings.toasts.baseUrlRequired": "Base URL is required for remote APIs",
    "modelSettings.toasts.builtinCannotCopy": "Built-in models cannot be copied",
    "modelSettings.toasts.builtinCannotDelete": "Built-in models cannot be deleted",
    "modelSettings.toasts.builtinCannotEdit": "Built-in models cannot be edited",
    "modelSettings.toasts.copied": "Model copied",
    "modelSettings.toasts.copyFailed": "Failed to copy model",
    "modelSettings.toasts.deleteFailed": "Failed to delete model",
    "modelSettings.toasts.deleted": "Model deleted",
    "modelSettings.toasts.dimensionInvalid": "Embedding dimension must be between 128 and 4096",
    "modelSettings.toasts.displayNameTooLong": "Display name cannot exceed 100 characters",
    "modelSettings.toasts.nameRequired": "Model name cannot be empty",
    "modelSettings.toasts.nameTooLong": "Model name cannot exceed 100 characters",
    "modelSettings.toasts.saveFailed": "Failed to save model",
    "modelSettings.toasts.updated": "Model updated",
    "modelSettings.typeShort.asr": "Speech",
    "modelSettings.typeShort.chat": "Chat",
    "modelSettings.typeShort.embedding": "Embedding",
    "modelSettings.typeShort.rerank": "ReRank",
    "modelSettings.typeShort.vllm": "Vision",
    "modelSettings.usage.agents": "Agents ({count})",
    "modelSettings.usage.bindings.asr_model": "Speech recognition model",
    "modelSettings.usage.bindings.chat_model": "Chat model",
    "modelSettings.usage.bindings.embedding_model": "Embedding model",
    "modelSettings.usage.bindings.extract_model": "Memory extraction model",
    "modelSettings.usage.bindings.follow_up_model": "Follow-up model",
    "modelSettings.usage.bindings.image_processing_model": "Image processing model",
    "modelSettings.usage.bindings.query_understand_model": "Query understanding model",
    "modelSettings.usage.bindings.rerank_model": "Re-ranking model",
    "modelSettings.usage.bindings.summary_model": "Summary model",
    "modelSettings.usage.bindings.unknown": "Other model setting",
    "modelSettings.usage.bindings.vlm_model": "Vision model",
    "modelSettings.usage.bindings.wiki_synthesis_model": "Wiki synthesis model",
    "modelSettings.usage.description": "Model \"{name}\" is still referenced by the following settings. Open each configuration and choose another model before deleting it.",
    "modelSettings.usage.knowledgeBases": "Knowledge bases ({count})",
    "modelSettings.usage.longTermMemory": "Long-term memory",
    "modelSettings.usage.openConfiguration": "Open settings",
    "modelSettings.usage.title": "Model cannot be deleted",
    "modelSettings.usage.truncated": "Showing the first {shown} of {total}",
    "modelSettings.vllm.desc": "Configure vision-language models for multimodal understanding",
    "modelSettings.vllm.empty": "No VLLM models",
    "modelSettings.vllm.title": "VLLM Vision Models",
    "settings.weknoraCloud.checkingStatus": "Checking credential status...",
    "settings.weknoraCloud.credentialExpired": "Credentials expired. Please reconfigure.",
    "settings.weknoraCloud.credentialUnconfigured": "WeKnoraCloud credentials not configured. Please set up APPID and APPSECRET first.",
    "settings.weknoraCloud.goToSettings": "Go to Settings",
    "settings.weknoraCloud.modelHintConfigured": "WeKnoraCloud credentials configured. See supported models in",
    "settings.weknoraCloud.modelHintDocsLink": "API docs",
  },
  "ja-JP": {
    "common.all": "すべて",
    "common.cancel": "キャンセル",
    "common.close": "閉じる",
    "common.copied": "コピーしました",
    "common.copy": "コピー",
    "common.delete": "削除",
    "common.edit": "編集",
    "common.no": "いいえ",
    "common.refresh": "更新",
    "common.yes": "はい",
    "model.editor.addTitle": "モデルを追加",
    "model.editor.apiKeyOptional": "APIキー（任意）",
    "model.editor.apiKeyPlaceholder": "APIキーを入力してください",
    "model.editor.baseUrlLabel": "ベースURL",
    "model.editor.baseUrlPlaceholder": "例: https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderAsr": "例: https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderVllm": "例: http://localhost:11434/v1",
    "model.editor.checkDimension": "次元数を検出",
    "model.editor.connectionConfigError": "接続に失敗しました。設定を確認してください",
    "model.editor.connectionFailed": "接続に失敗しました",
    "model.editor.connectionSuccess": "接続に成功しました",
    "model.editor.contextWindowDefaultHint": "未設定のためデフォルト値{value}を使用",
    "model.editor.contextWindowDesc": "1回のリクエストでこのモデルが受け付けられるトークン数です。エージェントの履歴圧縮はこの上限を基準にします。空欄の場合はデフォルト値の200000（200K）が使われます。プロバイダの実際のウィンドウサイズを指定してください。大きすぎる値を指定すると圧縮が働かず、プロバイダにリクエストを拒否されます。",
    "model.editor.contextWindowLabel": "コンテキストウィンドウ",
    "model.editor.contextWindowPlaceholder": "デフォルト値{value}",
    "model.editor.contextWindowTokens": "{count}トークン",
    "model.editor.customHeadersAdd": "ヘッダーを追加",
    "model.editor.customHeadersDesc": "リモートモデルAPIへのリクエストに追加するHTTPヘッダーです（企業ゲートウェイの認証やトレースなどに利用）。Authorization / Content-Typeなどの予約ヘッダーは無視されます。",
    "model.editor.customHeadersKeyPlaceholder": "ヘッダー名",
    "model.editor.customHeadersLabel": "カスタムリクエストヘッダー（任意）",
    "model.editor.customHeadersValuePlaceholder": "ヘッダーの値",
    "model.editor.description.asr": "音声の文字起こしに使う音声認識モデルを設定します",
    "model.editor.description.chat": "会話用の大規模言語モデルを設定します",
    "model.editor.description.default": "モデル情報を設定します",
    "model.editor.description.embedding": "テキストのベクトル化に使う埋め込みモデルを設定します",
    "model.editor.description.rerank": "検索結果のリランクに使うモデルを設定します",
    "model.editor.description.vllm": "マルチモーダル理解に使う視覚言語モデルを設定します",
    "model.editor.dimensionDetected": "検出に成功しました。ベクトル次元数: {value}",
    "model.editor.dimensionFailed": "検出に失敗しました。次元数を手動で入力してください",
    "model.editor.dimensionHint": "モデルを選択しました。「次元数を検出」をクリックするとベクトル次元数を自動取得できます。",
    "model.editor.dimensionLabel": "ベクトル次元数",
    "model.editor.dimensionOverrideDesc": "プロバイダのドキュメントでこのモデルがdimensionsパラメータに対応していると記載されている場合にのみ有効にしてください。デフォルトでは検出された実際の次元数のみを使用します。",
    "model.editor.dimensionOverrideLabel": "出力次元数のカスタマイズ",
    "model.editor.dimensionPlaceholder": "例: 1536",
    "model.editor.displayNameDesc": "UIでの表示にのみ使用されます。実行時の呼び出しには上のモデル名が使われます。",
    "model.editor.displayNameLabel": "表示名（任意）",
    "model.editor.displayNamePlaceholder": "例: サポートQA用モデル",
    "model.editor.downloadCompleted": "{name}のダウンロードが完了しました",
    "model.editor.downloadFailed": "{name}のダウンロードに失敗しました",
    "model.editor.downloadLabel": "ダウンロード: {keyword}",
    "model.editor.downloadStartFailed": "ダウンロードの開始に失敗しました",
    "model.editor.downloadStarted": "{name}のダウンロードを開始しました",
    "model.editor.editTitle": "モデルを編集",
    "model.editor.fillModelAndUrl": "先にモデル識別子とベースURLを入力してください",
    "model.editor.goToOllamaSettings": "設定を開く",
    "model.editor.listRefreshed": "一覧を更新しました",
    "model.editor.lkeap.regionDesc": "RunRerankはap-beijing、ap-guangzhouなどに対応しています。デフォルト値: ap-guangzhou",
    "model.editor.lkeap.regionLabel": "リージョン",
    "model.editor.lkeap.regionPlaceholder": "ap-guangzhou",
    "model.editor.lkeap.rerankCredentialHint": "リランクはTencent CloudのAPI署名（OpenAI形式のLKEAP APIキーではありません）を使用します。SecretId/SecretKeyはCAMコンソールで作成してください。",
    "model.editor.lkeap.secretIdLabel": "SecretId",
    "model.editor.lkeap.secretIdPlaceholder": "Tencent Cloud APIのSecretId",
    "model.editor.lkeap.secretKeyLabel": "SecretKey",
    "model.editor.lkeap.secretKeyPlaceholder": "Tencent Cloud APIのSecretKey",
    "model.editor.loadModelListFailed": "モデル一覧の読み込みに失敗しました",
    "model.editor.maxConcurrencyDesc": "このモデルへのバックグラウンド（取り込み・エンリッチメント）呼び出しの並列数を制限します。モデルごとにすべてのレプリカで共有されます。0または空欄の場合はグローバルのデフォルト値が使われます。対話型のチャットには影響しません。",
    "model.editor.maxConcurrencyLabel": "バックグラウンドの並列実行上限",
    "model.editor.maxConcurrencyPlaceholder": "0 = グローバルのデフォルト値を使用",
    "model.editor.modelNamePlaceholder.local": "例: llama2:latest",
    "model.editor.modelNamePlaceholder.localVllm": "例: llava:latest",
    "model.editor.modelNamePlaceholder.remote": "例: gpt-4, claude-3-opus",
    "model.editor.modelNamePlaceholder.remoteAsr": "例: whisper-1",
    "model.editor.modelNamePlaceholder.remoteVllm": "例: gpt-4-vision-preview",
    "model.editor.ollamaNotSupportRerank": "Ollamaはリランクモデルに対応していません。リモートAPIを使用してください",
    "model.editor.ollamaUnavailable": "Ollamaサービスを利用できないため、ローカルモデルを選択できません",
    "model.editor.providerLabel": "プロバイダ",
    "model.editor.providerPlaceholder": "モデルプロバイダを選択",
    "model.editor.providers.aliyun.description": "qwen-plus、tongyi-embedding-vision-plus、qwen3-rerankなど",
    "model.editor.providers.aliyun.label": "Aliyun DashScope",
    "model.editor.providers.anthropic.description": "ネイティブのAnthropic Messages API経由のClaudeモデル",
    "model.editor.providers.anthropic.label": "Anthropic",
    "model.editor.providers.azure_openai.description": "Microsoft AzureでホストされるOpenAIサービス",
    "model.editor.providers.azure_openai.label": "Azure OpenAI",
    "model.editor.providers.deepseek.description": "deepseek-chat、deepseek-reasonerなど",
    "model.editor.providers.deepseek.label": "DeepSeek",
    "model.editor.providers.gemini.description": "gemini-3-flash-preview、gemini-2.5-proなど",
    "model.editor.providers.gemini.label": "Google Gemini",
    "model.editor.providers.generic.description": "汎用のAPIエンドポイント",
    "model.editor.providers.generic.label": "カスタム（OpenAI互換）",
    "model.editor.providers.gpustack.description": "GPUStackにデプロイ済みのモデルを選択してください",
    "model.editor.providers.gpustack.label": "GPUStack",
    "model.editor.providers.hunyuan.description": "hunyuan-pro、hunyuan-standard、hunyuan-embeddingなど",
    "model.editor.providers.hunyuan.label": "Hunyuan",
    "model.editor.providers.jina.description": "jina-clip-v1、jina-embeddings-v2-base-zhなど",
    "model.editor.providers.jina.label": "Jina",
    "model.editor.providers.litellm.description": "100以上のプロバイダ（OpenAI、Anthropic、Gemini、Bedrockなど）へのセルフホスト型プロキシです。プレースホルダのURLを置き換えてください。ループバックのホストにはSSRF_WHITELISTの設定が必要です。",
    "model.editor.providers.litellm.label": "LiteLLM",
    "model.editor.providers.lkeap.description": "DeepSeek-R1、DeepSeek-V3、lke-reranker-baseなど",
    "model.editor.providers.lkeap.label": "Tencent Cloud LKEAP",
    "model.editor.providers.longcat.description": "LongCat-Flash-Chat、LongCat-Flash-Thinkingなど",
    "model.editor.providers.longcat.label": "LongCat AI",
    "model.editor.providers.mimo.description": "mimo-v2-flash",
    "model.editor.providers.mimo.label": "MiMo",
    "model.editor.providers.minimax.description": "MiniMax-M3、MiniMax-M2.7、MiniMax-M2.7-highspeedなど",
    "model.editor.providers.minimax.label": "MiniMax",
    "model.editor.providers.modelscope.description": "Qwen/Qwen3-8B、Qwen/Qwen3-Embedding-8Bなど",
    "model.editor.providers.modelscope.label": "ModelScope",
    "model.editor.providers.moonshot.description": "kimi-k2-turbo-preview、moonshot-v1-8k-vision-previewなど",
    "model.editor.providers.moonshot.label": "Moonshot",
    "model.editor.providers.novita.description": "moonshotai/kimi-k2.5、zai-org/glm-5、minimax/minimax-m2.7、qwen/qwen3-embedding-0.6bなど",
    "model.editor.providers.novita.label": "Novita AI",
    "model.editor.providers.nvidia.description": "deepseek-ai-deepseek-v3_1、nv-embed-v1、rerank-qa-mistral-4bなど",
    "model.editor.providers.nvidia.label": "NVIDIA",
    "model.editor.providers.openai.description": "gpt-5.2、gpt-5-miniなど",
    "model.editor.providers.openai.label": "OpenAI",
    "model.editor.providers.openrouter.description": "openai/gpt-5.2-chat、google/gemini-3-flash-previewなど",
    "model.editor.providers.openrouter.label": "OpenRouter",
    "model.editor.providers.qianfan.description": "ernie-5.0-thinking-preview、embedding-v1、bce-reranker-baseなど",
    "model.editor.providers.qianfan.label": "Baidu Qianfan",
    "model.editor.providers.qiniu.description": "deepseek/deepseek-v3.2-251201、z-ai/glm-4.7など",
    "model.editor.providers.qiniu.label": "Qiniu Cloud",
    "model.editor.providers.requesty.description": "openai/gpt-4o-mini、anthropic/claude-sonnet-4-5など",
    "model.editor.providers.requesty.label": "Requesty",
    "model.editor.providers.siliconflow.description": "deepseek-ai/DeepSeek-V3.1など",
    "model.editor.providers.siliconflow.label": "SiliconFlow",
    "model.editor.providers.volcengine.description": "doubao-1-5-pro-32k-250115、doubao-embedding-vision-250615など",
    "model.editor.providers.volcengine.label": "Volcengine",
    "model.editor.providers.zhipu.description": "glm-4.7、embedding-3、rerankなど",
    "model.editor.providers.zhipu.label": "Zhipu BigModel",
    "model.editor.refreshList": "一覧を更新",
    "model.editor.remoteBaseUrlRequired": "リモートAPIタイプではベースURLが必要です",
    "model.editor.remoteDimensionDetected": "検出したベクトル次元数: {value}",
    "model.editor.sectionAdvanced": "詳細オプション",
    "model.editor.sectionProvider": "プロバイダ設定",
    "model.editor.sectionSource": "ソース",
    "model.editor.sectionType": "モデルタイプ",
    "model.editor.sourceLabel": "モデルのソース",
    "model.editor.sourceLocal": "Ollama",
    "model.editor.sourceRemote": "API",
    "model.editor.supportsVisionDesc": "モデルが画像やマルチモーダル入力を受け付けるかどうか",
    "model.editor.supportsVisionLabel": "視覚・マルチモーダルに対応",
    "model.editor.testConnection": "接続をテスト",
    "model.editor.testing": "テスト中...",
    "model.editor.thinkingControl.chatTemplateKwargs.hint": "OpenAI互換のカスタムゲートウェイ、NVIDIA NIM、vLLM / ローカルのQwen",
    "model.editor.thinkingControl.chatTemplateKwargs.label": "chat_template_kwargs",
    "model.editor.thinkingControl.enableThinking.hint": "Alibaba DashScope: qwen3、qwen-plus、qwen-max、qwen-turbo",
    "model.editor.thinkingControl.enableThinking.label": "enable_thinking",
    "model.editor.thinkingControl.none.hint": "エージェントの「思考モード」の切り替えは効果がなく、リクエストに思考パラメータは送信されません",
    "model.editor.thinkingControl.none.label": "思考関連のフィールドを送信しない",
    "model.editor.thinkingControl.thinkingType.hint": "Volcengine Ark、Tencent LKEAP（DeepSeek V3など。LKEAPのデフォルト値。R1では「送信しない」を使用）",
    "model.editor.thinkingControl.thinkingType.label": "thinking.type",
    "model.editor.thinkingControlDesc": "エージェントの「思考モード」のオン/オフをAPIにどう送信するかを設定します。可能な場合はベンダやモデルに応じて自動選択されます。お使いのAPIドキュメントに合わせて変更してください。「送信しない」を選ぶと、エージェントの思考モードの切り替えは効果がありません。",
    "model.editor.thinkingControlLabel": "思考モードのリクエスト形式",
    "model.editor.typeLabel": "モデルタイプ",
    "model.editor.unsupportedModelType": "未対応のモデルタイプです",
    "model.editor.validation.baseUrlEmpty": "ベースURLは空にできません",
    "model.editor.validation.baseUrlInvalid": "ベースURLが無効です。有効なURLを入力してください",
    "model.editor.validation.baseUrlRequired": "ベースURLを入力してください",
    "model.editor.validation.modelNameEmpty": "モデル名は空にできません",
    "model.editor.validation.modelNameMax": "モデル名は100文字以内で入力してください",
    "model.editor.validation.modelNameRequired": "モデル名を入力してください",
    "model.editor.volcengine.accessKeyLabel": "Access Key ID",
    "model.editor.volcengine.accessKeyPlaceholder": "VolcengineのAccess Key ID",
    "model.editor.volcengine.rerankCredentialHint": "リランクはArkのAPIキーではなく、VikingDBのAK/SK署名を使用します。推奨モデル: doubao-seed-rerank。",
    "model.editor.volcengine.secretKeyLabel": "Secret Access Key",
    "model.editor.volcengine.secretKeyPlaceholder": "VolcengineのSecret Access Key",
    "model.modelName": "モデル名",
    "model.searchPlaceholder": "モデルを検索...",
    "modelSettings.actions.addModel": "モデルを追加",
    "modelSettings.actions.debugModel": "モデルテスト",
    "modelSettings.asr.desc": "音声の文字起こしに使用する音声認識モデルを設定します（例: OpenAI Whisper）",
    "modelSettings.asr.empty": "ASRモデルがありません",
    "modelSettings.asr.title": "ASR音声モデル",
    "modelSettings.builtinModels.description": "組み込みモデルはすべてのワークスペースに表示されます。機密情報は非表示で、編集や削除はできません。",
    "modelSettings.builtinModels.descriptionAdmin": "組み込みモデルはすべてのワークスペースに表示されます。システム管理者は設定と認証情報を編集できますが、削除はデプロイ側で管理されます。",
    "modelSettings.builtinModels.title": "組み込みモデル",
    "modelSettings.builtinModels.viewGuide": "組み込みモデルのガイドを見る",
    "modelSettings.builtinTag": "組み込み",
    "modelSettings.chat.desc": "会話に使用する大規模言語モデルを設定します",
    "modelSettings.chat.empty": "チャットモデルがありません",
    "modelSettings.chat.title": "チャットモデル",
    "modelSettings.confirmDelete": "モデル「{name}」を削除しますか？",
    "modelSettings.copySuffix": " のコピー",
    "modelSettings.debug.audioFile": "音声ファイル",
    "modelSettings.debug.chooseFile": "ファイルを選択",
    "modelSettings.debug.copyResult": "結果をコピー",
    "modelSettings.debug.description": "保存済みのモデルに実際のリクエストを送信し、応答を確認します",
    "modelSettings.debug.documents": "候補ドキュメント",
    "modelSettings.debug.documentsHint": "空でない各行が個別のドキュメントとしてリランクモデルに送信されます",
    "modelSettings.debug.documentsPlaceholder": "候補ドキュメントを1行に1件ずつ入力してください",
    "modelSettings.debug.embeddingInput": "埋め込み対象のテキスト",
    "modelSettings.debug.embeddingPlaceholder": "埋め込みを生成するテキストを入力してください",
    "modelSettings.debug.failed": "リクエストに失敗しました",
    "modelSettings.debug.groupInput": "テスト入力",
    "modelSettings.debug.groupModel": "モデルを選択",
    "modelSettings.debug.groupResult": "結果",
    "modelSettings.debug.history": "履歴",
    "modelSettings.debug.imageFile": "画像ファイル",
    "modelSettings.debug.metrics.answerChars": "回答文字数",
    "modelSettings.debug.metrics.dimension": "次元数",
    "modelSettings.debug.metrics.reasoningChars": "推論文字数",
    "modelSettings.debug.metrics.reasoningReturned": "推論内容の返却",
    "modelSettings.debug.metrics.resultCount": "結果件数",
    "modelSettings.debug.metrics.segmentCount": "セグメント数",
    "modelSettings.debug.metrics.textChars": "文字起こし文字数",
    "modelSettings.debug.model": "モデル",
    "modelSettings.debug.modelPlaceholder": "テストするモデルを選択",
    "modelSettings.debug.modelType": "モデルの種類",
    "modelSettings.debug.noModelsForType": "この種類の保存済みモデルはありません",
    "modelSettings.debug.parameters": "リクエストパラメータ",
    "modelSettings.debug.query": "入力",
    "modelSettings.debug.queryPlaceholder": "モデルに送信するテキストを入力してください",
    "modelSettings.debug.rawResponse": "応答",
    "modelSettings.debug.requestFailed": "モデルテストのリクエストに失敗しました",
    "modelSettings.debug.requestPreview": "リクエストのプレビュー",
    "modelSettings.debug.run": "テストを実行",
    "modelSettings.debug.runLabel": "実行 #{n}",
    "modelSettings.debug.success": "リクエストに成功しました",
    "modelSettings.debug.systemPrompt": "システムプロンプト",
    "modelSettings.debug.systemPromptPlaceholder": "システムプロンプト（任意）",
    "modelSettings.debug.thinkOff": "思考オフ",
    "modelSettings.debug.thinkOn": "思考オン",
    "modelSettings.debug.thinking": "思考モード",
    "modelSettings.debug.thinkingDesc": "思考に対応したモデルにのみ適用されます",
    "modelSettings.debug.title": "モデルテスト",
    "modelSettings.debug.vlmPrompt": "画像プロンプト",
    "modelSettings.debug.vlmPromptPlaceholder": "例: この画像を詳しく説明してください",
    "modelSettings.description": "ローカルのOllamaやリモートAPIなど、さまざまな種類のAIモデルを管理します",
    "modelSettings.embedding.desc": "テキストのベクトル化に使用する埋め込みモデルを設定します",
    "modelSettings.embedding.empty": "埋め込みモデルがありません",
    "modelSettings.embedding.title": "埋め込みモデル",
    "modelSettings.rerank.desc": "結果のリランクに使用するモデルを設定します",
    "modelSettings.rerank.empty": "リランクモデルがありません",
    "modelSettings.rerank.title": "リランクモデル",
    "modelSettings.source.custom": "カスタム",
    "modelSettings.source.openaiCompatible": "OpenAI互換",
    "modelSettings.source.remote": "リモート",
    "modelSettings.title": "モデル設定",
    "modelSettings.toasts.added": "モデルを追加しました",
    "modelSettings.toasts.baseUrlInvalid": "Base URLが正しくありません。有効なURLを入力してください",
    "modelSettings.toasts.baseUrlRequired": "リモートAPIにはBase URLが必要です",
    "modelSettings.toasts.builtinCannotCopy": "組み込みモデルはコピーできません",
    "modelSettings.toasts.builtinCannotDelete": "組み込みモデルは削除できません",
    "modelSettings.toasts.builtinCannotEdit": "組み込みモデルは編集できません",
    "modelSettings.toasts.copied": "モデルをコピーしました",
    "modelSettings.toasts.copyFailed": "モデルのコピーに失敗しました",
    "modelSettings.toasts.deleteFailed": "モデルの削除に失敗しました",
    "modelSettings.toasts.deleted": "モデルを削除しました",
    "modelSettings.toasts.dimensionInvalid": "埋め込み次元数は128〜4096の範囲で指定してください",
    "modelSettings.toasts.displayNameTooLong": "表示名は100文字以内で入力してください",
    "modelSettings.toasts.nameRequired": "モデル名を入力してください",
    "modelSettings.toasts.nameTooLong": "モデル名は100文字以内で入力してください",
    "modelSettings.toasts.saveFailed": "モデルの保存に失敗しました",
    "modelSettings.toasts.updated": "モデルを更新しました",
    "modelSettings.typeShort.asr": "音声",
    "modelSettings.typeShort.chat": "チャット",
    "modelSettings.typeShort.embedding": "埋め込み",
    "modelSettings.typeShort.rerank": "リランク",
    "modelSettings.typeShort.vllm": "視覚",
    "modelSettings.usage.agents": "エージェント（{count}）",
    "modelSettings.usage.bindings.asr_model": "音声認識モデル",
    "modelSettings.usage.bindings.chat_model": "チャットモデル",
    "modelSettings.usage.bindings.embedding_model": "埋め込みモデル",
    "modelSettings.usage.bindings.extract_model": "メモリ抽出モデル",
    "modelSettings.usage.bindings.follow_up_model": "フォローアップモデル",
    "modelSettings.usage.bindings.image_processing_model": "画像処理モデル",
    "modelSettings.usage.bindings.query_understand_model": "クエリ理解モデル",
    "modelSettings.usage.bindings.rerank_model": "リランクモデル",
    "modelSettings.usage.bindings.summary_model": "要約モデル",
    "modelSettings.usage.bindings.unknown": "その他のモデル設定",
    "modelSettings.usage.bindings.vlm_model": "視覚モデル",
    "modelSettings.usage.bindings.wiki_synthesis_model": "Wiki生成モデル",
    "modelSettings.usage.description": "モデル「{name}」は次の設定から参照されています。削除する前に各設定を開き、別のモデルを選択してください。",
    "modelSettings.usage.knowledgeBases": "ナレッジベース（{count}）",
    "modelSettings.usage.longTermMemory": "長期メモリ",
    "modelSettings.usage.openConfiguration": "設定を開く",
    "modelSettings.usage.title": "モデルを削除できません",
    "modelSettings.usage.truncated": "{total}件中の最初の{shown}件を表示",
    "modelSettings.vllm.desc": "マルチモーダル理解に使用する視覚言語モデルを設定します",
    "modelSettings.vllm.empty": "VLLMモデルがありません",
    "modelSettings.vllm.title": "VLLM視覚モデル",
    "settings.weknoraCloud.checkingStatus": "認証情報のステータスを確認中...",
    "settings.weknoraCloud.credentialExpired": "認証情報が無効になりました。再設定してください。",
    "settings.weknoraCloud.credentialUnconfigured": "WeKnoraCloudの認証情報が未設定です。先にAPPIDとAPPSECRETを設定してください。",
    "settings.weknoraCloud.goToSettings": "設定へ移動",
    "settings.weknoraCloud.modelHintConfigured": "WeKnoraCloudの認証情報は設定済みです。対応モデルは以下を参照してください:",
    "settings.weknoraCloud.modelHintDocsLink": "APIドキュメント",
  },
  "ko-KR": {
    "common.all": "전체",
    "common.cancel": "취소",
    "common.close": "닫기",
    "common.copied": "복사됨",
    "common.copy": "복사",
    "common.delete": "삭제",
    "common.edit": "편집",
    "common.no": "아니오",
    "common.refresh": "새로고침",
    "common.yes": "예",
    "model.editor.addTitle": "모델 추가",
    "model.editor.apiKeyOptional": "API 키 (선택)",
    "model.editor.apiKeyPlaceholder": "API 키 입력",
    "model.editor.baseUrlLabel": "Base URL",
    "model.editor.baseUrlPlaceholder": "예: https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderAsr": "예: https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderVllm": "예: http://localhost:11434/v1",
    "model.editor.checkDimension": "차원 감지",
    "model.editor.connectionConfigError": "연결 실패, 설정을 확인해주세요",
    "model.editor.connectionFailed": "연결 실패",
    "model.editor.connectionSuccess": "연결 성공",
    "model.editor.contextWindowDefaultHint": "설정되지 않음, 기본값 {value} 사용",
    "model.editor.contextWindowDesc": "모델이 한 요청에 수용할 수 있는 토큰 수입니다. 에이전트 대화 압축이 이 한도를 사용합니다. 비워 두면 기본값 200000(200K)을 사용합니다. 공급자 문서의 실제 값을 입력하세요. 더 크게 설정하면 압축이 발생하지 않고 요청이 거부될 수 있습니다.",
    "model.editor.contextWindowLabel": "컨텍스트 창",
    "model.editor.contextWindowPlaceholder": "기본값 {value}",
    "model.editor.contextWindowTokens": "{count} 토큰",
    "model.editor.customHeadersAdd": "헤더 추가",
    "model.editor.customHeadersDesc": "원격 모델 API 호출 시 추가되는 HTTP 헤더입니다. 기업 게이트웨이 인증이나 추적 등에 사용할 수 있으며, Authorization / Content-Type 같은 예약 헤더는 무시됩니다.",
    "model.editor.customHeadersKeyPlaceholder": "헤더 이름",
    "model.editor.customHeadersLabel": "사용자 정의 요청 헤더 (선택)",
    "model.editor.customHeadersValuePlaceholder": "헤더 값",
    "model.editor.description.asr": "음성 인식 및 오디오 전사를 위한 음성 인식 모델 설정",
    "model.editor.description.chat": "대화용 대규모 언어 모델 설정",
    "model.editor.description.default": "모델 정보 설정",
    "model.editor.description.embedding": "텍스트 벡터화용 임베딩 모델 설정",
    "model.editor.description.rerank": "결과 재정렬용 모델 설정",
    "model.editor.description.vllm": "시각 이해 및 멀티모달용 비전 언어 모델 설정",
    "model.editor.dimensionDetected": "감지 성공, 벡터 차원: {value}",
    "model.editor.dimensionFailed": "감지 실패, 차원을 수동으로 입력해주세요",
    "model.editor.dimensionHint": "모델이 선택되었습니다. \"차원 감지\" 버튼을 클릭하여 벡터 차원을 자동으로 가져옵니다",
    "model.editor.dimensionLabel": "벡터 차원",
    "model.editor.dimensionOverrideDesc": "제공자 문서에서 이 모델이 dimensions 매개변수를 지원한다고 확인한 경우에만 켜세요.",
    "model.editor.dimensionOverrideLabel": "사용자 지정 출력 차원",
    "model.editor.dimensionPlaceholder": "예: 1536",
    "model.editor.displayNameDesc": "UI 표시용으로만 사용되며 실제 호출은 위의 모델 이름을 사용합니다.",
    "model.editor.displayNameLabel": "표시 이름 (선택)",
    "model.editor.displayNamePlaceholder": "예: 고객지원 QA 모델",
    "model.editor.downloadCompleted": "{name} 다운로드 완료",
    "model.editor.downloadFailed": "{name} 다운로드 실패",
    "model.editor.downloadLabel": "다운로드: {keyword}",
    "model.editor.downloadStartFailed": "다운로드 시작 실패",
    "model.editor.downloadStarted": "{name} 다운로드 시작",
    "model.editor.editTitle": "모델 편집",
    "model.editor.fillModelAndUrl": "먼저 모델 식별자와 Base URL을 입력해주세요",
    "model.editor.goToOllamaSettings": "설정 보기",
    "model.editor.listRefreshed": "목록이 새로고침되었습니다",
    "model.editor.lkeap.regionDesc": "RunRerank supports ap-beijing, ap-guangzhou, etc. Default: ap-guangzhou",
    "model.editor.lkeap.regionLabel": "Region",
    "model.editor.lkeap.regionPlaceholder": "ap-guangzhou",
    "model.editor.lkeap.rerankCredentialHint": "Rerank uses Tencent Cloud API signature (not the OpenAI-style LKEAP API key). Create SecretId/SecretKey in the CAM console.",
    "model.editor.lkeap.secretIdLabel": "SecretId",
    "model.editor.lkeap.secretIdPlaceholder": "Tencent Cloud API SecretId",
    "model.editor.lkeap.secretKeyLabel": "SecretKey",
    "model.editor.lkeap.secretKeyPlaceholder": "Tencent Cloud API SecretKey",
    "model.editor.loadModelListFailed": "모델 목록 로드 실패",
    "model.editor.maxConcurrencyDesc": "문서 인덱싱/보강 등 백그라운드 작업이 이 모델을 호출하는 동시 실행 수를 제한합니다(모델별로 모든 복제본이 공유). 0 또는 비워 두면 전역 기본값을 사용하며, 대화형 채팅에는 영향을 주지 않습니다.",
    "model.editor.maxConcurrencyLabel": "백그라운드 동시 실행 상한",
    "model.editor.maxConcurrencyPlaceholder": "0이면 전역 기본값 사용",
    "model.editor.modelNamePlaceholder.local": "예: llama2:latest",
    "model.editor.modelNamePlaceholder.localVllm": "예: llava:latest",
    "model.editor.modelNamePlaceholder.remote": "예: gpt-4, claude-3-opus",
    "model.editor.modelNamePlaceholder.remoteAsr": "예: whisper-1",
    "model.editor.modelNamePlaceholder.remoteVllm": "예: gpt-4-vision-preview",
    "model.editor.ollamaNotSupportRerank": "Ollama는 ReRank 모델을 지원하지 않습니다. 원격 인터페이스를 사용하여 설정해주세요",
    "model.editor.ollamaUnavailable": "Ollama 서비스를 사용할 수 없어 로컬 모델을 선택할 수 없습니다",
    "model.editor.providerLabel": "프로바이더",
    "model.editor.providerPlaceholder": "모델 프로바이더 선택",
    "model.editor.providers.aliyun.description": "qwen-plus, tongyi-embedding-vision-plus, qwen3-rerank 등",
    "model.editor.providers.aliyun.label": "Aliyun DashScope",
    "model.editor.providers.anthropic.description": "Claude models via native Anthropic Messages API",
    "model.editor.providers.anthropic.label": "Anthropic",
    "model.editor.providers.azure_openai.description": "Microsoft Azure에서 호스팅되는 OpenAI 서비스",
    "model.editor.providers.azure_openai.label": "Azure OpenAI",
    "model.editor.providers.deepseek.description": "deepseek-chat, deepseek-reasoner 등",
    "model.editor.providers.deepseek.label": "DeepSeek",
    "model.editor.providers.gemini.description": "gemini-3-flash-preview, gemini-2.5-pro 등",
    "model.editor.providers.gemini.label": "Google Gemini",
    "model.editor.providers.generic.description": "Generic API endpoint",
    "model.editor.providers.generic.label": "사용자 정의 (OpenAI 호환)",
    "model.editor.providers.gpustack.description": "Choose your deployed model on GPUStack",
    "model.editor.providers.gpustack.label": "GPUStack",
    "model.editor.providers.hunyuan.description": "hunyuan-pro, hunyuan-standard, hunyuan-embedding 등",
    "model.editor.providers.hunyuan.label": "Hunyuan",
    "model.editor.providers.jina.description": "jina-clip-v1, jina-embeddings-v2-base-zh, etc.",
    "model.editor.providers.jina.label": "Jina",
    "model.editor.providers.litellm.description": "자체 호스팅 프록시로 OpenAI, Anthropic, Gemini, Bedrock 등 100+ 공급자를 연결합니다. 플레이스홀더 URL을 실제 주소로 바꾸세요. localhost는 SSRF_WHITELIST에 추가해야 합니다.",
    "model.editor.providers.litellm.label": "LiteLLM",
    "model.editor.providers.lkeap.description": "DeepSeek-R1, DeepSeek-V3, lke-reranker-base 등",
    "model.editor.providers.lkeap.label": "텐센트 클라우드 LKEAP",
    "model.editor.providers.longcat.description": "LongCat-Flash-Chat, LongCat-Flash-Thinking, etc.",
    "model.editor.providers.longcat.label": "LongCat AI",
    "model.editor.providers.mimo.description": "mimo-v2-flash",
    "model.editor.providers.mimo.label": "MiMo",
    "model.editor.providers.minimax.description": "MiniMax-M3, MiniMax-M2.7, MiniMax-M2.7-highspeed 등",
    "model.editor.providers.minimax.label": "MiniMax",
    "model.editor.providers.modelscope.description": "Qwen/Qwen3-8B, Qwen/Qwen3-Embedding-8B, etc.",
    "model.editor.providers.modelscope.label": "ModelScope",
    "model.editor.providers.moonshot.description": "kimi-k2-turbo-preview, moonshot-v1-8k-vision-preview, etc.",
    "model.editor.providers.moonshot.label": "Moonshot",
    "model.editor.providers.novita.description": "moonshotai/kimi-k2.5, zai-org/glm-5, minimax/minimax-m2.7, qwen/qwen3-embedding-0.6b 등",
    "model.editor.providers.novita.label": "Novita AI",
    "model.editor.providers.nvidia.description": "deepseek-ai-deepseek-v3_1, nv-embed-v1, rerank-qa-mistral-4b, etc.",
    "model.editor.providers.nvidia.label": "NVIDIA",
    "model.editor.providers.openai.description": "gpt-5.2, gpt-5-mini 등",
    "model.editor.providers.openai.label": "OpenAI",
    "model.editor.providers.openrouter.description": "openai/gpt-5.2-chat, google/gemini-3-flash-preview 등",
    "model.editor.providers.openrouter.label": "OpenRouter",
    "model.editor.providers.qianfan.description": "ernie-5.0-thinking-preview, embedding-v1, bce-reranker-base, etc.",
    "model.editor.providers.qianfan.label": "Baidu Qianfan",
    "model.editor.providers.qiniu.description": "deepseek/deepseek-v3.2-251201, z-ai/glm-4.7, etc.",
    "model.editor.providers.qiniu.label": "Qiniu Cloud",
    "model.editor.providers.requesty.description": "openai/gpt-4o-mini, anthropic/claude-sonnet-4-5 등",
    "model.editor.providers.requesty.label": "Requesty",
    "model.editor.providers.siliconflow.description": "deepseek-ai/DeepSeek-V3.1 등",
    "model.editor.providers.siliconflow.label": "SiliconFlow",
    "model.editor.providers.volcengine.description": "doubao-1-5-pro-32k-250115, doubao-embedding-vision-250615 등",
    "model.editor.providers.volcengine.label": "Volcengine",
    "model.editor.providers.zhipu.description": "glm-4.7, embedding-3, rerank, etc.",
    "model.editor.providers.zhipu.label": "Zhipu BigModel",
    "model.editor.refreshList": "목록 새로고침",
    "model.editor.remoteBaseUrlRequired": "Remote API 유형은 Base URL이 필수입니다",
    "model.editor.remoteDimensionDetected": "감지된 벡터 차원: {value}",
    "model.editor.sectionAdvanced": "고급 옵션",
    "model.editor.sectionProvider": "연결 설정",
    "model.editor.sectionSource": "모델 소스",
    "model.editor.sectionType": "모델 유형",
    "model.editor.sourceLabel": "모델 소스",
    "model.editor.sourceLocal": "Ollama",
    "model.editor.sourceRemote": "API",
    "model.editor.supportsVisionDesc": "모델의 이미지 등 멀티모달 입력 지원 여부",
    "model.editor.supportsVisionLabel": "비전/멀티모달 지원",
    "model.editor.testConnection": "연결 테스트",
    "model.editor.testing": "테스트 중...",
    "model.editor.thinkingControl.chatTemplateKwargs.hint": "사용자 정의 OpenAI 호환, NVIDIA NIM, vLLM / 로컬 Qwen 배포",
    "model.editor.thinkingControl.chatTemplateKwargs.label": "chat_template_kwargs",
    "model.editor.thinkingControl.enableThinking.hint": "Alibaba DashScope: qwen3, qwen-plus, qwen-max, qwen-turbo",
    "model.editor.thinkingControl.enableThinking.label": "enable_thinking",
    "model.editor.thinkingControl.none.hint": "에이전트 「사고 모드」 스위치가 효과 없음, 요청에 사고 관련 매개변수를 보내지 않음",
    "model.editor.thinkingControl.none.label": "사고 매개변수 전송 안 함",
    "model.editor.thinkingControl.thinkingType.hint": "Volcengine Ark; Tencent LKEAP (DeepSeek V3 등, LKEAP 기본값; R1은 「전송 안 함」)",
    "model.editor.thinkingControl.thinkingType.label": "thinking.type",
    "model.editor.thinkingControlDesc": "에이전트 「사고 모드」 켜기/끄기 시 API에 어떻게 기록할지 결정합니다. 벤더/모델에 따라 미리 선택되며, 실제 API와 다르면 문서에 맞게 수정하세요. 「전송 안 함」을 선택하면 에이전트 「사고 모드」 스위치가 효과가 없습니다.",
    "model.editor.thinkingControlLabel": "사고 모드 매개변수 형식",
    "model.editor.typeLabel": "모델 유형",
    "model.editor.unsupportedModelType": "지원되지 않는 모델 유형",
    "model.editor.validation.baseUrlEmpty": "Base URL은 비워둘 수 없습니다",
    "model.editor.validation.baseUrlInvalid": "Base URL 형식이 올바르지 않습니다. 유효한 URL을 입력해주세요",
    "model.editor.validation.baseUrlRequired": "Base URL을 입력해주세요",
    "model.editor.validation.modelNameEmpty": "모델 이름은 비워둘 수 없습니다",
    "model.editor.validation.modelNameMax": "모델 이름은 100자를 초과할 수 없습니다",
    "model.editor.validation.modelNameRequired": "모델 이름을 입력해주세요",
    "model.editor.volcengine.accessKeyLabel": "Access Key ID",
    "model.editor.volcengine.accessKeyPlaceholder": "Volcengine Access Key ID",
    "model.editor.volcengine.rerankCredentialHint": "Rerank은 Ark API 키가 아닌 VikingDB AK/SK 서명을 사용합니다. 권장 모델: doubao-seed-rerank.",
    "model.editor.volcengine.secretKeyLabel": "Secret Access Key",
    "model.editor.volcengine.secretKeyPlaceholder": "Volcengine Secret Access Key",
    "model.modelName": "모델 이름",
    "model.searchPlaceholder": "모델 검색...",
    "modelSettings.actions.addModel": "모델 추가",
    "modelSettings.actions.debugModel": "모델 테스트",
    "modelSettings.asr.desc": "음성 인식 및 오디오 전사를 위한 음성 인식 모델 설정 (예: OpenAI Whisper)",
    "modelSettings.asr.empty": "ASR 음성 모델 없음",
    "modelSettings.asr.title": "ASR 음성 모델",
    "modelSettings.builtinModels.description": "기본 제공 모델은 모든 워크스페이스에게 표시됩니다. 민감한 정보는 숨겨지며, 편집하거나 삭제할 수 없습니다.",
    "modelSettings.builtinModels.descriptionAdmin": "기본 제공 모델은 모든 워크스페이스에게 표시됩니다. 시스템 관리자는 구성과 자격 증명을 편집할 수 있으며, 삭제는 배포 구성에서 관리됩니다.",
    "modelSettings.builtinModels.title": "기본 제공 모델",
    "modelSettings.builtinModels.viewGuide": "기본 제공 모델 관리 가이드 보기",
    "modelSettings.builtinTag": "기본제공",
    "modelSettings.chat.desc": "대화용 대규모 언어 모델 설정",
    "modelSettings.chat.empty": "대화 모델 없음",
    "modelSettings.chat.title": "대화 모델",
    "modelSettings.confirmDelete": "모델 \"{name}\"을(를) 삭제하시겠습니까?",
    "modelSettings.copySuffix": " 사본",
    "modelSettings.debug.audioFile": "오디오 파일",
    "modelSettings.debug.chooseFile": "파일 선택",
    "modelSettings.debug.copyResult": "결과 복사",
    "modelSettings.debug.description": "구성된 모델에 실제 요청을 보내고 응답과 소요 시간을 확인합니다",
    "modelSettings.debug.documents": "후보 문서",
    "modelSettings.debug.documentsHint": "비어 있지 않은 각 줄은 ReRank 모델에 별도 문서로 전송됩니다",
    "modelSettings.debug.documentsPlaceholder": "후보 문서를 한 줄에 하나씩 입력하세요",
    "modelSettings.debug.embeddingInput": "벡터화할 텍스트",
    "modelSettings.debug.embeddingPlaceholder": "Embedding을 생성할 텍스트를 입력하세요",
    "modelSettings.debug.failed": "호출 실패",
    "modelSettings.debug.groupInput": "테스트 입력",
    "modelSettings.debug.groupModel": "모델 선택",
    "modelSettings.debug.groupResult": "실행 결과",
    "modelSettings.debug.history": "기록",
    "modelSettings.debug.imageFile": "이미지 파일",
    "modelSettings.debug.metrics.answerChars": "답변 문자 수",
    "modelSettings.debug.metrics.dimension": "벡터 차원",
    "modelSettings.debug.metrics.reasoningChars": "추론 문자 수",
    "modelSettings.debug.metrics.reasoningReturned": "추론 내용 반환",
    "modelSettings.debug.metrics.resultCount": "결과 수",
    "modelSettings.debug.metrics.segmentCount": "세그먼트 수",
    "modelSettings.debug.metrics.textChars": "전사 문자 수",
    "modelSettings.debug.model": "모델",
    "modelSettings.debug.modelPlaceholder": "테스트할 모델을 선택하세요",
    "modelSettings.debug.modelType": "모델 유형",
    "modelSettings.debug.noModelsForType": "이 유형의 저장된 모델이 없습니다",
    "modelSettings.debug.parameters": "요청 매개변수",
    "modelSettings.debug.query": "입력 내용",
    "modelSettings.debug.queryPlaceholder": "모델에 보낼 내용을 입력하세요",
    "modelSettings.debug.rawResponse": "응답 내용",
    "modelSettings.debug.requestFailed": "모델 테스트 요청 실패",
    "modelSettings.debug.requestPreview": "요청 미리보기",
    "modelSettings.debug.run": "테스트 실행",
    "modelSettings.debug.runLabel": "{n}번째 실행",
    "modelSettings.debug.success": "호출 성공",
    "modelSettings.debug.systemPrompt": "System Prompt",
    "modelSettings.debug.systemPromptPlaceholder": "선택 사항, 시스템 프롬프트 입력",
    "modelSettings.debug.thinkOff": "사고 꺼짐",
    "modelSettings.debug.thinkOn": "사고 켜짐",
    "modelSettings.debug.thinking": "사고 모드",
    "modelSettings.debug.thinkingDesc": "사고 모드를 지원하는 모델에만 적용됩니다",
    "modelSettings.debug.title": "모델 테스트",
    "modelSettings.debug.vlmPrompt": "이미지 프롬프트",
    "modelSettings.debug.vlmPromptPlaceholder": "예: 이 이미지를 자세히 설명해 주세요",
    "modelSettings.description": "다양한 유형의 AI 모델을 관리합니다. Ollama 로컬 모델과 원격 API를 지원합니다",
    "modelSettings.embedding.desc": "텍스트 벡터화용 임베딩 모델 설정",
    "modelSettings.embedding.empty": "Embedding 모델 없음",
    "modelSettings.embedding.title": "Embedding 모델",
    "modelSettings.rerank.desc": "결과 재정렬용 모델 설정",
    "modelSettings.rerank.empty": "ReRank 모델 없음",
    "modelSettings.rerank.title": "ReRank 모델",
    "modelSettings.source.custom": "사용자 지정",
    "modelSettings.source.openaiCompatible": "OpenAI 호환",
    "modelSettings.source.remote": "Remote",
    "modelSettings.title": "모델 설정",
    "modelSettings.toasts.added": "모델이 추가되었습니다",
    "modelSettings.toasts.baseUrlInvalid": "Base URL 형식이 올바르지 않습니다. 유효한 URL을 입력해주세요",
    "modelSettings.toasts.baseUrlRequired": "Remote API 유형은 Base URL이 필수입니다",
    "modelSettings.toasts.builtinCannotCopy": "기본 제공 모델은 복사할 수 없습니다",
    "modelSettings.toasts.builtinCannotDelete": "기본 제공 모델은 삭제할 수 없습니다",
    "modelSettings.toasts.builtinCannotEdit": "기본 제공 모델은 편집할 수 없습니다",
    "modelSettings.toasts.copied": "모델이 복사되었습니다",
    "modelSettings.toasts.copyFailed": "모델 복사에 실패했습니다",
    "modelSettings.toasts.deleteFailed": "모델 삭제 실패",
    "modelSettings.toasts.deleted": "모델이 삭제되었습니다",
    "modelSettings.toasts.dimensionInvalid": "Embedding 모델은 유효한 벡터 차원(128-4096)을 입력해야 합니다",
    "modelSettings.toasts.displayNameTooLong": "표시 이름은 100자를 초과할 수 없습니다",
    "modelSettings.toasts.nameRequired": "모델 이름은 비워둘 수 없습니다",
    "modelSettings.toasts.nameTooLong": "모델 이름은 100자를 초과할 수 없습니다",
    "modelSettings.toasts.saveFailed": "모델 저장 실패",
    "modelSettings.toasts.updated": "모델이 업데이트되었습니다",
    "modelSettings.typeShort.asr": "음성",
    "modelSettings.typeShort.chat": "대화",
    "modelSettings.typeShort.embedding": "Embedding",
    "modelSettings.typeShort.rerank": "ReRank",
    "modelSettings.typeShort.vllm": "비전",
    "modelSettings.usage.agents": "에이전트 ({count})",
    "modelSettings.usage.bindings.asr_model": "음성 인식 모델",
    "modelSettings.usage.bindings.chat_model": "대화 모델",
    "modelSettings.usage.bindings.embedding_model": "Embedding 모델",
    "modelSettings.usage.bindings.extract_model": "메모리 추출 모델",
    "modelSettings.usage.bindings.follow_up_model": "후속 질문 모델",
    "modelSettings.usage.bindings.image_processing_model": "이미지 처리 모델",
    "modelSettings.usage.bindings.query_understand_model": "질의 이해 모델",
    "modelSettings.usage.bindings.rerank_model": "재정렬 모델",
    "modelSettings.usage.bindings.summary_model": "요약 모델",
    "modelSettings.usage.bindings.unknown": "기타 모델 설정",
    "modelSettings.usage.bindings.vlm_model": "비전 모델",
    "modelSettings.usage.bindings.wiki_synthesis_model": "Wiki 종합 모델",
    "modelSettings.usage.description": "모델 \"{name}\"이(가) 다음 설정에서 사용 중입니다. 각 설정을 열어 다른 모델로 변경한 후 다시 삭제하세요.",
    "modelSettings.usage.knowledgeBases": "지식 베이스 ({count})",
    "modelSettings.usage.longTermMemory": "장기 메모리",
    "modelSettings.usage.openConfiguration": "설정 열기",
    "modelSettings.usage.title": "모델을 삭제할 수 없습니다",
    "modelSettings.usage.truncated": "전체 {total}개 중 앞 {shown}개만 표시합니다",
    "modelSettings.vllm.desc": "시각 이해 및 멀티모달용 비전 언어 모델 설정",
    "modelSettings.vllm.empty": "VLLM 비전 모델 없음",
    "modelSettings.vllm.title": "VLLM 비전 모델",
    "settings.weknoraCloud.checkingStatus": "자격 증명 상태 확인 중...",
    "settings.weknoraCloud.credentialExpired": "자격 증명이 만료되었습니다. 재설정하세요.",
    "settings.weknoraCloud.credentialUnconfigured": "WeKnoraCloud 자격 증명이 설정되지 않았습니다. APPID와 APPSECRET을 먼저 설정하세요.",
    "settings.weknoraCloud.goToSettings": "설정으로 이동",
    "settings.weknoraCloud.modelHintConfigured": "WeKnoraCloud 자격 증명이 설정되었습니다. 지원 모델은",
    "settings.weknoraCloud.modelHintDocsLink": "API 문서",
  },
  "ru-RU": {
    "common.all": "Все",
    "common.cancel": "Отмена",
    "common.close": "Закрыть",
    "common.copied": "Скопировано",
    "common.copy": "Копировать",
    "common.delete": "Удалить",
    "common.edit": "Редактировать",
    "common.no": "Нет",
    "common.refresh": "Обновить",
    "common.yes": "Да",
    "model.editor.addTitle": "Добавить модель",
    "model.editor.apiKeyOptional": "API Key (опционально)",
    "model.editor.apiKeyPlaceholder": "Введите API Key",
    "model.editor.baseUrlLabel": "Base URL",
    "model.editor.baseUrlPlaceholder": "например: https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderAsr": "например: https://api.openai.com/v1",
    "model.editor.baseUrlPlaceholderVllm": "например: http://localhost:11434/v1",
    "model.editor.checkDimension": "Определить размерность",
    "model.editor.connectionConfigError": "Соединение не установлено, проверьте конфигурацию",
    "model.editor.connectionFailed": "Соединение не установлено",
    "model.editor.connectionSuccess": "Соединение установлено",
    "model.editor.contextWindowDefaultHint": "Не задано, используется значение по умолчанию {value}",
    "model.editor.contextWindowDesc": "Сколько токенов модель принимает за один запрос. Сжатие истории агента использует этот лимит. Пустое значение — по умолчанию 200000 (200K). Укажите реальное окно провайдера: завышенное значение не запускает сжатие, и провайдер отклоняет запрос.",
    "model.editor.contextWindowLabel": "Контекстное окно",
    "model.editor.contextWindowPlaceholder": "По умолчанию {value}",
    "model.editor.contextWindowTokens": "{count} токенов",
    "model.editor.customHeadersAdd": "Добавить заголовок",
    "model.editor.customHeadersDesc": "Дополнительные HTTP-заголовки, добавляемые к запросам к удалённому API модели (например, для авторизации корпоративного шлюза или трассировки). Зарезервированные заголовки, такие как Authorization / Content-Type, игнорируются.",
    "model.editor.customHeadersKeyPlaceholder": "Имя заголовка",
    "model.editor.customHeadersLabel": "Пользовательские заголовки запроса (опционально)",
    "model.editor.customHeadersValuePlaceholder": "Значение заголовка",
    "model.editor.description.asr": "Настройте модель распознавания речи для транскрибации аудио",
    "model.editor.description.chat": "Настройте языковую модель для диалогов",
    "model.editor.description.default": "Настройте информацию о модели",
    "model.editor.description.embedding": "Настройте модель встраивания для текстовой векторизации",
    "model.editor.description.rerank": "Настройте модель для повторного ранжирования результатов",
    "model.editor.description.vllm": "Настройте визуально-языковую модель для мультимодального понимания",
    "model.editor.dimensionDetected": "Определение выполнено, размерность: {value}",
    "model.editor.dimensionFailed": "Не удалось определить, введите размерность вручную",
    "model.editor.dimensionHint": "Модель выбрана. Нажмите «Определить размерность», чтобы автоматически получить значение.",
    "model.editor.dimensionLabel": "Размерность вектора",
    "model.editor.dimensionOverrideDesc": "Включайте только если документация провайдера подтверждает поддержку параметра dimensions.",
    "model.editor.dimensionOverrideLabel": "Пользовательская выходная размерность",
    "model.editor.dimensionPlaceholder": "например: 1536",
    "model.editor.displayNameDesc": "Используется только в интерфейсе. Для вызовов по-прежнему используется имя модели выше.",
    "model.editor.displayNameLabel": "Отображаемое имя (опционально)",
    "model.editor.displayNamePlaceholder": "например: модель поддержки",
    "model.editor.downloadCompleted": "{name} успешно загружена",
    "model.editor.downloadFailed": "Не удалось загрузить {name}",
    "model.editor.downloadLabel": "Скачать: {keyword}",
    "model.editor.downloadStartFailed": "Не удалось запустить загрузку",
    "model.editor.downloadStarted": "Начата загрузка {name}",
    "model.editor.editTitle": "Редактировать модель",
    "model.editor.fillModelAndUrl": "Сначала заполните идентификатор модели и Base URL",
    "model.editor.goToOllamaSettings": "Открыть настройки",
    "model.editor.listRefreshed": "Список обновлён",
    "model.editor.lkeap.regionDesc": "RunRerank supports ap-beijing, ap-guangzhou, etc. Default: ap-guangzhou",
    "model.editor.lkeap.regionLabel": "Region",
    "model.editor.lkeap.regionPlaceholder": "ap-guangzhou",
    "model.editor.lkeap.rerankCredentialHint": "Rerank uses Tencent Cloud API signature (not the OpenAI-style LKEAP API key). Create SecretId/SecretKey in the CAM console.",
    "model.editor.lkeap.secretIdLabel": "SecretId",
    "model.editor.lkeap.secretIdPlaceholder": "Tencent Cloud API SecretId",
    "model.editor.lkeap.secretKeyLabel": "SecretKey",
    "model.editor.lkeap.secretKeyPlaceholder": "Tencent Cloud API SecretKey",
    "model.editor.loadModelListFailed": "Не удалось загрузить список моделей",
    "model.editor.maxConcurrencyDesc": "Ограничивает число одновременных фоновых вызовов (индексация/обогащение) к этой модели, общее для модели по всем репликам. 0 или пусто — используется глобальное значение по умолчанию; интерактивный чат не затрагивается.",
    "model.editor.maxConcurrencyLabel": "Лимит фоновой параллельности",
    "model.editor.maxConcurrencyPlaceholder": "0 — использовать глобальное значение",
    "model.editor.modelNamePlaceholder.local": "например: llama2:latest",
    "model.editor.modelNamePlaceholder.localVllm": "например: llava:latest",
    "model.editor.modelNamePlaceholder.remote": "например: gpt-4, claude-3-opus",
    "model.editor.modelNamePlaceholder.remoteAsr": "например: whisper-1",
    "model.editor.modelNamePlaceholder.remoteVllm": "например: gpt-4-vision-preview",
    "model.editor.ollamaNotSupportRerank": "Ollama не поддерживает модели ReRank, используйте удалённый API",
    "model.editor.ollamaUnavailable": "Сервис Ollama недоступен, локальные модели недоступны для выбора",
    "model.editor.providerLabel": "Провайдер",
    "model.editor.providerPlaceholder": "Выберите провайдера модели",
    "model.editor.providers.aliyun.description": "qwen-plus, tongyi-embedding-vision-plus, qwen3-rerank, etc.",
    "model.editor.providers.aliyun.label": "Aliyun DashScope",
    "model.editor.providers.anthropic.description": "Claude models via native Anthropic Messages API",
    "model.editor.providers.anthropic.label": "Anthropic",
    "model.editor.providers.azure_openai.description": "Сервис OpenAI на платформе Microsoft Azure",
    "model.editor.providers.azure_openai.label": "Azure OpenAI",
    "model.editor.providers.deepseek.description": "deepseek-chat, deepseek-reasoner, etc.",
    "model.editor.providers.deepseek.label": "DeepSeek",
    "model.editor.providers.gemini.description": "gemini-3-flash-preview, gemini-2.5-pro, etc.",
    "model.editor.providers.gemini.label": "Google Gemini",
    "model.editor.providers.generic.description": "Generic API endpoint",
    "model.editor.providers.generic.label": "Пользовательский (OpenAI-совместимый)",
    "model.editor.providers.gpustack.description": "Choose your deployed model on GPUStack",
    "model.editor.providers.gpustack.label": "GPUStack",
    "model.editor.providers.hunyuan.description": "hunyuan-pro, hunyuan-standard, hunyuan-embedding, etc.",
    "model.editor.providers.hunyuan.label": "Hunyuan",
    "model.editor.providers.jina.description": "jina-clip-v1, jina-embeddings-v2-base-zh, etc.",
    "model.editor.providers.jina.label": "Jina",
    "model.editor.providers.litellm.description": "Self-hosted прокси к 100+ провайдерам (OpenAI, Anthropic, Gemini, Bedrock и др.). Замените URL-заглушку; localhost нужно добавить в SSRF_WHITELIST.",
    "model.editor.providers.litellm.label": "LiteLLM",
    "model.editor.providers.lkeap.description": "DeepSeek-R1, DeepSeek-V3, lke-reranker-base и др.",
    "model.editor.providers.lkeap.label": "Tencent Cloud LKEAP",
    "model.editor.providers.longcat.description": "LongCat-Flash-Chat, LongCat-Flash-Thinking, etc.",
    "model.editor.providers.longcat.label": "LongCat AI",
    "model.editor.providers.mimo.description": "mimo-v2-flash",
    "model.editor.providers.mimo.label": "MiMo",
    "model.editor.providers.minimax.description": "MiniMax-M3, MiniMax-M2.7, MiniMax-M2.7-highspeed, etc.",
    "model.editor.providers.minimax.label": "MiniMax",
    "model.editor.providers.modelscope.description": "Qwen/Qwen3-8B, Qwen/Qwen3-Embedding-8B, etc.",
    "model.editor.providers.modelscope.label": "ModelScope",
    "model.editor.providers.moonshot.description": "kimi-k2-turbo-preview, moonshot-v1-8k-vision-preview, etc.",
    "model.editor.providers.moonshot.label": "Moonshot",
    "model.editor.providers.novita.description": "moonshotai/kimi-k2.5, zai-org/glm-5, minimax/minimax-m2.7, qwen/qwen3-embedding-0.6b, etc.",
    "model.editor.providers.novita.label": "Novita AI",
    "model.editor.providers.nvidia.description": "deepseek-ai-deepseek-v3_1, nv-embed-v1, rerank-qa-mistral-4b, etc.",
    "model.editor.providers.nvidia.label": "NVIDIA",
    "model.editor.providers.openai.description": "gpt-5.2, gpt-5-mini, etc.",
    "model.editor.providers.openai.label": "OpenAI",
    "model.editor.providers.openrouter.description": "openai/gpt-5.2-chat, google/gemini-3-flash-preview, etc.",
    "model.editor.providers.openrouter.label": "OpenRouter",
    "model.editor.providers.qianfan.description": "ernie-5.0-thinking-preview, embedding-v1, bce-reranker-base, etc.",
    "model.editor.providers.qianfan.label": "Baidu Qianfan",
    "model.editor.providers.qiniu.description": "deepseek/deepseek-v3.2-251201, z-ai/glm-4.7, etc.",
    "model.editor.providers.qiniu.label": "Qiniu Cloud",
    "model.editor.providers.requesty.description": "openai/gpt-4o-mini, anthropic/claude-sonnet-4-5, etc.",
    "model.editor.providers.requesty.label": "Requesty",
    "model.editor.providers.siliconflow.description": "deepseek-ai/DeepSeek-V3.1, etc.",
    "model.editor.providers.siliconflow.label": "SiliconFlow",
    "model.editor.providers.volcengine.description": "doubao-1-5-pro-32k-250115, doubao-embedding-vision-250615, etc.",
    "model.editor.providers.volcengine.label": "Volcengine",
    "model.editor.providers.zhipu.description": "glm-4.7, embedding-3, rerank, etc.",
    "model.editor.providers.zhipu.label": "Zhipu BigModel",
    "model.editor.refreshList": "Обновить список",
    "model.editor.remoteBaseUrlRequired": "Для Remote API необходимо указать Base URL",
    "model.editor.remoteDimensionDetected": "Обнаружена размерность: {value}",
    "model.editor.sectionAdvanced": "Дополнительные параметры",
    "model.editor.sectionProvider": "Настройки провайдера",
    "model.editor.sectionSource": "Источник",
    "model.editor.sectionType": "Тип модели",
    "model.editor.sourceLabel": "Источник модели",
    "model.editor.sourceLocal": "Ollama",
    "model.editor.sourceRemote": "API",
    "model.editor.supportsVisionDesc": "Поддерживает ли модель изображения и другой мультимодальный ввод",
    "model.editor.supportsVisionLabel": "Поддержка визуального / мультимодального ввода",
    "model.editor.testConnection": "Проверить соединение",
    "model.editor.testing": "Проверка...",
    "model.editor.thinkingControl.chatTemplateKwargs.hint": "Пользовательские OpenAI-совместимые шлюзы, NVIDIA NIM, vLLM / локальный Qwen",
    "model.editor.thinkingControl.chatTemplateKwargs.label": "chat_template_kwargs",
    "model.editor.thinkingControl.enableThinking.hint": "Alibaba DashScope: qwen3, qwen-plus, qwen-max, qwen-turbo",
    "model.editor.thinkingControl.enableThinking.label": "enable_thinking",
    "model.editor.thinkingControl.none.hint": "Переключатель «Режим размышления» агента не действует; параметры размышления не отправляются в запросе",
    "model.editor.thinkingControl.none.label": "Не отправлять параметры размышления",
    "model.editor.thinkingControl.thinkingType.hint": "Volcengine Ark; Tencent LKEAP (DeepSeek V3 и др.; по умолчанию для LKEAP; для R1 — «Не отправлять»)",
    "model.editor.thinkingControl.thinkingType.label": "thinking.type",
    "model.editor.thinkingControlDesc": "Определяет, как переключатель «Режим размышления» агента записывается в API. При возможности выбирается по поставщику/модели; при несоответствии измените по документации API. При выборе «Не отправлять» переключатель «Режим размышления» агента не действует.",
    "model.editor.thinkingControlLabel": "Формат параметров режима размышления",
    "model.editor.typeLabel": "Тип модели",
    "model.editor.unsupportedModelType": "Неподдерживаемый тип модели",
    "model.editor.validation.baseUrlEmpty": "Base URL не может быть пустым",
    "model.editor.validation.baseUrlInvalid": "Недопустимый Base URL, введите корректный адрес",
    "model.editor.validation.baseUrlRequired": "Введите Base URL",
    "model.editor.validation.modelNameEmpty": "Название модели не может быть пустым",
    "model.editor.validation.modelNameMax": "Название модели не может превышать 100 символов",
    "model.editor.validation.modelNameRequired": "Введите название модели",
    "model.editor.volcengine.accessKeyLabel": "Access Key ID",
    "model.editor.volcengine.accessKeyPlaceholder": "Volcengine Access Key ID",
    "model.editor.volcengine.rerankCredentialHint": "Rerank использует подпись VikingDB AK/SK, а не Ark API key. Рекомендуемая модель: doubao-seed-rerank.",
    "model.editor.volcengine.secretKeyLabel": "Secret Access Key",
    "model.editor.volcengine.secretKeyPlaceholder": "Volcengine Secret Access Key",
    "model.modelName": "Название модели",
    "model.searchPlaceholder": "Поиск моделей...",
    "modelSettings.actions.addModel": "Добавить модель",
    "modelSettings.actions.debugModel": "Тест модели",
    "modelSettings.asr.desc": "Модели распознавания речи для транскрибации аудио (например, OpenAI Whisper)",
    "modelSettings.asr.empty": "Нет ASR моделей",
    "modelSettings.asr.title": "ASR модели речи",
    "modelSettings.builtinModels.description": "Встроенные модели видны всем пространствам. Конфиденциальная информация скрыта, их нельзя редактировать или удалять.",
    "modelSettings.builtinModels.descriptionAdmin": "Встроенные модели видны всем пространствам. Системные администраторы могут изменять конфигурацию и учетные данные; удаление управляется конфигурацией развертывания.",
    "modelSettings.builtinModels.title": "Встроенные модели",
    "modelSettings.builtinModels.viewGuide": "Посмотреть руководство по управлению встроенными моделями",
    "modelSettings.builtinTag": "Встроенная",
    "modelSettings.chat.desc": "Модели для диалога",
    "modelSettings.chat.empty": "Нет моделей диалога",
    "modelSettings.chat.title": "Модели диалога",
    "modelSettings.confirmDelete": "Удалить модель «{name}»?",
    "modelSettings.copySuffix": " копия",
    "modelSettings.debug.audioFile": "Аудиофайл",
    "modelSettings.debug.chooseFile": "Выбрать файл",
    "modelSettings.debug.copyResult": "Копировать результат",
    "modelSettings.debug.description": "Отправьте реальный запрос к настроенной модели и проверьте ответ и время выполнения",
    "modelSettings.debug.documents": "Кандидаты документов",
    "modelSettings.debug.documentsHint": "Каждая непустая строка отправляется в модель ReRank как отдельный документ",
    "modelSettings.debug.documentsPlaceholder": "Введите один документ на строку",
    "modelSettings.debug.embeddingInput": "Текст для векторизации",
    "modelSettings.debug.embeddingPlaceholder": "Введите текст для создания embedding",
    "modelSettings.debug.failed": "Запрос не выполнен",
    "modelSettings.debug.groupInput": "Тестовый ввод",
    "modelSettings.debug.groupModel": "Выбор модели",
    "modelSettings.debug.groupResult": "Результат",
    "modelSettings.debug.history": "История",
    "modelSettings.debug.imageFile": "Файл изображения",
    "modelSettings.debug.metrics.answerChars": "Символов в ответе",
    "modelSettings.debug.metrics.dimension": "Размерность вектора",
    "modelSettings.debug.metrics.reasoningChars": "Символов рассуждения",
    "modelSettings.debug.metrics.reasoningReturned": "Рассуждение возвращено",
    "modelSettings.debug.metrics.resultCount": "Количество результатов",
    "modelSettings.debug.metrics.segmentCount": "Количество сегментов",
    "modelSettings.debug.metrics.textChars": "Символов транскрипции",
    "modelSettings.debug.model": "Модель",
    "modelSettings.debug.modelPlaceholder": "Выберите модель для теста",
    "modelSettings.debug.modelType": "Тип модели",
    "modelSettings.debug.noModelsForType": "Нет сохранённых моделей этого типа",
    "modelSettings.debug.parameters": "Параметры запроса",
    "modelSettings.debug.query": "Ввод",
    "modelSettings.debug.queryPlaceholder": "Введите текст для отправки модели",
    "modelSettings.debug.rawResponse": "Ответ",
    "modelSettings.debug.requestFailed": "Не удалось выполнить тест модели",
    "modelSettings.debug.requestPreview": "Просмотр запроса",
    "modelSettings.debug.run": "Запустить тест",
    "modelSettings.debug.runLabel": "Запуск №{n}",
    "modelSettings.debug.success": "Запрос выполнен",
    "modelSettings.debug.systemPrompt": "System Prompt",
    "modelSettings.debug.systemPromptPlaceholder": "Необязательно: системный промпт",
    "modelSettings.debug.thinkOff": "Размышление выкл.",
    "modelSettings.debug.thinkOn": "Размышление вкл.",
    "modelSettings.debug.thinking": "Режим размышления",
    "modelSettings.debug.thinkingDesc": "Применяется только к моделям с поддержкой размышления",
    "modelSettings.debug.title": "Тест модели",
    "modelSettings.debug.vlmPrompt": "Промпт для изображения",
    "modelSettings.debug.vlmPromptPlaceholder": "Например: подробно опишите это изображение",
    "modelSettings.description": "Управление типами AI‑моделей: локальные (Ollama) и удалённые API",
    "modelSettings.embedding.desc": "Модели для векторизации текста",
    "modelSettings.embedding.empty": "Нет моделей встраивания",
    "modelSettings.embedding.title": "Модели встраивания",
    "modelSettings.rerank.desc": "Модели для повторной ранжировки результатов",
    "modelSettings.rerank.empty": "Нет моделей ReRank",
    "modelSettings.rerank.title": "Модели ReRank",
    "modelSettings.source.custom": "Своё",
    "modelSettings.source.openaiCompatible": "Совместимо с OpenAI",
    "modelSettings.source.remote": "Удалённая",
    "modelSettings.title": "Настройки моделей",
    "modelSettings.toasts.added": "Модель добавлена",
    "modelSettings.toasts.baseUrlInvalid": "Некорректный Base URL, укажите правильный адрес",
    "modelSettings.toasts.baseUrlRequired": "Для удалённых API требуется Base URL",
    "modelSettings.toasts.builtinCannotCopy": "Встроенные модели нельзя копировать",
    "modelSettings.toasts.builtinCannotDelete": "Встроенные модели нельзя удалить",
    "modelSettings.toasts.builtinCannotEdit": "Встроенные модели нельзя редактировать",
    "modelSettings.toasts.copied": "Модель скопирована",
    "modelSettings.toasts.copyFailed": "Не удалось скопировать модель",
    "modelSettings.toasts.deleteFailed": "Не удалось удалить модель",
    "modelSettings.toasts.deleted": "Модель удалена",
    "modelSettings.toasts.dimensionInvalid": "Размерность встраивания должна быть 128–4096",
    "modelSettings.toasts.displayNameTooLong": "Отображаемое имя не может превышать 100 символов",
    "modelSettings.toasts.nameRequired": "Название модели не может быть пустым",
    "modelSettings.toasts.nameTooLong": "Название модели не может превышать 100 символов",
    "modelSettings.toasts.saveFailed": "Не удалось сохранить модель",
    "modelSettings.toasts.updated": "Модель обновлена",
    "modelSettings.typeShort.asr": "Речь",
    "modelSettings.typeShort.chat": "Чат",
    "modelSettings.typeShort.embedding": "Embedding",
    "modelSettings.typeShort.rerank": "ReRank",
    "modelSettings.typeShort.vllm": "Зрение",
    "modelSettings.usage.agents": "Агенты ({count})",
    "modelSettings.usage.bindings.asr_model": "Модель распознавания речи",
    "modelSettings.usage.bindings.chat_model": "Диалоговая модель",
    "modelSettings.usage.bindings.embedding_model": "Модель эмбеддингов",
    "modelSettings.usage.bindings.extract_model": "Модель извлечения памяти",
    "modelSettings.usage.bindings.follow_up_model": "Модель уточняющих вопросов",
    "modelSettings.usage.bindings.image_processing_model": "Модель обработки изображений",
    "modelSettings.usage.bindings.query_understand_model": "Модель понимания запроса",
    "modelSettings.usage.bindings.rerank_model": "Модель реранжирования",
    "modelSettings.usage.bindings.summary_model": "Модель суммаризации",
    "modelSettings.usage.bindings.unknown": "Другая настройка модели",
    "modelSettings.usage.bindings.vlm_model": "Модель компьютерного зрения",
    "modelSettings.usage.bindings.wiki_synthesis_model": "Модель синтеза Wiki",
    "modelSettings.usage.description": "Модель «{name}» используется в следующих настройках. Откройте каждую конфигурацию и выберите другую модель, затем повторите удаление.",
    "modelSettings.usage.knowledgeBases": "Базы знаний ({count})",
    "modelSettings.usage.longTermMemory": "Долговременная память",
    "modelSettings.usage.openConfiguration": "Открыть настройки",
    "modelSettings.usage.title": "Модель нельзя удалить",
    "modelSettings.usage.truncated": "Показаны первые {shown} из {total}",
    "modelSettings.vllm.desc": "Визуально-языковые модели для мультимодального понимания",
    "modelSettings.vllm.empty": "Нет VLLM моделей",
    "modelSettings.vllm.title": "VLLM модели зрения",
    "settings.weknoraCloud.checkingStatus": "Проверка статуса...",
    "settings.weknoraCloud.credentialExpired": "Данные истекли. Перенастройте.",
    "settings.weknoraCloud.credentialUnconfigured": "Учётные данные WeKnoraCloud не настроены. Заполните APPID и APPSECRET.",
    "settings.weknoraCloud.goToSettings": "Перейти в настройки",
    "settings.weknoraCloud.modelHintConfigured": "Учётные данные WeKnoraCloud настроены. Поддерживаемые модели см. в",
    "settings.weknoraCloud.modelHintDocsLink": "документации API",
  },
};
