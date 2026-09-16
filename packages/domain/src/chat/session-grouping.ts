/**
 * Sidebar session group-mode persistence (Vue sessionGrouping.ts:35-80).
 *
 * Only the group-by toggle state is persisted here. Source badges are derived
 * client-side below from the enriched list rows (im_platform / embed marker /
 * api-key owner id pass through contracts ChatSession's index signature).
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

/*
 * Session source badge (Vue sessionGrouping.ts resolveSessionOrigin).
 *
 * The session list endpoint enriches each row with its IM origin
 * (internal/types/session.go SessionListItem) and passes raw user_id /
 * description through, so the origin can be classified client-side:
 * - im_platform set          -> IM session (feishu / wechat / slack / ...)
 * - description embed marker -> embed channel session
 * - api-key owner prefix     -> tenant API-key session
 * - otherwise                -> the user's own Web-console chat
 */

/** Mirrors backend types.EmbedSessionMarkerPrefix. */
export const EMBED_SESSION_MARKER_PREFIX = 'embed_channel:'

/** Mirrors backend types.SessionOwnerAPITenantKeyPrefix. */
export const API_SESSION_OWNER_PREFIX = 'api_tenant_key:'

/** Mirrors backend types.SessionOwnerAPIExternalUserPrefix. */
export const API_EXTERNAL_USER_SESSION_OWNER_PREFIX = 'api_external_user:'

export interface SessionForOrigin {
  id: string
  im_platform?: unknown
  description?: unknown
  user_id?: unknown
}

export type SessionOrigin =
  | { kind: 'web' }
  | { kind: 'im'; platform: string }
  | { kind: 'embed'; channelId: string }
  | { kind: 'api' }

export function resolveSessionOrigin(session: SessionForOrigin): SessionOrigin {
  const platform = String(session.im_platform ?? '').trim().toLowerCase()
  if (platform) return { kind: 'im', platform }
  const description = String(session.description ?? '').trim()
  if (description.startsWith(EMBED_SESSION_MARKER_PREFIX)) {
    const channelId = description.slice(EMBED_SESSION_MARKER_PREFIX.length).trim()
    if (channelId) return { kind: 'embed', channelId }
  }
  const ownerId = String(session.user_id ?? '')
  if (ownerId.startsWith(API_SESSION_OWNER_PREFIX) || ownerId.startsWith(API_EXTERNAL_USER_SESSION_OWNER_PREFIX)) {
    return { kind: 'api' }
  }
  return { kind: 'web' }
}

export interface SessionSourceBadge {
  /** CSS modifier, e.g. "is-im"; "" for web. */
  kind: string
  label: string
}

/** Sidebar badge text for a session: Web / IM platform / Embed / API. */
export function sessionSourceBadge(session: SessionForOrigin): SessionSourceBadge {
  const origin = resolveSessionOrigin(session)
  switch (origin.kind) {
    case 'im':
      return { kind: 'is-im', label: origin.platform.toUpperCase() }
    case 'embed':
      return { kind: 'is-embed', label: 'Embed' }
    case 'api':
      return { kind: 'is-api', label: 'API' }
    default:
      return { kind: '', label: 'Web' }
  }
}
