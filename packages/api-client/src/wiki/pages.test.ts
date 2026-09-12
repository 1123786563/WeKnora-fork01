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

test('loads and strictly parses a bounded Wiki graph query', async () => {
  let path = '';
  const api = createWikiPagesApi(async (request) => {
    path = request.path;
    return {
      nodes: [{ slug: 'docs/start', title: 'Start', page_type: 'summary', link_count: 2, familiar: true }],
      edges: [{ source: 'docs/start', target: 'docs/next' }],
      meta: { mode: 'ego', total: 2, returned: 1, truncated: true, center: 'docs/start', depth: 2, familiar_count: 1 },
    };
  });
  const graph = await api.graph('kb/a', { mode: 'ego', center: 'docs/start', depth: 2, types: ['summary', 'entity'], limit: 50 });
  assert.equal(graph.nodes[0]?.familiar, true);
  assert.equal(graph.edges[0]?.target, 'docs/next');
  assert.equal(path, '/api/v1/knowledgebase/kb%2Fa/wiki/graph?mode=ego&center=docs%2Fstart&depth=2&types=summary%2Centity&limit=50');
});

test('rejects malformed Wiki graph rows instead of rendering fabricated nodes', async () => {
  const api = createWikiPagesApi(async () => ({
    nodes: [{ slug: 'docs/start', title: 'Start', page_type: 'summary', link_count: -1 }],
    edges: [],
    meta: { mode: 'overview', total: 1, returned: 1, truncated: false },
  }));
  await assert.rejects(api.graph('kb-1'), /Invalid Wiki graph node link_count/);
});

test('rejects unsafe Wiki revision pagination and versions', async () => {
  const invalidPagination = createWikiPagesApi(async () => ({ revisions: [], total: 1.5, current_version: 1 }));
  await assert.rejects(invalidPagination.revisions('kb-1', 'docs/start'), /Invalid Wiki revision pagination/);

  const invalidVersion = createWikiPagesApi(async () => ({ id: 'r-1', slug: 'docs/start', title: 'Start', summary: 'Old', version: -1 }));
  await assert.rejects(invalidVersion.getRevision('kb-1', 'docs/start', 1), /Invalid Wiki revision version/);
});
