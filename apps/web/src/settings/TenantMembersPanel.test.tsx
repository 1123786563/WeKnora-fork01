import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import type { TenantMember, WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  MouseEvent: dom.window.MouseEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { TenantMembersPanel } = await import('./TenantMembersPanel.tsx');

const alice: TenantMember = { user_id: 'u1', username: 'Alice', email: 'alice@example.com', role: 'viewer', status: 'active', joined_at: '2030-01-01' };
const bob: TenantMember = { user_id: 'u2', username: 'Bob', email: 'bob@example.com', role: 'viewer', status: 'active', joined_at: '2030-01-02' };
let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function clientWithList(list: (tenantId: number, input: { q?: string; page: number; pageSize: number }) => Promise<{ items: TenantMember[]; total: number }>): WeKnoraClient {
  return {
    auth: { me: async () => ({ user: { id: 'current-user' } }) },
    identity: { tenants: { members: { list }, invitations: { listTenant: async () => ({ items: [], total: 0 }) } } },
  } as unknown as WeKnoraClient;
}

async function mount(client: WeKnoraClient, initialMembers?: { items: TenantMember[]; total: number }, role: 'viewer' | 'admin' = 'viewer') {
  window.localStorage.setItem('locale', 'en-US');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<TenantMembersPanel client={client} tenantId={1} role={role} initialMembers={initialMembers} />);
  });
  return container;
}

test('tenant members keeps viewer state read-only', () => {
  const html = renderToStaticMarkup(<TenantMembersPanel client={{} as never} tenantId={1} role="viewer" initialMembers={{ items: [alice], total: 1 }} />);
  assert.match(html, /Alice/);
  assert.doesNotMatch(html, /Invite member|Send invitation|Remove/);
});

test('tenant members exposes manager search, invite and role controls', () => {
  const html = renderToStaticMarkup(<TenantMembersPanel client={{} as never} tenantId={1} role="admin" initialMembers={{ items: [alice], total: 1 }} />);
  assert.match(html, /Invite member/);
  assert.match(html, /Send invitation/);
  assert.match(html, /Role for Alice/);
  assert.match(html, /Remove/);
});

test('member list header and localized search stay mounted during initial loading', async () => {
  const request = deferred<{ items: TenantMember[]; total: number }>();
  const container = await mount(clientWithList(() => request.promise));

  const header = container.querySelector('.members-list-header');
  const search = container.querySelector<HTMLInputElement>('input[type="search"]');
  assert.ok(header);
  assert.equal(header.querySelector('.members-list-title')?.textContent, 'Workspace members');
  assert.equal(header.querySelector('.members-list-count-badge')?.textContent, '0');
  assert.equal(search?.placeholder, 'Search by name or email');
  assert.match(container.textContent ?? '', /Loading members/);

  await act(async () => request.resolve({ items: [alice], total: 1 }));
  assert.strictEqual(container.querySelector('.members-list-header'), header);
  assert.strictEqual(container.querySelector('input[type="search"]'), search);
  assert.equal(header.querySelector('.members-list-count-badge')?.textContent, '1');
  assert.match(container.textContent ?? '', /Alice/);
});

test('search remains mounted and clearing reloads the unfiltered member list', async () => {
  const searchRequest = deferred<{ items: TenantMember[]; total: number }>();
  const clearRequest = deferred<{ items: TenantMember[]; total: number }>();
  const queries: Array<string | undefined> = [];
  const client = clientWithList((_tenantId, input) => {
    queries.push(input.q);
    return input.q ? searchRequest.promise : clearRequest.promise;
  });
  const container = await mount(client, { items: [alice], total: 1 }, 'admin');
  const header = container.querySelector('.members-list-header');
  const search = container.querySelector<HTMLInputElement>('input[type="search"]');
  assert.ok(header && search);
  assert.match(container.textContent ?? '', /Invite member/);

  await act(async () => {
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
    setValue?.call(search, 'Bob');
    search.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    search.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
  assert.equal(search.value, 'Bob');
  await act(async () => search.form?.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true })));
  assert.strictEqual(container.querySelector('.members-list-header'), header);
  assert.strictEqual(container.querySelector('input[type="search"]'), search);
  assert.match(container.textContent ?? '', /Loading members/);

  await act(async () => searchRequest.resolve({ items: [bob], total: 1 }));
  assert.match(container.textContent ?? '', /Bob/);
  const clear = container.querySelector<HTMLButtonElement>('button[aria-label="Clear search"]');
  assert.ok(clear, 'a keyboard-focusable clear control should appear for a non-empty query');

  await act(async () => clear.click());
  assert.equal(search.value, '');
  assert.strictEqual(container.querySelector('.members-list-header'), header);
  assert.match(container.textContent ?? '', /Loading members/);

  await act(async () => clearRequest.resolve({ items: [alice], total: 2 }));
  assert.deepEqual(queries, ['Bob', undefined]);
  assert.match(container.textContent ?? '', /Alice/);
  assert.equal(header.querySelector('.members-list-count-badge')?.textContent, '2');
  assert.equal(container.querySelector('button[aria-label="Clear search"]'), null);
});
