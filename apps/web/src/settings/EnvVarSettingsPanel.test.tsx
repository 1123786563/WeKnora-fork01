import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document });

const { EnvVarSettingsPanel } = await import('./EnvVarSettingsPanel.tsx');

test('does not render stored environment-variable values in the page markup', () => {
  const html = renderToStaticMarkup(React.createElement(EnvVarSettingsPanel, {
    client: {} as never,
    initialPayload: [{ skill_id: 'skill-1', name: 'API_TOKEN', value: 'secret-value' }],
  }));

  assert.match(html, /API_TOKEN/);
  assert.doesNotMatch(html, /secret-value/);
  assert.match(html, /type="password"/);
});
