// Per-(user, tenant) favorites + per-user recents for knowledge bases — a
// port of frontend/src/composables/useResourcePins.ts.
//
// Favorites are DB-backed: the Vue composable calls GET/POST/DELETE
// /api/v1/user/favorites (internal/router/routes_agent.go:62-68), so they sync
// across devices and survive restarts/logouts by construction. Recents stay
// client-side; where Vue uses per-(user,tenant) localStorage, the mobile app
// persists through its existing storage layer (expo-secure-store — the same
// mechanism the runtime uses for locale and workspace selection), injected
// here as a key/value adapter so tests can run against an in-memory fake.

export interface KeyValueStorage {
  // Sync-or-async on purpose: expo-secure-store exposes both getItem and
  // getItemAsync; awaiting either shape works and tests pass an async fake.
  getItem(key: string): string | null | Promise<string | null>;
  setItem(key: string, value: string): void | Promise<void>;
}

export type PinResourceType = 'kb' | 'agent';

export interface PinEntry {
  type: PinResourceType;
  id: string;
  /** Unix ms when the pin was created (favorite) or last accessed (recent). */
  ts: number;
}

/** Vue RECENTS_CAP (useResourcePins.ts:33). */
export const RECENTS_CAP = 30;

const RECENTS_SUFFIX = 'resource_recents';

// Vue keys recents as `WeKnora_{userId}_t{tenantId}_resource_recents`.
// Anonymous sessions use the same explicit `anon` segment as Vue's
// preferenceStorage. Do not fall back to the old tenant-only key: doing so
// could expose another user's navigation history after logout/login.
export function kbRecentsStorageKey(userId: string | null, tenantId: string | null): string {
  const userSegment = userId && userId.trim() ? userId.trim() : 'anon';
  const tenantSegment = tenantId && tenantId.trim() ? `t${tenantId.trim()}_` : '';
  return `WeKnora_${userSegment}_${tenantSegment}${RECENTS_SUFFIX}`;
}

/** Vue readRecents validation: drop anything that is not a well-formed entry. */
export function parseKbRecents(raw: string | null): PinEntry[] {
  if (!raw) return [];
  let parsed: unknown;
  try { parsed = JSON.parse(raw); } catch { return []; }
  if (!Array.isArray(parsed)) return [];
  return parsed.filter((entry): entry is PinEntry =>
    !!entry
    && typeof (entry as PinEntry).type === 'string'
    && ((entry as PinEntry).type === 'kb' || (entry as PinEntry).type === 'agent')
    && typeof (entry as PinEntry).id === 'string'
    && typeof (entry as PinEntry).ts === 'number');
}

export function sortRecentsDesc(entries: PinEntry[]): PinEntry[] {
  return [...entries].sort((a, b) => b.ts - a.ts);
}

/** Monotonic guard for async pin hydration across user/tenant transitions. */
export function createPinsGeneration() {
  let current = 0;
  return {
    next(): number { current += 1; return current; },
    isCurrent(generation: number): boolean { return generation === current; },
  };
}

export async function readKbRecents(storage: KeyValueStorage, userId: string | null, tenantId: string | null): Promise<PinEntry[]> {
  return sortRecentsDesc(parseKbRecents(await storage.getItem(kbRecentsStorageKey(userId, tenantId))));
}

/** Vue touchRecent: move the entry to the front, refresh its timestamp and
 * cap the list at RECENTS_CAP; returns the persisted, sorted list. */
export async function touchKbRecent(storage: KeyValueStorage, userId: string | null, tenantId: string | null, id: string, now: number = Date.now()): Promise<PinEntry[]> {
  const list = parseKbRecents(await storage.getItem(kbRecentsStorageKey(userId, tenantId)));
  const next = list.filter((entry) => !(entry.type === 'kb' && entry.id === id));
  next.unshift({ type: 'kb', id, ts: now });
  if (next.length > RECENTS_CAP) next.length = RECENTS_CAP;
  await storage.setItem(kbRecentsStorageKey(userId, tenantId), JSON.stringify(next));
  return sortRecentsDesc(next);
}

export async function removeKbRecent(storage: KeyValueStorage, userId: string | null, tenantId: string | null, id: string): Promise<PinEntry[]> {
  const list = parseKbRecents(await storage.getItem(kbRecentsStorageKey(userId, tenantId)));
  const next = list.filter((entry) => !(entry.type === 'kb' && entry.id === id));
  if (next.length !== list.length) await storage.setItem(kbRecentsStorageKey(userId, tenantId), JSON.stringify(next));
  return sortRecentsDesc(next);
}

/** Minimal view of the api-client the favorites endpoints need; satisfied by
 * the public `client.request` surface. */
export interface FavoritesClient {
  request(input: { method: string; path: string; body?: unknown }): Promise<unknown>;
}

function parseFavoriteRows(value: unknown): Array<{ resource_type?: unknown; resource_id?: unknown }> {
  if (!value || typeof value !== 'object') return [];
  const data = (value as { data?: unknown }).data;
  return Array.isArray(data) ? data as Array<{ resource_type?: unknown; resource_id?: unknown }> : [];
}

/** GET /api/v1/user/favorites?type=kb — the server scopes rows to the active
 * (user, tenant) pair, exactly like the Vue composable. */
export async function fetchKbFavoriteIds(client: FavoritesClient): Promise<Set<string>> {
  const rows = parseFavoriteRows(await client.request({ method: 'GET', path: '/api/v1/user/favorites?type=kb' }));
  return new Set(rows.filter((row) => row.resource_type === 'kb' && typeof row.resource_id === 'string').map((row) => row.resource_id as string));
}

export async function addKbFavorite(client: FavoritesClient, id: string): Promise<void> {
  await client.request({ method: 'POST', path: '/api/v1/user/favorites', body: { type: 'kb', id } });
}

export async function removeKbFavorite(client: FavoritesClient, id: string): Promise<void> {
  await client.request({ method: 'DELETE', path: `/api/v1/user/favorites/kb/${encodeURIComponent(id)}` });
}
