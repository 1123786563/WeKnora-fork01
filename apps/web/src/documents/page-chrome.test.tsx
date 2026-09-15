import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register('data:text/javascript,' + encodeURIComponent([
  'export async function resolve(specifier, context, nextResolve) {',
  "  if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };",
  '  return nextResolve(specifier, context);',
  '}',
].join('\n')), import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const {
  DocumentsBreadcrumb,
  ParserHint,
  DocumentEmptyState,
} = await import('./DocumentsPageChrome.tsx');
const { KnowledgeDocumentsPage, DocumentCardGrid, folderPathCrumbs } = await import('./KnowledgeDocumentsPage.tsx');
const { createTranslator } = await import('../i18n.ts');

const t = createTranslator('zh-CN');
const kbId = '9727d104-cde4-4d03-879f-d7e3897b69a3';
const noop = () => {};

test('folder path breadcrumbs preserve Vue root and ancestor paths', () => {
  assert.deepEqual(folderPathCrumbs(undefined), []);
  assert.deepEqual(folderPathCrumbs('Product/Guides'), [
    { name: 'Product', path: 'Product' },
    { name: 'Guides', path: 'Product/Guides' },
  ]);
});

// --- DocumentsBreadcrumb (Vue KnowledgeBase.vue document-title-row parity) -------

test('breadcrumb shows 知识库 › kbName › 文档 and never the raw UUID', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentsBreadcrumb, {
    t,
    knowledgeBaseId: kbId,
    kbName: 'Parity KB Demo',
    kbList: [{ id: kbId, name: 'Parity KB Demo', type: 'document' }],
    canManage: true,
  }));
  assert.ok(html.includes('知识库'), 'renders 知识库 crumb');
  assert.ok(html.includes('Parity KB Demo'), 'renders kb name crumb');
  assert.ok(html.includes('文档'), 'renders 文档 current crumb');
  assert.ok(!html.includes(kbId), 'never exposes the raw KB UUID');
  assert.ok(html.includes('document-breadcrumb'), 'uses the Vue document-breadcrumb anatomy');
  assert.ok(html.includes('breadcrumb-link dropdown'), 'kbName crumb is a switcher dropdown');
  assert.ok(html.includes('breadcrumb-separator'), 'chevron separators present');
  assert.ok(html.includes('document-title-row flex min-h-8'), 'Vue title row keeps a 32px line box');
  assert.ok(html.includes('text-[20px] font-semibold leading-8'), 'Vue breadcrumb uses a 32px line height');
});

test('breadcrumb carries the info popover and settings gear like the FAQ page', () => {
  const managed = renderToStaticMarkup(React.createElement(DocumentsBreadcrumb, {
    t,
    knowledgeBaseId: kbId,
    kbName: 'Parity KB Demo',
    kbMeta: { type: 'document', description: 'demo kb', createdAt: '2026-09-13' },
    supportedFileTypes: ['docx', 'pdf'],
    canManage: true,
  }));
  assert.ok(managed.includes('kb-info-button'), 'info icon button present');
  assert.ok(managed.includes('kb-info-card'), 'info popover card present');
  assert.ok(managed.includes('知识库信息'), 'info card header rendered');
  assert.ok(managed.includes('可上传格式'), 'supported file types row rendered');
  assert.ok(managed.includes('.docx'), 'file type chip rendered');
  assert.ok(managed.includes('kb-settings-button'), 'settings gear button present');

  const viewer = renderToStaticMarkup(React.createElement(DocumentsBreadcrumb, {
    t,
    knowledgeBaseId: kbId,
    kbName: 'Parity KB Demo',
    canManage: false,
  }));
  assert.ok(!viewer.includes('kb-settings-button'), 'gear hidden without manage rights');
});

// --- ParserHint (Vue parser-hint warning line) -----------------------------------

test('parser hint lists unresolved extensions with the 前往配置 link', () => {
  const html = renderToStaticMarkup(React.createElement(ParserHint, {
    t,
    types: ['docm', 'odp', 'ods', 'odt', 'pptm', 'rtf', 'xlsm'],
    onConfigure: noop,
  }));
  assert.ok(html.includes('parser-hint'), 'warning line carries the Vue parser-hint class');
  assert.ok(html.includes('.docm、.odp、.ods、.odt、.pptm、.rtf、.xlsm'), 'extensions joined with 、');
  assert.ok(html.includes('暂无可用解析引擎，上传后将无法解析'), 'Vue warning copy');
  assert.ok(html.includes('前往配置'), 'configure link copy');
  assert.ok(html.includes('→'), 'trailing arrow');
});

test('parser hint renders nothing when every declared type resolves', () => {
  const html = renderToStaticMarkup(React.createElement(ParserHint, { t, types: [], onConfigure: noop }));
  assert.equal(html, '');
});

// --- DocumentEmptyState (Vue EmptyKnowledge + folder-tree variants) ---------------

test('empty state shows the illustration and the Vue drag-drop copy', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentEmptyState, { t, variant: 'illustration' }));
  assert.ok(html.includes('doc-empty-state'), 'Vue empty-state class');
  assert.ok(html.includes('<img'), 'illustration image present');
  assert.ok(html.includes('empty-img'), 'Vue empty-img class');
  assert.ok(html.includes('知识为空，拖放上传'), 'headline copy');
  assert.ok(html.includes('pdf、doc 格式文件，不超过10M'), 'pdf/doc size line');
  assert.ok(html.includes('text、markdown格式文件，不超过200K'), 'text/markdown size line');
});

test('empty state swaps copy for folder and filtered searches like Vue', () => {
  const folder = renderToStaticMarkup(React.createElement(DocumentEmptyState, { t, variant: 'folder' }));
  assert.ok(folder.includes('这个文件夹里还没有文档'));
  const search = renderToStaticMarkup(React.createElement(DocumentEmptyState, { t, variant: 'search' }));
  assert.ok(search.includes('没有匹配的文档'));
  assert.ok(!search.includes('<img'), 'text variants drop the illustration');
});

// --- Page-level chrome (SSR of KnowledgeDocumentsPage) ----------------------------

test('documents page renders Vue chrome instead of the UUID eyebrow', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentsPage, {
    client: {} as never,
    knowledgeBaseId: kbId,
  }));
  assert.ok(!html.includes('wk-eyebrow'), 'React-invented eyebrow removed');
  assert.ok(!html.includes('Knowledge base ·'), 'raw-UUID eyebrow copy removed');
  // The id may only live inside navigation hrefs, never in visible copy.
  let idIndex = html.indexOf(kbId);
  while (idIndex !== -1) {
    const prefix = html.slice(Math.max(0, idIndex - 21), idIndex);
    assert.ok(prefix.endsWith('href="/knowledgeBase/'), 'UUID appears only inside navigation hrefs, never as page copy');
    idIndex = html.indexOf(kbId, idIndex + 1);
  }
  assert.ok(html.includes('document-breadcrumb'), 'breadcrumb replaces the eyebrow');
  assert.ok(html.includes('支持点击或拖拽上传，多格式文档自动解析并智能分块，快速构建可检索的知识库'), 'Vue subtitle copy');
});

test('documents page filter bar matches the Vue anatomy', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentsPage, {
    client: {} as never,
    knowledgeBaseId: kbId,
  }));
  assert.ok(html.includes('搜索文档名称...'), 'full-width Vue search placeholder');
  assert.ok(html.includes('全部标签'), 'tag filter placeholder');
  assert.ok(html.includes('全部类型'), 'file type filter placeholder');
  assert.ok(html.includes('全部状态'), 'parse status filter placeholder');
  assert.ok(html.includes('全部来源'), 'source filter placeholder');
  assert.ok(html.includes('起始时间'), 'date-range start placeholder');
  assert.ok(html.includes('结束时间'), 'date-range end placeholder');
  assert.ok(html.includes('doc-filter-bar'), 'Vue filter bar class');
  assert.ok(html.includes('doc-search-field box-border h-8'), 'Vue document search control is 32px border-box');
  assert.match(html, /doc-filter-bar__filters[^\"]*h-8/, 'Vue filter row is a 32px line box');
  assert.ok(!html.includes('rounded-card border border-line bg-surface p-4'), 'Vue document area has no extra shadcn Card chrome');
});

test('upload entry moves to the Vue add-source dropdown; legacy form is gone', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentsPage, {
    client: {} as never,
    knowledgeBaseId: kbId,
  }));
  assert.ok(html.includes('添加文档'), 'add-document trigger (aria-label/title) present');
  assert.ok(!html.includes('wk-upload-panel'), 'legacy 来源/文件/上传文件 form removed');
});

test('document grid cards keep the Vue 240px/136px anatomy and footer metadata row', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentCardGrid, {
    items: [{ id: 'doc-1', file_name: 'guide.pdf', file_type: 'pdf', parse_status: 'completed', folder_path: 'Guides', description: 'A short guide' }],
    folders: [{ path: 'Specs', name: 'Specs', total_count: 2 }],
    selected: new Set<string>(),
    canContribute: false,
    t,
    onOpen: noop,
    onOpenFolder: noop,
    onToggle: noop,
    onTagEdit: noop,
    onReparse: noop,
    onCancelParse: noop,
  }));
  assert.match(html, /grid grid-cols-\[repeat\(auto-fill,minmax\(240px,1fr\)\)\]/);
  assert.match(html, /flex h-\[136px\] min-w-\[240px\] flex-col/);
  assert.ok(html.includes('border-t border-line-soft'), 'card footer has the Vue separator');
  assert.ok(html.includes('A short guide'), 'completed cards render their description in the content area');
  assert.ok(html.includes('Guides'), 'folder metadata remains in the footer');
});
