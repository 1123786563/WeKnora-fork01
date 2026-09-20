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

  const input = container.querySelector('input[type="range"]') as HTMLInputElement;
  assert.ok(input, 'retrieval control should render');
  await act(async () => {
    Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set?.call(input, '51');
    input.dispatchEvent(new window.Event('input', { bubbles: true }));
    input.dispatchEvent(new window.Event('change', { bubbles: true }));
  });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 550)); });
  assert.equal(calls.length, 1);
  assert.equal(calls[0]?.embedding_top_k, 51);
});

test('retrieval keeps the Vue rerank-model-first slider order', async () => {
  const client = { settings: { retrieval: { update: async (body: Record<string, unknown>) => body } } } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ConfigSettingsPanel client={client} section="retrieval" initialValue={{}} models={[{ id: 'rerank-1', name: 'Rerank' }]} />));

  const model = container.querySelector('[data-testid="rerank_model_id"]');
  const slider = container.querySelector('input[type="range"]');
  assert.ok(model, 'Vue renders the rerank selector first');
  assert.ok(slider, 'Vue retrieval thresholds use sliders rather than number steppers');
  assert.ok(Boolean(model.compareDocumentPosition(slider) & 4), 'the rerank selector precedes the threshold sliders');
});

// R483 F2 (R482 B2-D18): Vue RetrievalSettings.vue:148-155 treats a stored 0
// as "unset" for embedding_top_k / vector_threshold / keyword_threshold /
// rerank_top_k (`cfg.x || default`) but keeps 0 for rerank_threshold
// (`cfg.x ?? default`, the slider spans -10..10). React read the raw values,
// so an all-zero tenant config rendered 0.00/0.00 instead of 0.15/0.30.
test('retrieval falls back to the Vue defaults when the tenant config stores zeros', async () => {
  const client = { settings: { retrieval: { update: async (body: Record<string, unknown>) => body } } } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(
    <ConfigSettingsPanel
      client={client}
      section="retrieval"
      initialValue={{ embedding_top_k: 0, vector_threshold: 0, keyword_threshold: 0, rerank_top_k: 0, rerank_threshold: 0 }}
      models={[]}
    />,
  ));

  const outputs = [...container.querySelectorAll('output')].map((node) => node.textContent);
  assert.deepEqual(outputs, ['50', '0.15', '0.30', '10', '0.00'], 'zeros fall back per-field: embedding 50, vector 0.15, keyword 0.30, rerank top_k 10, rerank threshold stays 0.00 like Vue ??');
});

test('retrieval preserves non-zero and negative stored values like Vue', async () => {
  const client = { settings: { retrieval: { update: async (body: Record<string, unknown>) => body } } } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(
    <ConfigSettingsPanel
      client={client}
      section="retrieval"
      initialValue={{ embedding_top_k: 30, vector_threshold: 0.4, keyword_threshold: 0.65, rerank_top_k: 5, rerank_threshold: -1.5 }}
      models={[]}
    />,
  ));

  const outputs = [...container.querySelectorAll('output')].map((node) => node.textContent);
  assert.deepEqual(outputs, ['30', '0.40', '0.65', '5', '-1.50']);
});

test('chat history hides the embedding model row while indexing is disabled like Vue', async () => {
  const client = { settings: { chatHistory: { config: { update: async (body: Record<string, unknown>) => body } } } } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(
    <ConfigSettingsPanel
      client={client}
      section="chathistory"
      initialValue={{ enabled: false, embedding_model_id: 'embed-1' }}
      models={[{ id: 'embed-1', name: 'Embedding' }]}
    />,
  ));

  assert.equal(container.querySelector('[data-testid="embedding_model_id"]'), null);
});

test('parser exposes the Vue MinerU and PaddleOCR configuration controls', async () => {
  const client = { settings: { parser: { config: { update: async (body: Record<string, unknown>) => body, }, check: async () => ({ connected: true }) } } } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ConfigSettingsPanel client={client} section="parser" initialValue={{}} />));

  for (const field of ['mineru-model', 'mineru-vllm-server-url', 'mineru-parse-method', 'mineru-language', 'mineru-cloud-model', 'paddleocr-vl-endpoint', 'paddleocr-vl-cloud-model']) {
    assert.ok(container.querySelector(`[data-testid="${field}"]`), `${field} should be configurable like ParserEngineSettings.vue`);
  }
});

// R490 B6 (R489 D18) — the retrieval surface mirrors Vue RetrievalSettings.vue:
// the section-header carries the 搜索设置 title plus the
// 配置知识库搜索和消息搜索的全局检索参数 description, and like the debounced
// save flow it renders no 保存 button (only the parser section keeps one).
test('retrieval renders the Vue section header and hides the save button', async () => {
  const client = { settings: { retrieval: { update: async (body: Record<string, unknown>) => body } } } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ConfigSettingsPanel client={client} section="retrieval" initialValue={{ embedding_top_k: 50 }} models={[]} />));

  const header = container.querySelector('.section-header');
  assert.ok(header, 'Vue section-header block rendered');
  assert.ok(header.textContent?.includes('搜索设置'), 'h2 title (retrievalSettings.title)');
  assert.ok(container.textContent?.includes('配置知识库搜索和消息搜索的全局检索参数'), 'description under the section title (retrievalSettings.description)');
  assert.equal(container.querySelector('[data-testid="config-save"]'), null, 'Vue saves debounced without a button');
});

test('parser keeps its explicit save button', async () => {
  const client = { settings: { parser: { config: { update: async (body: Record<string, unknown>) => body }, check: async () => ({ connected: true }) } } } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ConfigSettingsPanel client={client} section="parser" initialValue={{}} />));

  assert.ok(container.querySelector('[data-testid="config-save"]'), 'parser section still saves explicitly');
});
