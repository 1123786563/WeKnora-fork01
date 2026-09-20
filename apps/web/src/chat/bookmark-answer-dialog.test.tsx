// R490 B3 — bookmark (添加到知识库) coverage: the Vue chatMessageShared
// prefill helpers and the manual-editor dialog wiring.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/creatChat' });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot } = await import('react-dom/client');
const { BookmarkAnswerDialog, buildManualBookmarkContent, formatManualBookmarkTitle } = await import('./BookmarkAnswerDialog.tsx');
const { resolveChatCopy } = await import('@weknora/views/chat/chat-copy');

const copy = resolveChatCopy('zh-CN');

// --- Vue chatMessageShared prefill helpers ---------------------------------------

test('formatManualBookmarkTitle mirrors Vue formatManualTitle (collapse, truncate, excerpt fallback)', () => {
  assert.equal(formatManualBookmarkTitle(undefined, copy.bookmarkSessionExcerpt), '会话摘录');
  assert.equal(formatManualBookmarkTitle('   ', copy.bookmarkSessionExcerpt), '会话摘录');
  assert.equal(formatManualBookmarkTitle('什么是  WeKnora？', copy.bookmarkSessionExcerpt), '什么是 WeKnora？');
  const long = 'a'.repeat(41);
  assert.equal(formatManualBookmarkTitle(long, copy.bookmarkSessionExcerpt), `${'a'.repeat(40)}...`);
  assert.equal(formatManualBookmarkTitle('a'.repeat(40), copy.bookmarkSessionExcerpt), 'a'.repeat(40));
});

test('buildManualBookmarkContent mirrors Vue buildManualMarkdown (trimmed answer, placeholder)', () => {
  assert.equal(buildManualBookmarkContent('  答案  ', copy.bookmarkNoAnswerContent), '答案');
  assert.equal(buildManualBookmarkContent('   ', copy.bookmarkNoAnswerContent), '（无回答内容）');
});

// --- BookmarkAnswerDialog ----------------------------------------------------------

interface ClientLike {
  knowledgeBases: {
    list: () => Promise<Array<{ id: string; name: string; type?: string }>>;
    documents: { createManual: (kbId: string, input: Record<string, unknown>) => Promise<unknown> };
  };
}

function makeClient(): { client: ClientLike; created: Array<{ kbId: string; input: Record<string, unknown> }> } {
  const created: Array<{ kbId: string; input: Record<string, unknown> }> = [];
  return {
    created,
    client: {
      knowledgeBases: {
        // faq KBs must not be offered (the Vue manual editor lists document KBs).
        list: () => Promise.resolve([
          { id: 'kb-doc', name: '文档库' },
          { id: 'kb-faq', name: 'FAQ库', type: 'faq' },
        ]),
        documents: {
          createManual: (kbId, input) => { created.push({ kbId, input }); return Promise.resolve({}); },
        },
      },
    },
  };
}

async function mountDialog(client: ClientLike, onSaved: () => void): Promise<() => void> {
  document.body.replaceChildren();
  const host = document.createElement('div');
  document.body.append(host);
  let root: { render: (node: React.ReactNode) => void; unmount: () => void } | null = null;
  act(() => { root = createRoot(host); });
  const current = root!;
  act(() => {
    current.render(React.createElement(BookmarkAnswerDialog, {
      client: client as never,
      copy,
      open: true,
      initialTitle: '什么是 WeKnora？',
      initialContent: 'WeKnora 是一个知识库系统',
      onClose: () => {},
      onSaved,
    }));
  });
  // Flush the KB list promise so the picker options are committed.
  await act(async () => { await Promise.resolve(); });
  return () => act(() => current.unmount());
}

test('bookmark dialog prefills the Vue editor fields and offers document KBs only', async () => {
  const { client } = makeClient();
  const unmount = await mountDialog(client, () => {});
  const titleInput = document.querySelector('input[maxlength="100"]') as HTMLInputElement;
  assert.ok(titleInput, 'title input rendered');
  assert.equal(titleInput.value, '什么是 WeKnora？');
  const contentArea = document.querySelector('textarea') as HTMLTextAreaElement;
  assert.equal(contentArea.value, 'WeKnora 是一个知识库系统');
  const select = document.querySelector('select') as HTMLSelectElement;
  const options = [...select.querySelectorAll('option')].map((option) => option.textContent);
  assert.deepEqual(options, ['请选择知识库', '文档库'], 'faq KB filtered out of the picker');
  unmount();
});

test('bookmark dialog warns before saving without a KB or title (Vue manualEditor warnings)', async () => {
  const { client, created } = makeClient();
  const unmount = await mountDialog(client, () => {});
  const save = [...document.querySelectorAll('button')].find((button) => button.textContent === '暂存草稿')!;
  await act(async () => { save.click(); });
  assert.equal(created.length, 0, 'no request without a KB');
  assert.ok(document.body.textContent!.includes('请选择目标知识库'), 'KB warning shown');

  const select = document.querySelector('select') as HTMLSelectElement;
  await act(async () => {
    select.value = 'kb-doc';
    select.dispatchEvent(new Event('change', { bubbles: true }));
  });
  const titleInput = document.querySelector('input[maxlength="100"]') as HTMLInputElement;
  const nativeSetter = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set!;
  await act(async () => {
    nativeSetter.call(titleInput, '   ');
    titleInput.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => { save.click(); });
  assert.equal(created.length, 0, 'no request with a blank title');
  assert.ok(document.body.textContent!.includes('请输入知识标题'), 'title warning shown');
  unmount();
});

test('bookmark dialog saves a draft through the manual endpoint', async () => {
  const { client, created } = makeClient();
  let saved = 0;
  const unmount = await mountDialog(client, () => { saved += 1; });
  const select = document.querySelector('select') as HTMLSelectElement;
  await act(async () => {
    select.value = 'kb-doc';
    select.dispatchEvent(new Event('change', { bubbles: true }));
  });
  const save = [...document.querySelectorAll('button')].find((button) => button.textContent === '暂存草稿')!;
  await act(async () => { save.click(); });
  assert.deepEqual(created, [{ kbId: 'kb-doc', input: { title: '什么是 WeKnora？', content: 'WeKnora 是一个知识库系统', status: 'draft' } }]);
  assert.equal(saved, 1, 'host toast hook fired');
  unmount();
});
