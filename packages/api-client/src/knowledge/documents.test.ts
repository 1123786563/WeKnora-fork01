import assert from 'node:assert/strict';
import test from 'node:test';

import { ApiError } from '../errors.ts';
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

test('preserves a structured duplicate code when the upload returns success false', async () => {
  const api = createKnowledgeDocumentsApi(async () => ({
    success: false,
    error: { code: 'duplicate_file', message: 'document already exists' },
  }));
  await assert.rejects(
    api.upload('kb-1', { file: new Blob(['hello']) }),
    (error: unknown) => error instanceof ApiError
      && error.code === 'duplicate_file'
      && error.message === 'document already exists',
  );
});

test('preserves duplicate code nested in a structured upload response body', async () => {
  const api = createKnowledgeDocumentsApi(async () => ({
    success: false,
    status: 409,
    body: { error: { code: 'duplicate_file', message: 'document already exists' } },
  }));
  await assert.rejects(
    api.upload('kb-1', { file: new Blob(['hello']) }),
    (error: unknown) => error instanceof ApiError
      && error.code === 'duplicate_file'
      && error.status === 409
      && error.message === 'document already exists',
  );
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

test('keeps a native file URI unconverted on the nativeFile seam without building multipart form data', async () => {
  let captured: { nativeFile?: unknown; body?: unknown; multipartFields?: Record<string, string> } | undefined;
  const originalFormData = globalThis.FormData;
  class ForbiddenFormData {
    constructor() { throw new Error('FormData must not be constructed for native file uploads'); }
  }
  globalThis.FormData = ForbiddenFormData as unknown as typeof FormData;
  const api = createKnowledgeDocumentsApi(async (request) => {
    captured = { nativeFile: request.nativeFile, body: request.body, multipartFields: request.multipartFields };
    return { success: true, data: { id: 'doc-native', parse_status: 'pending' } };
  });
  try {
    await api.upload('kb-1', { file: { uri: 'content://picker/large.pdf', name: 'large.pdf', type: 'application/pdf', size: 4_000_000 } });
    assert.deepEqual(captured?.nativeFile, { uri: 'content://picker/large.pdf', name: 'large.pdf', type: 'application/pdf', size: 4_000_000 });
    assert.equal(captured?.body, undefined);
  } finally {
    globalThis.FormData = originalFormData;
  }
});

test('loads folders, tags, detail, search, and creates an authenticated download path', async () => {
  const requests: string[] = [];
  const api = createKnowledgeDocumentsApi(async (request) => {
    requests.push(request.path);
    if (request.path.includes('/folders')) return { success: true, data: { root_document_count: 1, total_document_count: 2, folders: [{ path: 'docs', name: 'docs', document_count: 1, total_count: 1 }] } };
    if (request.path.includes('/tags')) return { success: true, data: { data: [{ id: 'tag-1', seq_id: 3, name: 'important' }], total: 51, page: 1, page_size: 50 } };
    if (request.path.includes('/search')) return { success: true, data: [{ id: 'doc-1', title: 'Guide' }], has_more: false, total: 1 };
    return { success: true, data: { id: 'doc-1', title: 'Guide' } };
  });
  assert.equal((await api.folders('kb/a')).folders[0]?.path, 'docs');
  assert.equal((await api.tags('kb/a', { keyword: 'imp' }))[0]?.name, 'important');
  assert.equal((await api.tagsPage('kb/a', { page: 1, page_size: 50, keyword: 'imp' })).total, 51);
  assert.equal((await api.get('doc/a', { agent_id: 'agent-1' })).title, 'Guide');
  assert.equal((await api.search({ keyword: 'guide', file_types: ['pdf', 'md'], limit: 10 })).total, 1);
  assert.equal(api.downloadPath('doc/a'), '/api/v1/knowledge/doc%2Fa/download');
  assert.deepEqual(requests, [
    '/api/v1/knowledge-bases/kb%2Fa/knowledge/folders',
    '/api/v1/knowledge-bases/kb%2Fa/tags?keyword=imp',
    '/api/v1/knowledge-bases/kb%2Fa/tags?page=1&page_size=50&keyword=imp',
    '/api/v1/knowledge/doc%2Fa?agent_id=agent-1',
    '/api/v1/knowledge/search?keyword=guide&limit=10&file_types=pdf%2Cmd',
  ]);
});

test('fetches preview and download bytes through the authenticated binary request seam', async () => {
  const requests: Array<{ method: string; path: string; signal?: AbortSignal }> = [];
  const api = createKnowledgeDocumentsApi(async (request) => {
    throw new Error(`JSON request was not expected: ${request.path}`);
  }, async (request) => {
    requests.push(request);
    return {
      body: new Blob(['# private guide\n'], { type: 'text/markdown' }),
      contentType: 'text/markdown',
      headers: { 'content-type': 'text/markdown', 'x-request-id': 'binary-1' },
    };
  });
  const controller = new AbortController();

  const preview = await api.preview('doc/a', controller.signal);
  const download = await api.download('doc/a');

  assert.equal(await (preview.body as Blob).text(), '# private guide\n');
  assert.equal(download.contentType, 'text/markdown');
  assert.deepEqual(requests.map((request) => ({ method: request.method, path: request.path })), [
    { method: 'GET', path: '/api/v1/knowledge/doc%2Fa/preview' },
    { method: 'GET', path: '/api/v1/knowledge/doc%2Fa/download' },
  ]);
  assert.equal(requests[0]?.signal, controller.signal);
});

test('batchDownload posts the selected ids and returns the ZIP blob', async () => {
  // Upstream api/knowledge-base/index.ts batchDownloadKnowledge: POST
  // /knowledge-bases/:id/knowledge/batch-download with {ids}, responseType
  // blob — credentials stay in headers, never in a download link.
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createKnowledgeDocumentsApi(async () => {
    throw new Error('JSON request was not expected');
  }, async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    return {
      body: new Blob(['zip-bytes'], { type: 'application/zip' }),
      contentType: 'application/zip',
      headers: { 'content-type': 'application/zip' },
    };
  });

  const zip = await api.batchDownload('kb/1', ['doc-1', 'doc-2']);

  assert.equal(zip.contentType, 'application/zip');
  assert.deepEqual(requests, [{
    method: 'POST',
    path: '/api/v1/knowledge-bases/kb%2F1/knowledge/batch-download',
    body: { ids: ['doc-1', 'doc-2'] },
  }]);
});

test('supports URL/manual sources and guarded document mutations', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createKnowledgeDocumentsApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    if (request.method === 'DELETE') return undefined;
    if (request.path.endsWith('/preview')) return undefined;
    return { success: true, data: { id: 'doc-2', parse_status: 'pending' } };
  });
  await api.createFromUrl('kb-1', { url: 'https://example.test/a', tag_ids: ['tag-1'] });
  await api.createManual('kb-1', { title: 'Manual', content: '# body', status: 'pending' });
  await api.updateManual('doc-2', { title: 'Manual v2', content: '# updated', status: 'draft' });
  await api.moveToFolder('kb-1', ['doc-2'], 'docs/spec');
  await api.renameFolder('kb-1', 'docs', 'archive');
  await api.updateTags({ 'doc-2': ['tag-1'] });
  await api.reparse('doc-2');
  await api.cancelParse('doc-2');
  await api.remove('doc-2');
  await api.batchDelete('kb-1', ['doc-2']);
  assert.equal(api.previewPath('doc-2'), '/api/v1/knowledge/doc-2/preview');
  assert.deepEqual(requests.map(({ method, path }) => `${method} ${path}`), [
    'POST /api/v1/knowledge-bases/kb-1/knowledge/url',
    'POST /api/v1/knowledge-bases/kb-1/knowledge/manual',
    'PUT /api/v1/knowledge/manual/doc-2',
    'POST /api/v1/knowledge/folder',
    'PUT /api/v1/knowledge-bases/kb-1/knowledge/folders',
    'PUT /api/v1/knowledge/tags',
    'POST /api/v1/knowledge/doc-2/reparse',
    'POST /api/v1/knowledge/doc-2/cancel-parse',
    'DELETE /api/v1/knowledge/doc-2',
    'POST /api/v1/knowledge/batch-delete',
  ]);
  assert.deepEqual(requests[3]?.body, { kb_id: 'kb-1', knowledge_ids: ['doc-2'], folder_path: 'docs/spec' });
  assert.deepEqual(requests[5]?.body, { updates: { 'doc-2': ['tag-1'] } });
});

test('keeps knowledge-base tag CRUD on the Vue endpoint contract', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createKnowledgeDocumentsApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    return { success: true, data: [] };
  });
  await api.createTag('kb/a', { name: '重要' });
  await api.updateTag('kb/a', 'tag/1', { name: '重点' });
  await api.deleteTag('kb/a', 3, true);
  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/knowledge-bases/kb%2Fa/tags', body: { name: '重要' } },
    { method: 'PUT', path: '/api/v1/knowledge-bases/kb%2Fa/tags/tag%2F1', body: { name: '重点' } },
    { method: 'DELETE', path: '/api/v1/knowledge-bases/kb%2Fa/tags/3?force=true', body: undefined },
  ]);
});

test('keeps document chunk paging and revision mutations on the Vue endpoint contract', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createKnowledgeDocumentsApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    if (request.path.endsWith('/revisions')) return { success: true, data: [{ revision: 2, content: 'old' }] };
    if (request.method === 'GET') return { success: true, data: [{ id: 'chunk-1', content: 'current', content_revision: 3 }], total: 1, page: 2, page_size: 25 };
    return { success: true, data: { id: 'chunk-1', content: 'updated', content_revision: 4 } };
  });

  const page = await api.chunks('doc/a', 2);
  const revisions = await api.chunkRevisions('doc/a', 'chunk/1');
  await api.updateChunk('doc/a', 'chunk/1', { content: 'updated', expected_revision: 3 });
  await api.revertChunk('doc/a', 'chunk/1', 2, 4);

  assert.equal(page.data[0]?.id, 'chunk-1');
  assert.equal(revisions[0]?.revision, 2);
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/chunks/doc%2Fa?page=2&page_size=25', body: undefined },
    { method: 'GET', path: '/api/v1/chunks/doc%2Fa/chunk%2F1/revisions', body: undefined },
    { method: 'PUT', path: '/api/v1/chunks/doc%2Fa/chunk%2F1', body: { content: 'updated', expected_revision: 3 } },
    { method: 'POST', path: '/api/v1/chunks/doc%2Fa/chunk%2F1/revert', body: { revision: 2, expected_revision: 4 } },
  ]);
});

test('loads a single chunk through the Vue by-id endpoint contract', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createKnowledgeDocumentsApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    return { success: true, data: { id: 'parent-1', content: '# Parent chunk' } };
  });

  const chunk = await api.getChunkById('chunk/1');

  assert.equal(chunk.id, 'parent-1');
  assert.equal(chunk.content, '# Parent chunk');
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/chunks/by-id/chunk%2F1', body: undefined },
  ]);
});

test('rejects a by-id chunk envelope without a usable id', async () => {
  const api = createKnowledgeDocumentsApi(async () => ({ success: true, data: { content: 'no id' } }));
  await assert.rejects(() => api.getChunkById('chunk-1'), /Invalid knowledge chunk/);
  const nullData = createKnowledgeDocumentsApi(async () => ({ success: true }));
  await assert.rejects(() => nullData.getChunkById('chunk-1'), /Invalid knowledge chunk/);
});

test('keeps generated-question mutations on the Vue by-id questions endpoint contract', async () => {
  // R471/A3 — Vue api/knowledge-base upsert/delete/regenerateGeneratedQuestion
  // (frontend/src/api/knowledge-base/index.ts) against handler chunk.go:
  // UpsertGeneratedQuestion/RegenerateGeneratedQuestions reply {success,data},
  // DeleteGeneratedQuestion replies {success,message} and reads question_id
  // from the JSON body (ShouldBindJSON), not a query param.
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createKnowledgeDocumentsApi(async (request) => {
    requests.push({ method: request.method, path: request.path, body: request.body });
    if (request.path.endsWith('/regenerate')) return { success: true, data: [{ id: 'q2', question: 'Regenerated Q', content_revision: 3 }] };
    if (request.method === 'DELETE') return { success: true, message: 'Generated question deleted' };
    return { success: true, data: { id: 'q1', question: 'Saved Q', content_revision: 3 } };
  });

  const saved = await api.upsertGeneratedQuestion('chunk/1', 'Saved Q');
  await api.upsertGeneratedQuestion('chunk/1', 'Edited Q', 'q1');
  await api.deleteGeneratedQuestion('chunk/1', 'q1');
  const regenerated = await api.regenerateGeneratedQuestions('chunk/1');

  assert.equal(saved.id, 'q1');
  assert.equal(saved.content_revision, 3);
  assert.deepEqual(regenerated.map((question) => question.id), ['q2']);
  assert.deepEqual(requests, [
    { method: 'PUT', path: '/api/v1/chunks/by-id/chunk%2F1/questions', body: { question_id: '', question: 'Saved Q' } },
    { method: 'PUT', path: '/api/v1/chunks/by-id/chunk%2F1/questions', body: { question_id: 'q1', question: 'Edited Q' } },
    { method: 'DELETE', path: '/api/v1/chunks/by-id/chunk%2F1/questions', body: { question_id: 'q1' } },
    { method: 'POST', path: '/api/v1/chunks/by-id/chunk%2F1/questions/regenerate', body: {} },
  ]);
});

test('rejects generated-question envelopes without usable data', async () => {
  const api = createKnowledgeDocumentsApi(async () => ({ success: true }));
  await assert.rejects(() => api.upsertGeneratedQuestion('c1', 'Q'), /Invalid generated question/);
  await assert.rejects(() => api.regenerateGeneratedQuestions('c1'), /Invalid generated questions/);
  const notArray = createKnowledgeDocumentsApi(async () => ({ success: true, data: { id: 'q1' } }));
  await assert.rejects(() => notArray.regenerateGeneratedQuestions('c1'), /Invalid generated questions/);
});

test('updates document summary and custom metadata through the guarded detail endpoint', async () => {
  let request: { method: string; path: string; body?: unknown } | undefined;
  const api = createKnowledgeDocumentsApi(async (input) => {
    request = { method: input.method, path: input.path, body: input.body };
    return { success: true, data: { id: 'doc-1', description: 'Updated', custom_metadata: { owner: 'docs' } } };
  });
  const result = await api.updateDetails('doc-1', { description: 'Updated', custom_metadata: { owner: 'docs' } });
  assert.equal(result.description, 'Updated');
  assert.deepEqual(request, {
    method: 'PUT',
    path: '/api/v1/knowledge/doc-1',
    body: { description: 'Updated', custom_metadata: { owner: 'docs' } },
  });
});
