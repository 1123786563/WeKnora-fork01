import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import type { AgentCardModel, AgentViewer } from './list.ts';

/* 弹层（Popup/Dialog portal）在 renderToStaticMarkup 中不可见；菜单/对话框
 * 用例走 jsdom 真挂载。全局补齐 tdesign 运行时依赖的 DOM 构造器。 */
const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
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
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const {
  AgentCard,
  AgentDeleteDialog,
  AgentDetailDrawer,
  AgentRail,
  AgentsPage,
  AgentsPageView,
  hasAgentChatModel,
  loadAgentsPageData,
  parseAgentEditDeepLink,
  resolveAgentEditTarget,
} = await import('./AgentsPage.tsx');
const {
  buildAllViewRows,
  buildSpaceViewRows,
  cornerBadge,
  expertSourceId,
  favoritesStorageKey,
  readFavoriteIds,
  sectionize,
  sharedAgentsFromRecords,
} = await import('./list.ts');
const { formatMessage } = await import('@weknora/i18n');

const t = (key: string, values?: Record<string, string | number>) => formatMessage('zh-CN', key, values);
const noop = () => {};
const admin: AgentViewer = { userId: 'user-1', isAdmin: true, isContributor: true };

test('agent creation readiness follows Vue KnowledgeQA model semantics', () => {
  assert.equal(hasAgentChatModel([{ type: 'KnowledgeQA' }]), true);
  assert.equal(hasAgentChatModel([{ type: 'llm' }]), false);
  assert.equal(hasAgentChatModel([{ type: 'Embedding' }]), false);
});

test('agent edit deep links parse the Vue query contract and ignore empty values', () => {
  assert.deepEqual(parseAgentEditDeepLink('?edit=a-1&section=tools&highlight=allowed_tools&sourceTenantId=10001'), {
    editId: 'a-1',
    section: 'tools',
    highlight: 'allowed_tools',
    sourceTenantId: '10001',
  });
  assert.deepEqual(parseAgentEditDeepLink('?edit=a-1&section=&highlight=&sourceTenantId='), {
    editId: 'a-1',
    section: 'basic',
    highlight: undefined,
    sourceTenantId: undefined,
  });
  assert.equal(parseAgentEditDeepLink('?edit=a-1&highlight=allowed_tools')?.section, 'tools');
  assert.equal(parseAgentEditDeepLink('?edit=a-1&highlight=unknown')?.section, 'basic');
  assert.equal(parseAgentEditDeepLink('?section=tools'), null);
});

test('agent edit deep links resolve own agents first and shared agents by source tenant', () => {
  const own = fixtureRows().find((row) => row.id === 'a-own')!;
  const shared = fixtureRows().find((row) => row.id === 'a-shared')!;
  assert.equal(resolveAgentEditTarget([own], [shared], 'a-own')?.id, 'a-own');
  assert.equal(resolveAgentEditTarget([], [shared], 'a-shared', '10001')?.id, 'a-shared');
  assert.equal(resolveAgentEditTarget([], [shared], 'a-shared', 'other'), null);
  assert.equal(resolveAgentEditTarget([], [shared], 'a-shared'), null);
});

const builtinAgents = [
  { id: 'builtin-quick-answer', name: '快速问答', is_builtin: true, config: { agent_mode: 'quick-answer' } },
  { id: 'builtin-smart-reasoning', name: '智能推理', is_builtin: true, config: { agent_mode: 'smart-reasoning' } },
  { id: 'builtin-wiki-qa', name: '维基问答', is_builtin: true, config: { agent_mode: 'quick-answer', kb_selection_mode: 'all' } },
  { id: 'builtin-data-analyst', name: '数据分析师', is_builtin: true, config: { agent_mode: 'smart-reasoning' } },
];

function fixtureRows() {
  const ownAgent = { id: 'a-own', name: '我的助手', description: '帮我写周报', is_builtin: false, created_by: 'user-1', config: { agent_mode: 'quick-answer', web_search_enabled: true, multi_turn_enabled: true } };
  const shared = sharedAgentsFromRecords([
    { agent: { id: 'a-shared', name: '共享助手', description: '', is_builtin: false, config: { agent_mode: 'smart-reasoning', kb_selection_mode: 'all', model_id: 'm-1', web_search_enabled: true } }, share_id: 's-1', organization_id: 'org-1', org_name: '空间一', permission: 'editor', source_tenant_id: 10001, disabled_by_me: false },
  ]);
  return buildAllViewRows([...builtinAgents, ownAgent], shared, { userId: 'user-1', disabledOwnIds: ['a-own'] });
}

const { createRoot } = await import('react-dom/client');
{
  const { renderAdapter } = await import('tdesign-react/lib/_util/react-render.js');
  renderAdapter(createRoot);
}

let pageTestRoot: { render: (node: React.ReactElement) => void; unmount: () => void } | null = null;
async function mountToBody(node: React.ReactElement): Promise<HTMLElement> {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  await act(async () => { root.render(node); });
  pageTestRoot = root;
  return container;
}
afterEach(async () => {
  if (pageTestRoot) await act(async () => { pageTestRoot?.unmount(); });
  pageTestRoot = null;
  document.body.replaceChildren();
});

const baseViewProps = {
  t,
  editorT: t,
  client: {} as never,
  viewer: admin,
  loading: false,
  space: 'all',
  rail: [],
  sections: [],
  flatCards: [],
  isSectioned: true,
  favorites: new Set<string>(),
  openMenuId: null as string | null,
  error: null as string | null,
  drawer: null as null,
  editor: null as null,
  deleteTarget: null as AgentCardModel | null,
  deleting: false,
  collapsedSections: new Set<string>(),
  notice: null as string | null,
  canCreate: true,
  onSpaceChange: noop,
  onOpenCard: noop,
  onToggleFavorite: noop,
  onToggleMenu: noop,
  onMenuAction: noop,
  onCreate: noop,
  onCloseDrawer: noop,
  onUseInChat: noop,
  onCloseEditor: noop,
  onEditorSaved: noop,
  onDeleteConfirm: noop,
  onDeleteCancel: noop,
  onToggleSection: noop,
};

// --- favorites write path (Task 9.5 fix round 1: DB hydrate + write-through) ----
// Vue useResourcePins parity: hydrate from GET /user/favorites?type=agent,
// optimistic toggle via POST/DELETE, rollback on failure. These mount the full
// AgentsPage with a spy client.userFavorites — view-level assertions cannot
// cover the network call sequence or the rollback path.

function favoritesSpyClient(options?: {
  favoriteRows?: Array<{ resource_id: string }>;
  addError?: Error;
  removeError?: Error;
}) {
  const calls: Array<{ op: 'list' | 'add' | 'remove'; type: string; id?: string }> = [];
  const client = {
    auth: {
      me: async () => ({
        user: { id: 'user-1', username: 'tester', email: '', avatar: '' },
        tenant: { id: 1, name: 'Home' },
        memberships: [{ tenant_id: 1, tenant_name: 'Home', role: 'owner' }],
      }),
    },
    configuration: {
      agents: { listWithState: async () => ({ items: [{ id: 'a-own', name: '我的助手', is_builtin: false, created_by: 'user-1', config: { agent_mode: 'quick-answer' } }], disabledOwnAgentIds: [] }) },
      models: { list: async () => [] },
    },
    identity: {
      organizations: {
        list: async () => ({ items: [], total: 0 }),
        agentShares: { listShared: async () => [], setDisabledByMe: noop },
      },
    },
    userFavorites: {
      list: async (type: string) => { calls.push({ op: 'list', type }); return options?.favoriteRows ?? []; },
      add: async (type: string, id: string) => { calls.push({ op: 'add', type, id }); if (options?.addError) throw options.addError; },
      remove: async (type: string, id: string) => { calls.push({ op: 'remove', type, id }); if (options?.removeError) throw options.removeError; },
    },
  };
  return { client, calls };
}

const starOf = (agentId: string) => document.querySelector(`[data-agent-id="${agentId}"] .agent-favorite-star`) as HTMLElement | null;
const isFavorited = (agentId: string) => Boolean(starOf(agentId)?.classList.contains('is-favorited'));
// AgentsPage without an explicit tenantId hydrates under the null tenant segment.
const mirrorIds = () => readFavoriteIds(window.localStorage, 'user-1', null);
const settlePage = (ms = 15) => act(async () => { await new Promise((resolve) => setTimeout(resolve, ms)); });

test('favorites hydrate from the DB list and render the star state', async () => {
  window.localStorage.clear();
  const { client, calls } = favoritesSpyClient({ favoriteRows: [{ resource_id: 'a-own' }] });
  await mountToBody(React.createElement(AgentsPage, { client: client as never }));
  await settlePage(30);
  assert.deepEqual(calls.filter((c) => c.op === 'list'), [{ op: 'list', type: 'agent' }], 'GET /user/favorites?type=agent fires once');
  assert.ok(isFavorited('a-own'), 'the DB-favorited agent renders is-favorited');
  assert.deepEqual(mirrorIds(), ['a-own'], 'localStorage mirrors the DB set');
});

test('toggling the star writes through to add/remove with the right arguments', async () => {
  window.localStorage.clear();
  const { client, calls } = favoritesSpyClient({ favoriteRows: [{ resource_id: 'a-own' }] });
  await mountToBody(React.createElement(AgentsPage, { client: client as never }));
  await settlePage(30);
  // Unfavorite → DELETE /user/favorites/agent/a-own.
  await act(async () => { starOf('a-own')?.click(); });
  assert.deepEqual(calls.filter((c) => c.op !== 'list'), [{ op: 'remove', type: 'agent', id: 'a-own' }]);
  assert.equal(isFavorited('a-own'), false, 'optimistic unfavorite clears the star');
  assert.deepEqual(mirrorIds(), [], 'localStorage mirrors the removal');
  // Favorite again → POST /user/favorites {type:'agent', id}.
  await act(async () => { starOf('a-own')?.click(); });
  assert.deepEqual(calls.filter((c) => c.op !== 'list'), [
    { op: 'remove', type: 'agent', id: 'a-own' },
    { op: 'add', type: 'agent', id: 'a-own' },
  ]);
  assert.ok(isFavorited('a-own'), 'favorite restores the star');
});

test('a failed unfavorite rolls the star back (Vue useResourcePins semantics)', async () => {
  window.localStorage.clear();
  const { client, calls } = favoritesSpyClient({ favoriteRows: [{ resource_id: 'a-own' }], removeError: new Error('boom') });
  await mountToBody(React.createElement(AgentsPage, { client: client as never }));
  await settlePage(30);
  await act(async () => { starOf('a-own')?.click(); });
  await settlePage(10);
  assert.deepEqual(calls.filter((c) => c.op !== 'list'), [{ op: 'remove', type: 'agent', id: 'a-own' }]);
  assert.ok(isFavorited('a-own'), 'remove failure rolls the optimistic unfavorite back');
  assert.deepEqual(mirrorIds(), ['a-own'], 'localStorage mirrors the rollback');
});

test('a rapid second click inside one batch dispatches add after remove, not add twice', async () => {
  // Important-1 regression: the decision used to be captured as a setFavorites
  // updater side effect, which React only pre-evaluates when the fiber lanes
  // are empty — the second click in the same batch read a stale wasFavorited
  // (unfavorite dispatched add) and the DB kept the row after refresh.
  window.localStorage.clear();
  const { client, calls } = favoritesSpyClient({ favoriteRows: [{ resource_id: 'a-own' }] });
  await mountToBody(React.createElement(AgentsPage, { client: client as never }));
  await settlePage(30);
  assert.ok(isFavorited('a-own'), 'precondition: DB-favorited');
  await act(async () => {
    starOf('a-own')?.click(); // unfavorite
    starOf('a-own')?.click(); // favorite again — same batch, no commit between
  });
  await settlePage(10);
  assert.deepEqual(calls.filter((c) => c.op !== 'list'), [
    { op: 'remove', type: 'agent', id: 'a-own' },
    { op: 'add', type: 'agent', id: 'a-own' },
  ], 'second click reads the ref-mirrored set: remove then add, never add+add');
  assert.ok(isFavorited('a-own'), 'net state stays favorited');
});

// --- data loading (mocked client) ----------------------------------------------

test('loadAgentsPageData fetches agents, organizations and shared agents', async () => {
  const requests: Array<{ method?: string; path?: string }> = [];
  const client = {
    configuration: { agents: { listWithState: async () => { requests.push({ path: '/api/v1/agents' }); return { items: builtinAgents, disabledOwnAgentIds: ['builtin-quick-answer'] }; } } },
    identity: { organizations: {
      list: async () => { requests.push({ path: '/api/v1/organizations' }); return { items: [{ id: 'org-1', name: '空间一' }], total: 1 }; },
      agentShares: { listShared: async () => { requests.push({ path: '/api/v1/shared-agents' }); return [{ agent: { id: 'a-shared', name: '共享助手', is_builtin: false, config: {} }, share_id: 's-1', organization_id: 'org-1', org_name: '空间一', permission: 'viewer', source_tenant_id: 10001 }]; },
      setDisabledByMe: noop,
    } } },
  };
  const data = await loadAgentsPageData(client as never);
  assert.deepEqual(requests.map((r) => r.path), ['/api/v1/agents', '/api/v1/organizations', '/api/v1/shared-agents']);
  assert.equal(data.disabledOwnIds.length, 1);
  assert.equal(data.ownAgents.length, 4);
  assert.equal(data.organizations[0]?.name, '空间一');
  assert.equal(data.sharedAgents[0]?.sourceTenantId, 10001);
});

// --- full view anatomy -----------------------------------------------------------

test('page header shows agent title, subtitle and the sparkles create button', () => {
  const html = renderToStaticMarkup(React.createElement(AgentsPageView, baseViewProps));
  assert.match(html, /<h2[^>]*>智能体<\/h2>/);
  assert.match(html, /配置和管理您的智能体，自定义对话行为和能力/);
  assert.match(html, /创建智能体/);
  const populated = renderToStaticMarkup(React.createElement(AgentsPageView, {
    ...baseViewProps,
    isSectioned: false,
    flatCards: [{ id: 'agent-1', name: '助手', is_builtin: false, isMine: true }],
  }));
  assert.doesNotMatch(populated, /bg-\[#07c05f\]/, 'populated Vue list has no extra green text create button');

test('list-load failure renders no raw error payload (Vue parity: silent empty state)', () => {
  const html = renderToStaticMarkup(React.createElement(AgentsPageView, {
    ...baseViewProps,
    error: '{"message":"mock failure"}',
    flatCards: [],
    sections: [],
    isSectioned: false,
  }));
  assert.doesNotMatch(html, /mock failure/);
  assert.doesNotMatch(html, /wk-status.*error|Status tone=.error/, 'no error status surface');
});
  assert.match(html, /data-guide="agent-list-create"/);
  const viewerOnly = renderToStaticMarkup(React.createElement(AgentsPageView, { ...baseViewProps, viewer: { userId: 'u', isAdmin: false, isContributor: false }, canCreate: false }));
  assert.doesNotMatch(viewerOnly, /创建智能体/);
});

test('collapsed rail renders labels and keeps counts in the tooltip title', () => {
  const rail = renderToStaticMarkup(React.createElement(AgentRail, {
    t,
    items: [
      { key: 'all', label: t('common.all'), icon: 'layers', count: 6, active: true },
      { key: 'favorites', label: t('common.favorite'), icon: 'star', count: 0, active: false },
      { key: 'recents', label: t('agent.empty.recentsTitle'), icon: 'history', count: 2, active: false },
      { key: 'mine', label: '本空间', icon: 'workspace', count: 5, active: false },
      { key: 'org-1', label: '空间一', icon: 'space', count: 1, active: false },
    ],
    onSelect: noop,
  }));
  assert.match(rail, /全部/);
  assert.match(rail, /收藏/);
  assert.match(rail, /最近/);
  assert.match(rail, /本空间/);
  assert.match(rail, /空间一/);
  assert.match(rail, /icon-item-labeled/);
  assert.match(rail, /data-space-key="all"/);
  assert.doesNotMatch(rail, />6<|>0<|>2<|>5<|>1</, 'collapsed Vue rail does not show count rows');
});

test('all view renders grouped sections with counts and one card per agent', () => {
  const rows = fixtureRows();
  const sections = sectionize(rows, 'user-1');
  const html = renderToStaticMarkup(React.createElement(AgentsPageView, {
    ...baseViewProps,
    sections,
  }));
  assert.match(html, /内置/);
  assert.match(html, /我创建的/);
  assert.match(html, /共享给我 · 可编辑/);
  assert.match(html, /data-agent-section="builtin"/);
  const cards = html.match(/data-agent-id="/g);
  assert.equal(cards?.length, 6);
  assert.match(html, /快速问答/);
  assert.match(html, /智能推理/);
  assert.match(html, /我的助手/);
  assert.match(html, /共享助手/);
});

test('section collapse control uses the Vue chevron icon shape', () => {
  const sections = sectionize(fixtureRows(), 'user-1');
  const html = renderToStaticMarkup(React.createElement(AgentsPageView, {
    ...baseViewProps,
    sections,
  }));
  assert.match(html, /data-agent-section-toggle="builtin"[^>]*>.*<svg/s);
  assert.doesNotMatch(html, /data-agent-section-toggle="builtin"[^>]*>›</);
});

test('builtin cards stay inside the builtin section with mode gradients', () => {
  const rows = fixtureRows();
  const sections = sectionize(rows, 'user-1');
  const html = renderToStaticMarkup(React.createElement(AgentsPageView, { ...baseViewProps, sections }));
  const builtinIdx = html.indexOf('data-agent-section="builtin"');
  const mineIdx = html.indexOf('data-agent-section="mine"');
  const quickIdx = html.indexOf('data-agent-id="builtin-quick-answer"');
  const ownIdx = html.indexOf('data-agent-id="a-own"');
  assert.ok(builtinIdx > -1 && mineIdx > -1);
  assert.ok(quickIdx > builtinIdx && quickIdx < mineIdx, 'quick-answer card must render inside builtin section');
  assert.ok(ownIdx > mineIdx, 'own card must render inside mine section');
  assert.match(html, /agent-mode-normal/);
  assert.match(html, /agent-mode-agent/);
});

// --- card anatomy ------------------------------------------------------------------

test('card shows avatar, name, description fallback and capability chips with zh titles', () => {
  const agent = fixtureRows().find((row) => row.id === 'a-own')!;
  const html = renderToStaticMarkup(React.createElement(AgentCard, {
    agent, t, viewer: admin, favorited: false, menuOpen: false,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.match(html, /我的助手/);
  assert.match(html, /帮我写周报/);
  assert.match(html, /data-feature-badge="webSearch"/);
  assert.match(html, /data-feature-badge="multiTurn"/);
  assert.match(html, /data-feature-badge="modeNormal"/);
  assert.match(html, /aria-pressed="false"/);
  assert.match(html, /aria-haspopup="menu"/);
});

test('card falls back to 暂无描述 and shared cards carry the org source pill', () => {
  const shared = fixtureRows().find((row) => row.id === 'a-shared')!;
  const html = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: shared, t, viewer: admin, favorited: false, menuOpen: false,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.match(html, /暂无描述/);
  assert.match(html, /空间一/);
  assert.match(html, /wk-agent-card-source/);
});

test('corner badge rules suppress redundant pills when section headers show', () => {
  const rows = fixtureRows();
  const builtin = rows.find((row) => row.id === 'builtin-quick-answer')!;
  const own = rows.find((row) => row.id === 'a-own')!;
  assert.equal(cornerBadge(builtin, 'user-1'), null);
  assert.equal(cornerBadge(own, 'user-1'), null);
});

test('card menu lists 编辑/复制/停用/删除 per permission and hides delete for builtin', async () => {
  // tdesign Popup content 走 body portal —— 静态标记不可见，改 jsdom 挂载断言
  const menuText = async (agent: AgentCardModel) => {
    const root = await mountToBody(React.createElement(AgentCard, {
      agent, t, viewer: admin, favorited: false, menuOpen: true,
      onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
    }));
    await act(async () => { await Promise.resolve(); });
    return document.body.textContent ?? '';
  };
  const ownText = await menuText(fixtureRows().find((row) => row.id === 'a-own')!);
  assert.match(ownText, /编辑/);
  assert.match(ownText, /复制/);
  assert.match(ownText, /停用/);
  assert.match(ownText, /删除/);
  await act(async () => { pageTestRoot?.unmount(); });
  document.body.replaceChildren();

  const builtinText = await menuText(fixtureRows().find((row) => row.id === 'builtin-quick-answer')!);
  assert.match(builtinText, /编辑/);
  assert.equal(builtinText.includes('删除'), false);
  await act(async () => { pageTestRoot?.unmount(); });
  document.body.replaceChildren();

  const sharedText = await menuText(fixtureRows().find((row) => row.id === 'a-shared')!);
  assert.match(sharedText, /停用/);
  assert.equal(sharedText.includes('编辑'), false);
});

test('disabled own agents show the 已停用 tag', () => {
  const rows = buildAllViewRows([{ id: 'a-off', name: '停用助手', is_builtin: false, created_by: 'user-1', config: {} }], [], { userId: 'user-1', disabledOwnIds: ['a-off'] });
  const html = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: rows[0]!, t, viewer: admin, favorited: false, menuOpen: false,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.match(html, /已停用/);
});

// --- expert provenance badge (Octop M2) ----------------------------------------------

test('cards carry the expert provenance badge only when config.expert_source names an expert', () => {
  const rows = buildAllViewRows([
    { id: 'a-expert', name: '股票助手', is_builtin: false, created_by: 'user-1', config: { agent_mode: 'quick-answer', expert_source: { expert_id: 'stock-assistant', source: 'builtin', slug: '' } } },
  ], [], { userId: 'user-1', disabledOwnIds: [] });
  const html = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: rows[0]!, t, viewer: admin, favorited: false, menuOpen: false,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.match(html, /data-agent-expert-badge/);
  assert.match(html, /专家·stock-assistant/);

  const plain = fixtureRows().find((row) => row.id === 'a-own')!;
  const plainHtml = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: plain, t, viewer: admin, favorited: false, menuOpen: false,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.doesNotMatch(plainHtml, /data-agent-expert-badge/);
  assert.doesNotMatch(plainHtml, /专家·/);
});

test('expertSourceId tolerates absent, null and shape-drift expert_source values', () => {
  assert.equal(expertSourceId(undefined), '');
  assert.equal(expertSourceId({}), '');
  assert.equal(expertSourceId({ expert_source: null }), '');
  assert.equal(expertSourceId({ expert_source: 'builtin' }), '');
  assert.equal(expertSourceId({ expert_source: { expert_id: '', source: 'builtin' } }), '');
  assert.equal(expertSourceId({ expert_source: { expert_id: 'stock-assistant', source: 'builtin', slug: '' } }), 'stock-assistant');
});

// --- drawers / dialogs ---------------------------------------------------------------

test('shared detail drawer shows share scope summary and the use-in-chat action', () => {
  const shared = fixtureRows().find((row) => row.id === 'a-shared')!;
  const html = renderToStaticMarkup(React.createElement(AgentDetailDrawer, {
    kind: 'shared', agent: shared, t, onClose: noop, onUseInChat: noop,
  }));
  assert.match(html, /智能体详情/);
  assert.match(html, /共享范围说明/);
  assert.match(html, /全部知识库/);
  assert.match(html, /已配置/);
  assert.match(html, /开启/);
  assert.match(html, /在对话中使用/);
});

// basics editing moved into AgentEditorModal (agent-editor.test.tsx covers it);
// the drawer below is the shared-agent detail view only.
test('shared detail drawer no longer offers editable basics (replaced by AgentEditorModal)', () => {
  const own = fixtureRows().find((row) => row.id === 'a-own')!;
  const html = renderToStaticMarkup(React.createElement(AgentDetailDrawer, {
    kind: 'shared', agent: own, t, onClose: noop, onUseInChat: noop,
  }));
  assert.match(html, /在对话中使用/);
  assert.doesNotMatch(html, /name="name"/);
});

test('delete dialog names the agent and offers confirm/cancel', async () => {
  const own = fixtureRows().find((row) => row.id === 'a-own')!;
  await mountToBody(React.createElement(AgentDeleteDialog, {
    agent: own, t, busy: false, onConfirm: noop, onCancel: noop,
  }));
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  const text = document.body.textContent ?? '';
  assert.match(text, /删除智能体/);
  assert.match(text, /确定要删除智能体「我的助手」吗？此操作不可恢复。/);
  assert.match(text, /确认删除/);
  assert.match(text, /取消/);
});

// --- empty states ------------------------------------------------------------------------

test('empty states match the Vue variants per space', () => {
  const all = renderToStaticMarkup(React.createElement(AgentsPageView, { ...baseViewProps, sections: [] }));
  assert.match(all, /暂无自定义智能体/);
  assert.match(all, /点击右上角按钮创建您的第一个智能体/);

  const favs = renderToStaticMarkup(React.createElement(AgentsPageView, { ...baseViewProps, space: 'favorites', isSectioned: false }));
  assert.match(favs, /暂无收藏/);
  assert.match(favs, /在智能体卡片右上角点击星标即可收藏/);
  // Header create stays for contributors, but the favorites empty state has no CTA
  // (AgentList.vue: 空状态：收藏 / 最近 — 不放创建按钮).
  assert.doesNotMatch(favs, /wk-agent-empty-btn/);

  const recents = renderToStaticMarkup(React.createElement(AgentsPageView, { ...baseViewProps, space: 'recents', isSectioned: false }));
  assert.match(recents, /暂无最近访问/);
});

test('favorites view renders pinned cards flat without section headers', () => {
  const rows = fixtureRows().filter((row) => row.id === 'a-own' || row.id === 'builtin-quick-answer');
  const html = renderToStaticMarkup(React.createElement(AgentsPageView, {
    ...baseViewProps, space: 'favorites', isSectioned: false, flatCards: rows, favorites: new Set(['a-own']),
  }));
  assert.doesNotMatch(html, /data-agent-section=/);
  const cards = html.match(/data-agent-id="/g);
  assert.equal(cards?.length, 2);
  assert.match(html, /aria-pressed="true"/);
});

test('space view shows a loading spinner while shared agents fetch (Vue spaceAgentsLoading)', () => {  // AgentList.vue:498-500 — while listOrganizationSharedAgents is in flight the
  // space tab renders a centered spinner instead of the "no shared agents"
  // empty state, which only appears once loading settles (AgentList.vue:710).
  const loading = renderToStaticMarkup(React.createElement(AgentsPageView, {
    ...baseViewProps, space: 'org-1', sections: [], flatCards: [], spaceLoading: true,
  }));
  assert.match(loading, /agent-list-main-loading/);
  assert.match(loading, /t-loading/);
  assert.doesNotMatch(loading, /暂无共享智能体/);

  const settled = renderToStaticMarkup(React.createElement(AgentsPageView, {
    ...baseViewProps, space: 'org-1', sections: [], flatCards: [], spaceLoading: false,
  }));
  assert.doesNotMatch(settled, /t-loading/);
  assert.match(settled, /暂无共享智能体/);
});

test('space view cards show no org source pill (Vue card-bottom carries only badges)', () => {
  // AgentList.vue space-tab card (546-635) ends with .feature-badges only —
  // the org source pill is rendered exclusively on "all"-view shared cards.
  const mineShared = buildSpaceViewRows([
    { agent: { id: 'a-mine', name: '我的助手', is_builtin: false, config: {} }, share_id: 's-mine', organization_id: 'org-1', org_name: '空间一', permission: 'editor', source_tenant_id: 10001, disabled_by_me: false, is_mine: true },
  ])[0]!;
  const html = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: mineShared, t, viewer: admin, favorited: false, menuOpen: false,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.doesNotMatch(html, /空间一/);

  const otherShared = fixtureRows().find((row) => row.id === 'a-shared')!;
  const otherHtml = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: otherShared, t, viewer: admin, favorited: false, menuOpen: false,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.match(otherHtml, /空间一/);
});
