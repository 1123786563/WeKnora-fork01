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
  // Vue isOwner (KnowledgeBase.vue:250-258) matches creator_id — the only
  // creator field the live KB payload carries (internal/types/knowledgebase.go);
  // user_id/created_by are legacy fallbacks kept for older snapshots.
  return kb.creator_id !== undefined && String(kb.creator_id) === String(userId)
    || kb.user_id !== undefined && String(kb.user_id) === String(userId)
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

/**
 * Moved verbatim from KnowledgeDocumentsPage so every KB surface (documents,
 * graph, …) gates management chrome (settings gear) with the same signal.
 * Vue canEditKB parity for the upload surface, including shared editor grants.
 */
export function canUploadKnowledgeDocuments(kb: KBSurfaceKB, me: KBSurfaceMe | null | undefined): boolean {
  const userId = me?.user?.id;
  const creatorId = kb.creator_id ?? kb.created_by ?? kb.user_id;
  const isCreator = userId !== undefined && userId !== null && creatorId !== undefined && String(userId) === String(creatorId);
  // Vue checks ownership before the effective share projection. A stale
  // my_permission=viewer on an owned KB must not hide the creator's upload
  // controls (the share-first restriction is resolved by the KB context).
  if (isCreator) return true;
  const permission = kb.my_permission ?? kb.permission;
  if (typeof permission === "string" && permission.trim()) {
    return ["owner", "admin", "editor"].includes(permission.trim().toLowerCase());
  }
  const isAdmin = Boolean(me?.user?.is_superuser === true || me?.user?.role === "admin" || me?.user?.role === "system_admin"
    || me?.memberships?.some((membership) => membership.role === "admin" || membership.role === "system_admin"));
  return isAdmin;
}

export type KBSurfaceTab = 'documents' | 'wiki' | 'graph';

/**
 * Vue gates the whole 文档/Wiki/图谱 tab row on isWiki alone
 * (KnowledgeBase.vue:89 `!!kbInfo.indexing_strategy.wiki_enabled`, template
 * 2359-2381): a wiki KB always shows the three tabs — the graph view lives
 * inside the wiki surface and is never gated separately — while a KB with the
 * wiki off renders the plain 文档 crumb and no tab row at all, even when graph
 * extraction is enabled. Empty result = the caller falls back to the crumb.
 */
export function resolveKBSurfaceTabs(kb: KBSurfaceKB): KBSurfaceTab[] {
  if (kb.indexing_strategy?.wiki_enabled !== true) return [];
  return ['documents', 'wiki', 'graph'];
}

/**
 * The ?tab=wiki|graph views only exist for wiki KBs: Vue keeps the URL but
 * renders the documents branch when isWiki is false (KnowledgeBase.vue:2412
 * gates .wiki-main-area on isWiki, 2418 renders the documents branch on
 * `!isWiki`). The React tab pages are separate routes, so a non-wiki deep
 * link falls back to the canonical documents URL — the same view Vue shows.
 * Mirrors kbTypeRedirectPath: undefined when the current URL is already right.
 */
export function kbWikiTabFallbackPath(kb: KBSurfaceKB): string | undefined {
  const id = typeof kb.id === 'string' ? kb.id : undefined;
  if (!id || kb.indexing_strategy?.wiki_enabled === true) return undefined;
  return `/knowledgeBase/${encodeURIComponent(id)}`;
}
