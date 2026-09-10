import assert from 'node:assert/strict';
import test from 'node:test';
import { createChatAttachmentsApi } from './attachments.ts';

test('uploads native chat attachments through the session-scoped multipart route', async () => {
  let captured: { method: string; path: string; body: unknown; signal?: AbortSignal } | undefined;
  const api = createChatAttachmentsApi(async (request) => {
    captured = request;
    return { success: true, data: { id: 'att-1', session_id: 'session-1', file_name: 'notes.md', file_type: 'md', file_size: 12, status: 'processing' } };
  });
  const controller = new AbortController();
  const result = await api.upload('session/a', { file: { uri: 'content://notes', name: 'notes.md', type: 'text/markdown' } }, controller.signal);
  assert.equal(result.status, 'processing');
  assert.equal(captured?.method, 'POST');
  assert.equal(captured?.path, '/api/v1/sessions/session%2Fa/attachments');
  assert.equal(captured?.signal, controller.signal);
});

test('lists and deletes attachment ids without swallowing server boundaries', async () => {
  const paths: string[] = [];
  const api = createChatAttachmentsApi(async (request) => {
    paths.push(`${request.method} ${request.path}`);
    if (request.method === 'GET') return { success: true, data: [] };
    return undefined;
  });
  assert.deepEqual(await api.list('session-1'), []);
  await api.remove('session-1', 'att/a');
  assert.deepEqual(paths, ['GET /api/v1/sessions/session-1/attachments', 'DELETE /api/v1/sessions/session-1/attachments/att%2Fa']);
});
