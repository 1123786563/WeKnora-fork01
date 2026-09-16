// Platform shell sidebar state tests (jsdom + react-dom), following the
// harness in apps/web/src/platform/platform-shell-guide-reopen.test.tsx.
//
// Slice: R442/A4 platform-shell sweep. Vue parity baseline —
//   frontend/src/components/menu.vue:5-8 (expanded logo row renders a
//   `<sup class="lite-badge">Lite</sup>` edition mark when isLiteEdition,
//   which stores/auth.ts:538 sources from
//   localStorage['weknora_lite_mode'] === 'true')
//   frontend/src/stores/ui.ts:23,123-126 (collapse state persists under the
//   Vue-era key `sidebar_collapsed` and is re-read on the next boot, so the
//   collapsed rail survives a reload and is shared with the Vue artifact).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// PlatformShell imports .css/.png assets; treat them as empty modules.
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.png')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default locale */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { PlatformShell } = await import('./PlatformShell.tsx');
const { formatMessage } = await import('@weknora/i18n');

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

function fakeClient(): Record<string, unknown> {
  return {
    auth: { me: async () => ({ user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '' }, tenant: { id: 'tenant-1', name: 'Parity' }, memberships: [] }) },
    sessions: { list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }) },
  };
}

async function mountShell(): Promise<HTMLElement> {
  window.localStorage.setItem('locale', 'zh-CN');
  window.localStorage.setItem('weknora:new-user-guide-done:v1', '1');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(
      PlatformShell,
      { client: fakeClient() as never, onLogout: () => undefined, children: React.createElement('div', null, 'page') },
    ));
  });
  await settle(20);
  return container;
}

const logoLink = () => document.querySelector<HTMLAnchorElement>('a[aria-label="WeKnora"]');
const collapseButton = () => {
  const label = formatMessage('zh-CN', 'menu.collapseSidebar');
  return [...document.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.getAttribute('aria-label') === label) ?? null;
};
const expandButton = () => {
  const label = formatMessage('zh-CN', 'menu.expandSidebar');
  return [...document.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.getAttribute('aria-label') === label) ?? null;
};

test('lite-edition flag renders the Vue Lite badge in the expanded logo row', async () => {
  window.localStorage.setItem('weknora_lite_mode', 'true');
  const container = await mountShell();
  const badge = container.querySelector('a[aria-label="WeKnora"] sup');
  assert.ok(badge, 'menu.vue renders a lite-badge sup next to the logo for lite editions');
  assert.equal(badge?.textContent, 'Lite', 'Vue keeps the edition mark literal ("Lite", untranslated)');
});

test('non-lite editions keep the logo row free of the Lite badge', async () => {
  const container = await mountShell();
  assert.ok(logoLink(), 'expanded shell shows the logo link');
  assert.ok(logoLink()?.querySelector('img'), 'logo image present in the expanded row');
  assert.equal(container.querySelector('a[aria-label="WeKnora"] sup'), null, 'no Lite badge without weknora_lite_mode');
});

test('collapsing the sidebar persists under the Vue key and a fresh mount restores it', async () => {
  await mountShell();
  assert.ok(logoLink()?.querySelector('img'), 'expanded shell shows the logo image');
  const button = collapseButton();
  assert.ok(button, 'collapse toggle present in the expanded shell');
  await act(async () => { button!.click(); await new Promise((resolve) => setTimeout(resolve, 5)); });
  assert.equal(window.localStorage.getItem('sidebar_collapsed'), 'true', 'Vue stores/ui.ts persists sidebar_collapsed');
  assert.equal(window.localStorage.getItem('weknora_sidebar_collapsed'), null, 'the React-only alias key must not linger');

  // Fresh boot reads the Vue-era key like stores/ui.ts:23 does.
  await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  const container = await mountShell();
  assert.ok(expandButton(), 'a remounted shell restores the collapsed rail from sidebar_collapsed');
  assert.equal(container.querySelector('a[aria-label="WeKnora"] img'), null, 'collapsed rail hides the logo image (Vue logo_row v-if)');
});
