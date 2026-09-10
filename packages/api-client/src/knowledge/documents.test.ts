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

test('keeps a native file URI inside multipart input without converting it to a shared DTO', async () => {
  let captured: { value?: unknown } | undefined;
  const originalFormData = globalThis.FormData;
  class NativeFormDataCapture {
    append(_name: string, value: unknown) { captured = { value }; }
  }
  globalThis.FormData = NativeFormDataCapture as unknown as typeof FormData;
  const api = createKnowledgeDocumentsApi(async (request) => {
    return { success: true, data: { id: 'doc-native', parse_status: 'pending' } };
  });
  try {
    await api.upload('kb-1', { file: { uri: 'content://picker/large.pdf', name: 'large.pdf', type: 'application/pdf', size: 4_000_000 } });
    assert.deepEqual(captured?.value, { uri: 'content://picker/large.pdf', name: 'large.pdf', type: 'application/pdf', size: 4_000_000 });
  } finally {
    globalThis.FormData = originalFormData;
  }
});

test('loads folders, tags, detail, search, and creates an authenticated download path', async () => {
  const requests: string[] = [];
  const api = createKnowledgeDocumentsApi(async (request) => {
    requests.push(request.path);
    if (request.path.includes('/folders')) return { success: true, data: { root_document_count: 1, total_document_count: 2, folders: [{ path: 'docs', name: 'docs', document_count: 1, total_count: 1 }] } };
    if (request.path.includes('/tags')) return { success: true, data: [{ id: 'tag-1', seq_id: 3, name: 'important' }] };
    if (request.path.includes('/search')) return { success: true, data: [{ id: 'doc-1', title: 'Guide' }], has_more: false, total: 1 };
    return { success: true, data: { id: 'doc-1', title: 'Guide' } };
  });
  assert.equal((await api.folders('kb/a')).folders[0]?.path, 'docs');
  assert.equal((await api.tags('kb/a', { keyword: 'imp' }))[0]?.name, 'important');
  assert.equal((await api.get('doc/a', { agent_id: 'agent-1' })).title, 'Guide');
  assert.equal((await api.search({ keyword: 'guide', file_types: ['pdf', 'md'], limit: 10 })).total, 1);
  assert.equal(api.downloadPath('doc/a'), '/api/v1/knowledge/doc%2Fa/download');
  assert.deepEqual(requests, [
    '/api/v1/knowledge-bases/kb%2Fa/knowledge/folders',
    '/api/v1/knowledge-bases/kb%2Fa/tags?keyword=imp',
    '/api/v1/knowledge/doc%2Fa?agent_id=agent-1',
    '/api/v1/knowledge/search?keyword=guide&limit=10&file_types=pdf%2Cmd',
  ]);
});
