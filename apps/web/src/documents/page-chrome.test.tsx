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
  DOCUMENT_FILE_TYPE_OPTIONS,
} = await import('./DocumentsPageChrome.tsx');
const { KnowledgeDocumentsPage, DocumentCardGrid, documentCardHoverPosition, documentStatus, documentSourceLabel, documentFileSizeLabel, folderPathCrumbs, hasDocumentGridContent, isStorageEngineMissing, canUploadKnowledgeDocuments, canDownloadKnowledgeDocuments, canMutateKnowledgeDocuments } = await import('./KnowledgeDocumentsPage.tsx');
const { createTranslator } = await import('../i18n.ts');
const { formatMessage, supportedLocales } = await import('@weknora/i18n');

const t = createTranslator('zh-CN');
const kbId = '9727d104-cde4-4d03-879f-d7e3897b69a3';
const noop = () => {};

// R483 F2 (R482 B1 差异1): the KB documents file-type filter's manual option
// must read knowledgeBase.typeManual (Vue KnowledgeBase.vue:635
// fileTypeOptions ->「手动创建」), not upload.onlineEdit — that key is the
// add-document dropdown's manual entry (KbUploadSourceDropdown.vue:124
// 「在线编辑」) and the two must not be conflated.
test('file type filter manual option uses the Vue typeManual label', () => {
  const manual = DOCUMENT_FILE_TYPE_OPTIONS.find((option) => option.value === 'manual');
  assert.equal(manual?.labelKey, 'knowledgeBase.typeManual');
  assert.equal(t('knowledgeBase.typeManual'), '手动创建');
  assert.notEqual(t('knowledgeBase.typeManual'), t('upload.onlineEdit'));
});

test('knowledgeBase.typeManual stays present and non-empty in every locale', () => {
  for (const locale of supportedLocales) {
    const label = formatMessage(locale, 'knowledgeBase.typeManual');
    assert.ok(label && label !== 'knowledgeBase.typeManual', `${locale} lacks knowledgeBase.typeManual`);
  }
});

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

test('upload permission keeps Vue creator precedence over a stale viewer projection', () => {
  assert.equal(
    canUploadKnowledgeDocuments(
      { id: 'kb-1', creator_id: 'creator-1', my_permission: 'viewer' },
      { user: { id: 'creator-1', role: 'viewer' } },
    ),
    true,
  );
});

test('document permissions keep Vue download and mutation gates separate from upload access', () => {
  const contributor = { user: { id: 'contributor' }, memberships: [{ role: 'contributor' }] };
  assert.equal(canDownloadKnowledgeDocuments({ id: 'kb-1' }, contributor), true);
  assert.equal(canMutateKnowledgeDocuments({ id: 'kb-1' }, contributor), true);
  const sharedEditor = { user: { id: 'viewer' }, memberships: [{ role: 'viewer' }] };
  assert.equal(canUploadKnowledgeDocuments({ id: 'kb-1', my_permission: 'editor' }, sharedEditor), true);
  assert.equal(canDownloadKnowledgeDocuments({ id: 'kb-1', my_permission: 'editor' }, sharedEditor), false);
  assert.equal(canMutateKnowledgeDocuments({ id: 'kb-1', my_permission: 'editor' }, sharedEditor), true);
  assert.equal(canDownloadKnowledgeDocuments({ id: 'kb-1', my_permission: 'viewer' }, contributor), false);
  assert.equal(canMutateKnowledgeDocuments({ id: 'kb-1', my_permission: 'viewer' }, contributor), false);
});

test('download gate follows the Vue effectiveKBPermission chain (share grant > my_permission; the row permission field is never consulted)', () => {
  const contributor = { user: { id: 'contributor' }, memberships: [{ role: 'contributor' }] };
  // Vue KnowledgeBase.vue:326 — effectiveKBPermission = orgStore.getKBPermission(kbId)
  // || kbInfo.my_permission || ''. The KB row's `permission` field is not in
  // the chain, so a stray 'viewer' there must NOT block downloads (the
  // R483 live FAIL on Parity KB Demo).
  assert.equal(canDownloadKnowledgeDocuments({ id: 'kb-1', permission: 'viewer' }, contributor, null), true);
  // The org shared-knowledge-bases grant is the authoritative first hop.
  assert.equal(
    canDownloadKnowledgeDocuments(
      { id: 'kb-1', my_permission: 'viewer' },
      contributor,
      [{ knowledge_base: { id: 'kb-1' }, permission: 'editor' }],
    ),
    true,
  );
  // Without a grant the empty chain falls through to "no permission" → allowed.
  assert.equal(canDownloadKnowledgeDocuments({ id: 'kb-1', my_permission: '' }, contributor, null), true);
});

test('hasRole("contributor") follows the Vue ROLE_LEVEL hierarchy — owner outranks contributor (R483 live FAIL fix)', () => {
  // Vue stores/auth.ts hasRole: viewer < contributor < admin < owner, so a
  // workspace owner passes the contributor gate. The parity-test account is
  // memberships[0].role = 'owner' with no user.role — React must not deny.
  const owner = { user: { id: 'owner-1' }, memberships: [{ role: 'owner' }] };
  assert.equal(canDownloadKnowledgeDocuments({ id: 'kb-1', permission: 'viewer' }, owner, null), true);
});

test('documents page wires the Vue download and mutation gates into both views', () => {
  const source = readFileSync(new URL('./KnowledgeDocumentsPage.tsx', import.meta.url), 'utf8');
  assert.match(source, /setCanDownload\(canDownloadKnowledgeDocuments\(kb as KBSurfaceKB/);
  assert.match(source, /setCanMutate\(canMutateKnowledgeDocuments\(kb as KBSurfaceKB/);
  assert.match(source, /canMutateKnowledge=\{canMutate\}/);
  assert.match(source, /canDownload=\{canDownload\}/);
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
  // tdesign 平移：32px 行盒与 20px/600 字号由 documents.td.css 承载
  //（.document-title-row / .document-breadcrumb），模板不再携带 Tailwind。
  assert.ok(html.includes('document-title-row'), 'Vue title row anatomy');
  assert.ok(html.includes('document-breadcrumb'), 'Vue breadcrumb anatomy');
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
  // tdesign Popup 内容懒渲染（打开态才挂载），SSR 静态标记不再含弹层卡片。
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
  assert.ok(html.includes('class="empty"'), 'Vue EmptyKnowledge empty class');
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
  assert.ok(html.includes('doc-type-select'), 'Vue doc-type-select filters (tdesign Select)');
  assert.ok(html.includes('doc-date-range'), 'Vue doc-date-range picker');
  assert.ok(html.includes('doc-filter-bar'), 'Vue filter bar class');
  assert.ok(html.includes('doc-search-field'), 'Vue document search control');
  assert.ok(html.includes('doc-filter-bar__filters'), 'Vue filter row anatomy');
  assert.ok(!html.includes('rounded-card border border-line bg-surface p-4'), 'Vue document area has no extra shadcn Card chrome');
});

test('shared read-only documents page shows no viewer-readonly banner like Vue', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentsPage, {
    client: {} as never,
    knowledgeBaseId: kbId,
  }));
  // Vue (KnowledgeBase.vue document-header) expresses read-only shared access
  // purely by hiding the editing entries — it never renders a banner. The
  // initial canContribute=false projection is the shared-viewer state.
  assert.ok(!html.includes('查看权限'), 'React-invented 查看权限 banner removed');
  assert.ok(!html.includes('编辑操作已隐藏'), 'readonly editing-hidden copy removed');
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
    tagListCount: 1,
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
  assert.ok(html.includes('doc-card-view'), 'Vue DocumentCardView wrapper');
  assert.ok(html.includes('doc-card-list doc-card-list-animated'), 'Vue card grid list');
  assert.match(html, /class="knowledge-card[^"]*is-selected/, 'selected card class');
  assert.ok(html.includes('card-bottom'), 'Vue card footer anatomy');
  assert.ok(html.includes('A short guide'), 'completed cards render their description in the content area');
  assert.ok(!html.includes('已完成'), 'Vue completed cards keep the title row clear and put the description in the content area');
  // Vue: footer 左槽在 folder_path 存在时渲染 card-folder（时间让位）。
  assert.ok(html.includes('card-folder'), 'footer swaps the time cell for the Vue folder chip');
  assert.ok(html.includes('PDF'), 'file type remains in the footer');
  assert.ok(html.includes('Release notes'), 'footer preserves the Vue tag metadata chips');
  assert.match(html, /knowledge-card[^\"]*is-selected/, 'Vue selected cards retain their selected visual state');
  assert.ok(!html.includes('选择 guide.pdf'), 'read-only cards do not expose the Vue canEdit-only checkbox');
});

test('document list keeps the Vue column order with actions after updated time', () => {
  const source = readFileSync(new URL('./KnowledgeDocumentsPage.tsx', import.meta.url), 'utf8');
  const updated = source.indexOf('formatDocumentTime(document.updated_at ?? document.created_at)');
  const actions = source.indexOf('className="cell cell-actions', updated);
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
  assert.match(html, /t-icon-loading/, 'in-flight card keeps the Vue t-icon loading affordance');
  assert.match(html, /解析中/, 'in-flight card uses the Vue card copy');
  assert.match(html, /查看 Trace/, 'trace action remains available');
});

test('editable document cards expose the Vue action-menu mutation entries', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentCardGrid, {
    items: [
      { id: 'doc-1', file_name: 'guide.md', file_type: 'md', type: 'manual', source: 'manual', parse_status: 'processing', trace: { name: 'root' } },
      { id: 'doc-2', file_name: 'source.pdf', file_type: 'pdf', type: 'file', source: 'file', parse_status: 'completed' },
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
  // 菜单内容经 t-popup 懒挂载，静态标记只含触发器；条目级门控由
  // doc-row-menu.test.ts（documentMenuItems 单测）覆盖。
  assert.ok(html.includes('more-wrap'), 'card has the Vue more-wrap action-menu trigger');
});

// R483 F4: Vue DocumentActionMenu gates batch-manage on canMutateKnowledge ||
// canDownload and leaves 删除文档 ungated (the menu itself renders only for
// contributors; the backend enforces the actual delete permission).
test('editor document cards keep edit actions but hide Vue contributor-only mutations', () => {
  const html = renderToStaticMarkup(React.createElement(DocumentCardGrid, {
    items: [{ id: 'manual-1', file_name: 'notes.md', type: 'manual', source: 'manual', file_type: 'md', parse_status: 'completed' }],
    folders: [],
    selected: new Set<string>(),
    batchMode: false,
    canContribute: true,
    canMutateKnowledge: false,
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
  // 菜单项门控断言收敛到 doc-row-menu.test.ts；此处保留触发器级断言。
  assert.ok(html.includes('more-wrap'), 'editor cards keep the action-menu trigger');
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
