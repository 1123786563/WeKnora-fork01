export type KnowledgeBaseType = 'document' | 'faq';

export type WikiExtractionGranularity = 'focused' | 'standard' | 'exhaustive';

export interface ParserEngineRule {
  parser: string;
  fileTypes: string[];
}

export interface GraphNode {
  name: string;
  attributes: string[];
}

export interface GraphRelation {
  node1: string;
  type: string;
  node2: string;
}

export interface KnowledgeSettingsFormState {
  type: KnowledgeBaseType;
  name: string;
  description: string;
  model: {
    embeddingModelId: string;
    llmModelId: string;
    wikiSynthesisModelId: string;
  };
  processing: {
    chunkSize: number;
    chunkOverlap: number;
    separators: string[];
    enableParentChild: boolean;
    parentChunkSize: number;
    childChunkSize: number;
    strategy: string;
    tokenLimit: number;
    languages: string[];
    tableMetadataInstructions: string;
    parserEngineRules?: ParserEngineRule[];
    multimodal: {
      enabled: boolean;
      vllmModelId: string;
      descriptionLanguage: string;
      customInstructions: string;
    };
    asr: {
      enabled: boolean;
      modelId: string;
      language: string;
    };
    questionGeneration: {
      enabled: boolean;
      questionCount: number;
      customInstructions: string;
    };
    autoTag: {
      enabled: boolean;
      modelId: string;
      maxTags: number;
      skipIfTagged: boolean;
    };
  };
  faq: {
    indexMode: string;
    questionIndexMode: string;
  };
  wiki: {
    maxPagesPerIngest: number;
    extractionGranularity: WikiExtractionGranularity;
    contentInstructions: string;
    extractionInstructions: string;
  };
  indexing: {
    vectorEnabled: boolean;
    keywordEnabled: boolean;
    wikiEnabled: boolean;
    graphEnabled: boolean;
  };
  graph: {
    enabled: boolean;
    text: string;
    tags: string[];
    nodes: GraphNode[];
    relations: GraphRelation[];
    customInstructions: string;
  };
  storage: {
    backendId: string;
    provider: string;
  };
  vectorStore: {
    id: string;
  };
}

export interface KnowledgeSettingsValidationError {
  field: string;
  section: 'basic' | 'models' | 'multimodal' | 'asr' | 'faq';
  message: string;
}

const defaultSeparators = ['\n\n', '\n', '。', '！', '？', ';', '；'];

function clone<T>(value: T): T {
  return structuredClone(value);
}

export function defaultKnowledgeSettingsFormState(type: KnowledgeBaseType = 'document'): KnowledgeSettingsFormState {
  return {
    type,
    name: '',
    description: '',
    model: { embeddingModelId: '', llmModelId: '', wikiSynthesisModelId: '' },
    processing: {
      chunkSize: 512,
      chunkOverlap: 80,
      separators: [...defaultSeparators],
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      strategy: 'auto',
      tokenLimit: 0,
      languages: [],
      tableMetadataInstructions: '',
      parserEngineRules: undefined,
      multimodal: { enabled: false, vllmModelId: '', descriptionLanguage: '', customInstructions: '' },
      asr: { enabled: false, modelId: '', language: '' },
      questionGeneration: { enabled: true, questionCount: 3, customInstructions: '' },
      autoTag: { enabled: false, modelId: '', maxTags: 3, skipIfTagged: true },
    },
    faq: { indexMode: 'question_only', questionIndexMode: 'separate' },
    wiki: {
      maxPagesPerIngest: 0,
      extractionGranularity: 'standard',
      contentInstructions: '',
      extractionInstructions: '',
    },
    indexing: { vectorEnabled: true, keywordEnabled: true, wikiEnabled: false, graphEnabled: false },
    graph: { enabled: false, text: '', tags: [], nodes: [], relations: [], customInstructions: '' },
    storage: { backendId: '', provider: 'local' },
    vectorStore: { id: '' },
  };
}

function stringValue(value: unknown, fallback: string): string {
  return typeof value === 'string' ? value : fallback;
}

function numberValue(value: unknown, fallback: number): number {
  return typeof value === 'number' ? value : fallback;
}

function booleanValue(value: unknown, fallback: boolean): boolean {
  return typeof value === 'boolean' ? value : fallback;
}

function stringArray(value: unknown, fallback: string[]): string[] {
  return Array.isArray(value) && value.every((item) => typeof item === 'string') ? [...value] : [...fallback];
}

export function normalizeKnowledgeSettingsFormState(input: Partial<KnowledgeSettingsFormState> & Record<string, unknown>): KnowledgeSettingsFormState {
  const type = input.type === 'faq' ? 'faq' : 'document';
  const defaults = defaultKnowledgeSettingsFormState(type);
  const source = input as Record<string, any>;
  const model = source.model ?? source.modelConfig ?? {};
  const processing = source.processing ?? source.chunkingConfig ?? {};
  const multimodal = processing.multimodal ?? source.multimodalConfig ?? source.vlm_config ?? {};
  const asr = processing.asr ?? source.asrConfig ?? source.asr_config ?? {};
  const questionGeneration = processing.questionGeneration ?? source.questionGenerationConfig ?? {};
  const autoTag = processing.autoTag ?? source.autoTagConfig ?? {};
  const wiki = source.wiki ?? source.wikiConfig ?? {};
  const indexing = source.indexing ?? source.indexingStrategy ?? {};
  const graph = source.graph ?? source.nodeExtractConfig ?? {};
  const storage = source.storage ?? {};
  const vectorStore = source.vectorStore ?? {};

  return {
    ...defaults,
    name: stringValue(source.name, defaults.name),
    description: stringValue(source.description, defaults.description),
    model: {
      embeddingModelId: stringValue(model.embeddingModelId, stringValue(source.embedding_model_id, defaults.model.embeddingModelId)),
      llmModelId: stringValue(model.llmModelId, stringValue(source.summary_model_id, defaults.model.llmModelId)),
      wikiSynthesisModelId: stringValue(model.wikiSynthesisModelId, stringValue(source.wiki_synthesis_model_id, defaults.model.wikiSynthesisModelId)),
    },
    processing: {
      ...defaults.processing,
      chunkSize: numberValue(processing.chunkSize ?? source.chunk_size, defaults.processing.chunkSize),
      chunkOverlap: numberValue(processing.chunkOverlap ?? source.chunk_overlap, defaults.processing.chunkOverlap),
      separators: stringArray(processing.separators, defaults.processing.separators),
      enableParentChild: booleanValue(processing.enableParentChild, defaults.processing.enableParentChild),
      parentChunkSize: numberValue(processing.parentChunkSize, defaults.processing.parentChunkSize),
      childChunkSize: numberValue(processing.childChunkSize, defaults.processing.childChunkSize),
      strategy: stringValue(processing.strategy, defaults.processing.strategy),
      tokenLimit: numberValue(processing.tokenLimit, defaults.processing.tokenLimit),
      languages: stringArray(processing.languages, defaults.processing.languages),
      tableMetadataInstructions: stringValue(processing.tableMetadataInstructions, defaults.processing.tableMetadataInstructions),
      parserEngineRules: Array.isArray(processing.parserEngineRules) ? clone(processing.parserEngineRules) : undefined,
      multimodal: {
        enabled: booleanValue(multimodal.enabled, defaults.processing.multimodal.enabled),
        vllmModelId: stringValue(multimodal.vllmModelId ?? multimodal.model_id, defaults.processing.multimodal.vllmModelId),
        descriptionLanguage: stringValue(multimodal.descriptionLanguage ?? multimodal.description_language, defaults.processing.multimodal.descriptionLanguage),
        customInstructions: stringValue(multimodal.customInstructions ?? multimodal.custom_instructions, defaults.processing.multimodal.customInstructions),
      },
      asr: {
        enabled: booleanValue(asr.enabled, defaults.processing.asr.enabled),
        modelId: stringValue(asr.modelId ?? asr.model_id, defaults.processing.asr.modelId),
        language: stringValue(asr.language, defaults.processing.asr.language),
      },
      questionGeneration: {
        enabled: booleanValue(questionGeneration.enabled, defaults.processing.questionGeneration.enabled),
        questionCount: numberValue(questionGeneration.questionCount, defaults.processing.questionGeneration.questionCount),
        customInstructions: stringValue(questionGeneration.customInstructions, defaults.processing.questionGeneration.customInstructions),
      },
      autoTag: {
        enabled: booleanValue(autoTag.enabled, defaults.processing.autoTag.enabled),
        modelId: stringValue(autoTag.modelId, defaults.processing.autoTag.modelId),
        maxTags: numberValue(autoTag.maxTags, defaults.processing.autoTag.maxTags),
        skipIfTagged: booleanValue(autoTag.skipIfTagged, defaults.processing.autoTag.skipIfTagged),
      },
    },
    faq: {
      indexMode: stringValue(source.faq?.indexMode ?? source.faq_config?.index_mode, defaults.faq.indexMode),
      questionIndexMode: stringValue(source.faq?.questionIndexMode ?? source.faq_config?.question_index_mode, defaults.faq.questionIndexMode),
    },
    wiki: {
      maxPagesPerIngest: numberValue(wiki.maxPagesPerIngest ?? source.wiki_config?.max_pages_per_ingest, defaults.wiki.maxPagesPerIngest),
      extractionGranularity: ['focused', 'standard', 'exhaustive'].includes(wiki.extractionGranularity ?? source.wiki_config?.extraction_granularity)
        ? (wiki.extractionGranularity ?? source.wiki_config.extraction_granularity)
        : defaults.wiki.extractionGranularity,
      contentInstructions: stringValue(wiki.contentInstructions ?? source.wiki_config?.content_instructions, defaults.wiki.contentInstructions),
      extractionInstructions: stringValue(wiki.extractionInstructions ?? source.wiki_config?.extraction_instructions, defaults.wiki.extractionInstructions),
    },
    indexing: {
      vectorEnabled: booleanValue(indexing.vectorEnabled ?? source.indexing_strategy?.vector_enabled, defaults.indexing.vectorEnabled),
      keywordEnabled: booleanValue(indexing.keywordEnabled ?? source.indexing_strategy?.keyword_enabled, defaults.indexing.keywordEnabled),
      wikiEnabled: booleanValue(indexing.wikiEnabled ?? source.indexing_strategy?.wiki_enabled, defaults.indexing.wikiEnabled),
      graphEnabled: booleanValue(indexing.graphEnabled ?? source.indexing_strategy?.graph_enabled, defaults.indexing.graphEnabled),
    },
    graph: {
      enabled: booleanValue(graph.enabled ?? source.extract_config?.enabled, defaults.graph.enabled),
      text: stringValue(graph.text ?? source.extract_config?.text, defaults.graph.text),
      tags: stringArray(graph.tags ?? source.extract_config?.tags, defaults.graph.tags),
      nodes: Array.isArray(graph.nodes ?? source.extract_config?.nodes) ? clone(graph.nodes ?? source.extract_config.nodes) : [],
      relations: Array.isArray(graph.relations ?? source.extract_config?.relations) ? clone(graph.relations ?? source.extract_config.relations) : [],
      customInstructions: stringValue(graph.customInstructions ?? source.extract_config?.custom_instructions, defaults.graph.customInstructions),
    },
    storage: {
      backendId: stringValue(storage.backendId ?? source.storage_backend_id, defaults.storage.backendId),
      provider: stringValue(storage.provider ?? source.storage_provider_config?.provider ?? source.storage_config?.provider, defaults.storage.provider),
    },
    vectorStore: { id: stringValue(vectorStore.id ?? source.vector_store_id, defaults.vectorStore.id) },
  };
}

export function validateKnowledgeSettingsFormState(state: KnowledgeSettingsFormState): KnowledgeSettingsValidationError[] {
  const errors: KnowledgeSettingsValidationError[] = [];
  if (!state.name.trim()) errors.push({ field: 'name', section: 'basic', message: 'Knowledge-base name is required' });
  if (state.type === 'faq' && !state.faq.indexMode) errors.push({ field: 'index_mode', section: 'faq', message: 'FAQ index mode is required' });
  if (state.type !== 'faq' && !state.indexing.vectorEnabled && !state.indexing.keywordEnabled && !state.indexing.wikiEnabled && !state.indexing.graphEnabled) {
    errors.push({ field: 'indexing', section: 'basic', message: 'At least one indexing strategy is required' });
  }
  if (state.indexing.vectorEnabled && !state.model.embeddingModelId) errors.push({ field: 'embedding_model_id', section: 'models', message: 'Embedding model is required' });
  if (!state.model.llmModelId) errors.push({ field: 'summary_model_id', section: 'models', message: 'Summary model is required' });
  if (state.processing.multimodal.enabled && !state.processing.multimodal.vllmModelId) errors.push({ field: 'vlm_model_id', section: 'multimodal', message: 'Multimodal model is required' });
  if (state.processing.asr.enabled && !state.processing.asr.modelId) errors.push({ field: 'asr_model_id', section: 'asr', message: 'ASR model is required' });
  return errors;
}

type ChunkingPayload = {
  chunk_size: number;
  chunk_overlap: number;
  separators: string[];
  enable_parent_child: boolean;
  parent_chunk_size: number;
  child_chunk_size: number;
  strategy: string;
  token_limit: number;
  languages: string[];
  table_metadata_instructions: string;
  parser_engine_rules?: ParserEngineRule[];
};

function chunkingPayload(state: KnowledgeSettingsFormState): ChunkingPayload {
  const chunking: ChunkingPayload = {
    chunk_size: state.processing.chunkSize,
    chunk_overlap: state.processing.chunkOverlap,
    separators: [...state.processing.separators],
    enable_parent_child: state.processing.enableParentChild,
    parent_chunk_size: state.processing.parentChunkSize,
    child_chunk_size: state.processing.childChunkSize,
    strategy: state.processing.strategy,
    token_limit: state.processing.tokenLimit,
    languages: [...state.processing.languages],
    table_metadata_instructions: state.processing.tableMetadataInstructions,
  };
  if (state.processing.parserEngineRules?.length) chunking.parser_engine_rules = clone(state.processing.parserEngineRules);
  return chunking;
}

function vlmPayload(state: KnowledgeSettingsFormState) {
  return {
    enabled: state.processing.multimodal.enabled,
    model_id: state.processing.multimodal.enabled ? state.processing.multimodal.vllmModelId : '',
    description_language: state.processing.multimodal.descriptionLanguage,
    custom_instructions: state.processing.multimodal.customInstructions,
  };
}

function asrPayload(state: KnowledgeSettingsFormState) {
  return {
    enabled: state.processing.asr.enabled,
    model_id: state.processing.asr.enabled ? state.processing.asr.modelId : '',
    language: state.processing.asr.language,
  };
}

function wikiPayload(state: KnowledgeSettingsFormState) {
  return {
    synthesis_model_id: state.model.wikiSynthesisModelId,
    max_pages_per_ingest: state.wiki.maxPagesPerIngest,
    extraction_granularity: state.wiki.extractionGranularity,
    content_instructions: state.wiki.contentInstructions,
    extraction_instructions: state.wiki.extractionInstructions,
  };
}

function indexingPayload(state: KnowledgeSettingsFormState) {
  return {
    vector_enabled: state.indexing.vectorEnabled,
    keyword_enabled: state.indexing.keywordEnabled,
    wiki_enabled: state.indexing.wikiEnabled,
    graph_enabled: state.indexing.graphEnabled,
  };
}

function extractPayload(state: KnowledgeSettingsFormState) {
  return {
    enabled: state.graph.enabled,
    text: state.graph.text,
    tags: [...state.graph.tags],
    nodes: clone(state.graph.nodes),
    relations: clone(state.graph.relations),
    custom_instructions: state.graph.customInstructions,
  };
}

function commonPayload(state: KnowledgeSettingsFormState) {
  const payload: Record<string, unknown> = {
    name: state.name,
    description: state.description,
    type: state.type,
    chunking_config: chunkingPayload(state),
    embedding_model_id: state.model.embeddingModelId,
    summary_model_id: state.model.llmModelId,
    vlm_config: vlmPayload(state),
    asr_config: asrPayload(state),
    storage_provider_config: { provider: state.storage.provider },
    storage_config: { provider: state.storage.provider },
    question_generation_config: {
      enabled: state.processing.questionGeneration.enabled,
      question_count: state.processing.questionGeneration.questionCount,
      custom_instructions: state.processing.questionGeneration.customInstructions,
    },
    auto_tag_config: {
      enabled: state.processing.autoTag.enabled,
      model_id: state.processing.autoTag.modelId,
      max_tags: state.processing.autoTag.maxTags,
      skip_if_tagged: state.processing.autoTag.skipIfTagged,
    },
    extract_config: extractPayload(state),
  };
  if (state.vectorStore.id) payload.vector_store_id = state.vectorStore.id;
  if (state.storage.backendId) payload.storage_backend_id = state.storage.backendId;
  if (state.type === 'faq') {
    payload.faq_config = { index_mode: state.faq.indexMode, question_index_mode: state.faq.questionIndexMode };
  } else {
    payload.wiki_config = wikiPayload(state);
    payload.indexing_strategy = indexingPayload(state);
  }
  return payload;
}

export function buildKnowledgeSettingsPayload(state: KnowledgeSettingsFormState, mode: 'create' | 'update'): Record<string, any> {
  const payload = commonPayload(state);
  if (mode === 'create') return payload;

  const baseConfig: Record<string, unknown> = {
    wiki_config: wikiPayload(state),
    auto_tag_config: payload.auto_tag_config,
    indexing_strategy: indexingPayload(state),
  };
  if (state.type === 'faq') {
    delete baseConfig.wiki_config;
    delete baseConfig.auto_tag_config;
    delete baseConfig.indexing_strategy;
    baseConfig.faq_config = payload.faq_config;
  }

  return {
    base: { name: state.name, description: state.description, config: baseConfig },
    config: {
      chunking_config: payload.chunking_config,
      embedding_model_id: payload.embedding_model_id,
      summary_model_id: payload.summary_model_id,
      vlm_config: payload.vlm_config,
      asr_config: payload.asr_config,
      storage_provider: state.storage.provider,
      storage_provider_config: payload.storage_provider_config,
      storage_config: payload.storage_config,
      question_generation_config: payload.question_generation_config,
      extract_config: payload.extract_config,
    },
  };
}

// Short aliases keep the model convenient for consumers that call it a form.
export const defaultSettingsFormState = defaultKnowledgeSettingsFormState;
export const normalizeSettingsFormState = normalizeKnowledgeSettingsFormState;
export const validateSettingsFormState = validateKnowledgeSettingsFormState;
export const buildSettingsPayload = buildKnowledgeSettingsPayload;

export const defaultKnowledgeBaseEditorState = defaultKnowledgeSettingsFormState;
export const normalizeKnowledgeBaseEditorState = normalizeKnowledgeSettingsFormState;
export const buildKnowledgeBasePayload = buildKnowledgeSettingsPayload;
export type KnowledgeBaseEditorState = KnowledgeSettingsFormState;
