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

const { TagFilterPanel, TagManageDialog, TagPickerDialog } = await import('./TagPickerDialog.tsx');
const { tagSurfaceT } = await import('./tags-locale.ts');
const { KnowledgeDocumentsPage } = await import('./KnowledgeDocumentsPage.tsx');
const { createTranslator } = await import('../i18n.ts');

const t = createTranslator('zh-CN');
const tt = tagSurfaceT('zh-CN');
const kbId = '9727d104-cde4-4d03-879f-d7e3897b69a3';
const tags = [
  { id: '7', name: '重要', knowledge_count: 12 },
  { id: '9', name: '合同', knowledge_count: 3 },
];

// --- tagSurfaceT: shared i18n first, byte-exact Vue fallback for missing keys ------

test('tagSurfaceT consumes shared i18n keys via formatMessage', () => {
  assert.equal(tt('knowledgeBase.batchTagDialogHeading'), '批量打标签');
  assert.equal(tt('knowledgeBase.batchTagSubtitle', { count: 4 }), '为选中的 4 个文档统一设置标签（将替换文档原有标签）');
  assert.equal(tt('knowledgeBase.tagClearAction'), '清空已选');
});

test('tagSurfaceT falls back to the Vue locale copy packages/i18n still misses', () => {
  // Vue zh-CN common.confirm (frontend/src/i18n/locales/zh-CN.ts L4673)
  assert.equal(tt('common.confirm'), '确认');
  // Vue common.clear (L4701)
  assert.equal(tt('common.clear'), '清空');
  // Vue tenant.loadMore (L3318)
  assert.equal(tt('tenant.loadMore'), '加载更多');
  assert.equal(tt('common.operationFailed'), '操作失败');
  const en = tagSurfaceT('en-US');
  assert.equal(en('common.confirm'), 'Confirm');
  assert.equal(en('common.clear'), 'Clear');
  assert.equal(en('tenant.loadMore'), 'Load more');
  // Unknown keys still echo, same contract as formatMessage.
  assert.equal(tt('no.such.key'), 'no.such.key');
});

// --- TagPickerDialog (Vue BatchTagDialog.vue / TagEditDialog.vue) -------------------

test('batch tag dialog renders the Vue heading, subtitle, sections and footer count', () => {
  const html = renderToStaticMarkup(React.createElement(TagPickerDialog, {
    open: true,
    t: tt,
    tags,
    mode: 'batch',
    count: 4,
    preSelectedIds: ['7'],
    canManage: true,
    createTag: (name: string) => Promise.resolve({ id: '99', name }),
    onConfirm: () => {},
    onClose: () => {},
  }));
  assert.ok(html.includes('批量打标签'), 'heading copy');
  assert.ok(html.includes('为选中的 4 个文档统一设置标签'), 'subtitle carries the selection count');
  assert.ok(html.includes('已选标签'), 'selected section');
  assert.ok(html.includes('可选标签'), 'available section');
  assert.ok(html.includes('重要'), 'selected chip visible');
  assert.ok(html.includes('合同'), 'available chip visible');
  assert.ok(html.includes('清空已选'), 'clear selection affordance');
  assert.ok(html.includes('已选 1 个标签'), 'footer count mirrors Vue tagSelectedCount');
  assert.ok(html.includes('确认'), 'Vue common.confirm footer button');
  assert.ok(html.includes('取消'), 'Vue common.cancel footer button');
  assert.ok(html.includes('搜索标签...'), 'search placeholder (tagEditSearch)');
  assert.ok(html.includes('输入新标签名称，回车添加'), 'create-tag input (tagNewPlaceholder)');
});

test('single-document tag dialog swaps in the Vue TagEditDialog copy', () => {
  const html = renderToStaticMarkup(React.createElement(TagPickerDialog, {
    open: true,
    t: tt,
    tags,
    mode: 'single',
    preSelectedIds: [],
    canManage: false,
    onConfirm: () => {},
    onClose: () => {},
  }));
  assert.ok(html.includes('编辑标签'), 'tagEditDialogHeading');
  assert.ok(!html.includes('为选中的'), 'no batch subtitle in single mode');
  assert.ok(html.includes('暂未选择'), 'tagEditNoSelected empty copy');
  assert.ok(!html.includes('批量'), 'no batch vocabulary leaks into the edit dialog');
});

test('tag picker dialog renders nothing when closed', () => {
  const html = renderToStaticMarkup(React.createElement(TagPickerDialog, {
    open: false,
    t: tt,
    tags,
    mode: 'batch',
    count: 1,
    preSelectedIds: [],
    onConfirm: () => {},
    onClose: () => {},
  }));
  assert.equal(html, '');
});

// --- TagFilterPanel (Vue tag-filter-panel, KnowledgeBase.vue L2468-2523) ------------

test('tag filter panel lists chips with counts and marks the active selection', () => {
  const html = renderToStaticMarkup(React.createElement(TagFilterPanel, {
    t: tt,
    tags,
    selectedIds: ['9'],
    onToggle: () => {},
    onClear: () => {},
    onClose: () => {},
  }));
  assert.ok(html.includes('按标签筛选'), 'panel title (tagFilterTitle)');
  assert.ok(html.includes('(2)'), 'category count next to the title');
  assert.ok(html.includes('12'), 'important tag knowledge_count shown');
  assert.ok(html.includes('active'), 'selected chip carries the Vue active state');
  // Vue 面板的清空动作在触发器 suffix（面板本体不再渲染清空按钮）。
});

test('tag filter panel shows the Vue empty result and hides clear without selection', () => {
  const html = renderToStaticMarkup(React.createElement(TagFilterPanel, {
    t: tt,
    tags: [],
    selectedIds: [],
    onToggle: () => {},
    onClear: () => {},
    onClose: () => {},
  }));
  assert.ok(html.includes('未找到匹配的标签'), 'tagEmptyResult copy');
  assert.ok(html.includes('输入标签名称关键字'), 'panel search placeholder (tagSearchPlaceholder)');
  assert.ok(!html.includes('清空已选'), 'clear hidden with no selection');
});

// --- R490 B2: tag manage entry + dialog (KbTagManageDrawer.vue) ---------------------

test('tag filter panel shows the 管理标签… footer entry only for contributors (Vue canEdit)', () => {
  const managed = renderToStaticMarkup(React.createElement(TagFilterPanel, {
    t: tt,
    tags,
    selectedIds: [],
    onToggle: () => {},
    onClear: () => {},
    onClose: () => {},
    canManage: true,
    onManage: () => {},
  }));
  assert.ok(managed.includes('管理标签…'), 'manage entry (tagManageLink)');

  const readonly = renderToStaticMarkup(React.createElement(TagFilterPanel, {
    t: tt,
    tags,
    selectedIds: [],
    onToggle: () => {},
    onClear: () => {},
    onClose: () => {},
  }));
  assert.ok(!readonly.includes('管理标签…'), 'viewer surface hides the manage entry (Vue v-if=canEdit)');
});

// --- Documents page chrome (SSR) ----------------------------------------------------

test('documents page keeps Vue batch controls hidden until batch mode and drops the raw-ID prompt', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentsPage, {
    client: {} as never,
    knowledgeBaseId: kbId,
  }));
  assert.ok(!html.includes('批量打标签'), 'batch actions stay hidden until Vue batch mode is entered');
  assert.ok(!html.includes('Set tag IDs for selected documents'), 'window.prompt copy is gone');
  assert.ok(!html.includes('取消选择'), 'batch bar stays hidden before Vue batch mode');
  assert.ok(!html.includes('全选'), 'select-all stays hidden before Vue batch mode');
  assert.ok(html.includes('doc-tag-filter-trigger'), 'tag filter trigger button');
  assert.ok(html.includes('按标签筛选'), 'trigger aria-label/title');
  assert.ok(html.includes('全部标签'), 'default trigger label (allTags)');
});

test('documents page keeps the pre-existing filter chrome green', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentsPage, {
    client: {} as never,
    knowledgeBaseId: kbId,
  }));
  assert.ok(html.includes('doc-type-select'), 'tdesign doc-type-select filters');
  assert.ok(html.includes('doc-filter-bar'), 'Vue filter bar class');
});

test('tag manage dialog renders the Vue drawer copy, doc counts and per-row actions', () => {
  const html = renderToStaticMarkup(React.createElement(TagManageDialog, {
    t: tt,
    open: true,
    tags: [
      { id: '7', name: '重要', knowledge_count: 12, seq_id: 7 },
      { id: '9', name: '合同', knowledge_count: 3, seq_id: 9 },
      { id: '11', name: '无序号', knowledge_count: 0 },
    ],
    createTag: () => Promise.resolve(),
    updateTag: () => Promise.resolve(),
    deleteTag: () => Promise.resolve(),
    onClose: () => {},
  }));
  assert.ok(html.includes('管理标签</'), 'dialog title (tagManageTitle)');
  assert.ok(html.includes('新建、重命名或删除知识库标签'), 'description (tagManageDescription)');
  assert.ok(html.includes('12 个文档'), 'row count uses the doc variant (tagManageDocCount)');
  assert.ok(html.includes('3 个文档'), 'second row count');
  assert.ok(html.includes('重命名'), 'rename action (tagEditAction)');
  assert.ok(html.includes('删除'), 'delete action (tagDeleteAction)');
  assert.ok(html.includes('新建标签'), 'create affordance (tagCreateAction)');
  assert.ok(html.includes('输入标签名称关键字'), 'search placeholder (tagSearchPlaceholder)');
  // Vue deleteTag uses tag.seq_id; a tag without a safe seq_id disables delete.
  const disabledCount = (html.match(/disabled(="")?/g) ?? []).length;
  assert.ok(disabledCount >= 1, 'row without seq_id keeps delete disabled');
});

test('tag manage dialog hides its body when closed', () => {
  const html = renderToStaticMarkup(React.createElement(TagManageDialog, {
    t: tt,
    open: false,
    tags,
    createTag: () => Promise.resolve(),
    updateTag: () => Promise.resolve(),
    deleteTag: () => Promise.resolve(),
    onClose: () => {},
  }));
  assert.equal(html, '', 'closed dialog renders nothing');
});
