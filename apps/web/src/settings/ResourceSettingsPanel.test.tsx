import assert from 'node:assert/strict';
import test from 'node:test';
import * as React from 'react';
import * as nodeModule from 'node:module';
import { renderToStaticMarkup } from 'react-dom/server';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
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

  assert.match(html, /provider-card/);
  assert.match(html, /Docs Search/);
  assert.match(html, /tavily/);
  assert.match(html, /Search public documentation/);
  assert.match(html, /https:\/\/proxy\.example\.test/);
});

test('WebSearch provider cards expose Vue admin actions and add affordance', () => {
  const html = renderToStaticMarkup(React.createElement(ResourceSettingsPanel, {
    client,
    section: 'websearch',
    initialValue: [{ id: 'provider-1', name: 'Docs Search', provider: 'tavily' }],
  }));

  assert.match(html, /aria-label="编辑 Docs Search"/);
  assert.match(html, /aria-label="删除 Docs Search"/);
  assert.match(html, /provider-card--add/);
});
