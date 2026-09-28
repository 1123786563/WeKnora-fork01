import '../test-tdom-harness.ts'; // jsdom 全局（tdesign Popup 运行时）
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import * as React from 'react';
import { act } from 'react';
import test, { afterEach } from 'node:test';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledgeBase/kb-1?tab=wiki' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  FocusEvent: dom.window.FocusEvent,
  NodeFilter: dom.window.NodeFilter,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  PointerEvent: dom.window.PointerEvent,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });

const { createRoot } = await import('react-dom/client');
const { WikiPage } = await import('./WikiPage.tsx');

let root: Root | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
  window.sessionStorage.clear();
});

function setInputValue(input: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = input instanceof dom.window.HTMLTextAreaElement
    ? dom.window.HTMLTextAreaElement.prototype
    : dom.window.HTMLInputElement.prototype;
  const setValue = Object.getOwnPropertyDescriptor(proto, 'value')!.set!;
  setValue.call(input, value);
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
}

function wikiClient(): WeKnoraClient & { created: Array<Record<string, unknown>> } {
  const created: Array<Record<string, unknown>> = [];
  return {
    wiki: {
      list: async () => ({ pages: [], total: 0, page: 1, page_size: 50, total_pages: 0 }),
      stats: async () => ({ total_pages: 0, pages_by_type: {}, pending_issues: 0 }),
      folders: async () => ({ parent_id: '', folders: [] }),
      index: async () => ({ intro: '', version: 0, groups: [] }),
      get: async (_kbId: string, slug: string) => ({ id: `p-${slug}`, slug, title: slug, content: '', summary: '', version: 1 }),
      create: async (_kbId: string, input: Record<string, unknown>) => {
        created.push(input);
        return { id: 'p-new', slug: String(input.slug ?? ''), title: String(input.title ?? ''), content: String(input.content ?? ''), summary: '', version: 1 };
      },
    },
    knowledgeBases: {
      settings: {
        get: async () => ({ id: 'kb-1', name: 'Wiki库', type: 'document', indexing_strategy: { wiki_enabled: true }, chunking_config: {} }),
        parserEngines: async () => ({ data: [{ Name: 'builtin', FileTypes: ['pdf', 'md'], Available: true }] }),
      },
      list: async () => [{ id: 'kb-1', name: 'Wiki库' }],
    },
    // me=null keeps canContribute at the caller's prop (probe short-circuit).
    auth: { me: async () => null },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => [] } } },
    knowledge: { documents: { get: async () => ({}) } },
    created,
  } as unknown as WeKnoraClient & { created: Array<Record<string, unknown>> };
}

async function mountCreateForm(client: WeKnoraClient) {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => {
    root?.render(<WikiPage client={client} knowledgeBaseId="kb-1" canContribute />);
  });
  await act(async () => {});
  const newPageBtn = document.body.querySelector<HTMLButtonElement>('button[aria-label="新建页面"]');
  assert.ok(newPageBtn, '新建页面 entry button present for contributors');
  await act(async () => newPageBtn.click());
  await act(async () => {});
  // R486 P3-1: Vue renders creation as a t-dialog (WikiBrowser.vue L734) — the
  // React port rides the raw tdesign Dialog since parity batch 2 (panel
  // px-kb-wiki-tab-wiki-newpage 76.154%→0, run auto-scan/2026-09-24T08-41-39):
  // the DOM is the literal .t-dialog__wrap tree, so the t-dialog shape is
  // asserted on the tdesign classes instead of the retired WkDialog
  // ([role=dialog]/aria-modal/.wk-dialog-close never existed on raw t-dialog).
  const dialog = document.body.querySelector('.t-dialog__wrap');
  assert.ok(dialog, 'create form opens as a modal dialog (Vue t-dialog shape)');
  assert.ok(dialogVisibleCard(), 'the dialog card is shown (tdesign hides via display:none, it does not unmount)');
  const form = dialog.querySelector('.wiki-create-page-form');
  assert.ok(form, 'create form inside the dialog');
  return { container, dialog: dialog as HTMLElement, form: form as HTMLElement };
}

/** tdesign Dialog 关闭后保留 .t-dialog__wrap 骨架（卡片 display:none）——判定
 * 「弹窗开着」要看没有 display:none 的 .t-dialog 卡片。注意：不能用
 * `:not([style*=…])` 复合选择器——jsdom 的 nwsapi 对它在稍大的子树上会
 * 病态回溯直接卡死（t6 实测），改成简单选择器 + JS 过滤。 */
function dialogVisibleCard(): HTMLElement | null {
  const cards = document.body.querySelectorAll<HTMLElement>('.t-dialog__wrap .t-dialog');
  for (const card of cards) {
    if (!(card.getAttribute('style') ?? '').includes('display: none')) return card;
  }
  return null;
}

/** tdesign Dialog 关闭后走 zoom-exit 过渡（jsdom 无 transitionend，靠兜底
 * 定时器收尾）。不等过渡完成就 unmount 会让 act 永远等不到静止——关闭弹窗的
 * 测试在收尾前先等 ~350ms 让退场过渡落地。 */
async function settleDialogExit() {
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 350)); });
}

/** tdesign footer 按文案取按钮（取消在前、确认在后，tdesign 固定顺序）。 */
function footerButton(dialog: Element, text: string): HTMLButtonElement {
  const button = [...dialog.querySelectorAll('.t-dialog__footer button')].find((b) => b.textContent?.trim() === text);
  assert.ok(button, `${text} button in the t-dialog footer`);
  return button as HTMLButtonElement;
}

function fieldControl(form: Element, labelText: string): HTMLInputElement | HTMLTextAreaElement | null {
  // Vue 平移后的字段骨架：.wiki-create-page-field = label + tdesign 控件兄弟节点。
  const label = [...form.querySelectorAll('label')].find((candidate) => candidate.textContent?.trim().startsWith(labelText));
  const field = label?.parentElement;
  if (!field) return null;
  return field.querySelector('input, textarea');
}

/** tdesign Select 交互（children-options 形态，WikiPage TdSelect.Option 同款，
 * 实测需 inner .t-input 上的 mousedown+click 才开层；options-prop 形态才是
 * wrap click 即开——见 TenantMembersPanel.test）。 */
async function pickSelectOption(scope: Element, optionText: string) {
  await openSelectPopup(scope);
  const options = () => Array.from(document.body.querySelectorAll<HTMLElement>('.t-select-option'));
  const target = options().find((el) => (el.textContent ?? '').trim() === optionText);
  assert.ok(target, `option missing: ${optionText} (have ${options().map((el) => el.textContent).join(',')})`);
  await act(async () => { target.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
}

/** 打开 tdesign Select 弹层（children-options 形态）。 */
async function openSelectPopup(scope: Element) {
  const inner = scope.querySelector('.t-select__wrap .t-input') as HTMLElement | null;
  assert.ok(inner, '页面类型 t-select trigger present');
  await act(async () => {
    inner.dispatchEvent(new dom.window.MouseEvent('mousedown', { bubbles: true, cancelable: true }));
    inner.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true }));
  });
  // tdesign Popup 的开层经 rAF/异步过渡，0ms tick 不稳定（submit 流实测需 ~50ms）。
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
}

// ─── R484 B1-6: create-form field set parity (Vue WikiBrowser.vue t-dialog L734-764) ───
//
// Vue 新建 Wiki 页面 dialog: 标题 / Slug（页面地址）+ hint / 页面类型
// (concept|entity|synthesis|comparison, default concept) / 正文（可选）with
// cancel + confirm buttons; the payload posts { slug, title, page_type,
// content } — no summary field (that one belongs to the edit form).

// ─── R486 P3-1: the create flow renders as a Dialog, mirroring the Vue
// t-dialog「新建 Wiki 页面」(WikiBrowser.vue L733-765) instead of the inline
// wk-wiki-editor form (which belongs to page EDIT mode only). ───

test('新建页面 opens a modal dialog titled 新建 Wiki 页面, leaving the inline editor to edit mode', async () => {
  const { dialog, form } = await mountCreateForm(wikiClient());
  assert.ok(dialog.querySelector('.t-dialog'), 'tdesign modal dialog card rendered (t-dialog shape)');
  assert.equal(dialog.querySelector('.t-dialog__header-content')?.textContent, '新建 Wiki 页面', 'dialog header uses wikiBrowser.newPageTitle');
  // The create form lives in the dialog, not in the inline wk-wiki-editor.
  assert.equal(form.className.includes('wk-wiki-editor'), false, 'create form is not the inline editor');
  const inlineEditor = document.body.querySelector('form.wk-wiki-editor');
  assert.ok(
    !inlineEditor || (inlineEditor.getAttribute('style') ?? '').includes('display: none'),
    'inline wk-wiki-editor stays hidden while the create dialog is open',
  );
  // Vue t-dialog close affordance: the header × closes without posting.
  const closeBtn = dialog.querySelector<HTMLElement>('.t-dialog__close');
  assert.ok(closeBtn, 'dialog header close (×) present');
});

test('create form mirrors the Vue dialog field set: 标题/Slug+hint/页面类型/正文（可选）, no summary', async () => {
  const { form } = await mountCreateForm(wikiClient());
  // Direct text nodes only — the slug hint span and the select options are
  // child elements, not part of the label text.
  const labels = [...form.querySelectorAll('label')].map((label) =>
    [...label.childNodes].filter((node) => node.nodeType === 3).map((node) => node.textContent).join('').trim());

  // Field ORDER matches the Vue dialog (title → slug → type → content).
  assert.deepEqual(
    labels.filter((text) => text && !text.includes('取消') && !text.includes('确认')),
    ['标题', 'Slug（页面地址）', '页面类型', '正文（可选）'],
    `field label set/order: ${JSON.stringify(labels)}`,
  );

  // The R> 一句话摘要 field does not belong to the create form.
  assert.equal(labels.some((text) => (text ?? '').includes('一句话摘要')), false, 'create form must not render the edit-mode summary field');

  // Placeholders + the slug help line mirror the Vue dialog copy.
  assert.equal((fieldControl(form, '标题') as HTMLInputElement)?.placeholder, '请输入页面标题');
  assert.equal((fieldControl(form, 'Slug') as HTMLInputElement)?.placeholder, '例如 concept/my-topic');
  assert.match(form.textContent ?? '', /由字母\/数字\/中划线组成，可用 \/ 分层，创建后不可修改/, 'slug help hint present');
  assert.equal((fieldControl(form, '正文') as HTMLTextAreaElement)?.placeholder, 'Markdown 正文，支持 [[页面链接]] 语法');

  // Content is optional in Vue (newPageContentLabel 正文（可选）): no required attr.
  assert.equal((fieldControl(form, '正文') as HTMLTextAreaElement)?.required, false, '正文 is optional like Vue');
});

test('create form type select offers the four Vue page types with 概念 default', async () => {
  const { form } = await mountCreateForm(wikiClient());
  const trigger = fieldControl(form, '页面类型') as HTMLInputElement | null;
  assert.ok(trigger, '页面类型 t-select trigger present');
  assert.equal(trigger.value, '概念', 'concept (概念) is the Vue default shown in the trigger');
  // 打开弹层枚举选项（值→文案映射由 submit 测试的 page_type=entity 断言覆盖）。
  await openSelectPopup(form);
  const options = [...document.body.querySelectorAll<HTMLElement>('.t-select-option')].map((option) => (option.textContent ?? '').trim());
  assert.deepEqual(options, ['概念', '实体', '综合', '对比'], 'four Vue page types in order');
});





test('create form title input auto-syncs the slug as <type>/<slugified-title> until touched', async () => {
  const { form } = await mountCreateForm(wikiClient());
  const title = fieldControl(form, '标题') as HTMLInputElement;
  const slug = fieldControl(form, 'Slug') as HTMLInputElement;

  await act(async () => setInputValue(title, 'My Topic!'));
  assert.equal(slug.value, 'concept/my-topic', 'slug derives from the type prefix + slugified title');

  // Once the user edits the slug, later title edits stop overriding it.
  await act(async () => setInputValue(slug, 'concept/custom'));
  await act(async () => setInputValue(title, 'Another Title'));
  assert.equal(slug.value, 'concept/custom', 'touched slug stays manual (Vue createPageSlugTouched)');

  // A fresh title edit before touching keeps syncing with the ASCII filter.
  const client2 = wikiClient();
  const second = await mountCreateForm(client2);
  const title2 = fieldControl(second.form, '标题') as HTMLInputElement;
  await act(async () => setInputValue(title2, 'Hello--World 2026'));
  assert.equal((fieldControl(second.form, 'Slug') as HTMLInputElement).value, 'concept/hello-world-2026', 'repeated dashes collapse and edges trim');
});

test('create submit posts the Vue payload {slug,title,page_type,content} without summary', async () => {
  const client = wikiClient();
  const { dialog, form } = await mountCreateForm(client);
  // 先开 select 弹层选实体（concept → entity，page_type 值映射），再填字段。
  await pickSelectOption(form, '实体');
  await act(async () => setInputValue(fieldControl(form, '标题') as HTMLInputElement, '架构决策'));
  await act(async () => setInputValue(fieldControl(form, 'Slug') as HTMLInputElement, 'concept/adr'));
  await act(async () => setInputValue(fieldControl(form, '正文') as HTMLTextAreaElement, '# 正文内容'));

  const confirm = footerButton(dialog, '确认');
  await act(async () => confirm.click());
  await act(async () => {});
  await settleDialogExit();

  assert.equal(client.created.length, 1, 'one create call');
  assert.deepEqual(Object.keys(client.created[0]!).sort(), ['content', 'page_type', 'slug', 'title'], `payload keys: ${JSON.stringify(client.created[0])}`);
  assert.equal(client.created[0]!.page_type, 'entity');
  assert.equal(client.created[0]!.slug, 'concept/adr');
  assert.equal(client.created[0]!.title, '架构决策');
  assert.equal(client.created[0]!.content, '# 正文内容');

  // Vue submitCreatePage closes the dialog on success (WikiBrowser.vue L3052).
  assert.equal(dialogVisibleCard(), null, 'dialog closes after a successful create');
});

test('create form allows an empty 正文 (Vue optional content) and blocks empty title/slug', async () => {
  const client = wikiClient();
  const { dialog, form } = await mountCreateForm(client);
  const confirm = footerButton(dialog, '确认');

  // Empty title + slug → the Vue newPageMissingFields warning, no request.
  await act(async () => confirm.click());
  await act(async () => {});
  assert.equal(client.created.length, 0, 'nothing posted with empty title/slug');
  const dialogAfterInvalid = dialogVisibleCard();
  assert.ok(dialogAfterInvalid, 'dialog stays open on validation failure so the user can fix the fields');
  assert.match(dialogAfterInvalid.textContent ?? '', /请填写标题和 Slug/, 'Vue newPageMissingFields warning shown');

  // Title+slug with EMPTY content still creates (正文 is optional).
  await act(async () => setInputValue(fieldControl(form, '标题') as HTMLInputElement, '空正文页'));
  await act(async () => setInputValue(fieldControl(form, 'Slug') as HTMLInputElement, 'concept/empty-body'));
  await act(async () => confirm.click());
  await act(async () => {});
  await settleDialogExit();
  assert.equal(client.created.length, 1, 'empty content is submittable like Vue');
  assert.equal(client.created[0]!.content, '');
  assert.equal(dialogVisibleCard(), null, 'dialog closes after the successful create');
});

test('create form pairs 确认 with a 取消 cancel that closes the dialog without posting', async () => {
  const client = wikiClient();
  const { dialog, form } = await mountCreateForm(client);
  const cancel = footerButton(dialog, '取消');
  await act(async () => setInputValue(fieldControl(form, '标题') as HTMLInputElement, '草稿'));
  await act(async () => cancel.click());
  await act(async () => {});
  assert.equal(client.created.length, 0, 'cancel never posts');
  await settleDialogExit();
  // Vue t-dialog cancel just sets visible=false: the dialog disappears and
  // the draft is dropped (the form is re-seeded on the next open).
  assert.equal(dialogVisibleCard(), null, 'cancel closes the create dialog');
  const inlineEditor = document.body.querySelector('form.wk-wiki-editor');
  assert.ok(!inlineEditor || (inlineEditor.getAttribute('style') ?? '').includes('display: none'), 'cancel does not fall back into the inline editor');
});

test('dialog × close drops the draft like the Vue t-dialog cancel', async () => {
  const client = wikiClient();
  const { dialog } = await mountCreateForm(client);
  await act(async () => setInputValue(dialog.querySelector('.wiki-create-page-form input') as HTMLInputElement, '草稿'));
  const closeBtn = dialog.querySelector<HTMLElement>('.t-dialog__close');
  assert.ok(closeBtn);
  await act(async () => closeBtn.click());
  await act(async () => {});
  assert.equal(client.created.length, 0, 'header close never posts');
  await settleDialogExit();
  assert.equal(dialogVisibleCard(), null, 'header close closes the create dialog');
});
