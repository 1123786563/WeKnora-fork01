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

const { FAQBreadcrumb, FAQPageView, FAQSearchResults, createFaqTranslator, faqKBListPath, faqKBSettingsPath, faqHasMore, setEntryStatus, importFormatFromName, importProgressText, faqImportTaskView, pushListItem, removeListItem, editorFormError, faqSaveResultKey, faqBatchSuccessKey, faqDeleteSuccessKey, isSectionCollapsed, toggleSection, FAQ_ANSWER_CAP, FAQ_SIMILAR_CAP, faqSearchDefaultForm, faqSearchBlocked, faqSearchRequestFrom, faqSearchResultsFromResponse, toggleSearchResultId, filterFaqTags, faqMasonryColumnCount, faqImportBlocked } = await import('./FAQPage.tsx');

const t = createFaqTranslator('zh-CN');
const kbId = '8b26f48e-7196-405f-9803-ccf93be3cd37';
const noop = () => {};

test('FAQ batch feedback resolves to shared localized Vue keys', () => {
  assert.equal(faqBatchSuccessKey({ is_enabled: true }), 'knowledgeEditor.faq.statusEnableSuccess');
  assert.equal(faqBatchSuccessKey({ is_enabled: false }), 'knowledgeEditor.faq.statusDisableSuccess');
  assert.equal(faqBatchSuccessKey({ is_recommended: true }), 'knowledgeEditor.faq.recommendedEnabled');
  assert.equal(faqBatchSuccessKey({ tag_id: 3 }), 'knowledgeBase.tagUpdateSuccess');
  assert.equal(faqDeleteSuccessKey(1), 'knowledgeEditor.faqImport.deleteSuccess');
  assert.equal(faqDeleteSuccessKey(2), 'knowledgeEditor.faq.batchDeleteSuccess');
});

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
  assert.ok(html.includes('faq-breadcrumb-separator'), 'chevron separators present');
  assert.ok(html.includes('faq-kb-info-button'), 'info icon button present');
  assert.ok(html.includes('faq-kb-settings-button'), 'settings gear button present');
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

test('FAQ view defaults to no write actions when capability is absent', () => {
  const props = baseViewProps();
  delete (props as Partial<FAQViewProps>).canContribute;
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, props));
  assert.ok(!html.includes('aria-label="新建"'), 'create/import actions fail closed');
  assert.ok(!html.includes('faq-card-check'), 'selection actions fail closed');
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

test('tag filter searches labels like the Vue tag-search input', () => {
  const tags = [
    { id: '1', seq_id: 1, name: '产品手册', chunk_count: 2 },
    { id: '2', seq_id: 2, name: '客服', chunk_count: 1 },
  ] as never;
  assert.deepEqual(filterFaqTags(tags, ' 手册 '), [tags[0]], 'trimmed query filters by label');
  assert.deepEqual(filterFaqTags(tags, ''), tags, 'blank query keeps all tags');
});

test('import dialog carries the Vue mode radio group instead of the header select', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ importOpen: true })));
  assert.ok(html.includes('批量导入 FAQ'), 'import dialog title');
  assert.ok(html.includes('导入模式'), 'mode label');
  assert.ok(html.includes('追加导入'), 'append radio label');
  assert.ok(html.includes('替换现有条目'), 'replace radio label');
  assert.ok(html.includes('点击上传文件'), 'upload affordance');
  assert.ok(html.includes('下载示例'), 'Vue import dialog exposes the example-download menu');
});

test('active tag filter exposes the Vue clear affordance', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({
    activeTagIds: ['tag-1'],
    onClearTagFilter: noop,
  })));
  assert.ok(html.includes('aria-label="清除"'), 'selected tag filter can be cleared without reopening the menu');
});

test('import submission requires a parsed file preview like Vue handleImport', () => {
  assert.equal(faqImportBlocked(null, 0), true);
  assert.equal(faqImportBlocked('faq.json', 0), true);
  assert.equal(faqImportBlocked('faq.json', 1), false);
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

// --- B1: editor drawer (Vue t-drawer 520px, FAQEntryManager.vue:440-577) ----------

const drawerForm: any = {
  question: '标准问',
  similarQuestions: ['相似问一'],
  negativeQuestions: [] as string[],
  answers: ['答案一'],
  tagId: '3',
  enabled: true,
  recommended: false,
  similarDraft: '',
  negativeDraft: '',
  answerDraft: '',
};

test('editor drawer mirrors the Vue form: labels, per-field desc copy, required marks', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, editorTitle: '新增 FAQ 条目', editorMode: 'create', form: drawerForm })));
  assert.ok(html.includes('faq-editor-drawer'), 'drawer shell present');
  assert.ok(html.includes(t('knowledgeEditor.faq.standardQuestionDesc')), 'standard question desc key');
  assert.ok(html.includes(t('knowledgeEditor.faq.similarQuestionsDesc')), 'similar questions desc key');
  assert.ok(html.includes(t('knowledgeEditor.faq.negativeQuestionsDesc')), 'negative questions desc key');
  assert.ok(html.includes(t('knowledgeEditor.faq.answersDesc')), 'answers desc key');
  assert.ok(html.includes(t('knowledgeEditor.faq.tagDesc')), 'tag desc key');
  assert.match(html, /max(?:length|Length)=\"200\"/, 'standard question capped at 200 (Vue t-input :maxlength)');
  assert.equal((html.match(/required-mark/g) || []).length, 2, '标准问 + 答案 carry the required mark');
  assert.ok(!html.includes('faq-editor-checks'), 'Vue editor has no enable/recommended checkboxes');
  // R488: t-select semantics — the placeholder lives in the trigger only while
  // no tag is selected (the old native-select placeholder <option> was K2 noise).
  const untagged = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, form: { ...drawerForm, tagId: '' } })));
  assert.ok(untagged.includes(t('knowledgeEditor.faq.tagPlaceholder')), 'tag trigger shows the Vue placeholder while unselected');
});

// R488 A3 residual (K2): the Vue tag field is a t-select — its options live in
// a dropdown, so a closed editor leaks only the placeholder line. A native
// <select> leaks every <option> text into innerText (K2 noise: 请选择标签 +
// every tag name). The React field must be a closed-state combobox trigger.
test('editor tag field is a closed-state combobox, not a native select leaking options', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, editorTitle: '新增 FAQ 条目', editorMode: 'create', form: drawerForm })));
  const tagRow = html.slice(html.indexOf('faq-editor-tag'));
  assert.ok(tagRow.includes('role="combobox"'), 'tag trigger carries the combobox role');
  assert.ok(!html.includes('<option'), 'no native <option> text leaks into the editor drawer');
  assert.ok(!html.includes('>重要</option>'), 'closed tag dropdown does not list tag names');
  // Selected tag renders inside the trigger (Vue t-select shows the label).
  const tagged = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, form: { ...drawerForm, tagId: '3' } })));
  assert.ok(tagged.includes('>重要</span>'), 'trigger shows the selected tag name');
});

test('editor drawer builds Vue list fields: add buttons, item rows, n/5 counter', () => {
  // 2 committed answers => counter reads 2/5 (Vue item-count counts committed only)
  const capped = { ...drawerForm, similarQuestions: Array.from({ length: 10 }, (_, i) => '相似' + i), answers: ['答1', '答2'], similarDraft: '还想要一条', answerDraft: '新的答案' };
  const cappedHtml = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, form: capped })));
  assert.ok(cappedHtml.includes('add-item-btn'), 'per-list add button rendered');
  assert.ok(cappedHtml.includes('item-row'), 'list items render as removable rows');
  assert.ok(cappedHtml.includes('2/5'), 'answer counter renders as n/5 (Vue item-count)');
  assert.ok(/<button[^>]*add-item-btn[^>]* disabled=""/.test(cappedHtml), 'similar add disabled at cap 10');
  const open = { ...drawerForm, similarDraft: '相似草稿', negativeDraft: '反例草稿', answerDraft: '答案草稿' };
  const openHtml = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, form: open })));
  assert.ok(!/<button[^>]*add-item-btn[^>]* disabled=""/.test(openHtml), 'add buttons enable when below cap with a draft');
  assert.ok(openHtml.includes(t('knowledgeEditor.faq.similarPlaceholder')), 'similar placeholder from shared catalog');
  assert.ok(openHtml.includes(t('knowledgeEditor.faq.negativePlaceholder')), 'negative placeholder from shared catalog');
  assert.ok(openHtml.includes(t('knowledgeEditor.faq.answerPlaceholder')), 'answer placeholder from shared catalog');
});

test('drawer footer submit follows the Vue create/save label split', () => {
  const createHtml = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, editorTitle: '新增 FAQ 条目', editorMode: 'create', form: drawerForm })));
  assert.ok(createHtml.includes('新增 FAQ 条目'), 'create submit labelled editorCreate');
  const editHtml = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, editorTitle: '编辑 FAQ 条目', editorMode: 'edit', form: drawerForm })));
  assert.ok(editHtml.includes('保存'), 'edit submit labelled common.save');
});

test('editorFormError mirrors the Vue editorRules order', () => {
  assert.equal(editorFormError({ question: '  ', answers: ['a'] }), 'question', 'standard_question required first');
  assert.equal(editorFormError({ question: 'q', answers: [] }), 'answers', 'answers required second');
  assert.equal(editorFormError({ question: 'q', answers: ['a'] }), null);
});

test('faqSaveResultKey resolves the shared Vue success keys', () => {
  assert.equal(faqSaveResultKey(false), 'knowledgeEditor.messages.createSuccess');
  assert.equal(faqSaveResultKey(true), 'knowledgeEditor.messages.updateSuccess');
});

test('drawer surfaces validation errors inline', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ editorOpen: true, form: drawerForm, message: { tone: 'error', text: '请输入知识库名称' } })));
  assert.ok(html.includes('faq-editor-error'), 'error slot rendered inside the drawer');
  assert.ok(html.includes('请输入知识库名称'), 'Vue validation copy visible');
});

test('pushListItem trims, dedupes and caps like the Vue add handlers', () => {
  assert.deepEqual(pushListItem([], '  hello  ', 10), { list: ['hello'], added: true }, 'trims before push');
  assert.deepEqual(pushListItem(['hello'], 'hello', 10), { list: ['hello'], added: false }, 'duplicate rejected');
  assert.deepEqual(pushListItem(['a', 'b'], '  ', 2), { list: ['a', 'b'], added: false }, 'empty draft rejected');
  assert.deepEqual(pushListItem(['a', 'b'], 'c', 2), { list: ['a', 'b'], added: false }, 'cap 2 blocks third item');
  assert.equal(FAQ_SIMILAR_CAP, 10, 'Vue similar cap');
  assert.equal(FAQ_ANSWER_CAP, 5, 'Vue answer cap');
});

test('removeListItem splices immutably like the Vue remove handlers', () => {
  const source = ['a', 'b', 'c'];
  const next = removeListItem(source, 1);
  assert.deepEqual(next, ['a', 'c']);
  assert.deepEqual(source, ['a', 'b', 'c'], 'source untouched');
});

// --- B3: entry cards (Vue faq-card three-section collapse, FAQEntryManager.vue:253-410) ---

const cardRows = [
  { id: 1, standard_question: '如何部署？', similar_questions: ['docker?', 'k8s?'], negative_questions: ['什么是反例'], answers: ['使用 Docker。'], is_enabled: true, is_recommended: false, tag_id: 3 },
] as never;

function renderCards(overrides: Partial<FAQViewProps> = {}): string {
  return renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ entries: cardRows, total: 1, ...overrides })));
}

// R437: Vue FAQBatchBar.vue:53-63 — 批量启用 shows only when the selection has
// disabled entries (disabledCount > 0), 批量禁用 only when it has enabled ones.
test('FAQ batch enable/disable render conditionally on the selected entries status (Vue FAQBatchBar)', () => {
  const rows = [
    { id: 1, standard_question: 'q1', similar_questions: [], negative_questions: [], answers: [], is_enabled: true, is_recommended: false },
    { id: 2, standard_question: 'q2', similar_questions: [], negative_questions: [], answers: [], is_enabled: false, is_recommended: false },
  ] as never;
  const renderWith = (selected: Set<number>) => renderToStaticMarkup(
    React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ entries: rows, selected })),
  );
  const mixed = renderWith(new Set([1, 2]));
  assert.ok(mixed.includes('批量启用') && mixed.includes('批量禁用'), 'mixed selection keeps both actions');
  const allEnabled = renderWith(new Set([1]));
  assert.ok(allEnabled.includes('批量禁用'), 'enabled-only selection keeps disable');
  assert.ok(!allEnabled.includes('批量启用'), 'enabled-only selection hides enable (Vue disabledCount === 0)');
  const allDisabled = renderWith(new Set([2]));
  assert.ok(allDisabled.includes('批量启用'), 'disabled-only selection keeps enable');
  assert.ok(!allDisabled.includes('批量禁用'), 'disabled-only selection hides disable (Vue enabledCount === 0)');
});

test('entries render as Vue faq-cards with a question header and more menu', () => {
  const html = renderCards();
  assert.ok(html.includes('faq-card-list'), 'Vue card list container');
  assert.ok(html.includes('is-selectable'), 'cards are click-selectable (Vue handleCardSelect)');
  assert.ok(html.includes('faq-question'), 'question header block');
  assert.ok(html.includes('title=\"如何部署？\"'), 'question carries the native tooltip');
  assert.ok(!html.includes('wk-faq-item'), 'old list rows removed');
  assert.ok(html.includes('card-more-btn'), 'more trigger rendered');
  assert.ok(html.includes('aria-label=\"操作\"'), 'more trigger labelled from the shared catalog');
  assert.ok(html.includes('编辑'), 'more menu carries the edit item');
  assert.ok(html.includes('删除'), 'more menu carries the delete item');
});

test('FAQ cards use the Vue responsive masonry breakpoints', () => {
  assert.equal(faqMasonryColumnCount(639), 1);
  assert.equal(faqMasonryColumnCount(640), 3);
  assert.equal(faqMasonryColumnCount(1024), 5);
  assert.equal(faqMasonryColumnCount(1920), 10);
  assert.equal(faqMasonryColumnCount(2560), 12);
});

test('cards expose checkbox multi-select wired to the selection set', () => {
  const html = renderCards({ selected: new Set([1]) });
  assert.ok(html.includes('faq-card-check'), 'per-card checkbox present');
  assert.ok(html.includes('checked'), 'selected card checkbox checked');
  assert.ok(/faq-card[^\"']* selected/.test(html), 'card carries the Vue selected class');
});

test('viewers get neither selection affordances nor the more menu', () => {
  const viewer = renderCards({ canContribute: false });
  assert.ok(!viewer.includes('faq-card-check'), 'no checkbox for viewers');
  assert.ok(!viewer.includes('card-more-btn'), 'no more menu for viewers');
  assert.ok(!viewer.includes('is-selectable'), 'cards not selectable for viewers');
});

test('cards render the three collapsible sections collapsed by default', () => {
  const html = renderCards();
  assert.ok(html.includes('faq-section similar'), 'similar section');
  assert.ok(html.includes('faq-section negative'), 'negative section');
  assert.ok(html.includes('faq-section answers'), 'answers section');
  assert.ok(html.includes('相似问') && html.includes('反例') && html.includes('答案'), 'section labels from the shared catalog');
  assert.ok(html.includes('>(2)</span>'), 'similar count rendered as (n)');
  assert.ok(/faq-section-label[^>]*aria-expanded=\"false\"/.test(html), 'sections collapsed by default (FAQEntryManager.vue:1592-1594)');
  assert.ok(/class=\"faq-tags[^\"]*\" hidden/.test(html), 'collapsed section bodies hidden but kept in the DOM');
});

test('empty sections disappear while answers always render', () => {
  const sparse = [{ id: 2, standard_question: 'q', similar_questions: [], negative_questions: [], answers: [], is_enabled: true, is_recommended: false }] as never;
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ entries: sparse, total: 1 })));
  assert.ok(!html.includes('faq-section similar'), 'similar section hidden when empty (Vue v-if)');
  assert.ok(!html.includes('faq-section negative'), 'negative section hidden when empty');
  assert.ok(html.includes('faq-section answers'), 'answers section always present');
});

test('card footer keeps the tag chip and status switch', () => {
  const html = renderCards();
  assert.ok(html.includes('faq-card-footer'), 'footer present');
  assert.ok(html.includes('faq-tag-chip'), 'tag chip present');
  assert.ok(html.includes('重要'), 'tag name resolved via seq_id');
  assert.ok(html.includes('faq-status-switch'), 'status switch kept in the footer');
  const untagged = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({
    entries: [{ id: 3, standard_question: 'q2', similar_questions: [], negative_questions: [], answers: ['a'], is_enabled: true, is_recommended: false }] as never,
    total: 1,
  })));
  assert.ok(untagged.includes('无标签'), 'missing tag falls back to knowledgeBase.untagged');
});

test('tag chip carries the full tag name in the FAQTagTooltip bubble, not a native title', () => {
  // B5 refine: the d3a39b7b native title is replaced by the FAQTagTooltip bubble
  // (FAQEntryManager.vue:362-376 footer chip + frontend/src/components/FAQTagTooltip.vue):
  // hover reveals the full text in a fixed, viewport-clamped bubble; the chip
  // itself carries no title attribute.
  const html = renderCards();
  assert.match(html, /<span class="faq-tag-wrapper[^"]*"><span class="faq-tag-chip[^"]*"><span class="tag-text[^"]*">重要<\/span><\/span><\/span>/, 'resolved tag name renders inside the tooltip wrapper');
  assert.ok(!/class="faq-tag-chip[^"]*" title=/.test(html), 'native title removed in favour of the bubble');
  const untagged = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({
    entries: [{ id: 3, standard_question: 'q2', similar_questions: [], negative_questions: [], answers: ['a'], is_enabled: true, is_recommended: false }] as never,
    total: 1,
  })));
  assert.match(untagged, /<span class="tag-text[^"]*">无标签<\/span>/, 'untagged fallback feeds the bubble content');
  assert.ok(!untagged.includes('title="无标签"'), 'untagged chip has no native title');
});

test('section collapse helpers default to collapsed and flip immutably', () => {
  assert.equal(isSectionCollapsed({}, 1, 'answers'), true, 'Vue defaults every section collapsed');
  const expanded = toggleSection({}, 1, 'answers');
  assert.equal(isSectionCollapsed(expanded, 1, 'answers'), false, 'toggle flips the section');
  assert.equal(isSectionCollapsed({}, 1, 'answers'), true, 'source state untouched');
  assert.equal(isSectionCollapsed(toggleSection(expanded, 1, 'answers'), 1, 'answers'), true, 'second toggle restores');
  assert.equal(isSectionCollapsed(toggleSection({}, 2, 'similar'), 3, 'similar'), true, 'state is per entry id');
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

// --- B4: search test drawer (Vue t-drawer 420px, FAQEntryManager.vue:734-853) -----

const searchHits = [
  { id: 2, standard_question: '如何扩容？', similar_questions: ['怎么扩容'], negative_questions: [], answers: ['加节点。'], is_enabled: true, is_recommended: false, score: 0.9123, matched_question: '如何扩容集群' },
  { id: 1, standard_question: '如何部署？', similar_questions: [], negative_questions: [], answers: [], is_enabled: true, is_recommended: false, score: 0.5 },
] as never;

function renderSearchResults(overrides: { results?: unknown; expandedIds?: ReadonlySet<number> } = {}): string {
  const results = 'results' in overrides ? overrides.results : searchHits;
  const expandedIds = overrides.expandedIds ?? new Set<number>();
  const Props = FAQSearchResults as unknown as React.ComponentType<{ t: typeof t; results: unknown; expandedIds: ReadonlySet<number>; onToggle: (id: number) => void }>;
  return renderToStaticMarkup(React.createElement(Props, { t, results, expandedIds, onToggle: noop }));
}

test('search drawer is closed by default and opens only on request', () => {
  const closed = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps()));
  assert.ok(!closed.includes('faq-search-drawer'), 'no drawer markup before it is requested');
  const open = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ searchOpen: true })));
  assert.ok(open.includes('faq-search-drawer'), 'drawer shell present');
  assert.ok(open.includes('FAQ 检索测试'), 'Vue searchTestTitle header');
});

test('search drawer mirrors the Vue form: labels, descs, defaults and slider bounds', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ searchOpen: true })));
  assert.ok(html.includes('查询内容'), 'query label');
  assert.ok(html.includes('请输入要检索的问题'), 'query placeholder doubles as the Vue desc');
  assert.ok(html.includes('相似度阈值'), 'threshold label');
  assert.ok(html.includes('范围 0-1，默认 0.7'), 'threshold desc');
  assert.ok(html.includes('0.70'), 'threshold value renders toFixed(2)');
  assert.ok(html.includes('结果数量'), 'match count label');
  assert.ok(html.includes('范围 1-50，默认 10'), 'match count desc');
  assert.ok(/>10</.test(html), 'match count value renders as integer');
  assert.ok(/type="range"[^>]*min="0"[^>]*max="1"[^>]*step="0\.1"/.test(html), 'threshold slider bounds 0-1 step 0.1');
  assert.ok(/type="range"[^>]*min="1"[^>]*max="50"[^>]*step="1"/.test(html), 'count slider bounds 1-50 step 1');
  assert.ok(html.includes('开始检索'), 'submit button uses searchButton copy');
});

test('searching pins the submit button to 检索中... and disables it', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ searchOpen: true, searching: true })));
  assert.ok(html.includes('检索中...'), 'searching label');
  assert.ok(!html.includes('开始检索'), 'idle label replaced while searching');
  assert.ok(/aria-busy="true"/.test(html), 'button exposes busy state');
});

test('result cards rank 1-based with 3-decimal scores and the matched question', () => {
  const html = renderSearchResults();
  assert.ok(html.includes('检索结果 (2)'), 'results header carries the count');
  assert.ok(html.includes('1.'), 'first card indexed from 1');
  assert.ok(html.includes('0.912') && html.includes('0.500'), 'scores render toFixed(3)');
  assert.ok(html.includes('命中问题'), 'matched label rendered');
  assert.ok(html.includes('如何扩容集群'), 'matched question text shown when it differs');
  const same = [{ id: 5, standard_question: '同问', similar_questions: [], negative_questions: [], answers: [], is_enabled: true, is_recommended: false, score: 1, matched_question: '同问' }];
  const sameHtml = renderSearchResults({ results: same });
  assert.ok(!sameHtml.includes('命中问题'), 'matched line hidden when identical to the standard question (Vue v-if)');
});

test('hit bodies default collapsed; expansion reveals answers and similar questions', () => {
  const collapsed = renderSearchResults();
  assert.ok(!collapsed.includes('加节点。'), 'answers hidden while collapsed (Vue expanded=false)');
  assert.ok(!collapsed.includes('怎么扩容'), 'similar hidden while collapsed');
  assert.ok(/aria-expanded="false"/.test(collapsed), 'expander state exposed');
  const expanded = renderSearchResults({ expandedIds: new Set([2]) });
  assert.ok(expanded.includes('答案') && expanded.includes('加节点。'), 'answers section reveals');
  assert.ok(expanded.includes('相似问') && expanded.includes('怎么扩容'), 'similar section reveals');
  assert.ok(/aria-expanded="true"/.test(expanded), 'expander flips');
});

test('toggleSearchResultId flips one hit immutably', () => {
  const ids = toggleSearchResultId(new Set([1]), 2);
  assert.deepEqual([...ids].sort(), [1, 2], 'adds the new id');
  assert.deepEqual([...toggleSearchResultId(ids, 1)], [2], 'second toggle removes');
});

test('empty search state renders noResults once a search ran, nothing before', () => {
  const empty = renderSearchResults({ results: [] });
  assert.ok(empty.includes('未找到匹配的 FAQ 条目'), 'Vue noResults copy');
  const fresh = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ searchOpen: true })));
  assert.ok(!fresh.includes('search-results'), 'no results block before the first search (Vue hasSearched gate)');
  const searched = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ searchOpen: true, hasSearched: true, searchResults: [] as never })));
  assert.ok(searched.includes('未找到匹配的 FAQ 条目'), 'results block appears once hasSearched');
});

test('search errors surface inside the drawer like the editor drawer', () => {
  const html = renderToStaticMarkup(React.createElement<FAQViewProps>(FAQPageView, baseViewProps({ searchOpen: true, message: { tone: 'error', text: '检索失败' } })));
  assert.ok(html.includes('faq-editor-error'), 'error slot rendered inside the drawer');
  assert.ok(html.includes('检索失败'), 'error text visible');
});

test('faqSearchDefaultForm matches the Vue defaults (query blank, 0.7, 10)', () => {
  assert.deepEqual(faqSearchDefaultForm(), { query: '', vectorThreshold: 0.7, matchCount: 10 });
});

test('faqSearchBlocked guards the blank query like Vue', () => {
  assert.equal(faqSearchBlocked({ query: '   ', vectorThreshold: 0.7, matchCount: 10 }), true, 'blank query blocks');
  assert.equal(faqSearchBlocked({ query: '部署', vectorThreshold: 0.7, matchCount: 10 }), false, 'real query passes');
});

test('faqSearchRequestFrom trims the query and forwards thresholds', () => {
  assert.deepEqual(faqSearchRequestFrom({ query: '  如何部署  ', vectorThreshold: 0.7, matchCount: 10 }),
    { query_text: '如何部署', vector_threshold: 0.7, match_count: 10 });
});

test('faqSearchResultsFromResponse unwraps the envelope, defaults score, sorts desc', () => {
  const raw = { success: true, data: [
    { id: 1, standard_question: '低分', answers: [], similar_questions: [], negative_questions: [], is_enabled: true, is_recommended: false, score: 0.4 },
    { id: 2, standard_question: '高分', answers: [], similar_questions: [], negative_questions: [], is_enabled: true, is_recommended: false, score: 0.9 },
    { id: 3, standard_question: '无分', answers: [], similar_questions: [], negative_questions: [], is_enabled: true, is_recommended: false },
  ] };
  const hits = faqSearchResultsFromResponse(raw) as Array<{ id: number; score: number }>;
  assert.deepEqual(hits.map((hit) => hit.id), [2, 1, 3], 'sorted by score desc (Vue :2673)');
  assert.equal(hits[2].score, 0, 'missing score defaults to 0 (Vue score||0)');
  assert.deepEqual(faqSearchResultsFromResponse({ success: true }), [], 'missing data array yields no hits');
});
