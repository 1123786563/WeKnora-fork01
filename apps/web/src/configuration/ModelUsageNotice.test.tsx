import assert from 'node:assert/strict';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import * as nodeModule from 'node:module';
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`export async function resolve(specifier, context, nextResolve) { if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' }; return nextResolve(specifier, context); }`)}`, import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;
const { ModelUsageNotice } = await import('./ModelUsageNotice.tsx');

test('renders model deletion bindings and totals', () => {
  const html = renderToStaticMarkup(React.createElement(ModelUsageNotice, {
    modelName: 'Embedding model',
    details: {
      knowledge_bases: [{ id: 'kb-1', name: 'Docs', bindings: ['embedding_model'] }],
      agents: [{ id: 'agent-1', name: 'Support', bindings: ['chat_model'] }],
      long_term_memory: { bindings: ['embedding_model'] },
      knowledge_base_total: 2,
      agent_total: 1,
    },
    onClose: () => undefined,
  }));
  assert.match(html, /Model is still in use/);
  assert.match(html, /Knowledge bases \(2\)/);
  assert.match(html, /Showing 1 of 2 knowledge-base bindings/);
  assert.match(html, /embedding model/);
  assert.match(html, /Long-term memory/);
});
