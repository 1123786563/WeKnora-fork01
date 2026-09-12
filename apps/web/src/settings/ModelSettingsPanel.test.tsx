import assert from 'node:assert/strict';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import * as nodeModule from 'node:module';
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);

// The component is imported after the CSS resolver so SSR exercises the same
// module boundary as the existing Web component tests.
(globalThis as typeof globalThis & { React: typeof React }).React = React;
const { ModelSettingsPanel } = await import('./ModelSettingsPanel.tsx');
const client = {} as never;

test('model settings keeps the Vue read-only empty state for a viewer', () => {
  const html = renderToStaticMarkup(React.createElement(ModelSettingsPanel, { client, role: 'viewer', initialModels: [] }));
  assert.match(html, /No models configured/);
  assert.doesNotMatch(html, /Add model/);
});

test('model settings renders typed model cards and admin actions', () => {
  const html = renderToStaticMarkup(React.createElement(ModelSettingsPanel, {
    client,
    role: 'admin',
    initialModels: [{ id: 'model-1', name: 'embed-small', type: 'EmbeddingModel', source: 'remote', parameters: { provider: 'openai', dimension: 1536 } }],
  }));
  assert.match(html, /Embedding/);
  assert.match(html, /embed-small/);
  assert.match(html, /dimension 1536/);
  assert.match(html, /Add model/);
  assert.match(html, /Edit/);
  assert.match(html, /Delete/);
});
