import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import test, { afterEach } from 'node:test';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => {
  if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
  // @weknora/ui is workspace-linked and Node resolves its real path outside
  // apps/web, so direct node:test runs need the consuming app's React DOM.
  if (specifier === 'react-dom') return { shortCircuit: true, url: new URL('../../node_modules/react-dom/index.js', import.meta.url).href };
  return nextResolve(specifier, context);
};
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register('data:text/javascript,' + encodeURIComponent([
  'export async function resolve(specifier, context, nextResolve) {',
  "  if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };",
  "  if (specifier === 'react-dom') return { shortCircuit: true, url: new URL('../../node_modules/react-dom/index.js', import.meta.url).href };",
  '  return nextResolve(specifier, context);',
  '}',
].join('\n')), import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/knowledgeBase/kb-1/documents/doc-1' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');

const { KnowledgeDocumentDetailPage, knowledgeTraceNodeState } = await import('./KnowledgeDocumentDetailPage.tsx');

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

function detailClient(get: () => Promise<Record<string, unknown>>, role = 'contributor', knowledgeBaseOwnerId = 'user-1') {
  return {
    knowledgeBases: {
      documents: {
        get,
        previewPath: (id: string) => `/api/v1/knowledge/${id}/preview`,
        downloadPath: (id: string) => `/api/v1/knowledge/${id}/download`,
        spans: async () => ({ trace: null }),
        preview: async () => ({ body: '', headers: {}, contentType: 'text/plain' }),
        download: async () => ({ body: '', headers: {}, contentType: 'text/plain' }),
        reparse: async () => undefined,
        cancelParse: async () => undefined,
      },
      settings: { get: async () => ({ id: 'kb-1', user_id: knowledgeBaseOwnerId }) },
    },
    auth: { me: async () => ({ user: { id: 'user-1', role }, memberships: [{ role }] }) },
  } as never;
}

async function mountDetail(client: never, documentId = 'doc-1') {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeDocumentDetailPage client={client} documentId={documentId} onBack={() => {}} />);
  });
  await act(async () => {});
  await act(async () => {});
  return container;
}

test('document trace nodes expose the same status semantics as the Vue trace surface', () => {
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { status: 'failed' } }), 'failed');
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { status: 'running' } }), 'running');
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { status: 'completed' } }), 'done');
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: {} }), 'pending');
});

test('document detail uses the Vue document title-row anatomy', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentDetailPage, {
    client: {} as never,
    documentId: 'doc-1',
    onBack: () => {},
  }));

  assert.ok(html.includes('document-title-row'), 'detail header keeps the Vue title row');
  assert.ok(html.includes('document-breadcrumb'), 'detail header uses breadcrumb anatomy');
  assert.ok(html.includes('文档'), 'detail header keeps the localized document label');
  assert.ok(html.includes('返回'), 'detail header keeps the back action');
  assert.ok(!html.includes('wk-eyebrow'), 'React-only eyebrow is not rendered');
});

test('loaded file detail matches the Vue drawer title and metadata anatomy', async () => {
  const container = await mountDetail(detailClient(async () => ({
    id: 'doc-1',
    knowledge_base_id: 'kb-1',
    file_name: 'Deployment Guide.pdf',
    source: 'file',
    file_type: 'pdf',
    parse_status: 'pending',
    folder_path: 'Guides',
    created_at: '2026-09-15T13:36:00Z',
  })));

  assert.equal(container.querySelector('.breadcrumb-current')?.textContent, 'Deployment Guide', 'Vue removes a file extension from the detail title');
  assert.ok(container.querySelector('.wk-document-detail-surface'), 'detail content uses the grouped drawer-like surface');
  assert.ok(container.querySelector('.wk-document-metadata-section'), 'Vue basic-info section is retained');
  assert.ok(container.textContent?.includes('基本信息'), 'metadata section has the Vue title');
});

test('detail load errors retain navigation and provide a retry without exposing the route document ID', async () => {
  let attempts = 0;
  const container = await mountDetail(detailClient(async () => {
    attempts += 1;
    throw new Error('network unavailable');
  }), 'private-doc-id');

  assert.equal(container.querySelector('[role="alert"]')?.textContent, 'network unavailable');
  assert.equal(container.querySelector('.breadcrumb-current')?.textContent, '文档', 'a failed load does not present the opaque route ID as a title');
  const retry = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '重试');
  assert.ok(retry, 'error state offers the Vue-style retry affordance');
  await act(async () => { retry!.click(); });
  await act(async () => {});
  assert.equal(attempts, 2, 'retry performs a new document request');
});

test('viewer detail keeps Vue download affordances hidden until permission grants access', async () => {
  const container = await mountDetail(detailClient(async () => ({
    id: 'doc-1',
    knowledge_base_id: 'kb-1',
    file_name: 'Viewer Guide.pdf',
    source: 'file',
    file_type: 'pdf',
    parse_status: 'pending',
  }), 'viewer', 'owner-1'));

  assert.equal(Array.from(container.querySelectorAll('button')).some((button) => button.textContent?.includes('下载')), false, 'viewer cannot see the original-file download action');
});
