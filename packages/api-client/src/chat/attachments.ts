import {
  parseTemporaryAttachmentListResponse,
  parseTemporaryAttachmentResponse,
  type TemporaryAttachment,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';
import type { NativeFileSource } from '../ports.ts';

export interface ChatAttachmentUploadInput {
  file: Blob | NativeFileSource;
  fileName?: string;
  agentId?: string;
  agentSourceTenantId?: string;
  parserEngine?: string;
}

function isNativeFileSource(value: Blob | NativeFileSource): value is NativeFileSource {
  return typeof Blob === 'undefined' || !(value instanceof Blob);
}

function encoded(value: string, name: string): string {
  if (!value.trim()) throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

export function createChatAttachmentsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async upload(sessionId: string, input: ChatAttachmentUploadInput, signal?: AbortSignal): Promise<TemporaryAttachment> {
      const form = new FormData();
      if (isNativeFileSource(input.file)) form.append('file', input.file as unknown as Blob);
      else form.append('file', input.file, input.fileName);
      if (input.agentId) form.append('agent_id', input.agentId);
      if (input.agentSourceTenantId) form.append('agent_source_tenant_id', input.agentSourceTenantId);
      if (input.parserEngine) form.append('parser_engine', input.parserEngine);
      return parseTemporaryAttachmentResponse(await request({ method: 'POST', path: `/api/v1/sessions/${encoded(sessionId, 'sessionId')}/attachments`, body: form, signal }));
    },
    async list(sessionId: string, signal?: AbortSignal): Promise<TemporaryAttachment[]> {
      return (parseTemporaryAttachmentListResponse(await request({ method: 'GET', path: `/api/v1/sessions/${encoded(sessionId, 'sessionId')}/attachments`, signal }))).data;
    },
    async get(sessionId: string, attachmentId: string, signal?: AbortSignal): Promise<TemporaryAttachment> {
      return parseTemporaryAttachmentResponse(await request({ method: 'GET', path: `/api/v1/sessions/${encoded(sessionId, 'sessionId')}/attachments/${encoded(attachmentId, 'attachmentId')}`, signal }));
    },
    async remove(sessionId: string, attachmentId: string, signal?: AbortSignal): Promise<void> {
      await request({ method: 'DELETE', path: `/api/v1/sessions/${encoded(sessionId, 'sessionId')}/attachments/${encoded(attachmentId, 'attachmentId')}`, signal });
    },
  };
}

export type ChatAttachmentsApi = ReturnType<typeof createChatAttachmentsApi>;
