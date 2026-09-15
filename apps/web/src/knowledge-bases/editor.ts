export type KnowledgeBaseType = 'document' | 'faq'
export type KnowledgeBaseEditorSection = 'basic' | 'models' | 'vectorStore' | 'parser' | 'multimodal' | 'asr' | 'storage' | 'chunking' | 'graph' | 'advanced' | 'faq'

export interface KnowledgeBaseEditorState {
  type: KnowledgeBaseType; name: string; description: string;
  model: { embeddingModelId: string; llmModelId: string };
  vectorStore: { id: string; source: string; name: string; engineType: string; status: string };
  faq: { indexMode: string; questionIndexMode: string };
  chunking: { chunkSize: number; chunkOverlap: number; separators: string[]; enableParentChild: boolean; parentChunkSize: number; childChunkSize: number; strategy: string; tokenLimit: number; languages: string[]; tableMetadataInstructions: string };
  processing: {
    chunkSize: number; chunkOverlap: number; separators: string[]; parserEngineRules?: Array<Record<string, unknown>>;
    enableParentChild: boolean; parentChunkSize: number; childChunkSize: number; strategy: string; tokenLimit: number;
    languages: string[]; tableMetadataInstructions: string;
    multimodal: { enabled: boolean; vllmModelId: string; descriptionLanguage: string; customInstructions: string };
    asr: { enabled: boolean; modelId: string; language: string };
    graph: { enabled: boolean; text: string; tags: string[]; nodes: Array<{ name: string; attributes: string[] }>; relations: Array<{ node1: string; node2: string; type: string }>; customInstructions: string };
    questionGeneration: { enabled: boolean; questionCount: number; customInstructions: string };
    autoTag: { enabled: boolean; modelId: string; maxTags: number; skipIfTagged: boolean };
    wiki: { enabled: boolean; synthesisModelId: string; maxPagesPerIngest: number; extractionGranularity: 'focused' | 'standard' | 'exhaustive'; contentInstructions: string; extractionInstructions: string };
  };
  indexing: { vectorEnabled: boolean; keywordEnabled: boolean; wikiEnabled: boolean; graphEnabled: boolean };
  storage: { backendId: string; provider: string };
}

type RecordValue = Record<string, any>
const separators = () => ['\n\n', '\n', '。', '！', '？', ';', '；']
const valueOr = <T>(value: T | null | undefined, fallback: T): T => value == null ? fallback : value

export function getKnowledgeBaseSections(type: KnowledgeBaseType): Array<{ key: KnowledgeBaseEditorSection; label: string }> {
  return type === 'faq'
    ? [{ key: 'basic', label: 'Basic' }, { key: 'models', label: 'Models' }, { key: 'vectorStore', label: 'Vector store' }, { key: 'faq', label: 'FAQ' }]
    : ['basic', 'models', 'vectorStore', 'parser', 'multimodal', 'asr', 'storage', 'chunking', 'graph', 'advanced'].map((key) => ({ key: key as KnowledgeBaseEditorSection, label: key }))
}

export function defaultKnowledgeBaseEditorState(type: KnowledgeBaseType = 'document'): KnowledgeBaseEditorState {
  const chunking = { chunkSize: 512, chunkOverlap: 80, separators: separators(), enableParentChild: true, parentChunkSize: 4096, childChunkSize: 384, strategy: 'auto', tokenLimit: 0, languages: [] as string[], tableMetadataInstructions: '' }
  return {
    type, name: '', description: '', model: { embeddingModelId: '', llmModelId: '' },
    vectorStore: { id: '', source: '', name: '', engineType: '', status: '' }, faq: { indexMode: 'question_only', questionIndexMode: 'separate' }, chunking,
    processing: { ...chunking,
      multimodal: { enabled: false, vllmModelId: '', descriptionLanguage: '', customInstructions: '' }, asr: { enabled: false, modelId: '', language: '' },
      graph: { enabled: false, text: '', tags: [], nodes: [], relations: [], customInstructions: '' }, questionGeneration: { enabled: true, questionCount: 3, customInstructions: '' }, autoTag: { enabled: false, modelId: '', maxTags: 3, skipIfTagged: true },
      wiki: { enabled: false, synthesisModelId: '', maxPagesPerIngest: 0, extractionGranularity: 'standard', contentInstructions: '', extractionInstructions: '' } },
    indexing: { vectorEnabled: true, keywordEnabled: true, wikiEnabled: false, graphEnabled: false }, storage: { backendId: '', provider: 'local' },
  }
}

export function normalizeKnowledgeBaseEditorState(record: RecordValue = {}): KnowledgeBaseEditorState {
  const state = defaultKnowledgeBaseEditorState(record.type === 'faq' ? 'faq' : 'document')
  const chunk = record.chunking_config ?? {}; const indexing = record.indexing_strategy ?? {}; const vlm = record.vlm_config ?? {}; const asr = record.asr_config ?? {}; const extract = record.extract_config ?? {}; const questions = record.question_generation_config ?? {}; const auto = record.auto_tag_config ?? {}; const wiki = record.wiki_config ?? {}
  state.name = valueOr(record.name, ''); state.description = valueOr(record.description, ''); state.model.embeddingModelId = valueOr(record.embedding_model_id, ''); state.model.llmModelId = valueOr(record.summary_model_id, '')
  state.faq.indexMode = valueOr(record.faq_config?.index_mode, state.faq.indexMode); state.faq.questionIndexMode = valueOr(record.faq_config?.question_index_mode, state.faq.questionIndexMode)
  Object.assign(state.processing, { chunkSize: valueOr(chunk.chunk_size, state.processing.chunkSize), chunkOverlap: valueOr(chunk.chunk_overlap, state.processing.chunkOverlap), separators: Array.isArray(chunk.separators) ? [...chunk.separators] : state.processing.separators, parserEngineRules: Array.isArray(chunk.parser_engine_rules) ? structuredClone(chunk.parser_engine_rules) : undefined, enableParentChild: valueOr(chunk.enable_parent_child, true), parentChunkSize: valueOr(chunk.parent_chunk_size, 4096), childChunkSize: valueOr(chunk.child_chunk_size, 384), strategy: valueOr(chunk.strategy, 'auto'), tokenLimit: valueOr(chunk.token_limit, 0), languages: Array.isArray(chunk.languages) ? [...chunk.languages] : [], tableMetadataInstructions: valueOr(chunk.table_metadata_instructions, '') })
  state.chunking = { ...state.chunking, ...state.processing, separators: [...state.processing.separators], languages: [...state.processing.languages] }
  state.processing.multimodal = { enabled: valueOr(vlm.enabled, false), vllmModelId: valueOr(vlm.model_id, ''), descriptionLanguage: valueOr(vlm.description_language, ''), customInstructions: valueOr(vlm.custom_instructions, '') }
  state.processing.asr = { enabled: valueOr(asr.enabled, false), modelId: valueOr(asr.model_id, ''), language: valueOr(asr.language, '') }
  state.processing.graph = { enabled: valueOr(extract.enabled, false), text: valueOr(extract.text, ''), tags: Array.isArray(extract.tags) ? [...extract.tags] : [], nodes: Array.isArray(extract.nodes) ? structuredClone(extract.nodes) : [], relations: Array.isArray(extract.relations) ? structuredClone(extract.relations) : [], customInstructions: valueOr(extract.custom_instructions, '') }
  state.processing.questionGeneration = { enabled: valueOr(questions.enabled, false), questionCount: valueOr(questions.question_count, 3), customInstructions: valueOr(questions.custom_instructions, '') }
  state.processing.autoTag = { enabled: valueOr(auto.enabled, false), modelId: valueOr(auto.model_id, ''), maxTags: valueOr(auto.max_tags, 3), skipIfTagged: valueOr(auto.skip_if_tagged, true) }
  state.processing.wiki = { enabled: valueOr(indexing.wiki_enabled, false), synthesisModelId: valueOr(wiki.synthesis_model_id, ''), maxPagesPerIngest: valueOr(wiki.max_pages_per_ingest, 0), extractionGranularity: wiki.extraction_granularity === 'focused' || wiki.extraction_granularity === 'exhaustive' ? wiki.extraction_granularity : 'standard', contentInstructions: valueOr(wiki.content_instructions, ''), extractionInstructions: valueOr(wiki.extraction_instructions, '') }
  state.indexing = { vectorEnabled: valueOr(indexing.vector_enabled, true), keywordEnabled: valueOr(indexing.keyword_enabled, true), wikiEnabled: valueOr(indexing.wiki_enabled, false), graphEnabled: valueOr(indexing.graph_enabled, false) }
  state.storage = { backendId: valueOr(record.storage_backend_id, ''), provider: valueOr(record.storage_provider_config?.provider, valueOr(record.storage_config?.provider, 'local')) }
  state.vectorStore = { id: valueOr(record.vector_store_id, ''), source: valueOr(record.vector_store_source, ''), name: valueOr(record.vector_store_name, ''), engineType: valueOr(record.vector_store_engine_type, ''), status: valueOr(record.vector_store_status, '') }
  return state
}

const commonPayload = (state: KnowledgeBaseEditorState) => ({ name: state.name, description: state.description, type: state.type, chunking_config: { chunk_size: state.processing.chunkSize, chunk_overlap: state.processing.chunkOverlap, separators: [...state.processing.separators], ...(state.processing.parserEngineRules?.length ? { parser_engine_rules: structuredClone(state.processing.parserEngineRules) } : {}), enable_parent_child: state.processing.enableParentChild, parent_chunk_size: state.processing.parentChunkSize, child_chunk_size: state.processing.childChunkSize, strategy: state.processing.strategy, token_limit: state.processing.tokenLimit, languages: [...state.processing.languages], table_metadata_instructions: state.processing.tableMetadataInstructions }, embedding_model_id: state.model.embeddingModelId, summary_model_id: state.model.llmModelId, ...(state.vectorStore.id ? { vector_store_id: state.vectorStore.id } : {}), vlm_config: { enabled: state.processing.multimodal.enabled, model_id: state.processing.multimodal.enabled ? state.processing.multimodal.vllmModelId : '', description_language: state.processing.multimodal.descriptionLanguage, custom_instructions: state.processing.multimodal.customInstructions }, asr_config: { enabled: state.processing.asr.enabled, model_id: state.processing.asr.modelId, language: state.processing.asr.language }, storage_provider_config: { provider: state.storage.provider }, storage_config: { provider: state.storage.provider }, question_generation_config: { enabled: state.processing.questionGeneration.enabled, question_count: state.processing.questionGeneration.questionCount, custom_instructions: state.processing.questionGeneration.customInstructions }, auto_tag_config: { enabled: state.processing.autoTag.enabled, model_id: state.processing.autoTag.modelId, max_tags: state.processing.autoTag.maxTags, skip_if_tagged: state.processing.autoTag.skipIfTagged }, extract_config: { enabled: state.processing.graph.enabled, text: state.processing.graph.text, tags: [...state.processing.graph.tags], nodes: structuredClone(state.processing.graph.nodes), relations: structuredClone(state.processing.graph.relations), custom_instructions: state.processing.graph.customInstructions } })

export function buildKnowledgeBasePayload(state: KnowledgeBaseEditorState, mode: 'create' | 'update' = 'create'): RecordValue {
  const common = commonPayload(state); const wiki_config = { synthesis_model_id: state.processing.wiki.synthesisModelId, max_pages_per_ingest: state.processing.wiki.maxPagesPerIngest, extraction_granularity: state.processing.wiki.extractionGranularity, content_instructions: state.processing.wiki.contentInstructions, extraction_instructions: state.processing.wiki.extractionInstructions }; const indexing_strategy = { vector_enabled: state.indexing.vectorEnabled, keyword_enabled: state.indexing.keywordEnabled, wiki_enabled: state.indexing.wikiEnabled, graph_enabled: state.indexing.graphEnabled }; const faq_config = { index_mode: state.faq.indexMode, question_index_mode: state.faq.questionIndexMode }
  if (mode === 'create') return { ...common, ...(state.type === 'faq' ? { faq_config } : { wiki_config, indexing_strategy }) }
  const { name: _name, description: _description, type: _type, vector_store_id: _vectorStoreId, ...config } = common
  return { base: { name: state.name, description: state.description, config: state.type === 'faq' ? { faq_config } : { wiki_config, indexing_strategy, auto_tag_config: common.auto_tag_config } }, config: { ...config, ...(state.type === 'faq' ? {} : { indexing_strategy }) } }
}

export function validateKnowledgeBaseEditorState(state: KnowledgeBaseEditorState): Array<{ field: string; section: KnowledgeBaseEditorSection; message: string }> {
  const errors: Array<{ field: string; section: KnowledgeBaseEditorSection; message: string }> = []
  if (!state.name.trim()) errors.push({ field: 'name', section: 'basic', message: 'Knowledge-base name is required' })
  if (state.type === 'document' && !Object.values(state.indexing).some(Boolean)) errors.push({ field: 'indexing', section: 'basic', message: 'At least one indexing strategy is required' })
  if (state.type === 'document' && state.indexing.vectorEnabled && !state.model.embeddingModelId.trim()) errors.push({ field: 'embedding_model_id', section: 'models', message: 'Embedding model is required' })
  if (!state.model.llmModelId.trim()) errors.push({ field: 'summary_model_id', section: 'models', message: 'Summary model is required' })
  return errors
}
