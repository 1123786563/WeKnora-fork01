import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// B1 question-minimap（ChatPage 渲染路径）经 window.requestAnimationFrame
// 调度测量；jsdom 无 raf，window 侧与 globalThis 侧都垫 setTimeout 帧垫片
// （同 settings 域判例 SkillSettingsPanel.test）。
const w = dom.window as unknown as { requestAnimationFrame?: unknown; cancelAnimationFrame?: unknown };
w.requestAnimationFrame = w.requestAnimationFrame ?? ((cb: (t: number) => void) => setTimeout(() => cb(Date.now()), 16));
w.cancelAnimationFrame = w.cancelAnimationFrame ?? ((id: ReturnType<typeof setTimeout>) => clearTimeout(id));
(globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame
  = (globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame ?? w.requestAnimationFrame;
(globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame
  = (globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame ?? w.cancelAnimationFrame;
// B1 question-minimap 指针粗细探测（question-minimap.tsx setIsCoarsePointer）：
// jsdom 无 matchMedia，垫 coarse=false 的最小桩。
w.matchMedia = w.matchMedia ?? ((query: string) => ({ matches: false, media: query, addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {}, onchange: null, dispatchEvent: () => false }));
const { createRoot } = await import('react-dom/client');
const { ChatComposer, ChatPage, resolveChatCopy } = await import('@weknora/views');

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
  // Vue MentionSelector 同构：弹层无搜索框，键盘导航挂在 textarea（wk-chat-draft）。
  const textarea = container.querySelector<HTMLTextAreaElement>('#wk-chat-draft');
  assert.ok(textarea);
  const options = () => [...container.querySelectorAll<HTMLElement>('[role="option"]')];
  assert.equal(options()[0]?.getAttribute('id'), 'wk-chat-mention-option-kb-2');
  assert.equal(options()[0]?.getAttribute('aria-selected'), 'true');
  await act(async () => textarea?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })));
  assert.equal(options()[1]?.getAttribute('aria-selected'), 'true');
  await act(async () => textarea?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true })));
  assert.equal(options()[0]?.getAttribute('aria-selected'), 'true');
  await act(async () => textarea?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })));
  await act(async () => textarea?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })));
  assert.deepEqual(selected, ['kb-3']);
  assert.equal(container.querySelector('[role="listbox"]'), null);
  await act(async () => trigger?.click());
  await act(async () => textarea?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })));
  assert.equal(container.querySelector('[role="listbox"]'), null);
  await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="关闭: 产品文档"]')?.click());
  assert.deepEqual(removed, ['kb-1']);
});

test('resource mention options expose stable type markers for non-KB resources', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatComposer copy={resolveChatCopy('zh-CN')} draft="" onDraftChange={() => undefined} onSubmit={() => undefined} mentionOptions={[
    { id: 'file-1', name: '设计文档', type: 'file' },
    { id: 'tag-1', name: '重要', type: 'tag' },
    { id: 'mcp-1', name: 'Docs MCP', type: 'mcp' },
    { id: 'skill-1', name: 'summarize', type: 'skill' },
  ]} />));
  await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="知识库"]')?.click());
  assert.deepEqual([...container.querySelectorAll('[role="option"]')].map((node) => node.getAttribute('data-mention-type')), ['file', 'tag', 'mcp', 'skill']);
  // Vue .mention-item 结构：icon-wrap > svg（sprite 类型图标）替代旧文本 marker。
  assert.equal(container.querySelectorAll('[role="option"] .icon-wrap svg').length, 4);
});

test('streaming steer picker supports keyboard navigation and blocks existing attachments', async () => {
  const selected: string[] = [];
  const steers: string[] = [];
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatPage
    sessions={[{ id: 'session-1', title: 'Chat', is_pinned: false }]}
    selectedSessionId="session-1"
    messages={[]}
    draft=""
    locale="zh-CN"
    onSelectSession={() => undefined}
    onCreateSession={() => undefined}
    onDraftChange={() => undefined}
    send={async () => undefined}
    mentionOptions={[{ id: 'kb-1', name: '产品文档', type: 'kb' }, { id: 'kb-2', name: 'FAQ', type: 'kb' }, { id: 'kb-3', name: '研发手册', type: 'kb' }]}
    mentionedItems={[]}
    onMentionSelect={(item) => { selected.push(item.id); }}
    onSteer={async (content) => { steers.push(content); }}
    attachments={[{ id: 'att-1', name: 'guide.pdf', status: 'ready', attachmentId: 'att-1' }]}
    stream={{ phase: 'streaming', thinking: '', references: [], toolCalls: [] }}
  />));
  const trigger = container.querySelector<HTMLButtonElement>('#wk-chat-steer-mention');
  assert.ok(trigger);
  await act(async () => trigger?.click());
  const search = container.querySelector<HTMLInputElement>('[role="listbox"] input');
  assert.ok(search);
  assert.equal(search?.getAttribute('aria-activedescendant'), 'wk-chat-steer-mention-option-kb-1');
  assert.equal(container.querySelector('[role="option"]')?.getAttribute('aria-selected'), 'true');
  await act(async () => search?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })));
  assert.equal(search?.getAttribute('aria-activedescendant'), 'wk-chat-steer-mention-option-kb-2');
  assert.equal(container.querySelector('#wk-chat-steer-mention-option-kb-2')?.getAttribute('aria-selected'), 'true');
  await act(async () => search?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })));
  assert.deepEqual(selected, ['kb-2']);
  assert.equal(container.querySelector('[role="listbox"]'), null);
  await act(async () => trigger?.click());
  const escapeSearch = container.querySelector<HTMLInputElement>('[role="listbox"] input');
  assert.ok(escapeSearch);
  await act(async () => escapeSearch?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })));
  assert.equal(container.querySelector('[role="listbox"]'), null);

  const draft = container.querySelector<HTMLTextAreaElement>('#wk-chat-steer-draft');
  assert.ok(draft);
  await act(async () => {
    Object.getOwnPropertyDescriptor(dom.window.HTMLTextAreaElement.prototype, 'value')?.set?.call(draft, 'please use the attachment');
    draft!.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  assert.equal(draft?.value, 'please use the attachment');
  await act(async () => container.querySelector<HTMLButtonElement>('form.wk-chat-steer button[type="submit"]')?.click());
  assert.deepEqual(steers, []);
  assert.match(container.querySelector('form.wk-chat-steer [role="alert"]')?.textContent ?? '', /请先移除附件/);
});
