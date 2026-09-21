import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain';

// R438 A1 slice — shared knowledge base detail drawer (Vue authority
// KnowledgeBaseList.vue:294-315 trigger, :709-776 drawer, :1489-1525 logic):
//   (a) info-circle "查看详情" entry renders on non-own shared cards only
//   (b) right drawer mirrors the Vue fields: title / name / source type /
//       source org or agent (+ agent KB strategy) / shared at / my permission
//   (c) footer: 关闭 (outline) + 进入知识库 (primary, browse icon) navigating
//       to the KB detail route and closing the drawer
//   (d) close semantics: Esc, footer 关闭

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
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });

const { createRoot } = await import('react-dom/client');
/* tdesign 命令式 API（MessagePlugin）的 React19 render adapter（main.tsx 同款）。 */
{
  const { renderAdapter } = await import('tdesign-react/lib/_util/react-render.js');
  renderAdapter(createRoot);
}
const { KnowledgeBasesPage } = await import('../App.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.history.replaceState(null, '', '/platform/knowledge-bases');
  dom.window.localStorage.clear();
});

function makeClient(options: { owned?: unknown[]; shared?: unknown[] } = {}) {
  const owned = options.owned ?? [
    { id: 'kb-mine', name: '我自己的库', description: 'owned', type: 'document', knowledge_count: 2, creator_id: 'u-1', summary_model_id: 'm-1', embedding_model_id: 'm-2' },
  ];
  return {
    auth: {
      me: async () => ({ user: { id: 'u-1', is_system_admin: false }, memberships: [{ tenant_id: 't-1', role: 'contributor' }] }),
    },
    knowledgeBases: {
      list: async () => owned,
      togglePin: async () => ({ is_pinned: true }),
      duplicate: async () => ({}),
      remove: async () => ({}),
      create: async () => ({ id: 'kb-new' }),
      update: async () => ({}),
    },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => options.shared ?? [] } } },
  } as unknown as WeKnoraClient;
}

interface SharedRow {
  knowledge_base: Record<string, unknown> | null;
  permission: string;
  shared_at: string;
  share_id: string;
  organization_id: string;
  org_name: string;
  source_from_agent?: { agent_id: string; agent_name: string; kb_selection_mode?: string };
}

function sharedRow(overrides: Partial<SharedRow> & { share_id: string; knowledge_base: Record<string, unknown> }): SharedRow {
  return {
    permission: 'editor',
    shared_at: '2026-01-01T00:00:00Z',
    organization_id: 'org-alpha',
    org_name: '设计组',
    ...overrides,
  } as SharedRow;
}

async function mountPage(client: WeKnoraClient) {
  // Shared cards live in the "all" scope; pin it explicitly because a
  // contributor otherwise lands on the workspace scope by default.
  dom.window.history.replaceState(null, '', '/platform/knowledge-bases?scope=all');
  const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' });
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
  });
  await act(async () => {});
  return container;
}

function click(element: Element) {
  return act(async () => {
    element.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true }));
  });
}

// Vue formatStringDate (frontend/src/utils/index.ts:55): local
// YYYY-MM-DD HH:mm:ss. Computed from the row's own date so the assertion
// stays timezone-independent.
function expectedDateString(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

const directShareRows = (): SharedRow[] => [
  sharedRow({ share_id: 'share-1', knowledge_base: { id: 'kb-shared-1', name: '共享库一', type: 'document', knowledge_count: 4, creator_id: 'u-2' }, permission: 'editor' }),
  sharedRow({ share_id: 'share-2', knowledge_base: { id: 'kb-shared-2', name: '共享库二', type: 'faq', chunk_count: 9, creator_id: 'u-3' }, permission: 'viewer' }),
  sharedRow({ share_id: 'share-3', knowledge_base: { id: 'kb-shared-3', name: '共享库三', type: 'document', knowledge_count: 0, creator_id: 'u-4' }, permission: 'viewer' }),
];

test('(a) shared cards carry the info-circle 查看详情 entry; owned cards do not', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  const sharedCard = container.querySelector('[data-kb-id="kb-shared-1"]');
  const ownedCard = container.querySelector('[data-kb-id="kb-mine"]');
  assert.ok(sharedCard, 'shared card renders');
  assert.ok(ownedCard, 'owned card renders');
  /* Task 11a：触发器是 Vue .shared-detail-trigger（t-tooltip content 无原生
     title），图标是 t-icon sprite（use href）。 */
  const trigger = sharedCard.querySelector('.shared-detail-trigger');
  assert.ok(trigger, 'shared card has the .shared-detail-trigger entry (KnowledgeBaseList.vue:312)');
  assert.equal(trigger.getAttribute('aria-label'), '查看详情', 'trigger aria-label = knowledgeList.menu.viewDetails');
  assert.ok(trigger.querySelector('use[href="#t-icon-info-circle"]'), 'trigger uses the info-circle t-icon');
  assert.equal(ownedCard.querySelector('.shared-detail-trigger'), null, 'owned card must not offer the entry');
  // R445 item 1 — Vue shared card header (KnowledgeBaseList.vue:304-315) has
  // ONLY the 查看详情 trigger: no three-dot settings menu on non-own cards.
  // Boolean form: feeding a live jsdom Element to assert.equal's diff OOMs the runner.
  assert.ok(!sharedCard.querySelector('.more-wrap'), 'shared card must not offer the three-dot 设置 entry');
  assert.ok(ownedCard.querySelector('.more-wrap'), 'owned card keeps the three-dot menu control');
  // The trigger must not navigate: card click opens the KB; trigger stops propagation.
  await click(trigger);
  await act(async () => {});
  assert.notEqual(dom.window.location.pathname, '/platform/knowledge-bases/kb-shared-1', 'trigger click is stopPropagation-ed (Vue @click.stop)');
});

// R445 item 3 — Vue shared card badge (KnowledgeBaseList.vue:336): empty
// count renders '-' (`kb.knowledge_count || '-'`); own cards keep `|| 0`.
test('(a) shared card badge shows - for an empty count like Vue', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  const emptyShared = container.querySelector('[data-kb-id="kb-shared-3"] .badge-count');
  assert.equal(emptyShared?.textContent, '-', 'shared card with knowledge_count 0 renders - (KnowledgeBaseList.vue:336)');
  const populatedShared = container.querySelector('[data-kb-id="kb-shared-1"] .badge-count');
  assert.equal(populatedShared?.textContent, '4', 'shared card with a real count keeps the number');
  const owned = container.querySelector('[data-kb-id="kb-mine"] .badge-count');
  assert.equal(owned?.textContent, '2', 'owned card keeps the numeric count');
});

test('(b) drawer mirrors the Vue fields for a directly shared KB', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  assert.equal(document.querySelector('.kb-shared-detail-drawer'), null, 'drawer closed initially');
  await click(container.querySelector('[data-kb-id="kb-shared-1"] .shared-detail-trigger')!);
  const drawer = document.querySelector('.shared-detail-drawer');
  assert.ok(drawer, 'drawer opens (KnowledgeBaseList.vue:712 shared-detail-drawer, createPortal body)');
  assert.equal(drawer.getAttribute('role'), 'dialog', 'drawer is a dialog');
  const text = drawer.textContent ?? '';
  assert.ok(text.includes('共享知识库'), 'title = knowledgeList.detail.title');
  assert.ok(text.includes('名称'), 'row label knowledgeBase.name');
  assert.ok(text.includes('共享库一'), 'value = knowledge_base.name');
  assert.ok(text.includes('来源方式'), 'row label knowledgeList.detail.sourceType');
  assert.ok(text.includes('知识库直接共享到本空间'), 'sourceTypeKbShare for a direct share');
  assert.ok(text.includes('来源空间'), 'row label knowledgeList.detail.sourceOrg');
  assert.ok(text.includes('设计组'), 'org_name value');
  assert.ok(text.includes('共享时间'), 'row label knowledgeList.detail.sharedAt');
  assert.ok(text.includes(expectedDateString('2026-01-01T00:00:00Z')), 'shared_at rendered as Vue formatStringDate');
  assert.ok(text.includes('我的权限'), 'row label knowledgeList.detail.myPermission');
  const tag = drawer.querySelector('.t-tag');
  assert.ok(tag, 'permission rendered as a t-tag');
  assert.equal(tag.textContent, '编辑', 'permission tag text = organization.role.editor');
  // Agent-only rows must not appear for a direct share.
  assert.equal(text.includes('智能体知识库策略'), false, 'agentKbStrategy row hidden without source_from_agent');
  // Footer buttons.
  const closeBtn = Array.from(drawer.querySelectorAll('button')).find((b) => b.textContent === '关闭');
  const goBtn = Array.from(drawer.querySelectorAll('button')).find((b) => (b.textContent ?? '').includes('进入知识库'));
  assert.ok(closeBtn, 'footer 关闭 button (common.close)');
  assert.ok(goBtn, 'footer 进入知识库 button (knowledgeList.detail.goToKb)');
  assert.ok(goBtn.querySelector('use[href="#t-icon-browse"]'), 'go button carries the browse t-icon');
  // R445 item 4 — Vue header close (KnowledgeBaseList.vue:713-716) is the ×
  // icon button with aria-label $t('general.close') = 「关闭设置」.
  const headerClose = drawer.querySelector('.shared-detail-drawer-close');
  assert.ok(headerClose, 'header × close button renders (.shared-detail-drawer-close)');
  assert.equal(headerClose.getAttribute('aria-label'), '关闭设置', 'header close aria-label = general.close (关闭设置)');
});

test('(b) viewer permission renders the read-only role label', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  await click(container.querySelector('[data-kb-id="kb-shared-2"] .shared-detail-trigger')!);
  const tag = document.querySelector('.shared-detail-drawer .t-tag');
  assert.equal(tag?.textContent, '只读', 'organization.role.viewer');
});

test('(b) agent-carried share switches source rows to the agent variant', async () => {
  const container = await mountPage(makeClient({
    shared: [
      sharedRow({
        share_id: 'share-agent',
        knowledge_base: { id: 'kb-agent', name: '智能体带进的库', type: 'document', knowledge_count: 1, creator_id: 'u-4' },
        permission: 'viewer',
        source_from_agent: { agent_id: 'agent-9', agent_name: '问答助手', kb_selection_mode: 'selected' },
      }),
    ],
  }));
  await click(container.querySelector('[data-kb-id="kb-agent"] .shared-detail-trigger')!);
  const drawer = document.querySelector('.shared-detail-drawer');
  assert.ok(drawer, 'drawer opens for agent-carried shares');
  const text = drawer.textContent ?? '';
  assert.ok(text.includes('智能体可访问（通过共享智能体可见）'), 'sourceTypeAgent copy');
  assert.ok(text.includes('智能体'), 'row label knowledgeList.detail.sourceFromAgent');
  assert.ok(text.includes('问答助手'), 'agent_name value');
  assert.ok(text.includes('智能体知识库策略'), 'agentKbStrategy row appears with source_from_agent');
  assert.ok(text.includes('指定知识库'), 'kb_selection_mode selected -> agentKbStrategySelected');
  assert.equal(text.includes('设计组'), false, 'direct-share org row replaced by the agent rows');
});

test('(c) 进入知识库 navigates to the KB detail route and closes the drawer', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  await click(container.querySelector('[data-kb-id="kb-shared-1"] .shared-detail-trigger')!);
  const drawer = document.querySelector('.shared-detail-drawer');
  const goBtn = Array.from(drawer!.querySelectorAll('button')).find((b) => (b.textContent ?? '').includes('进入知识库'))!;
  await click(goBtn);
  assert.equal(dom.window.location.pathname, '/platform/knowledge-bases/kb-shared-1', 'navigates like Vue goToSharedKbFromPanel');
  assert.equal(document.querySelector('.shared-detail-drawer'), null, 'drawer closes after navigation');
});

test('(d) the overlay backdrop, header × and the footer 关闭 button close the drawer', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  await click(container.querySelector('[data-kb-id="kb-shared-1"] .shared-detail-trigger')!);
  assert.ok(document.querySelector('.shared-detail-drawer'), 'open');

  /* Vue 关闭语义（KnowledgeBaseList.vue:713-714 closeSharedDetailPanel 三入口）：
     overlay click.self / header × / footer 关闭。旧 Sheet 的 Esc 行为随旧栈废弃。 */
  const overlay = document.querySelector('.shared-detail-drawer-overlay')!;
  await act(async () => {
    overlay.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true }));
  });
  await act(async () => {});
  assert.equal(document.querySelector('.shared-detail-drawer'), null, 'overlay backdrop click closes');

  await click(container.querySelector('[data-kb-id="kb-shared-2"] .shared-detail-trigger')!);
  const drawer = document.querySelector('.shared-detail-drawer');
  assert.ok(drawer, 'reopens for the second share');
  await click(drawer!.querySelector('.shared-detail-drawer-close')!);
  assert.equal(document.querySelector('.shared-detail-drawer'), null, 'header × closes');

  await click(container.querySelector('[data-kb-id="kb-shared-1"] .shared-detail-trigger')!);
  const drawer2 = document.querySelector('.shared-detail-drawer');
  assert.ok(drawer2, 'reopens for the footer check');
  const closeBtn = Array.from(drawer2!.querySelectorAll('button')).find((b) => b.textContent === '关闭')!;
  await click(closeBtn);
  assert.equal(document.querySelector('.shared-detail-drawer'), null, 'footer 关闭 closes');
});
