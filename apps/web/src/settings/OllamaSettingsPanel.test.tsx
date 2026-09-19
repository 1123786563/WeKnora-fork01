import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings?section=ollama' });
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
const { OllamaSettingsPanel } = await import('./OllamaSettingsPanel.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.clear();
});

function makeClient(onDownload?: () => void): WeKnoraClient {
  return {
    settings: {
      ollama: {
        status: async () => ({ available: false }),
        models: async () => [],
        download: async () => { onDownload?.(); return { task_id: 'task-1' }; },
        progress: async () => ({ status: 'running' }),
      },
    },
  } as unknown as WeKnoraClient;
}

async function mount(initialValue: unknown, client = makeClient()) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => mountedRoot?.render(<OllamaSettingsPanel client={client} initialValue={initialValue} />));
  return container;
}

test('hides model management sections when Ollama is unavailable', async () => {
  const container = await mount({ status: { available: false }, models: [] });
  const text = container.textContent ?? '';
  assert.equal(text.includes('下载新模型'), false);
  assert.equal(text.includes('已下载的模型'), false);
});

test('shows the Vue warning banner and unavailable tag when the partition failed to load', async () => {
  const container = await mount(null);
  const text = container.textContent ?? '';
  assert.equal(text.includes('连接失败，请检查 Ollama 是否运行或服务地址是否正确'), true, 'the friendly Vue warning banner renders');
  assert.equal(text.includes('不可用'), true, 'the status tag reads unavailable');
  assert.equal(text.includes('未检测'), false, 'a failed load must not read as untested');
});

test('shows the Vue warning banner when the loaded status is unavailable', async () => {
  const container = await mount({ status: { available: false }, models: [] });
  const text = container.textContent ?? '';
  assert.equal(text.includes('连接失败，请检查 Ollama 是否运行或服务地址是否正确'), true, 'the friendly Vue warning banner renders below the address row');
});

test('keeps the banner hidden when Ollama is available', async () => {
  const container = await mount({ status: { available: true, version: '0.1' }, models: [{ name: 'llama3', size: '4.7 GB' }] });
  const text = container.textContent ?? '';
  assert.equal(text.includes('连接失败，请检查 Ollama 是否运行或服务地址是否正确'), false);
  assert.equal(text.includes('可用'), true);
});

test('keeps the Vue model-library link in the download section', async () => {
  const container = await mount({ status: { available: true }, models: [] });
  const link = container.querySelector('a[href="https://ollama.com/search"]');
  assert.ok(link, 'the Ollama model library link is rendered');
  assert.equal(link?.getAttribute('target'), '_blank');
  assert.equal(link?.textContent, '浏览 Ollama 模型库');
});

test('disables the Vue download action until a non-blank model name is entered', async () => {
  let downloadCalls = 0;
  const container = await mount(
    { status: { available: true }, models: [] },
    makeClient(() => { downloadCalls += 1; }),
  );
  const downloadButton = Array.from(container.querySelectorAll<HTMLButtonElement>('button'))
    .find((button) => button.textContent?.includes('下载'));
  assert.ok(downloadButton, 'the download action renders');
  assert.equal(downloadButton.disabled, true, 'Vue disables an empty download action');

  await act(async () => downloadButton.click());
  assert.equal(downloadCalls, 0, 'an empty model name must not call the download API');
});

test('shows the Vue testing state and hides model management while retesting', async () => {
  let resolveStatus: ((value: { available: boolean }) => void) | undefined;
  const statusPromise = new Promise<{ available: boolean }>((resolve) => { resolveStatus = resolve; });
  const client = makeClient();
  client.settings.ollama.status = async () => statusPromise;
  const container = await mount({ status: { available: true }, models: [] }, client);

  const retest = Array.from(container.querySelectorAll<HTMLButtonElement>('button'))
    .find((button) => button.textContent?.includes('重新检测'));
  assert.ok(retest, 'the retest action renders');
  await act(async () => retest?.click());

  assert.match(container.textContent ?? '', /检测中/);
  assert.doesNotMatch(container.textContent ?? '', /下载新模型/);
  assert.doesNotMatch(container.textContent ?? '', /已下载的模型/);

  resolveStatus?.({ available: true });
  await act(async () => { await statusPromise; });
  assert.doesNotMatch(container.textContent ?? '', /检测中/);
  assert.match(container.textContent ?? '', /可用/);
});
