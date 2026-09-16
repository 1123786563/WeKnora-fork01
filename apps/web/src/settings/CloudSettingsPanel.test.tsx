import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => void }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } as never : nextResolve(specifier, context) });
const { CloudSettingsPanel } = await import('./CloudSettingsPanel.tsx');
const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document });

test('configured cloud credentials remain redacted until the user chooses reconfigure', () => {
  const html = renderToStaticMarkup(React.createElement(CloudSettingsPanel, {
    client: { settings: { weknoraCloud: { status: async () => ({}), saveCredentials: async () => ({}) } } } as never,
    initialValue: { has_models: true, needs_reinit: false },
  }));

  assert.match(html, /已配置/);
  assert.match(html, /重新配置/);
  assert.doesNotMatch(html, /<form/);
  assert.doesNotMatch(html, /value="[^"]+"/);
});
