import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import type { ModelConfiguration, WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { ModelDebugPanel } = await import('./ModelDebugPanel.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

const chatModel = {
  id: 'chat-1', name: 'gpt-4o', display_name: 'GPT', type: 'KnowledgeQA', source: 'remote',
  parameters: { provider: 'openai', context_window: 128000 },
} as never;
const qwenModel = {
  id: 'chat-2', name: 'qwen3-8b', type: 'KnowledgeQA', source: 'remote',
  parameters: { provider: 'aliyun' },
} as never;
const rerankModel = { id: 'rank-1', name: 'rank', type: 'Rerank', source: 'remote', parameters: {} } as never;

function clientWithDebug(debug?: (modelId: string, input: Record<string, unknown>) => Promise<unknown>) {
  return {
    configuration: {
      models: {
        debug: debug ?? (async () => {
          throw new Error('not stubbed');
        }),
      },
    },
  } as unknown as WeKnoraClient;
}

async function mount(client: WeKnoraClient, models: readonly ModelConfiguration[]) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<ModelDebugPanel client={client} models={models} onClose={() => undefined} />);
  });
  return container;
}
async function setInput(input: HTMLInputElement | HTMLTextAreaElement, value: string) {
  await act(async () => {
    const proto = input instanceof dom.window.HTMLTextAreaElement
      ? dom.window.HTMLTextAreaElement.prototype
      : dom.window.HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value')?.set?.call(input as never, value);
    input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    input.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
}
async function click(button: Element) {
  await act(async () => button.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })));
}

test('model debug panel renders the Vue copy and per-type inputs', () => {
  const html = renderToStaticMarkup(
    <ModelDebugPanel
      client={{} as never}
      models={[chatModel, rerankModel]}
      onClose={() => undefined}
    />,
  );
  assert.match(html, /模型测试/);
  assert.match(html, /向已配置的模型发送真实请求，查看响应与耗时/);
  // Model option meta shows vendor label and formatted context window
  // (ModelDebugDrawer.vue lines 60-66).
  assert.match(html, /GPT · OpenAI · 128K/);
  assert.match(html, /System Prompt/);
  // Thinking toggle hidden when the provider default resolves to none
  // (ModelDebugDrawer.vue line 141 modelSupportsThinking).
  assert.doesNotMatch(html, /思考模式/);
});

test('model debug gates the thinking toggle on modelSupportsThinking', async () => {
  const container = await mount(clientWithDebug(), [qwenModel]);
  assert.match(container.textContent ?? '', /思考模式/);
  assert.match(container.textContent ?? '', /仅对支持思考模式的模型生效/);
});

test('model debug clears thinking after selecting a model that does not support it', async () => {
  const container = await mount(clientWithDebug(), [qwenModel, chatModel]);
  const thinkingToggle = container.querySelector<HTMLButtonElement>('[role="switch"]');
  assert.ok(thinkingToggle);
  await click(thinkingToggle);
  assert.equal(thinkingToggle.getAttribute('aria-checked'), 'true');

  const selectModel = async (modelId: string) => {
    const combobox = container.querySelector<HTMLButtonElement>('[role="combobox"]');
    assert.ok(combobox);
    await click(combobox);
    const option = container.querySelector<HTMLButtonElement>(`[role="option"][data-value="${modelId}"]`);
    assert.ok(option);
    await click(option);
  };

  await selectModel('chat-1');
  assert.equal(container.querySelector('[role="switch"]'), null, 'OpenAI does not expose thinking controls');
  await selectModel('chat-2');
  assert.equal(container.querySelector<HTMLButtonElement>('[role="switch"]')?.getAttribute('aria-checked'), 'false');
});

test('model debug rerank asks for documents and ranks only with both inputs', async () => {
  const container = await mount(clientWithDebug(), [rerankModel]);
  const text = container.textContent ?? '';
  assert.match(text, /候选文档/);
  assert.match(text, /每个非空行会作为一个独立文档发送给 ReRank 模型/);
  const run = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '运行测试');
  assert.ok(run);
  assert.equal((run as HTMLButtonElement).disabled, true);

  const textareas = Array.from(container.querySelectorAll('textarea'));
  assert.equal(textareas.length, 2);
  await setInput(textareas[0]!, '什么是 WeKnora');
  assert.equal((run as HTMLButtonElement).disabled, true, 'documents are still missing');
  await setInput(textareas[1]!, 'WeKnora 是一个 RAG 知识库\n另一个文档');
  assert.equal((run as HTMLButtonElement).disabled, false);
});

test('model debug run shows the Vue result banner, metrics and history labels', async () => {
  const inputs: Array<Record<string, unknown>> = [];
  const client = clientWithDebug(async (_modelId, input) => {
    inputs.push(input as Record<string, unknown>);
    return {
      ok: true, elapsedMs: 42, rawResponse: { answer: 'hi' }, request: { model: 'qwen3-8b' },
      observations: { dimension: 1024, result_count: 3, reasoning_returned: true },
    };
  });
  const container = await mount(client, [qwenModel]);
  const textarea = container.querySelector('textarea');
  assert.ok(textarea);
  await setInput(textarea, '你好');
  const run = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '运行测试');
  assert.ok(run);
  await click(run);
  await act(async () => {});

  const text = container.textContent ?? '';
  assert.match(text, /调用成功/);
  assert.match(text, /42 ms/);
  // Metric chips use modelSettings.debug.metrics.* labels; booleans map to common.yes/no
  // (ModelDebugDrawer.vue lines 323-349).
  assert.match(text, /向量维度: 1024/);
  assert.match(text, /结果数量: 3/);
  assert.match(text, /返回推理内容: 是/);
  assert.match(text, /复制结果/);
  assert.equal(inputs.length, 1);
  assert.deepEqual(inputs[0].options, { systemPrompt: undefined, temperature: 0.7, topP: 1, maxTokens: 1024, thinking: false });

  // Second run with the thinking toggle on exposes the history labels
  // (ModelDebugDrawer.vue lines 150-168, 420-425).
  const thinkingToggle = container.querySelector<HTMLButtonElement>('[role="switch"]');
  assert.ok(thinkingToggle);
  await click(thinkingToggle);
  await click(run);
  await act(async () => {});
  assert.equal(inputs.length, 2);
  assert.deepEqual(inputs[1].options, { systemPrompt: undefined, temperature: 0.7, topP: 1, maxTokens: 1024, thinking: true });
  const historyText = container.textContent ?? '';
  assert.match(historyText, /思考开启/);
  assert.match(historyText, /思考关闭/);
});
