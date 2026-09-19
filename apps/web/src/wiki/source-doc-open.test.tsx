import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
hooks.registerHooks?.({ resolve: (specifier, context, nextResolve) => (specifier.endsWith('.css') || specifier.endsWith('.svg')) ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

// A real URL is required: opaque origins make jsdom's localStorage throw.
import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/' });
(globalThis as typeof globalThis & { window: unknown; document: unknown }).window = dom.window;
(globalThis as typeof globalThis & { window: unknown; document: unknown }).document = dom.window.document;

import * as React from 'react';
import { createRoot } from 'react-dom/client';

const { wikiSourceDocPath, createWikiSourceDocOpener } = await import('./source-doc-open.ts');
const { WikiReaderFooter } = await import('./WikiPage.tsx');

// ─── R457 A3: host wiring for the Vue `open-source-doc` emit ───
//
// WikiBrowser.vue:607 `@click.prevent="emit('open-source-doc', ref.id)"` and
// KnowledgeBase.vue:1431 `openSourceDoc(knowledgeId)` open the clicked source
// document for viewing. The React host's established "view a document"
// affordance is the document-detail route (/knowledgeBase/:kbId/documents/:docId,
// routes.tsx kind 'knowledge-document' — the same target the documents surface
// navigates to via onOpenDocument). The mount point (main.tsx) is owned by
// another agent this round, so the wiring helper lives here in wiki/** and the
// host passes it as the onOpenSourceDoc prop in one line.

test('wikiSourceDocPath builds the host document-detail route', () => {
  assert.equal(wikiSourceDocPath('kb-1', 'doc-1'), '/knowledgeBase/kb-1/documents/doc-1');
  assert.equal(
    wikiSourceDocPath('kb id/x', 'doc id/y'),
    `/knowledgeBase/${encodeURIComponent('kb id/x')}/documents/${encodeURIComponent('doc id/y')}`,
    'route segments are URL-encoded like the host navigate calls',
  );
});

test('createWikiSourceDocOpener opens the clicked source document via the host route', () => {
  const navigated: string[] = [];
  const navigate = (path: string) => navigated.push(path);
  const openSourceDoc = createWikiSourceDocOpener({ knowledgeBaseId: 'kb-1', navigate });
  openSourceDoc('012345678901234567890');
  openSourceDoc('doc-2');
  assert.deepEqual(navigated, [
    '/knowledgeBase/kb-1/documents/012345678901234567890',
    '/knowledgeBase/kb-1/documents/doc-2',
  ], 'each clicked source ref opens its document-detail route');
});

test('clicking a footer source link triggers the host navigation', async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const navigated: string[] = [];
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  await React.act(async () => {
    root.render(
      React.createElement(WikiReaderFooter, {
        page: { source_refs: ['doc-1|报告.pdf'] },
        resolveSlugName: (slug: string) => slug,
        onNavigate: () => {},
        onOpenSourceDoc: createWikiSourceDocOpener({ knowledgeBaseId: 'kb-1', navigate: (path) => navigated.push(path) }),
      }),
    );
  });
  const link = container.querySelector<HTMLAnchorElement>('a[data-source-id="doc-1"]');
  assert.ok(link, 'the source document link renders in the footer');
  await React.act(async () => {
    link!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true }));
  });
  assert.deepEqual(navigated, ['/knowledgeBase/kb-1/documents/doc-1'], 'the click opens the source document');
  await React.act(async () => {
    root.unmount();
  });
  container.remove();
});
