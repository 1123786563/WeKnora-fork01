// R450-A2 — parity for the Vue `weknora_lite_mode` gating that R449 left
// open. Vue sources the flag from localStorage ('weknora_lite_mode' ===
// 'true', stores/auth.ts:538) plus the system-info edition probe, and gates
// the shell chrome with it:
//   - UserMenu.vue:56 + 247-253  tenant identity line / panel  (!isLiteMode)
//   - UserMenu.vue:81            「空间设置」 quick link          (!isLiteMode)
//   - UserMenu.vue:136-144       divider + logout item           (!isLiteMode)
//   - UserMenu.vue:249+          canManage* admin shortcuts      (!isLiteMode)
//   - stores/menu.ts:64,73       sidebar logout/organizations    (liteHiddenPaths)
// The React shell already renders the Lite edition mark (PlatformShell
// logo, Vue menu.vue:7) from the same key; these tests pin the menu/nav
// gating on the same localStorage source (equivalent acquisition — no
// Vue-specific build system involved).
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
  HTMLAnchorElement: dom.window.HTMLAnchorElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
try { Object.defineProperty(dom.window.navigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep jsdom default */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { PlatformShell } = await import('./PlatformShell.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

const settle = (ms = 10) => act(async () => { await new Promise((resolve) => setTimeout(resolve, ms)); });

function fakeClient(options?: { canAccessAllTenants?: boolean; memberships?: number }) {
  const memberships = Array.from({ length: options?.memberships ?? 1 }, (_, i) => ({
    tenant_id: i + 1,
    tenant_name: i === 0 ? 'Home' : `Space ${i + 1}`,
    role: 'owner',
  }));
  return {
    auth: {
      me: async () => ({
        user: {
          id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '',
          can_access_all_tenants: options?.canAccessAllTenants ?? false,
        },
        tenant: { id: 1, name: 'Home' },
        memberships,
      }),
    },
    sessions: { list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }) },
    organizations: { list: async () => ({ items: [], total: 0, resourceCounts: undefined }) },
  };
}

async function mount(clientOptions?: { canAccessAllTenants?: boolean; memberships?: number }) {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  document.body.replaceChildren();
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => mountedRoot?.render(React.createElement(PlatformShell, {
    client: fakeClient(clientOptions) as never,
    onLogout: () => undefined,
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

function menuItems(): HTMLElement[] {
  const menu = document.querySelector('[role="menu"]');
  assert.ok(menu, 'user dropdown renders');
  return Array.from(menu.querySelectorAll('[role="menuitem"]'));
}

// Vue UserMenu.vue:81 — the 「空间设置」 quick link hides in lite mode.
test('R450-A2: lite mode hides the workspace settings quick link and logout, keeps personal settings', async () => {
  window.localStorage.setItem('weknora_lite_mode', 'true');
  await mount();
  await openUserMenu();
  const items = menuItems();
  const texts = items.map((el) => el.textContent ?? '');
  assert.ok(texts.some((t) => t.includes('个人设置')), 'personal settings entry stays (Vue UserMenu.vue:74-78 has no lite gate)');
  assert.ok(!texts.some((t) => t.includes('空间设置')), '空间设置 quick link is gated off in lite mode');
  assert.ok(!texts.some((t) => t.includes('退出')), 'logout entry is gated off in lite mode (Vue UserMenu.vue:136-144)');
});

// Vue UserMenu.vue:249+ — admin shortcuts (members/models/skills) render
// only through canManage*, which returns false in lite mode ("Lite 模式下
// 没有 RBAC 概念，统一隐藏").
test('R450-A2: lite mode hides the admin shortcut entries', async () => {
  await mount({ canAccessAllTenants: true });
  await openUserMenu();
  const liteHrefs = menuItems()
    .filter((el) => el instanceof HTMLAnchorElement)
    .map((el) => (el as HTMLAnchorElement).getAttribute('href') ?? '');
  assert.ok(liteHrefs.some((h) => h.includes('section=members')), 'precondition: members shortcut renders outside lite mode');
  window.localStorage.setItem('weknora_lite_mode', 'true');
  await mount({ canAccessAllTenants: true });
  await openUserMenu();
  const hrefs = menuItems()
    .filter((el) => el instanceof HTMLAnchorElement)
    .map((el) => (el as HTMLAnchorElement).getAttribute('href') ?? '');
  for (const section of ['members', 'models', 'skills']) {
    assert.ok(!hrefs.some((h) => h.includes(`section=${section}`)), `${section} shortcut is gated off in lite mode`);
  }
});

// Vue UserMenu.vue:56 + 247-253 — the tenant identity line (and the switcher
// group that lives in the same panel) collapse in lite mode; the button falls
// back to the plain name/email two-line layout.
test('R450-A2: lite mode hides the tenant identity line and the switcher group', async () => {
  await mount({ canAccessAllTenants: true });
  const button = document.querySelector('[data-guide="user-menu"]') as HTMLButtonElement;
  assert.ok((button.textContent ?? '').includes('Home'), 'precondition: identity line renders outside lite mode');
  window.localStorage.setItem('weknora_lite_mode', 'true');
  await mount({ canAccessAllTenants: true });
  const liteButton = document.querySelector('[data-guide="user-menu"]') as HTMLButtonElement;
  assert.ok(!(liteButton.textContent ?? '').includes('Home'), 'identity line hidden in lite mode');
  assert.equal(document.querySelector('[role="group"]'), null, 'tenant switcher group hidden in lite mode');
});

// Vue stores/menu.ts:64,73 — the sidebar hides logout/organizations entries
// in lite mode; the React rail owns the organizations entry.
test('R450-A2: lite mode drops the organizations rail entry', async () => {
  await mount({ canAccessAllTenants: true });
  assert.ok(document.querySelector('a[href="/platform/organizations"]'), 'precondition: organizations rail entry renders outside lite mode');
  window.localStorage.setItem('weknora_lite_mode', 'true');
  await mount({ canAccessAllTenants: true });
  assert.equal(document.querySelector('a[href="/platform/organizations"]'), null, 'organizations rail entry hidden in lite mode');
});

// Regression: without the localStorage key nothing changes.
test('R450-A2: without the flag the menu keeps workspace settings and logout', async () => {
  await mount();
  await openUserMenu();
  const texts = menuItems().map((el) => el.textContent ?? '');
  assert.ok(texts.some((t) => t.includes('空间设置')), 'workspace settings renders');
  assert.ok(texts.some((t) => t.includes('退出')), 'logout renders');
});
