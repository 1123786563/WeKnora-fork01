import { parseKnowledgeBaseResponse, type KnowledgeBase } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

type JsonRecord = Record<string, unknown>;

export interface ParserEngineRule {
  parser: string;
  fileTypes: string[];
  [key: string]: unknown;
}

export interface ChunkingConfigInput {
  chunk_size?: number;
  chunk_overlap?: number;
  separators?: string[];
  enable_parent_child?: boolean;
  parent_chunk_size?: number;
  child_chunk_size?: number;
  strategy?: string;
  token_limit?: number;
  languages?: string[];
  table_metadata_instructions?: string;
  parser_engine_rules?: ParserEngineRule[];
  [key: string]: unknown;
}

export interface KnowledgeBaseConfigInput extends JsonRecord {
  name?: string;
  description?: string;
  type?: 'document' | 'faq' | string;
  chunking_config?: ChunkingConfigInput;
  image_processing_config?: JsonRecord;
  embedding_model_id?: string;
  summary_model_id?: string;
  vector_store_id?: string;
  vlm_config?: JsonRecord;
  asr_config?: JsonRecord;
  storage_backend_id?: string;
  storage_provider_config?: JsonRecord;
  storage_config?: JsonRecord;
  question_generation_config?: JsonRecord;
  auto_tag_config?: JsonRecord;
  faq_config?: JsonRecord;
  wiki_config?: JsonRecord;
  indexing_strategy?: JsonRecord;
  extract_config?: JsonRecord;
}

export interface KnowledgeBaseConfigUpdateInput extends JsonRecord {
  name?: string;
  description?: string;
  config?: JsonRecord;
}

function record(value: unknown, path: string): JsonRecord {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(`Invalid ${path}: expected object`);
  const prototype = Object.getPrototypeOf(value);
  if (prototype !== Object.prototype && prototype !== null) throw new Error(`Invalid ${path}: expected plain object`);
  return value as JsonRecord;
}

function string(value: unknown, path: string): asserts value is string {
  if (typeof value !== 'string') throw new Error(`Invalid ${path}: expected string`);
}

function nonEmptyString(value: unknown, path: string): asserts value is string {
  string(value, path);
  if (value.trim() === '') throw new Error(`Invalid ${path}: expected non-empty string`);
}

function number(value: unknown, path: string): asserts value is number {
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new Error(`Invalid ${path}: expected finite number`);
}

function boolean(value: unknown, path: string): asserts value is boolean {
  if (typeof value !== 'boolean') throw new Error(`Invalid ${path}: expected boolean`);
}

function stringArray(value: unknown, path: string): asserts value is string[] {
  if (!Array.isArray(value)) throw new Error(`Invalid ${path}: expected array`);
  value.forEach((item, index) => string(item, `${path}[${index}]`));
}

function optional(value: JsonRecord, key: string, validate: (value: unknown, path: string) => void, path: string): void {
  if (value[key] !== undefined) validate(value[key], `${path}.${key}`);
}

function parseParserRules(value: unknown, path: string): void {
  if (!Array.isArray(value)) throw new Error(`Invalid ${path}: expected array`);
  value.forEach((item, index) => {
    const rule = record(item, `${path}[${index}]`);
    nonEmptyString(rule.parser, `${path}[${index}].parser`);
    stringArray(rule.fileTypes, `${path}[${index}].fileTypes`);
  });
}

function validateChunkingConfig(value: unknown, path: string): void {
  const config = record(value, path);
  for (const key of ['chunk_size', 'chunk_overlap', 'parent_chunk_size', 'child_chunk_size', 'token_limit']) {
    optional(config, key, number, path);
  }
  for (const key of ['separators', 'languages']) optional(config, key, stringArray, path);
  for (const key of ['enable_parent_child']) optional(config, key, boolean, path);
  for (const key of ['strategy', 'table_metadata_instructions']) optional(config, key, string, path);
  optional(config, 'parser_engine_rules', parseParserRules, path);
}

function validateVlmConfig(value: unknown, path: string): void {
  const config = record(value, path);
  optional(config, 'enabled', boolean, path);
  for (const key of ['model_id', 'description_language', 'custom_instructions']) optional(config, key, string, path);
}

function validateAsrConfig(value: unknown, path: string): void {
  const config = record(value, path);
  optional(config, 'enabled', boolean, path);
  for (const key of ['model_id', 'language']) optional(config, key, string, path);
}

function validateProviderConfig(value: unknown, path: string): void {
  const config = record(value, path);
  optional(config, 'provider', string, path);
}

function validateQuestionGenerationConfig(value: unknown, path: string): void {
  const config = record(value, path);
  optional(config, 'enabled', boolean, path);
  optional(config, 'question_count', number, path);
  optional(config, 'custom_instructions', string, path);
}

function validateAutoTagConfig(value: unknown, path: string): void {
  const config = record(value, path);
  optional(config, 'enabled', boolean, path);
  optional(config, 'model_id', string, path);
  optional(config, 'max_tags', number, path);
  optional(config, 'skip_if_tagged', boolean, path);
}

function validateFaqConfig(value: unknown, path: string): void {
  const config = record(value, path);
  optional(config, 'index_mode', string, path);
  optional(config, 'question_index_mode', string, path);
}

function validateWikiConfig(value: unknown, path: string): void {
  const config = record(value, path);
  optional(config, 'synthesis_model_id', string, path);
  optional(config, 'max_pages_per_ingest', number, path);
  for (const key of ['extraction_granularity', 'content_instructions', 'extraction_instructions']) optional(config, key, string, path);
}

function validateIndexingStrategy(value: unknown, path: string): void {
  const config = record(value, path);
  for (const key of ['vector_enabled', 'keyword_enabled', 'wiki_enabled', 'graph_enabled']) optional(config, key, boolean, path);
}

function validateExtractConfig(value: unknown, path: string): void {
  const config = record(value, path);
  optional(config, 'enabled', boolean, path);
  optional(config, 'text', string, path);
  for (const key of ['tags', 'nodes', 'relations']) optional(config, key, stringArray, path);
  optional(config, 'custom_instructions', string, path);
}

function validateConfigFields(input: JsonRecord, path: string): void {
  optional(input, 'chunking_config', validateChunkingConfig, path);
  for (const key of ['embedding_model_id', 'summary_model_id', 'vector_store_id', 'storage_backend_id']) optional(input, key, string, path);
  optional(input, 'vlm_config', validateVlmConfig, path);
  optional(input, 'asr_config', validateAsrConfig, path);
  optional(input, 'storage_provider_config', validateProviderConfig, path);
  optional(input, 'storage_config', validateProviderConfig, path);
  optional(input, 'question_generation_config', validateQuestionGenerationConfig, path);
  optional(input, 'auto_tag_config', validateAutoTagConfig, path);
  optional(input, 'faq_config', validateFaqConfig, path);
  optional(input, 'wiki_config', validateWikiConfig, path);
  optional(input, 'indexing_strategy', validateIndexingStrategy, path);
  optional(input, 'extract_config', validateExtractConfig, path);
}

export function parseKnowledgeBaseConfigInput(value: unknown): KnowledgeBaseConfigInput {
  const input = record(value, 'knowledge base config');
  nonEmptyString(input.name, 'name');
  optional(input, 'description', string, 'knowledge base config');
  optional(input, 'type', string, 'knowledge base config');
  validateConfigFields(input, 'knowledge base config');
  return input as KnowledgeBaseConfigInput;
}

export function parseKnowledgeBaseConfigUpdateInput(value: unknown): KnowledgeBaseConfigUpdateInput {
  const input = record(value, 'knowledge base config update');
  optional(input, 'name', nonEmptyString, 'knowledge base config update');
  optional(input, 'description', string, 'knowledge base config update');
  if (input.config !== undefined) {
    const config = record(input.config, 'config');
    validateConfigFields(config, 'config');
  }
  return input as KnowledgeBaseConfigUpdateInput;
}

export interface KnowledgeBaseUpdateInput {
  name: string;
  description?: string;
  config?: KnowledgeBaseConfigInput;
}

export interface ParserEngineInfo {
  Name: string;
  Description: string;
  FileTypes: string[];
  Available?: boolean;
  UnavailableReason?: string;
}

export interface ParserEnginesResult {
  data: ParserEngineInfo[];
  connected?: boolean;
  docreader_addr?: string;
  docreader_transport?: string;
}

export interface ChunkingPreviewInput {
  text: string;
  chunking_config: Record<string, unknown>;
}

export interface ChunkingPreviewResult {
  selected_tier: string;
  tier_chain: string[];
  rejected: unknown[];
  chunks: Array<Record<string, unknown>>;
  stats: Record<string, number>;
  profile?: Record<string, unknown> | null;
}

export interface StorageBackendView {
  id: string;
  name: string;
  provider: string;
  config: Record<string, unknown>;
  source: string;
  status: string;
  [key: string]: unknown;
}

export interface VectorStoreView {
  id: string;
  name: string;
  engine_type: string;
  connection_config: Record<string, unknown>;
  index_config: Record<string, unknown>;
  source: string;
  readonly: boolean;
  [key: string]: unknown;
}

export interface KnowledgeBaseActivityEntry {
  id: number;
  action: string;
  outcome: string;
  created_at: string;
  [key: string]: unknown;
}

export interface KnowledgeBaseActivityResult {
  success: boolean;
  data?: KnowledgeBaseActivityEntry[];
  next_cursor?: number;
}

export interface KnowledgeBaseActivityQuery {
  afterId?: number;
  action?: string;
  outcome?: string;
  limit?: number;
}

function nonNegativeInteger(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new Error(`Invalid ${label}`);
  return value;
}

function envelopeData(value: unknown, label: string): unknown {
  const body = record(value, label);
  if (body.success !== true) throw new Error(`Invalid ${label} response`);
  return body.data;
}

function parseParserEngines(value: unknown): ParserEnginesResult {
  const body = record(value, 'parser engines');
  if (body.code !== 0 || !Array.isArray(body.data)) throw new Error('Invalid parser engines response');
  const data = body.data.map((item, index) => {
    const engine = record(item, `parser engine ${index}`);
    if (typeof engine.Name !== 'string' || engine.Name.trim() === '' || typeof engine.Description !== 'string' || !Array.isArray(engine.FileTypes) || engine.FileTypes.some((type) => typeof type !== 'string') || (engine.Available !== undefined && typeof engine.Available !== 'boolean') || (engine.UnavailableReason !== undefined && typeof engine.UnavailableReason !== 'string')) {
      throw new Error(`Invalid parser engine ${index}`);
    }
    return engine as unknown as ParserEngineInfo;
  });
  if (body.connected !== undefined && typeof body.connected !== 'boolean') throw new Error('Invalid parser engines response');
  if (body.docreader_addr !== undefined && typeof body.docreader_addr !== 'string') throw new Error('Invalid parser engines response');
  if (body.docreader_transport !== undefined && typeof body.docreader_transport !== 'string') throw new Error('Invalid parser engines response');
  return { data, ...(body.connected === undefined ? {} : { connected: body.connected }), ...(body.docreader_addr === undefined ? {} : { docreader_addr: body.docreader_addr }), ...(body.docreader_transport === undefined ? {} : { docreader_transport: body.docreader_transport }) };
}

function parseChunkingPreview(value: unknown): ChunkingPreviewResult {
  // The Go handler (internal/handler/chunker_debug.go) always wraps the
  // PreviewChunkingResponse in a gin.H{"success": true, "data": ...}
  // envelope — the Vue client (frontend/src/api/chunker) reads the payload
  // from `data` the same way. R446 D2: parsing the top level produced
  // "Invalid chunking preview response" against the live backend.
  const envelope = record(value, 'chunking preview');
  if (envelope.success !== true) throw new Error('Invalid chunking preview response');
  const body = record(envelope.data, 'chunking preview');
  if (typeof body.selected_tier !== 'string' || body.selected_tier.trim() === '' || !Array.isArray(body.tier_chain) || body.tier_chain.some((item) => typeof item !== 'string') || !Array.isArray(body.rejected) || !Array.isArray(body.chunks)) throw new Error('Invalid chunking preview response');
  const stats = record(body.stats, 'chunking preview stats');
  if (Object.values(stats).some((item) => typeof item !== 'number' || !Number.isFinite(item))) throw new Error('Invalid chunking preview stats');
  return { selected_tier: body.selected_tier, tier_chain: body.tier_chain as string[], rejected: body.rejected, chunks: body.chunks.map((item) => record(item, 'chunking preview chunk')), stats: stats as Record<string, number>, profile: body.profile === null || body.profile === undefined ? body.profile : record(body.profile, 'chunking preview profile') };
}

function parseStorageBackends(value: unknown): { data: StorageBackendView[]; default_storage_backend_id?: string | null } {
  const body = record(value, 'storage backends');
  if (body.success !== true || !Array.isArray(body.data)) throw new Error('Invalid storage backends response');
  const data = body.data.map((item, index) => {
    const backend = record(item, `storage backend ${index}`);
    for (const key of ['id', 'name', 'provider', 'source', 'status']) if (typeof backend[key] !== 'string' || backend[key] === '') throw new Error(`Invalid storage backend ${index}`);
    if (typeof backend.config !== 'object' || backend.config === null || Array.isArray(backend.config)) throw new Error(`Invalid storage backend config ${index}`);
    return backend as unknown as StorageBackendView;
  });
  if (body.default_storage_backend_id !== undefined && body.default_storage_backend_id !== null && (typeof body.default_storage_backend_id !== 'string' || body.default_storage_backend_id.trim() === '')) throw new Error('Invalid default storage backend');
  return { data, ...(body.default_storage_backend_id === undefined ? {} : { default_storage_backend_id: body.default_storage_backend_id as string | null }) };
}

function parseVectorStores(value: unknown): { data: VectorStoreView[] } {
  const body = record(value, 'vector stores');
  if (body.success !== true || !Array.isArray(body.data)) throw new Error('Invalid vector stores response');
  return { data: body.data.map((item, index) => {
    const store = record(item, `vector store ${index}`);
    for (const key of ['id', 'name', 'engine_type', 'source']) if (typeof store[key] !== 'string' || store[key] === '') throw new Error(`Invalid vector store ${index}`);
    if (typeof store.readonly !== 'boolean' || typeof store.connection_config !== 'object' || store.connection_config === null || Array.isArray(store.connection_config) || typeof store.index_config !== 'object' || store.index_config === null || Array.isArray(store.index_config)) throw new Error(`Invalid vector store ${index}`);
    return store as unknown as VectorStoreView;
  }) };
}

function parseActivity(value: unknown): KnowledgeBaseActivityResult {
  const body = record(value, 'knowledge base activity');
  if (body.success !== true || !Array.isArray(body.data)) throw new Error('Invalid knowledge base activity response');
  const data = body.data.map((item, index) => {
    const entry = record(item, `knowledge base activity ${index}`);
    if (typeof entry.action !== 'string' || typeof entry.outcome !== 'string' || typeof entry.created_at !== 'string') throw new Error(`Invalid knowledge base activity ${index}`);
    nonNegativeInteger(entry.id, `knowledge base activity ${index}`);
    return entry as unknown as KnowledgeBaseActivityEntry;
  });
  return { success: true, data, ...(body.next_cursor === undefined ? {} : { next_cursor: nonNegativeInteger(body.next_cursor, 'knowledge base activity cursor') }) };
}

export function createKnowledgeSettingsApi(request: (input: ClientRequest) => Promise<unknown>) {
  const kbPath = (id: string) => `/api/v1/knowledge-bases/${encodeURIComponent(id)}`;
  return {
    async get(id: string): Promise<KnowledgeBase> {
      return parseKnowledgeBaseResponse(await request({ method: 'GET', path: kbPath(id) }));
    },
    async update(id: string, input: KnowledgeBaseUpdateInput): Promise<KnowledgeBase> {
      return parseKnowledgeBaseResponse(await request({ method: 'PUT', path: kbPath(id), body: input }));
    },
    async parserEngines(): Promise<ParserEnginesResult> {
      return parseParserEngines(await request({ method: 'GET', path: '/api/v1/system/parser-engines' }));
    },
    async previewChunking(input: ChunkingPreviewInput): Promise<ChunkingPreviewResult> {
      return parseChunkingPreview(await request({ method: 'POST', path: '/api/v1/chunker/preview', body: input }));
    },
    async storageBackends(): Promise<{ data: StorageBackendView[]; default_storage_backend_id?: string | null }> {
      return parseStorageBackends(await request({ method: 'GET', path: '/api/v1/storage-backends' }));
    },
    async vectorStores(): Promise<{ data: VectorStoreView[] }> {
      return parseVectorStores(await request({ method: 'GET', path: '/api/v1/vector-stores' }));
    },
    async activity(id: string, options?: number | KnowledgeBaseActivityQuery): Promise<KnowledgeBaseActivityResult> {
      const queryOptions = typeof options === 'number' ? { afterId: options } : (options ?? {});
      const params = new URLSearchParams();
      if (queryOptions.afterId !== undefined) params.set('after_id', String(queryOptions.afterId));
      if (queryOptions.action) params.set('action', queryOptions.action);
      if (queryOptions.outcome) params.set('outcome', queryOptions.outcome);
      if (queryOptions.limit !== undefined) params.set('limit', String(queryOptions.limit));
      const encodedQuery = params.toString();
      const query = encodedQuery ? `?${encodedQuery}` : '';
      return parseActivity(await request({ method: 'GET', path: `${kbPath(id)}/activity${query}` }));
    },
  };
}
