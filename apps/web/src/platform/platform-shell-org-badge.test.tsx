// R443-A3 platform shell leftovers from R442:
// 1. organizations nav pending-join badge (Vue menu.vue:93-98 +
//    stores/organization.ts:100-102 totalPendingJoinRequestCount).
// 2. tenant submenu throttled memberships refresh (Vue UserMenu.vue:430-443,
//    TENANT_SUBMENU_MEMBERSHIP_REFRESH_MS = 2000).
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
const { PlatformShell } = await import('./PlatformShell.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

const settle = (ms = 10) => act(async () => { await new Promise((resolve) => setTimeout(resolve, ms)); });
const waitPastThrottleWindow = () => act(async () => { await new Promise((resolve) => setTimeout(resolve, 2050)); });

type Membership = { tenant_id: number; tenant_name: string; role: string };
type OrgRow = { id: string; name: string; pending_join_request_count?: number };

function fakeClient(options: {
  orgs?: OrgRow[];
  orgsReject?: boolean;
  memberships?: Membership[];
  canAccessAllTenants?: boolean;
} = {}) {
  let meCallCount = 0;
  let memberships = options.memberships ?? [{ tenant_id: 1, tenant_name: 'Home', role: 'owner' }];
  const meBase = async () => {
    meCallCount += 1;
    return {
      user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '', can_access_all_tenants: options.canAccessAllTenants ?? true },
      tenant: { id: 1, name: 'Home' },
      memberships,
    };
  };
  const client = {
    auth: { me: meBase },
    sessions: { list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }) },
    organizations: {
      list: async () => {
        if (options.orgsReject) throw new Error('organizations unavailable');
        return { items: options.orgs ?? [], total: options.orgs?.length ?? 0, resourceCounts: undefined };
      },
    },
  };
  return {
    client,
    meCallCount: () => meCallCount,
    setMemberships: (next: Membership[]) => { memberships = next; },
    failNextMe: () => {
      // Next invocation rejects (Vue refreshFromAuthMe swallows the error);
      // later calls recover so tests can probe the post-failure state.
      client.auth.me = async () => {
        client.auth.me = meBase;
        throw new Error('network down');
      };
    },
  };
}

async function mount(handle: ReturnType<typeof fakeClient>) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => mountedRoot?.render(React.createElement(PlatformShell, {
    client: handle.client as never,
    onLogout: () => undefined,
    onTenantSwitch: async () => undefined,
    children: React.createElement('div', null, 'page'),
  })));
  await settle(20);
}

const orgNavLink = () => document.querySelector('a[href="/platform/organizations"]');

async function openUserMenu() {
  const userMenu = document.querySelector('[data-guide="user-menu"]') as HTMLButtonElement | null;
  assert.ok(userMenu, 'user menu button exists');
  await act(async () => userMenu.click());
  await settle();
}

async function toggleTenantSubmenu() {
  const toggle = Array.from(document.querySelectorAll('button[aria-expanded]'))
    .find((b) => b.textContent?.includes('切换空间')) as HTMLButtonElement | undefined;
  assert.ok(toggle, 'tenant submenu toggle exists');
  await act(async () => toggle.click());
  await settle(20);
  return toggle;
}

test('organizations nav shows pending-join badge with the raw total (no 9+ cap)', async () => {
  await mount(fakeClient({
    orgs: [
      { id: 'o1', name: 'Alpha', pending_join_request_count: 3 },
      { id: 'o2', name: 'Beta', pending_join_request_count: 9 },
      { id: 'o3', name: 'Gamma' },
    ],
  }));
  const link = orgNavLink();
  assert.ok(link, 'organizations nav entry renders');
  const badge = link.querySelector('[data-testid="org-pending-badge"]');
  assert.ok(badge, 'pending badge renders inside the organizations entry');
  // Vue renders the raw count — no 9+ truncation.
  assert.equal(badge.textContent, '12');
  assert.equal(badge.getAttribute('title'), '有待审批的加入申请');
});

test('pending badge is hidden when the total is zero', async () => {
  await mount(fakeClient({ orgs: [{ id: 'o1', name: 'Alpha', pending_join_request_count: 0 }] }));
  const link = orgNavLink();
  assert.ok(link, 'organizations nav entry renders');
  assert.equal(link.querySelector('[data-testid="org-pending-badge"]'), null);
});

test('organizations list failure degrades to a hidden badge without crashing', async () => {
  await mount(fakeClient({ orgsReject: true }));
  const link = orgNavLink();
  assert.ok(link, 'organizations nav entry still renders');
  assert.equal(link.querySelector('[data-testid="org-pending-badge"]'), null);
});

test('pending badge is hidden in the collapsed rail (expanded-only, like Vue)', async () => {
  window.localStorage.setItem('sidebar_collapsed', 'true');
  await mount(fakeClient({ orgs: [{ id: 'o1', name: 'Alpha', pending_join_request_count: 5 }] }));
  const link = orgNavLink();
  assert.ok(link, 'organizations nav entry renders in the rail');
  assert.equal(link.querySelector('[data-testid="org-pending-badge"]'), null);
});

test('tenant submenu open refreshes memberships at most once per 2s', async () => {
  const handle = fakeClient({
    memberships: [{ tenant_id: 1, tenant_name: 'Home', role: 'owner' }],
  });
  await mount(handle);
  const afterMount = handle.meCallCount();
  assert.ok(afterMount >= 1, 'initial auth/me resolves');

  await openUserMenu();
  await toggleTenantSubmenu();
  assert.equal(handle.meCallCount(), afterMount + 1, 'first submenu open refreshes memberships');
  assert.ok(document.querySelector('[role="listbox"]'), 'tenant submenu lists memberships');

  // Close and reopen within the 2s window: Vue's timestamp throttle skips.
  await toggleTenantSubmenu();
  await toggleTenantSubmenu();
  assert.equal(handle.meCallCount(), afterMount + 1, 'immediate reopen is throttled');

  // Past the window the next open refreshes again.
  handle.setMemberships([
    { tenant_id: 1, tenant_name: 'Home', role: 'owner' },
    { tenant_id: 2, tenant_name: 'Peer', role: 'contributor' },
  ]);
  await waitPastThrottleWindow();
  await toggleTenantSubmenu(); // close
  await toggleTenantSubmenu(); // reopen
  assert.equal(handle.meCallCount(), afterMount + 2, 'reopen after the window refreshes again');
  const options = Array.from(document.querySelectorAll('[role="option"]'));
  assert.ok(options.some((o) => o.textContent?.includes('Peer')), 'refreshed membership appears in the submenu');
});

test('failed submenu refresh keeps the last-known membership list', async () => {
  const handle = fakeClient({
    memberships: [{ tenant_id: 1, tenant_name: 'Home', role: 'owner' }],
  });
  await mount(handle);
  handle.setMemberships([
    { tenant_id: 1, tenant_name: 'Home', role: 'owner' },
    { tenant_id: 2, tenant_name: 'Peer', role: 'contributor' },
  ]);
  handle.failNextMe();

  await openUserMenu();
  const toggle = await toggleTenantSubmenu();
  assert.ok(document.querySelector('[role="option"]'), 'submenu still renders after failed refresh');

  await waitPastThrottleWindow();
  await toggleTenantSubmenu(); // close
  await toggleTenantSubmenu(); // reopen past the window
  const options = Array.from(document.querySelectorAll('[role="option"]'));
  assert.equal(options.length, 2, 'last-known membership list survives the failed refresh');
  assert.ok(options.some((o) => o.textContent?.includes('Peer')), 'cached Peer membership stays listed');
});
