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
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });

const { createRoot } = await import('react-dom/client');
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
];

test('(a) shared cards carry the info-circle 查看详情 entry; owned cards do not', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  const sharedCard = container.querySelector('[data-kb-id="kb-shared-1"]');
  const ownedCard = container.querySelector('[data-kb-id="kb-mine"]');
  assert.ok(sharedCard, 'shared card renders');
  assert.ok(ownedCard, 'owned card renders');
  const trigger = sharedCard.querySelector('.kb-shared-detail-trigger');
  assert.ok(trigger, 'shared card has the .kb-shared-detail-trigger entry (KnowledgeBaseList.vue:312)');
  assert.equal(trigger.getAttribute('aria-label'), '查看详情', 'trigger aria-label = knowledgeList.menu.viewDetails');
  assert.equal(trigger.getAttribute('title'), '查看详情', 'trigger tooltip = knowledgeList.menu.viewDetails');
  assert.ok(trigger.querySelector('svg[data-kb-icon="info-circle"]'), 'trigger uses the info-circle icon');
  assert.equal(ownedCard.querySelector('.kb-shared-detail-trigger'), null, 'owned card must not offer the entry');
  // The trigger must not navigate: card click opens the KB; trigger stops propagation.
  await click(trigger);
  await act(async () => {});
  assert.notEqual(dom.window.location.pathname, '/platform/knowledge-bases/kb-shared-1', 'trigger click is stopPropagation-ed (Vue @click.stop)');
});

test('(b) drawer mirrors the Vue fields for a directly shared KB', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  assert.equal(document.querySelector('.kb-shared-detail-drawer'), null, 'drawer closed initially');
  await click(container.querySelector('[data-kb-id="kb-shared-1"] .kb-shared-detail-trigger')!);
  const drawer = document.querySelector('.kb-shared-detail-drawer');
  assert.ok(drawer, 'drawer opens (KnowledgeBaseList.vue:712 shared-detail-drawer)');
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
  const tag = drawer.querySelector('.kb-shared-detail-permission');
  assert.ok(tag, 'permission rendered as a tag');
  assert.equal(tag.textContent, '编辑', 'permission tag text = organization.role.editor');
  // Agent-only rows must not appear for a direct share.
  assert.equal(text.includes('智能体知识库策略'), false, 'agentKbStrategy row hidden without source_from_agent');
  // Footer buttons.
  const closeBtn = Array.from(drawer.querySelectorAll('button')).find((b) => b.textContent === '关闭');
  const goBtn = Array.from(drawer.querySelectorAll('button')).find((b) => (b.textContent ?? '').includes('进入知识库'));
  assert.ok(closeBtn, 'footer 关闭 button (common.close)');
  assert.ok(goBtn, 'footer 进入知识库 button (knowledgeList.detail.goToKb)');
  assert.ok(goBtn.querySelector('svg[data-kb-icon="browse"]'), 'go button carries the browse icon (Vue t-icon browse)');
});

test('(b) viewer permission renders the read-only role label', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  await click(container.querySelector('[data-kb-id="kb-shared-2"] .kb-shared-detail-trigger')!);
  const tag = document.querySelector('.kb-shared-detail-drawer .kb-shared-detail-permission');
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
  await click(container.querySelector('[data-kb-id="kb-agent"] .kb-shared-detail-trigger')!);
  const drawer = document.querySelector('.kb-shared-detail-drawer');
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
  await click(container.querySelector('[data-kb-id="kb-shared-1"] .kb-shared-detail-trigger')!);
  const drawer = document.querySelector('.kb-shared-detail-drawer');
  const goBtn = Array.from(drawer!.querySelectorAll('button')).find((b) => (b.textContent ?? '').includes('进入知识库'))!;
  await click(goBtn);
  assert.equal(dom.window.location.pathname, '/platform/knowledge-bases/kb-shared-1', 'navigates like Vue goToSharedKbFromPanel');
  assert.equal(document.querySelector('.kb-shared-detail-drawer'), null, 'drawer closes after navigation');
});

test('(d) Esc and the footer 关闭 button close the drawer', async () => {
  const container = await mountPage(makeClient({ shared: directShareRows() }));
  await click(container.querySelector('[data-kb-id="kb-shared-1"] .kb-shared-detail-trigger')!);
  assert.ok(document.querySelector('.kb-shared-detail-drawer'), 'open');

  await act(async () => {
    document.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
  });
  assert.equal(document.querySelector('.kb-shared-detail-drawer'), null, 'Esc closes');

  await click(container.querySelector('[data-kb-id="kb-shared-2"] .kb-shared-detail-trigger')!);
  const drawer = document.querySelector('.kb-shared-detail-drawer');
  assert.ok(drawer, 'reopens for the second share');
  const closeBtn = Array.from(drawer!.querySelectorAll('button')).find((b) => b.textContent === '关闭')!;
  await click(closeBtn);
  assert.equal(document.querySelector('.kb-shared-detail-drawer'), null, 'footer 关闭 closes');
});
