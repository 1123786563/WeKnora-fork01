import '../test-tdom-harness.ts'; // jsdom 全局（tdesign Popup 运行时）
// R490 B2 — TagManageDialog interaction coverage: the delete/rename/create
// flows against the injected actions, mirroring the Vue drawer's behaviors
// (confirm before delete, Enter submits, unchanged rename cancels).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge/kb-1/documents' });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  MouseEvent: dom.window.MouseEvent,
  KeyboardEvent: dom.window.KeyboardEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot } = await import('react-dom/client');
const { TagManageDialog, TagFilterPanel } = await import('./TagPickerDialog.tsx');
const { tagSurfaceT } = await import('./tags-locale.ts');

const tt = tagSurfaceT('zh-CN');
const tags = [
  { id: '7', name: '重要', knowledge_count: 12, seq_id: 7 },
  { id: '9', name: '合同', knowledge_count: 3, seq_id: 9 },
];

interface MountOptions {
  confirm?: boolean;
  changed: (payload?: { deletedTagId?: string }) => void;
  deleted?: (tag: unknown) => void;
  created?: (name: string) => void;
}

function mountDialog(options: MountOptions): { root: RootLike; unmount: () => void } {
  window.confirm = () => options.confirm ?? true;
  document.body.replaceChildren();
  const host = document.createElement('div');
  document.body.append(host);
  let root: RootLike | null = null;
  act(() => {
    root = createRoot(host);
  });
  const current = root!;
  act(() => {
    current.render(React.createElement(TagManageDialog, {
      t: tt,
      open: true,
      tags,
      createTag: (name: string) => { options.created?.(name); return Promise.resolve(); },
      updateTag: (tagId: string, name: string) => { options.created?.(`${tagId}:${name}`); return Promise.resolve(); },
      deleteTag: (tag: unknown) => { options.deleted?.(tag); return Promise.resolve(); },
      onClose: () => {},
      onChanged: options.changed,
    }));
  });
  return { root: current, unmount: () => act(() => current.unmount()) };
}

interface RootLike { render(node: React.ReactNode): void; unmount(): void }

test('delete flow confirms then reports the deleted tag id', async () => {
  const calls: Array<{ deletedTagId?: string } | undefined> = [];
  const deletedTags: unknown[] = [];
  const { unmount } = mountDialog({
    confirm: true,
    deleted: (tag) => deletedTags.push(tag),
    changed: (payload) => calls.push(payload),
  });
  const deleteButtons = [...document.querySelectorAll('button')].filter((button) => button.textContent === '删除');
  assert.equal(deleteButtons.length, 2, 'one delete button per row');
  await act(async () => { deleteButtons[0]!.click(); });
  assert.equal(deletedTags.length, 1, 'deleteTag action invoked');
  assert.deepEqual((deletedTags[0] as { id: string }).id, '7');
  assert.deepEqual(calls, [{ deletedTagId: '7' }], 'changed carries deletedTagId like the Vue drawer emit');
  unmount();
});

test('delete cancelled in the confirm dialog never calls the action', async () => {
  const calls: Array<{ deletedTagId?: string } | undefined> = [];
  const deletedTags: unknown[] = [];
  const { unmount } = mountDialog({
    confirm: false,
    deleted: (tag) => deletedTags.push(tag),
    changed: (payload) => calls.push(payload),
  });
  const deleteButtons = [...document.querySelectorAll('button')].filter((button) => button.textContent === '删除');
  await act(async () => { deleteButtons[0]!.click(); });
  assert.equal(deletedTags.length, 0);
  assert.equal(calls.length, 0);
  unmount();
});

test('rename flow submits through Enter and reports the change', async () => {
  const renamed: string[] = [];
  const { unmount } = mountDialog({
    confirm: true,
    created: (value) => renamed.push(value),
    changed: () => {},
  });
  const renameButton = [...document.querySelectorAll('button')].find((button) => button.textContent === '重命名')!;
  await act(async () => { renameButton.click(); });
  const input = document.querySelector('.tag-manage-row input') as HTMLInputElement;
  assert.ok(input, 'inline editor mounted');
  // React's controlled input ignores a plain value write; drive the native setter.
  const nativeSetter = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  assert.ok(nativeSetter, 'native value setter available');
  await act(async () => {
    nativeSetter!.call(input, '核心文档');
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => {
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  });
  assert.deepEqual(renamed, ['7:核心文档'], 'updateTag called with the trimmed name');
  unmount();
});

test('tag filter panel manage entry invokes onManage (Vue closes the panel first)', async () => {
  let managed = 0;
  document.body.replaceChildren();
  const host = document.createElement('div');
  document.body.append(host);
  let root: RootLike | null = null;
  act(() => { root = createRoot(host); });
  const current = root!;
  act(() => {
    current.render(React.createElement(TagFilterPanel, {
      t: tt,
      tags,
      selectedIds: [],
      onToggle: () => {},
      onClear: () => {},
      onClose: () => {},
      canManage: true,
      onManage: () => { managed += 1; },
    }));
  });
  const entry = [...document.querySelectorAll('button')].find((button) => button.textContent === '管理标签…');
  assert.ok(entry, 'manage entry rendered for contributors');
  await act(async () => { entry!.click(); });
  assert.equal(managed, 1);
  act(() => current.unmount());
});
