import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { TenantMember, WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => Window & typeof globalThis };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  Event: dom.window.Event,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { AdministrationPage } = await import('./AdministrationPage.tsx');
const { formatMessage } = await import('@weknora/i18n');

const alice: TenantMember = { user_id: 'u1', username: 'Alice', email: 'alice@example.com', role: 'viewer', status: 'active', joined_at: '2030-01-01' } as TenantMember;
const bob: TenantMember = { user_id: 'u2', username: 'Bob', email: 'bob@example.com', role: 'contributor', status: 'active', joined_at: '2030-01-02' } as TenantMember;

type ListCall = { q?: string; page: number; pageSize: number };
let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

function sleep(ms: number) { return new Promise((resolve) => setTimeout(resolve, ms)); }

function administrationClient(list: (tenantId: number, params: ListCall) => Promise<{ items: TenantMember[]; total: number }>): WeKnoraClient {
  return {
    auth: { me: async () => ({ user: { id: 'current-user' }, memberships: [{ tenant_id: 1, role: 'owner' }] }) },
    identity: {
      tenants: {
        members: { list },
        invitations: { listTenant: async () => ({ items: [], total: 0 }) },
        auditLog: { list: async () => ({ items: [], nextCursor: 0 }) },
      },
    },
  } as unknown as WeKnoraClient;
}

async function mount(client: WeKnoraClient): Promise<HTMLElement> {
  window.localStorage.setItem('locale', 'en-US');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => { mountedRoot?.render(<AdministrationPage client={client} tenantId={1} />); });
  await act(async () => { await sleep(0); });
  return container;
}

function memberSearchInput(container: HTMLElement): HTMLInputElement {
  const placeholder = formatMessage('en-US', 'mobileAdministration.searchPlaceholder');
  const input = Array.from(container.querySelectorAll('input')).find((node) => node.getAttribute('placeholder') === placeholder || node.getAttribute('aria-label') === placeholder);
  assert.ok(input, `member search input with placeholder "${placeholder}" should exist`);
  return input as HTMLInputElement;
}

async function setSearchQuery(container: HTMLElement, value: string) {
  const input = memberSearchInput(container);
  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  await act(async () => {
    setValue?.call(input, value);
    input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    input.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
}

test('member list supports the Vue debounced server-side search contract', async () => {
  const calls: ListCall[] = [];
  let lastPage: { items: TenantMember[]; total: number } = { items: [alice, bob], total: 2 };
  const client = administrationClient((_tenantId, params) => {
    calls.push(params);
    const q = params.q ?? '';
    const items = [alice, bob].filter((member) => member.username.toLowerCase().includes(q.toLowerCase()));
    lastPage = { items, total: items.length };
    return Promise.resolve(lastPage);
  });
  const container = await mount(client);
  assert.deepEqual(calls[0], { page: 1, pageSize: 100, q: undefined });
  await setSearchQuery(container, 'ali');
  assert.deepEqual(calls[calls.length - 1], { page: 1, pageSize: 100, q: undefined }, 'no server call before the 320ms debounce window elapses');
  await act(async () => { await sleep(400); });
  assert.ok(calls.length >= 2, 'debounced server-side search fires after 320ms');
  const debounced = calls[calls.length - 1];
  assert.equal(debounced.q, 'ali', 'search term is applied server-side as q');
  assert.equal(debounced.page, 1, 'search resets pagination to the first page');
  assert.match(container.textContent ?? '', /Alice/);
  assert.doesNotMatch(container.textContent ?? '', /Bob/);
});

test('search-aware empty state mirrors the Vue emptySearch copy', async () => {
  const client = administrationClient((_tenantId, params) => {
    const q = params.q ?? '';
    const items = [alice, bob].filter((member) => member.username.toLowerCase().includes(q.toLowerCase()));
    return Promise.resolve({ items, total: items.length });
  });
  const container = await mount(client);
  await setSearchQuery(container, 'zzz');
  await act(async () => { await sleep(400); });
  const expected = formatMessage('en-US', 'mobileAdministration.noMembersForQuery', { q: 'zzz' });
  assert.notEqual(expected, 'mobileAdministration.noMembersForQuery', 'empty-search copy must exist in the shared locale catalog');
  assert.match(container.textContent ?? '', new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  assert.doesNotMatch(container.textContent ?? '', /No members returned\./);
});

test('invite form defaults to the Vue contributor role', async () => {
  const client = administrationClient(() => Promise.resolve({ items: [alice], total: 1 }));
  const container = await mount(client);
  const form = container.querySelector('form');
  assert.ok(form, 'invite form should render for managers');
  const roleSelect = form?.querySelector('select');
  assert.ok(roleSelect, 'invite form exposes a role select');
  assert.equal(roleSelect?.value, 'contributor');
});
