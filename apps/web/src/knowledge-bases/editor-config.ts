export type KnowledgeBaseType = 'document' | 'faq'
export type ExtractionGranularity = 'focused' | 'standard' | 'exhaustive'

export interface ParserEngineRule {
  parser: string
  fileTypes: string[]
  [key: string]: unknown
}

export interface KnowledgeBaseEditorConfig {
  type: KnowledgeBaseType
  name: string
  description: string
  model: {
    embeddingModelId: string
    llmModelId: string
    wikiSynthesisModelId: string
  }
  faq: {
    indexMode: string
    questionIndexMode: string
  }
  chunking: {
    chunkSize: number
    chunkOverlap: number
    separators: string[]
    parserEngineRules?: ParserEngineRule[]
    enableParentChild: boolean
    parentChunkSize: number
    childChunkSize: number
    strategy: string
    tokenLimit: number
    languages: string[]
    tableMetadataInstructions: string
  }
  processing: {
    languages: string[]
    multimodal: {
      enabled: boolean
      modelId: string
      descriptionLanguage: string
      customInstructions: string
    }
    asr: {
      enabled: boolean
      modelId: string
      language: string
    }
    graph: {
      enabled: boolean
      text: string
      tags: string[]
      nodes: Array<{ name: string; attributes: string[] }>
      relations: Array<{ node1: string; node2: string; type: string }>
      customInstructions: string
    }
    questionGeneration: {
      enabled: boolean
      questionCount: number
      customInstructions: string
    }
    autoTag: {
      enabled: boolean
      modelId: string
      maxTags: number
      skipIfTagged: boolean
    }
    wiki: {
      enabled: boolean
      synthesisModelId: string
      maxPagesPerIngest: number
      extractionGranularity: ExtractionGranularity
      contentInstructions: string
      extractionInstructions: string
    }
  }
  indexing: {
    vectorEnabled: boolean
    keywordEnabled: boolean
    wikiEnabled: boolean
    graphEnabled: boolean
  }
  storage: {
    backendId: string
    provider: string
  }
  vectorStoreId: string
}

type ApiRecord = Record<string, any>

const defaultSeparators = () => ['\n\n', '\n', '。', '！', '？', ';', '；']

export function defaultKnowledgeBaseEditorConfig(type: KnowledgeBaseType = 'document'): KnowledgeBaseEditorConfig {
  return {
    type,
    name: '',
    description: '',
    model: { embeddingModelId: '', llmModelId: '', wikiSynthesisModelId: '' },
    faq: { indexMode: 'question_only', questionIndexMode: 'separate' },
    chunking: {
      chunkSize: 512,
      chunkOverlap: 80,
      separators: defaultSeparators(),
      parserEngineRules: undefined,
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      strategy: 'auto',
      tokenLimit: 0,
      languages: [],
      tableMetadataInstructions: '',
    },
    processing: {
      languages: [],
      multimodal: { enabled: false, modelId: '', descriptionLanguage: '', customInstructions: '' },
      asr: { enabled: false, modelId: '', language: '' },
      graph: { enabled: false, text: '', tags: [], nodes: [], relations: [], customInstructions: '' },
      questionGeneration: { enabled: true, questionCount: 3, customInstructions: '' },
      autoTag: { enabled: false, modelId: '', maxTags: 3, skipIfTagged: true },
      wiki: {
        enabled: false,
        synthesisModelId: '',
        maxPagesPerIngest: 0,
        extractionGranularity: 'standard',
        contentInstructions: '',
        extractionInstructions: '',
      },
    },
    indexing: { vectorEnabled: true, keywordEnabled: true, wikiEnabled: false, graphEnabled: false },
    storage: { backendId: '', provider: 'local' },
    vectorStoreId: '',
  }
}

const valueOr = <T>(value: T | null | undefined, fallback: T): T => value == null ? fallback : value

function cloneNodes(value: unknown): Array<{ name: string; attributes: string[] }> {
  return Array.isArray(value)
    ? value.map((node) => ({ name: valueOr(node?.name, ''), attributes: [...valueOr(node?.attributes, [])] }))
    : []
}

function cloneRelations(value: unknown): Array<{ node1: string; node2: string; type: string }> {
  return Array.isArray(value)
    ? value.map((relation) => ({
      node1: valueOr(relation?.node1, ''), node2: valueOr(relation?.node2, ''), type: valueOr(relation?.type, ''),
    }))
    : []
}

export function hydrateKnowledgeBaseEditorConfig(record: ApiRecord = {}): KnowledgeBaseEditorConfig {
  const state = defaultKnowledgeBaseEditorConfig(record.type === 'faq' ? 'faq' : 'document')
  const chunking = record.chunking_config ?? {}
  const vlm = record.vlm_config ?? {}
  const asr = record.asr_config ?? {}
  const extract = record.extract_config ?? {}
  const questions = record.question_generation_config ?? {}
  const autoTag = record.auto_tag_config ?? {}
  const wiki = record.wiki_config ?? {}
  const indexing = record.indexing_strategy ?? {}

  state.name = valueOr(record.name, state.name)
  state.description = valueOr(record.description, state.description)
  state.model.embeddingModelId = valueOr(record.embedding_model_id, state.model.embeddingModelId)
  state.model.llmModelId = valueOr(record.summary_model_id, state.model.llmModelId)
  state.model.wikiSynthesisModelId = valueOr(wiki.synthesis_model_id, state.model.wikiSynthesisModelId)
  state.faq.indexMode = valueOr(record.faq_config?.index_mode, state.faq.indexMode)
  state.faq.questionIndexMode = valueOr(record.faq_config?.question_index_mode, state.faq.questionIndexMode)

  state.chunking = {
    chunkSize: valueOr(chunking.chunk_size, state.chunking.chunkSize),
    chunkOverlap: valueOr(chunking.chunk_overlap, state.chunking.chunkOverlap),
    separators: Array.isArray(chunking.separators) ? [...chunking.separators] : state.chunking.separators,
    parserEngineRules: Array.isArray(chunking.parser_engine_rules) ? structuredClone(chunking.parser_engine_rules) : undefined,
    enableParentChild: valueOr(chunking.enable_parent_child, state.chunking.enableParentChild),
    parentChunkSize: valueOr(chunking.parent_chunk_size, state.chunking.parentChunkSize),
    childChunkSize: valueOr(chunking.child_chunk_size, state.chunking.childChunkSize),
    strategy: valueOr(chunking.strategy, state.chunking.strategy),
    tokenLimit: valueOr(chunking.token_limit, state.chunking.tokenLimit),
    languages: Array.isArray(chunking.languages) ? [...chunking.languages] : state.chunking.languages,
    tableMetadataInstructions: valueOr(chunking.table_metadata_instructions, state.chunking.tableMetadataInstructions),
  }
  state.processing.languages = [...state.chunking.languages]
  state.processing.multimodal = {
    enabled: valueOr(vlm.enabled, false),
    modelId: valueOr(vlm.enabled, false) ? valueOr(vlm.model_id, '') : '',
    descriptionLanguage: valueOr(vlm.description_language, ''),
    customInstructions: valueOr(vlm.custom_instructions, ''),
  }
  state.processing.asr = {
    enabled: valueOr(asr.enabled, false),
    modelId: valueOr(asr.model_id, ''),
    language: valueOr(asr.language, ''),
  }
  state.processing.graph = {
    enabled: valueOr(extract.enabled, false),
    text: valueOr(extract.text, ''),
    tags: Array.isArray(extract.tags) ? [...extract.tags] : [],
    nodes: cloneNodes(extract.nodes),
    relations: cloneRelations(extract.relations),
    customInstructions: valueOr(extract.custom_instructions, ''),
  }
  state.processing.questionGeneration = {
    enabled: valueOr(questions.enabled, false),
    questionCount: valueOr(questions.question_count, 3),
    customInstructions: valueOr(questions.custom_instructions, ''),
  }
  state.processing.autoTag = {
    enabled: valueOr(autoTag.enabled, false),
    modelId: valueOr(autoTag.model_id, ''),
    maxTags: valueOr(autoTag.max_tags, 3),
    skipIfTagged: valueOr(autoTag.skip_if_tagged, true),
  }
  const granularity = wiki.extraction_granularity
  state.processing.wiki = {
    enabled: valueOr(indexing.wiki_enabled, false),
    synthesisModelId: valueOr(wiki.synthesis_model_id, ''),
    maxPagesPerIngest: valueOr(wiki.max_pages_per_ingest, 0),
    extractionGranularity: granularity === 'focused' || granularity === 'exhaustive' ? granularity : 'standard',
    contentInstructions: valueOr(wiki.content_instructions, ''),
    extractionInstructions: valueOr(wiki.extraction_instructions, ''),
  }
  state.indexing = {
    vectorEnabled: valueOr(indexing.vector_enabled, true),
    keywordEnabled: valueOr(indexing.keyword_enabled, true),
    wikiEnabled: valueOr(indexing.wiki_enabled, false),
    graphEnabled: valueOr(indexing.graph_enabled, false),
  }
  state.storage = {
    backendId: valueOr(record.storage_backend_id, ''),
    provider: valueOr(record.storage_provider_config?.provider, valueOr(record.storage_config?.provider, 'local')),
  }
  state.vectorStoreId = valueOr(record.vector_store_id, '')
  return state
}

function buildCommonPayload(state: KnowledgeBaseEditorConfig): ApiRecord {
  return {
    name: state.name,
    description: state.description,
    type: state.type,
    chunking_config: {
      chunk_size: state.chunking.chunkSize,
      chunk_overlap: state.chunking.chunkOverlap,
      separators: [...state.chunking.separators],
      ...(state.chunking.parserEngineRules?.length
        ? { parser_engine_rules: structuredClone(state.chunking.parserEngineRules) }
        : {}),
      enable_parent_child: state.chunking.enableParentChild,
      parent_chunk_size: state.chunking.parentChunkSize,
      child_chunk_size: state.chunking.childChunkSize,
      strategy: state.chunking.strategy,
      token_limit: state.chunking.tokenLimit,
      languages: [...state.chunking.languages],
      table_metadata_instructions: state.chunking.tableMetadataInstructions,
    },
    embedding_model_id: state.model.embeddingModelId,
    summary_model_id: state.model.llmModelId,
    ...(state.storage.backendId ? { storage_backend_id: state.storage.backendId } : {}),
    storage_provider_config: { provider: state.storage.provider },
    storage_config: { provider: state.storage.provider },
    vlm_config: {
      enabled: state.processing.multimodal.enabled,
      model_id: state.processing.multimodal.enabled ? state.processing.multimodal.modelId : '',
      description_language: state.processing.multimodal.descriptionLanguage,
      custom_instructions: state.processing.multimodal.customInstructions,
    },
    asr_config: {
      enabled: state.processing.asr.enabled,
      model_id: state.processing.asr.modelId,
      language: state.processing.asr.language,
    },
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
    extract_config: {
      enabled: state.processing.graph.enabled,
      text: state.processing.graph.text,
      tags: [...state.processing.graph.tags],
      nodes: structuredClone(state.processing.graph.nodes),
      relations: structuredClone(state.processing.graph.relations),
      custom_instructions: state.processing.graph.customInstructions,
    },
  }
}

export function buildKnowledgeBaseEditorPayload(
  state: KnowledgeBaseEditorConfig,
  mode: 'create' | 'update' = 'create',
): ApiRecord {
  const common = buildCommonPayload(state)
  const indexing_strategy = {
    vector_enabled: state.indexing.vectorEnabled,
    keyword_enabled: state.indexing.keywordEnabled,
    wiki_enabled: state.indexing.wikiEnabled,
    graph_enabled: state.indexing.graphEnabled,
  }
  const faq_config = {
    index_mode: state.faq.indexMode,
    question_index_mode: state.faq.questionIndexMode,
  }
  const wiki_config = {
    synthesis_model_id: state.processing.wiki.synthesisModelId,
    max_pages_per_ingest: state.processing.wiki.maxPagesPerIngest,
    extraction_granularity: state.processing.wiki.extractionGranularity,
    content_instructions: state.processing.wiki.contentInstructions,
    extraction_instructions: state.processing.wiki.extractionInstructions,
  }

  if (mode === 'create') {
    return {
      ...common,
      ...(state.type === 'faq' ? { faq_config } : { wiki_config, indexing_strategy }),
    }
  }

  return {
    base: {
      name: state.name,
      description: state.description,
      config: state.type === 'faq' ? { faq_config } : { wiki_config, indexing_strategy, auto_tag_config: common.auto_tag_config },
    },
    config: {
      embedding_model_id: common.embedding_model_id,
      summary_model_id: common.summary_model_id,
      chunking_config: common.chunking_config,
      vlm_config: common.vlm_config,
      asr_config: common.asr_config,
      storage_provider_config: common.storage_provider_config,
      storage_config: common.storage_config,
      question_generation_config: common.question_generation_config,
      auto_tag_config: common.auto_tag_config,
      extract_config: common.extract_config,
    },
  }
}

// Short aliases make the pure seam convenient for callers that call the object a state.
export const defaultKnowledgeBaseEditorState = defaultKnowledgeBaseEditorConfig
export const normalizeKnowledgeBaseEditorState = hydrateKnowledgeBaseEditorConfig
export const buildKnowledgeBasePayload = buildKnowledgeBaseEditorPayload

export type KnowledgeBaseEditorState = KnowledgeBaseEditorConfig
