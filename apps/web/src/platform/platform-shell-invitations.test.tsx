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
  // S6 换装：InvitationInbox 弹层走 tdesign Dialog/Button（settings 域同款 rAF shim）。
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
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

function invitation(id = 7) {
  return {
    id,
    tenant_id: 12,
    tenant_name: '研发空间',
    invitee_user_id: 'u1',
    inviter_name: '管理员',
    role: 'contributor' as const,
    status: 'pending' as const,
    expires_at: '2030-01-01T00:00:00Z',
    created_at: '2029-01-01T00:00:00Z',
  };
}

function fakeClient(options: { pending?: number; list?: () => Promise<unknown>; accept?: () => Promise<unknown>; decline?: () => Promise<unknown> } = {}) {
  return {
    auth: { me: async () => ({ user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '' }, tenant: { id: 'tenant-1', name: 'Parity' }, memberships: [] }) },
    sessions: { list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }) },
    identity: {
      tenants: {
        invitations: {
          pendingCount: async () => ({ pendingCount: options.pending ?? 0 }),
          listMine: options.list ?? (async () => ({ items: [invitation()], total: 1, page: 1, pageSize: 1 })),
          accept: options.accept ?? (async () => ({ tenantId: 12, role: 'contributor', status: 'active', joinedAt: '2030-01-01T00:00:00Z' })),
          decline: options.decline ?? (async () => undefined),
        },
      },
    },
  };
}

async function mount(client: ReturnType<typeof fakeClient>) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => mountedRoot?.render(React.createElement(PlatformShell, {
    client: client as never,
    onLogout: () => undefined,
    children: React.createElement('div', null, 'page'),
  })));
  await settle(20);
}

test('global invitation bell is absent when the pending count is zero', async () => {
  await mount(fakeClient({ pending: 0 }));
  assert.equal(document.querySelector('[data-testid="global-invitation-bell"]'), null);
});

test('global invitation bell opens a loading inbox and then renders pending invitations', async () => {
  let resolveList!: (value: unknown) => void;
  const list = () => new Promise((resolve) => { resolveList = resolve; });
  await mount(fakeClient({ pending: 1, list }));
  const bell = document.querySelector<HTMLButtonElement>('[data-testid="global-invitation-bell"]');
  assert.ok(bell);
  assert.equal(bell.getAttribute('aria-label'), '查看邀请');
  await act(async () => bell!.click());
  assert.match(document.querySelector('.t-dialog')?.textContent ?? '', /加载/);
  await act(async () => resolveList({ items: [invitation()], total: 1, page: 1, pageSize: 1 }));
  await settle();
  assert.match(document.querySelector('.t-dialog')?.textContent ?? '', /研发空间/);
});

test('invitation accept and decline remove the row and refresh the pending count', async () => {
  let accepted = 0;
  let declined = 0;
  await mount(fakeClient({ pending: 2, list: async () => ({ items: [invitation(7), invitation(8)], total: 2, page: 1, pageSize: 2 }), accept: async () => { accepted += 1; return {}; }, decline: async () => { declined += 1; } }));
  await act(async () => document.querySelector<HTMLButtonElement>('[data-testid="global-invitation-bell"]')!.click());
  await settle();
  const dialog = document.querySelector('.t-dialog')!;
  const buttons = [...dialog.querySelectorAll<HTMLButtonElement>('button')];
  await act(async () => buttons.find((button) => button.dataset.action === 'accept')!.click());
  await settle();
  assert.equal(accepted, 1);
  assert.equal(document.querySelectorAll('[data-testid="invitation-row"]').length, 1);

  await act(async () => document.querySelector<HTMLButtonElement>('[data-testid="global-invitation-bell"]')!.click());
  await settle();
  const dialogAgain = document.querySelector('.t-dialog')!;
  await act(async () => [...dialogAgain.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.dataset.action === 'decline')!.click());
  await settle();
  assert.equal(declined, 1);
});

test('invitation load failure exposes retry and Escape closes the inbox', async () => {
  let attempts = 0;
  const list = async () => { attempts += 1; if (attempts === 1) throw new Error('网络失败'); return { items: [], total: 0, page: 1, pageSize: 0 }; };
  await mount(fakeClient({ pending: 1, list }));
  const bell = document.querySelector<HTMLButtonElement>('[data-testid="global-invitation-bell"]')!;
  await act(async () => { bell.focus(); bell.click(); });
  await settle();
  assert.match(document.querySelector('.t-dialog')?.textContent ?? '', /网络失败/);
  await act(async () => document.querySelector<HTMLButtonElement>('[data-action="retry-invitations"]')!.click());
  await settle();
  assert.match(document.querySelector('.t-dialog')?.textContent ?? '', /没有待处理的邀请/);
  await act(async () => document.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })));
  // S6 换装：tdesign Dialog 退场动画 300ms 后才卸载 DOM（显式 timeout 兜底）。
  await settle(350);
  assert.equal(document.querySelector('.t-dialog'), null);
});
