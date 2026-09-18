/**
 * Pure helpers for the React agents list page (parity port of
 * frontend/src/views/agent/AgentList.vue + useResourcePins.ts).
 *
 * No React / DOM here — everything is data-in / data-out so the list
 * semantics (grouping order, permission gates, pin storage, URL shapes)
 * can be tested without a renderer.
 */

export type AgentMode = 'quick-answer' | 'smart-reasoning';

/** Subset of CustomAgentConfig (frontend/src/api/agent/index.ts) the list needs. */
export interface AgentListConfig {
  agent_mode?: AgentMode;
  web_search_enabled?: boolean;
  knowledge_bases?: string[];
  kb_selection_mode?: 'all' | 'selected' | 'none';
  mcp_services?: string[];
  mcp_selection_mode?: 'all' | 'selected' | 'none';
  multi_turn_enabled?: boolean;
  model_id?: string;
  rerank_model_id?: string;
  [key: string]: unknown;
}

export interface AgentViewer { userId: string; isAdmin: boolean; isContributor: boolean }

/** One renderable agent card; mirrors AgentList.vue's DisplayAgent union. */
export interface AgentCardModel {
  id: string;
  name: string;
  description?: string;
  avatar?: string;
  is_builtin: boolean;
  created_by?: string;
  creator_name?: string;
  config?: AgentListConfig;
  created_at?: string;
  updated_at?: string;
  /** Own-tenant agent (the agent list endpoint) — shared entries are false. */
  isMine: boolean;
  /** Own agent currently disabled for this tenant's chat dropdown. */
  disabledByMe?: boolean;
  // Shared-only fields (AgentList.vue DisplayAgent shared branch).
  shareId?: string;
  orgName?: string;
  sourceTenantId?: number;
  permission?: string;
  /** Space view: this agent was shared into the org BY me. */
  sharedByMe?: boolean;
  [key: string]: unknown;
}

export type AgentSectionKey = 'builtin' | 'mine' | 'tenantOthers' | 'sharedByMe' | 'sharedEditable' | 'sharedReadonly';

export const AGENT_SECTION_ORDER: readonly AgentSectionKey[] = ['builtin', 'mine', 'tenantOthers', 'sharedByMe', 'sharedEditable', 'sharedReadonly'];

export const SECTION_ICON_KEYS: Record<AgentSectionKey, string> = {
  builtin: 'app',
  mine: 'user',
  tenantOthers: 'usergroup',
  sharedByMe: 'share',
  sharedEditable: 'usergroup-add',
  sharedReadonly: 'usergroup-add',
};

/** agent.sections.* label key; the tenant bucket flips per role (AgentList.vue:1405). */
export function agentSectionLabelKey(key: AgentSectionKey, viewer: { isAdmin: boolean }): string {
  switch (key) {
    case 'builtin': return 'agent.sections.builtin';
    case 'mine': return 'agent.sections.mine';
    case 'tenantOthers': return viewer.isAdmin ? 'agent.sections.tenantOthers' : 'agent.sections.tenantReadonly';
    case 'sharedByMe': return 'agent.sections.sharedByMe';
    case 'sharedEditable': return 'agent.sections.sharedEditable';
    case 'sharedReadonly': return 'agent.sections.sharedReadonly';
  }
}

// --- ownership / permission gates (AgentList.vue:1350-1396) ------------------

export function isMyAgent(agent: { created_by?: string }, userId: string): boolean {
  return !!(agent.created_by && userId && agent.created_by === userId);
}

/** Mirrors the server-side OwnedAgentOrAdmin guard: creator, else Admin+. */
export function canManageAgent(agent: { created_by?: string }, viewer: { userId: string; isAdmin: boolean }): boolean {
  if (isMyAgent(agent, viewer.userId)) return true;
  return viewer.isAdmin;
}

const AGENT_EDITABLE_PERMS = new Set(['admin', 'editor']);
export function isSharedAgentEditable(perm?: string): boolean {
  return !!perm && AGENT_EDITABLE_PERMS.has(perm);
}

export function agentSectionOf(agent: AgentCardModel, userId: string): AgentSectionKey {
  if (agent.is_builtin === true) return 'builtin';
  if (agent.sharedByMe) return 'sharedByMe';
  if (agent.isMine === false) return isSharedAgentEditable(agent.permission) ? 'sharedEditable' : 'sharedReadonly';
  return isMyAgent(agent, userId) ? 'mine' : 'tenantOthers';
}

interface RawAgent {
  id?: unknown; name?: unknown; description?: unknown; avatar?: unknown;
  is_builtin?: unknown; created_by?: unknown; creator_name?: unknown;
  config?: unknown; created_at?: unknown; updated_at?: unknown;
  [key: string]: unknown;
}

function asString(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

function normalizeConfig(value: unknown): AgentListConfig | undefined {
  return value && typeof value === 'object' ? (value as AgentListConfig) : undefined;
}

function ownCard(agent: RawAgent, disabledOwnIds: string[]): AgentCardModel {
  const id = asString(agent.id) ?? '';
  return {
    ...(agent as Record<string, unknown>),
    id,
    name: asString(agent.name) ?? '',
    description: asString(agent.description),
    avatar: asString(agent.avatar),
    is_builtin: agent.is_builtin === true,
    created_by: asString(agent.created_by),
    creator_name: asString(agent.creator_name),
    config: normalizeConfig(agent.config),
    isMine: true,
    disabledByMe: disabledOwnIds.includes(id),
  };
}

function splitOwnTenant(agents: ReadonlyArray<RawAgent>, userId: string): { builtin: RawAgent[]; own: RawAgent[]; teammate: RawAgent[] } {
  const builtin: RawAgent[] = [];
  const own: RawAgent[] = [];
  const teammate: RawAgent[] = [];
  for (const agent of agents) {
    if (agent.is_builtin === true) builtin.push(agent);
    else if (isMyAgent(agent as { created_by?: string }, userId)) own.push(agent);
    else teammate.push(agent);
  }
  return { builtin, own, teammate };
}

export interface SharedAgentSummary {
  agent: RawAgent;
  shareId: string;
  organizationId: string;
  orgName: string;
  permission: string;
  sourceTenantId: number;
  disabledByMe: boolean;
  /** Row-level is_mine from GET /organizations/:id/shared-agents (space view). */
  isMine: boolean;
}

/** Normalize the JsonRecord[] rows returned by agentShares.listShared(). */
export function sharedAgentsFromRecords(records: ReadonlyArray<Record<string, unknown>>): SharedAgentSummary[] {
  const rows: SharedAgentSummary[] = [];
  for (const record of records) {
    const agent = record.agent;
    if (!agent || typeof agent !== 'object') continue;
    const source = record.source_tenant_id;
    rows.push({
      agent: agent as RawAgent,
      shareId: asString(record.share_id) ?? '',
      organizationId: asString(record.organization_id) ?? '',
      orgName: asString(record.org_name) ?? '',
      permission: asString(record.permission) ?? '',
      sourceTenantId: typeof source === 'number' ? source : Number(source) || 0,
      disabledByMe: record.disabled_by_me === true,
      isMine: record.is_mine === true,
    });
  }
  return rows;
}

function sharedCard(shared: SharedAgentSummary, sharedByMe: boolean): AgentCardModel {
  return {
    ...(shared.agent as Record<string, unknown>),
    id: asString(shared.agent.id) ?? '',
    name: asString(shared.agent.name) ?? '',
    description: asString(shared.agent.description),
    avatar: asString(shared.agent.avatar),
    is_builtin: shared.agent.is_builtin === true,
    created_by: asString(shared.agent.created_by),
    creator_name: asString(shared.agent.creator_name),
    config: normalizeConfig(shared.agent.config),
    isMine: false,
    disabledByMe: shared.disabledByMe,
    shareId: shared.shareId,
    orgName: shared.orgName,
    sourceTenantId: shared.sourceTenantId,
    permission: shared.permission,
    sharedByMe,
  };
}

/** "all" view — AgentList.vue filteredAgents: builtin → own → teammate → shared (editable first). */
export function buildAllViewRows(
  ownAgents: ReadonlyArray<RawAgent>,
  sharedAgents: ReadonlyArray<SharedAgentSummary>,
  options: { userId: string; disabledOwnIds: string[] },
): AgentCardModel[] {
  const { userId, disabledOwnIds } = options;
  const { builtin, own, teammate } = splitOwnTenant(ownAgents, userId);
  const rows: AgentCardModel[] = [
    ...[...builtin, ...own, ...teammate].map((agent) => ownCard(agent, disabledOwnIds)),
  ];
  const sortedShared = [...sharedAgents].sort(
    (a, b) => (isSharedAgentEditable(a.permission) ? 0 : 1) - (isSharedAgentEditable(b.permission) ? 0 : 1),
  );
  for (const shared of sortedShared) rows.push(sharedCard(shared, false));
  return rows;
}

/** "mine" view — AgentList.vue sortedMineAgents: builtin → own → teammate. */
export function buildMineViewRows(
  ownAgents: ReadonlyArray<RawAgent>,
  options: { userId: string; disabledOwnIds: string[] },
): AgentCardModel[] {
  const { userId, disabledOwnIds } = options;
  const { builtin, own, teammate } = splitOwnTenant(ownAgents, userId);
  return [...builtin, ...own, ...teammate].map((agent) => ownCard(agent, disabledOwnIds));
}

/** Space view — GET /organizations/:id/shared-agents rows: is_mine first, then editable first. */
export function buildSpaceViewRows(items: ReadonlyArray<Record<string, unknown>>): AgentCardModel[] {
  const rows = sharedAgentsFromRecords(items).map((shared) => sharedCard(shared, shared.isMine));
  return rows.sort((a, b) => {
    const aMine = a.sharedByMe ? 0 : 1;
    const bMine = b.sharedByMe ? 0 : 1;
    if (aMine !== bMine) return aMine - bMine;
    return (isSharedAgentEditable(a.permission) ? 0 : 1) - (isSharedAgentEditable(b.permission) ? 0 : 1);
  });
}

export interface AgentSectionView { key: AgentSectionKey; count: number; cards: AgentCardModel[] }

/** Non-empty sections in AGENT_SECTION_ORDER, preserving card order inside each. */
export function sectionize(rows: ReadonlyArray<AgentCardModel>, userId: string): AgentSectionView[] {
  const byKey = new Map<AgentSectionKey, AgentCardModel[]>();
  for (const row of rows) {
    const key = agentSectionOf(row, userId);
    const bucket = byKey.get(key) ?? [];
    bucket.push(row);
    byKey.set(key, bucket);
  }
  return AGENT_SECTION_ORDER
    .filter((key) => (byKey.get(key)?.length ?? 0) > 0)
    .map((key) => ({ key, count: byKey.get(key)!.length, cards: byKey.get(key)! }));
}

/** Shared-agent count per organization id, for the space rail entries. */
export function sharedCountByOrg(shared: ReadonlyArray<SharedAgentSummary>): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const row of shared) {
    if (!row.organizationId) continue;
    counts[row.organizationId] = (counts[row.organizationId] ?? 0) + 1;
  }
  return counts;
}

// --- card menu (AgentList.vue popup-menu permission gates) --------------------

export type AgentCardAction = 'edit' | 'copy' | 'toggle' | 'delete';

export function cardActions(agent: AgentCardModel, viewer: AgentViewer): AgentCardAction[] {
  if (!agent.isMine) {
    // Space-view rows shared BY me carry no popup menu at all
    // (AgentList.vue:572 gates the popup on !shared.is_mine).
    if (agent.sharedByMe) return [];
    // Shared cards: admins get the enable/disable toggle only.
    return viewer.isAdmin ? ['toggle'] : [];
  }
  const actions: AgentCardAction[] = [];
  if (canManageAgent(agent, viewer)) actions.push('edit');
  if (viewer.isContributor) actions.push('copy');
  if (viewer.isAdmin) actions.push('toggle');
  if (!agent.is_builtin && canManageAgent(agent, viewer)) actions.push('delete');
  return actions;
}

/**
 * Card click destination (AgentList.vue handleCardClick vs
 * handleSpaceAgentCardClick): own agents and space-view "shared by me" rows
 * open the editor; every other shared row opens the readonly detail drawer.
 */
export function opensEditorOnCardClick(agent: Pick<AgentCardModel, 'isMine' | 'sharedByMe'>): boolean {
  return agent.isMine || agent.sharedByMe === true;
}

// --- corner badges (AgentList.vue card-bottom-right + card-list-badge rules) --

export type CornerBadge = { kind: 'builtin' } | { kind: 'creator'; name: string } | null;

/**
 * Port of the Vue badge chain: shared cards render the org source pill in the
 * card component; own cards show a builtin / creator pill only when the
 * section header does not already carry the information
 * (frontend/src/utils/card-list-badge.ts with showSectionHeaders: true).
 */
export function cornerBadge(agent: AgentCardModel, userId: string, showSectionHeaders = true): CornerBadge {
  if (!agent.isMine) return null;
  if (showSectionHeaders) {
    const section = agentSectionOf(agent, userId);
    if (section === 'builtin' || section === 'mine') return null;
    if (section === 'tenantOthers') {
      const name = (agent.creator_name ?? '').trim();
      return name ? { kind: 'creator', name } : null;
    }
  }
  return agent.is_builtin ? { kind: 'builtin' } : null;
}

// --- feature badges (AgentList.vue card-bottom .feature-badge conditions) -----

export type FeatureBadgeKey = 'modeNormal' | 'modeAgent' | 'webSearch' | 'knowledge' | 'mcp' | 'multiTurn';

export function featureBadges(agent: Pick<AgentCardModel, 'config'>): FeatureBadgeKey[] {
  const config = agent.config ?? {};
  const badges: FeatureBadgeKey[] = [config.agent_mode === 'smart-reasoning' ? 'modeAgent' : 'modeNormal'];
  if (config.web_search_enabled === true) badges.push('webSearch');
  if ((config.knowledge_bases?.length ?? 0) > 0 || config.kb_selection_mode === 'all') badges.push('knowledge');
  if ((config.mcp_services?.length ?? 0) > 0 || config.mcp_selection_mode === 'all') badges.push('mcp');
  if (config.multi_turn_enabled === true) badges.push('multiTurn');
  return badges;
}

export const FEATURE_BADGE_TITLE_KEYS: Record<FeatureBadgeKey, string> = {
  modeNormal: 'agent.mode.normal',
  modeAgent: 'agent.mode.agent',
  webSearch: 'agent.features.webSearch',
  knowledge: 'agent.features.knowledgeBase',
  mcp: 'agent.features.mcp',
  multiTurn: 'agent.features.multiTurn',
};

// --- avatar (port of frontend/src/components/AgentAvatar.vue) -----------------

const AVATAR_GRADIENTS: ReadonlyArray<{ from: string; to: string }> = [
  { from: '#667eea', to: '#764ba2' },
  { from: '#4facfe', to: '#00f2fe' },
  { from: '#43e97b', to: '#38f9d7' },
  { from: '#11998e', to: '#38ef7d' },
  { from: '#5ee7df', to: '#b490ca' },
  { from: '#48c6ef', to: '#6f86d6' },
  { from: '#a8edea', to: '#fed6e3' },
  { from: '#667db6', to: '#0082c8' },
  { from: '#36d1dc', to: '#5b86e5' },
  { from: '#56ab2f', to: '#a8e063' },
  { from: '#614385', to: '#516395' },
  { from: '#02aab0', to: '#00cdac' },
  { from: '#6a82fb', to: '#fc5c7d' },
  { from: '#834d9b', to: '#d04ed6' },
  { from: '#4776e6', to: '#8e54e9' },
  { from: '#00b09b', to: '#96c93d' },
];

function nameHash(name: string): number {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = ((hash << 5) - hash) + name.charCodeAt(i);
    hash = hash & hash;
  }
  return Math.abs(hash);
}

export function avatarGradient(name: string): { from: string; to: string } {
  return AVATAR_GRADIENTS[nameHash(name || '') % AVATAR_GRADIENTS.length]!;
}

export function avatarLetter(name: string): string {
  const trimmed = (name ?? '').trim();
  if (!trimmed) return '?';
  const first = trimmed.charAt(0);
  return /[a-zA-Z]/.test(first) ? first.toUpperCase() : first;
}

// --- share scope summary (AgentList.vue sharedAgent*ScopeText) ----------------

export type ScopeSummary = { kind: 'all' } | { kind: 'selected'; count: number } | { kind: 'none' };

export function kbScope(config?: AgentListConfig): ScopeSummary {
  const mode = config?.kb_selection_mode;
  if (mode === 'all') return { kind: 'all' };
  if (mode === 'selected' && (config?.knowledge_bases?.length ?? 0) > 0) return { kind: 'selected', count: config!.knowledge_bases!.length };
  return { kind: 'none' };
}

export function mcpScope(config?: AgentListConfig): ScopeSummary {
  const mode = config?.mcp_selection_mode;
  if (mode === 'all') return { kind: 'all' };
  if (mode === 'selected' && (config?.mcp_services?.length ?? 0) > 0) return { kind: 'selected', count: config!.mcp_services!.length };
  return { kind: 'none' };
}

export function scopeSummaryText(scope: ScopeSummary, t: (key: string, values?: Record<string, string | number>) => string): string {
  if (scope.kind === 'all') return t('agent.shareScope.kbAll');
  if (scope.kind === 'selected') return scope.kind === undefined ? '' : t('agent.shareScope.kbSelected', { count: scope.count });
  return t('agent.shareScope.kbNone');
}

/** Vue GlobalCommandPalette.vue:449 + AgentList drawer query shape. */
export function chatNavigationPath(agent: Pick<AgentCardModel, 'id' | 'sourceTenantId'>): string {
  const base = `/platform/creatChat?agent_id=${encodeURIComponent(agent.id)}`;
  return agent.sourceTenantId === undefined || agent.sourceTenantId === null
    ? base
    : `${base}&source_tenant_id=${encodeURIComponent(String(agent.sourceTenantId))}`;
}

// --- pins: favorites + recents -------------------------------------------------
//
// Recents mirror Vue useResourcePins storage byte-for-byte:
// key `WeKnora_<userId>_[t<tenantId>_]resource_recents`, PinEntry[] {type,id,ts},
// cap 30 — the same storage the Vue client writes. Favorites in Vue are
// DB-backed (GET/POST /user/favorites) which the React api-client does not
// expose yet; until then the agents slice persists them in localStorage under
// the same per-(user, tenant) key shape (recorded parity gap).

export interface PinEntry { type: 'kb' | 'agent'; id: string; ts: number }

const RECENTS_CAP = 30;

function tenantSegment(tenantId: string | null | undefined): string {
  return tenantId ? `t${tenantId}_` : '';
}

export function recentsStorageKey(userId: string, tenantId: string | null | undefined): string {
  return `WeKnora_${userId}_${tenantSegment(tenantId)}resource_recents`;
}

export function favoritesStorageKey(userId: string, tenantId: string | null | undefined): string {
  return `WeKnora_${userId}_${tenantSegment(tenantId)}agent_favorites`;
}

type MinimalStorage = Pick<Storage, 'getItem' | 'setItem'>;

function parsePinEntries(raw: string | null): PinEntry[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((
      (entry: unknown): entry is PinEntry => !!entry && typeof entry === 'object'
        && ((entry as PinEntry).type === 'kb' || (entry as PinEntry).type === 'agent')
        && typeof (entry as PinEntry).id === 'string'
        && typeof (entry as PinEntry).ts === 'number'
    ));
  } catch {
    return [];
  }
}

export function readAgentRecents(storage: MinimalStorage, userId: string, tenantId: string | null | undefined): PinEntry[] {
  return parsePinEntries(storage.getItem(recentsStorageKey(userId, tenantId)))
    .filter((entry) => entry.type === 'agent')
    .sort((a, b) => b.ts - a.ts);
}

export function writeAgentRecents(storage: MinimalStorage, userId: string, tenantId: string | null | undefined, entries: ReadonlyArray<PinEntry>): void {
  try { storage.setItem(recentsStorageKey(userId, tenantId), JSON.stringify(entries)); } catch { /* storage unavailable */ }
}

export function touchAgentRecent(entries: ReadonlyArray<PinEntry>, id: string, now: number, cap = RECENTS_CAP): PinEntry[] {
  const next = entries.filter((entry) => !(entry.type === 'agent' && entry.id === id));
  next.unshift({ type: 'agent', id, ts: now });
  return next.slice(0, cap);
}

export function readFavoriteIds(storage: MinimalStorage, userId: string, tenantId: string | null | undefined): string[] {
  const raw = storage.getItem(favoritesStorageKey(userId, tenantId));
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === 'string') : [];
  } catch {
    return [];
  }
}

export function writeFavoriteIds(storage: MinimalStorage, userId: string, tenantId: string | null | undefined, ids: ReadonlyArray<string>): void {
  try { storage.setItem(favoritesStorageKey(userId, tenantId), JSON.stringify(ids)); } catch { /* storage unavailable */ }
}

export function toggleFavoriteId(ids: ReadonlyArray<string>, id: string): string[] {
  if (ids.includes(id)) return ids.filter((existing) => existing !== id);
  return [id, ...ids];
}

// --- favorites / recents hydration against the loaded index --------------------

export interface AgentPinIndex { byId: Map<string, AgentCardModel> }

export function buildPinIndex(ownAgents: ReadonlyArray<RawAgent>, shared: ReadonlyArray<SharedAgentSummary>, options: { userId: string; disabledOwnIds: string[] }): AgentPinIndex {
  const byId = new Map<string, AgentCardModel>();
  for (const card of buildAllViewRows(ownAgents, shared, options)) byId.set(card.id, card);
  return { byId };
}

/** Hydrate pinned ids into cards, preserving the pin order (Vue favoritesAgentList). */
export function hydratePinnedCards(index: AgentPinIndex, ids: ReadonlyArray<string>): AgentCardModel[] {
  const cards: AgentCardModel[] = [];
  for (const id of ids) {
    const card = index.byId.get(id);
    if (card) cards.push(card);
  }
  return cards;
}
