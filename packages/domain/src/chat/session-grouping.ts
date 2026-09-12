/**
 * Sidebar session group-mode persistence (Vue sessionGrouping.ts:35-80).
 *
 * Only the group-by toggle state is persisted here: the current chat session
 * rows (contracts ChatSession) carry no origin info (no im_platform / embed
 * markers are populated by the API client mapping), so source-based grouping
 * cannot be reconstructed client-side. That stays a backend gap.
 */

export type SessionGroupMode = 'none' | 'date'

export const SESSION_GROUP_MODE_STORAGE_KEY = 'weknora:session-group-mode'
export const DEFAULT_SESSION_GROUP_MODE: SessionGroupMode = 'none'

export function readStoredGroupMode(storage?: Storage | null): SessionGroupMode {
  const store = storage ?? (typeof localStorage === 'undefined' ? null : localStorage)
  if (!store) return DEFAULT_SESSION_GROUP_MODE
  const raw = store.getItem(SESSION_GROUP_MODE_STORAGE_KEY)
  if (raw === 'none' || raw === 'date') return raw
  // Legacy "source" mode — no longer selectable; fall back to the default.
  return DEFAULT_SESSION_GROUP_MODE
}

export function storeGroupMode(mode: SessionGroupMode, storage?: Storage | null): void {
  const store = storage ?? (typeof localStorage === 'undefined' ? null : localStorage)
  if (!store) return
  store.setItem(SESSION_GROUP_MODE_STORAGE_KEY, mode)
}
