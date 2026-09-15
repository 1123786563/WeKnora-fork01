import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
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
const { KnowledgeDocumentsPage, DocumentCardGrid, documentCardHoverPosition, documentStatus, documentSourceLabel, documentFileSizeLabel, folderPathCrumbs, hasDocumentGridContent, isStorageEngineMissing, canUploadKnowledgeDocuments } = await import('./KnowledgeDocumentsPage.tsx');
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

test('storage engine availability follows the Vue upload gate', () => {
  assert.equal(isStorageEngineMissing({ type: 'document' }), true, 'a document KB without either storage binding is blocked');
  assert.equal(isStorageEngineMissing({ type: 'document', storage_backend_id: 'storage-1' }), false, 'the authoritative storage backend binding enables uploads');
  assert.equal(isStorageEngineMissing({ type: 'document', storage_provider_config: { provider: 's3' } }), false, 'the legacy provider projection remains compatible');
  assert.equal(isStorageEngineMissing({ type: 'faq' }), false, 'FAQ KBs do not use the documents upload gate');
});

test('upload permission honors the Vue shared editor grant but denies shared viewers', () => {
  const me = { user: { id: 'viewer-in-home' }, memberships: [{ role: 'viewer' }] };
  assert.equal(canUploadKnowledgeDocuments({ id: 'kb-1', my_permission: 'editor' }, me), true);
  assert.equal(canUploadKnowledgeDocuments({ id: 'kb-1', my_permission: 'viewer' }, me), false);
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

test('upload entry stays hidden until the Vue capability resolves; legacy form is gone', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentsPage, {
    client: {} as never,
    knowledgeBaseId: kbId,
  }));
  assert.ok(!html.includes('添加文档'), 'unknown capability does not expose upload');
  assert.ok(!html.includes('wk-upload-panel'), 'legacy 来源/文件/上传文件 form removed');
});

test('document grid cards keep the Vue 240px/136px anatomy, footer metadata, and completed content', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentCardGrid, {
    items: [{ id: 'doc-1', file_name: 'guide.pdf', file_type: 'pdf', parse_status: 'completed', folder_path: 'Guides', description: 'A short guide', updated_at: '2026-09-15T13:36:00Z', tags: [{ id: 'tag-1', name: 'Release notes' }] }],
    folders: [{ path: 'Specs', name: 'Specs', total_count: 2 }],
    selected: new Set<string>(['doc-1']),
    batchMode: false,
    canContribute: false,
    canDownload: false,
    t,
    onOpen: noop,
    onOpenFolder: noop,
    onToggle: noop,
    onTagEdit: noop,
    onReparse: noop,
    onCancelParse: noop,
    onDownload: noop,
    onEdit: noop,
    onViewTrace: noop,
    onMove: noop,
    onBatchManage: noop,
    onDelete: noop,
  }));
  assert.match(html, /grid grid-cols-\[repeat\(auto-fill,minmax\(240px,1fr\)\)\]/);
  assert.match(html, /flex h-\[136px\] min-w-\[240px\] flex-col/);
  assert.ok(html.includes('border-t border-line-soft'), 'card footer has the Vue separator');
  assert.ok(html.includes('A short guide'), 'completed cards render their description in the content area');
  assert.ok(!html.includes('已完成'), 'Vue completed cards keep the title row clear and put the description in the content area');
  assert.ok(html.includes('26-09-15 21:36'), 'updated time remains in the footer');
  assert.ok(html.includes('PDF'), 'file type remains in the footer');
  assert.ok(html.includes('Release notes'), 'footer preserves the Vue tag metadata chips');
  assert.match(html, /knowledge-card[^\"]*is-selected/, 'Vue selected cards retain their selected visual state');
  assert.ok(!html.includes('选择 guide.pdf'), 'read-only cards do not expose the Vue canEdit-only checkbox');
  assert.match(html, /knowledge-card[^\"]*cursor-pointer/, 'Vue cards open from the whole card surface');
});

test('document list keeps the Vue column order with actions after updated time', () => {
  const source = readFileSync(new URL('./KnowledgeDocumentsPage.tsx', import.meta.url), 'utf8');
  const updated = source.indexOf('formatDocumentTime(document.updated_at ?? document.created_at)');
  const actions = source.indexOf('className="wk-row-actions', updated);
  assert.ok(updated >= 0 && actions > updated, 'updated time must render before the trailing action column');
});

test('document statuses preserve Vue cancelled warning and pending summary copy', () => {
  assert.equal(documentStatus({ id: 'cancelled', parse_status: 'cancelled' } as never, t).tone, 'warning');
  assert.equal(documentStatus({ id: 'summary', parse_status: 'completed', summary_status: 'pending' } as never, t).label, t('knowledgeBase.generatingSummary'));
});

test('document list metadata uses Vue source labels and human file sizes', () => {
  assert.equal(documentSourceLabel({ channel: 'feishu' } as never, t), t('knowledgeBase.channelFeishu'));
  assert.equal(documentSourceLabel({ type: 'manual' } as never, t), t('knowledgeBase.channelManual'));
  assert.equal(documentSourceLabel({ source: 'file' } as never, t), t('knowledgeBase.channelUpload'));
  assert.equal(documentFileSizeLabel(1024), '1 KB');
  assert.equal(documentFileSizeLabel(1024 * 1024), '1 MB');
  assert.equal(documentFileSizeLabel(undefined), '--');
});

test('document in-flight cards use Vue parsing/finalizing copy instead of filter labels', () => {
  assert.equal(documentStatus({ id: 'pending', parse_status: 'pending' } as never, t).label, t('knowledgeBase.parsingInProgress'));
  assert.equal(documentStatus({ id: 'processing', parse_status: 'processing' } as never, t).label, t('knowledgeBase.parsingInProgress'));
  assert.equal(documentStatus({ id: 'finalizing', parse_status: 'finalizing' } as never, t).label, t('knowledgeBase.parseStatusFinalizing'));
  assert.equal(documentStatus({ id: 'finalizing-summary', parse_status: 'finalizing', summary_status: 'processing' } as never, t).label, t('knowledgeBase.generatingSummary'));
});

test('document cards show the Vue summary-generating copy for pending summaries', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentCardGrid, {
    items: [{ id: 'summary', file_name: 'guide.pdf', file_type: 'pdf', parse_status: 'completed', summary_status: 'pending' }],
    folders: [], selected: new Set<string>(), batchMode: false, canContribute: false, canDownload: false, t,
    onOpen: noop, onOpenFolder: noop, onToggle: noop, onTagEdit: noop, onReparse: noop,
    onCancelParse: noop, onDownload: noop, onEdit: noop, onViewTrace: noop, onMove: noop,
    onBatchManage: noop, onDelete: noop,
  }));
  assert.match(html, /生成摘要中/);
});

test('failed document cards reserve the content row for the Vue failure and trace affordance', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentCardGrid, {
    items: [{ id: 'failed', file_name: 'broken.pdf', file_type: 'pdf', parse_status: 'failed', description: 'do not show this' }],
    folders: [], selected: new Set<string>(), batchMode: false, canContribute: true, canDownload: false, t,
    onOpen: noop, onOpenFolder: noop, onToggle: noop, onTagEdit: noop, onReparse: noop,
    onCancelParse: noop, onDownload: noop, onEdit: noop, onViewTrace: noop, onMove: noop,
    onBatchManage: noop, onDelete: noop,
  }));
  assert.match(html, /解析失败/, 'failure is visible in the card content rather than a title-row badge');
  assert.match(html, /查看 Trace/, 'failed card retains the trace action');
  assert.ok(!html.includes('do not show this'), 'failure state replaces the completed-only description');
});

test('in-flight document cards expose Vue-style spinner and trace action', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentCardGrid, {
    items: [{ id: 'processing', file_name: 'guide.pdf', file_type: 'pdf', parse_status: 'processing' }],
    folders: [], selected: new Set<string>(), batchMode: false, canContribute: true, canDownload: false, t,
    onOpen: noop, onOpenFolder: noop, onToggle: noop, onTagEdit: noop, onReparse: noop,
    onCancelParse: noop, onDownload: noop, onEdit: noop, onViewTrace: noop, onMove: noop,
    onBatchManage: noop, onDelete: noop,
  }));
  assert.match(html, /animate-spin/, 'in-flight card keeps a visible loading affordance');
  assert.match(html, /解析中/, 'in-flight card uses the Vue card copy');
  assert.match(html, /查看 Trace/, 'trace action remains available');
});

test('editable document cards expose the Vue action-menu mutation entries', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentCardGrid, {
    items: [
      { id: 'doc-1', file_name: 'guide.md', file_type: 'md', source: 'manual', parse_status: 'processing', trace: { name: 'root' } },
      { id: 'doc-2', file_name: 'source.pdf', file_type: 'pdf', source: 'file', parse_status: 'completed' },
    ],
    folders: [],
    selected: new Set<string>(),
    batchMode: true,
    canContribute: true,
    canDownload: true,
    t,
    onOpen: noop,
    onOpenFolder: noop,
    onToggle: noop,
    onTagEdit: noop,
    onReparse: noop,
    onCancelParse: noop,
    onDownload: noop,
    onEdit: noop,
    onViewTrace: noop,
    onMove: noop,
    onBatchManage: noop,
    onDelete: noop,
  }));
  assert.ok(html.includes('aria-haspopup="menu"'), 'card has an accessible action-menu trigger');
  assert.ok(html.includes('编辑文档'), 'edit action is present for manual documents');
  assert.ok(html.includes('解析进度'), 'trace action is present while parsing');
  assert.ok(html.includes('下载 source.pdf'), 'download action is present for file documents');
  assert.ok(html.includes('移动到目录'), 'folder move action is present');
  assert.ok(html.includes('批量管理'), 'batch management action is present');
  assert.ok(html.includes('选择 guide.md'), 'selection checkboxes appear only after entering batch mode');
  assert.ok(html.includes('删除文档'), 'delete action is present');
});

test('document card hover placement prefers the right side and falls back within the viewport', () => {
  assert.deepEqual(documentCardHoverPosition({ left: 100, right: 300, top: 40 }, { width: 1000, height: 800 }), { x: 312, y: 40 });
  assert.deepEqual(documentCardHoverPosition({ left: 700, right: 900, top: 40 }, { width: 1000, height: 800 }), { x: 328, y: 40 });
  assert.deepEqual(documentCardHoverPosition({ left: 250, right: 350, top: 450, bottom: 586 }, { width: 600, height: 800 }), { x: 230, y: 138 });
});

test('document grid remains mounted for a directory containing only child folders', () => {
  assert.equal(hasDocumentGridContent([], [{ path: 'Specs/API' }]), true);
  assert.equal(hasDocumentGridContent([], []), false);
});
