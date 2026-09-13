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

const { FAQBreadcrumb, FAQPageView, faqKBListPath, faqKBSettingsPath } = await import('./FAQPage.tsx');
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