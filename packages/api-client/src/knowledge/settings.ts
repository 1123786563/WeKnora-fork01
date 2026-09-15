import { parseKnowledgeBaseResponse, type KnowledgeBase } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

export interface KnowledgeBaseConfigInput {
  chunking_config?: Record<string, unknown>;
  image_processing_config?: Record<string, unknown>;
  faq_config?: Record<string, unknown>;
  wiki_config?: Record<string, unknown>;
  auto_tag_config?: Record<string, unknown>;
  indexing_strategy?: {
    vector_enabled: boolean;
    keyword_enabled: boolean;
    wiki_enabled: boolean;
    graph_enabled: boolean;
  };
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

function record(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(`Invalid ${label}`);
  return value as Record<string, unknown>;
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
  const body = record(value, 'chunking preview');
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
