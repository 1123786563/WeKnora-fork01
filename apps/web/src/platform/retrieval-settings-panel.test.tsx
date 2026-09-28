// R481-A3 — retrieval-config load failure in the palette drawer (Vue parity).
// Vue contract (GlobalCommandPalette.vue): the boot prefetch failure is
// silently swallowed and the drawer's RetrievalSettings form renders with
// DEFAULT values (empty rerank model, Top K 50, default thresholds) — no
// error UI, no retry. The React wrapper must therefore degrade
// retrieval.get() failures to the default-value ConfigSettingsPanel
// (initialValue=null) instead of replacing the form with a bare error <p>.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Event: dom.window.Event,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  IS_REACT_ACT_ENVIRONMENT: true,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { PaletteRetrievalSettings } = await import('./retrieval-settings-panel.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

async function mountPanel(client: WeKnoraClient): Promise<HTMLDivElement> {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<PaletteRetrievalSettings client={client} locale="zh-CN" />);
  });
  // Wait for the fetch promise chain plus the lazy ConfigSettingsPanel import.
  await settle(50);
  return container;
}

test('retrieval.get() failure renders the default-value form, not a bare error (R480/R481)', async () => {
  const client = {
    settings: {
      retrieval: {
        get: async () => { throw 'simulated upstream failure'; },
        update: async (body: Record<string, unknown>) => body,
      },
    },
    configuration: { models: { list: async () => [] } },
  } as unknown as WeKnoraClient;
  const container = await mountPanel(client);

  const text = container.textContent ?? '';
  assert.doesNotMatch(text, /simulated upstream failure/, 'the drawer must not surface the raw fetch failure like Vue silently swallows it');
  assert.match(text, /向量检索数量 \(Top K\)/, 'the default retrieval form renders with its Top K slider');
  const outputs = [...container.querySelectorAll('output')].map((node) => node.textContent);
  assert.ok(outputs.includes('50'), `embedding_top_k falls back to the default 50, got ${JSON.stringify(outputs)}`);
  assert.match(text, /Rerank 模型/, 'the rerank model field stays rendered (empty default)');
});

test('retrieval.get() success renders the fetched config values (green-path regression)', async () => {
  const client = {
    settings: {
      retrieval: {
        get: async () => ({ embedding_top_k: 7, vector_threshold: 0.45, rerank_top_k: 12, rerank_threshold: 1.5, rerank_model_id: 'rerank-x' }),
        update: async (body: Record<string, unknown>) => body,
      },
    },
    configuration: { models: { list: async () => [] } },
  } as unknown as WeKnoraClient;
  const container = await mountPanel(client);

  const outputs = [...container.querySelectorAll('output')].map((node) => node.textContent);
  assert.ok(outputs.includes('7'), `embedding_top_k shows the fetched 7, got ${JSON.stringify(outputs)}`);
  assert.ok(outputs.includes('0.45'), `vector_threshold shows the fetched 0.45, got ${JSON.stringify(outputs)}`);
  const rerankInput = container.querySelector('input:not([type="range"])') as HTMLInputElement | null;
  assert.ok(rerankInput, 'models=[] renders the rerank model as a text input');
  assert.equal(rerankInput.value, 'rerank-x');
});
