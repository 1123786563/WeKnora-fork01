// R447-A2 platform shell defects from the R446 browser evidence:
// D6 — the user menu rendered the raw key `general.helpAndDocs` because the
//      key was absent from packages/i18n (Vue UserMenu.vue:118 uses
//      $t('general.helpAndDocs') with a full 5-locale inventory).
// D7 — the 「退出」 menu item must actually trigger the onLogout chain and
//      close the menu (Vue UserMenu.vue:518-536 handleLogout: close menu →
//      logout API → local cleanup → land on /login).
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
try { Object.defineProperty(dom.window.navigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep jsdom default */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { PlatformShell, runShellLogout } = await import('./PlatformShell.tsx');
const { formatMessage, messages, supportedLocales } = await import('../../../../packages/i18n/src/index.ts');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

const settle = (ms = 10) => act(async () => { await new Promise((resolve) => setTimeout(resolve, ms)); });

function fakeClient() {
  return {
    auth: {
      me: async () => ({
        user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '', can_access_all_tenants: false },
        tenant: { id: 1, name: 'Home' },
        memberships: [{ tenant_id: 1, tenant_name: 'Home', role: 'owner' }],
      }),
    },
    sessions: { list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }) },
    organizations: { list: async () => ({ items: [], total: 0, resourceCounts: undefined }) },
  };
}

async function mount(onLogout: () => void | Promise<void>) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => mountedRoot?.render(React.createElement(PlatformShell, {
    client: fakeClient() as never,
    onLogout,
    onTenantSwitch: async () => undefined,
    children: React.createElement('div', null, 'page'),
  })));
  await settle(20);
}

async function openUserMenu() {
  const userMenu = document.querySelector('[data-guide="user-menu"]') as HTMLButtonElement | null;
  assert.ok(userMenu, 'user menu button exists');
  await act(async () => userMenu.click());
  await settle();
}

// D6 — Vue UserMenu.vue:118 labels the docs entry with $t('general.helpAndDocs');
// the key must resolve in every locale instead of leaking the raw key.
test('D6: user menu help entry renders translated text, not the raw key', async () => {
  await mount(() => undefined);
  await openUserMenu();
  const menu = document.querySelector('[role="menu"]');
  assert.ok(menu, 'user dropdown renders');
  const text = menu.textContent ?? '';
  assert.ok(!text.includes('general.helpAndDocs'), 'raw i18n key must not leak into the menu');
  assert.ok(text.includes('帮助与文档'), 'zh-CN label matches the Vue locale value');
});

test('D6: general.helpAndDocs exists with the Vue locale copy in all five locales', () => {
  const expected: Record<string, string> = {
    'zh-CN': '帮助与文档',
    'en-US': 'Help & Documentation',
    'ja-JP': 'ヘルプとドキュメント',
    'ko-KR': '도움말 및 문서',
    'ru-RU': 'Справка и документация',
  };
  for (const locale of supportedLocales) {
    assert.equal(messages[locale]['general.helpAndDocs'], expected[locale], `${locale} copy matches Vue`);
    assert.equal(formatMessage(locale, 'general.helpAndDocs'), expected[locale]);
  }
});

// D7 — Vue UserMenu.vue:138-141 renders the danger logout item; clicking it
// must invoke the shell's onLogout chain and close the menu.
test('D7: clicking the logout item invokes onLogout and closes the menu', async () => {
  let logoutCalls = 0;
  await mount(() => { logoutCalls += 1; });
  await openUserMenu();
  const menu = document.querySelector('[role="menu"]');
  assert.ok(menu, 'user dropdown renders');
  const logoutButton = Array.from(menu.querySelectorAll('button')).find((b) => b.textContent === '退出') as HTMLButtonElement | undefined;
  assert.ok(logoutButton, 'logout menu item renders with the translated 退出 label');
  await act(async () => logoutButton.click());
  await settle();
  assert.equal(logoutCalls, 1, 'onLogout fired exactly once');
  assert.equal(document.querySelector('[role="menu"]'), null, 'menu closed after logout click');
});

// D7 hardening — Vue UserMenu.vue:518-536 always ends on the login page even
// when the logout API call fails. A rejected onLogout chain (or one that never
// settles) must fall back to a hard navigation instead of stranding the user.
test('D7: a rejected onLogout chain still navigates to /login', async () => {
  const navigated: string[] = [];
  await runShellLogout(() => Promise.reject(new Error('logout endpoint down')), (url) => navigated.push(url), 15);
  assert.deepEqual(navigated, ['/login'], 'fallback navigation fired after rejection');
});

test('D7: a hung onLogout chain falls back to /login after the grace window', async () => {
  const navigated: string[] = [];
  await runShellLogout(() => new Promise<void>(() => {}), (url) => navigated.push(url), 15);
  assert.deepEqual(navigated, ['/login'], 'fallback navigation fired after the timeout');
});

test('D7: a successful onLogout chain does not double-navigate', async () => {
  const navigated: string[] = [];
  await runShellLogout(() => Promise.resolve(), (url) => navigated.push(url), 15);
  assert.deepEqual(navigated, [], 'parent chain owns navigation on success');
});

// R448-A2 — Vue UserMenu.vue:96-100 renders an unconditional 「全部设置」
// ($t('general.allSettings')) entry right after the section quick links:
// handleSettings closes the menu, opens settings and lands on
// /platform/settings WITHOUT a section query (the ?section= links above it
// are scoped quick navs; this one is the catch-all entry point).
test('R448-A2: user menu renders the 全部设置 entry between the quick links and the docs entry', async () => {
  await mount(() => undefined);
  await openUserMenu();
  const menu = document.querySelector('[role="menu"]');
  assert.ok(menu, 'user dropdown renders');
  const items = Array.from(menu.querySelectorAll('[role="menuitem"]'));
  const allSettings = items.find((el) => (el.textContent ?? '').includes('全部设置'));
  assert.ok(allSettings, '全部设置 entry renders with translated copy');
  assert.equal((allSettings as HTMLAnchorElement).getAttribute('href'), '/platform/settings', 'navigates to settings without a section query');
  const idx = (el: Element | undefined) => (el ? items.indexOf(el) : -1);
  const personal = items.find((el) => (el.textContent ?? '').includes('个人设置'));
  const docs = items.find((el) => (el.textContent ?? '').includes('帮助与文档'));
  assert.ok(personal && docs, 'context entries render');
  assert.ok(idx(personal) < idx(allSettings), 'ordered after the section quick links');
  assert.ok(idx(allSettings) < idx(docs), 'ordered before the external docs entry');
});

test('R448-A2: clicking 全部设置 navigates to /platform/settings without a query and closes the menu', async () => {
  await mount(() => undefined);
  await openUserMenu();
  const menu = document.querySelector('[role="menu"]');
  assert.ok(menu);
  const entry = Array.from(menu.querySelectorAll('[role="menuitem"]'))
    .find((el) => (el.textContent ?? '').includes('全部设置')) as HTMLAnchorElement;
  assert.ok(entry, 'entry present before the click');
  await act(async () => entry.click());
  await settle();
  assert.equal(window.location.pathname, '/platform/settings', 'lands on the settings surface');
  assert.equal(window.location.search, '', 'Vue handleSettings semantics: no ?section= query');
  assert.equal(document.querySelector('[role="menu"]'), null, 'menu closed after navigation');
});

test('R448-A2: general.allSettings exists with the Vue locale copy in all five locales', () => {
  // frontend/src/i18n/locales/*.ts general.allSettings (zh-CN:2542 et al).
  const expected: Record<string, string> = {
    'zh-CN': '全部设置',
    'en-US': 'All Settings',
    'ja-JP': 'すべての設定',
    'ko-KR': '모든 설정',
    'ru-RU': 'Все настройки',
  };
  for (const locale of supportedLocales) {
    assert.equal(messages[locale]['general.allSettings'], expected[locale], `${locale} copy matches Vue`);
    assert.equal(formatMessage(locale, 'general.allSettings'), expected[locale]);
  }
});
