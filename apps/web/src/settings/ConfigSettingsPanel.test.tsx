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

/*
 * R479 A3 — R021 错误路径交互锚定（Vue 契约）。
 *
 * - ChatHistorySettings.vue:171-174 保存失败：errorMessage = error?.message ||
 *   'Unknown error'，MessagePlugin.error(saveFailed 模板插值)。React 以面板内
 *   <Status tone="error"> 呈现同一条消息链（settings 域 R471/R472 口径），且
 *   Vue 失败后草稿保留 → React 表单值/开关同样保持可编辑。
 * - ParserEngineSettings.vue:667 check 失败：checkMessage = e?.message ||
 *   checkFailed（行内 footer 消息，非 toast）→ React 同为行内。
 */

test('chat history save failure surfaces the backend message and keeps the form editable (R021)', async () => {
  const calls: number[] = [];
  const client = {
    settings: { chatHistory: { config: { update: async () => { calls.push(calls.length); throw new Error('upstream down'); } } } },
  } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(
    <ConfigSettingsPanel
      client={client}
      section="chathistory"
      initialValue={{ enabled: false, embedding_model_id: '' }}
      models={[{ id: 'embed-1', name: 'Embedding' }]}
    />,
  ));

  const toggle = container.querySelector('button[role="switch"]') as HTMLButtonElement | null;
  assert.ok(toggle, 'expected the enable switch');
  assert.equal(toggle.getAttribute('aria-checked'), 'false');
  // Toggling marks the form dirty and arms the Vue 500ms debounced save.
  await act(async () => { toggle.dispatchEvent(new window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 550)); });

  assert.equal(calls.length, 1, 'the debounced save should have fired once');
  assert.match(container.textContent ?? '', /upstream down/, 'the backend message is surfaced');
  // UI stays intact: the switch reflects the draft change and remains operable.
  const toggleAfter = container.querySelector('button[role="switch"]') as HTMLButtonElement | null;
  assert.ok(toggleAfter, 'the switch must stay rendered after a failed save');
  assert.equal(toggleAfter.getAttribute('aria-checked'), 'true', 'the draft change is retained like the Vue draft model');
});

test('parser connection-check failure falls back to the localized checkFailed message (R021)', async () => {
  const client = {
    settings: { parser: { config: { update: async (body: Record<string, unknown>) => body }, check: async () => { throw 'probe rejected'; } } },
  } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ConfigSettingsPanel client={client} section="parser" initialValue={{}} />));

  const checkButton = [...container.querySelectorAll('button')].find((button) => button.textContent === '测试连接');
  assert.ok(checkButton, 'expected the parser test-connection button');
  await act(async () => { checkButton.dispatchEvent(new window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); });

  assert.match(container.textContent ?? '', /检测失败/, 'non-Error rejections fall back to settings.parser.checkFailed');
});

test('parser form save failure surfaces the backend error and keeps controls rendered (R021)', async () => {
  const client = {
    settings: { parser: { config: { update: async () => { throw new Error('bad payload'); } }, check: async () => ({ connected: true }) } },
  } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ConfigSettingsPanel client={client} section="parser" initialValue={{}} />));

  const endpointHost = container.querySelector('[data-testid="mineru-endpoint"]');
  assert.ok(endpointHost, 'expected the mineru endpoint field');
  const endpoint = (endpointHost.tagName === 'INPUT' ? endpointHost : endpointHost.querySelector('input')) as HTMLInputElement | null;
  assert.ok(endpoint, 'expected an input inside the mineru endpoint field');
  await act(async () => {
    Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set?.call(endpoint, 'http://mineru.local');
    endpoint.dispatchEvent(new window.Event('input', { bubbles: true }));
  });
  const save = container.querySelector('[data-testid="config-save"]') as HTMLButtonElement | null;
  assert.ok(save, 'expected the save button');
  await act(async () => { save.dispatchEvent(new window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); });

  assert.match(container.textContent ?? '', /bad payload/, 'the backend message is surfaced on the inline status');
  assert.ok(container.querySelector('[data-testid="mineru-model"]'), 'form controls stay rendered after a failed save');
});

/*
 * R481-A3 — 锁行为：上游 retrieval-config 加载失败时（设置页深链
 * ?section=retrieval 静默化后 payload=null，或抽屉降级），面板必须把
 * initialValue=null 渲染为 Vue RetrievalSettings 的默认值表单：
 * embedding_top_k=50、vector_threshold=0.15、keyword_threshold=0.3、
 * rerank_top_k=10、rerank_threshold=0.2、rerank_model_id=''，无异常。
 */
test('retrieval section renders the Vue default form when initialValue is null (R480/R481)', async () => {
  const client = { settings: { retrieval: { update: async (body: Record<string, unknown>) => body } } } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ConfigSettingsPanel client={client} section="retrieval" initialValue={null} models={[]} />));

  const text = container.textContent ?? '';
  assert.match(text, /向量检索数量 \(Top K\)/, 'the default retrieval form renders');
  const outputs = [...container.querySelectorAll('output')].map((node) => node.textContent);
  assert.deepEqual(outputs, ['50', '0.15', '0.30', '10', '0.20'], 'sliders fall back to the Vue defaults (Top K 50, thresholds 0.15/0.30/10/0.20)');
  const rerankInput = container.querySelector('input:not([type="range"])') as HTMLInputElement | null;
  assert.ok(rerankInput, 'models=[] renders the rerank model as a text input');
  assert.equal(rerankInput.value, '', 'rerank model defaults to empty');
});
