// S00 N-2: Vue renders the tenant-create form in a centered t-dialog modal
// (frontend/src/components/CreateTenantDialog.vue: t-dialog width=480px,
// header 创建新空间, subtitle tip, textarea description, 取消/创建 actions).
// React used to render the same form as an inline card on the page. This
// spec pins the modal presentation and the Vue dialog copy.
import test from 'node:test';
import assert from 'node:assert/strict';
import React, { act } from 'react';
import type { ReactNode } from 'react';
import { afterEach } from 'node:test';

import nodeModule from 'node:module';

type Root = { render: (node: ReactNode) => void; unmount: () => Promise<void> | void };

const isCss = (specifier: string) => specifier.endsWith('.css');
type ResolveHook = (specifier: string, context: { parentURL?: string }, nextResolve: (s: string, c: { parentURL?: string }) => { url: string }) => { url: string; shortCircuit?: boolean };
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => isCss(specifier)
  ? { url: 'data:text/javascript,export default {}', shortCircuit: true }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/onboarding' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  FocusEvent: dom.window.FocusEvent,
  NodeFilter: dom.window.NodeFilter,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  IS_REACT_ACT_ENVIRONMENT: true,
});
dom.window.localStorage.setItem('locale', 'zh-CN');

const { createRoot } = await import('react-dom/client');
const { WorkspaceOnboardingPage } = await import('./WorkspaceOnboardingPage.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

function fakeDeps(): { client: Record<string, unknown>; scopeRuntime: Record<string, unknown> } {
  return {
    client: {
      auth: { me: async () => ({ user: { id: 'u1' } }) },
      identity: { tenants: { invitations: { pendingCount: async () => ({ pendingCount: 0 }) } } },
    },
    scopeRuntime: {
      hydrate: () => {},
      requiresWorkspace: () => true,
      can: () => true,
    },
  };
}

function invitationDeps(): { client: Record<string, unknown>; scopeRuntime: Record<string, unknown> } {
  return {
    client: {
      auth: { me: async () => ({ user: { id: 'u1' } }) },
      identity: {
        tenants: {
          invitations: {
            pendingCount: async () => ({ pendingCount: 1 }),
            listMine: async () => ({ items: [{ id: 7, tenant_id: 12, tenant_name: '研发空间', role: 'member', status: 'pending' }] }),
          },
        },
      },
    },
    scopeRuntime: {
      hydrate: () => {},
      requiresWorkspace: () => true,
      can: () => true,
    },
  };
}

test('workspace creation trims the Vue dialog values before submitting', async () => {
  let createdInput: unknown;
  const deps = fakeDeps();
  (deps.client.identity as Record<string, unknown>) = { tenants: { admin: { create: async (input: unknown) => { createdInput = input; } }, invitations: { pendingCount: async () => ({ pendingCount: 0 }) } } };
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => { mountedRoot?.render(React.createElement(WorkspaceOnboardingPage, { client: deps.client as never, scopeRuntime: deps.scopeRuntime as never, onLogout: async () => {} })); });
  await settle(20);
  const createEntry = [...document.querySelectorAll('button')].find((n) => n.textContent === '创建空间') as HTMLButtonElement;
  await act(async () => { createEntry.click(); });
  const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
  const name = dialog.querySelector('input') as HTMLInputElement;
  const description = dialog.querySelector('textarea') as HTMLTextAreaElement;
  const setValue = (element: HTMLInputElement | HTMLTextAreaElement, value: string) => {
    const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(element), 'value')!.set!;
    setter.call(element, value);
    element.dispatchEvent(new window.Event('input', { bubbles: true }));
  };
  await act(async () => { setValue(name, '  新空间  '); setValue(description, '  描述  '); });
  await act(async () => { (dialog.querySelector('button[type="submit"]') as HTMLButtonElement).click(); await settle(10); });
  assert.deepEqual(createdInput, { name: '新空间', description: '描述' });
});

test('invitation actions disable both buttons while responding and show success copy', async () => {
  let resolveAccept!: () => void;
  const deps = invitationDeps();
  (deps.client.identity as any).tenants.invitations.accept = () => new Promise<void>((resolve) => { resolveAccept = resolve; });
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => { mountedRoot?.render(React.createElement(WorkspaceOnboardingPage, { client: deps.client as never, scopeRuntime: deps.scopeRuntime as never, onLogout: async () => {} })); });
  await settle(20);
  await act(async () => { ( [...document.querySelectorAll('button')].find((n) => (n.textContent ?? '').startsWith('查看邀请')) as HTMLButtonElement).click(); await settle(10); });
  const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
  const accept = [...dialog.querySelectorAll('button')].find((n) => n.textContent === '接受') as HTMLButtonElement;
  const decline = [...dialog.querySelectorAll('button')].find((n) => n.textContent === '拒绝') as HTMLButtonElement;
  await act(async () => { accept.click(); });
  assert.equal(accept.disabled, true);
  assert.equal(decline.disabled, true);
  await act(async () => { resolveAccept(); await settle(10); });
  assert.match(dialog.textContent ?? '', /已加入/);
});

test('tenant creation renders in a modal dialog with the Vue t-dialog copy (S00 N-2)', async () => {
  const { client, scopeRuntime } = fakeDeps();
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(WorkspaceOnboardingPage, {
      client: client as never,
      scopeRuntime: scopeRuntime as never,
      onLogout: async () => {},
    }));
  });
  await settle(20);

  const createEntry = [...document.querySelectorAll('button')].find((n) => n.textContent === '创建空间');
  assert.ok(createEntry, 'expected the 创建空间 entry action after policy load');

  await act(async () => { createEntry!.click(); await settle(10); });

  // Vue: t-dialog modal — role=dialog, aria-modal, overlay backdrop.
  const dialog = document.querySelector('[role="dialog"]') as HTMLElement | null;
  assert.ok(dialog, 'expected the create form inside a role=dialog modal, not an inline card');
  assert.equal(dialog.getAttribute('aria-modal'), 'true');

  // Vue dialog header title + subtitle tip.
  assert.match(dialog.textContent || '', /创建新空间/);
  assert.match(dialog.textContent || '', /你将自动成为新空间的所有者/);

  // Vue: description is a textarea with the Vue placeholder.
  const description = dialog.querySelector('textarea') as HTMLTextAreaElement | null;
  assert.ok(description, 'expected the description field to be a textarea like Vue t-textarea');
  assert.equal(description.getAttribute('maxlength'), '512');
  assert.equal(description.getAttribute('placeholder'), '简单描述一下这个空间的用途');

  // Vue: cancel action inside the dialog.
  const cancel = [...dialog.querySelectorAll('button')].find((n) => n.textContent === '取消');
  assert.ok(cancel, 'expected a 取消 action inside the dialog');
  await act(async () => { cancel!.click(); await settle(10); });
  assert.equal(document.querySelector('[role="dialog"]'), null, 'cancel closes the dialog');
});

test('my invitations renders in a modal dialog like Vue WorkspaceOnboarding', async () => {
  const { client, scopeRuntime } = invitationDeps();
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(WorkspaceOnboardingPage, {
      client: client as never,
      scopeRuntime: scopeRuntime as never,
      onLogout: async () => {},
    }));
  });
  await settle(20);

  const invitationsEntry = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').startsWith('查看邀请'));
  assert.ok(invitationsEntry, 'expected the invitations entry action');
  await act(async () => { invitationsEntry!.click(); await settle(10); });

  const dialog = document.querySelector('[role="dialog"]') as HTMLElement | null;
  assert.ok(dialog, 'expected invitations to open in a modal dialog');
  assert.equal(dialog.getAttribute('aria-modal'), 'true');
  assert.match(dialog.textContent || '', /查看邀请/);
  assert.match(dialog.textContent || '', /研发空间/);
  assert.equal(document.querySelector('[role="dialog"]')?.closest('.wk-card') ?? null, null);
});

test('workspace onboarding keeps the Vue workspace mark before the heading', async () => {
  const { client, scopeRuntime } = fakeDeps();
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(WorkspaceOnboardingPage, {
      client: client as never,
      scopeRuntime: scopeRuntime as never,
      onLogout: async () => {},
    }));
  });
  await settle(20);

  const mark = document.querySelector('[data-testid="workspace-mark"]') as HTMLElement | null;
  assert.ok(mark, 'Vue renders a workspace mark above the onboarding heading');
  assert.equal(mark?.getAttribute('aria-hidden'), 'true');
  // S7：utilities 平移为 auth-u.css 语义类（h-16 w-16 rounded-[18px] → .wk-onb-1）
  assert.match(mark?.className ?? '', /wk-onb-1/);
  assert.equal(mark?.querySelector('svg')?.getAttribute('width'), '30');
  assert.equal(mark?.nextElementSibling?.tagName, 'H1');
});
