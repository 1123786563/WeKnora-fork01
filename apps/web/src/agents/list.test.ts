import assert from 'node:assert/strict';
import test from 'node:test';

import {
  AGENT_SECTION_ORDER,
  agentSectionLabelKey,
  agentSectionOf,
  avatarGradient,
  avatarLetter,
  buildAllViewRows,
  buildMineViewRows,
  buildSpaceViewRows,
  canManageAgent,
  cardActions,
  chatNavigationPath,
  cornerBadge,
  favoritesStorageKey,
  featureBadges,
  isMyAgent,
  isSharedAgentEditable,
  kbScope,
  mcpScope,
  opensEditorOnCardClick,
  readAgentRecents,
  readFavoriteIds,
  recentsStorageKey,
  sectionize,
  sharedAgentsFromRecords,
  sharedCountByOrg,
  toggleFavoriteId,
  touchAgentRecent,
  type AgentCardModel,
  type PinEntry,
} from './list.ts';

const userId = 'user-1';

const builtinQuick = { id: 'builtin-quick-answer', name: '快速问答', is_builtin: true, config: { agent_mode: 'quick-answer' } };
const builtinSmart = { id: 'builtin-smart-reasoning', name: '智能推理', is_builtin: true, config: { agent_mode: 'smart-reasoning' } };
const ownAgent = { id: 'a-own', name: '我的助手', is_builtin: false, created_by: userId, config: { web_search_enabled: true, multi_turn_enabled: true } };
const teammateAgent = { id: 'a-mate', name: '同事助手', is_builtin: false, created_by: 'user-2', config: {} };

function sharedRow(overrides: Record<string, unknown> = {}) {
  return {
    agent: { id: 'a-shared', name: '共享助手', description: '', is_builtin: false, config: { kb_selection_mode: 'all' } },
    share_id: 'share-1',
    organization_id: 'org-1',
    org_name: '空间一',
    permission: 'viewer',
    source_tenant_id: 10001,
    disabled_by_me: false,
    ...overrides,
  };
}

// --- grouping: "all" view mirrors AgentList.vue filteredAgents ---------------

test('all view orders builtin → my agents → teammates → shared (editable first)', () => {
  const shared = sharedAgentsFromRecords([
    sharedRow({ share_id: 's-view', permission: 'viewer' }),
    sharedRow({ share_id: 's-edit', permission: 'editor', agent: { id: 'a-shared-2', name: '可编辑', is_builtin: false, config: {} } }),
  ]);
  const rows = buildAllViewRows([ownAgent, teammateAgent, builtinSmart, builtinQuick], shared, { userId, disabledOwnIds: [] });
  // Buckets keep server order (AgentList.vue pushes in forEach order); builtin first.
  assert.deepEqual(rows.map((row) => row.id), [
    'builtin-smart-reasoning',
    'builtin-quick-answer',
    'a-own',
    'a-mate',
    'a-shared-2',
    'a-shared',
  ]);
  assert.equal(rows[0]?.isMine, true);
  assert.equal(rows[4]?.isMine, false);
  assert.equal(rows[4]?.permission, 'editor');
  assert.equal(rows[5]?.orgName, '空间一');
});

test('all view flags disabled-by-me own agents from the list envelope', () => {
  const rows = buildAllViewRows([ownAgent], [], { userId, disabledOwnIds: ['a-own'] });
  assert.equal(rows[0]?.disabledByMe, true);
});

// --- grouping: "mine" view ---------------------------------------------------

test('mine view keeps builtin → own → teammate order and marks everything own-tenant', () => {
  const rows = buildMineViewRows([teammateAgent, builtinQuick, ownAgent], { userId, disabledOwnIds: [] });
  assert.deepEqual(rows.map((row) => row.id), ['builtin-quick-answer', 'a-own', 'a-mate']);
  assert.ok(rows.every((row) => row.isMine));
});

// --- sections ----------------------------------------------------------------

test('agentSectionOf buckets rows like AgentList.vue agentSectionOf', () => {
  const rows = buildAllViewRows([ownAgent, teammateAgent, builtinQuick], sharedAgentsFromRecords([sharedRow()]), { userId, disabledOwnIds: [] });
  assert.equal(agentSectionOf(rows[0]!, userId), 'builtin');
  assert.equal(agentSectionOf(rows[1]!, userId), 'mine');
  assert.equal(agentSectionOf(rows[2]!, userId), 'tenantOthers');
  assert.equal(agentSectionOf(rows[3]!, userId), 'sharedReadonly');
});

test('sectionize emits non-empty groups in fixed order with counts', () => {
  const rows = buildAllViewRows([ownAgent, builtinQuick], sharedAgentsFromRecords([sharedRow()]), { userId, disabledOwnIds: [] });
  const sections = sectionize(rows, userId);
  assert.deepEqual(sections.map((s) => s.key), ['builtin', 'mine', 'sharedReadonly']);
  assert.deepEqual(sections.map((s) => s.count), [1, 1, 1]);
  assert.deepEqual(AGENT_SECTION_ORDER, ['builtin', 'mine', 'tenantOthers', 'sharedByMe', 'sharedEditable', 'sharedReadonly']);
});

test('tenant section label flips between admin and viewer wording', () => {
  assert.equal(agentSectionLabelKey('tenantOthers', { isAdmin: true }), 'agent.sections.tenantOthers');
  assert.equal(agentSectionLabelKey('tenantOthers', { isAdmin: false }), 'agent.sections.tenantReadonly');
  assert.equal(agentSectionLabelKey('builtin', { isAdmin: false }), 'agent.sections.builtin');
});

// --- space view --------------------------------------------------------------

test('space view puts my shared agents first, then editable-before-readonly', () => {
  const rows = buildSpaceViewRows([
    sharedRow({ share_id: 's-mine', is_mine: true, permission: 'editor' }),
    sharedRow({ share_id: 's-view', is_mine: false, permission: 'viewer' }),
    sharedRow({ share_id: 's-edit', is_mine: false, permission: 'editor', agent: { id: 'a-2', name: 'B', is_builtin: false, config: {} } }),
  ]);
  assert.deepEqual(rows.map((row) => row.shareId), ['s-mine', 's-edit', 's-view']);
  assert.equal(agentSectionOf(rows[0]!, userId), 'sharedByMe');
});

test('space view sharedByMe cards follow the Vue click + menu contract', () => {
  // AgentList.vue handleSpaceAgentCardClick (1286-1292): is_mine rows open the
  // editor; only other people's shared rows open the readonly detail drawer.
  const mineShared = buildSpaceViewRows([sharedRow({ share_id: 's-mine', is_mine: true, permission: 'editor' })])[0]!;
  const otherShared = buildAllViewRows([], sharedAgentsFromRecords([sharedRow()]), { userId, disabledOwnIds: [] })[0]!;
  const own = buildAllViewRows([ownAgent], [], { userId, disabledOwnIds: [] })[0]!;
  assert.equal(opensEditorOnCardClick(mineShared), true);
  assert.equal(opensEditorOnCardClick(own), true);
  assert.equal(opensEditorOnCardClick(otherShared), false);
  // AgentList.vue:572 gates the space-view popup on !shared.is_mine — my own
  // shared rows carry no card menu, even for admins.
  const admin = { userId, isAdmin: true, isContributor: true };
  const contributor = { userId, isAdmin: false, isContributor: true };
  assert.deepEqual(cardActions(mineShared, admin), []);
  assert.deepEqual(cardActions(mineShared, contributor), []);
});

test('sharedAgentsFromRecords skips malformed rows and normalizes fields', () => {
  const rows = sharedAgentsFromRecords([sharedRow({ source_tenant_id: '10001', disabled_by_me: true }), { broken: true } as unknown as Record<string, unknown>]);
  assert.equal(rows.length, 1);
  assert.equal(rows[0]?.sourceTenantId, 10001);
  assert.equal(rows[0]?.disabledByMe, true);
});

test('sharedCountByOrg counts shared agents per organization id', () => {
  const shared = sharedAgentsFromRecords([sharedRow(), sharedRow({ organization_id: 'org-1' }), sharedRow({ organization_id: 'org-2', share_id: 's-2' })]);
  assert.deepEqual(sharedCountByOrg(shared), { 'org-1': 2, 'org-2': 1 });
});

// --- permissions / card menu -------------------------------------------------

test('canManageAgent mirrors OwnedAgentOrAdmin: creator or admin', () => {
  assert.equal(canManageAgent({ created_by: userId } as AgentCardModel, { userId, isAdmin: false }), true);
  assert.equal(canManageAgent({ created_by: 'user-2' } as AgentCardModel, { userId, isAdmin: false }), false);
  assert.equal(canManageAgent({ created_by: 'user-2' } as AgentCardModel, { userId, isAdmin: true }), true);
  assert.equal(canManageAgent({ created_by: '' } as AgentCardModel, { userId, isAdmin: false }), false);
  assert.ok(isMyAgent({ created_by: userId }, userId));
  assert.ok(!isMyAgent({ created_by: '' }, userId));
  assert.equal(isSharedAgentEditable('editor'), true);
  assert.equal(isSharedAgentEditable('viewer'), false);
  assert.equal(isSharedAgentEditable(undefined), false);
});

test('cardActions match the Vue popup menu permission gates', () => {
  const contributor = { userId, isAdmin: false, isContributor: true };
  const admin = { userId, isAdmin: true, isContributor: true };
  const own = buildAllViewRows([ownAgent], [], { userId, disabledOwnIds: [] })[0]!;
  assert.deepEqual(cardActions(own, contributor), ['edit', 'copy', 'delete']);
  assert.deepEqual(cardActions(own, admin), ['edit', 'copy', 'toggle', 'delete']);
  const builtin = buildAllViewRows([builtinQuick], [], { userId, disabledOwnIds: [] })[0]!;
  assert.deepEqual(cardActions(builtin, admin), ['edit', 'copy', 'toggle']);
  const shared = buildAllViewRows([], sharedAgentsFromRecords([sharedRow()]), { userId, disabledOwnIds: [] })[0]!;
  assert.deepEqual(cardActions(shared, admin), ['toggle']);
  assert.deepEqual(cardActions(shared, contributor), []);
});

// --- corner badges (ResourceOriginBadge + card-list-badge suppression rules) --

test('cornerBadge follows the section-header suppression rules', () => {
  const rows = buildAllViewRows(
    [builtinQuick, ownAgent, { ...teammateAgent, creator_name: '张三' }, { ...teammateAgent, id: 'a-mate-2', creator_name: undefined }],
    sharedAgentsFromRecords([sharedRow()]),
    { userId, disabledOwnIds: [] },
  );
  const builtin = rows.find((row) => row.id === 'builtin-quick-answer')!;
  const own = rows.find((row) => row.id === 'a-own')!;
  const named = rows.find((row) => row.id === 'a-mate')!;
  // Section headers are always shown in AgentList, so builtin/mine pills are redundant.
  assert.equal(cornerBadge(builtin, userId), null);
  assert.equal(cornerBadge(own, userId), null);
  // tenantOthers creator badge shows the creator name when the backend provides one.
  assert.deepEqual(cornerBadge(named, userId), { kind: 'creator', name: '张三' });
  // Without a creator name the header alone carries the information.
  const unnamed = rows.find((row) => row.id === 'a-mate-2')!;
  assert.equal(cornerBadge(unnamed, userId), null);
  // Shared cards never get a corner badge — they render the org source pill instead.
  const shared = rows.find((row) => row.id === 'a-shared')!;
  assert.equal(cornerBadge(shared, userId), null);
});

// --- feature badges ----------------------------------------------------------

test('featureBadges reflect agent config like the Vue card badges', () => {
  assert.deepEqual(featureBadges({ config: { agent_mode: 'quick-answer' } } as AgentCardModel), ['modeNormal']);
  assert.deepEqual(
    featureBadges({
      config: {
        agent_mode: 'smart-reasoning',
        web_search_enabled: true,
        knowledge_bases: ['kb-1'],
        mcp_selection_mode: 'all',
        multi_turn_enabled: true,
      },
    } as AgentCardModel),
    ['modeAgent', 'webSearch', 'knowledge', 'mcp', 'multiTurn'],
  );
  assert.deepEqual(featureBadges({ config: { kb_selection_mode: 'all' } } as AgentCardModel), ['modePlain', 'knowledge']);
});

// --- avatar ------------------------------------------------------------------

test('avatar gradient is stable per name and letter matches the Vue component', () => {
  assert.deepEqual(avatarGradient('快速问答'), avatarGradient('快速问答'));
  // Verified against the verbatim Vue AgentAvatar.vue hashCode: index 10 for 快速问答.
  assert.deepEqual(avatarGradient('快速问答'), { from: '#614385', to: '#516395' });
  assert.equal(avatarLetter('abc'), 'A');
  assert.equal(avatarLetter('智'), '智');
  assert.equal(avatarLetter(''), '?');
});

// --- share scope / chat navigation -------------------------------------------

test('kbScope and mcpScope map selection modes to drawer summary values', () => {
  assert.deepEqual(kbScope({ kb_selection_mode: 'all' }), { kind: 'all' });
  assert.deepEqual(kbScope({ kb_selection_mode: 'selected', knowledge_bases: ['a', 'b'] }), { kind: 'selected', count: 2 });
  assert.deepEqual(kbScope({ kb_selection_mode: 'selected' }), { kind: 'none' });
  assert.deepEqual(kbScope({}), { kind: 'none' });
  assert.deepEqual(mcpScope({ mcp_selection_mode: 'all' }), { kind: 'all' });
  assert.deepEqual(mcpScope({ mcp_services: ['m1'], mcp_selection_mode: 'selected' }), { kind: 'selected', count: 1 });
  assert.deepEqual(mcpScope({}), { kind: 'none' });
});

test('chatNavigationPath builds the Vue creatChat agent URL shape', () => {
  const own = buildAllViewRows([ownAgent], [], { userId, disabledOwnIds: [] })[0]!;
  assert.equal(chatNavigationPath(own), '/platform/creatChat?agent_id=a-own');
  const shared = sharedAgentsFromRecords([sharedRow()])[0]!;
  const sharedCard = buildAllViewRows([], [shared], { userId, disabledOwnIds: [] })[0]!;
  assert.equal(chatNavigationPath(sharedCard), '/platform/creatChat?agent_id=a-shared&source_tenant_id=10001');
});

// --- pins: recents (localStorage, Vue key shape) + favorites ------------------

function fakeStorage(initial: Record<string, string> = {}) {
  const map = new Map(Object.entries(initial));
  return {
    getItem: (key: string) => (map.has(key) ? (map.get(key) as string) : null),
    setItem: (key: string, value: string) => void map.set(key, value),
  };
}

test('recents key is scoped per user and tenant like Vue useResourcePins', () => {
  assert.equal(recentsStorageKey('u1', '10000'), 'WeKnora_u1_t10000_resource_recents');
  assert.equal(recentsStorageKey('u1', null), 'WeKnora_u1_resource_recents');
  assert.equal(favoritesStorageKey('u1', '10000'), 'WeKnora_u1_t10000_agent_favorites');
});

test('readAgentRecents reads, validates and sorts the Vue PinEntry list', () => {
  const storage = fakeStorage({
    WeKnora_u1_t1_resource_recents: JSON.stringify([
      { type: 'kb', id: 'kb-1', ts: 5 },
      { type: 'agent', id: 'agent-old', ts: 10 },
      { type: 'agent', id: 'agent-new', ts: 20 },
      { broken: true },
      'junk',
    ]),
  });
  const entries = readAgentRecents(storage as Storage, 'u1', '1');
  assert.deepEqual(entries.map((e) => e.id), ['agent-new', 'agent-old']);
  assert.equal(entries[0]?.type, 'agent');
});

test('touchAgentRecent dedupes, prepends and caps at 30', () => {
  const base: PinEntry[] = [{ type: 'agent', id: 'a1', ts: 1 }, { type: 'agent', id: 'a2', ts: 2 }];
  const next = touchAgentRecent(base, 'a1', 99);
  assert.deepEqual(next.map((e) => e.id), ['a1', 'a2']);
  assert.equal(next[0]?.ts, 99);
  let grown: PinEntry[] = [];
  for (let i = 0; i < 35; i++) grown = touchAgentRecent(grown, 'id-' + i, i);
  assert.equal(grown.length, 30);
  assert.equal(grown[0]?.id, 'id-34');
});

test('favorites toggle and persistence helpers round-trip per user+tenant', () => {
  const storage = fakeStorage();
  assert.deepEqual(readFavoriteIds(storage as Storage, 'u1', 't1'), []);
  const next = toggleFavoriteId(readFavoriteIds(storage as Storage, 'u1', 't1'), 'agent-9');
  assert.deepEqual(next, ['agent-9']);
  assert.deepEqual(toggleFavoriteId(next, 'agent-9'), []);
  // Newest favorite first, matching the Vue ts-desc favorites ordering.
  assert.deepEqual(toggleFavoriteId(next, 'agent-8'), ['agent-8', 'agent-9']);
});
