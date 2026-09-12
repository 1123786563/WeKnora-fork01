// Pure, DOM-free logic for the global command palette (⌘K / Ctrl+K).
//
// Vue reference: frontend/src/components/GlobalCommandPalette.vue,
// frontend/src/components/GlobalCommandPalette/commands.ts,
// frontend/src/stores/commandPalette.ts.
//
// This module intentionally keeps semantic chunk/message/KB/agent search out
// of scope for the React port (R011/N003, slice S03): that would require
// touching document, chat, and knowledge-base search endpoints outside this
// slice's file ownership. What is ported is the part that makes the palette
// *visible and usable* — the static command catalogue, recent-query
// persistence, keyboard navigation, and the global shortcut/`?cmdk=` wiring.
// "Product tour" is intentionally omitted from the command catalogue:
// frontend/src/components/NewUserGuide.vue has no React port yet, so there is
// nothing for that command to open.

export interface CommandDescriptor {
  id: string;
  labelKey: string;
  icon: 'new-chat' | 'knowledge-bases' | 'agents' | 'organizations' | 'settings';
  keywords: string[];
  path: string;
}

// Mirrors GlobalCommandPalette/commands.ts buildCommands() order and targets.
export const COMMANDS: readonly CommandDescriptor[] = [
  { id: 'new-chat', labelKey: 'commandPalette.quick.newChat', icon: 'new-chat', keywords: ['new', 'chat', 'conversation', '新建', '对话', 'создать'], path: '/platform/creatChat' },
  { id: 'open-kb-list', labelKey: 'commandPalette.quick.knowledgeBases', icon: 'knowledge-bases', keywords: ['kb', 'knowledge', 'base', '知识库', '文档'], path: '/platform/knowledge-bases' },
  { id: 'open-agents', labelKey: 'commandPalette.quick.agents', icon: 'agents', keywords: ['agent', 'bot', '智能体', '助手'], path: '/platform/agents' },
  { id: 'open-organizations', labelKey: 'commandPalette.quick.organizations', icon: 'organizations', keywords: ['org', 'organization', 'team', 'space', '组织', '共享'], path: '/platform/organizations' },
  { id: 'open-settings', labelKey: 'commandPalette.quick.settings', icon: 'settings', keywords: ['settings', 'preferences', 'config', '设置', '配置'], path: '/platform/settings' },
];

/**
 * Filter commands by a free-text query. Matches on the localized label OR any
 * keyword, case-insensitive. Empty/blank query returns the full list
 * unchanged (mirrors Vue filterCommands()).
 */
export function filterCommands(
  commands: readonly CommandDescriptor[],
  query: string,
  translate: (key: string) => string,
): CommandDescriptor[] {
  const q = query.trim().toLowerCase();
  if (!q) return [...commands];
  return commands.filter((cmd) => {
    if (translate(cmd.labelKey).toLowerCase().includes(q)) return true;
    return cmd.keywords.some((keyword) => keyword.toLowerCase().includes(q));
  });
}

// ─── Recent queries (⌘K search history) ───
//
// Namespaced per (userId, tenantId) so a browser shared between accounts, or
// an account switching tenants, never leaks the other identity's search
// terms (mirrors frontend/src/stores/commandPalette.ts recentKey()).

const RECENT_KEY_PREFIX = 'weknora_cmdk_recent';
export const RECENT_QUERIES_LIMIT = 4;

export function recentQueriesStorageKey(
  userId: string | null | undefined,
  tenantId: string | number | null | undefined,
): string {
  const user = userId ? String(userId) : 'anon';
  const tenant = tenantId !== null && tenantId !== undefined && tenantId !== '' ? String(tenantId) : 'none';
  return `${RECENT_KEY_PREFIX}:${user}:${tenant}`;
}

export interface KeyValueStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export function loadRecentQueries(storage: KeyValueStorage, key: string): string[] {
  try {
    const raw = storage.getItem(key);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((entry): entry is string => typeof entry === 'string') : [];
  } catch {
    return [];
  }
}

export function pushRecentQuery(
  storage: KeyValueStorage,
  key: string,
  query: string,
  limit: number = RECENT_QUERIES_LIMIT,
): string[] {
  const trimmed = query.trim();
  const current = loadRecentQueries(storage, key);
  if (!trimmed) return current;
  const next = [trimmed, ...current.filter((entry) => entry !== trimmed)].slice(0, limit);
  try {
    storage.setItem(key, JSON.stringify(next));
  } catch {
    /* ignore quota errors, mirrors Vue store */
  }
  return next;
}

export function clearRecentQueries(storage: KeyValueStorage, key: string): void {
  try {
    storage.removeItem(key);
  } catch {
    /* ignore */
  }
}

// ─── Keyboard navigation ───

/** Wrapping ArrowUp/ArrowDown selection move (mirrors Vue moveSelection()). */
export function nextSelectedIndex(current: number, delta: number, total: number): number {
  if (total <= 0) return -1;
  return (current + delta + total) % total;
}

/** ⌘1-9 jump digit for a flat index, or undefined past the 9th slot. */
export function shortcutDigitFor(flatIndex: number): number | undefined {
  return flatIndex >= 0 && flatIndex <= 8 ? flatIndex + 1 : undefined;
}

// ─── Global ⌘K / Ctrl+K shortcut ───

export type GlobalShortcutAction = 'toggle' | 'open' | 'none';

export interface GlobalShortcutEvent {
  metaKey: boolean;
  ctrlKey: boolean;
  key: string;
}

/**
 * Decide what a global keydown should do to the palette. Mirrors
 * GlobalCommandPalette.vue onGlobalKey(): ⌘K/Ctrl+K always toggles (even from
 * an input); a bare "/" opens the palette only when the palette is closed and
 * nothing editable is currently focused.
 */
export function decideGlobalShortcutAction(
  event: GlobalShortcutEvent,
  context: { open: boolean; isEditingTarget: boolean },
): GlobalShortcutAction {
  const isCmd = event.metaKey || event.ctrlKey;
  if (isCmd && event.key.toLowerCase() === 'k') return 'toggle';
  if (event.key === '/' && !context.open && !context.isEditingTarget) return 'open';
  return 'none';
}

// ─── `?cmdk=` query consumption ───
//
// `/platform/knowledge-search?q=...` redirects to
// `/platform/knowledge-bases?cmdk=...` (routes.tsx routeRedirect). The
// platform shell must consume that query param once — open the palette with
// it — and strip it from the URL so Back/Refresh doesn't reopen it (mirrors
// platform/index.vue's `watch(() => route.query.cmdk, ...)`).
export function consumeCmdkParam(search: string): { query: string | null; remainingSearch: string } {
  const params = new URLSearchParams(search);
  if (!params.has('cmdk')) return { query: null, remainingSearch: search };
  const query = params.get('cmdk') ?? '';
  params.delete('cmdk');
  const remaining = params.toString();
  return { query, remainingSearch: remaining ? `?${remaining}` : '' };
}
