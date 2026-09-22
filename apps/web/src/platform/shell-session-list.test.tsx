// Platform shell session-list tests (jsdom + react-dom), following the
// harness in apps/web/src/platform/new-user-guide.test.tsx.
//
// Slice: the Vue session list lives IN the platform sidebar on every protected
// page (frontend/src/components/menu.vue .submenu) — grouped by date, active
// row driven by the route, hover ⋯ menu with pin/rename/clear/delete. The
// React shell now renders that list (SessionSidebarList) and the in-page chat
// sidebar must NOT duplicate it on chat routes (Vue chat/index.vue has no
// sidebar of its own).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// PlatformShell imports .css files; teach the ESM loader to treat them as
// empty modules (same approach as new-user-guide.test.tsx).
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.png')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/chat/session-2' });
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
// Shadow the jsdom navigator's language (prototype getter) before react-dom loads.
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default locale */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { PlatformShell } = await import('./PlatformShell.tsx');
const { ChatPage } = await import('@weknora/views');

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

const NOW = new Date();
const iso = (msAgo: number) => new Date(NOW.getTime() - msAgo).toISOString();
const DAY = 24 * 60 * 60 * 1000;

const SESSIONS = [
  { id: 'session-pin', title: '置顶的会话', is_pinned: true, updated_at: iso(0) },
  { id: 'session-1', title: '今天的会话', is_pinned: false, updated_at: iso(0) },
  { id: 'session-2', title: '昨天的会话', is_pinned: false, updated_at: iso(DAY) },
  { id: 'session-3', title: '更早的会话', is_pinned: false, updated_at: iso(40 * DAY) },
  // Anchor the untitled session to yesterday 12:00 local: a raw '25h ago'
  // crosses the 昨天/近7天 calendar bucket when the suite runs just after
  // midnight, which is time-of-day flakiness, not a product change.
  { id: 'session-4', title: '', is_pinned: false, updated_at: (() => { const d = new Date(NOW); d.setDate(d.getDate() - 1); d.setHours(12, 0, 0, 0); return d.toISOString(); })() },
];

function fakeClient(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  const { embed, auth, ...sessionOverrides } = overrides;
  return {
    auth: auth ?? { me: async () => ({ user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '' } }) },
    sessions: {
      list: async () => ({ data: SESSIONS, total: SESSIONS.length, page: 1, page_size: 30 }),
      pin: async () => undefined,
      unpin: async () => undefined,
      update: async (sessionId: string, input: { title: string }) => ({ ...SESSIONS.find((s) => s.id === sessionId), title: input.title }),
      clear: async () => undefined,
      remove: async () => undefined,
      batchRemove: async () => undefined,
      ...sessionOverrides,
    },
    embed: embed ?? { channels: { listAll: async () => [] }, im: { listAll: async () => [] } },
  };
}

async function mountShell(options: { client?: Record<string, unknown>; children?: React.ReactNode } = {}) {
  window.localStorage.setItem('weknora:new-user-guide-done:v1', '1');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  const client = options.client ?? fakeClient();
  await act(async () => {
    mountedRoot?.render(React.createElement(
      PlatformShell,
      {
        client: client as never,
        onLogout: () => undefined,
        children: options.children ?? React.createElement('div', { 'data-testid': 'outlet-page' }, 'page'),
      },
    ));
  });
  await settle(20);
  return container;
}

// Task 9.5 — the session list DOM mirrors Vue menu.vue/SessionSidebarRow.vue
// (classes ported in platform-shell.td.css); queries ride the Vue class
// vocabulary: group headers as .timeline_header, titles as .submenu_title-text,
// rows as .session-chat-row > .session-list-row > .__body > .submenu_item.
const shellList = () => document.querySelector('nav[aria-label="我的对话"]');
const groupHeaders = () => [...document.querySelectorAll('nav[aria-label="我的对话"] .timeline_header')].map((node) => node.textContent);
const rowTitles = () => [...document.querySelectorAll('nav[aria-label="我的对话"] .submenu_title-text')].map((node) => node.textContent);

test('(a) shell sidebar renders the grouped session list on a chat route', async () => {
  await mountShell();
  assert.ok(shellList(), 'expected the shell session list region');
  assert.deepEqual(groupHeaders(), ['已置顶', '今天', '昨天', '更早']);
  // Untitled rows render menu.newSession (新会话) like Vue mapSessionRow.
  assert.deepEqual(rowTitles(), ['置顶的会话', '今天的会话', '昨天的会话', '新会话', '更早的会话']);
});

test('(b) active session follows the route (/platform/chat/:id)', async () => {
  await mountShell();
  const activeButtons = [...document.querySelectorAll('nav[aria-label="我的对话"] .submenu_item[aria-current="page"]')];
  assert.equal(activeButtons.length, 1, 'exactly one active row');
  assert.equal(activeButtons[0]?.getAttribute('aria-current'), 'page');
  assert.equal(activeButtons[0]?.textContent?.includes('昨天的会话'), true, 'session-2 (from the URL) is the active row');
});

test('(c) selecting a session on a chat route asks the chat page for an in-place route switch', async () => {
  await mountShell();
  const events: string[] = [];
  const listener = (event: Event) => {
    events.push((event as CustomEvent<{ sessionId?: string }>).detail?.sessionId ?? '');
  };
  window.addEventListener('weknora:session-route-change', listener);
  const target = [...document.querySelectorAll('nav[aria-label="我的对话"] .session-chat-row .submenu_item')]
    .find((node) => node.querySelector('.submenu_title-text')?.textContent === '今天的会话') as HTMLElement;
  assert.ok(target);
  await act(async () => { target.click(); await new Promise((resolve) => setTimeout(resolve, 5)); });
  window.removeEventListener('weknora:session-route-change', listener);
  assert.deepEqual(events, ['session-1'], 'shell dispatches the session route-change event with the target id');
});

test('(d) row ⋯ menu wires 置顶/取消置顶/清空消息/删除记录 to the session API actions', async () => {
  const calls: string[] = [];
  const client = fakeClient({
    pin: async (sessionId: string) => { calls.push(`pin:${sessionId}`); },
    unpin: async (sessionId: string) => { calls.push(`unpin:${sessionId}`); },
    clear: async (sessionId: string) => { calls.push(`clear:${sessionId}`); },
    remove: async (sessionId: string) => { calls.push(`remove:${sessionId}`); },
  });
  const originalConfirm = window.confirm;
  let nativeConfirmCalls = 0;
  window.confirm = () => { nativeConfirmCalls += 1; return true; };
  try {
    await mountShell({ client });
    const menus = [...document.querySelectorAll('nav[aria-label="我的对话"] .session-chat-row details')];
    assert.equal(menus.length, 5, 'each row carries the hover ⋯ menu');
    const menuOf = (title: string) => {
      const row = [...document.querySelectorAll('nav[aria-label="我的对话"] .session-chat-row')]
        .find((node) => node.textContent?.includes(title));
      return row?.querySelector('details') as HTMLDetailsElement | null;
    };
    // Pin the unpinned 今天的会话.
    const menu1 = menuOf('今天的会话');
    assert.ok(menu1);
    await act(async () => { (menu1 as HTMLDetailsElement).open = true; await settle(2); });
    const pinItem = [...menu1.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '置顶');
    assert.ok(pinItem, 'expected the 置顶 menu item');
    await act(async () => { (pinItem as HTMLButtonElement).click(); await settle(5); });
    assert.ok(calls.includes('pin:session-1'), `expected pin:session-1, saw ${calls.join(',')}`);
    // Unpin the pinned row.
    const menuPinned = menuOf('置顶的会话');
    assert.ok(menuPinned);
    (menuPinned as HTMLDetailsElement).open = true;
    const unpinItem = [...menuPinned.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '取消置顶');
    assert.ok(unpinItem, 'expected the 取消置顶 menu item');
    await act(async () => { (unpinItem as HTMLButtonElement).click(); await settle(5); });
    assert.ok(calls.includes('unpin:session-pin'));
    // Clear + delete use Vue-equivalent confirms.
    const menu3 = menuOf('更早的会话');
    assert.ok(menu3);
    (menu3 as HTMLDetailsElement).open = true;
    const clearItem = [...menu3.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '清空消息');
    assert.ok(clearItem, 'expected the 清空消息 menu item');
    await act(async () => { (clearItem as HTMLButtonElement).click(); await settle(5); });
    const clearConfirm = menu3.querySelector('.wk-chat-session-confirm button:last-child');
    assert.ok(clearConfirm, 'expected the clear confirmation action');
    await act(async () => { (clearConfirm as HTMLButtonElement).click(); await settle(5); });
    assert.ok(calls.includes('clear:session-3'));
    assert.equal(nativeConfirmCalls, 0, 'session danger actions use the row confirmation only');
    // Vue menu.vue row menu uses upload.deleteRecord (删除记录), not chatHeader.deleteSession.
    const deleteItem = [...menu3.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '删除记录');
    assert.ok(deleteItem, 'expected the 删除记录 menu item');
    await act(async () => { (deleteItem as HTMLButtonElement).click(); await settle(5); });
    const deleteConfirm = menu3.querySelector('.wk-chat-session-confirm button:last-child');
    assert.ok(deleteConfirm, 'expected the delete confirmation action');
    await act(async () => { (deleteConfirm as HTMLButtonElement).click(); await settle(5); });
    assert.ok(calls.includes('remove:session-3'));
    assert.equal(nativeConfirmCalls, 0, 'session danger actions use the row confirmation only');
  } finally {
    window.confirm = originalConfirm;
  }
});

test('(e) deleting a non-active session only refreshes the shell list; no in-page duplicate sidebar renders under the shell', async () => {
  const removed: string[] = [];
  const client = fakeClient({ remove: async (sessionId: string) => { removed.push(sessionId); } });
  const originalConfirm = window.confirm;
  window.confirm = () => true;
  try {
    const chatPage = React.createElement(ChatPage, {
      sessions: [{ id: 'session-2', title: '昨天的会话', is_pinned: false }],
      selectedSessionId: 'session-2',
      messages: [],
      draft: '',
      onSelectSession: () => undefined,
      onCreateSession: () => undefined,
      onDraftChange: () => undefined,
      send: async () => undefined,
    });
    await mountShell({ client, children: chatPage });
    // The chat page's own sidebar must be suppressed by the shell context…
    assert.equal(document.querySelectorAll('aside[aria-label="我的对话"]').length, 0, 'chat page must not render its own sidebar under the shell');
    // …while the shell list renders exactly one session list.
    assert.equal(document.querySelectorAll('nav[aria-label="我的对话"]').length, 1);
    // Deleting a non-active session removes it from the shell list without navigation.
    const menu3 = [...document.querySelectorAll('nav[aria-label="我的对话"] .session-chat-row')]
      .find((node) => node.textContent?.includes('更早的会话'))
      ?.querySelector('details') as HTMLDetailsElement;
    assert.ok(menu3);
    menu3.open = true;
    const deleteItem = [...menu3.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '删除记录') as HTMLButtonElement;
    assert.ok(deleteItem);
    await act(async () => { deleteItem.click(); await settle(5); });
    const deleteConfirm = menu3.querySelector('.wk-chat-session-confirm button:last-child');
    assert.ok(deleteConfirm);
    await act(async () => { (deleteConfirm as HTMLButtonElement).click(); await settle(5); });
    assert.deepEqual(removed, ['session-3']);
    assert.equal(rowTitles().includes('更早的会话'), false, 'deleted session leaves the shell list');
    assert.equal(window.location.pathname, '/platform/chat/session-2', 'active route is untouched');
  } finally {
    window.confirm = originalConfirm;
  }
});

test('(f) empty session list renders the zh-CN 暂无对话 empty state', async () => {
  const client = fakeClient();
  (client.sessions as Record<string, unknown>).list = async () => ({ data: [], total: 0, page: 1, page_size: 30 });
  await mountShell({ client });
  assert.ok(shellList());
  const empty = document.querySelector('nav[aria-label="我的对话"] p[role="status"]');
  assert.equal(empty?.textContent, '暂无对话');
});

test('(g) shell session list requests the next web page when scrolled near the end', async () => {
  const calls: number[] = [];
  const client = fakeClient({
    list: async (params: { page?: number }) => {
      const page = params.page ?? 1;
      calls.push(page);
      return page === 1
        ? { data: SESSIONS, total: 6, page: 1, page_size: 5 }
        : { data: [{ id: 'session-6', title: '下一页的会话', is_pinned: false, updated_at: iso(0) }], total: 6, page: 2, page_size: 1 };
    },
  });
  await mountShell({ client });
  const scrollContainer = shellList()?.parentElement as HTMLElement;
  assert.ok(scrollContainer);
  Object.defineProperties(scrollContainer, {
    scrollHeight: { configurable: true, value: 400 },
    clientHeight: { configurable: true, value: 200 },
    scrollTop: { configurable: true, writable: true, value: 220 },
  });
  await act(async () => { scrollContainer.dispatchEvent(new Event('scroll')); await new Promise((resolve) => setTimeout(resolve, 10)); });
  assert.deepEqual(calls, [1, 2]);
  assert.equal(rowTitles().includes('下一页的会话'), true);
});

test('(h) a failed initial session load shows retry instead of the empty state', async () => {
  let attempts = 0;
  const client = fakeClient({
    list: async () => {
      attempts += 1;
      if (attempts === 1) throw new Error('session list unavailable');
      return { data: SESSIONS, total: SESSIONS.length, page: 1, page_size: 30 };
    },
  });
  await mountShell({ client });
  const status = document.querySelector('nav[aria-label="我的对话"] p[role="status"]');
  assert.equal(status?.textContent, '发生错误 重试');
  assert.equal(status?.textContent?.includes('暂无对话'), false);
  const retry = status?.querySelector('button') as HTMLButtonElement | null;
  assert.ok(retry);
  await act(async () => { retry?.click(); await new Promise((resolve) => setTimeout(resolve, 10)); });
  assert.equal(attempts, 2);
  assert.equal(rowTitles().includes('今天的会话'), true);
});

test('(i) shell session source filter exposes Vue-backed web, API, and configured channel buckets', async () => {
  const sourceCalls: string[] = [];
  const metadataCalls: string[] = [];
  const client = fakeClient({
    auth: { me: async () => ({ user: { id: 'u1', username: 'admin', email: '', avatar: '' }, tenant: { id: 'tenant-1' }, memberships: [{ tenant_id: 'tenant-1', role: 'admin' }] }) },
    list: async (params: { source?: string }) => { sourceCalls.push(params.source ?? ''); return { data: [], total: 1, page: 1, page_size: 30 }; },
    embed: {
      channels: { listAll: async () => { metadataCalls.push('embed'); return [{ id: 'embed-1', name: '帮助中心' }]; } },
      im: { listAll: async () => { metadataCalls.push('im'); return [{ id: 'im-1', platform: 'feishu' }]; } },
    },
  });
  const container = await mountShell({ client });
  const filter = container.querySelector('nav[aria-label="我的对话"] select[aria-label="会话来源"]') as HTMLSelectElement | null;
  assert.ok(filter, 'expected the source filter select');
  assert.deepEqual(metadataCalls.sort(), ['embed', 'im']);
  assert.deepEqual([...filter.options].map((option) => option.value), ['web', 'api', 'embed:embed-1', 'im:feishu']);
  assert.deepEqual([...filter.options].map((option) => option.textContent), ['我的对话', 'API 会话', '帮助中心', 'feishu']);
  await act(async () => {
    filter.value = 'api';
    filter.dispatchEvent(new Event('change', { bubbles: true }));
    await settle(40);
  });
  // The second 'web' is the reload once auth/me resolves the admin role: the
  // scope change must restart the load (it used to strand the list on
  // Loading), then the explicit bucket switch appends its own request.
  assert.deepEqual(sourceCalls, ['web', 'web', 'api', 'embed:embed-1', 'feishu', 'api']);
});

test('(j) viewers and unknown mounts keep admin session sources hidden', async () => {
  const client = fakeClient({
    auth: { me: async () => ({ user: { id: 'u1', username: 'viewer', email: '', avatar: '' }, tenant: { id: 'tenant-1' }, memberships: [{ tenant_id: 'tenant-1', role: 'viewer' }] }) },
    embed: { channels: { listAll: async () => [{ id: 'embed-1', name: '帮助中心' }] }, im: { listAll: async () => [{ id: 'im-1', platform: 'feishu' }] } },
  });
  const container = await mountShell({ client });
  assert.equal(container.querySelector('nav[aria-label="我的对话"] select[aria-label="会话来源"]'), null);
});

test('(k) admin source filter stays hidden when API and configured channels have no sessions', async () => {
  const client = fakeClient({
    auth: { me: async () => ({ user: { id: 'u1', username: 'admin', email: '', avatar: '' }, tenant: { id: 'tenant-1' }, memberships: [{ tenant_id: 'tenant-1', role: 'admin' }] }) },
    embed: { channels: { listAll: async () => [{ id: 'embed-1', name: '帮助中心' }] }, im: { listAll: async () => [{ id: 'im-1', platform: 'feishu' }] } },
    list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }),
  });
  const container = await mountShell({ client });
  assert.equal(container.querySelector('nav[aria-label="我的对话"] select[aria-label="会话来源"]'), null);
});

test('(l) replacing an admin client with a viewer client resets an invalid source to web before loading', async () => {
  const adminCalls: string[] = [];
  const viewerCalls: string[] = [];
  const admin = fakeClient({
    auth: { me: async () => ({ user: { id: 'u1', username: 'admin', email: '', avatar: '' }, tenant: { id: 'tenant-1' }, memberships: [{ tenant_id: 'tenant-1', role: 'admin' }] }) },
    embed: { channels: { listAll: async () => [] }, im: { listAll: async () => [] } },
    list: async (params: { source?: string }) => { adminCalls.push(params.source ?? ''); return { data: [], total: 1, page: 1, page_size: 30 }; },
  });
  const viewer = fakeClient({
    auth: { me: async () => ({ user: { id: 'u1', username: 'viewer', email: '', avatar: '' }, tenant: { id: 'tenant-1' }, memberships: [{ tenant_id: 'tenant-1', role: 'viewer' }] }) },
    list: async (params: { source?: string }) => { viewerCalls.push(params.source ?? ''); return { data: [], total: 0, page: 1, page_size: 30 }; },
  });
  const container = await mountShell({ client: admin });
  const filter = container.querySelector('nav[aria-label="我的对话"] select[aria-label="会话来源"]') as HTMLSelectElement;
  assert.ok(filter);
  await act(async () => { filter.value = 'api'; filter.dispatchEvent(new Event('change', { bubbles: true })); await settle(10); });
  await act(async () => {
    mountedRoot?.render(React.createElement(PlatformShell, { client: viewer as never, onLogout: () => undefined, children: React.createElement('div', null, 'page') }));
    await settle(30);
  });
  // The viewer client must only ever see web-source requests: the client
  // swap resets the invalid 'api' source, and the later auth/me scope
  // resolution reloads web once more (same restart semantics as (i)/(l2)).
  assert.ok(viewerCalls.length > 0, 'expected the viewer client to load the web bucket');
  assert.ok(viewerCalls.every((source) => source === 'web'), `viewer client must only request web, saw ${viewerCalls.join(',')}`);
  assert.equal(container.querySelector('nav[aria-label="我的对话"] select[aria-label="会话来源"]'), null);
});

test('(l2) admin auth/me scope flip during the first load must not strand the list on Loading', async () => {
  // R428 parity bug: the shell mounts before auth/me lands with
  // canSeeAdminSessionSources=false; when me resolves with an admin role the
  // session effect used to early-return on the scope change while the first
  // in-flight request's finally skipped setSessionsLoading(false)
  // (sessionsMountedRef already false) — the sidebar stayed on "Loading..."
  // forever even though the request succeeded. The source is already web, so
  // the effect must fall through and reload with the new generation.
  let listCalls = 0;
  let releaseFirst: (() => void) | undefined;
  const firstGate = new Promise<void>((resolve) => { releaseFirst = resolve; });
  const client = fakeClient({
    auth: { me: async () => ({ user: { id: 'u1', username: 'admin', email: '', avatar: '' }, tenant: { id: 'tenant-1' }, memberships: [{ tenant_id: 'tenant-1', role: 'admin' }] }) },
    list: async () => {
      listCalls += 1;
      if (listCalls === 1) await firstGate;
      return { data: SESSIONS, total: SESSIONS.length, page: 1, page_size: 30 };
    },
  });
  const container = await mountShell({ client });
  assert.equal(listCalls >= 2, true, `expected the scope flip to restart the load, saw ${listCalls} calls`);
  assert.equal(rowTitles().includes('今天的会话'), true, 'session rows render after the admin scope flip');
  const nav = container.querySelector('nav[aria-label="我的对话"]') as HTMLElement;
  assert.ok(nav);
  assert.equal(nav.textContent?.includes('Loading...'), false, 'no stranded text loading label');
  assert.equal(nav.querySelector('[role="status"]'), null, 'no skeleton or error status remains after success');
  // The stale first-generation response must stay discarded after release.
  releaseFirst?.();
  await settle(10);
  assert.equal(rowTitles().includes('今天的会话'), true);
  assert.equal(nav.querySelector('[role="status"]'), null);
});

test('(m) batch management exposes accessible selection and deletes selected sessions', async () => {
  const removed: string[] = [];
  const client = fakeClient({
    list: async () => ({ data: SESSIONS.filter((session) => !removed.includes(session.id)), total: SESSIONS.length - removed.length, page: 1, page_size: 30 }),
    batchRemove: async (sessionIds: string[]) => { removed.push(...sessionIds); },
  });
  const originalConfirm = window.confirm;
  window.confirm = () => true;
  try {
    const container = await mountShell({ client });
    const menuToggle = container.querySelector('nav[aria-label="我的对话"] details summary') as HTMLElement | null;
    assert.ok(menuToggle, 'expected a session actions menu');
    await act(async () => { menuToggle?.click(); await settle(1); });
    const manage = container.querySelector('nav[aria-label="我的对话"] button[aria-label="批量管理"]') as HTMLButtonElement | null;
    assert.ok(manage);
    await act(async () => { manage?.click(); await settle(2); });
    const selectAll = container.querySelector('nav[aria-label="我的对话"] input[aria-label="全选会话"]') as HTMLInputElement | null;
    assert.ok(selectAll);
    const checkboxes = [...container.querySelectorAll('nav[aria-label="我的对话"] input[type="checkbox"]')]
      .filter((node) => node !== selectAll) as HTMLInputElement[];
    assert.equal(checkboxes.length, SESSIONS.length);
    await act(async () => { checkboxes[0]?.click(); await settle(1); });
    await act(async () => { checkboxes[2]?.click(); await settle(1); });
    const deleteButton = container.querySelector('nav[aria-label="我的对话"] button[aria-label^="删除所选"]') as HTMLButtonElement | null;
    assert.ok(deleteButton);
    assert.equal(deleteButton?.disabled, false);
    await act(async () => { deleteButton?.click(); await settle(10); });
    assert.deepEqual(removed.sort(), ['session-2', 'session-pin']);
    assert.equal(rowTitles().includes('昨天的会话'), false);
    assert.equal(rowTitles().includes('置顶的会话'), false);
  } finally {
    window.confirm = originalConfirm;
  }
});

test('(n) failed batch deletion keeps selection and offers retry', async () => {
  let attempt = 0;
  const removed: string[] = [];
  const client = fakeClient({
    list: async () => ({ data: SESSIONS.filter((session) => !removed.includes(session.id)), total: SESSIONS.length - removed.length, page: 1, page_size: 30 }),
    batchRemove: async (sessionIds: string[]) => { attempt += 1; if (attempt === 1) throw new Error('batch delete failed'); removed.push(...sessionIds); },
  });
  const originalConfirm = window.confirm;
  window.confirm = () => true;
  try {
    const container = await mountShell({ client });
    const menuToggle = container.querySelector('nav[aria-label="我的对话"] details summary') as HTMLElement | null;
    assert.ok(menuToggle, 'expected a session actions menu');
    await act(async () => { menuToggle.click(); await settle(1); });
    await act(async () => { (container.querySelector('nav[aria-label="我的对话"] button[aria-label="批量管理"]') as HTMLButtonElement).click(); await settle(1); });
    const first = container.querySelector('nav[aria-label="我的对话"] input[type="checkbox"]:not([aria-label="全选会话"])') as HTMLInputElement;
    await act(async () => { first.click(); await settle(1); });
    await act(async () => { (container.querySelector('nav[aria-label="我的对话"] button[aria-label^="删除所选"]') as HTMLButtonElement).click(); await settle(10); });
    const status = container.querySelector('nav[aria-label="我的对话"] [role="alert"]');
    assert.equal(status?.textContent?.includes('批量删除失败'), true);
    assert.equal((first as HTMLInputElement).checked, true);
    const retry = container.querySelector('nav[aria-label="我的对话"] button[aria-label="重试"]') as HTMLButtonElement | null;
    assert.ok(retry);
    await act(async () => { retry?.click(); await settle(10); });
    assert.equal(rowTitles().includes('置顶的会话'), false);
  } finally {
    window.confirm = originalConfirm;
  }
});

test('(i) renaming a session uses an inline editor with Vue normalization and commits once on Enter', async () => {
  const updates: Array<{ id: string; title: string }> = [];
  const client = fakeClient({ update: async (id: string, input: { title: string }) => {
    updates.push({ id, title: input.title });
    return { ...SESSIONS.find((s) => s.id === id), title: input.title };
  }});
  const container = await mountShell({ client });
  const row = [...document.querySelectorAll('nav[aria-label="我的对话"] .session-chat-row')].find((node) => node.textContent?.includes('今天的会话'))!;
  const details = row.querySelector('details') as HTMLDetailsElement;
  details.open = true;
  const rename = [...details.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '修改标题') as HTMLButtonElement;
  await act(async () => { rename.click(); await settle(1); });
  const input = row.querySelector('input[aria-label="修改标题"]') as HTMLInputElement;
  assert.ok(input, 'rename opens an inline input');
  assert.equal(input.maxLength, 80);
  Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!.call(input, '  新   标题  ');
  input.dispatchEvent(new Event('input', { bubbles: true }));
  await act(async () => { input.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); await settle(5); });
  assert.deepEqual(updates, [{ id: 'session-1', title: '新 标题' }]);
  assert.equal(container.querySelector('input[aria-label="修改标题"]'), null);
  assert.ok(row.textContent?.includes('新 标题'));
});

test('(j) Escape cancels inline rename and blur submits only once', async () => {
  let calls = 0;
  const client = fakeClient({ update: async (id: string, input: { title: string }) => { calls += 1; return { ...SESSIONS.find((s) => s.id === id), title: input.title }; } });
  await mountShell({ client });
  const row = [...document.querySelectorAll('nav[aria-label="我的对话"] .session-chat-row')].find((node) => node.textContent?.includes('今天的会话'))!;
  const details = row.querySelector('details') as HTMLDetailsElement;
  details.open = true;
  const rename = [...details.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '修改标题') as HTMLButtonElement;
  await act(async () => { rename.click(); await settle(1); });
  const input = row.querySelector('input[aria-label="修改标题"]') as HTMLInputElement;
  await act(async () => { Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!.call(input, '取消的标题'); input.dispatchEvent(new Event('input', { bubbles: true })); });
  await act(async () => { input.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await settle(1); });
  assert.equal(row.querySelector('input[aria-label="修改标题"]'), null);
  assert.equal(calls, 0);
  const renameAgain = [...details.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '修改标题') as HTMLButtonElement;
  await act(async () => { renameAgain.click(); await settle(1); });
  const second = row.querySelector('input[aria-label="修改标题"]') as HTMLInputElement;
  await act(async () => { Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!.call(second, '失焦标题'); second.dispatchEvent(new Event('input', { bubbles: true })); });
  await act(async () => { second.focus(); second.blur(); await settle(5); });
  assert.equal(calls, 1);
});

test('(k) failed rename keeps the editor open with a recoverable error', async () => {
  const client = fakeClient({ update: async () => { throw new Error('rename failed'); } });
  await mountShell({ client });
  const row = [...document.querySelectorAll('nav[aria-label="我的对话"] .session-chat-row')].find((node) => node.textContent?.includes('今天的会话'))!;
  const details = row.querySelector('details') as HTMLDetailsElement;
  details.open = true;
  const rename = [...details.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '修改标题') as HTMLButtonElement;
  await act(async () => { rename.click(); await settle(1); });
  const input = row.querySelector('input[aria-label="修改标题"]') as HTMLInputElement;
  Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!.call(input, '重试标题'); input.dispatchEvent(new Event('input', { bubbles: true }));
  await act(async () => { input.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); await settle(5); });
  assert.ok(row.querySelector('input[aria-label="修改标题"]'), 'failed rename remains editable');
  assert.equal(row.querySelector('[role="alert"]')?.textContent, 'rename failed');
});

test('(l) a renamed session keeps its title when a later page is appended', async () => {
  const calls: number[] = [];
  const client = fakeClient({
    update: async (id: string, input: { title: string }) => ({ ...SESSIONS.find((s) => s.id === id), title: input.title }),
    list: async (params: { page?: number }) => {
      const page = params.page ?? 1;
      calls.push(page);
      return page === 1
        ? { data: SESSIONS, total: 6, page: 1, page_size: 5 }
        : { data: [{ ...SESSIONS.find((s) => s.id === 'session-1'), title: '服务端旧标题' }, { id: 'session-6', title: '下一页的会话', is_pinned: false, updated_at: iso(0) }], total: 6, page: 2, page_size: 2 };
    },
  });
  const container = await mountShell({ client });
  const row = [...document.querySelectorAll('nav[aria-label="我的对话"] .session-chat-row')].find((node) => node.textContent?.includes('今天的会话'))!;
  const details = row.querySelector('details') as HTMLDetailsElement;
  details.open = true;
  const rename = [...details.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '修改标题') as HTMLButtonElement;
  await act(async () => { rename.click(); await settle(1); });
  const input = row.querySelector('input[aria-label="修改标题"]') as HTMLInputElement;
  await act(async () => {
    Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!.call(input, '持久标题');
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    await settle(5);
  });
  assert.equal(rowTitles().includes('持久标题'), true);
  const scrollContainer = shellList()?.parentElement as HTMLElement;
  Object.defineProperties(scrollContainer, {
    scrollHeight: { configurable: true, value: 400 },
    clientHeight: { configurable: true, value: 200 },
    scrollTop: { configurable: true, writable: true, value: 220 },
  });
  await act(async () => { scrollContainer.dispatchEvent(new Event('scroll')); await settle(10); });
  assert.deepEqual(calls, [1, 2]);
  assert.equal(rowTitles().includes('持久标题'), true, 'pagination must not restore the pre-rename title');
  assert.equal(container.textContent?.includes('服务端旧标题'), false);
});

// Vue sessionActivity parity（menu.vue :running + stores/sessionActivityState）：
// 会话行 running spinner 标记。JSDOM URL 固定在 /platform/chat/session-2，
// 活跃会话即 session-2（「昨天的会话」）——正好复刻 ix-chat-mention 扫描里
// 「打开一个 stuck-incomplete 会话」的形态。
const chatMessages = (sessionId: string, assistantCompleted: boolean) => [
  { id: 'm-user', session_id: sessionId, role: 'user' as const, content: 'q', is_completed: true },
  { id: 'm-asst', session_id: sessionId, role: 'assistant' as const, content: assistantCompleted ? 'done' : 'partial', is_completed: assistantCompleted },
];

test('(o) active chat with a trailing incomplete assistant marks its sidebar row running', async () => {
  window.history.pushState({}, '', '/platform/chat/session-2');
  const client = fakeClient({
    messages: async (sessionId: string) => sessionId === 'session-2' ? chatMessages(sessionId, false) : [],
  });
  await mountShell({ client });
  const rows = [...document.querySelectorAll('nav[aria-label="我的对话"] .session-chat-row')];
  const runningRows = rows.filter((node) => node.querySelector('.session-running-indicator'));
  assert.equal(runningRows.length, 1, 'only the stuck-incomplete session row shows the spinner');
  assert.equal(runningRows[0]?.textContent?.includes('昨天的会话'), true, 'the marker sits on the active session-2 row');
  const indicator = runningRows[0]?.querySelector('.session-running-indicator');
  assert.equal(indicator?.getAttribute('role'), 'status', 'SessionSidebarRow.vue role=status parity');
  assert.ok(indicator?.querySelector('.session-running-indicator__spinner'), 'inner 12px spinner ring renders');
});

test('(p) a fully completed history keeps the sidebar free of running markers', async () => {
  window.history.pushState({}, '', '/platform/chat/session-2');
  const client = fakeClient({
    messages: async (sessionId: string) => chatMessages(sessionId, true),
  });
  await mountShell({ client });
  assert.equal(document.querySelectorAll('nav[aria-label="我的对话"] .session-running-indicator').length, 0,
    'no marker when the trailing assistant message is completed');
});

test('(q) the 5s poll clears the marker once the assistant message completes', async () => {
  window.history.pushState({}, '', '/platform/chat/session-2');
  let assistantCompleted = false;
  const client = fakeClient({
    messages: async (sessionId: string) => chatMessages(sessionId, assistantCompleted),
  });
  await mountShell({ client });
  assert.equal(document.querySelectorAll('nav[aria-label="我的对话"] .session-running-indicator').length, 1,
    'marker present while the message streams');
  assistantCompleted = true;
  await settle(5300); // menu.vue:977 5s interval → sessionActivityState.refresh
  assert.equal(document.querySelectorAll('nav[aria-label="我的对话"] .session-running-indicator').length, 0,
    'marker cleared after the poll observes completion');
});
