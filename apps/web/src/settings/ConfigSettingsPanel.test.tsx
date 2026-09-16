import assert from 'node:assert/strict';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');
const { ConfigSettingsPanel } = await import('./ConfigSettingsPanel.tsx');

let root: Root | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

test('retrieval changes auto-save after the Vue 500ms debounce', async () => {
  const calls: Record<string, unknown>[] = [];
  const client = { settings: { retrieval: { update: async (body: Record<string, unknown>) => { calls.push(body); return body; } } } } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ConfigSettingsPanel client={client} section="retrieval" initialValue={{ embedding_top_k: 50 }} models={[]} />));

  const input = container.querySelector('input') as HTMLInputElement;
  assert.ok(input, 'retrieval control should render');
  await act(async () => {
    (container.querySelector('button[aria-label="Increase"]') as HTMLButtonElement).click();
  });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 550)); });
  assert.equal(calls.length, 1);
  assert.equal(calls[0]?.embedding_top_k, 51);
});
