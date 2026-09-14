import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');
const { ChatComposer, resolveChatCopy } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

test('KB mention picker supports search, Enter selection, Escape, and chip removal', async () => {
  const selected: string[] = [];
  const removed: string[] = [];
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatComposer copy={resolveChatCopy('zh-CN')} draft="" onDraftChange={() => undefined} onSubmit={() => undefined} mentionOptions={[{ id: 'kb-1', name: '产品文档', type: 'kb' }, { id: 'kb-2', name: 'FAQ 库', type: 'kb' }, { id: 'kb-3', name: '研发手册', type: 'kb' }]} onMentionSelect={(item) => selected.push(item.id)} mentionedItems={[{ id: 'kb-1', name: '产品文档', type: 'kb' }]} onMentionRemove={(id) => removed.push(id)} />));
  const trigger = container.querySelector<HTMLButtonElement>('button[aria-label="知识库"]');
  assert.ok(trigger);
  await act(async () => trigger?.click());
  assert.ok(container.querySelector('[role="listbox"]'));
  assert.equal(trigger?.getAttribute('aria-expanded'), 'true');
  const search = container.querySelector<HTMLInputElement>('[role="listbox"] input');
  assert.ok(search);
  await act(async () => {
    search!.value = 'FAQ';
    search!.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  assert.match(container.querySelector('[role="option"]')?.textContent ?? '', /FAQ 库/);
  await act(async () => {
    search!.value = '';
    search!.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  const firstOption = container.querySelector<HTMLElement>('[role="option"]');
  assert.equal(firstOption?.getAttribute('id'), 'wk-chat-mention-option-kb-2');
  assert.equal(search?.getAttribute('aria-activedescendant'), 'wk-chat-mention-option-kb-2');
  await act(async () => search?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })));
  assert.equal(search?.getAttribute('aria-activedescendant'), 'wk-chat-mention-option-kb-3');
  await act(async () => search?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true })));
  assert.equal(search?.getAttribute('aria-activedescendant'), 'wk-chat-mention-option-kb-2');
  await act(async () => search?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })));
  await act(async () => search?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })));
  assert.deepEqual(selected, ['kb-3']);
  assert.equal(container.querySelector('[role="listbox"]'), null);
  await act(async () => trigger?.click());
  const escapeSearch = container.querySelector<HTMLInputElement>('[role="listbox"] input');
  assert.ok(escapeSearch);
  await act(async () => escapeSearch?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })));
  assert.equal(container.querySelector('[role="listbox"]'), null);
  await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="关闭: 产品文档"]')?.click());
  assert.deepEqual(removed, ['kb-1']);
});
