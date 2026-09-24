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
// B1 bookmark 抽屉 tdesign 化（Drawer/Tooltip 含 popup 链）后，tdesign 的
// listener/popup 模块在加载期探测 document、渲染期引用全局 Element 系列，
// 且 TTooltip/TPopup 依赖 MutationObserver/requestAnimationFrame/getComputedStyle
// 全局垫片——同 settings 域判例（SkillSettingsPanel.test 的顺序约定：
// 垫片必须在动态 import 组件之前就位）。
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  ShadowRoot: dom.window.ShadowRoot,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  FocusEvent: dom.window.FocusEvent,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  IS_REACT_ACT_ENVIRONMENT: true,
});
(globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame
  = (globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame
  ?? ((cb: (t: number) => void) => setTimeout(() => cb(Date.now()), 16));
(globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame
  = (globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame
  ?? ((id: ReturnType<typeof setTimeout>) => clearTimeout(id));

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
// B1 px-chat-addtokb 收敛：对话框换壳 tdesign 右抽屉（Drawer/Input/Select/Textarea，
// portal 挂 document.body）。KB 选择走 .t-select popup（.t-select-option 点击，
// ResourceSettingsPanel.test 判例），无原生 <select>；tag 多选 UI 让渡给上传确认流
// （save 恒传 tag_ids: []，BookmarkAnswerDialog.tsx "upload-confirm owns tags"），
// 对应 R491 tag 深特性测试随组件面收敛移除。

interface ClientLike {
  knowledgeBases: {
    list: () => Promise<Array<{ id: string; name: string; type?: string }>>;
    documents: {
      createManual: (kbId: string, input: Record<string, unknown>) => Promise<unknown>;
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
  // Flush the KB list promise and settle the tdesign Drawer portal.
  await act(async () => { await Promise.resolve(); await new Promise((resolve) => setTimeout(resolve, 30)); });
  return () => act(() => current.unmount());
}

// tdesign Select 无原生 <select>：点 .t-select__wrap .t-input 开 popup，
// 点 document.body 里对应文本的 .t-select-option（ResourceSettingsPanel.test 判例）。
async function selectKb(label: string): Promise<void> {
  const trigger = document.querySelector('.t-select__wrap .t-input') as HTMLElement;
  assert.ok(trigger, 'kb select trigger rendered');
  await act(async () => { trigger.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 30)); });
  const option = [...document.querySelectorAll('.t-select-option')]
    .find((node) => (node.textContent ?? '').trim() === label);
  assert.ok(option, `expected the ${label} option in the kb select popup`);
  await act(async () => { option.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 30)); });
}

test('bookmark dialog prefills the Vue editor fields and offers document KBs only', async () => {
  const { client } = makeClient();
  const unmount = await mountDialog(client, () => {});
  const titleInput = document.querySelector('input.t-input__inner[placeholder="请输入标题"]') as HTMLInputElement;
  assert.ok(titleInput, 'title input rendered');
  assert.equal(titleInput.value, '什么是 WeKnora？');
  const contentArea = document.querySelector('textarea.t-textarea__inner') as HTMLTextAreaElement;
  assert.ok(contentArea, 'content textarea rendered');
  assert.equal(contentArea.value, 'WeKnora 是一个知识库系统');
  const trigger = document.querySelector('.t-select__wrap .t-input') as HTMLElement;
  assert.ok(trigger, 'kb select trigger rendered');
  assert.equal(trigger.querySelector('input')?.placeholder, '请选择知识库', 'Vue manualEditor kb placeholder');
  await act(async () => { trigger.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 30)); });
  const options = [...document.querySelectorAll('.t-select-option')].map((node) => (node.textContent ?? '').trim());
  assert.deepEqual(options, ['文档库'], 'faq KB filtered out of the picker');
  unmount();
});

test('bookmark dialog warns before saving without a KB or title (Vue manualEditor warnings)', async () => {
  // B1 预选：列表非空时 initialize 预选首个文档库（BookmarkAnswerDialog.tsx
  // "Vue manual-knowledge-editor.vue initialize (471-483) preselects the first
  // KB"）——「无 KB」警告仅在列表为空（或全 faq）时可达。
  const empty = makeClient();
  empty.client.knowledgeBases.list = () => Promise.resolve([{ id: 'kb-faq', name: 'FAQ库', type: 'faq' }]);
  const unmountEmpty = await mountDialog(empty.client, () => {});
  const saveEmpty = [...document.querySelectorAll('button')].find((button) => button.textContent === '暂存草稿')!;
  await act(async () => { saveEmpty.click(); });
  assert.equal(empty.created.length, 0, 'no request without a KB');
  assert.ok(document.body.textContent!.includes('请选择目标知识库'), 'KB warning shown');
  unmountEmpty();

  const { client, created } = makeClient();
  const unmount = await mountDialog(client, () => {});
  const save = [...document.querySelectorAll('button')].find((button) => button.textContent === '暂存草稿')!;
  // KB 已预选：清空标题触发 Vue enterTitle 警告。
  const titleInput = document.querySelector('input.t-input__inner[placeholder="请输入标题"]') as HTMLInputElement;
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
  await selectKb('文档库');
  const save = [...document.querySelectorAll('button')].find((button) => button.textContent === '暂存草稿')!;
  await act(async () => { save.click(); await new Promise((resolve) => setTimeout(resolve, 30)); });
  // Vue manual editor payload (manual-knowledge-editor.vue:627-638). B1 keeps
  // tag_ids always [] (upload-confirm owns tags — BookmarkAnswerDialog.tsx save()).
  assert.deepEqual(created, [{ kbId: 'kb-doc', input: { title: '什么是 WeKnora？', content: 'WeKnora 是一个知识库系统', status: 'draft', tag_ids: [] } }]);
  assert.equal(saved, 1, 'host toast hook fired');
  unmount();
});

// --- B1 收敛后的剩余行为面 ---------------------------------------------------------
// B1 px-chat-addtokb 把 R491 P2 的 tag 多选深特性让渡给上传确认流（save 恒传
// tag_ids: []，BookmarkAnswerDialog.tsx "upload-confirm owns tags"），四个 tag
// 加载/切换/失败/空态测试随组件面移除；preview 切换与发布校验保留，DOM 断言按
// tdesign 化后的双 pane（display 切换，.editor-pane / .editor-pane--preview）适配。

test('preview toggle renders markdown and returns to editing (Vue manualEditor view toggle)', async () => {
  const { client } = makeClient();
  const unmount = await mountDialog(client, () => {}, { initialContent: '# WeKnora 标题\n\n正文段落' });
  const toggle = [...document.querySelectorAll('button')].find((button) => button.textContent === '预览内容')!;
  assert.ok(toggle, 'preview toggle rendered in edit mode (Vue view.previewLabel)');
  await act(async () => { toggle.click(); });
  // B1 双 pane 常挂（display 切换）：编辑 pane 藏、预览 pane 现。
  const editPane = document.querySelector('.editor-pane:not(.editor-pane--preview)') as HTMLElement;
  const previewPane = document.querySelector('.editor-pane--preview') as HTMLElement;
  assert.equal(editPane.style.display, 'none', 'editor pane hidden in preview mode');
  assert.notEqual(previewPane.style.display, 'none', 'preview pane shown');
  const heading = previewPane.querySelector('.preview-container h1');
  assert.ok(heading, 'markdown rendered in preview');
  assert.equal(heading?.textContent, 'WeKnora 标题');
  assert.ok(previewPane.querySelector('.preview-container')?.textContent!.includes('正文段落'));
  const back = [...document.querySelectorAll('button')].find((button) => button.textContent === '返回编辑')!;
  assert.ok(back, 'toggle now offers edit mode (Vue view.editLabel)');
  await act(async () => { back.click(); });
  assert.notEqual((document.querySelector('.editor-pane:not(.editor-pane--preview)') as HTMLElement).style.display, 'none', 'editor pane restored');
  assert.equal((document.querySelector('textarea.t-textarea__inner') as HTMLTextAreaElement).value, '# WeKnora 标题\n\n正文段落', 'content preserved across the toggle');
  unmount();
});

test('preview shows the empty placeholder when the content is blank', async () => {
  const { client } = makeClient();
  const unmount = await mountDialog(client, () => {}, { initialContent: '   ' });
  const toggle = [...document.querySelectorAll('button')].find((button) => button.textContent === '预览内容')!;
  await act(async () => { toggle.click(); });
  const previewPane = document.querySelector('.editor-pane--preview') as HTMLElement;
  assert.ok(previewPane.textContent!.includes('暂无内容'), 'Vue manualEditor.preview.empty placeholder');
  unmount();
});

test('publish action validates KB, blank and too-short content with the Vue manualEditor warnings', async () => {
  // KB 警告分支仅在列表为空（或全 faq）时可达（B1 预选首个文档库，同上注）。
  const empty = makeClient();
  empty.client.knowledgeBases.list = () => Promise.resolve([{ id: 'kb-faq', name: 'FAQ库', type: 'faq' }]);
  const unmountEmpty = await mountDialog(empty.client, () => {});
  const publishEmpty = [...document.querySelectorAll('button')].find((button) => button.textContent === '发布入库')!;
  assert.ok(publishEmpty, 'publish action rendered (Vue manualEditor.actions.publish)');
  await act(async () => { publishEmpty.click(); });
  assert.equal(empty.created.length, 0, 'no request without a KB');
  assert.ok(document.body.textContent!.includes('请选择目标知识库'));
  unmountEmpty();

  // 正常列表（B1 已预选首个文档库）：空白内容 → enterContent；短内容 → contentTooShort。
  const { client, created } = makeClient();
  const unmount = await mountDialog(client, () => {});
  const publish = [...document.querySelectorAll('button')].find((button) => button.textContent === '发布入库')!;
  // Blank the content: Vue warning.enterContent fires before the length rule.
  const textarea = document.querySelector('textarea.t-textarea__inner') as HTMLTextAreaElement;
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

test('publish action posts status publish and reports the publish status', async () => {
  const { client, created } = makeClient();
  const statuses: string[] = [];
  const unmount = await mountDialog(client, (status) => { statuses.push(status); });
  await selectKb('文档库');
  const publish = [...document.querySelectorAll('button')].find((button) => button.textContent === '发布入库')!;
  await act(async () => { publish.click(); await new Promise((resolve) => setTimeout(resolve, 30)); });
  assert.deepEqual(created, [{ kbId: 'kb-doc', input: { title: '什么是 WeKnora？', content: 'WeKnora 是一个知识库系统', status: 'publish', tag_ids: [] } }]);
  assert.deepEqual(statuses, ['publish'], 'host toast hook receives the publish status');
  unmount();
});
