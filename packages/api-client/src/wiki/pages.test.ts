import assert from 'node:assert/strict';
import test from 'node:test';

import { createWikiPagesApi } from './pages.ts';

const wikiPage = { id: 'p-1', slug: 'docs/start', title: 'Start', content: 'Body', summary: 'Intro', version: 2 };

test('lists Wiki pages with encoded KB and query parameters', async () => {
  let path = '';
  const api = createWikiPagesApi(async (request) => {
    path = request.path;
    return { pages: [wikiPage], total: 1, page: 1, page_size: 20, total_pages: 1 };
  });
  assert.equal((await api.list('kb/a', { page: 1, page_size: 20 })).pages[0]?.slug, 'docs/start');
  assert.equal(path, '/api/v1/knowledgebase/kb%2Fa/wiki/pages?page=1&page_size=20');
});

test('updates hierarchical slugs segment-by-segment and preserves optimistic version', async () => {
  let captured: { path: string; body: unknown } | undefined;
  const api = createWikiPagesApi(async (request) => {
    captured = { path: request.path, body: request.body };
    return wikiPage;
  });
  await api.update('kb-1', 'docs/my page', { content: 'New', version: 2 });
  assert.equal(captured?.path, '/api/v1/knowledgebase/kb-1/wiki/pages/docs/my%20page');
  assert.deepEqual(captured?.body, { content: 'New', version: 2 });
});

test('rejects a page response without the required version', async () => {
  const api = createWikiPagesApi(async () => ({ ...wikiPage, version: 0 }));
  await assert.rejects(api.get('kb-1', 'start'), /Invalid Wiki page version/);
});

test('loads a full historical revision with its version-specific query', async () => {
  let path = '';
  const api = createWikiPagesApi(async (request) => { path = request.path; return { id: 'r-1', slug: 'docs/start', title: 'Start', summary: 'Old', content: 'Old body', version: 1, edit_source: 'user', edited_at: '2026-09-10T00:00:00Z' }; });
  const revision = await api.getRevision('kb-1', 'docs/start', 1);
  assert.equal(revision.content, 'Old body');
  assert.equal(path, '/api/v1/knowledgebase/kb-1/wiki/revisions/docs/start?version=1');
});
