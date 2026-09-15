// Shell sessions-header slice tests (jsdom + react-dom), following the
// harness in shell-session-list.test.tsx.
//
// Slice: the Vue sidebar session area (frontend/src/components/menu.vue) uses
// a semantic session region with date/row content but no visible 我的对话
// heading. The React shell must preserve the semantic label without adding a
// second visible title.
// Coordinator ruling (strict parity): the 新建对话⌘1 fingerprint belongs to the
// command palette's first quick action, so the sidebar nav gets no kbd hint
// and ⌘1 stays palette-scoped — tests (b)/(d) pin that absence.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

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
// zh-CN like the acceptance environment; resolveLocale reads navigator.language.
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default locale */ }
// Pin an Apple-like platform so the ⌘ variant of the hint is deterministic
// (Vue menu.vue:303-304 keys the label off navigator.platform).
try { Object.defineProperty(jsdomNavigator, 'platform', { value: 'MacIntel', configurable: true }); } catch { /* keep default platform */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { PlatformShell, platformModKeyLabel } = await import('./PlatformShell.tsx');

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

function fakeClient(me: Record<string, unknown> = { user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '' } }): Record<string, unknown> {
  return {
    auth: { me: async () => me },
    sessions: {
      list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }),
    },
  };
}

async function mountShell(options: { collapsed?: boolean; me?: Record<string, unknown> } = {}) {
  window.localStorage.setItem('weknora:new-user-guide-done:v1', '1');
  if (options.collapsed) window.localStorage.setItem('weknora_sidebar_collapsed', 'true');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(
      PlatformShell,
      { client: fakeClient(options.me) as never, onLogout: () => undefined, children: React.createElement('div', { 'data-testid': 'outlet-page' }, 'page') },
    ));
  });
  await settle(20);
  return container;
}

const pressKey = (init: { key: string; metaKey?: boolean; ctrlKey?: boolean; shiftKey?: boolean }, target?: EventTarget) =>
  (target ?? window).dispatchEvent(new window.KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }));

test('(a) the shell sessions region keeps 我的对话 semantic-only', async () => {
  await mountShell();
  const nav = document.querySelector('nav[aria-label="我的对话"]');
  assert.ok(nav, 'expected the shell session list region');
  assert.equal(nav.getAttribute('aria-label'), '我的对话');
  const title = nav.querySelector('h2');
  assert.equal(title, null, 'Vue does not render a visible sessions title');
});

test('(b) the 新对话 entry carries no shortcut hint (Vue sidebar nav has none)', async () => {
  await mountShell();
  const item = [...document.querySelectorAll('nav[aria-label="Platform"] a')]
    .find((node) => node.textContent?.includes('新对话')) as HTMLAnchorElement | undefined;
  assert.ok(item, 'expected the 新对话 nav entry');
  // Coordinator ruling on strict parity: the 新建对话⌘1 fingerprint belongs to
  // the command palette's first quick action (GlobalCommandPalette ⌘1-9 chips,
  // palette-scoped), not the sidebar nav. The Vue sidebar 新对话 entry shows no
  // kbd hint, so React must not render one either.
  assert.equal(item.querySelector('kbd'), null, 'no shortcut hint on the sidebar nav entry');
  // Collapsed sidebars hide the session area entirely (Vue parity).
  await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  await mountShell({ collapsed: true });
  assert.equal(document.querySelectorAll('nav[aria-label="我的对话"]').length, 0, 'collapsed rail hides the sessions area');
  const expand = document.querySelector('button[aria-label="展开侧边栏"]');
  assert.ok(expand, 'collapsed rail uses localized expand label');
});

test('(c) platformModKeyLabel mirrors Vue menu.vue:303-304 (⌘ on Apple, Ctrl+ elsewhere)', () => {
  assert.equal(platformModKeyLabel('MacIntel'), '⌘');
  assert.equal(platformModKeyLabel('Macintosh; Intel Mac OS X 10_15_7'), '⌘');
  assert.equal(platformModKeyLabel('iPad'), '⌘');
  assert.equal(platformModKeyLabel('Win32'), 'Ctrl+');
  assert.equal(platformModKeyLabel('Linux x86_64'), 'Ctrl+');
  assert.equal(platformModKeyLabel(''), 'Ctrl+');
});

test('(d) ⌘1 / Ctrl+1 never navigate app-wide (Vue binds ⌘1-9 only inside the open palette)', async () => {
  await mountShell();
  // Strict parity: Vue has no app-wide ⌘1 binding — the ⌘1-9 shortcuts are
  // scoped to the open command palette. Guard against re-introducing a
  // global hijack: nothing may navigate for these presses.
  const before = window.location.href;
  pressKey({ key: '1', metaKey: true });
  pressKey({ key: '1', ctrlKey: true });
  await settle(5);
  assert.equal(window.location.href, before, 'no app-wide ⌘1 navigation');
});

test('(e) multi-space identity shows the active tenant and localized role like Vue UserMenu', async () => {
  await mountShell({
    me: {
      user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '', can_access_all_tenants: false },
      tenant: { id: 1, name: '团队空间' },
      memberships: [{ tenant_id: 1, role: 'admin' }, { tenant_id: 2, role: 'viewer' }],
    },
  });
  const button = document.querySelector('[data-guide="user-menu"]');
  assert.ok(button);
  assert.match(button.textContent ?? '', /团队空间/);
  assert.match(button.textContent ?? '', /tester/);
  assert.match(button.textContent ?? '', /管理员/);
  assert.doesNotMatch(button.textContent ?? '', /tester@local\.dev/);
});
