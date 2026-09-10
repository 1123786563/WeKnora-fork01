import type { KnowledgeBase, KnowledgeBaseUpdateInput } from '@weknora/api-client';

export interface ParserRule { file_types: string[]; engine: string; xlsx_first_row_as_header?: boolean }

export interface KnowledgeBaseSettingsForm {
  name: string;
  description: string;
  chunkSize: number;
  chunkOverlap: number;
  chunkStrategy: string;
  parentChild: boolean;
  parentChunkSize: number;
  childChunkSize: number;
  tokenLimit: number;
  parserRules: ParserRule[];
  embeddingModelId: string;
  summaryModelId: string;
  storageBackendId: string;
  vectorStoreId: string;
  indexing: { vector_enabled: boolean; keyword_enabled: boolean; wiki_enabled: boolean; graph_enabled: boolean };
  questionGenerationEnabled: boolean;
  autoTagEnabled: boolean;
  questionGenerationConfig: Record<string, unknown>;
  autoTagConfig: Record<string, unknown>;
  tableMetadataInstructions: string;
}

function object(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function number(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function boolean(value: unknown, fallback: boolean): boolean { return typeof value === 'boolean' ? value : fallback; }

export function formFromKnowledgeBase(kb: KnowledgeBase): KnowledgeBaseSettingsForm {
  const chunk = object(kb.chunking_config);
  const indexing = object(kb.indexing_strategy);
  const question = object(kb.question_generation_config);
  const autoTag = object(kb.auto_tag_config);
  return {
    name: typeof kb.name === 'string' ? kb.name : '',
    description: typeof kb.description === 'string' ? kb.description : '',
    chunkSize: number(chunk.chunk_size, 512),
    chunkOverlap: number(chunk.chunk_overlap, 50),
    chunkStrategy: typeof chunk.strategy === 'string' && chunk.strategy ? chunk.strategy : 'auto',
    parentChild: boolean(chunk.enable_parent_child, false),
    parentChunkSize: number(chunk.parent_chunk_size, 4096),
    childChunkSize: number(chunk.child_chunk_size, 384),
    tokenLimit: number(chunk.token_limit, 0),
    parserRules: Array.isArray(chunk.parser_engine_rules) ? chunk.parser_engine_rules.flatMap((item) => {
      const row = object(item);
      return typeof row.engine === 'string' && Array.isArray(row.file_types) && row.file_types.every((type) => typeof type === 'string') ? [{ file_types: row.file_types as string[], engine: row.engine, ...(typeof row.xlsx_first_row_as_header === 'boolean' ? { xlsx_first_row_as_header: row.xlsx_first_row_as_header } : {}) }] : [];
    }) : [],
    embeddingModelId: typeof kb.embedding_model_id === 'string' ? kb.embedding_model_id : '',
    summaryModelId: typeof kb.summary_model_id === 'string' ? kb.summary_model_id : '',
    storageBackendId: typeof kb.storage_backend_id === 'string' ? kb.storage_backend_id : '',
    vectorStoreId: typeof kb.vector_store_id === 'string' ? kb.vector_store_id : '',
    indexing: {
      vector_enabled: boolean(indexing.vector_enabled, true),
      keyword_enabled: boolean(indexing.keyword_enabled, true),
      wiki_enabled: boolean(indexing.wiki_enabled, false),
      graph_enabled: boolean(indexing.graph_enabled, false),
    },
    questionGenerationEnabled: boolean(object(kb.question_generation_config).enabled, false),
    autoTagEnabled: boolean(autoTag.enabled, false),
    questionGenerationConfig: { ...question },
    autoTagConfig: { ...autoTag },
    tableMetadataInstructions: typeof chunk.table_metadata_instructions === 'string' ? chunk.table_metadata_instructions : '',
  };
}

export function buildKnowledgeBaseSettingsInput(form: KnowledgeBaseSettingsForm): KnowledgeBaseUpdateInput {
  return {
    name: form.name.trim(),
    description: form.description,
    config: {
      chunking_config: {
        chunk_size: Math.max(1, Math.trunc(form.chunkSize)),
        chunk_overlap: Math.max(0, Math.trunc(form.chunkOverlap)),
        strategy: form.chunkStrategy,
        enable_parent_child: form.parentChild,
        parent_chunk_size: Math.max(1, Math.trunc(form.parentChunkSize)),
        child_chunk_size: Math.max(1, Math.trunc(form.childChunkSize)),
        token_limit: Math.max(0, Math.trunc(form.tokenLimit)),
        parser_engine_rules: form.parserRules,
        table_metadata_instructions: form.tableMetadataInstructions,
      },
      indexing_strategy: { ...form.indexing },
      auto_tag_config: { ...form.autoTagConfig, enabled: form.autoTagEnabled },
    },
  };
}

export function updateParserRule(rules: ParserRule[], fileTypes: string[], engine: string): ParserRule[] {
  const normalized = new Set(fileTypes.map((type) => type.trim().toLowerCase()).filter(Boolean));
  const remaining = rules.flatMap((rule) => {
    const kept = rule.file_types.filter((type) => !normalized.has(type.toLowerCase()));
    return kept.length ? [{ ...rule, file_types: kept }] : [];
  });
  return engine.trim() && normalized.size ? [...remaining, { file_types: [...normalized], engine: engine.trim() }] : remaining;
}
