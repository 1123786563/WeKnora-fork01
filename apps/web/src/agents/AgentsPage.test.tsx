import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import type { AgentCardModel, AgentViewer } from './list.ts';

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
      { key: 'mine', label: t('common.mine'), icon: 'workspace', count: 5, active: false },
      { key: 'org-1', label: '空间一', icon: 'space', count: 1, active: false },
    ],
    onSelect: noop,
  }));
  assert.match(rail, /全部/);
  assert.match(rail, /收藏/);
  assert.match(rail, /最近/);
  assert.match(rail, /我的/);
  assert.match(rail, /空间一/);
  assert.match(rail, /aria-current="true"/);
  assert.match(rail, /data-space-key="all"/);
  assert.match(rail, /title="全部 \(6\)"/);
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
  assert.match(html, /title="支持网络搜索"/);
  assert.match(html, /title="多轮对话"/);
  assert.match(html, /title="快速问答"/);
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

test('card menu lists 编辑/复制/停用/删除 per permission and hides delete for builtin', () => {
  const own = fixtureRows().find((row) => row.id === 'a-own')!;
  const ownHtml = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: own, t, viewer: admin, favorited: false, menuOpen: true,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.match(ownHtml, /编辑/);
  assert.match(ownHtml, /复制/);
  assert.match(ownHtml, /停用/);
  assert.match(ownHtml, /删除/);

  const builtin = fixtureRows().find((row) => row.id === 'builtin-quick-answer')!;
  const builtinHtml = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: builtin, t, viewer: admin, favorited: false, menuOpen: true,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.match(builtinHtml, /编辑/);
  assert.doesNotMatch(builtinHtml, /删除/);

  const shared = fixtureRows().find((row) => row.id === 'a-shared')!;
  const sharedHtml = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: shared, t, viewer: admin, favorited: false, menuOpen: true,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.match(sharedHtml, /停用/);
  assert.doesNotMatch(sharedHtml, /编辑/);
});

test('disabled own agents show the 已停用 tag', () => {
  const rows = buildAllViewRows([{ id: 'a-off', name: '停用助手', is_builtin: false, created_by: 'user-1', config: {} }], [], { userId: 'user-1', disabledOwnIds: ['a-off'] });
  const html = renderToStaticMarkup(React.createElement(AgentCard, {
    agent: rows[0]!, t, viewer: admin, favorited: false, menuOpen: false,
    onOpen: noop, onToggleFavorite: noop, onToggleMenu: noop, onMenuAction: noop,
  }));
  assert.match(html, /已停用/);
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

test('delete dialog names the agent and offers confirm/cancel', () => {
  const own = fixtureRows().find((row) => row.id === 'a-own')!;
  const html = renderToStaticMarkup(React.createElement(AgentDeleteDialog, {
    agent: own, t, busy: false, onConfirm: noop, onCancel: noop,
  }));
  assert.match(html, /删除智能体/);
  assert.match(html, /确定要删除智能体「我的助手」吗？此操作不可恢复。/);
  assert.match(html, /确认删除/);
  assert.match(html, /取消/);
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
  assert.match(loading, /animate-spin/);
  assert.doesNotMatch(loading, /暂无共享智能体/);

  const settled = renderToStaticMarkup(React.createElement(AgentsPageView, {
    ...baseViewProps, space: 'org-1', sections: [], flatCards: [], spaceLoading: false,
  }));
  assert.doesNotMatch(settled, /animate-spin/);
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
