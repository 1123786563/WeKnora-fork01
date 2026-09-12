// KB-surface permission + routing helpers.
//
// The pages only receive knowledgeBaseId, so the KB record (GET
// /api/v1/knowledge-bases/:id — routes_knowledge.go declares the read route)
// and /auth/me provide the gating inputs. Mirrors the Vue rule: a plain
// KB viewer (or org-shared read-only member) cannot upload or run batch
// destructive actions; creator, system admin and contributor+ roles can.

export interface KBSurfaceKB {
  id?: unknown;
  type?: unknown;
  user_id?: unknown;
  created_by?: unknown;
  indexing_strategy?: { wiki_enabled?: unknown; graph_enabled?: unknown } | null;
  [key: string]: unknown;
}

export interface KBSurfaceMe {
  user?: { id?: unknown; role?: unknown; is_superuser?: unknown } | null;
  memberships?: { role?: unknown; tenant_id?: unknown }[] | null;
  [key: string]: unknown;
}

export interface KBPermissions {
  canContribute: boolean;
  /** Viewer-only: hide upload form and batch destructive controls. */
  viewerOnly: boolean;
}

const CONTRIBUTOR_ROLES = new Set(['owner', 'admin', 'contributor', 'editor', 'developer']);

function isSystemAdmin(me: KBSurfaceMe): boolean {
  if (me.user?.role === 'system_admin' || me.user?.role === 'admin') return true;
  if (me.user?.is_superuser === true) return true;
  return Array.isArray(me.memberships) && me.memberships.some((membership) => membership?.role === 'system_admin');
}

function isCreator(kb: KBSurfaceKB, me: KBSurfaceMe): boolean {
  const userId = me.user?.id;
  if (userId === undefined || userId === null) return false;
  return kb.user_id !== undefined && String(kb.user_id) === String(userId)
    || kb.created_by !== undefined && String(kb.created_by) === String(userId);
}

function membershipAllowsWrite(me: KBSurfaceMe): boolean {
  if (!Array.isArray(me.memberships) || me.memberships.length === 0) return true;
  return me.memberships.some((membership) => typeof membership?.role !== 'string' || CONTRIBUTOR_ROLES.has(membership.role));
}

/** Viewer-only unless the user is a system admin, the KB creator, or holds a
 *  contributor+ role. No membership evidence at all keeps editing enabled so
 *  the default stays backwards-compatible. */
export function computeKBPermissions(kb: KBSurfaceKB, me: KBSurfaceMe | null | undefined): KBPermissions {
  const canContribute = !me || isSystemAdmin(me) || isCreator(kb, me) || membershipAllowsWrite(me);
  return { canContribute, viewerOnly: !canContribute };
}

/** Audit #6: an FAQ-type KB must land on the FAQ route, never the documents list. */
export function kbTypeRedirectPath(kb: KBSurfaceKB): string | undefined {
  const id = typeof kb.id === 'string' ? kb.id : undefined;
  if (!id || typeof kb.type !== 'string' || kb.type.toLowerCase() !== 'faq') return undefined;
  return `/knowledgeBase/${encodeURIComponent(id)}/faq`;
}

export type KBSurfaceTab = 'documents' | 'wiki' | 'graph';

/** Wiki/graph tabs only exist when the indexing strategy enables them. */
export function resolveKBSurfaceTabs(kb: KBSurfaceKB): KBSurfaceTab[] {
  const tabs: KBSurfaceTab[] = ['documents'];
  if (kb.indexing_strategy?.wiki_enabled === true) tabs.push('wiki');
  if (kb.indexing_strategy?.graph_enabled === true) tabs.push('graph');
  return tabs;
}
