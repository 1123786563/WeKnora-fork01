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
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  PointerEvent: dom.window.PointerEvent,
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
  // R486 P3-1: Vue renders creation as a t-dialog (WikiBrowser.vue L734), not
  // an inline editor — the React port must open a modal dialog instead.
  const dialog = document.body.querySelector('[role="dialog"]');
  assert.ok(dialog, 'create form opens as a modal dialog (Vue t-dialog shape)');
  const form = dialog.querySelector('form');
  assert.ok(form, 'create form inside the dialog');
  return { container, dialog: dialog as HTMLElement, form: form as HTMLFormElement };
}

function fieldInput(form: HTMLFormElement, labelText: string): HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null {
  const labels = [...form.querySelectorAll('label')];
  const label = labels.find((candidate) => candidate.textContent?.trim().startsWith(labelText));
  if (!label) return null;
  const control = label.querySelector('input, textarea, select');
  return control as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null;
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
  assert.equal(dialog.getAttribute('aria-modal'), 'true', 'dialog is modal like t-dialog');
  assert.equal(dialog.querySelector('h2')?.textContent, '新建 Wiki 页面', 'dialog header uses wikiBrowser.newPageTitle');
  // The create form lives in the dialog, not in the inline wk-wiki-editor.
  assert.equal(form.className.includes('wk-wiki-editor'), false, 'create form is not the inline editor');
  const inlineEditor = document.body.querySelector('form.wk-wiki-editor');
  assert.ok(
    !inlineEditor || (inlineEditor.getAttribute('style') ?? '').includes('display: none'),
    'inline wk-wiki-editor stays hidden while the create dialog is open',
  );
  // Vue t-dialog close affordance: the header × closes without posting.
  const closeBtn = dialog.querySelector<HTMLButtonElement>('button.wk-dialog-close');
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
  assert.equal((fieldInput(form, '标题') as HTMLInputElement)?.placeholder, '请输入页面标题');
  assert.equal((fieldInput(form, 'Slug') as HTMLInputElement)?.placeholder, '例如 concept/my-topic');
  assert.match(form.textContent ?? '', /由字母\/数字\/中划线组成，可用 \/ 分层，创建后不可修改/, 'slug help hint present');
  assert.equal((fieldInput(form, '正文') as HTMLTextAreaElement)?.placeholder, 'Markdown 正文，支持 [[页面链接]] 语法');

  // Content is optional in Vue (newPageContentLabel 正文（可选）): no required attr.
  assert.equal((fieldInput(form, '正文') as HTMLTextAreaElement)?.required, false, '正文 is optional like Vue');
});

test('create form type select offers the four Vue page types with 概念 default', async () => {
  const { form } = await mountCreateForm(wikiClient());
  const typeSelect = fieldInput(form, '页面类型');
  assert.ok(typeSelect, '页面类型 select present');
  assert.equal(typeSelect.tagName, 'SELECT');
  const options = [...(typeSelect as HTMLSelectElement).options].map((option) => option.textContent?.trim());
  assert.deepEqual(options, ['概念', '实体', '综合', '对比']);
  assert.deepEqual([...(typeSelect as HTMLSelectElement).options].map((option) => option.value), ['concept', 'entity', 'synthesis', 'comparison']);
  assert.equal((typeSelect as HTMLSelectElement).value, 'concept', 'concept is the Vue default');
});

test('create form title input auto-syncs the slug as <type>/<slugified-title> until touched', async () => {
  const { form } = await mountCreateForm(wikiClient());
  const title = fieldInput(form, '标题') as HTMLInputElement;
  const slug = fieldInput(form, 'Slug') as HTMLInputElement;

  await act(async () => setInputValue(title, 'My Topic!'));
  assert.equal(slug.value, 'concept/my-topic', 'slug derives from the type prefix + slugified title');

  // Once the user edits the slug, later title edits stop overriding it.
  await act(async () => setInputValue(slug, 'concept/custom'));
  await act(async () => setInputValue(title, 'Another Title'));
  assert.equal(slug.value, 'concept/custom', 'touched slug stays manual (Vue createPageSlugTouched)');

  // A fresh title edit before touching keeps syncing with the ASCII filter.
  const client2 = wikiClient();
  const second = await mountCreateForm(client2);
  const title2 = fieldInput(second.form, '标题') as HTMLInputElement;
  await act(async () => setInputValue(title2, 'Hello--World 2026'));
  assert.equal((fieldInput(second.form, 'Slug') as HTMLInputElement).value, 'concept/hello-world-2026', 'repeated dashes collapse and edges trim');
});

test('create submit posts the Vue payload {slug,title,page_type,content} without summary', async () => {
  const client = wikiClient();
  const { form } = await mountCreateForm(client);
  await act(async () => setInputValue(fieldInput(form, '标题') as HTMLInputElement, '架构决策'));
  await act(async () => setInputValue(fieldInput(form, 'Slug') as HTMLInputElement, 'concept/adr'));
  const select = fieldInput(form, '页面类型') as HTMLSelectElement;
  await act(async () => {
    select.value = 'entity';
    select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
  await act(async () => setInputValue(fieldInput(form, '正文') as HTMLTextAreaElement, '# 正文内容'));

  const confirm = [...form.querySelectorAll('button')].find((button) => button.textContent?.trim() === '确认');
  assert.ok(confirm, 'confirm button uses common.confirm 确认');
  await act(async () => confirm.click());
  await act(async () => {});

  assert.equal(client.created.length, 1, 'one create call');
  assert.deepEqual(Object.keys(client.created[0]!).sort(), ['content', 'page_type', 'slug', 'title'], `payload keys: ${JSON.stringify(client.created[0])}`);
  assert.equal(client.created[0]!.page_type, 'entity');
  assert.equal(client.created[0]!.slug, 'concept/adr');
  assert.equal(client.created[0]!.title, '架构决策');
  assert.equal(client.created[0]!.content, '# 正文内容');

  // Vue submitCreatePage closes the dialog on success (WikiBrowser.vue L3052).
  assert.equal(document.body.querySelector('[role="dialog"]'), null, 'dialog closes after a successful create');
});

test('create form allows an empty 正文 (Vue optional content) and blocks empty title/slug', async () => {
  const client = wikiClient();
  const { form } = await mountCreateForm(client);
  const confirm = [...form.querySelectorAll('button')].find((button) => button.textContent?.trim() === '确认');
  assert.ok(confirm);

  // Empty title + slug → the Vue newPageMissingFields warning, no request.
  await act(async () => confirm.click());
  await act(async () => {});
  assert.equal(client.created.length, 0, 'nothing posted with empty title/slug');
  const dialogAfterInvalid = document.body.querySelector('[role="dialog"]');
  assert.ok(dialogAfterInvalid, 'dialog stays open on validation failure so the user can fix the fields');
  assert.match(dialogAfterInvalid.textContent ?? '', /请填写标题和 Slug/, 'Vue newPageMissingFields warning shown');

  // Title+slug with EMPTY content still creates (正文 is optional).
  await act(async () => setInputValue(fieldInput(form, '标题') as HTMLInputElement, '空正文页'));
  await act(async () => setInputValue(fieldInput(form, 'Slug') as HTMLInputElement, 'concept/empty-body'));
  await act(async () => confirm.click());
  await act(async () => {});
  assert.equal(client.created.length, 1, 'empty content is submittable like Vue');
  assert.equal(client.created[0]!.content, '');
  assert.equal(document.body.querySelector('[role="dialog"]'), null, 'dialog closes after the successful create');
});

test('create form pairs 确认 with a 取消 cancel that closes the dialog without posting', async () => {
  const client = wikiClient();
  const { form } = await mountCreateForm(client);
  const cancel = [...form.querySelectorAll('button')].find((button) => button.textContent?.trim() === '取消');
  assert.ok(cancel, 'cancel button uses common.cancel 取消');
  await act(async () => setInputValue(fieldInput(form, '标题') as HTMLInputElement, '草稿'));
  await act(async () => cancel.click());
  await act(async () => {});
  assert.equal(client.created.length, 0, 'cancel never posts');
  // Vue t-dialog cancel just sets visible=false: the dialog disappears and
  // the draft is dropped (the form is re-seeded on the next open).
  assert.equal(document.body.querySelector('[role="dialog"]'), null, 'cancel closes the create dialog');
  const inlineEditor = document.body.querySelector('form.wk-wiki-editor');
  assert.ok(!inlineEditor || (inlineEditor.getAttribute('style') ?? '').includes('display: none'), 'cancel does not fall back into the inline editor');
});

test('dialog × close drops the draft like the Vue t-dialog cancel', async () => {
  const client = wikiClient();
  const { dialog } = await mountCreateForm(client);
  await act(async () => setInputValue(dialog.querySelector('form input') as HTMLInputElement, '草稿'));
  const closeBtn = dialog.querySelector<HTMLButtonElement>('button.wk-dialog-close');
  assert.ok(closeBtn);
  await act(async () => closeBtn.click());
  await act(async () => {});
  assert.equal(client.created.length, 0, 'header close never posts');
  assert.equal(document.body.querySelector('[role="dialog"]'), null, 'header close closes the create dialog');
});
