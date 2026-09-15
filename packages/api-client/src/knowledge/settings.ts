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
  name: string;
  description?: string;
  type?: 'document' | 'faq' | string;
  chunking_config?: ChunkingConfigInput;
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
