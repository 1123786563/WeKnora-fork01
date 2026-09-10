import assert from 'node:assert/strict';
import test from 'node:test';

import { createKnowledgeDocumentsApi } from './documents.ts';

test('lists documents with encoded KB id and filters', async () => {
  const requests: Array<{ path: string }> = [];
  const api = createKnowledgeDocumentsApi(async (request) => {
    requests.push({ path: request.path });
    return { success: true, data: [], total: 0, page: 1, page_size: 20 };
  });
  const result = await api.list('kb/a', { page: 1, page_size: 20, folder_path: '', folder_recursive: true });
  assert.equal(result.total, 0);
  assert.equal(requests[0]?.path, '/api/v1/knowledge-bases/kb%2Fa/knowledge?page=1&page_size=20&folder_path=&folder_recursive=true');
});

test('builds multipart upload without inventing a JSON content type', async () => {
  let captured: { method: string; path: string; body: unknown } | undefined;
  const api = createKnowledgeDocumentsApi(async (request) => {
    captured = { method: request.method, path: request.path, body: request.body };
    return { success: true, data: { id: 'doc-1', parse_status: 'pending' } };
  });
  const result = await api.upload('kb-1', {
    file: new Blob(['hello'], { type: 'text/plain' }),
    fileName: 'hello.txt',
    tag_ids: ['tag-1', 'tag-2'],
    process_config: { graph_enabled: false },
  });
  assert.equal(result.id, 'doc-1');
  assert.equal(captured?.method, 'POST');
  assert.equal(captured?.path, '/api/v1/knowledge-bases/kb-1/knowledge/file');
  assert.ok(captured?.body instanceof FormData);
  const form = captured?.body as FormData;
  assert.equal(form.get('tag_ids'), 'tag-1,tag-2');
  assert.equal(form.get('process_config'), '{"graph_enabled":false}');
  assert.equal((form.get('file') as File).name, 'hello.txt');
});

test('passes cancellation to the upload request', async () => {
  const controller = new AbortController();
  let signal: AbortSignal | undefined;
  const api = createKnowledgeDocumentsApi(async (request) => {
    signal = request.signal;
    throw new Error('cancelled by transport');
  });
  await assert.rejects(api.upload('kb-1', { file: new Blob(['x']) }, controller.signal), /cancelled/);
  assert.equal(signal, controller.signal);
});
