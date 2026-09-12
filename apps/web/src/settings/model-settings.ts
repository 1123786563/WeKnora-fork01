import type { ModelConfiguration } from '@weknora/api-client';

export type ModelType = 'chat' | 'embedding' | 'rerank' | 'vllm' | 'asr';
export interface ModelDraft {
  id?: string;
  name: string;
  displayName: string;
  type: ModelType;
  source: 'remote' | 'local';
  provider: string;
  baseUrl: string;
  dimension: number | '';
  supportsVision: boolean;
  contextWindow: number | '';
  maxConcurrency: number | '';
  thinkingControl: string;
  customHeaders: Record<string, string>;
  apiKey: string;
  appSecret: string;
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

function backendModelType(type: ModelType): 'KnowledgeQA' | 'Embedding' | 'Rerank' | 'VLLM' | 'ASR' {
  return type === 'chat' ? 'KnowledgeQA' : type[0]!.toUpperCase() + type.slice(1) as 'Embedding' | 'Rerank' | 'VLLM' | 'ASR';
}

export function modelDraftFromRecord(value: ModelConfiguration): ModelDraft {
  const row = value as Record<string, unknown>;
  const parameters = record(row.parameters);
  const embedding = record(parameters.embedding_parameters);
  return {
    id: value.id,
    name: value.name,
    displayName: typeof row.display_name === 'string' ? row.display_name : '',
    type: modelType(value),
    source: row.source === 'local' ? 'local' : 'remote',
    provider: typeof parameters.provider === 'string' ? parameters.provider : 'generic',
    baseUrl: typeof parameters.base_url === 'string' ? parameters.base_url : '',
    dimension: typeof embedding.dimension === 'number' ? embedding.dimension : '',
    supportsVision: parameters.supports_vision === true,
    contextWindow: typeof parameters.context_window === 'number' ? parameters.context_window : '',
    maxConcurrency: typeof parameters.max_concurrency === 'number' ? parameters.max_concurrency : '',
    thinkingControl: typeof record(parameters.extra_config).thinking_control === 'string' ? String(record(parameters.extra_config).thinking_control) : '',
    customHeaders: Object.fromEntries(Object.entries(record(parameters.custom_headers)).filter(([, item]) => typeof item === 'string')) as Record<string, string>,
    apiKey: '',
    appSecret: '',
  };
}

export function newModelDraft(): ModelDraft {
  return { name: '', displayName: '', type: 'chat', source: 'remote', provider: 'generic', baseUrl: '', dimension: '', supportsVision: false, contextWindow: '', maxConcurrency: '', thinkingControl: '', customHeaders: {}, apiKey: '', appSecret: '' };
}

export function validateModelDraft(draft: ModelDraft): string[] {
  const errors: string[] = [];
  if (!draft.name.trim()) errors.push('modelNameRequired');
  else if (draft.name.trim().length > 100) errors.push('modelNameMax');
  if (draft.source === 'remote' && draft.provider !== 'weknoracloud') {
    if (!draft.baseUrl.trim()) errors.push('baseUrlRequired');
    else { try { new URL(draft.baseUrl.trim()); } catch { errors.push('baseUrlInvalid'); } }
  }
  if (draft.type === 'embedding' && (typeof draft.dimension !== 'number' || draft.dimension < 128 || draft.dimension > 4096)) errors.push('dimensionInvalid');
  return errors;
}

export function modelPayload(draft: ModelDraft): Record<string, unknown> {
  const parameters: Record<string, unknown> = { base_url: draft.baseUrl.trim(), provider: draft.provider };
  if (draft.type === 'embedding' && typeof draft.dimension === 'number') parameters.embedding_parameters = { dimension: draft.dimension, truncate_prompt_tokens: 0, supports_dimension_override: false };
  if ((draft.type === 'chat' || draft.type === 'vllm') && typeof draft.contextWindow === 'number' && draft.contextWindow >= 1024) parameters.context_window = Math.round(draft.contextWindow);
  if (['chat', 'embedding', 'vllm'].includes(draft.type) && typeof draft.maxConcurrency === 'number' && draft.maxConcurrency > 0) parameters.max_concurrency = draft.maxConcurrency;
  if (draft.type === 'vllm' || (draft.type === 'chat' && draft.supportsVision)) parameters.supports_vision = true;
  if (draft.thinkingControl && draft.type === 'chat' && draft.source === 'remote') parameters.extra_config = { thinking_control: draft.thinkingControl };
  if (Object.keys(draft.customHeaders).length > 0) parameters.custom_headers = draft.customHeaders;
  return { name: draft.name.trim(), display_name: draft.displayName.trim() || draft.name.trim(), description: '', type: backendModelType(draft.type), source: draft.source, parameters };
}

export function modelCredentialInput(draft: ModelDraft): Record<string, string> {
  return Object.fromEntries([['apiKey', draft.apiKey], ['appSecret', draft.appSecret]].filter(([, value]) => value.trim() !== '').map(([key, value]) => [key, value.trim()]));
}
