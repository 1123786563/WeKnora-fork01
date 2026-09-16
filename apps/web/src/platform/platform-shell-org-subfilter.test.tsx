// PlatformShell organizations sub-filter tests (jsdom + react-dom), following
// the harness in apps/web/src/platform/platform-shell-guide-reopen.test.tsx.
//
// Slice: R017 shell sub-filter closeout. Vue parity baseline —
//   frontend/src/views/organization/OrganizationList.vue:3-4 (ListSpaceSidebar
//   mode="organization" — the shared-space scope rail: 全部 / 我创建的 /
//   我加入的, backed by spaceSelection 'all' | 'created' | 'joined').
//   Vue menu.vue itself has NO organizations submenu (only the
//   pending-join-requests badge on the nav entry); the scope rail belongs to
//   OrganizationList.vue, not the shell.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') || specifier.endsWith('.png')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/organizations' });
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

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
  window.history.replaceState({}, '', '/platform/organizations');
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

function fakeClient(): Record<string, unknown> {
  return {
    auth: { me: async () => ({ user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '' } }) },
    sessions: {
      list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }),
    },
  };
}

async function mountShell(atPath: string): Promise<HTMLElement> {
  window.history.replaceState({}, '', atPath);
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

// The old shell quick-filter block is intentionally absent; the page rail
// owns these controls, as in Vue.
const orgFilters = (root: HTMLElement) => root.querySelector('[role="navigation"][aria-label="共享空间"]');

test('organizations route leaves scope filtering to the page rail', async () => {
  const root = await mountShell('/platform/organizations');
  assert.equal(orgFilters(root), null, 'Vue menu does not render an organizations submenu');
});

test('scope query does not create duplicate shell controls', async () => {
  const root = await mountShell('/platform/organizations?scope=created');
  assert.equal(orgFilters(root), null);
});

test('other routes keep the shell clean: KB page has no duplicate scope block', async () => {
  const kbRoot = await mountShell('/platform/knowledge-bases');
  assert.equal(orgFilters(kbRoot), null, 'org sub-filter must not render outside /platform/organizations');
  assert.equal(kbRoot.querySelector('[role="navigation"][aria-label="知识库"]'), null);

  const chatRoot = await mountShell('/platform/creatChat');
  assert.equal(orgFilters(chatRoot), null);
  assert.equal(chatRoot.querySelector('[role="navigation"]'), null);
});

test('collapsed sidebar hides the org sub-filter like the other shell blocks', async () => {
  window.localStorage.setItem('weknora_sidebar_collapsed', 'true');
  const root = await mountShell('/platform/organizations');
  assert.equal(orgFilters(root), null, 'collapsed sidebar hides the org sub-filter');
});

// — RBAC entry visibility (Vue menu.ts:72-81) ——————————————————————————————
// Vue hides the organizations entry below admin (viewer/contributor see no
// shared-space management entry); the can_access_all_tenants superuser flag
// passes the gate. Me payloads follow AuthMe (user/tenant/memberships);
// membership rows carry { tenant_id, role }. Assertions are collect-then-
// assert (Round N+8): query the DOM once after settle, assert on plain
// values — no awaits between mount and assert to avoid act-loop hangs.

function fakeClientWithMe(me: Record<string, unknown>): Record<string, unknown> {
  return {
    auth: { me: async () => me },
    sessions: {
      list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }),
    },
  };
}

async function mountShellWithMe(atPath: string, me: Record<string, unknown>): Promise<HTMLElement> {
  window.history.replaceState({}, '', atPath);
  window.localStorage.setItem('weknora:new-user-guide-done:v1', '1');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(
      PlatformShell,
      { client: fakeClientWithMe(me) as never, onLogout: () => undefined, children: React.createElement('div', null, 'page') },
    ));
  });
  await settle(20);
  return container;
}

const navHrefs = (root: HTMLElement): string[] =>
  [...root.querySelectorAll('nav[aria-label="Platform"] a')]
    .map((a) => (a instanceof dom.window.HTMLAnchorElement ? a.getAttribute('href') : null))
    .filter((href): href is string => typeof href === 'string');

const ORG_HREF = '/platform/organizations';

test('viewer membership hides the organizations nav entry (Vue menu.ts:76-78)', async () => {
  const root = await mountShellWithMe('/platform/knowledge-bases', {
    user: { id: 'u1', username: 'viewer', email: 'viewer@local.dev', avatar: '' },
    tenant: { id: 7, name: 'Home' },
    memberships: [{ tenant_id: 7, role: 'viewer' }],
  });
  const hrefs = navHrefs(root);
  assert.ok(!hrefs.includes(ORG_HREF), 'viewer must not see the organizations entry');
  assert.ok(hrefs.includes('/platform/knowledge-bases'), 'other nav entries stay visible');
});

test('admin membership keeps the organizations nav entry visible', async () => {
  const root = await mountShellWithMe('/platform/knowledge-bases', {
    user: { id: 'u1', username: 'admin', email: 'admin@local.dev', avatar: '' },
    tenant: { id: 7, name: 'Home' },
    memberships: [{ tenant_id: 7, role: 'admin' }],
  });
  const hrefs = navHrefs(root);
  assert.ok(hrefs.includes(ORG_HREF), 'admin must see the organizations entry');
});

test('viewer membership with can_access_all_tenants keeps organizations visible (superuser)', async () => {
  const root = await mountShellWithMe('/platform/knowledge-bases', {
    user: { id: 'u1', username: 'super', email: 'super@local.dev', avatar: '', can_access_all_tenants: true },
    tenant: { id: 7, name: 'Home' },
    memberships: [{ tenant_id: 7, role: 'viewer' }],
  });
  const hrefs = navHrefs(root);
  assert.ok(hrefs.includes(ORG_HREF), 'superuser flag must pass the admin gate');
});
