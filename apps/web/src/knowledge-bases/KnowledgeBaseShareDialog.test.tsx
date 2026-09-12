import assert from 'node:assert/strict';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import test from 'node:test';

const hooks = (await import('node:module')) as unknown as { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
(globalThis as typeof globalThis & { React: typeof React }).React = React;
const { KnowledgeBaseShareDialog } = await import('./KnowledgeBaseShareDialog.tsx');

test('knowledge-base share dialog keeps the Vue share form and list entry points', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeBaseShareDialog, { client: {} as never, knowledgeBaseId: 'kb-1', knowledgeBaseName: 'Docs', open: true, onClose: () => undefined }));
  assert.match(html, /Share Docs/);
  assert.match(html, /Organization/);
  assert.match(html, /Read-only/);
  assert.match(html, /Confirm share/);
  assert.match(html, /Shared to organizations/);
});
