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
    documents: {
      createManual: (kbId: string, input: Record<string, unknown>) => Promise<unknown>;
      tagsPage: (kbId: string, params: Record<string, unknown>) => Promise<{ success: true; data: Array<{ id: string; name: string }> }>;
    };
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
          tagsPage: () => Promise.resolve({ success: true as const, data: [] }),
        },
      },
    },
  };
}

async function mountDialog(client: ClientLike, onSaved: (status: 'draft' | 'publish') => void, overrides: { initialContent?: string } = {}): Promise<() => void> {
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
      initialContent: overrides.initialContent ?? 'WeKnora 是一个知识库系统',
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
  // Vue manual editor payload (manual-knowledge-editor.vue:627-638): tag_ids
  // is always sent (empty array when nothing is selected).
  assert.deepEqual(created, [{ kbId: 'kb-doc', input: { title: '什么是 WeKnora？', content: 'WeKnora 是一个知识库系统', status: 'draft', tag_ids: [] } }]);
  assert.equal(saved, 1, 'host toast hook fired');
  unmount();
});

// --- R491 P2 — deep sub-features ported from the Vue manual editor ----------------
// Sources: frontend/src/components/manual-knowledge-editor.vue (preview toggle,
// publish action) and the manual-mode upload confirm (tag multi-select,
// UploadConfirmDialog.vue loadTags + tags section).

const DEEP_TAGS: Record<string, Array<{ id: string; name: string }>> = {
  'kb-doc': [{ id: 'tag-product', name: '产品' }, { id: 'tag-faq', name: '常见问题' }],
  'kb-doc2': [{ id: 'tag-other', name: '其他' }],
};

function makeDeepClient(options: { tagError?: boolean } = {}): { client: ClientLike; created: Array<{ kbId: string; input: Record<string, unknown> }>; tagCalls: Array<{ kbId: string; params: Record<string, unknown> }> } {
  const created: Array<{ kbId: string; input: Record<string, unknown> }> = [];
  const tagCalls: Array<{ kbId: string; params: Record<string, unknown> }> = [];
  return {
    created,
    tagCalls,
    client: {
      knowledgeBases: {
        list: () => Promise.resolve([
          { id: 'kb-doc', name: '文档库' },
          { id: 'kb-doc2', name: '文档库二' },
        ]),
        documents: {
          createManual: (kbId, input) => { created.push({ kbId, input }); return Promise.resolve({}); },
          tagsPage: (kbId, params) => {
            tagCalls.push({ kbId, params });
            if (options.tagError) return Promise.reject(new Error('boom'));
            return Promise.resolve({ success: true as const, data: DEEP_TAGS[kbId] ?? [] });
          },
        },
      },
    },
  };
}

async function selectKb(kbId: string): Promise<void> {
  const select = document.querySelector('select') as HTMLSelectElement;
  await act(async () => {
    select.value = kbId;
    select.dispatchEvent(new Event('change', { bubbles: true }));
  });
  // Flush the tag list promise triggered by the KB change.
  await act(async () => { await Promise.resolve(); });
}

test('tag multi-select loads per-KB tags with the Vue listKnowledgeTags params and submits tag_ids', async () => {
  const { client, created, tagCalls } = makeDeepClient();
  const unmount = await mountDialog(client, () => {});
  await selectKb('kb-doc');
  // Vue UploadConfirmDialog loadTags: listKnowledgeTags(kbId, { page: 1, page_size: 1000 }).
  assert.deepEqual(tagCalls, [{ kbId: 'kb-doc', params: { page: 1, page_size: 1000 } }]);
  const boxes = [...document.querySelectorAll('input[type="checkbox"]')] as HTMLInputElement[];
  assert.deepEqual(boxes.map((box) => box.getAttribute('aria-label')), ['产品', '常见问题'], 'tag checkboxes rendered');
  await act(async () => { boxes[0]!.click(); });
  await act(async () => { boxes[1]!.click(); });
  const save = [...document.querySelectorAll('button')].find((button) => button.textContent === '暂存草稿')!;
  await act(async () => { save.click(); });
  assert.deepEqual(created[0]?.input.tag_ids, ['tag-product', 'tag-faq'], 'draft payload carries the selected tag ids');
  unmount();
});

test('switching the KB reloads tags from the new KB and clears the selection', async () => {
  const { client, created, tagCalls } = makeDeepClient();
  const unmount = await mountDialog(client, () => {});
  await selectKb('kb-doc');
  await act(async () => { ((document.querySelector('input[type="checkbox"]') as HTMLInputElement).click()); });
  await selectKb('kb-doc2');
  assert.deepEqual(tagCalls.map((call) => call.kbId), ['kb-doc', 'kb-doc2']);
  const boxes = [...document.querySelectorAll('input[type="checkbox"]')] as HTMLInputElement[];
  assert.deepEqual(boxes.map((box) => box.getAttribute('aria-label')), ['其他'], 'new KB tags loaded');
  assert.ok(boxes.every((box) => !box.checked), 'selection cleared on KB switch');
  const save = [...document.querySelectorAll('button')].find((button) => button.textContent === '暂存草稿')!;
  await act(async () => { save.click(); });
  assert.deepEqual(created[0]?.input.tag_ids, [], 'cleared selection is not submitted');
  unmount();
});

test('tag loading failure surfaces the uploadConfirm tagsLoadFailed copy', async () => {
  const { client } = makeDeepClient({ tagError: true });
  const unmount = await mountDialog(client, () => {});
  await selectKb('kb-doc');
  assert.ok(document.body.textContent!.includes('标签加载失败，可稍后在文档列表中设置'), 'load-failure hint shown');
  unmount();
});

test('tag section shows the empty hint when the KB has no tags', async () => {
  const { client } = makeDeepClient();
  client.knowledgeBases.list = () => Promise.resolve([{ id: 'kb-empty', name: '空标签库' }]);
  const unmount = await mountDialog(client, () => {});
  await selectKb('kb-empty');
  assert.ok(document.body.textContent!.includes('当前知识库暂无标签，可上传后在标签管理中创建'), 'empty hint shown');
  unmount();
});

test('preview toggle renders markdown and returns to editing (Vue manualEditor view toggle)', async () => {
  const { client } = makeDeepClient();
  const unmount = await mountDialog(client, () => {}, { initialContent: '# WeKnora 标题\n\n正文段落' });
  const toggle = [...document.querySelectorAll('button')].find((button) => button.textContent === '预览内容')!;
  assert.ok(toggle, 'preview toggle rendered in edit mode (Vue view.previewLabel)');
  await act(async () => { toggle.click(); });
  assert.equal(document.querySelector('textarea'), null, 'editor pane hidden in preview mode');
  const heading = document.querySelector('.wk-bookmark-preview h1');
  assert.ok(heading, 'markdown rendered in preview');
  assert.equal(heading?.textContent, 'WeKnora 标题');
  assert.ok(document.querySelector('.wk-bookmark-preview')?.textContent!.includes('正文段落'));
  const back = [...document.querySelectorAll('button')].find((button) => button.textContent === '返回编辑')!;
  assert.ok(back, 'toggle now offers edit mode (Vue view.editLabel)');
  await act(async () => { back.click(); });
  assert.ok(document.querySelector('textarea'), 'editor pane restored');
  assert.equal((document.querySelector('textarea') as HTMLTextAreaElement).value, '# WeKnora 标题\n\n正文段落', 'content preserved across the toggle');
  unmount();
});

test('preview shows the empty placeholder when the content is blank', async () => {
  const { client } = makeDeepClient();
  const unmount = await mountDialog(client, () => {}, { initialContent: '   ' });
  const toggle = [...document.querySelectorAll('button')].find((button) => button.textContent === '预览内容')!;
  await act(async () => { toggle.click(); });
  const preview = document.querySelector('.wk-bookmark-preview');
  assert.ok(preview?.textContent!.includes('暂无内容'), 'Vue manualEditor.preview.empty placeholder');
  unmount();
});

test('publish action validates KB, blank and too-short content with the Vue manualEditor warnings', async () => {
  const { client, created } = makeDeepClient();
  const unmount = await mountDialog(client, () => {});
  const publish = [...document.querySelectorAll('button')].find((button) => button.textContent === '发布入库')!;
  assert.ok(publish, 'publish action rendered (Vue manualEditor.actions.publish)');
  await act(async () => { publish.click(); });
  assert.equal(created.length, 0, 'no request without a KB');
  assert.ok(document.body.textContent!.includes('请选择目标知识库'));

  await selectKb('kb-doc');
  // Blank the content: Vue warning.enterContent fires before the length rule.
  const textarea = document.querySelector('textarea') as HTMLTextAreaElement;
  const nativeAreaSetter = Object.getOwnPropertyDescriptor(dom.window.HTMLTextAreaElement.prototype, 'value')?.set!;
  await act(async () => {
    nativeAreaSetter.call(textarea, '   ');
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => { publish.click(); });
  assert.equal(created.length, 0, 'no request with blank content');
  assert.ok(document.body.textContent!.includes('请输入知识内容'), 'Vue warning.enterContent');

  // Short-but-present content: publish requires >= 10 trimmed chars (Vue
  // validateForm contentTooShort). The draft path is unaffected.
  await act(async () => {
    nativeAreaSetter.call(textarea, '太短');
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => { publish.click(); });
  assert.equal(created.length, 0, 'no publish request under 10 chars');
  assert.ok(document.body.textContent!.includes('内容过短，建议补充更多信息后再发布'), 'Vue warning.contentTooShort');
  unmount();
});

test('publish action posts status publish with tag_ids and reports the publish status', async () => {
  const { client, created } = makeDeepClient();
  const statuses: string[] = [];
  const unmount = await mountDialog(client, (status) => { statuses.push(status); });
  await selectKb('kb-doc');
  await act(async () => { ((document.querySelector('input[type="checkbox"]') as HTMLInputElement).click()); });
  const publish = [...document.querySelectorAll('button')].find((button) => button.textContent === '发布入库')!;
  await act(async () => { publish.click(); });
  assert.deepEqual(created, [{ kbId: 'kb-doc', input: { title: '什么是 WeKnora？', content: 'WeKnora 是一个知识库系统', status: 'publish', tag_ids: ['tag-product'] } }]);
  assert.deepEqual(statuses, ['publish'], 'host toast hook receives the publish status');
  unmount();
});
