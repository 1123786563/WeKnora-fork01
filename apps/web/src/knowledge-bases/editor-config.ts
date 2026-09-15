import './editor-layout.ts';

export type KnowledgeEditorConfig = {
  faqConfig: { indexMode: 'question_only' | 'question_answer'; questionIndexMode: 'combined' | 'separate' };
  indexingStrategy: { vectorEnabled: boolean; keywordEnabled: boolean; wikiEnabled: boolean; graphEnabled: boolean };
  chunkingConfig: {
    chunkSize: number; chunkOverlap: number; strategy: string; separators: string[];
    enableParentChild: boolean; parentChunkSize: number; childChunkSize: number;
    tokenLimit: number; languages: string[]; tableMetadataInstructions: string;
    parserEngineRules: Array<{ file_types: string[]; engine: string; xlsx_first_row_as_header?: boolean }>;
  };
  storageBackendId: string;
  /** Legacy Vue compatibility projection used by rolling backend upgrades. */
  storageProvider: string;
  vectorStoreId: string;
  multimodalConfig: { enabled: boolean; vllmModelId: string; descriptionLanguage: string; customInstructions: string };
  asrConfig: { enabled: boolean; modelId: string; language: string };
  nodeExtractConfig: { enabled: boolean; text: string; tags: string[]; nodes: unknown[]; relations: unknown[]; customInstructions: string };
  questionGenerationConfig: { enabled: boolean; questionCount: number; customInstructions: string };
  autoTagConfig: { enabled: boolean; modelId: string; maxTags: number; skipIfTagged: boolean };
  wikiConfig: { synthesisModelId: string; maxPagesPerIngest: number; extractionGranularity: 'focused' | 'standard' | 'exhaustive'; contentInstructions: string; extractionInstructions: string };
};

const record = (value: unknown): Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : {};
const stringValue = (value: unknown, fallback = '') => typeof value === 'string' ? value : fallback;
const numberValue = (value: unknown, fallback: number) => typeof value === 'number' && Number.isFinite(value) ? value : fallback;
const boolValue = (value: unknown, fallback: boolean) => typeof value === 'boolean' ? value : fallback;

export function defaultKnowledgeEditorConfig(): KnowledgeEditorConfig {
  return {
    faqConfig: { indexMode: 'question_only', questionIndexMode: 'separate' },
    indexingStrategy: { vectorEnabled: true, keywordEnabled: true, wikiEnabled: false, graphEnabled: false },
    chunkingConfig: { chunkSize: 512, chunkOverlap: 80, strategy: 'auto', separators: ['\n\n', '\n', '。', '！', '？', ';', '；'], enableParentChild: true, parentChunkSize: 4096, childChunkSize: 384, tokenLimit: 0, languages: [], tableMetadataInstructions: '', parserEngineRules: [] },
    storageBackendId: '', storageProvider: '', vectorStoreId: '',
    multimodalConfig: { enabled: false, vllmModelId: '', descriptionLanguage: '', customInstructions: '' },
    asrConfig: { enabled: false, modelId: '', language: '' },
    nodeExtractConfig: { enabled: false, text: '', tags: [], nodes: [], relations: [], customInstructions: '' },
    questionGenerationConfig: { enabled: true, questionCount: 3, customInstructions: '' },
    autoTagConfig: { enabled: false, modelId: '', maxTags: 3, skipIfTagged: true },
    wikiConfig: { synthesisModelId: '', maxPagesPerIngest: 0, extractionGranularity: 'standard', contentInstructions: '', extractionInstructions: '' },
  };
}

export function hydrateKnowledgeEditorConfig(source: Record<string, unknown>): KnowledgeEditorConfig {
  const defaults = defaultKnowledgeEditorConfig();
  const chunk = record(source.chunking_config);
  const faq = record(source.faq_config);
  const indexing = record(source.indexing_strategy);
  const image = record(source.vlm_config ?? source.image_processing_config);
  const asr = record(source.asr_config);
  const extract = record(source.extract_config);
  const questions = record(source.question_generation_config);
  const autoTag = record(source.auto_tag_config);
  const wiki = record(source.wiki_config);
  const rules = Array.isArray(chunk.parser_engine_rules) ? chunk.parser_engine_rules.flatMap((item) => {
    const row = record(item);
    return typeof row.engine === 'string' && Array.isArray(row.file_types) ? [{ file_types: row.file_types.filter((v): v is string => typeof v === 'string'), engine: row.engine, ...(typeof row.xlsx_first_row_as_header === 'boolean' ? { xlsx_first_row_as_header: row.xlsx_first_row_as_header } : {}) }] : [];
  }) : defaults.chunkingConfig.parserEngineRules;
  return {
    faqConfig: { indexMode: faq.index_mode === 'question_answer' ? 'question_answer' : defaults.faqConfig.indexMode, questionIndexMode: faq.question_index_mode === 'combined' ? 'combined' : defaults.faqConfig.questionIndexMode },
    indexingStrategy: {
      vectorEnabled: boolValue(indexing.vector_enabled, defaults.indexingStrategy.vectorEnabled),
      keywordEnabled: boolValue(indexing.keyword_enabled, defaults.indexingStrategy.keywordEnabled),
      wikiEnabled: boolValue(indexing.wiki_enabled, defaults.indexingStrategy.wikiEnabled),
      graphEnabled: boolValue(indexing.graph_enabled, defaults.indexingStrategy.graphEnabled),
    },
    // Vue initializes create-mode values separately. Existing KBs are hydrated
    // with its edit-mode fallbacks: an absent strategy stays legacy-empty,
    // parent/child chunking is off, and question generation is off.
    chunkingConfig: { chunkSize: numberValue(chunk.chunk_size, defaults.chunkingConfig.chunkSize) || defaults.chunkingConfig.chunkSize, chunkOverlap: numberValue(chunk.chunk_overlap, defaults.chunkingConfig.chunkOverlap) || defaults.chunkingConfig.chunkOverlap, strategy: stringValue(chunk.strategy), separators: Array.isArray(chunk.separators) ? chunk.separators.filter((v): v is string => typeof v === 'string') : defaults.chunkingConfig.separators, enableParentChild: boolValue(chunk.enable_parent_child, false), parentChunkSize: numberValue(chunk.parent_chunk_size, defaults.chunkingConfig.parentChunkSize) || defaults.chunkingConfig.parentChunkSize, childChunkSize: numberValue(chunk.child_chunk_size, defaults.chunkingConfig.childChunkSize) || defaults.chunkingConfig.childChunkSize, tokenLimit: numberValue(chunk.token_limit, defaults.chunkingConfig.tokenLimit), languages: Array.isArray(chunk.languages) ? chunk.languages.filter((v): v is string => typeof v === 'string') : [], tableMetadataInstructions: stringValue(chunk.table_metadata_instructions), parserEngineRules: rules },
    storageBackendId: stringValue(source.storage_backend_id),
    storageProvider: stringValue(record(source.storage_provider_config).provider, stringValue(record(source.storage_config).provider, 'local')),
    vectorStoreId: stringValue(source.vector_store_id),
    multimodalConfig: { enabled: boolValue(image.enabled, false), vllmModelId: stringValue(image.model_id), descriptionLanguage: stringValue(image.description_language), customInstructions: stringValue(image.custom_instructions) },
    asrConfig: { enabled: boolValue(asr.enabled, false), modelId: stringValue(asr.model_id), language: stringValue(asr.language) },
    nodeExtractConfig: { enabled: boolValue(extract.enabled, false), text: stringValue(extract.text), tags: Array.isArray(extract.tags) ? extract.tags.filter((v): v is string => typeof v === 'string') : [], nodes: Array.isArray(extract.nodes) ? extract.nodes : [], relations: Array.isArray(extract.relations) ? extract.relations : [], customInstructions: stringValue(extract.custom_instructions) },
    questionGenerationConfig: { enabled: boolValue(questions.enabled, false), questionCount: numberValue(questions.question_count, defaults.questionGenerationConfig.questionCount) || defaults.questionGenerationConfig.questionCount, customInstructions: stringValue(questions.custom_instructions) },
    autoTagConfig: { enabled: boolValue(autoTag.enabled, false), modelId: stringValue(autoTag.model_id), maxTags: numberValue(autoTag.max_tags, 3) || 3, skipIfTagged: boolValue(autoTag.skip_if_tagged, true) },
    wikiConfig: { synthesisModelId: stringValue(wiki.synthesis_model_id), maxPagesPerIngest: numberValue(wiki.max_pages_per_ingest, 0), extractionGranularity: wiki.extraction_granularity === 'focused' || wiki.extraction_granularity === 'exhaustive' ? wiki.extraction_granularity : 'standard', contentInstructions: stringValue(wiki.content_instructions), extractionInstructions: stringValue(wiki.extraction_instructions) },
  };
}

export function knowledgeEditorConfigPayload(config: KnowledgeEditorConfig, type: 'document' | 'faq'): Record<string, unknown> {
  const chunk = config.chunkingConfig;
  const payload: Record<string, unknown> = {
    chunking_config: { chunk_size: Math.max(1, Math.trunc(chunk.chunkSize)), chunk_overlap: Math.max(0, Math.trunc(chunk.chunkOverlap)), strategy: chunk.strategy, separators: chunk.separators, enable_parent_child: chunk.enableParentChild, parent_chunk_size: Math.max(1, Math.trunc(chunk.parentChunkSize)), child_chunk_size: Math.max(1, Math.trunc(chunk.childChunkSize)), token_limit: Math.max(0, Math.trunc(chunk.tokenLimit)), languages: chunk.languages, table_metadata_instructions: chunk.tableMetadataInstructions, ...(chunk.parserEngineRules.length ? { parser_engine_rules: chunk.parserEngineRules } : {}) },
    vlm_config: { enabled: config.multimodalConfig.enabled, model_id: config.multimodalConfig.enabled ? config.multimodalConfig.vllmModelId : '', description_language: config.multimodalConfig.descriptionLanguage, custom_instructions: config.multimodalConfig.customInstructions },
    asr_config: { enabled: config.asrConfig.enabled, model_id: config.asrConfig.enabled ? config.asrConfig.modelId : '', language: config.asrConfig.language },
    // Vue sends this block for both enabled and disabled states. Only the
    // numeric control falls back when the input is cleared or zero.
    question_generation_config: {
      enabled: config.questionGenerationConfig.enabled,
      question_count: config.questionGenerationConfig.questionCount || 3,
      custom_instructions: config.questionGenerationConfig.customInstructions,
    },
    auto_tag_config: { enabled: config.autoTagConfig.enabled, model_id: config.autoTagConfig.modelId, max_tags: config.autoTagConfig.maxTags || 3, skip_if_tagged: config.autoTagConfig.skipIfTagged },
    extract_config: { enabled: config.nodeExtractConfig.enabled, text: config.nodeExtractConfig.text, tags: config.nodeExtractConfig.tags, nodes: config.nodeExtractConfig.nodes, relations: config.nodeExtractConfig.relations, custom_instructions: config.nodeExtractConfig.customInstructions },
    ...(config.storageBackendId ? { storage_backend_id: config.storageBackendId } : {}),
    storage_provider_config: { provider: config.storageProvider || 'local' },
    storage_config: { provider: config.storageProvider || 'local' },
    ...(config.vectorStoreId ? { vector_store_id: config.vectorStoreId } : {}),
  };
  if (type === 'faq') payload.faq_config = { index_mode: config.faqConfig.indexMode, question_index_mode: config.faqConfig.questionIndexMode };
  else {
    payload.indexing_strategy = { vector_enabled: config.indexingStrategy.vectorEnabled, keyword_enabled: config.indexingStrategy.keywordEnabled, wiki_enabled: config.indexingStrategy.wikiEnabled, graph_enabled: config.indexingStrategy.graphEnabled };
    payload.wiki_config = { synthesis_model_id: config.wikiConfig.synthesisModelId, max_pages_per_ingest: config.wikiConfig.maxPagesPerIngest, extraction_granularity: config.wikiConfig.extractionGranularity, content_instructions: config.wikiConfig.contentInstructions, extraction_instructions: config.wikiConfig.extractionInstructions };
  }
  return payload;
}
