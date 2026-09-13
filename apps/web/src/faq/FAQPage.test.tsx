import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register('data:text/javascript,' + encodeURIComponent([
  'export async function resolve(specifier, context, nextResolve) {',
  "  if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };",
  '  return nextResolve(specifier, context);',
  '}',
].join('\n')), import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const { FAQBreadcrumb, FAQPageView, faqKBListPath, faqKBSettingsPath, faqHasMore, setEntryStatus, importFormatFromName, importProgressText, faqImportTaskView } = await import('./FAQPage.tsx');
const { createTranslator } = await import('../i18n.ts');

const t = createTranslator('zh-CN');
const kbId = '8b26f48e-7196-405f-9803-ccf93be3cd37';
const noop = () => {};

type FAQViewProps = NonNullable<React.ComponentProps<typeof FAQPageView>>;
type FAQBreadcrumbProps = NonNullable<React.ComponentProps<typeof FAQBreadcrumb>>;

function baseViewProps(overrides: Partial<FAQViewProps> = {}): FAQViewProps {
  return {
    t,
    locale: 'zh-CN' as const,
    knowledgeBaseId: kbId,
    kbName: 'parity-faq-kb',
    kbList: [
      { id: kbId, name: 'parity-faq-kb', type: 'faq' },
      { id: 'kb-doc-1', name: 'docs-kb', type: 'document' },
    ],
    tags: [{ id: 'tag-1', seq_id: 3, name: '重要', chunk_count: 5 }],
    activeTagIds: [] as string[],
    entries: [] as unknown[],
    total: 0,
    page: 1,
    pageSize: 50,
    loading: false,
    canContribute: true,
    selectedCount: 0,
    keywordDraft: '',
    importOpen: false,
    importMode: 'append' as const,
    editorOpen: false,
    editorTitle: '',
    message: null,
    onNavigate: noop,
    onKeywordDraftChange: noop,
    onSearchSubmit: noop,
    onSearchClear: noop,
    onToggleTag: noop,
    onOpenCreate: noop,
    onOpenImport: noop,
    onCloseImport: noop,
    onImportModeChange: noop,
    onImportFile: noop,
    onExport: noop,
    onCloseEditor: noop,
    ...overrides,
  } as FAQViewProps;
}

const breadcrumbProps: FAQBreadcrumbProps = {
  t,
  knowledgeBaseId: kbId,
  kbName: 'parity-faq-kb',
  kbList: [{ id: kbId, name: 'parity-faq-kb', type: 'faq' }],
  onNavigate: noop,
};

// --- Breadcrumb (Vue faq-breadcrumb parity) --------------------------------------

test('breadcrumb shows 知识库 › kbName › 问答 instead of the raw UUID eyebrow', () => {
  const html = renderToStaticMarkup(React.createElement<FAQBreadcrumbProps>(FAQBreadcrumb, breadcrumbProps));
  assert.ok(html.includes('知识库'), 'renders 知识库 crumb');
  assert.ok(html.includes('parity-faq-kb'), 'renders kb name crumb');
  assert.ok(html.includes('问答'), 'renders 问答 current crumb');
  assert.ok(!html.includes(kbId), 'never exposes the raw KB UUID');
});

test('breadcrumb destinations mirror Vue: KB list, KB detail and KB settings', () => {
  assert.equal(faqKBListPath, '/platform/knowledge-bases');
  assert.equal(faqKBSettingsPath(kbId), '/knowledgeBase/' + kbId + '/settings');
});

test('kbName crumb is a dropdown switcher and info + gear icons are present', () => {
  const html = renderToStaticMarkup(React.createElement<FAQBreadcrumbProps>(FAQBreadcrumb, breadcrumbProps));
  assert.ok(html.includes('breadcrumb-link dropdown'), 'kbName crumb carries the switcher dropdown');
  assert.ok(html.includes('breadcrumb-separator'), 'chevron separators present');
  assert.ok(html.includes('kb-info-button'), 'info icon button present');
  assert.ok(html.includes('kb-settings-button'), 'settings gear button present');
});

// --- Page anatomy (Vue faq-header / faq-filter-bar / faq-empty-state parity) ------

test('empty page renders Vue copy, full-width search, tag filter and icon buttons', () => {
  const raw = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps()));
  // React escapes quotes in text nodes; unescape before the byte-exact assertions.
  const html = raw.replace(/&quot;/g, '"').replace(/&#x27;/g, "'");
  // Subtitle + empty state copy, byte-exact zh-CN
  assert.ok(html.includes('结构化问答管理，支持标准问、相似问和反例，精准匹配用户查询，提升问答准确率'));
  assert.ok(html.includes('暂无 FAQ 条目'));
  assert.ok(html.includes('点击上方"新增 FAQ 条目"按钮开始创建'));
  assert.ok(!html.includes('No FAQ entries match the current search.'), 'English empty state removed');
  // Full-width search with Vue placeholder
  assert.ok(html.includes('搜索问题和答案...'));
  assert.ok(!html.includes('标准问题'), 'React-specific 标准问题 placeholder removed');
  // Tag filter trigger defaults to 全部标签
  assert.ok(html.includes('全部标签'));
  // Icon buttons: create (+), export (download), search-test (search)
  assert.ok(html.includes('aria-label="新建"'));
  assert.ok(html.includes('aria-label="导出"'));
  assert.ok(html.includes('aria-label="检索测试"'));
  assert.ok(!html.includes('新建问答'), 'header text buttons removed');
  assert.ok(!html.includes('导入 CSV/JSON'), 'import header button removed');
  // Vue renders search + actions in one flat filter bar (faq-filter-bar), not the
  // old bordered Card/toolbar block.
  assert.ok(!raw.includes('wk-toolbar'), 'bordered search toolbar card removed');
  // Search box is a bare input (Vue faq-search-input), not a labelled toolbar field
  assert.ok(!html.includes('>搜索</label>'), '搜索 field label removed');
});

test('create and export icon buttons carry their Vue dropdown actions', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps()));
  assert.ok(html.includes('新增 FAQ 条目'), 'create dropdown item');
  assert.ok(html.includes('导入 FAQ'), 'import dropdown item');
  assert.ok(html.includes('导出 CSV'), 'export dropdown item');
  assert.ok(html.includes('导出 JSON'), 'export dropdown item');
});

test('contributor tag filter exposes the Vue tag-management entry point', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ onOpenTagManage: noop })));
  assert.ok(html.includes('管理标签'), 'tag management link');
});

test('import dialog carries the Vue mode radio group instead of the header select', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ importOpen: true })));
  assert.ok(html.includes('批量导入 FAQ'), 'import dialog title');
  assert.ok(html.includes('导入模式'), 'mode label');
  assert.ok(html.includes('追加导入'), 'append radio label');
  assert.ok(html.includes('替换现有条目'), 'replace radio label');
  assert.ok(html.includes('点击上传文件'), 'upload affordance');
});

test('editor drawer opens with the Vue create title', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, editorTitle: '新增 FAQ 条目' })));
  assert.ok(html.includes('标准问'), 'standard question field');
});

test('entries render as selectable rows with standard question and answers', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({
    entries: [{ id: 1, standard_question: '如何部署？', similar_questions: [], negative_questions: [], answers: ['使用 Docker。'], is_enabled: true, is_recommended: false }],
    total: 1,
  })));
  assert.ok(html.includes('如何部署？'));
  assert.ok(html.includes('使用 Docker。'));
  assert.ok(!html.includes('暂无 FAQ 条目'), 'empty state hidden when entries exist');
});

// --- Leftover (1): Vue scrolls for more entries, it has no Previous/Next pager ----

test('list drops the pager and renders the Vue infinite-scroll affordances', () => {
  const rows = [
    { id: 1, standard_question: '一', similar_questions: [], negative_questions: [], answers: ['a'], is_enabled: true, is_recommended: false },
    { id: 2, standard_question: '二', similar_questions: [], negative_questions: [], answers: ['b'], is_enabled: true, is_recommended: false },
  ];
  const idle = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ entries: rows as never, total: 75, hasMore: true })));
  assert.ok(!idle.includes('wk-pagination'), 'Vue has no Previous/Next pager');
  assert.ok(!idle.includes('上一页') && !idle.includes('下一页'), 'pager copy removed');
  assert.ok(!idle.includes('faq-load-more'), 'no spinner while idle');
  assert.ok(!idle.includes('faq-no-more'), 'no end hint while more pages exist');

  const loading = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ entries: rows as never, total: 75, hasMore: true, loadingMore: true })));
  assert.ok(loading.includes('faq-load-more'), 'load-more spinner renders while appending');
  assert.ok(loading.includes('加载中...'), 'spinner uses common.loading copy');

  const exhausted = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ entries: rows as never, total: 2, hasMore: false })));
  assert.ok(exhausted.includes('faq-no-more'), 'end hint renders once exhausted');
  assert.ok(exhausted.includes('已加载全部内容'), 'end hint uses common.noMoreData copy');
  assert.ok(!exhausted.includes('faq-load-more'), 'no spinner once exhausted');
});

test('faqHasMore mirrors the Vue entries.length < total rule', () => {
  assert.equal(faqHasMore(20, 41), true);
  assert.equal(faqHasMore(41, 41), false);
  assert.equal(faqHasMore(5, 0), false, 'empty total never reports more');
});

// --- Leftover (5): import dialog shows the Vue parsed-file preview block ----------

test('import dialog previews parsed entries: count, first five, more-hint', () => {
  const preview = Array.from({ length: 7 }, (_, index) => ({
    standard_question: '问题' + (index + 1), answers: ['答案'],
  }));
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ importOpen: true, importPreview: preview as never })));
  assert.ok(html.includes('import-preview'), 'preview block present');
  assert.ok(html.includes('共解析 7 条记录'), 'previewCount header');
  assert.ok(html.includes('问题1') && html.includes('问题5'), 'first five questions listed');
  assert.ok(!html.includes('问题6'), 'only five rows rendered');
  assert.ok(html.includes('还有 2 条未展示'), 'previewMore hint counts the rest');
});

test('import dialog hides the preview block until a file parses', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ importOpen: true })));
  assert.ok(!html.includes('import-preview'), 'no preview before a file is chosen');
});

test('importFormatFromName follows the Vue processFile format split', () => {
  assert.equal(importFormatFromName('entries.JSON'), 'json');
  assert.equal(importFormatFromName('entries.csv'), 'csv');
  assert.equal(importFormatFromName('table.xlsx'), 'excel');
  assert.equal(importFormatFromName('table.xls'), 'excel');
  assert.equal(importFormatFromName('data.txt'), 'csv');
});

// --- Leftover (5b): header import-progress strip (Vue faq-import-strip) -----------

test('header renders the import progress strip with bar and processed count', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({
    importTask: { status: 'running', text: '导入中...', progress: 40, processed: 2, total: 5 } as never,
  })));
  assert.ok(html.includes('faq-import-strip faq-import-strip--running'), 'status class present');
  assert.ok(html.includes('导入中...'), 'strip text rendered');
  assert.ok(html.includes('width:40%'), 'bar fill matches progress percent');
  assert.ok(html.includes('>2/5</span>'), 'processed/total count rendered');
  assert.ok(html.includes('faq-import-strip__bar-fill'), 'progress bar present');
});

test('import progress strip resolves Vue status copy via the zh fallback map', () => {
  assert.equal(importProgressText({ status: 'running' }), '导入中...');
  assert.equal(importProgressText({ status: 'success' }), '导入完成');
  assert.equal(importProgressText({ status: 'failed' }), '导入失败');
  assert.equal(importProgressText({ status: 'pending' }), '等待中...');
  assert.equal(importProgressText({ status: 'running', message: ' 服务器消息 ', error: '' }), '服务器消息', 'server message wins when present');
  assert.equal(importProgressText({ status: 'failed', error: 'boom', message: 'ignored' }), 'boom', 'error wins over message');
});

test('faqImportTaskView normalises the raw progress payload for the strip', () => {
  assert.deepEqual(faqImportTaskView({ status: 'running', progress: 133, processed: 3, total: 8 }),
    { status: 'running', text: '导入中...', progress: 100, processed: 3, total: 8 }, 'progress clamps to 100');
  assert.deepEqual(faqImportTaskView({ status: 'pending' }),
    { status: 'pending', text: '等待中...', progress: 0, processed: 0, total: 0 }, 'missing fields default to zero');
});

// --- Leftover (6): per-entry enable switch mirrors the Vue card footer ------------

test('entry rows carry an enable switch with the Vue tooltip copy', () => {
  const rows = [
    { id: 1, standard_question: '开', similar_questions: [], negative_questions: [], answers: ['a'], is_enabled: true, is_recommended: false },
    { id: 2, standard_question: '关', similar_questions: [], negative_questions: [], answers: ['b'], is_enabled: false, is_recommended: false },
  ];
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ entries: rows as never })));
  assert.ok(html.includes('faq-status-switch'), 'switch element rendered');
  assert.ok(html.includes('role="switch"'), 'switch role exposed');
  assert.ok(html.includes('aria-checked="true"'), 'enabled entry switch on');
  assert.ok(html.includes('aria-checked="false"'), 'disabled entry switch off');
  assert.ok(html.includes('title="已启用"'), 'enabled tooltip (Vue t-tooltip content)');
  assert.ok(html.includes('title="已禁用"'), 'disabled tooltip (Vue t-tooltip content)');
});

test('entry switch disables during per-entry updates and for viewers', () => {
  const rows = [
    { id: 1, standard_question: '开', similar_questions: [], negative_questions: [], answers: ['a'], is_enabled: true, is_recommended: false },
  ];
  const busy = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ entries: rows as never, statusUpdatingIds: [1] as never })));
  assert.ok(busy.includes('disabled'), 'switch disabled while its update is in flight');
  const viewer = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ entries: rows as never, canContribute: false })));
  assert.ok(!viewer.includes('faq-status-switch'), 'viewers get no switch (Vue :disabled="!canEdit")');
});

test('setEntryStatus updates immutably so a failed update can roll back', () => {
  const rows = [
    { id: 1, standard_question: 'a', answers: ['x'], is_enabled: true, similar_questions: [], negative_questions: [], is_recommended: false },
    { id: 2, standard_question: 'b', answers: ['y'], is_enabled: true, similar_questions: [], negative_questions: [], is_recommended: false },
  ] as never[];
  const next = setEntryStatus(rows, 2, false);
  assert.equal((next[1] as { is_enabled: boolean }).is_enabled, false);
  assert.equal((rows[1] as { is_enabled: boolean }).is_enabled, true, 'source array untouched');
  assert.equal(next[0], rows[0], 'untouched rows keep identity');
  assert.equal(setEntryStatus(rows, 99, false), rows, 'unknown id returns the input array');
});
