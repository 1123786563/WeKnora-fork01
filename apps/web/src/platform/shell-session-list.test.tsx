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
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
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
  return {
    auth: { me: async () => ({ user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '' } }) },
    sessions: {
      list: async () => ({ data: SESSIONS, total: SESSIONS.length, page: 1, page_size: 30 }),
      pin: async () => undefined,
      unpin: async () => undefined,
      update: async (sessionId: string, input: { title: string }) => ({ ...SESSIONS.find((s) => s.id === sessionId), title: input.title }),
      clear: async () => undefined,
      remove: async () => undefined,
      ...overrides,
    },
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

// Class names became Tailwind utilities (shell.css is gone); queries use the
// semantic anchors instead: the sessions nav by its aria-label, group headers
// as h3, and the title span as the only button span without aria-hidden/title.
const shellList = () => document.querySelector('nav[aria-label="我的对话"]');
const groupHeaders = () => [...document.querySelectorAll('nav[aria-label="我的对话"] h3')].map((node) => node.textContent);
const rowTitles = () => [...document.querySelectorAll('nav[aria-label="我的对话"] li button span:not([aria-hidden]):not([title])')].map((node) => node.textContent);

test('(a) shell sidebar renders the grouped session list on a chat route', async () => {
  await mountShell();
  assert.ok(shellList(), 'expected the shell session list region');
  assert.deepEqual(groupHeaders(), ['已置顶', '今天', '昨天', '更早']);
  // Untitled rows render menu.newSession (新会话) like Vue mapSessionRow.
  assert.deepEqual(rowTitles(), ['置顶的会话', '今天的会话', '昨天的会话', '新会话', '更早的会话']);
});

test('(b) active session follows the route (/platform/chat/:id)', async () => {
  await mountShell();
  const activeButtons = [...document.querySelectorAll('nav[aria-label="我的对话"] button[aria-current="page"]')];
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
  const target = [...document.querySelectorAll('nav[aria-label="我的对话"] li button')]
    .find((node) => node.textContent === '今天的会话') as HTMLButtonElement;
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
  window.confirm = () => true;
  try {
    await mountShell({ client });
    const menus = [...document.querySelectorAll('nav[aria-label="我的对话"] li details')];
    assert.equal(menus.length, 5, 'each row carries the hover ⋯ menu');
    const menuOf = (title: string) => {
      const row = [...document.querySelectorAll('nav[aria-label="我的对话"] li')]
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
    assert.ok(calls.includes('clear:session-3'));
    // Vue menu.vue row menu uses upload.deleteRecord (删除记录), not chatHeader.deleteSession.
    const deleteItem = [...menu3.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '删除记录');
    assert.ok(deleteItem, 'expected the 删除记录 menu item');
    await act(async () => { (deleteItem as HTMLButtonElement).click(); await settle(5); });
    assert.ok(calls.includes('remove:session-3'));
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
    const menu3 = [...document.querySelectorAll('nav[aria-label="我的对话"] li')]
      .find((node) => node.textContent?.includes('更早的会话'))
      ?.querySelector('details') as HTMLDetailsElement;
    assert.ok(menu3);
    menu3.open = true;
    const deleteItem = [...menu3.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent === '删除记录') as HTMLButtonElement;
    assert.ok(deleteItem);
    await act(async () => { deleteItem.click(); await settle(5); });
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
