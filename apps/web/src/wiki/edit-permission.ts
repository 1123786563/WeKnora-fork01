// Wiki edit-entry permission, mirroring the Vue canEdit contract exactly.
//
// Vue (frontend/src/views/knowledge/KnowledgeBase.vue:288-293) gates the
// WikiBrowser edit entry with:
//   isViaShare ? canEditKB            // share grant 'admin' | 'editor'
//   : isOwner   ? true                // kbInfo.creator_id === me.id (non-empty)
//   : hasRole('admin') ? true         // active-tenant role >= admin (admin|owner)
//   : canEditKB                       // same share-list grant lookup
// where canEditKB (stores/organization.ts:809) resolves the grant from the
// org shared-knowledge-bases list (`permission` field).
//
// React's previous probe (computeKBPermissions) diverged in three ways:
// 1. It read `kb.user_id ?? kb.created_by` for ownership, but the backend KB
//    JSON only ever emits `creator_id` (internal/types/knowledgebase.go:77) —
//    the creator check could never match.
// 2. It read the share grant from the KB row (`my_permission ?? permission`),
//    but the backend only attaches `my_permission` when the KB's tenant differs
//    from the active tenant (internal/handler/knowledgebase.go:492-495). The
//    org share list is the authoritative signal Vue relies on ("Presence in
//    the share list is the authoritative signal" — a member of both the source
//    and receiving tenants still gets their share grant honored).
// 3. It accepted a tenant 'contributor' membership as write; Vue explicitly
//    excludes contributor ("being a Contributor in a tenant does not by itself
//    grant edit on someone else's KB").

import type { KBSurfaceKB, KBSurfaceMe } from '../knowledge/permissions.ts';

/** Vue canEditKB: only these share grants unlock edit. */
const EDITABLE_SHARE_PERMISSIONS: ReadonlySet<string> = new Set(['admin', 'editor']);

/** Vue ROLE_LEVEL: hasRole('admin') is satisfied by admin (30) and owner (40). */
const TENANT_ADMIN_ROLES: ReadonlySet<string> = new Set(['admin', 'owner']);

export interface SharedKBGrant {
  permission: string;
}

function normalizePermission(value: unknown): string {
  return typeof value === 'string' ? value.trim().toLowerCase() : '';
}

/**
 * Find this KB's row in GET /api/v1/shared-knowledge-bases (Vue
 * sharedKnowledgeBases / orgStore.getKBPermission). Rows carry
 * `{ knowledge_base: { id }, permission }`; the permission is what the space
 * was granted, not the per-user effective `my_permission`.
 */
export function findSharedKBGrant(rows: unknown, knowledgeBaseId: string): SharedKBGrant | null {
  if (!Array.isArray(rows)) return null;
  for (const row of rows) {
    if (!row || typeof row !== 'object' || Array.isArray(row)) continue;
    const knowledgeBase = (row as Record<string, unknown>).knowledge_base;
    const id = knowledgeBase && typeof knowledgeBase === 'object' && !Array.isArray(knowledgeBase)
      ? (knowledgeBase as Record<string, unknown>).id
      : undefined;
    if (id === undefined || id === null || String(id) !== knowledgeBaseId) continue;
    return { permission: normalizePermission((row as Record<string, unknown>).permission) };
  }
  return null;
}

function rowSharePermission(kb: KBSurfaceKB): string {
  return normalizePermission(kb.my_permission ?? kb.permission);
}

function isKBImmOwner(kb: KBSurfaceKB, me: KBSurfaceMe): boolean {
  // Vue isOwner reads kbInfo.creator_id and requires it non-empty (legacy KBs
  // with an empty creator fall through to the role gate).
  const creatorId = kb.creator_id ?? kb.created_by ?? kb.user_id;
  const userId = me.user?.id;
  if (typeof creatorId !== 'string' || !creatorId) return false;
  if (userId === undefined || userId === null) return false;
  return creatorId === String(userId);
}

function activeTenantId(me: KBSurfaceMe): string | null {
  const tenant = (me as { tenant?: unknown }).tenant;
  if (!tenant || typeof tenant !== 'object' || Array.isArray(tenant)) return null;
  const id = (tenant as { id?: unknown }).id;
  return id === undefined || id === null ? null : String(id);
}

/** Vue hasRole('admin'): the ACTIVE tenant's membership role must be >= admin. */
function activeTenantAdmin(me: KBSurfaceMe): boolean {
  const tenantId = activeTenantId(me);
  if (tenantId === null) return false;
  return Array.isArray(me.memberships) && me.memberships.some((membership) => {
    if (!membership || typeof membership !== 'object') return false;
    const row = membership as Record<string, unknown>;
    const rowTenantId = row.tenant_id ?? row.tenantId;
    return rowTenantId !== undefined && rowTenantId !== null && String(rowTenantId) === tenantId
      && TENANT_ADMIN_ROLES.has(typeof row.role === 'string' ? row.role.toLowerCase() : '');
  });
}

/**
 * Vue canEdit for the wiki edit entry. Inputs:
 * - `kb`: GET /knowledge-bases/:id row (creator_id, my_permission).
 * - `me`: GET /auth/me (user.id, memberships, active tenant).
 * - `sharedRows`: GET /shared-knowledge-bases rows (authoritative share
 *   grant; null/undefined when the optional fetch failed).
 *
 * The KB row's `my_permission` is the backend-computed mirror of the same
 * share grant (only attached on cross-tenant access), so it is honored as a
 * fallback when the share list is unavailable — it cannot exceed what the
 * org grant itself allows.
 */
export function wikiEditPermission(kb: KBSurfaceKB, me: KBSurfaceMe | null | undefined, sharedRows: unknown): boolean {
  if (!me) return false;
  const kbId = typeof kb.id === 'string' ? kb.id : '';
  const grant = kbId ? findSharedKBGrant(sharedRows, kbId) : null;
  const rowPermission = rowSharePermission(kb);
  // isViaShare: once a share signal exists (list row or row-level mirror),
  // only the grant counts — local tenant role/creator never override it.
  if (grant || rowPermission) {
    return EDITABLE_SHARE_PERMISSIONS.has(grant ? grant.permission : rowPermission);
  }
  return isKBImmOwner(kb, me) || activeTenantAdmin(me);
}
