import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { Organization, WeKnoraClient } from '@weknora/api-client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
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
const { KnowledgeBaseShareDialog } = await import('./KnowledgeBaseShareDialog.tsx');

type Share = { id: string; organization_id: string; organization_name: string; permission: 'viewer' | 'editor' };

const organizations = [
  { id: 'org-editor', name: 'Editors', is_owner: false, my_role: 'editor' },
  { id: 'org-viewer', name: 'Viewers', is_owner: false, my_role: 'viewer' },
] as unknown as Organization[];

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function clientFor(shares: () => Promise<{ items: Share[]; total: number }>, overrides: Partial<{
  create: WeKnoraClient['identity']['organizations']['knowledgeBaseShares']['create'];
  remove: WeKnoraClient['identity']['organizations']['knowledgeBaseShares']['remove'];
}> = {}): WeKnoraClient {
  return {
    identity: {
      organizations: {
        list: async () => ({ items: organizations, total: organizations.length }),
        knowledgeBaseShares: {
          list: async () => shares(),
          create: overrides.create ?? (async () => ({ id: 'new-share' })),
          remove: overrides.remove ?? (async () => undefined),
        },
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
  window.confirm = () => true;
});

async function mount(client: WeKnoraClient) {
  window.localStorage.setItem('locale', 'en-US');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeBaseShareDialog client={client} knowledgeBaseId="kb-1" knowledgeBaseName="Docs" open onClose={() => undefined} />);
  });
  return container;
}

function button(container: HTMLElement, label: string) {
  return [...container.querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent?.includes(label));
}

test('opens on the share form and toggles to the shared-list view and back', async () => {
  const request = deferred<{ items: Share[]; total: number }>();
  const container = await mount(clientFor(() => request.promise));

  assert.match(container.textContent ?? '', /Loading shares/);
  await act(async () => request.resolve({ items: [{ id: 'share-1', organization_id: 'org-editor', organization_name: 'Editors', permission: 'editor' }], total: 1 }));

  assert.ok(container.querySelector('form'), 'the dialog should start on the share form');
  const showList = button(container, 'Shared to organizations (1)');
  assert.ok(showList, 'existing shares should expose a shared-list entry point');

  await act(async () => showList?.click());
  assert.equal(container.querySelector('form'), null, 'the form should be hidden in shared-list view');
  assert.match(container.textContent ?? '', /Editors/);
  assert.match(container.textContent ?? '', /Editable/);

  const back = button(container, 'Back');
  assert.ok(back);
  await act(async () => back?.click());
  assert.ok(container.querySelector('form'), 'back should return to the share form');
});

test('confirms unshare and prevents duplicate removal while the mutation is busy', async () => {
  const calls: string[] = [];
  const removal = deferred<void>();
  const client = clientFor(
    async () => ({ items: [{ id: 'share-1', organization_id: 'org-editor', organization_name: 'Editors', permission: 'viewer' }], total: 1 }),
    { remove: async (_kbId, shareId) => { calls.push(shareId); await removal.promise; } },
  );
  const container = await mount(client);
  await act(async () => button(container, 'Shared to organizations (1)')?.click());

  let confirmCalls = 0;
  window.confirm = () => { confirmCalls += 1; return true; };
  const remove = button(container, 'Remove');
  assert.ok(remove);
  await act(async () => {
    remove?.click();
    remove?.click();
  });
  assert.deepEqual(calls, ['share-1']);
  assert.equal(confirmCalls, 1);
  assert.equal(remove?.disabled, true);

  await act(async () => removal.resolve());
});
