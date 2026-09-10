import { parseKnowledgeDocumentListResponse, type KnowledgeDocument, type KnowledgeDocumentListResponse } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

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
  file: Blob;
  fileName?: string;
  tag_ids?: string[];
  metadata?: Record<string, string>;
  process_config?: unknown;
  enable_multimodel?: boolean;
  channel?: string;
}

export function createKnowledgeDocumentsApi(request: (input: ClientRequest) => Promise<unknown>) {
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
      form.append('file', input.file, input.fileName);
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
  };
}
