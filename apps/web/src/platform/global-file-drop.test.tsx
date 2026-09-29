// 全局拖拽上传（Vue platform/index.vue:69-232 同构平移）jsdom 契约测试：
//   - document 级 dragenter/dragover/dragleave/drop 监听的挂载/卸载与计数式
//     遮罩显隐（dragenter++/dragleave--/drop 重置，子元素抖动不误隐藏）；
//   - 仅 dataTransfer.types 含 "Files" 的 OS 文件拖拽接管（元素拖拽放行）；
//   - 聊天路由 drop → weknora:chat-file-drop（stopPropagation）；
//   - 知识库路由 drop → 先校验 KB 初始化，再派发 weknora:knowledge-file-drop；
//   - 无 kbId / 未初始化 / 拉取失败 / 空文件集 → Vue 同款提示文案；
//   - 遮罩文案中英文（file.upload + pdfDocFormat/textMarkdownFormat）。
// Harness 跟随 apps/web/src/platform/platform-shell-guide-reopen.test.tsx。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// PlatformShell imports .css/.svg 资产；ESM loader 置空模块。
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') || specifier.endsWith('.png')
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
  IS_REACT_ACT_ENVIRONMENT: true,
});
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default locale */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { PlatformShell } = await import('./PlatformShell.tsx');
const {
  isChatFileDropPath,
  kbIdFromPathname,
  knowledgeBaseInitializationWarning,
  fileUploadTitle,
} = await import('./global-file-drop.ts');
const { formatMessage } = await import('@weknora/i18n');

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
  window.history.pushState({}, '', '/platform/knowledge-bases');
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

function fakeClient(kb: Record<string, unknown> | null = { summary_model_id: 'summary-1', embedding_model_id: 'embed-1' }): Record<string, unknown> {
  return {
    auth: { me: async () => ({ user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '' }, tenant: { id: 'tenant-1', name: 'Parity' }, memberships: [] }) },
    sessions: { list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }) },
    knowledgeBases: { settings: { get: async () => kb } },
  };
}

async function mountShell(url: string, client: Record<string, unknown> = fakeClient(), locale = 'zh-CN') {
  window.history.pushState({}, '', url);
  window.localStorage.setItem('locale', locale);
  window.localStorage.setItem('weknora:new-user-guide-done:v1', '1');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(
      PlatformShell,
      { client: client as never, onLogout: () => undefined, children: React.createElement('div', null, 'page') },
    ));
  });
  await settle(20);
  return container;
}

type FakeDataTransfer = { types: string[]; files: File[]; items?: unknown[]; effectAllowed?: string; dropEffect?: string };

function fireDragEvent(type: 'dragenter' | 'dragover' | 'dragleave' | 'drop', dataTransfer: FakeDataTransfer, target: EventTarget = document.body): { event: Event } {
  const event = new dom.window.Event(type, { bubbles: true, cancelable: true });
  Object.defineProperty(event, 'dataTransfer', { value: dataTransfer });
  act(() => { target.dispatchEvent(event); });
  return { event };
}

const fileDrag = (files: File[]): FakeDataTransfer => ({ types: ['Files'], files, items: [] });
const textDrag = (): FakeDataTransfer => ({ types: ['text/plain'], files: [], items: [] });
const aFile = (name: string) => new dom.window.File([`-- ${name} fixture`], name, { type: 'text/markdown' });

const mask = () => document.querySelector('[data-testid="wk-upload-drag-mask"]');
const notice = () => document.querySelector('[role="status"].wk-shell-6');
const noticeText = () => notice()?.textContent ?? '';

async function captureDrop(url: string, files: File[], client?: Record<string, unknown>): Promise<{ chatDrop: { files?: File[] } | null; kbDrop: { kbId?: string; files?: File[] } | null; event: Event }> {
  const drops: { chatDrop: { files?: File[] } | null; kbDrop: { kbId?: string; files?: File[] } | null } = { chatDrop: null, kbDrop: null };
  const onChatDrop = (event: Event) => { drops.chatDrop = (event as CustomEvent).detail; };
  const onKbDrop = (event: Event) => { drops.kbDrop = (event as CustomEvent).detail; };
  window.addEventListener('weknora:chat-file-drop', onChatDrop);
  window.addEventListener('weknora:knowledge-file-drop', onKbDrop);
  await mountShell(url, client);
  const { event } = fireDragEvent('drop', fileDrag(files));
  await settle(10);
  window.removeEventListener('weknora:chat-file-drop', onChatDrop);
  window.removeEventListener('weknora:knowledge-file-drop', onKbDrop);
  return { ...drops, event };
}

// ---------- 纯函数（Vue 判定的 pathname/KB 投影） ----------

test('isChatFileDropPath covers the three Vue chat drop routes', () => {
  assert.equal(isChatFileDropPath('/platform/creatChat'), true, 'globalCreatChat');
  assert.equal(isChatFileDropPath('/platform/chat/abc-123'), true, 'chat detail');
  assert.equal(isChatFileDropPath('/platform/knowledge-bases/kb-9/creatChat'), true, 'kbCreatChat');
  assert.equal(isChatFileDropPath('/platform/knowledge-bases/kb-9'), false, 'KB detail is not a chat route');
  assert.equal(isChatFileDropPath('/knowledgeBase/kb-9'), false, 'React-native KB route is not a chat route');
  assert.equal(isChatFileDropPath('/platform/agents'), false);
});

test('kbIdFromPathname mirrors Vue route.params.kbId for both KB path forms', () => {
  assert.equal(kbIdFromPathname('/knowledgeBase/kb-1'), 'kb-1');
  assert.equal(kbIdFromPathname('/knowledgeBase/kb-1/documents/doc-2'), 'kb-1');
  assert.equal(kbIdFromPathname('/platform/knowledge-bases/kb-1/wiki'), 'kb-1');
  assert.equal(kbIdFromPathname('/platform/knowledge-bases/kb-1/creatChat'), 'kb-1', 'chat branch consumes it first, but the id still resolves');
  assert.equal(kbIdFromPathname('/platform/creatChat'), null);
  assert.equal(kbIdFromPathname('/platform/agents'), null);
});

test('knowledgeBaseInitializationWarning follows the Vue model checks', () => {
  assert.equal(knowledgeBaseInitializationWarning(null), 'knowledgeBase.notInitialized');
  assert.equal(knowledgeBaseInitializationWarning({}), 'knowledgeBase.notInitialized', 'no summary model');
  assert.equal(knowledgeBaseInitializationWarning({ summary_model_id: 's' }), 'knowledgeBase.notInitialized', 'default strategy needs embedding');
  assert.equal(knowledgeBaseInitializationWarning({ summary_model_id: 's', embedding_model_id: 'e' }), null);
  assert.equal(knowledgeBaseInitializationWarning({ summary_model_id: 's', indexing_strategy: { vector_enabled: true }, embedding_model_id: 'e' }), null);
  assert.equal(knowledgeBaseInitializationWarning({ summary_model_id: 's', indexing_strategy: { vector_enabled: false, keyword_enabled: true }, embedding_model_id: '' }), 'knowledgeBase.notInitialized', 'keyword strategy still needs embedding');
  assert.equal(knowledgeBaseInitializationWarning({ summary_model_id: 's', indexing_strategy: { vector_enabled: false, keyword_enabled: false } }), null, 'no embedding needed when both indexes are off');
});

test('fileUploadTitle carries the Vue file.upload copy per locale', () => {
  assert.equal(fileUploadTitle('zh-CN'), '上传文件');
  assert.equal(fileUploadTitle('en-US'), 'Upload File');
  assert.equal(fileUploadTitle('ja-JP'), 'ファイルをアップロード');
  assert.equal(fileUploadTitle('ko-KR'), '파일 업로드');
  assert.equal(fileUploadTitle('ru-RU'), 'Загрузить файл');
});

// ---------- 遮罩显隐（计数式判定） ----------

test('file dragenter shows the mask and paired dragleave hides it (counter, not per-event)', async () => {
  await mountShell('/platform/knowledge-bases');
  assert.equal(mask(), null, 'mask hidden before any drag');

  fireDragEvent('dragenter', fileDrag([aFile('a.md')]));
  assert.ok(mask(), 'mask shows on the first file dragenter');

  fireDragEvent('dragenter', fileDrag([aFile('a.md')]));
  fireDragEvent('dragleave', fileDrag([aFile('a.md')]));
  assert.ok(mask(), 'nested enter/leave keeps the mask (counter > 0)');

  fireDragEvent('dragleave', fileDrag([aFile('a.md')]));
  assert.equal(mask(), null, 'mask hides when the counter drains to 0');
});

test('drop resets the counter so the next drag starts clean', async () => {
  await mountShell('/platform/agents', fakeClient(null));
  fireDragEvent('dragenter', fileDrag([aFile('a.md')]));
  fireDragEvent('drop', fileDrag([]));
  assert.equal(mask(), null, 'drop hides the mask');

  // A stray post-drop dragleave must not drive the counter negative and
  // break the next drag cycle (clamped at 0, a superset of the Vue counter).
  fireDragEvent('dragleave', fileDrag([aFile('a.md')]));
  fireDragEvent('dragenter', fileDrag([aFile('a.md')]));
  assert.ok(mask(), 'the next drag still shows the mask after a stray leave');
});

test('element drags (text/* only) never open the mask nor get prevented', async () => {
  await mountShell('/platform/knowledge-bases');
  const enter = fireDragEvent('dragenter', textDrag());
  assert.equal(mask(), null, 'wiki-style element drags do not trigger the mask');
  assert.equal(enter.event.defaultPrevented, false, 'element drags are left to the originating component');

  const over = fireDragEvent('dragover', textDrag());
  assert.equal(over.event.defaultPrevented, false);

  const drop = fireDragEvent('drop', textDrag());
  assert.equal(drop.event.defaultPrevented, false);
  assert.equal(notice(), null, 'no drop notice for element drags');
});

test('file dragover is prevented with copy dropEffect (enables the drop)', async () => {
  await mountShell('/platform/knowledge-bases');
  const data = fileDrag([aFile('a.md')]);
  const { event } = fireDragEvent('dragover', data);
  assert.equal(event.defaultPrevented, true);
  assert.equal(data.dropEffect, 'copy');
});

// ---------- 遮罩文案 ----------

test('mask copy mirrors the Vue upload-mask.vue strings (zh-CN and en-US)', async () => {
  await mountShell('/platform/knowledge-bases');
  fireDragEvent('dragenter', fileDrag([aFile('a.md')]));
  const zh = mask();
  assert.ok(zh);
  assert.equal(zh!.querySelector('.drag-txt')?.textContent, '上传文件');
  assert.equal(zh!.querySelectorAll('.drag-type-txt')[0]?.textContent, formatMessage('zh-CN', 'knowledgeBase.pdfDocFormat'));
  assert.equal(zh!.querySelectorAll('.drag-type-txt')[1]?.textContent, formatMessage('zh-CN', 'knowledgeBase.textMarkdownFormat'));

  await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();

  await mountShell('/platform/knowledge-bases', fakeClient(), 'en-US');
  fireDragEvent('dragenter', fileDrag([aFile('a.md')]));
  const en = mask();
  assert.ok(en);
  assert.equal(en!.querySelector('.drag-txt')?.textContent, 'Upload File');
  assert.equal(en!.querySelectorAll('.drag-type-txt')[0]?.textContent, formatMessage('en-US', 'knowledgeBase.pdfDocFormat'));
});

// ---------- drop 事件流 ----------

test('chat-route drop dispatches weknora:chat-file-drop (stopPropagation after the collect await, Vue placement)', async () => {
  const file = aFile('chat-note.md');
  const { chatDrop, kbDrop, event } = await captureDrop('/platform/creatChat', [file]);
  assert.ok(chatDrop, 'chat drop event dispatched');
  assert.deepEqual(chatDrop!.files, [file], 'the CustomEvent detail carries the dropped File[]');
  assert.equal(kbDrop, null, 'knowledge drop is not dispatched on chat routes');
  // Vue platform/index.vue:164-169 在 await collectDroppedFiles 之后才
  // stopPropagation —— 异步续体里传播已完成，该调用与 Vue 一样不阻断
  // 本次派发（聊天页面无本地文件 dropzone，双重消费本就不存在）。
  // 真正同步生效的是 drop 的 preventDefault（阻止浏览器打开文件）。
  assert.equal(event.defaultPrevented, true, 'drop default prevented synchronously');
});

test('KB-route drop dispatches weknora:knowledge-file-drop with the current kbId after the init check', async () => {
  const file = aFile('kb-doc.md');
  const { chatDrop, kbDrop } = await captureDrop('/platform/knowledge-bases/kb-9', [file]);
  assert.equal(chatDrop, null);
  assert.ok(kbDrop, 'knowledge drop event dispatched');
  assert.equal(kbDrop!.kbId, 'kb-9');
  assert.deepEqual(kbDrop!.files, [file]);
});

test('a non-chat drop without a kbId toasts the Vue missingId copy and dispatches nothing', async () => {
  const { chatDrop, kbDrop } = await captureDrop('/platform/agents', [aFile(' stray.md')], fakeClient(null));
  assert.equal(chatDrop, null);
  assert.equal(kbDrop, null);
  assert.equal(noticeText(), formatMessage('zh-CN', 'knowledgeBase.missingId'));
});

test('an uninitialized KB blocks the dispatch with the Vue notInitialized copy', async () => {
  const { kbDrop } = await captureDrop('/platform/knowledge-bases/kb-9', [aFile('kb-doc.md')], fakeClient({}));
  assert.equal(kbDrop, null, 'no knowledge drop for an uninitialized KB');
  assert.equal(noticeText(), formatMessage('zh-CN', 'knowledgeBase.notInitialized'));
});

test('a failed KB fetch toasts getInfoFailed and dispatches nothing', async () => {
  const failingClient = {
    ...fakeClient(),
    knowledgeBases: { settings: { get: async () => { throw new Error('boom'); } } },
  };
  const { kbDrop } = await captureDrop('/platform/knowledge-bases/kb-9', [aFile('kb-doc.md')], failingClient);
  assert.equal(kbDrop, null);
  assert.equal(noticeText(), formatMessage('zh-CN', 'knowledgeBase.getInfoFailed'));
});

test('a file drop that yields no files toasts dragFileNotText (Vue empty-collection guard)', async () => {
  const { chatDrop, kbDrop } = await captureDrop('/platform/creatChat', []);
  assert.equal(chatDrop, null);
  assert.equal(kbDrop, null);
  assert.equal(noticeText(), formatMessage('zh-CN', 'knowledgeBase.dragFileNotText'));
});

// ---------- 卸载 ----------

test('unmount removes the document-level drag listeners and the mask', async () => {
  await mountShell('/platform/knowledge-bases');
  await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;

  const enter = fireDragEvent('dragenter', fileDrag([aFile('a.md')]));
  assert.equal(mask(), null, 'no mask after unmount');
  assert.equal(enter.event.defaultPrevented, false, 'listeners are detached (drop/dragover no longer prevented)');

  const drop = fireDragEvent('drop', fileDrag([aFile('a.md')]));
  assert.equal(drop.event.defaultPrevented, false);
  assert.equal(notice(), null);
});
