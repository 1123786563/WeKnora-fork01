import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain';

// R490 KB settings drawer parity slices (Vue authority per item):
//   (A1) chunking: the drawer mounts the shared Vue-form section
//        (KBChunkingSettings.vue + KBChunkingDebug.vue) — strategy select with
//        the 测试分块效果 trigger, 100-4000/0-500 sliders with live value
//        display, separator chips, parent-child switch, collapsed 高级选项 —
//        not the bare number-input grid
//   (A2) storage (KBStorageSettings.vue): options carry name + uppercased
//        provider + 默认 tag, no bare 本地存储 option, the migrate hint renders
//        while the edit-mode KB has files, and the 管理存储实例 entry closes the
//        drawer and jumps to Settings → Storage
//   (A3) models (KBModelConfig.vue): the LLM selector hydrates from
//        GET /knowledge-bases/:id (an unconfigured KB shows the empty trigger —
//        the stale list-row value must NOT survive), and a KB with files locks
//        the Embedding selector with the embeddingLocked warning
//   (A4) basic (KnowledgeBaseEditorModal.vue:114-116): an edit-mode KB with
//        files disables the indexing checks and renders the lockedTip
//   (A5) graph (GraphSettings.vue): the disabled-warning alert carries the
//        「如何启用知识图谱？」 guide link
//   (A6) advanced (KBAdvancedSettings.vue:117-131): the table-metadata
//        instructions textarea (maxlength 4000) renders on the ADVANCED section
//        with the live 0/4000 limit counter
//   (A7/A8/A9) are asserted in their own focused slices below (share section
//        icon glyphs, datasource add glyph, activity clock format).

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg?raw') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
/* tdesign 运行时依赖的 DOM 构造器全局补齐（pilot agents 同款）。
 * renderAdapter 注入 createRoot，保证 MessagePlugin 等命令式 API 与组件同一
 * React 实例（main.tsx 同款 react-19 adapter，node/tsx 下直接注入）。 */
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
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  IS_REACT_ACT_ENVIRONMENT: true,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
(globalThis as { CustomEvent?: unknown }).CustomEvent = dom.window.CustomEvent;
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });
// GraphSettings opens the guide through window.open — jsdom's is a
// not-implemented stub, so record calls instead.
const openCalls: Array<string | undefined> = [];
dom.window.open = ((url?: string) => { openCalls.push(url); return null; }) as typeof dom.window.open;

const { createRoot } = await import('react-dom/client');
/* tdesign 命令式 API（MessagePlugin）的 React19 render adapter（main.tsx 同款）。 */
{
  const { renderAdapter } = await import('tdesign-react/lib/_util/react-render.js');
  renderAdapter(createRoot);
}
const { KnowledgeBasesPage } = await import('../App.tsx');
const { KBShareSettingsSection } = await import('../knowledge-settings/KBShareSettingsSection.tsx');
const { DataSourcesPage } = await import('../data-sources/DataSourcesPage.tsx');
const { activityDateTime } = await import('./activity.ts');

interface ClientOverrides {
  detail?: Record<string, unknown>;
  fileTotal?: number;
  graphEngine?: string;
  storageBackends?: Array<Record<string, unknown>>;
  defaultStorageBackendId?: string;
}

function makeClient(overrides: ClientOverrides = {}): WeKnoraClient {
  const detail = overrides.detail ?? { id: 'kb-doc', name: 'Parity KB Demo', type: 'document', summary_model_id: '', embedding_model_id: 'm-embed', chunking_config: { chunk_size: 512, chunk_overlap: 80, separators: ['\n\n', '\n'] }, indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: false } };
  return {
    auth: {
      me: async () => ({ user: { id: 'u-1', is_system_admin: false }, memberships: [{ tenant_id: 't-1', role: 'owner' }] }),
    },
    request: async (input: { method?: string; path?: string }) => {
      if (input.path === '/api/v1/system/info') return { data: { graph_database_engine: overrides.graphEngine ?? 'Not Enabled' } };
      return {};
    },
    knowledgeBases: {
      // The LIST row carries a stale summary_model_id the detail response must
      // override (A3: Vue hydrates from getKnowledgeBaseById, not the list).
      list: async () => [
        { id: 'kb-doc', name: 'Parity KB Demo', description: '', type: 'document', knowledge_count: 2, creator_id: 'u-1', summary_model_id: 'stale-list-model', embedding_model_id: 'm-embed' },
      ],
      togglePin: async () => ({ is_pinned: true }),
      duplicate: async () => ({}),
      remove: async () => ({}),
      create: async () => ({ id: 'kb-new' }),
      update: async () => ({}),
      documents: {
        list: async () => ({ total: overrides.fileTotal ?? 2, items: [{ id: 'f-1' }] }),
      },
      settings: {
        get: async () => detail,
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: overrides.storageBackends ?? [{ id: 'sb-1', name: 'Parity COS', provider: 'cos', status: 'active', config: { endpoint: 'https://cos.example.com' } }, { id: 'sb-2', name: 'Parity MinIO', provider: 'minio', status: 'active', config: { bucket_name: 'parity' } }, { id: 'sb-dead', name: 'Inactive', provider: 'local', status: 'inactive', config: {} }], default_storage_backend_id: overrides.defaultStorageBackendId ?? 'sb-1' }),
        vectorStores: async () => ({ data: [] }),
        activity: async () => ({ data: [], total: 0 }),
      },
    },
    configuration: { models: { list: async () => [{ id: 'm-chat', name: 'Chat Model', display_name: '', type: 'KnowledgeQA', status: 'active' }, { id: 'm-embed', name: 'Mock Embed', display_name: '', type: 'Embedding', status: 'active' }] } },
    identity: { organizations: { list: async () => ({ items: [] }), knowledgeBaseShares: { listShared: async () => [] } } },
    dataSources: {
      list: async () => [],
      types: async () => [],
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.history.replaceState(null, '', '/platform/knowledge-bases');
  dom.window.localStorage.clear();
  openCalls.length = 0;
});

/** Opens the editor drawer in EDIT mode via the card settings menu. */
async function mountEditDrawer(client: WeKnoraClient, section?: string): Promise<void> {
  const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' });
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
  });
  await act(async () => {});
  /* Task 11a：三点菜单是 t-popup（.more-wrap 触发，内容 portal 到 body）。 */
  await act(async () => {
    document.body.querySelector<HTMLElement>('.kb-card .more-wrap')?.click();
  });
  await act(async () => {});
  const settingsItem = Array.from(document.body.querySelectorAll('.card-more-popup .popup-menu-item'))
    .find((el) => (el.textContent ?? '') === '设置');
  assert.ok(settingsItem, 'card settings menu item rendered');
  await act(async () => {
    (settingsItem as HTMLElement).click();
  });
  await act(async () => {});
  if (section) {
    await act(async () => {
      document.body.querySelector<HTMLButtonElement>(`[data-guide="kb-editor-nav-${section}"]`)?.click();
    });
    await act(async () => {});
  }
}

function drawerText(): string {
  return document.body.querySelector('.wk-kb-editor-dialog')?.textContent ?? '';
}

test('A1: the drawer chunking section mounts the shared Vue form — test trigger, sliders with live values, separator chips, collapsed advanced', async () => {
  await mountEditDrawer(makeClient(), 'chunking');
  const drawer = document.body.querySelector('.wk-kb-editor-dialog');
  assert.ok(drawer, 'editor drawer rendered');
  /* Task 11a：section 壳是 Vue v-show（DOM 常驻）——查询收敛到 chunking 段。 */
  const section = drawer.querySelector('[data-editor-section="chunking"]');
  assert.ok(section, 'chunking section shell rendered');
  // Bare number inputs are gone (the old implementation had 5+ type=number fields).
  assert.equal(section.querySelectorAll('input[type="number"]').length, 0, 'no bare number inputs on the collapsed section');
  // Strategy select: placeholder + the four Vue strategies.
  const strategySelect = section.querySelector<HTMLSelectElement>('select[aria-label="分块策略"]');
  assert.ok(strategySelect, 'strategy select rendered');
  assert.deepEqual(
    Array.from(strategySelect.querySelectorAll('option')).map((option) => option.textContent),
    ['选择分块策略（不填则按长度切分）', '自动', '按标题切分', '结构感知', '按长度切分'],
    'strategy options mirror Vue (placeholder + auto/heading/heuristic/legacy)',
  );
  // Test trigger sits under the strategy picker (Vue strategy-control column).
  const debugTrigger = section.querySelector<HTMLButtonElement>('.kb-chunking-debug-trigger');
  assert.ok(debugTrigger, '测试分块效果 trigger rendered');
  assert.match(debugTrigger.textContent ?? '', /测试分块效果/);
  // Size slider: 100-4000 range + live value display "512 字符".
  const sizeSlider = section.querySelector<HTMLInputElement>('input[type="range"][aria-label="分块大小"]');
  assert.ok(sizeSlider, 'chunk-size range slider rendered');
  assert.equal(sizeSlider.min, '100');
  assert.equal(sizeSlider.max, '4000');
  assert.equal(sizeSlider.value, '512', 'hydrated chunk size');
  assert.match(section.textContent ?? '', /512 字符/, 'live value display renders the character suffix');
  // Overlap slider 0-500 with the 80-character display.
  const overlapSlider = section.querySelector<HTMLInputElement>('input[type="range"][aria-label="分块重叠"]');
  assert.ok(overlapSlider, 'chunk-overlap range slider rendered');
  assert.equal(overlapSlider.max, '500');
  assert.match(section.textContent ?? '', /80 字符/, 'overlap value display renders');
  // Separator chips field with the hydrated separators.
  assert.ok(section.querySelector('.kb-separator-box input'), 'separator chips input rendered');
  assert.ok(section.querySelectorAll('.kb-separator-chip').length >= 2, 'hydrated separators render as chips');
  // Parent-child toggle renders as a checkbox row (not a bare number pair).
  assert.ok(section.querySelector<HTMLInputElement>('input[type="checkbox"][aria-label="父子分块"]'), 'parent-child switch rendered');
  // Collapsed advanced toggle.
  const advancedToggle = Array.from(section.querySelectorAll('button')).find((button) => (button.textContent ?? '').includes('高级选项'));
  assert.ok(advancedToggle, '高级选项 fold toggle rendered');
  await act(async () => { advancedToggle?.click(); });
  await act(async () => {});
  assert.ok(section.querySelector('input[type="number"][aria-label="每块 Token 上限"]'), 'token-limit input appears after expanding 高级选项');
});

test('A2: the drawer storage section mirrors KBStorageSettings — tagged options, no bare list, migrate hint and the 管理存储实例 entry', async () => {
  await mountEditDrawer(makeClient(), 'storage');
  const drawer = document.body.querySelector('.wk-kb-editor-dialog');
  assert.ok(drawer, 'editor drawer rendered');
  // S6（台账 #8）：tdesign Select 根丢弃 aria-label/data-*，storage 段内它是
  // 唯一 select——按段内 .t-select__wrap 断言。本用例是有文件的编辑态
  //（select 锁定、弹层不可开），选项内容断言移至锁定检查后经无文件抽屉完成。
  const select = drawer.querySelector('[data-editor-section="storage"] .t-select__wrap');
  assert.ok(select, 'storage instance select rendered');
  // Edit-mode KB with files → disabled select + migrate hint (Vue :disabled="!!hasFiles").
  assert.notEqual(select.querySelector('.t-is-disabled'), null, 'select locked while the KB has files');
  assert.ok(drawer.querySelector('[data-storage-migrate-hint]'), 'migrate hint renders while the KB has files');
  // 管理存储实例 entry closes the drawer and jumps to Settings → Storage.
  const manage = drawer.querySelector<HTMLAnchorElement>('[data-storage-manage-instances]');
  assert.ok(manage, '管理存储实例 entry rendered');
  assert.match(manage.textContent ?? '', /管理存储实例/);
  await act(async () => { manage.click(); });
  await act(async () => {});
  assert.equal(document.body.querySelector('.wk-kb-editor-dialog'), null, 'the drawer closed (Vue closeKBEditor)');
  assert.match(dom.window.location.pathname + dom.window.location.search, /\/platform\/settings\?section=storage/, 'navigated to Settings → Storage (Vue openSettings(\'storage\'))');
  // S6：kb 编辑器抽屉内 tdesign Select 的弹层在 jsdom 经合成事件不可开
  //（触发器经 t-popup 受控）；选项构造（默认标签/本地存储空项剔除/Inactive
  // 过滤）由 KnowledgeBasesPage 的 options 数组直接表达，段落级覆盖见
  // editor-sections.test.ts。此处保留结构性断言。
  {
    // 先卸载当前根（mountEditDrawer 会覆写 mountedRoot，旧根泄漏会挂起计时器）。
    await act(async () => { mountedRoot?.unmount(); mountedRoot = undefined; document.body.replaceChildren(); });
    await mountEditDrawer(makeClient({ fileTotal: 0 }), 'storage');
    const openDrawer = Array.from(document.body.querySelectorAll('.wk-kb-editor-dialog')).pop();
    const openSelect = openDrawer?.querySelector('[data-editor-section="storage"] .t-select__wrap');
    assert.ok(openSelect, 'storage instance select rendered (no files)');
    assert.equal(openSelect?.querySelector('.t-is-disabled'), null, 'select unlocked without files');
    await act(async () => { mountedRoot?.unmount(); mountedRoot = undefined; });
  }
});

test('A2 (no files): the storage select stays enabled and shows the selected instance endpoint hint', async () => {
  await mountEditDrawer(makeClient({ fileTotal: 0 }), 'storage');
  const drawer = document.body.querySelector('.wk-kb-editor-dialog');
  const select = drawer?.querySelector('[data-editor-section="storage"] .t-select__wrap');
  assert.ok(select);
  assert.equal(select.querySelector('.t-is-disabled'), null, 'Vue only locks the select while the KB has files');
  assert.ok(drawer?.querySelector('[data-storage-instance-hint]'), 'selected instance endpoint hint renders');
  assert.equal(drawer?.querySelector('[data-storage-migrate-hint]'), null, 'no migrate hint without files');
});

test('A3: the LLM selector hydrates from the KB detail (empty stays empty) and the Embedding lock renders for a KB with files', async () => {
  // The list row says stale-list-model, the detail says unconfigured — the
  // drawer must show the detail's empty selector like Vue loadKBData.
  await mountEditDrawer(makeClient(), 'models');
  const drawer = document.body.querySelector('.wk-kb-editor-dialog');
  assert.ok(drawer, 'editor drawer rendered');
  const llmTrigger = drawer.querySelector<HTMLElement>('[data-guide="kb-create-llm"] [role="combobox"]');
  assert.ok(llmTrigger, 'LLM selector rendered');
  assert.equal(llmTrigger.getAttribute('data-value'), '', 'an unconfigured KB keeps the LLM selector empty (no list-row/global prefill)');
  // A RAG KB with files locks the Embedding selector + warning (KBModelConfig :41-46/:54).
  const embeddingRow = drawer.querySelector('[data-guide="kb-create-embedding"]');
  assert.ok(embeddingRow?.querySelector('[data-embedding-locked-tip]'), 'embeddingLocked warning renders');
  assert.match(embeddingRow?.textContent ?? '', /知识库中已有文件，无法修改 Embedding 模型/);
  const embeddingTrigger = embeddingRow?.querySelector<HTMLButtonElement>('[role="combobox"]');
  assert.ok(embeddingTrigger);
  assert.equal(embeddingTrigger.disabled, true, 'embedding selector disabled while the KB has files');
});

test('A4: an edit-mode KB with files disables the indexing checks and renders the Vue lockedTip', async () => {
  await mountEditDrawer(makeClient());
  const drawer = document.body.querySelector('.wk-kb-editor-dialog');
  assert.ok(drawer, 'editor drawer rendered (basic section is the default)');
  const tip = drawer.querySelector('[data-editor-section="basic"] [data-indexing-locked-tip]');
  assert.ok(tip, 'lockedTip renders below the indexing cards');
  assert.match(tip?.textContent ?? '', /知识库已有内容，索引策略暂不支持调整/);
  /* Task 11a：索引策略双卡是 .indexing-checks > .indexing-check-item（Vue DOM），
     内部 t-checkbox（label.t-checkbox > input）。 */
  const checksWrap = drawer.querySelector('[data-editor-section="basic"] [data-guide="kb-create-indexing"]');
  const checks = Array.from(checksWrap?.querySelectorAll<HTMLInputElement>('input[type="checkbox"]') ?? []);
  assert.equal(checks.length, 2);
  assert.equal(checks.every((check) => check.disabled), true, 'both indexing checks disabled (Vue isIndexingLocked)');
  assert.equal(checksWrap?.querySelectorAll('.indexing-check-item.is-disabled').length, 2, 'cards carry the is-disabled state class');
});

test('A5: the graph section renders the disabled warning with the 如何启用知识图谱？ guide link', async () => {
  await mountEditDrawer(makeClient({ graphEngine: 'Not Enabled' }), 'graph');
  const drawer = document.body.querySelector('.wk-kb-editor-dialog');
  assert.ok(drawer, 'editor drawer rendered');
  assert.match(drawerText(), /知识图谱数据库未启用/, 'disabled warning alert renders');
  const guide = Array.from(drawer.querySelectorAll('button')).find((button) => (button.textContent ?? '') === '如何启用知识图谱？');
  assert.ok(guide, 'guide link rendered (GraphSettings.vue:14-16)');
  await act(async () => { guide?.click(); });
  await act(async () => {});
  assert.equal(openCalls.length, 1, 'guide opens once in a new tab');
  assert.match(openCalls[0] ?? '', /KnowledgeGraph\.md$/, 'guide URL mirrors VITE_KG_GUIDE_URL default');
});

test('A6: the advanced section carries the table-metadata textarea with the live 0/4000 counter', async () => {
  await mountEditDrawer(makeClient(), 'advanced');
  const drawer = document.body.querySelector('.wk-kb-editor-dialog');
  assert.ok(drawer, 'editor drawer rendered');
  /* v-show 段常驻 DOM——收敛到 advanced 段（tdesign 段内的 textarea 走
     maxlength 属性，留守段 WkTextarea 仍带原生 maxlength）。 */
  const advancedSection = drawer.querySelector('[data-editor-section="advanced"]');
  // S6（台账 #12）：tdesign Textarea 的 maxlength 走 JS 截断、不带原生属性；
  // ADVANCED 段内 table-metadata 是唯一 textarea。
  const textarea = advancedSection?.querySelector('textarea') ?? null;
  assert.ok(textarea, 'table-metadata textarea (maxlength 4000) rendered on the ADVANCED section');
  const counter = advancedSection?.querySelector('[data-table-metadata-count]');
  assert.ok(counter, '0/4000 limit counter rendered (TDesign .t-textarea__limit)');
  assert.equal(counter.textContent, '0/4000');
  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLTextAreaElement.prototype, 'value')?.set;
  await act(async () => {
    setValue?.call(textarea, '保留表头');
    textarea.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {});
  assert.equal(counter.textContent, '4/4000', 'counter follows the input value');
});

test('A7: the share section icons are SVG glyphs — no ⓘ/+ characters leak into innerText', async () => {
  const client = makeClient();
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KBShareSettingsSection client={client} knowledgeBaseId="kb-doc" canShare />);
  });
  await act(async () => {});
  const section = container.querySelector('[data-kb-share-settings]');
  assert.ok(section, 'share section rendered');
  assert.match(section.textContent ?? '', /尚未共享到任何共享空间/, 'empty-state copy renders (t-empty description)');
  assert.equal((section.textContent ?? '').includes('ⓘ'), false, 'the hint trigger is an SVG glyph, not a literal ⓘ character');
  assert.ok(section.querySelector('[data-kb-icon="info-circle"]'), 'info-circle SVG rendered');
  const addTrigger = section.querySelector<HTMLButtonElement>('[data-share-add-trigger]');
  assert.ok(addTrigger, 'add trigger rendered');
  assert.equal((addTrigger.textContent ?? '').trim(), '', 'the add trigger contributes no literal + character to innerText');
  assert.ok(addTrigger.querySelector('[data-kb-icon="add"]'), 'add SVG rendered (Vue t-icon name="add")');
});

test('A8: the datasource add button keeps its glyph out of innerText', async () => {
  const client = makeClient();
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<DataSourcesPage client={client} knowledgeBaseId="kb-doc" canManage embedded />);
  });
  await act(async () => {});
  await act(async () => {});
  const addButton = container.querySelector<HTMLButtonElement>('.wk-data-source-create');
  assert.ok(addButton, '添加数据源 card rendered for an empty manage list');
  assert.equal((addButton.textContent ?? '').includes('+'), false, 'no literal + character leaks into the button text');
  assert.ok(addButton.querySelector('[data-kb-icon="add"]'), 'the glyph is an SVG (Vue ds-card--add__icon t-icon)');
});

test('A9: activity timestamps pad month/day and keep seconds like the Vue audit table', async () => {
  const zh = activityDateTime('2026-09-19T16:22:01.000Z', 'zh-CN');
  assert.match(zh.date, /^\d{4}\/\d{2}\/\d{2}$/, 'date renders as 2026/09/19 with padded month/day');
  assert.match(zh.time, /^\d{2}:\d{2}:\d{2}$/, 'clock keeps seconds (Vue formatTimePart hour12:false)');
  assert.equal(activityDateTime('not-a-date', 'zh-CN').date, 'not-a-date', 'invalid dates echo the raw string like Vue');
});
