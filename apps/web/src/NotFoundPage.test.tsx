// NotFoundPage 单测（S4）。404 页不可进像素扫描：Vue 端 router（frontend/src/
// router/index.ts）没有 catch-all 路由，未匹配路径渲染空白 #app（probe 取证：
// /platform/definitely-not-a-page → bodyText 为空）；React 端按既有 pre-router
// UX 在 platform shell 内渲染 NotFoundPage。按 task-s4 简报约定以单测+目检
// 代替扫描（目检证据：shell outlet 内渲染「页面不存在: <path>」+ 返回知识库）。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/missing' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default locale */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { NotFoundPage } = await import('./NotFoundPage.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

async function mount(props: { path: string }) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(NotFoundPage, props));
  });
  return container;
}

test('renders the zh-CN not-found copy with the unmatched path echoed', async () => {
  await mount({ path: '/platform/definitely-not-a-page' });
  const main = document.querySelector('main');
  assert.ok(main, 'expected the page <main> region');
  assert.match(main.textContent ?? '', /页面不存在: \/platform\/definitely-not-a-page/);
  const back = main.querySelector('a');
  assert.equal(back?.getAttribute('href'), '/platform/knowledge-bases');
  assert.match(back?.textContent ?? '', /返回知识库/);
});

test('echoes arbitrary unmatched paths inside the alert region', async () => {
  await mount({ path: '/knowledgeBase/zzz/unknown' });
  const alert = document.querySelector('main [role="alert"]');
  assert.ok(alert, 'expected an alert region');
  assert.match(alert.textContent ?? '', /\/knowledgeBase\/zzz\/unknown/);
});
