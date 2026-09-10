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
  type KnowledgeTag,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';
import type { NativeFileSource } from '../ports.ts';

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

export interface KnowledgeTagListParams {
  page?: number;
  page_size?: number;
  keyword?: string;
}

function isNativeFileSource(value: Blob | NativeFileSource): value is NativeFileSource {
  return typeof Blob === 'undefined' || !(value instanceof Blob);
}

export function createKnowledgeDocumentsApi(request: (input: ClientRequest) => Promise<unknown>) {
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
      if (isNativeFileSource(input.file)) {
        form.append('file', input.file as unknown as Blob);
      } else {
        form.append('file', input.file, input.fileName);
      }
      if (input.tag_ids) form.append('tag_ids', input.tag_ids.join(','));
      if (input.metadata) form.append('metadata', JSON.stringify(input.metadata));
      if (input.process_config !== undefined) form.append('process_config', JSON.stringify(input.process_config));
      if (input.enable_multimodel !== undefined) form.append('enable_multimodel', String(input.enable_multimodel));
      if (input.channel) form.append('channel', input.channel);
      const response = await request({
        method: 'POST',
        path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/knowledge/file`,
        body: form,
        signal,
      });
      if (typeof response !== 'object' || response === null || !('success' in response) || (response as { success?: unknown }).success !== true) {
        throw new Error('Invalid knowledge upload response');
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
      const query = new URLSearchParams();
      for (const [key, value] of Object.entries(params)) if (value !== undefined) query.set(key, String(value));
      const suffix = query.toString();
      const path = `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/tags${suffix ? `?${suffix}` : ''}`;
      return parseKnowledgeTagListResponse(await request({ method: 'GET', path })).data;
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
    downloadPath(id: string): string {
      return knowledgePath(id, '/download');
    },
    previewPath(id: string): string {
      return knowledgePath(id, '/preview');
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
