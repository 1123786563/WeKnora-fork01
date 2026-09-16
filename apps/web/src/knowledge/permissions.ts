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

export interface KnowledgeBaseMetadataError {
  kind: 'forbidden' | 'error';
  message: string;
}

/** Preserve a failed metadata request as an error state, not as viewer access. */
export function classifyKnowledgeBaseMetadataError(error: unknown, fallback = 'Unable to load knowledge base'): KnowledgeBaseMetadataError {
  const candidate = error as { status?: unknown; code?: unknown; message?: unknown };
  const status = candidate?.status;
  const code = typeof candidate?.code === 'string' ? candidate.code.toLowerCase() : '';
  const forbidden = status === 403 || code === '403' || code === 'http_403' || code.includes('forbidden');
  return {
    kind: forbidden ? 'forbidden' : 'error',
    message: typeof candidate?.message === 'string' && candidate.message.trim() ? candidate.message : fallback,
  };
}

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

function explicitKBPermission(kb: KBSurfaceKB): 'owner' | 'admin' | 'editor' | 'viewer' | undefined {
  const value = kb.my_permission ?? kb.permission;
  if (typeof value !== 'string' || !value.trim()) return undefined;
  const normalized = value.trim().toLowerCase();
  return ['owner', 'admin', 'editor', 'viewer'].includes(normalized)
    ? normalized as 'owner' | 'admin' | 'editor' | 'viewer'
    : undefined;
}

function membershipAllowsWrite(me: KBSurfaceMe): boolean {
  return Array.isArray(me.memberships) && me.memberships.some((membership) => {
    const role = typeof membership?.role === 'string' ? membership.role.toLowerCase() : '';
    return role === 'admin' || role === 'contributor' || role === 'editor';
  });
}

/** Viewer-only unless the user is a system admin or the KB creator. */
export function computeKBPermissions(kb: KBSurfaceKB, me: KBSurfaceMe | null | undefined): KBPermissions {
  const sharePermission = explicitKBPermission(kb);
  const canContribute = !!me && (
    isSystemAdmin(me)
    || isCreator(kb, me)
    || (sharePermission !== 'viewer' && (
      sharePermission === 'owner' || sharePermission === 'admin' || sharePermission === 'editor'
      || (!sharePermission && membershipAllowsWrite(me))
    ))
  );
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
