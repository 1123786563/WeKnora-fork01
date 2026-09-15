import {
  parseKnowledgeDocumentListResponse,
  parseKnowledgeDocumentResponse,
  parseKnowledgeFolderTreeResponse,
  parseKnowledgeSearchResponse,
  parseKnowledgeTagListResponse,
  type KnowledgeDocument,
  type KnowledgeDocumentListResponse,
  type KnowledgeFolderTree,
  type KnowledgeSearchResponse,
  type KnowledgeTagListResponse,
  type KnowledgeTag,
} from '@weknora/contracts';
import type { ClientBinaryResponse, ClientRequest } from '../client.ts';
import type { UploadProgressEvent } from '../ports.ts';
import type { NativeFileSource } from '../ports.ts';
import { ApiError } from '../errors.ts';

/** Minimal trace-node view (structurally compatible with the domain type). */
export interface KnowledgeSpanNodeView {
  name?: string;
  stage?: string;
  status?: string;
  start_time?: string;
  end_time?: string;
  duration_ms?: number;
  children?: KnowledgeSpanNodeView[];
  [key: string]: unknown;
}

/** GET /knowledge/:id/spans response (routes_knowledge.go:105 → GetKnowledgeSpans).
 *  Loose view: the trace tree is backend-owned and only read for display. */
export interface KnowledgeSpansResponse {
  knowledge_id?: string;
  attempt?: number;
  parse_status?: string;
  current_stage?: string;
  trace?: KnowledgeSpanNodeView | null;
  last_error?: { error_code?: string; error_message?: string } | null;
  [key: string]: unknown;
}

export interface KnowledgeDocumentListParams {
  page?: number;
  page_size?: number;
  tag_ids?: string;
  keyword?: string;
  file_type?: string;
  parse_status?: string;
  source?: string;
  start_time?: string;
  end_time?: string;
  folder_path?: string;
  folder_recursive?: boolean;
}

export interface KnowledgeDocumentUploadInput {
  file: Blob | NativeFileSource;
  fileName?: string;
  tag_ids?: string[];
  metadata?: Record<string, string>;
  process_config?: unknown;
  enable_multimodel?: boolean;
  channel?: string;
  onProgress?: (progress: UploadProgressEvent) => void;
}

export interface KnowledgeDocumentUrlInput {
  url: string;
  tag_ids?: string[];
  process_config?: unknown;
  enable_multimodel?: boolean;
}

export interface KnowledgeDocumentManualInput {
  title: string;
  content: string;
  status: string;
  tag_ids?: string[];
  process_config?: unknown;
}

export interface KnowledgeDocumentSearchParams {
  keyword?: string;
  offset?: number;
  limit?: number;
  file_types?: string[];
  agent_id?: string;
  agent_source_tenant_id?: string;
  recent?: boolean;
}

export interface KnowledgeChunk {
  id: string;
  content?: string;
  is_enabled?: boolean;
  content_revision?: number;
  index_status?: string;
  [key: string]: unknown;
}

export interface KnowledgeChunkPage {
  data: KnowledgeChunk[];
  total: number;
  page: number;
  page_size: number;
}

export interface KnowledgeChunkRevision {
  revision: number;
  content?: string;
  is_enabled?: boolean;
  [key: string]: unknown;
}

export interface KnowledgeChunkUpdateInput {
  content?: string;
  is_enabled?: boolean;
  expected_revision?: number;
}

export interface KnowledgeDocumentDetailsUpdateInput {
  description?: string;
  custom_metadata?: Record<string, unknown>;
}

export interface KnowledgeTagListParams {
  page?: number;
  page_size?: number;
  keyword?: string;
}

function isNativeFileSource(value: Blob | NativeFileSource): value is NativeFileSource {
  return typeof Blob === 'undefined' || !(value instanceof Blob);
}

function uploadResponseError(response: unknown): ApiError {
  const findField = (value: unknown, keys: readonly string[], depth = 0): unknown => {
    if (depth > 3 || value === null || typeof value !== 'object') return undefined;
    const record = value as Record<string, unknown>;
    for (const key of keys) if (record[key] !== undefined) return record[key];
    for (const key of ['error', 'body', 'response', 'details']) {
      const nested = findField(record[key], keys, depth + 1);
      if (nested !== undefined) return nested;
    }
    return undefined;
  };
  const codeValue = findField(response, ['code', 'error_code']);
  const code = typeof codeValue === 'string' && codeValue.trim() ? codeValue : typeof codeValue === 'number' ? String(codeValue) : 'UPLOAD_FAILED';
  const messageValue = findField(response, ['message', 'error_message']);
  const message = typeof messageValue === 'string' && messageValue.trim() ? messageValue : 'Knowledge document upload failed';
  const statusValue = findField(response, ['status']);
  const status = typeof statusValue === 'number' ? statusValue : undefined;
  return new ApiError({ code, message, status, details: response });
}

function parseChunkPage(value: unknown): KnowledgeChunkPage {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('Invalid knowledge chunk page');
  const row = value as Record<string, unknown>;
  if (!Array.isArray(row.data)) throw new Error('Invalid knowledge chunk data');
  const data = row.data.map((item, index) => {
    if (typeof item !== 'object' || item === null || Array.isArray(item) || typeof (item as { id?: unknown }).id !== 'string') {
      throw new Error(`Invalid knowledge chunk id at data[${index}]`);
    }
    return item as KnowledgeChunk;
  });
  return {
    data,
    total: Number(row.total ?? data.length),
    page: Number(row.page ?? 1),
    page_size: Number(row.page_size ?? 25),
  };
}

function parseChunkRevisions(value: unknown): KnowledgeChunkRevision[] {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('Invalid knowledge chunk revisions');
  const row = value as Record<string, unknown>;
  if (!Array.isArray(row.data)) throw new Error('Invalid knowledge chunk revisions data');
  return row.data.map((item, index) => {
    if (typeof item !== 'object' || item === null || Array.isArray(item) || typeof (item as { revision?: unknown }).revision !== 'number') {
      throw new Error(`Invalid knowledge chunk revision at data[${index}]`);
    }
    return item as KnowledgeChunkRevision;
  });
}

export function createKnowledgeDocumentsApi(
  request: (input: ClientRequest) => Promise<unknown>,
  requestBinary?: (input: ClientRequest) => Promise<ClientBinaryResponse>,
) {
  const knowledgePath = (id: string, suffix = '') => `/api/v1/knowledge/${encodeURIComponent(id)}${suffix}`;

  async function parseDocumentMutation(response: unknown): Promise<KnowledgeDocument> {
    return parseKnowledgeDocumentResponse(response);
  }

  return {
    async list(knowledgeBaseId: string, params: KnowledgeDocumentListParams = {}): Promise<KnowledgeDocumentListResponse> {
      const query = new URLSearchParams();
      for (const [key, value] of Object.entries(params)) {
        if (value !== undefined) query.set(key, String(value));
      }
      const suffix = query.toString();
      const path = `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/knowledge${suffix ? `?${suffix}` : ''}`;
      return parseKnowledgeDocumentListResponse(await request({ method: 'GET', path }));
    },
    async upload(knowledgeBaseId: string, input: KnowledgeDocumentUploadInput, signal?: AbortSignal): Promise<KnowledgeDocument> {
      const form = new FormData();
      const nativeFile = isNativeFileSource(input.file) ? input.file : undefined;
      if (nativeFile) {
        form.append('file', nativeFile as unknown as Blob);
      } else {
        form.append('file', input.file as Blob, input.fileName);
      }
      const multipartFields: Record<string, string> = {};
      if (input.fileName) multipartFields.fileName = input.fileName;
      if (input.tag_ids) multipartFields.tag_ids = input.tag_ids.join(',');
      if (input.metadata) multipartFields.metadata = JSON.stringify(input.metadata);
      if (input.process_config !== undefined) multipartFields.process_config = JSON.stringify(input.process_config);
      if (input.enable_multimodel !== undefined) multipartFields.enable_multimodel = String(input.enable_multimodel);
      if (input.channel) multipartFields.channel = input.channel;
      for (const [key, value] of Object.entries(multipartFields)) form.append(key, value);
      const response = await request({
        method: 'POST',
        path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/knowledge/file`,
        body: form,
        nativeFile,
        multipartFields,
        signal,
        onProgress: input.onProgress,
      });
      if (typeof response !== 'object' || response === null || !('success' in response) || (response as { success?: unknown }).success !== true) {
        throw uploadResponseError(response);
      }
      const data = (response as { data?: unknown }).data;
      if (typeof data !== 'object' || data === null || Array.isArray(data) || typeof (data as { id?: unknown }).id !== 'string') {
        throw new Error('Invalid knowledge upload data');
      }
      return data as KnowledgeDocument;
    },
    async createFromUrl(knowledgeBaseId: string, input: KnowledgeDocumentUrlInput): Promise<KnowledgeDocument> {
      return parseDocumentMutation(await request({
        method: 'POST',
        path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/knowledge/url`,
        body: input,
      }));
    },
    async createManual(knowledgeBaseId: string, input: KnowledgeDocumentManualInput): Promise<KnowledgeDocument> {
      return parseDocumentMutation(await request({
        method: 'POST',
        path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/knowledge/manual`,
        body: input,
      }));
    },
    async updateManual(id: string, input: KnowledgeDocumentManualInput): Promise<KnowledgeDocument> {
      return parseDocumentMutation(await request({
        method: 'PUT',
        path: `/api/v1/knowledge/manual/${encodeURIComponent(id)}`,
        body: input,
      }));
    },
    async moveToFolder(knowledgeBaseId: string, knowledgeIds: string[], folderPath: string): Promise<void> {
      await request({ method: 'POST', path: '/api/v1/knowledge/folder', body: { kb_id: knowledgeBaseId, knowledge_ids: knowledgeIds, folder_path: folderPath } });
    },
    async renameFolder(knowledgeBaseId: string, from: string, to: string): Promise<void> {
      await request({ method: 'PUT', path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/knowledge/folders`, body: { from, to } });
    },
    async updateTags(updates: Record<string, string[]>): Promise<void> {
      await request({ method: 'PUT', path: '/api/v1/knowledge/tags', body: { updates } });
    },
    async folders(knowledgeBaseId: string): Promise<KnowledgeFolderTree> {
      return parseKnowledgeFolderTreeResponse(await request({
        method: 'GET',
        path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/knowledge/folders`,
      }));
    },
    async tags(knowledgeBaseId: string, params: KnowledgeTagListParams = {}): Promise<KnowledgeTag[]> {
      return (await this.tagsPage(knowledgeBaseId, params)).data;
    },
    async tagsPage(knowledgeBaseId: string, params: KnowledgeTagListParams = {}): Promise<KnowledgeTagListResponse> {
      const query = new URLSearchParams();
      for (const [key, value] of Object.entries(params)) if (value !== undefined) query.set(key, String(value));
      const suffix = query.toString();
      const path = `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/tags${suffix ? `?${suffix}` : ''}`;
      return parseKnowledgeTagListResponse(await request({ method: 'GET', path }));
    },
    async createTag(knowledgeBaseId: string, input: { name: string; color?: string; sort_order?: number }): Promise<void> {
      await request({ method: 'POST', path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/tags`, body: input });
    },
    async updateTag(knowledgeBaseId: string, tagId: string, input: { name?: string; color?: string; sort_order?: number }): Promise<void> {
      await request({ method: 'PUT', path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/tags/${encodeURIComponent(tagId)}`, body: input });
    },
    async deleteTag(knowledgeBaseId: string, tagSeqId: number, force = false): Promise<void> {
      await request({ method: 'DELETE', path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/tags/${encodeURIComponent(String(tagSeqId))}${force ? '?force=true' : ''}` });
    },
    async get(id: string, options: { agent_id?: string; agent_source_tenant_id?: string } = {}): Promise<KnowledgeDocument> {
      const query = new URLSearchParams();
      for (const [key, value] of Object.entries(options)) if (value !== undefined) query.set(key, value);
      const suffix = query.toString();
      return parseKnowledgeDocumentResponse(await request({
        method: 'GET',
        path: `${knowledgePath(id)}${suffix ? `?${suffix}` : ''}`,
      }));
    },
    async chunks(id: string, page = 1, pageSize = 25): Promise<KnowledgeChunkPage> {
      return parseChunkPage(await request({
        method: 'GET',
        path: `/api/v1/chunks/${encodeURIComponent(id)}?page=${page}&page_size=${pageSize}`,
      }));
    },
    async updateDetails(id: string, input: KnowledgeDocumentDetailsUpdateInput): Promise<KnowledgeDocument> {
      return parseDocumentMutation(await request({
        method: 'PUT',
        path: knowledgePath(id),
        body: input,
      }));
    },
    async updateChunk(id: string, chunkId: string, input: KnowledgeChunkUpdateInput): Promise<KnowledgeChunk> {
      const response = await request({
        method: 'PUT',
        path: `/api/v1/chunks/${encodeURIComponent(id)}/${encodeURIComponent(chunkId)}`,
        body: input,
      });
      const parsed = typeof response === 'object' && response !== null ? response as Record<string, unknown> : {};
      const data = parsed.data;
      if (typeof data !== 'object' || data === null || Array.isArray(data) || typeof (data as { id?: unknown }).id !== 'string') throw new Error('Invalid knowledge chunk update');
      return data as KnowledgeChunk;
    },
    async chunkRevisions(id: string, chunkId: string): Promise<KnowledgeChunkRevision[]> {
      return parseChunkRevisions(await request({
        method: 'GET',
        path: `/api/v1/chunks/${encodeURIComponent(id)}/${encodeURIComponent(chunkId)}/revisions`,
      }));
    },
    async revertChunk(id: string, chunkId: string, revision: number, expectedRevision: number): Promise<KnowledgeChunk> {
      const response = await request({
        method: 'POST',
        path: `/api/v1/chunks/${encodeURIComponent(id)}/${encodeURIComponent(chunkId)}/revert`,
        body: { revision, expected_revision: expectedRevision },
      });
      const parsed = typeof response === 'object' && response !== null ? response as Record<string, unknown> : {};
      const data = parsed.data;
      if (typeof data !== 'object' || data === null || Array.isArray(data) || typeof (data as { id?: unknown }).id !== 'string') throw new Error('Invalid knowledge chunk revert');
      return data as KnowledgeChunk;
    },
    downloadPath(id: string): string {
      return knowledgePath(id, '/download');
    },
    previewPath(id: string): string {
      return knowledgePath(id, '/preview');
    },
    async spans(id: string): Promise<KnowledgeSpansResponse> {
      const response = await request({ method: 'GET', path: knowledgePath(id, '/spans') });
      if (typeof response !== 'object' || response === null || Array.isArray(response)) throw new Error('Invalid knowledge spans response');
      return response as KnowledgeSpansResponse;
    },
    async preview(id: string, signal?: AbortSignal): Promise<ClientBinaryResponse> {
      if (!requestBinary) throw new Error('Binary transport is unavailable');
      return requestBinary({ method: 'GET', path: knowledgePath(id, '/preview'), signal });
    },
    async download(id: string, signal?: AbortSignal): Promise<ClientBinaryResponse> {
      if (!requestBinary) throw new Error('Binary transport is unavailable');
      return requestBinary({ method: 'GET', path: knowledgePath(id, '/download'), signal });
    },
    async reparse(id: string, process_config?: unknown): Promise<void> {
      await request({ method: 'POST', path: knowledgePath(id, '/reparse'), body: process_config === undefined ? undefined : { process_config } });
    },
    async cancelParse(id: string): Promise<void> {
      await request({ method: 'POST', path: knowledgePath(id, '/cancel-parse') });
    },
    async remove(id: string): Promise<void> {
      await request({ method: 'DELETE', path: knowledgePath(id) });
    },
    async batchDelete(knowledgeBaseId: string, ids: string[]): Promise<void> {
      await request({ method: 'POST', path: '/api/v1/knowledge/batch-delete', body: { kb_id: knowledgeBaseId, ids } });
    },
    async search(params: KnowledgeDocumentSearchParams = {}): Promise<KnowledgeSearchResponse> {
      const query = new URLSearchParams();
      if (params.keyword) query.set('keyword', params.keyword);
      if (params.offset !== undefined) query.set('offset', String(params.offset));
      if (params.limit !== undefined) query.set('limit', String(params.limit));
      if (params.file_types?.length) query.set('file_types', params.file_types.join(','));
      for (const key of ['agent_id', 'agent_source_tenant_id'] as const) if (params[key] !== undefined) query.set(key, params[key]!);
      if (params.recent !== undefined) query.set('recent', String(params.recent));
      return parseKnowledgeSearchResponse(await request({ method: 'GET', path: `/api/v1/knowledge/search?${query.toString()}` }));
    },
  };
}
