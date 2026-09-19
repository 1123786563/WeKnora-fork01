import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');
const { ChatComposer, resolveChatCopy } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

/*
 * R484 D15 — Vue Input-field.vue:2690-2719 (t-tooltip .websearch-btn):
 * a globe toggle sits after the agent chip / before the image-upload button,
 * rendered only while the tenant (or selected agent) is web-search ready.
 * - title/aria follow input.webSearch.toggleOn/toggleOff/notConfigured
 * - active state = enabled && configured (Vue :class active binding)
 * - click delegates to the host toggle (Vue toggleWebSearch)
 */

async function renderComposer(props: Partial<Parameters<typeof ChatComposer>[0]> = {}) {
  const container = document.createElement('div');
  document.body.append(container);
  const r = createRoot(container);
  root = r;
  await act(async () => r.render(<ChatComposer copy={resolveChatCopy('zh-CN')} draft="" onDraftChange={() => undefined} onSubmit={() => undefined} {...props} />));
  return container;
}

test('web search button renders with the Vue globe icon between agent chip and attachments (D15)', async () => {
  const container = await renderComposer({ webSearchVisible: true, webSearchConfigured: true, webSearchEnabled: false, onWebSearchToggle: () => undefined });
  const button = container.querySelector<HTMLButtonElement>('[data-web-search-toggle]');
  assert.ok(button, 'web search toggle rendered when visible');
  assert.equal(button.getAttribute('aria-label'), '开启网络搜索');
  assert.ok(button.querySelector('svg'), 'globe icon svg present');
  const attach = [...container.querySelectorAll('button')].find((b) => b.getAttribute('aria-label') === '上传附件');
  assert.ok(attach, 'attachment button present as the ordering landmark');
  assert.ok(button.compareDocumentPosition(attach) & dom.window.Node.DOCUMENT_POSITION_FOLLOWING, 'web search precedes the attachment button');
});

test('web search button stays hidden when the host reports no readiness (D15)', async () => {
  const container = await renderComposer();
  assert.equal(container.querySelector('[data-web-search-toggle]'), null, 'no readiness props → no button');
});

test('configured+enabled state flips the title to toggleOff and marks the button active (D15)', async () => {
  const container = await renderComposer({ webSearchVisible: true, webSearchConfigured: true, webSearchEnabled: true, onWebSearchToggle: () => undefined });
  const button = container.querySelector<HTMLButtonElement>('[data-web-search-toggle]');
  assert.ok(button);
  assert.equal(button.getAttribute('aria-label'), '关闭网络搜索');
  assert.equal(button.getAttribute('data-active'), 'true');
});

test('unconfigured engine keeps the notConfigured title and non-active look (D15)', async () => {
  const container = await renderComposer({ webSearchVisible: true, webSearchConfigured: false, webSearchEnabled: false, onWebSearchToggle: () => undefined });
  const button = container.querySelector<HTMLButtonElement>('[data-web-search-toggle]');
  assert.ok(button);
  assert.equal(button.getAttribute('aria-label'), '未配置网络搜索引擎');
  assert.equal(button.getAttribute('data-active'), null);
  assert.equal(button.getAttribute('data-configured'), 'false');
});

test('clicking the toggle delegates to onWebSearchToggle (Vue toggleWebSearch host side) (D15)', async () => {
  const toggles: number[] = [];
  const container = await renderComposer({ webSearchVisible: true, webSearchConfigured: true, webSearchEnabled: false, onWebSearchToggle: () => toggles.push(1) });
  await act(async () => container.querySelector<HTMLButtonElement>('[data-web-search-toggle]')?.click());
  assert.deepEqual(toggles, [1]);
});
