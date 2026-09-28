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
  // S6：tdesign Input 挂载期调用 rAF（autoWidth 校准），jsdom 非 visual 无此全局。
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLButtonElement: dom.window.HTMLButtonElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  PointerEvent: dom.window.PointerEvent,
  NodeFilter: dom.window.NodeFilter,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
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

async function mount(client: WeKnoraClient, onChanged: () => void = () => undefined, locale = 'en-US', knowledgeBaseId = 'kb-1') {
  window.localStorage.setItem('locale', locale);
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeBaseShareDialog client={client} knowledgeBaseId={knowledgeBaseId} knowledgeBaseName="Docs" open onClose={() => undefined} onChanged={onChanged} />);
  });
  // Dialog is a project-owned Radix/shadcn primitive whose content is portaled
  // to body, just like Vue's Teleport. Query the actual rendered surface so
  // these tests exercise the portal rather than the empty mount host.
  return document.body;
}

// R462: the inline mount mirrors Vue KBShareSettings embedded in the editor
// modal. It may be hosted inside the editor's save <form> (App.tsx onSubmit),
// so the host defaults to an outer form to exercise the nesting contract.
async function mountInline(client: WeKnoraClient, host?: HTMLElement) {
  window.localStorage.setItem('locale', 'en-US');
  const container = document.createElement('div');
  (host ?? document.body).append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeBaseShareDialog client={client} knowledgeBaseId="kb-1" knowledgeBaseName="Docs" open inline onClose={() => undefined} onChanged={() => undefined} />);
  });
  return container;
}

// Vue KBShareSettings opens on the share list; the add-share entry (t-popup
// trigger, `knowledgeEditor.share.addShare`) toggles to the add-share form.
async function openInlineShareForm(container: HTMLElement) {
  const addShare = [...container.querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent?.trim() === 'Share');
  assert.ok(addShare, 'inline share list should expose the add-share entry');
  await act(async () => addShare?.click());
}

function button(container: HTMLElement, label: string) {
  // S6：disabled 的 tdesign Button 渲染 div.t-button（台账 #7），双查询。
  return [...container.querySelectorAll<HTMLElement>('button, .t-button')].find((item) => item.textContent?.includes(label));
}

function labelledButton(container: HTMLElement, label: string) {
  // S6：disabled 的 tdesign Button 渲染 div.t-button（台账 #7），双查询。
  return container.querySelector<HTMLElement>(`button[aria-label="${label}"], .t-button[aria-label="${label}"]`);
}

async function select(container: HTMLElement, label: string, value: string) {
  if (label === 'Permission') {
    const option = [...container.querySelectorAll<HTMLButtonElement>('[role="radio"]')].find((item) => item.textContent === (value === 'editor' ? 'Editable' : 'Read-only'));
    assert.ok(option, `${label} option should exist`);
    await act(async () => option.click());
    return;
  }
  const element = [...container.querySelectorAll<HTMLSelectElement>('select')].find((item) => item.parentElement?.textContent?.includes(label));
  assert.ok(element, `${label} select should exist`);
  await act(async () => {
    element.value = value;
    element.dispatchEvent(new Event('change', { bubbles: true }));
  });
}

test('opens on the share form and toggles to the shared-list view and back', async () => {
  const request = deferred<{ items: Share[]; total: number }>();
  const container = await mount(clientFor(() => request.promise));

  assert.match(container.textContent ?? '', /Loading…/);
  await act(async () => request.resolve({ items: [{ id: 'share-1', organization_id: 'org-editor', organization_name: 'Editors', permission: 'editor' }], total: 1 }));

  assert.ok(container.querySelector('form'), 'the dialog should start on the share form');
  const showList = button(container, 'Shared to (1)');
  assert.ok(showList, 'existing shares should expose a shared-list entry point');
  assert.equal(showList?.closest('form') !== null, true, 'Vue places the shared-list action in the form footer');

  await act(async () => showList?.click());
  assert.equal(container.querySelector('form'), null, 'the form should be hidden in shared-list view');
  assert.match(container.textContent ?? '', /Editors/);
  assert.match(container.textContent ?? '', /Editable/);

  const back = button(container, 'Back');
  assert.ok(back);
  await act(async () => back?.click());
  assert.ok(container.querySelector('form'), 'back should return to the share form');
});

test('renders only loading content until the initial share data has settled', async () => {
  const request = deferred<{ items: Share[]; total: number }>();
  const container = await mount(clientFor(() => request.promise));

  try {
    assert.match(container.textContent ?? '', /Loading/);
    assert.equal(container.querySelector('form'), null, 'the share form should not render while data is loading');
    assert.doesNotMatch(container.textContent ?? '', /No shared knowledge bases yet|No shares\./);
  } finally {
    await act(async () => request.resolve({ items: [], total: 0 }));
  }
  assert.doesNotMatch(container.textContent ?? '', /Loading…/);
});

test('uses the existing shared-space translations for the share form', async () => {
  const container = await mount(clientFor(async () => ({ items: [], total: 0 })), () => undefined, 'zh-CN');

  assert.match(container.textContent ?? '', /共享到共享空间/);
  assert.match(container.textContent ?? '', /选择共享空间/);
  assert.match(container.textContent ?? '', /权限/);
});

test('permission uses the Vue radio-button group instead of a native select', async () => {
  const container = await mount(clientFor(async () => ({ items: [], total: 0 })));
  const group = container.querySelector('[role="radiogroup"]');
  assert.ok(group);
  assert.equal(group.querySelectorAll('[role="radio"]').length, 2);
  assert.equal(group.querySelector('[aria-checked="true"]')?.textContent, 'Read-only');
});

test('resets permission to read-only whenever the dialog is reopened like Vue', async () => {
  const client = clientFor(async () => ({ items: [], total: 0 }));
  const container = await mount(client);
  await select(container, 'Permission', 'editor');
  assert.equal(container.querySelector('[role="radio"][aria-checked="true"]')?.textContent, 'Editable');

  await act(async () => {
    mountedRoot?.render(<KnowledgeBaseShareDialog client={client} knowledgeBaseId="kb-1" knowledgeBaseName="Docs" open={false} onClose={() => undefined} />);
  });
  await act(async () => {
    mountedRoot?.render(<KnowledgeBaseShareDialog client={client} knowledgeBaseId="kb-1" knowledgeBaseName="Docs" open onClose={() => undefined} />);
  });

  assert.equal(container.querySelector('[role="radio"][aria-checked="true"]')?.textContent, 'Read-only');
});

test('filters viewer organizations and sends the selected permission in the create payload', async () => {
  const payloads: unknown[] = [];
  const client = clientFor(async () => ({ items: [], total: 0 }), {
    create: async (_kbId, payload) => { payloads.push(payload); return { id: 'share-1' }; },
  });
  let changed = 0;
  const container = await mount(client, () => { changed += 1; });

  const organizationSelect = container.querySelectorAll<HTMLSelectElement>('select')[0];
  assert.ok(organizationSelect);
  assert.deepEqual([...organizationSelect.options].map((option) => option.textContent), ['Select a shared space to share with', 'Editors']);
  assert.equal([...organizationSelect.options].some((option) => option.textContent === 'Viewers'), false);

  await select(container, 'Select Shared Space', 'org-editor');
  await select(container, 'Permission', 'editor');
  await act(async () => button(container, 'Confirm')?.click());

  assert.deepEqual(payloads, [{ organization_id: 'org-editor', permission: 'editor' }]);
  assert.equal(changed, 1);
});

test('resets the share form after a successful share like Vue', async () => {
  let shares: Share[] = [];
  const client = clientFor(async () => ({ items: shares, total: shares.length }), {
    create: async () => {
      shares = [{ id: 'share-1', organization_id: 'org-editor', organization_name: 'Editors', permission: 'editor' }];
      return { id: 'share-1' };
    },
  });
  const container = await mount(client);

  await select(container, 'Select Shared Space', 'org-editor');
  await select(container, 'Permission', 'editor');
  await act(async () => button(container, 'Confirm')?.click());

  assert.equal(container.querySelector('[role="radio"][aria-checked="true"]')?.textContent, 'Read-only');
  assert.equal(container.querySelector<HTMLSelectElement>('select')?.value, '', 'Vue clears the organization after sharing');
  assert.equal(button(container, 'Confirm')?.classList.contains('t-is-disabled'), true, 'the cleared form cannot be submitted again');
});

test('renders Vue-shaped organization options and shared-list actions', async () => {
  const enriched = [{ id: 'org-editor', name: 'Editors', is_owner: true, my_role: 'admin', member_count: 7, share_count: 3, agent_share_count: 2 }, { id: 'org-viewer', name: 'Viewers', my_role: 'editor' }] as unknown as Organization[];
  const client = clientFor(async () => ({ items: [{ id: 'share-1', organization_id: 'org-other', organization_name: 'Editors', permission: 'editor' }], total: 1 }));
  client.identity.organizations.list = async () => ({ items: enriched, total: 1 });
  const container = await mount(client);

  await select(container, 'Select Shared Space', 'org-editor');
  await act(async () => container.querySelector<HTMLButtonElement>('[role="combobox"]')?.click());
  const option = container.querySelector('[role="option"]');
  assert.ok(option, 'organization options should expose the Vue option anatomy');
  assert.match(option.textContent ?? '', /Editors/);
  assert.match(option.textContent ?? '', /7/);
  assert.match(option.textContent ?? '', /3/);
  assert.match(option.textContent ?? '', /2/);
  const actions = [...container.querySelectorAll<HTMLDivElement>('form div')].find((row) => row.querySelector('button[type="submit"]'));
  assert.ok(actions, 'form should have a separated action footer');
  assert.ok(button(container, 'Cancel'));
  assert.ok(button(container, 'Confirm'));

  await act(async () => button(container, 'Shared to (1)')?.click());
  assert.ok(container.querySelector('li span[aria-hidden="true"]'));
  assert.equal(container.querySelectorAll('li button').length, 2, 'shared rows should expose organization settings and remove actions');
  assert.equal(container.querySelectorAll('li select').length, 0, 'Vue renders the existing permission as a tag, not an editable select');
});

test('organization picker exposes a keyboard-safe custom Vue-style option list', async () => {
  const enriched = [{ id: 'org-editor', name: 'Editors', is_owner: true, my_role: 'admin', member_count: 7, share_count: 3, agent_share_count: 2 }, { id: 'org-viewer', name: 'Viewers', my_role: 'editor' }] as unknown as Organization[];
  const client = clientFor(async () => ({ items: [], total: 0 }));
  client.identity.organizations.list = async () => ({ items: enriched, total: 1 });
  const container = await mount(client);
  const trigger = container.querySelector<HTMLButtonElement>('[role="combobox"]');
  assert.ok(trigger);
  await act(async () => trigger?.click());
  assert.ok(container.querySelector('[role="listbox"]'));
  assert.match(container.textContent ?? '', /Editors/);
  assert.match(container.textContent ?? '', /7/);
  await act(async () => trigger?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })));
  await act(async () => trigger?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })));
  assert.equal(trigger?.textContent?.includes('Viewers'), true);
  await act(async () => trigger?.click());
  await act(async () => container.querySelector<HTMLButtonElement>('[role="option"]')?.click());
  assert.equal(trigger?.textContent?.includes('Editors'), true);
  assert.equal(container.querySelector('[role="listbox"]'), null);
  await act(async () => trigger?.click());
  await act(async () => document.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape' })));
  assert.equal(container.querySelector('[role="listbox"]'), null);
});

test('shows the create failure and does not fire the change callback', async () => {
  let changed = 0;
  const client = clientFor(async () => ({ items: [], total: 0 }), {
    create: async () => { throw new Error('share request failed'); },
  });
  const container = await mount(client, () => { changed += 1; });

  await select(container, 'Select Shared Space', 'org-editor');
  await act(async () => button(container, 'Confirm')?.click());

  assert.match(container.textContent ?? '', /share request failed/);
  assert.doesNotMatch(container.textContent ?? '', /Knowledge base shared/);
  assert.equal(changed, 0);
  assert.equal(button(container, 'Confirm')?.classList.contains('t-is-disabled'), false);
});

test('prevents duplicate removal while the mutation is busy', async () => {
  const calls: string[] = [];
  const removal = deferred<void>();
  const client = clientFor(
    async () => ({ items: [{ id: 'share-1', organization_id: 'org-editor', organization_name: 'Editors', permission: 'viewer' }], total: 1 }),
    { remove: async (_kbId, shareId) => { calls.push(shareId); await removal.promise; } },
  );
  const container = await mount(client);
  await act(async () => button(container, 'Shared to (1)')?.click());

  const remove = labelledButton(container, 'Remove share');
  assert.ok(remove);
  await act(async () => {
    remove?.click();
    remove?.click();
  });
  assert.deepEqual(calls, ['share-1']);
  // S6：busy 后重查（div↔button 根标签切换，台账 #7）。
  assert.equal(labelledButton(container, 'Remove share')?.classList.contains('t-is-disabled'), true);

  await act(async () => removal.resolve());
});

test('removes a share directly without an extra confirmation prompt like Vue', async () => {
  let removeCalls = 0;
  const client = clientFor(async () => ({ items: [{ id: 'share-1', organization_id: 'org-editor', organization_name: 'Editors', permission: 'viewer' }], total: 1 }), {
    remove: async () => { removeCalls += 1; },
  });
  const container = await mount(client);
  await act(async () => button(container, 'Shared to (1)')?.click());

  let confirmCalls = 0;
  window.confirm = () => { confirmCalls += 1; return true; };
  await act(async () => labelledButton(container, 'Remove share')?.click());

  assert.equal(removeCalls, 1);
  assert.equal(confirmCalls, 0, 'Vue removes the share without an extra confirmation prompt');
});

test('shows an unshare failure without firing the change callback', async () => {
  let changed = 0;
  const client = clientFor(async () => ({ items: [{ id: 'share-1', organization_id: 'org-editor', organization_name: 'Editors', permission: 'viewer' }], total: 1 }), {
    remove: async () => { throw new Error('remove request failed'); },
  });
  const container = await mount(client, () => { changed += 1; });
  await act(async () => button(container, 'Shared to (1)')?.click());
  window.confirm = () => true;

  await act(async () => labelledButton(container, 'Remove share')?.click());

  assert.match(container.textContent ?? '', /remove request failed/);
  assert.doesNotMatch(container.textContent ?? '', /Share cancelled/);
  assert.equal(changed, 0);
  assert.equal(labelledButton(container, 'Remove share')?.classList.contains('t-is-disabled'), false);
});

test('ignores a stale load when the knowledge base changes while requests are pending', async () => {
  const first = deferred<{ items: Share[]; total: number }>();
  const second = deferred<{ items: Share[]; total: number }>();
  let calls = 0;
  const client = clientFor(async () => { calls += 1; return calls === 1 ? first.promise : second.promise; });
  const container = await mount(client);
  await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
  await act(async () => {
    mountedRoot?.render(<KnowledgeBaseShareDialog client={client} knowledgeBaseId="kb-2" knowledgeBaseName="Docs" open onClose={() => undefined} />);
  });
  await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
  await act(async () => second.resolve({ items: [{ id: 'share-2', organization_id: 'org-editor', organization_name: 'Current KB', permission: 'viewer' }], total: 1 }));
  await act(async () => first.resolve({ items: [{ id: 'share-1', organization_id: 'org-editor', organization_name: 'Stale KB', permission: 'viewer' }], total: 1 }));
  await act(async () => button(container, 'Shared to (1)')?.click());
  assert.match(container.textContent ?? '', /Current KB/);
  assert.doesNotMatch(container.textContent ?? '', /Stale KB/);
});

test('does not report a successful share when the post-mutation reload fails', async () => {
  let listCalls = 0;
  let changed = 0;
  const client = clientFor(async () => {
    listCalls += 1;
    if (listCalls === 1) return { items: [], total: 0 };
    throw new Error('reload failed');
  }, { create: async () => ({ id: 'share-1' }) });
  const container = await mount(client, () => { changed += 1; });
  await select(container, 'Select Shared Space', 'org-editor');
  await act(async () => button(container, 'Confirm')?.click());
  assert.match(container.textContent ?? '', /reload failed/);
  assert.doesNotMatch(container.textContent ?? '', /Knowledge base shared/);
  assert.equal(changed, 0);
});

// R462: the inline mount lives inside the editor save <form> (App.tsx
// `onSubmit={save}`, share section of the "integration" nav group). Vue
// KBShareSettings (KnowledgeBaseEditorModal share section) renders no <form>
// and confirms via t-button @click, so the React inline mount must not emit a
// <form> either — a nested form breaks HTML parsing/hydration.
test('inline share section renders without a form so the editor save form never nests', async () => {
  const client = clientFor(async () => ({ items: [], total: 0 }));
  const container = await mountInline(client);
  await openInlineShareForm(container);

  assert.ok(container.textContent?.includes('Select Shared Space'), 'the add-share form should be visible');
  assert.equal(container.querySelector('form'), null, 'the inline share section must not render a <form> (Vue KBShareSettings has none)');
  const confirm = button(container, 'Confirm');
  assert.ok(confirm, 'confirm action stays available');
  assert.equal(confirm?.getAttribute('type'), 'button', 'Vue confirms with @click, not a submit button');
});

test('inline share inside the outer save form submits without nested forms and keeps the guards', async () => {
  const payloads: unknown[] = [];
  const client = clientFor(async () => ({ items: [], total: 0 }), {
    create: async (_kbId, payload) => { payloads.push(payload); return { id: 'share-1' }; },
  });
  const outer = document.createElement('form');
  document.body.append(outer);
  const container = await mountInline(client, outer);
  await openInlineShareForm(container);

  assert.equal(outer.querySelectorAll('form').length, 0, 'no nested <form> inside the outer save form');
  // Vue-style org selection through the combobox; inline has no form-select fallback.
  await act(async () => container.querySelector<HTMLButtonElement>('[role="combobox"]')?.click());
  await act(async () => container.querySelector<HTMLButtonElement>('[role="option"]')?.click());

  const confirm = button(container, 'Confirm');
  assert.ok(confirm);
  assert.equal(confirm?.classList.contains('t-is-disabled'), false, 'confirm enables once an organization is picked');
  await act(async () => confirm?.click());
  assert.deepEqual(payloads, [{ organization_id: 'org-editor', permission: 'viewer' }], 'clicking confirm still submits the share like Vue handleShare');
});
