import assert from 'node:assert/strict';
import test from 'node:test';
import * as React from 'react';
import * as nodeModule from 'node:module';
import { renderToStaticMarkup } from 'react-dom/server';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => (specifier.endsWith('.css') || specifier.endsWith('.svg')) ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);

const { ResourceSettingsPanel } = await import('./ResourceSettingsPanel.tsx');

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document });

const client = { settings: { webSearch: { providers: {
  list: async () => [], create: async () => ({}), update: async () => ({}), remove: async () => ({}), testById: async () => ({ success: true }),
} } } } as never;

test('WebSearch provider cards preserve Vue metadata anatomy', () => {
  const html = renderToStaticMarkup(React.createElement(ResourceSettingsPanel, {
    client,
    section: 'websearch',
    initialValue: [{
      id: 'provider-1',
      name: 'Docs Search',
      provider: 'tavily',
      description: 'Search public documentation',
      parameters: { proxy_url: 'https://proxy.example.test' },
    }],
  }));

  // Vue StorageBackendSettings backend-card anatomy shared by all three
  // resource surfaces: badge initial, name, provider·meta subtitle, and the
  // dashed add card.
  assert.match(html, /backend-card/);
  assert.match(html, /Docs Search/);
  assert.match(html, /TAVILY/);
  assert.match(html, /Search public documentation/);
  assert.match(html, /role="button"/);
  assert.match(html, /backend-card--add/);
  assert.match(html, /添加搜索引擎/);
});

test('WebSearch provider cards open the edit drawer instead of inline actions', () => {
  const html = renderToStaticMarkup(React.createElement(ResourceSettingsPanel, {
    client,
    section: 'websearch',
    initialValue: [{ id: 'provider-1', name: 'Docs Search', provider: 'tavily' }],
  }));

  // Vue keeps card actions inside the drawer (create/edit titles) — the
  // card itself is a button; no always-visible 编辑/删除 labels.
  assert.doesNotMatch(html, /aria-label="编辑 Docs Search"/);
  assert.doesNotMatch(html, /aria-label="删除 Docs Search"/);
  assert.match(html, /添加搜索引擎/);
});
