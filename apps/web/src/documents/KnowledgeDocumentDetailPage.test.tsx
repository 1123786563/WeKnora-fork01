import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import test from 'node:test';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register('data:text/javascript,' + encodeURIComponent([
  'export async function resolve(specifier, context, nextResolve) {',
  "  if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };",
  '  return nextResolve(specifier, context);',
  '}',
].join('\n')), import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const { KnowledgeDocumentDetailPage } = await import('./KnowledgeDocumentDetailPage.tsx');

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
