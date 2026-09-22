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
// T12b：chathistory 用例引入 tdesign-react（Select 弹层经 Popup 挂 body），
// jsdom globals 扩展与 settings-error-ux.test 同款（T12a d1fba03aa 先例）。
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
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  IS_REACT_ACT_ENVIRONMENT: true,
});
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
  // T12b：chathistory 分区已迁 ChatHistorySettingsPanel（Vue DOM 直挂 .section）。
  const { ChatHistorySettingsPanel } = await import('./ChatHistorySettingsPanel.tsx');
  await act(async () => root?.render(
    <ChatHistorySettingsPanel
      client={client}
      initialValue={{ enabled: false, embedding_model_id: 'embed-1' }}
      models={[{ id: 'embed-1', name: 'Embedding', type: 'Embedding' }]}
    />,
  ));

  assert.equal(container.querySelector('.model-selector'), null, 'the embedding row stays hidden while disabled (Vue v-if="localEnabled")');
  assert.ok(container.querySelector('.chat-history-settings'), 'the Vue root class renders');
  assert.ok(container.querySelector('.t-switch'), 'the enable row renders a tdesign switch');
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
  // T12b：chathistory 已迁 ChatHistorySettingsPanel；Vue 失败走
  // MessagePlugin.error → React settings 域 toast 宿主（pushSettingsToast）。
  const { ChatHistorySettingsPanel } = await import('./ChatHistorySettingsPanel.tsx');
  const { SettingsToastHost } = await import('./settings-toast.tsx');
  await act(async () => root?.render(
    <>
      <SettingsToastHost />
      <ChatHistorySettingsPanel
        client={client}
        initialValue={{ enabled: false, embedding_model_id: '' }}
        models={[{ id: 'embed-1', name: 'Embedding', type: 'Embedding' }]}
      />
    </>
  ));

  const toggle = container.querySelector('button[role="switch"]') as HTMLButtonElement | null;
  assert.ok(toggle, 'expected the enable switch');
  // Toggling arms the Vue 500ms debounced save.
  await act(async () => { toggle.dispatchEvent(new window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 550)); });

  assert.equal(calls.length, 1, 'the debounced save should have fired once');
  assert.match(document.body.textContent ?? '', /upstream down/, 'the backend message surfaces via the settings toast (Vue MessagePlugin.error)');
  // UI stays intact: the switch reflects the draft change and remains operable.
  const toggleAfter = container.querySelector('button[role="switch"]') as HTMLButtonElement | null;
  assert.ok(toggleAfter, 'the switch must stay rendered after a failed save');
  // T12b fix round（评审 Minor-1）：恢复迁移时丢失的「draft 变更保留」断言。
  // tdesign Switch 选中态经 .t-is-checked 类表达（台账 #2：React 根标签
  // button role="switch"，断言走 classList 而非 aria-checked；sandbox 先例同款）。
  assert.equal(
    toggleAfter?.classList.contains('t-is-checked'), true,
    'the draft toggle state survives the failed save (Vue keeps the editable draft)',
  );
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
