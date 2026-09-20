import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react';
import type { ModelConfiguration, WeKnoraClient } from '@weknora/api-client';
import { createScopeController, scopedKey, filterKnowledgeBases } from '@weknora/domain';
import {
  canDuplicateKBCard,
  canManageKBCard,
  isKnowledgeBaseInitialized,
  isSharedKbEditable,
  groupKnowledgeBaseSections,
  mergeAllScopeKnowledgeBases,
  type MergedKnowledgeBase,
} from '@weknora/domain';
import { formatMessage, type MessageValues } from '@weknora/i18n';
import { usePreferredLocale } from './locale.ts';
import { navigate } from './platform/navigation.ts';
import { Button, Checkbox, Dialog, Input, Select, Status, Textarea } from '@weknora/ui';
import {
  isContextualGuideDone,
  markContextualGuideDone,
  openContextualGuide,
} from '../../../packages/views/src/guides/contextual-guides.ts';
import {
  createDeleteGuard,
  knowledgeBaseDetailPath,
  loadKnowledgeBaseListPage,
  saveKnowledgeBase,
  type KnowledgeBaseListPageState,
  type KnowledgeBaseSaveInput,
} from './knowledge-bases/list.ts';
import { KnowledgeBaseShareDialog } from './knowledge-bases/KnowledgeBaseShareDialog.tsx';
import { KnowledgeBaseActivityPanel } from './knowledge-bases/KnowledgeBaseActivityPanel.tsx';
import { SharedKnowledgeBaseDrawer } from './knowledge-bases/SharedKnowledgeBaseDrawer.tsx';
import { defaultKnowledgeEditorConfig, hydrateKnowledgeEditorConfig, knowledgeEditorConfigPayload, type KnowledgeEditorConfig } from './knowledge-bases/editor-config.ts';
import { visibleKnowledgeEditorSections, type KnowledgeEditorSection } from './knowledge-bases/editor-sections.ts';
// R490 A1/A5: the editor drawer mounts the same Vue-form chunking/graph
// implementations the KB settings page uses (KBChunkingSettings.vue /
// KBChunkingDebug.vue / GraphSettings.vue authority), replacing the divergent
// bare-label sections.
import { ChunkingSettingsFields } from './knowledge-settings/chunkingSection.tsx';
import { GraphSettings, type GraphExtractConfig } from './knowledge-settings/GraphSettings.tsx';
import { patchUploadTask, summarizeUploadTasks, upsertUploadTask, type UploadTaskState } from './knowledge-bases/upload-progress.ts';
import { KbIcon, type KbIconName } from './knowledge-bases/kb-list-icons.tsx';
import { DataSourcesPage } from './data-sources/DataSourcesPage.tsx';
import { KB_EMPTY_SVG } from './knowledge-bases/empty-kb-svg.ts';
import { ModelOptionSelect, type ModelOption } from './settings/ModelOptionSelect.tsx';
import { selectInitialModelId } from './agents/agent-editor.ts';
import { resolveChatModelOptions, listChatModels } from './chat/model-chip.ts';
import './knowledge-list.css';

interface KnowledgeBasesPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
}

interface Viewer {
  userId: string;
  isAdmin: boolean;
  isContributor: boolean;
}

// Vue ?scope= semantics (KnowledgeBaseList.vue:821-830, 897-906): the state
// lives in the URL so links are shareable. The four reserved pseudo-scopes
// keep their Vue names; any other value is a per-org space id (?scope=<orgId>).
type KbListSpace = 'all' | 'mine' | 'favorites' | 'recents' | (string & {});

const KB_RESERVED_SCOPES: ReadonlySet<string> = new Set(['all', 'mine', 'favorites', 'recents']);

function isOrgScope(space: KbListSpace): boolean {
  return !KB_RESERVED_SCOPES.has(space);
}

// Vue truncateLabel (ListSpaceSidebar.vue:244-247): keeps the collapsed-strip
// label visually balanced (~44px); callers show the full name via title.
function truncateRailLabel(text: string, max = 4): string {
  return text.length > max ? text.slice(0, max) + '…' : text;
}

const SKELETON_CARD_COUNT = 6;

function readScopeFromUrl(): KbListSpace {
  const value = new URLSearchParams(window.location.search).get('scope');
  // Stale URL guard (KnowledgeBaseList.vue:1246-1251): an older "协作"
  // view used scope=shared; that view is gone — land on all (the URL is
  // cleaned by the mount effect below).
  // Vue spaceSelectionOrgId gates on !!s (KnowledgeBaseList.vue:897-906):
  // an empty ?scope= is not an org id, it falls back to the all view.
  if (value === null || value === '' || value === 'all' || value === 'shared') return 'all';
  if (value === 'mine' || value === 'favorites' || value === 'recents') return value;
  return value; // per-org space id (?scope=<orgId>)
}

function writeScopeToUrl(space: KbListSpace): void {
  const url = new URL(window.location.href);
  if (space === 'all') url.searchParams.delete('scope');
  else url.searchParams.set('scope', space);
  window.history.replaceState({}, '', url);
}

function readFavorites(): Set<string> {
  try {
    const raw = window.localStorage.getItem('wk-kb-favorites');
    return new Set(raw ? (JSON.parse(raw) as string[]) : []);
  } catch {
    return new Set();
  }
}

function membershipRole(memberships: unknown, tenantId: string | null): string {
  if (!Array.isArray(memberships)) return 'viewer';
  for (const item of memberships) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) continue;
    const row = item as Record<string, unknown>;
    const id = row.tenant_id ?? row.tenantId;
    const role = row.role;
    if (tenantId !== null && String(id) === tenantId && typeof role === 'string') return role;
  }
  return 'viewer';
}

// Vue KnowledgeBaseList.vue:2203-2283 — section rows render inline in the
// grid with per-section icons (pinned/mine/tenant/shared). The two
// "shared with me" groups pair a main usergroup-add icon with a 12px
// sub-icon (edit / browse) marking the permission, per Vue :162-163/:179-180.
const SECTION_ICONS: Record<string, KbIconName> = {
  pinned: 'pin-filled',
  mine: 'user',
  tenantOthers: 'usergroup',
  sharedByMe: 'share',
  sharedEditable: 'usergroup-add',
  sharedReadonly: 'usergroup-add',
};

const SECTION_SUBICONS: Record<string, KbIconName> = {
  sharedEditable: 'edit',
  sharedReadonly: 'browse',
};

type KbListRow =
  | { kind: 'header'; key: string; labelKey: string; count: number; expanded: boolean }
  | { kind: 'card'; card: MergedKnowledgeBase };

type KbIndexingStrategy = {
  vector_enabled: boolean;
  keyword_enabled: boolean;
  wiki_enabled: boolean;
  graph_enabled: boolean;
};

type KnowledgeEditorOptions = {
  parserEngines: Array<{ Name: string; Description: string; Available?: boolean }>;
  // R490 A2 (Vue KBStorageSettings): the storage rows echo the backend config
  // (endpoint/bucket/path hint) and mark the default instance — both read off
  // listStorageBackends() like the KB settings page does.
  storageBackends: Array<{ id: string; name: string; provider: string; status: string; config?: Record<string, unknown> }>;
  defaultStorageBackendId: string;
  vectorStores: Array<{ id: string; name: string; engine_type: string; source: string; readonly: boolean }>;
  loading: boolean;
  error: string | null;
};

const DEFAULT_KB_INDEXING: KbIndexingStrategy = {
  vector_enabled: true,
  keyword_enabled: true,
  wiki_enabled: false,
  graph_enabled: false,
};

// Expanded-panel count badge visibility (Vue ListSpaceSidebar.vue:83/93/101/109):
// all/mine render whenever the count is defined (including 0); favorites and
// recents hide an empty count.
function railCountVisible(key: KbListSpace, count: number): boolean {
  return key === 'favorites' || key === 'recents' ? count > 0 : true;
}

export function KnowledgeBasesPage({ client, scopeController }: KnowledgeBasesPageProps) {
  const locale = usePreferredLocale();
  const t = useCallback((key: string, values: MessageValues = {}) => formatMessage(locale, key, values), [locale]);

  const [reloadToken, setReloadToken] = useState(0);
  const [pageState, setPageState] = useState<KnowledgeBaseListPageState>({ status: 'loading' });
  const [viewer, setViewer] = useState<Viewer>({ userId: '', isAdmin: false, isContributor: false });
  const [space, setSpaceState] = useState<KbListSpace>(readScopeFromUrl);
  const [query, setQuery] = useState(() => new URLSearchParams(window.location.search).get('q') ?? '');
  const [favorites, setFavorites] = useState<Set<string>>(readFavorites);
  const [recents, setRecents] = useState<Set<string>>(() => {
    try { return new Set(JSON.parse(window.localStorage.getItem('wk-kb-recents') ?? '[]') as string[]); } catch { return new Set(); }
  });
  const [collapsedSections, setCollapsedSections] = useState<ReadonlySet<string>>(new Set());
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [highlightId, setHighlightId] = useState<string | null>(() => {
    const hl = new URLSearchParams(window.location.search).get('highlightKbId');
    return hl || null;
  });
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [uploadTasks, setUploadTasks] = useState<UploadTaskState[]>([]);
  const uploadCleanupTimers = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  const uploadRefreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Create / edit dialog state. Prefills the full config when editing.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editorSection, setEditorSection] = useState<KnowledgeEditorSection>('basic');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [type, setType] = useState<'document' | 'faq'>('document');
  const [embeddingModelId, setEmbeddingModelId] = useState('');
  const [summaryModelId, setSummaryModelId] = useState('');
  const [indexingStrategy, setIndexingStrategy] = useState<KbIndexingStrategy>(DEFAULT_KB_INDEXING);
  const [editorConfig, setEditorConfig] = useState<KnowledgeEditorConfig>(defaultKnowledgeEditorConfig);
  // R488 summary_model_id slice: the Vue editor feeds its ModelSelectors from
  // GET /api/v1/models (KnowledgeBaseEditorModal.vue:680-693); React keeps the
  // same rows for the models section dropdowns and the create-time prefill.
  const [modelRows, setModelRows] = useState<readonly ModelConfiguration[]>([]);
  // Vue KnowledgeBaseEditorModal.vue:687-693 — a fresh create form preseeds the
  // model config with the tenant defaults (is_default first, else the first
  // active row) once the model rows land, so the summary model is never left
  // blank behind a placeholder that claims 可选. Runs as an effect (not inside
  // loadEditorOptions) because openCreate batches setEditingId(null) — the
  // fetch closure would still see the stale editing id.
  useEffect(() => {
    if (!dialogOpen || editingId !== null) return;
    if (summaryModelId && embeddingModelId) return;
    const candidates = modelRows.map((row) => ({
      id: row.id,
      type: typeof row.type === 'string' ? row.type : undefined,
      status: typeof row.status === 'string' ? row.status : undefined,
      is_default: row.is_default === true,
    }));
    if (!summaryModelId) setSummaryModelId(selectInitialModelId(candidates, 'KnowledgeQA') || '');
    if (!embeddingModelId) setEmbeddingModelId(selectInitialModelId(candidates, 'Embedding') || '');
    // modelRows is the arrival signal; manual clears during the dialog session
    // do not re-run this (value changes are not dependencies).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dialogOpen, editingId, modelRows]);
  const [editorOptions, setEditorOptions] = useState<KnowledgeEditorOptions>({ parserEngines: [], storageBackends: [], defaultStorageBackendId: '', vectorStores: [], loading: false, error: null });
  // R490 A3/A4 (Vue loadKBData): edit mode mirrors Vue's listKnowledgeFiles
  // total>0 check so the Embedding lock (KBModelConfig :41-46) and the indexing
  // lock + lockedTip (KnowledgeBaseEditorModal.vue:114-116 isIndexingLocked)
  // render on exactly the same condition as Vue.
  const [editingHasFiles, setEditingHasFiles] = useState(false);
  // Guards the edit-mode detail/files hydration against a newer open (Vue's
  // kbEditorLoadGeneration counterpart): a stale response must not repaint the
  // form after the user has moved on to another KB or a fresh create.
  const editLoadGeneration = useRef(0);
  const [editorActivity, setEditorActivity] = useState<Array<{ id: number; action: string; outcome: string; created_at: string }>>([]);
  const [editorActivityLoading, setEditorActivityLoading] = useState(false);
  const [sharingKb, setSharingKb] = useState<{ id: string; name: string } | null>(null);
  // Vue sharedDetailPanelVisible + currentSharedKbForDetail
  // (KnowledgeBaseList.vue:1489-1491): the drawer binds the raw share row.
  const [sharedDetail, setSharedDetail] = useState<Record<string, unknown> | null>(null);
  const [saving, setSaving] = useState(false);

  // Delete confirmation dialog state; DELETE fires only after confirm.
  const [deletingKb, setDeletingKb] = useState<{ id: string; name: string } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const deleteGuard = useRef(createDeleteGuard(client));

  // URL deep-link scope (?scope=) wins; otherwise the default follows the
  // viewer role once known (Vue KnowledgeBaseList.vue:816-830: contributor
  // lands on the workspace scope, viewers on "all"). Snapshotted once on
  // first render: the stale-scope cleanup below rewrites the URL, and a
  // recomputed check would let the role default override the deep link.
  const urlHasScope = useRef(new URLSearchParams(window.location.search).has('scope'));
  const userPickedScope = useRef(false);

  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'knowledge-bases'), [scope.scope]);

  const setSpace = (next: KbListSpace) => {
    userPickedScope.current = true;
    setSpaceState(next);
    writeScopeToUrl(next);
  };

  useEffect(() => {
    let active = true;
    scopeController.current().scope.userId && setViewer((current) => ({ ...current, userId: scopeController.current().scope.userId ?? '' }));
    // Role gating needs the membership role; the scope only carries ids.
    void client.auth.me().then((me) => {
      if (!active) return;
      const tenantId = scopeController.current().scope.tenantId;
      const role = membershipRole(me.memberships, tenantId);
      setViewer({
        userId: typeof me.user.id === 'string' ? me.user.id : '',
        isAdmin: role === 'owner' || role === 'admin' || me.user.is_system_admin === true,
        isContributor: role === 'owner' || role === 'admin' || role === 'contributor' || me.user.is_system_admin === true,
      });
    }).catch(() => { /* gating falls back to creator-id matching only */ });
    return () => { active = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, scopeController]);

  // Vue defaultScope (KnowledgeBaseList.vue:826): contributor -> 'mine'.
  useEffect(() => {
    if (urlHasScope.current || userPickedScope.current || !viewer.isContributor) return;
    setSpaceState((current) => (current === 'all' ? 'mine' : current));
  }, [viewer.isContributor]);

  // Stale-URL cleanup for the legacy scope=shared aggregate view
  // (KnowledgeBaseList.vue:1246-1251): the state reset happens in
  // readScopeFromUrl; the URL sync mirrors Vue's useListUrlState watcher.
  useEffect(() => {
    if (new URLSearchParams(window.location.search).get('scope') === 'shared') writeScopeToUrl('all');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    let active = true;
    setPageState({ status: 'loading' });
    void loadKnowledgeBaseListPage(client, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setPageState(next);
    }).catch(() => { /* aborted */ });
    return () => { active = false; };
  }, [client, reloadToken, scopeController, scope.scope, scope.signal]);

  // Vue renders the empty state on list failure (fetchList never surfaces an
  // error view, KnowledgeBaseList.vue:1227-1242); keep a console breadcrumb
  // for diagnostics but never leak the raw payload into the UI.
  useEffect(() => {
    if (pageState.status === 'error') console.error('[kb-list] load failed:', pageState.message);
  }, [pageState]);

  // Merged (owned + shared) cards drive the "all" scope and the rail counts.
  const mergedCards = useMemo<MergedKnowledgeBase[]>(() => {
    if (pageState.status !== 'success') return [];
    return mergeAllScopeKnowledgeBases(pageState.owned, pageState.shared, viewer.userId || undefined);
  }, [pageState, viewer.userId]);

  // Vue sharedCountByOrg (KnowledgeBaseList.vue:966-985) groups share rows by
  // organization_id for the rail counts. The React share rows carry
  // organization_id + org_name and merged shared cards keep share_id, so the
  // rail entries and the ?scope=<orgId> filter join locally — no new API.
  const orgShareData = useMemo(() => {
    const countByOrg = new Map<string, { id: string; name: string; count: number }>();
    const orgIdByShareId = new Map<string, string>();
    if (pageState.status === 'success') {
      for (const entry of pageState.shared) {
        const row = entry as Record<string, unknown>;
        const orgId = typeof row.organization_id === 'string' ? row.organization_id : '';
        if (!orgId) continue;
        if (typeof row.share_id === 'string') orgIdByShareId.set(row.share_id, orgId);
        const name = typeof row.org_name === 'string' && row.org_name ? row.org_name : orgId;
        const bucket = countByOrg.get(orgId);
        if (bucket) bucket.count += 1;
        else countByOrg.set(orgId, { id: orgId, name, count: 1 });
      }
    }
    // Vue organizationsWithCount (ListSpaceSidebar.vue:271-274): only orgs
    // with a positive count render in the rail.
    return { orgs: [...countByOrg.values()].filter((org) => org.count > 0), orgIdByShareId };
  }, [pageState]);

  const scopedCards = useMemo<MergedKnowledgeBase[]>(() => {
    if (space === 'mine') {
      if (pageState.status !== 'success') return [];
      return pageState.owned.map((kb) => ({ ...kb, isMine: true as const }));
    }
    if (space === 'favorites') return mergedCards.filter((c) => favorites.has(c.id));
    if (space === 'recents') return mergedCards.filter((c) => recents.has(c.id));
    if (isOrgScope(space)) {
      // Vue per-space view (KnowledgeBaseList.vue:908-913): the org's shared
      // KBs, joined back through share_id -> organization_id.
      return mergedCards.filter((card) => card.isMine === false && orgShareData.orgIdByShareId.get(card.share_id) === space);
    }
    return mergedCards;
  }, [favorites, mergedCards, orgShareData, pageState, recents, space]);

  // ?q= keeps working as a deep link even though the page renders no search
  // box (Vue has none on this surface; search lives in the command palette).
  const filtered = useMemo(() => filterKnowledgeBases(scopedCards, {
    query,
    currentUserId: viewer.userId || undefined,
    pageSize: Number.MAX_SAFE_INTEGER,
  }), [query, scopedCards, viewer.userId]);

  // Vue KnowledgeBaseList.vue:97-185 — collapsible section headers render
  // inline in the grid for every scope (pinned / mine / tenant / shared).
  const sections = useMemo(() => {
    if (pageState.status !== 'success' || isOrgScope(space)) return [];
    // Vue tenantSectionLabelKey: contributor/viewer read「本空间 · 仅查看」.
    return groupKnowledgeBaseSections(filtered.items, viewer.userId || undefined, { tenantReadonly: !viewer.isAdmin });
  }, [filtered.items, pageState, space, viewer.userId, viewer.isAdmin]);

  const rows = useMemo<KbListRow[]>(() => {
    if (sections.length === 0) return filtered.items.map((card) => ({ kind: 'card' as const, card }));
    const out: KbListRow[] = [];
    for (const section of sections) {
      const collapsed = collapsedSections.has(section.key);
      out.push({ kind: 'header', key: section.key, labelKey: section.labelKey, count: section.items.length, expanded: !collapsed });
      if (!collapsed) for (const item of section.items) out.push({ kind: 'card', card: item });
    }
    return out;
  }, [collapsedSections, filtered.items, sections]);

  const toggleSection = (key: string) => {
    setCollapsedSections((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  // Close the per-card more menu on any outside click / Escape.
  useEffect(() => {
    if (!menuFor) return;
    const close = () => setMenuFor(null);
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') setMenuFor(null); };
    window.addEventListener('click', close);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('click', close);
      window.removeEventListener('keydown', onKey);
    };
  }, [menuFor]);

  // R009 rail drag-expand (Vue ListSpaceSidebar.vue:160-235): the icon strip
  // and the expanded nav panel are two states of one rail. The right-edge
  // resize handle drags the width between 56 and 228; on release it snaps to
  // 208px (expanded) when the dragged width reached 120, else back to 56.
  // The expanded state persists under the same storage key Vue uses
  // (collapsedKey default 'sidebar-collapsed-list' + '-expanded').
  const KB_RAIL_STORAGE_KEY = 'sidebar-collapsed-list-expanded';
  const KB_RAIL_COLLAPSED_WIDTH = 56;
  const KB_RAIL_EXPANDED_WIDTH = 208;
  const KB_RAIL_SNAP_THRESHOLD = 120;
  const KB_RAIL_MAX_DRAG_WIDTH = KB_RAIL_EXPANDED_WIDTH + 20;

  const [railExpanded, setRailExpanded] = useState(() => {
    try { return window.localStorage.getItem(KB_RAIL_STORAGE_KEY) === 'true'; } catch { return false; }
  });
  const [railDragging, setRailDragging] = useState(false);
  const [railDragWidth, setRailDragWidth] = useState<number | null>(null);
  const railDrag = useRef<{ startX: number; startWidth: number; width: number } | null>(null);

  const onRailDragStart = (event: ReactMouseEvent<HTMLElement>) => {
    event.preventDefault();
    const startWidth = railExpanded ? KB_RAIL_EXPANDED_WIDTH : KB_RAIL_COLLAPSED_WIDTH;
    railDrag.current = { startX: event.clientX, startWidth, width: startWidth };
    setRailDragWidth(startWidth);
    setRailDragging(true);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  };

  useEffect(() => {
    if (!railDragging) return;
    const onMove = (event: MouseEvent) => {
      const drag = railDrag.current;
      if (!drag) return;
      drag.width = Math.max(KB_RAIL_COLLAPSED_WIDTH, Math.min(KB_RAIL_MAX_DRAG_WIDTH, drag.startWidth + (event.clientX - drag.startX)));
      setRailDragWidth(drag.width);
    };
    const onUp = () => {
      const width = railDrag.current?.width ?? KB_RAIL_COLLAPSED_WIDTH;
      railDrag.current = null;
      const shouldExpand = width >= KB_RAIL_SNAP_THRESHOLD;
      setRailExpanded(shouldExpand);
      try { window.localStorage.setItem(KB_RAIL_STORAGE_KEY, String(shouldExpand)); } catch { /* storage unavailable */ }
      setRailDragging(false);
      setRailDragWidth(null);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
    return () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [railDragging]);

  const railItems: { key: KbListSpace; label: string; icon: KbIconName; count: number; org?: { id: string; name: string } }[] = [
    { key: 'all', label: t('listSpaceSidebar.all'), icon: 'layers', count: mergedCards.length },
    { key: 'favorites', label: t('listSpaceSidebar.favorites'), icon: 'star', count: favorites.size },
    { key: 'recents', label: t('listSpaceSidebar.recents'), icon: 'history', count: recents.size },
    { key: 'mine', label: t('listSpaceSidebar.workspace'), icon: 'workspace', count: pageState.status === 'success' ? pageState.owned.length : 0 },
    // Vue shared-spaces group (ListSpaceSidebar.vue:111-125): per-org
    // entries below the workspace bucket, positive counts only, click ->
    // ?scope=<orgId> per-space view (KnowledgeBaseList.vue:897-913).
    ...orgShareData.orgs.map((org) => ({ key: org.id, label: org.name, icon: 'workspace' as KbIconName, count: org.count, org: { id: org.id, name: org.name } })),
  ];

  useEffect(() => {
    type UploadEventDetail = { uploadId?: string; kbId?: string | number; fileName?: string; progress?: number; status?: UploadTaskState['status']; error?: string };
    const addOrPatch = (event: Event, fallbackStatus: UploadTaskState['status']) => {
      const detail = (event as CustomEvent<UploadEventDetail>).detail;
      if (!detail?.uploadId || detail.kbId === undefined) return;
      const task: UploadTaskState = {
        uploadId: detail.uploadId,
        kbId: String(detail.kbId),
        fileName: detail.fileName,
        progress: typeof detail.progress === 'number' ? detail.progress : 0,
        status: detail.status ?? fallbackStatus,
        error: detail.error,
      };
      setUploadTasks((current) => upsertUploadTask(current, task));
    };
    const onStart = (event: Event) => addOrPatch(event, 'uploading');
    const onProgress = (event: Event) => {
      const detail = (event as CustomEvent<UploadEventDetail>).detail;
      if (!detail?.uploadId || typeof detail.progress !== 'number') return;
      const uploadId = detail.uploadId;
      const progress = detail.progress;
      if (detail.kbId === undefined) return;
      setUploadTasks((current) => current.some((task) => task.uploadId === uploadId)
        ? patchUploadTask(current, uploadId, { progress })
        : upsertUploadTask(current, { uploadId, kbId: String(detail.kbId), progress, status: 'uploading' }));
    };
    const onComplete = (event: Event) => {
      const detail = (event as CustomEvent<UploadEventDetail>).detail;
      if (!detail?.uploadId || detail.kbId === undefined) return;
      setUploadTasks((current) => upsertUploadTask(current, {
        uploadId: detail.uploadId!, kbId: String(detail.kbId), fileName: detail.fileName,
        progress: typeof detail.progress === 'number' ? detail.progress : 100,
        status: detail.status ?? 'success', error: detail.error,
      }));
      const existing = uploadCleanupTimers.current.get(detail.uploadId);
      if (existing) clearTimeout(existing);
      const timer = setTimeout(() => {
        setUploadTasks((current) => current.filter((task) => task.uploadId !== detail.uploadId));
        uploadCleanupTimers.current.delete(detail.uploadId!);
      }, 10000);
      uploadCleanupTimers.current.set(detail.uploadId, timer);
    };
    const onFinished = (event: Event) => {
      const detail = (event as CustomEvent<{ kbId?: string | number }>).detail;
      if (detail?.kbId === undefined) return;
      if (uploadRefreshTimer.current) clearTimeout(uploadRefreshTimer.current);
      uploadRefreshTimer.current = setTimeout(() => {
        setReloadToken((value) => value + 1);
        uploadRefreshTimer.current = null;
      }, 800);
    };
    window.addEventListener('knowledgeFileUploadStart', onStart);
    window.addEventListener('knowledgeFileUploadProgress', onProgress);
    window.addEventListener('knowledgeFileUploadComplete', onComplete);
    window.addEventListener('knowledgeFileUploaded', onFinished);
    return () => {
      window.removeEventListener('knowledgeFileUploadStart', onStart);
      window.removeEventListener('knowledgeFileUploadProgress', onProgress);
      window.removeEventListener('knowledgeFileUploadComplete', onComplete);
      window.removeEventListener('knowledgeFileUploaded', onFinished);
      uploadCleanupTimers.current.forEach((timer) => clearTimeout(timer));
      uploadCleanupTimers.current.clear();
      if (uploadRefreshTimer.current) clearTimeout(uploadRefreshTimer.current);
      uploadRefreshTimer.current = null;
    };
  }, []);

  const uploadSummaries = useMemo(() => summarizeUploadTasks(uploadTasks, (kbId) => {
    const match = mergedCards.find((card) => String(card.id) === kbId);
    return match ? String(match.name ?? '') : t('knowledgeList.uploadProgress.unknownKb', { id: kbId });
  }), [mergedCards, t, uploadTasks]);

  useEffect(() => {
    if (!highlightId || pageState.status !== 'success') return;
    const escaped = typeof CSS !== 'undefined' && typeof CSS.escape === 'function' ? CSS.escape(highlightId) : highlightId.replace(/[^a-zA-Z0-9_-]/g, '\\$&');
    const element = document.querySelector<HTMLElement>(`[data-kb-id="${escaped}"]`);
    if (element) element.scrollIntoView({ behavior: 'smooth', block: 'center' });
    const url = new URL(window.location.href);
    url.searchParams.delete('highlightKbId');
    window.history.replaceState({}, '', url);
    const timer = setTimeout(() => {
      setHighlightId(null);
    }, 3000);
    return () => clearTimeout(timer);
  }, [highlightId, pageState.status]);

  const hasUninitialized = useMemo(
    () => pageState.status === 'success' && pageState.owned.some((kb) => !isKnowledgeBaseInitialized(kb as never)),
    [pageState],
  );

  // Vue KnowledgeBaseList.vue:1195-1197 — the kbList tour's trigger condition
  // (contributor-visible empty list, editor closed). The shell-level guide
  // host applies the dismissal + welcome-tour gates.
  const kbListGuideWhen = pageState.status === 'success'
    && viewer.isContributor
    && !dialogOpen
    && (space === 'all' || space === 'mine')
    && scopedCards.length === 0;
  useEffect(() => {
    if (!kbListGuideWhen) return;
    openContextualGuide('kbList');
  }, [kbListGuideWhen]);

  const toggleFavorite = (kbId: string) => {
    setFavorites((current) => {
      const next = new Set(current);
      if (next.has(kbId)) next.delete(kbId);
      else next.add(kbId);
      try { window.localStorage.setItem('wk-kb-favorites', JSON.stringify([...next])); } catch { /* storage unavailable */ }
      return next;
    });
  };

  async function loadEditorOptions() {
    setEditorOptions((current) => ({ ...current, loading: true, error: null }));
    const [parser, storage, vector, models] = await Promise.allSettled([
      client.knowledgeBases.settings.parserEngines(),
      client.knowledgeBases.settings.storageBackends(),
      client.knowledgeBases.settings.vectorStores(),
      // R488: model rows for the ModelSelector dropdowns (Vue loads the same
      // list inside loadKBData). A rejected fetch only leaves the dropdowns
      // empty — it must not fail the editor open like the settings trio.
      client.configuration.models.list(),
    ]);
    if (parser.status === 'rejected' && storage.status === 'rejected' && vector.status === 'rejected') {
      setEditorOptions((current) => ({ ...current, loading: false, error: t('knowledgeEditor.messages.loadDataFailed') }));
      return;
    }
    const nextModelRows = models.status === 'fulfilled' ? models.value : [];
    setModelRows(nextModelRows);
    const defaultStorageId = storage.status === 'fulfilled' && typeof storage.value.default_storage_backend_id === 'string' ? storage.value.default_storage_backend_id : '';
    setEditorOptions({
      parserEngines: parser.status === 'fulfilled' ? parser.value.data.map((item) => ({ Name: item.Name, Description: item.Description, ...(item.Available === undefined ? {} : { Available: item.Available }) })) : [],
      // Vue KBStorageSettings filters listStorageBackends() to status === 'active'.
      storageBackends: storage.status === 'fulfilled' ? storage.value.data.filter((item) => item.status === 'active').map((item) => ({ id: item.id, name: item.name, provider: item.provider, status: item.status, config: item.config })) : [],
      // Vue KBStorageSettings reads response.default_storage_backend_id to tag
      // the default instance and preselect it on a fresh create.
      defaultStorageBackendId: defaultStorageId,
      vectorStores: vector.status === 'fulfilled' ? vector.value.data.map((item) => ({ id: item.id, name: item.name, engine_type: item.engine_type, source: item.source, readonly: item.readonly })) : [],
      loading: false,
      error: null,
    });
    // Vue KBStorageSettings load(): when no backend is bound yet the tenant
    // default (or the first active row) preselects and emits the binding —
    // both create mode and an edit-mode KB with no stored backend.
    if (defaultStorageId) {
      setEditorConfig((current) => (current.storageBackendId ? current : { ...current, storageBackendId: defaultStorageId }));
    }
  }

  async function loadEditorActivity(id: string) {
    setEditorActivityLoading(true);
    try {
      const result = await client.knowledgeBases.settings.activity(id);
      setEditorActivity((result.data ?? []).map((entry) => ({ id: entry.id, action: entry.action, outcome: entry.outcome, created_at: entry.created_at })));
    } catch {
      setEditorActivity([]);
    } finally {
      setEditorActivityLoading(false);
    }
  }

  // R488: model dropdown options + the Vue KBModelConfig.vue:33 embedding
  // required rule (ragEnabled = vector OR keyword indexing on a document KB).
  const modelsEmbeddingRequired = type === 'document' && (indexingStrategy.vector_enabled || indexingStrategy.keyword_enabled);
  const chatModelOptions: ModelOption[] = resolveChatModelOptions(listChatModels(modelRows)).map((option) => ({ value: option.id, label: option.name }));
  const embeddingModelOptions: ModelOption[] = modelRows
    .filter((row) => row.type === 'Embedding')
    .map((row) => ({ value: row.id, label: String(row.display_name || row.name || row.id) }));

  function openCreate() {
    editLoadGeneration.current += 1; // retire any in-flight edit hydration
    setEditingId(null);
    setName('');
    setDescription('');
    setType('document');
    setEmbeddingModelId('');
    setSummaryModelId('');
    setIndexingStrategy({ ...DEFAULT_KB_INDEXING });
    setEditorConfig(defaultKnowledgeEditorConfig());
    setEditingHasFiles(false);
    void loadEditorOptions();
    setEditorSection('basic');
    // Vue KnowledgeBaseList.vue:1696 — opening the create wizard retires the
    // empty-list kbList tour; KnowledgeBaseEditorModal.vue:464 arms the
    // kbCreate tour for document KBs (the React dialog always exposes the
    // embedding field, so needsEmbedding follows the document type default).
    markContextualGuideDone(window.localStorage, 'kbList');
    openContextualGuide('kbCreate', { isFaq: false, needsEmbedding: true });
    setDialogOpen(true);
  }

  function openEdit(kb: Record<string, unknown>) {
    setEditingId(String(kb.id));
    setName(String(kb.name ?? ''));
    setDescription(String(kb.description ?? ''));
    setType(kb.type === 'faq' ? 'faq' : 'document');
    setEmbeddingModelId(String(kb.embedding_model_id ?? ''));
    setSummaryModelId(String(kb.summary_model_id ?? ''));
    const serverStrategy = kb.indexing_strategy as Partial<KbIndexingStrategy> | null | undefined;
    setIndexingStrategy({
      vector_enabled: serverStrategy?.vector_enabled ?? true,
      keyword_enabled: serverStrategy?.keyword_enabled ?? true,
      wiki_enabled: serverStrategy?.wiki_enabled ?? false,
      graph_enabled: serverStrategy?.graph_enabled ?? false,
    });
    setEditorConfig(hydrateKnowledgeEditorConfig(kb));
    setEditingHasFiles(false);
    // R490 A3① (Vue loadKBData, KnowledgeBaseEditorModal.vue:831-969): the
    // edit form hydrates from GET /knowledge-bases/:id plus a 1-row
    // listKnowledgeFiles probe — the list row is only a first paint. The
    // detail response is what fills the LLM selector (kb.summary_model_id —
    // an unconfigured KB must NOT inherit any global default) and the file
    // probe drives the Embedding/indexing locks (hasFiles).
    const editGeneration = ++editLoadGeneration.current;
    void (async () => {
      try {
        const [detail, files] = await Promise.allSettled([
          client.knowledgeBases.settings.get(String(kb.id)),
          client.knowledgeBases.documents.list(String(kb.id), { page: 1, page_size: 1 }),
        ]);
        if (editGeneration !== editLoadGeneration.current) return; // a newer open took over
        const fileTotal = files.status === 'fulfilled' ? Number(files.value.total ?? files.value.data?.length ?? 0) : 0;
        setEditingHasFiles(fileTotal > 0);
        if (detail.status !== 'fulfilled') return; // keep the list-row first paint
        const row = detail.value as Record<string, unknown>;
        setSummaryModelId(typeof row.summary_model_id === 'string' ? row.summary_model_id : '');
        setEmbeddingModelId(typeof row.embedding_model_id === 'string' ? row.embedding_model_id : '');
      } catch { /* keep the list-row first paint (degraded client surface) */ }
    })();
    void loadEditorOptions();
    setEditorSection('basic');
    setDialogOpen(true);
  }

  // Vue KnowledgeBaseEditorModal.vue:1335-1365 handleSubmit → doSubmit: the
  // footer button is a plain @click handler (no native form), so the save
  // pipeline is entered from onClick and guards itself (double-submit,
  // blank-name, section-jumping validation all live here).
  async function save() {
    if (saving) return; // double-submit guard
    if (!name.trim()) return; // blank names are blocked
    if (type === 'document' && !Object.values(indexingStrategy).some(Boolean)) {
      setError(t('knowledgeEditor.indexing.atLeastOne'));
      return;
    }
    if (type === 'document' && (indexingStrategy.vector_enabled || indexingStrategy.keyword_enabled) && !embeddingModelId) {
      setError(t('knowledgeEditor.indexing.embeddingRequired'));
      return;
    }
    // R488: Vue validateForm (KnowledgeBaseEditorModal.vue:1170-1174) requires
    // the summary model for BOTH document and FAQ editors, jumps to the models
    // section and shows summaryRequired — before this the React form left a
    // free-text blank behind a 可选 placeholder and only failed inside the
    // mutation boundary with a raw English error.
    if (!summaryModelId) {
      setEditorSection('models');
      setError(t('knowledgeEditor.messages.summaryRequired'));
      return;
    }
    if (type === 'faq' && !editorConfig.faqConfig.indexMode) {
      setEditorSection('faq');
      setError(t('knowledgeEditor.messages.indexModeRequired'));
      return;
    }
    if (type === 'document' && editorConfig.multimodalConfig.enabled && !editorConfig.multimodalConfig.vllmModelId.trim()) {
      setEditorSection('multimodal');
      setError(t('knowledgeEditor.messages.multimodalInvalid'));
      return;
    }
    setSaving(true);
    setError(null);
    const input: KnowledgeBaseSaveInput = {
      name: name.trim(),
      type,
      ...(description.trim() ? { description: description.trim() } : {}),
      ...(embeddingModelId ? { embedding_model_id: embeddingModelId } : {}),
      ...(summaryModelId ? { summary_model_id: summaryModelId } : {}),
      ...knowledgeEditorConfigPayload(editorConfig, type),
      ...(type === 'document' ? { indexing_strategy: { ...indexingStrategy, graph_enabled: editorConfig.nodeExtractConfig.enabled } } : {}),
    };
    try {
      const record = await saveKnowledgeBase(client, editingId, input) as { id?: unknown };
      // Vue KnowledgeBaseEditorModal.vue:1386 — a successful editor run retires
      // the kbCreate tour.
      markContextualGuideDone(window.localStorage, 'kbCreate');
      setDialogOpen(false);
      setEditingId(null);
      setReloadToken((value) => value + 1);
      // Vue KnowledgeBaseList.vue handleKBEditorSuccess: when the kbDetail
      // upload tour has not run yet, land on the new KB's detail page where
      // the guide arms (queue the intent for the next document).
      if (editingId === null && record?.id !== undefined && !isContextualGuideDone(window.localStorage, 'kbDetail')) {
        openContextualGuide('kbDetail');
        navigate(knowledgeBaseDetailPath(String(record.id)));
      }
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : t('knowledgeList.messages.deleteFailed'));
    } finally {
      setSaving(false);
    }
  }

  async function confirmDelete() {
    if (!deletingKb || deleting) return; // exactly one DELETE per confirmed action
    setDeleting(true);
    setError(null);
    try {
      const result = await deleteGuard.current.confirm(deletingKb.id);
      if (result === 'deleted') {
        setDeletingKb(null);
        setNotice(t('knowledgeList.messages.deleted'));
        setReloadToken((value) => value + 1);
      }
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : t('knowledgeList.messages.deleteFailed'));
    } finally {
      setDeleting(false);
    }
  }

  async function togglePin(kb: Record<string, unknown>) {
    setError(null);
    try {
      const result = await client.knowledgeBases.togglePin(String(kb.id));
      setNotice(t(result.is_pinned ? 'knowledgeList.pin.pinSuccess' : 'knowledgeList.pin.unpinSuccess'));
      setReloadToken((value) => value + 1);
    } catch {
      setError(t('knowledgeList.pin.failed'));
    }
  }

  async function duplicate(kb: Record<string, unknown>) {
    setError(null);
    try {
      await client.knowledgeBases.duplicate(String(kb.id));
      setNotice(t('knowledgeList.messages.duplicateSuccess'));
      setReloadToken((value) => value + 1);
    } catch {
      setError(t('knowledgeList.messages.duplicateFailed'));
    }
  }

  // Vue handleSettingsById → open the in-place KnowledgeBaseEditorModal.vue.
  // Keep the list/detail context visible behind the modal; a full-page
  // settings URL remains available for historical/deep-link callers, but the
  // primary list action must preserve Vue's overlay interaction.
  function openKbSettings(kb: Record<string, unknown>) {
    openEdit(kb);
  }

  // Vue openSharedDetailFromAll (KnowledgeBaseList.vue:1499-1512): re-find the
  // raw share row (knowledge_base / source_from_agent / shared_at) by share_id
  // so the drawer shows the share-entry fields the merged card flattens away.
  function openSharedDetail(card: MergedKnowledgeBase) {
    const rows = pageState.status === 'success' ? (pageState.shared as Array<Record<string, unknown>>) : [];
    const raw = typeof card.share_id === 'string' ? rows.find((row) => row.share_id === card.share_id) : undefined;
    setSharedDetail(raw ?? {
      name: card.name,
      permission: card.permission,
      shared_at: card.shared_at,
      org_name: typeof card.org_name === 'string' ? card.org_name : undefined,
      knowledge_base: { id: card.id },
    });
  }

  function openCard(kb: Record<string, unknown>) {
    const id = String(kb.id);
    try {
      const raw = window.localStorage.getItem('wk-kb-recents');
      const list: string[] = raw ? JSON.parse(raw) : [];
      const next = [id, ...list.filter((v) => v !== id)].slice(0, 20);
      window.localStorage.setItem('wk-kb-recents', JSON.stringify(next));
      setRecents(new Set(next));
    } catch { /* storage unavailable */ }
    if (isKnowledgeBaseInitialized(kb as never)) {
      // Vue KnowledgeBase.vue:339-345 — the kbDetail tour arms on detail entry
      // for an editable, non-FAQ, empty knowledge base. The React detail page
      // is a separate document, so queue the intent for its shell host.
      const count = typeof kb.knowledge_count === 'number' ? kb.knowledge_count : 0;
      if (kb.type !== 'faq' && count === 0 && canManageKBCard(kb, { userId: viewer.userId, isAdmin: viewer.isAdmin })) {
        openContextualGuide('kbDetail');
      }
      navigate(knowledgeBaseDetailPath(id));
      return;
    }
    // Vue handleCardClick else-branch: uninitialized card click opens the
    // same in-place settings modal without a tenant-model detour.
    openEdit(kb);
  }

  const isLoading = pageState.status === 'loading';
  const listVisible = pageState.status === 'success' && filtered.total > 0;
  // Vue authority: a failed list fetch renders the same empty state as an
  // empty result (KnowledgeBaseList.vue:633-643) — no raw error output.
  const emptyVisible = pageState.status === 'error' || (pageState.status === 'success' && filtered.total === 0);

  return (
    <main className="wk-page kb-list-page box-border flex h-full w-full min-w-0 flex-1 flex-col overflow-y-auto">
      <div className="kb-list-container relative flex min-h-0 flex-1">
        {/* Vue ListSpaceSidebar dual state: collapsed icon strip ↔ expanded
            nav panel, toggled by dragging the right-edge resize handle
            (ListSpaceSidebar.vue:2-150). The collapsed strip keeps the
            label (count) tooltip; the expanded panel shows the full label
            plus a count badge (Vue .expanded-panel, :77-144). */}
        <aside
          className={[
            'kb-list-rail group/rail relative z-10 min-h-0 w-14 shrink-0 transition-[width] duration-[250ms] ease-[cubic-bezier(0.4,0,0.2,1)]',
            railExpanded ? 'kb-list-rail-expanded w-[208px]' : '',
            railDragging ? 'kb-list-rail-dragging transition-none' : '',
          ].filter(Boolean).join(' ')}
          aria-label={t('common.knowledgeBases')}
          style={railDragging && railDragWidth !== null ? { width: railDragWidth } : undefined}
        >
          {railExpanded ? (
            <nav className="kb-list-rail-panel flex min-h-0 w-full flex-1 flex-col items-stretch gap-0.5 overflow-y-auto overflow-x-hidden border-r border-[#e3e7ee] px-2 py-3 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
              {railItems.map((item, index) => (
                <Fragment key={item.key}>
                  {index === 3 ? <div className="kb-list-rail-divider mx-1 my-1.5 h-px shrink-0 bg-[#f3f3f3]" aria-hidden="true" /> : null}
                  {index === 4 && item.org ? <div className="kb-list-rail-section-title mt-0.5 shrink-0 overflow-hidden border-t border-[#e3e7ee] px-2 pt-2 pb-0.5 text-left text-xs font-semibold leading-[1.4] text-[#646e74]">{t('listSpaceSidebar.spaces')}</div> : null}
                  <button
                    type="button"
                    className={space === item.key
                      ? 'kb-list-rail-panel-item kb-list-rail-panel-item-active group/item flex shrink-0 cursor-pointer items-center justify-between rounded-[7px] border-0 bg-[#f3f3f3] px-2 py-1.5 text-left text-sm font-[inherit] text-[#07c05f] transition-all duration-150 hover:bg-[#f3f3f3]'
                      : 'kb-list-rail-panel-item group/item flex shrink-0 cursor-pointer items-center justify-between rounded-[7px] border-0 bg-transparent px-2 py-1.5 text-left text-sm font-[inherit] text-[#1d2129] transition-all duration-150 hover:bg-[#f3f3f3]'}
                    aria-pressed={space === item.key}
                    onClick={() => setSpace(item.key)}
                  >
                    <span className="kb-list-rail-panel-left flex min-w-0 flex-1 items-center gap-1.5">
                      <KbIcon name={item.icon} size={16} />
                      <span className="kb-list-rail-panel-label min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-left text-[13px] leading-[1.4]" title={item.org ? item.org.name : undefined}>{item.label}</span>
                    </span>
                    {railCountVisible(item.key, item.count) ? <span className={`kb-list-rail-panel-count ml-1.5 shrink-0 rounded-lg bg-[#f3f3f3] px-[7px] py-0.5 text-xs font-medium transition-all duration-150 ${space === item.key ? 'text-[#07c05f]' : 'text-[#646e74] group-hover/item:text-[#1d2129]'}`}>{item.count}</span> : null}
                  </button>
                </Fragment>
              ))}
            </nav>
          ) : (
            <div className="kb-list-rail-strip flex min-h-0 flex-1 flex-col items-center gap-1 overflow-y-auto overflow-x-hidden pt-3 pb-1.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
              {railItems.map((item, index) => (
                <Fragment key={item.key}>
                  {index === 4 && item.org ? <div className="kb-list-rail-divider kb-list-rail-strip-divider my-[3px] h-px w-6 shrink-0 bg-[#f3f3f3]" aria-hidden="true" /> : null}
                  <button
                    key={item.key}
                    type="button"
                    className={space === item.key
                      ? 'kb-list-rail-item kb-list-rail-item-active flex w-[46px] shrink-0 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-lg border-0 bg-[#f3f3f3] pt-[5px] pb-0.5 font-[inherit] text-[#07c05f] transition-all duration-150 hover:bg-[#f3f3f3] hover:text-[#07c05f]'
                      : 'kb-list-rail-item flex w-[46px] shrink-0 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-lg border-0 bg-transparent pt-[5px] pb-0.5 font-[inherit] text-[#646e74] transition-all duration-150 hover:bg-[#f3f3f3] hover:text-[#1d2129]'}
                    title={`${item.label} (${item.count})`}
                    aria-pressed={space === item.key}
                    onClick={() => setSpace(item.key)}
                  >
                    <KbIcon name={item.icon} size={16} />
                    <span className="kb-list-rail-label max-w-[52px] overflow-hidden text-ellipsis whitespace-nowrap text-center text-[11px] leading-[1.25]">{item.org ? truncateRailLabel(item.org.name) : item.label}</span>
                  </button>
                </Fragment>
              ))}
            </div>
          )}
          {/* Vue .resize-handle (:146-149): mousedown starts the drag; the
              mouseup snap threshold (>= 120px) decides expand vs collapse. */}
          <div
            className="kb-list-rail-handle group/handle absolute bottom-0 right-[-6px] top-0 z-[12] flex w-3 cursor-col-resize items-center justify-center"
            role="separator"
            aria-orientation="vertical"
            onMouseDown={onRailDragStart}
          >
            <div className={`kb-list-rail-handle-line h-10 w-0.5 rounded-[1px] transition-[opacity,background] duration-200 ease-[ease] ${railDragging ? 'bg-[#07c05f] opacity-100' : 'bg-[#eeeeee] opacity-[0.45] group-hover/handle:bg-[#07c05f] group-hover/handle:opacity-100'}`} />
          </div>
        </aside>
        <div className="kb-list-content flex min-w-0 flex-1 flex-col pt-5 pl-7">
          {/* Vue header: title + 28x28 create icon button + subtitle */}
          <header className="kb-list-header mb-4">
            <div className="flex items-center gap-2">
              <h1 className="m-0 text-2xl font-semibold leading-8 text-[#17233d]">{t('common.knowledgeBases')}</h1>
              {viewer.isContributor ? (
                <button
                  type="button"
                  className="kb-list-header-action inline-flex h-7 w-7 min-w-[28px] cursor-pointer items-center justify-center rounded-md border border-[#e3e7ee] bg-[#f2f4f8] p-0 text-[#646e74] transition-colors duration-200 hover:text-[#1d2129] [&_svg]:text-[#07c05f]"
                  data-guide="kb-list-create"
                  title={t('knowledgeList.create')}
                  aria-label={t('knowledgeList.create')}
                  onClick={openCreate}
                >
                  <KbIcon name="folder-add" size={16} />
                </button>
              ) : null}
            </div>
            <p className="kb-list-subtitle mb-0 mt-1 text-sm font-normal leading-5 text-[#8a94a6]">{t('knowledgeList.subtitle')}</p>
          </header>
          <div className="kb-list-main min-w-0 flex-1 overflow-y-auto overflow-x-hidden pb-2 pr-7">
            {error ? <Status tone="error">{error}</Status> : null}
            {notice ? <Status tone="success">{notice}</Status> : null}
            {/* Vue amber uninitialized banner (KnowledgeBaseList.vue:30-34) */}
            {hasUninitialized ? (
              <div className="kb-list-warning mb-5 flex items-center gap-2 rounded-md border border-[#f9e0c7] bg-[#fef3e6] px-4 py-3 text-sm leading-5 text-[#ed7b2f] [&_svg]:shrink-0" role="status">
                <KbIcon name="info-circle" size={16} />
                <span>{t('knowledgeList.uninitializedBanner')}</span>
              </div>
            ) : null}
            {uploadSummaries.length ? <div className="wk-upload-progress-panel mb-5 flex flex-col gap-3" aria-live="polite">
              {uploadSummaries.map((summary) => <div className="wk-upload-progress-item flex items-center gap-3 rounded-lg border border-[#e3e7ee] bg-white px-4 py-3" key={summary.kbId}>
                <div className="wk-upload-progress-icon flex flex-none items-center justify-center text-[#07c05f]" aria-hidden="true"><KbIcon name={summary.completed === summary.total ? 'check-circle-filled' : 'upload'} size={20} /></div>
                <div className="wk-upload-progress-content min-w-0 flex-1">
                  <div className="wk-upload-progress-title mb-0.5 text-sm font-semibold leading-[22px] text-[#1d2129]">{summary.completed === summary.total ? t('knowledgeList.uploadProgress.completedTitle', { name: summary.kbName }) : t('knowledgeList.uploadProgress.uploadingTitle', { name: summary.kbName })}</div>
                  <div className="wk-upload-progress-subtitle text-xs leading-[18px] text-[#646e74]">{summary.completed === summary.total ? t('knowledgeList.uploadProgress.completedDetail', { total: summary.total }) : t('knowledgeList.uploadProgress.detail', { completed: summary.completed, total: summary.total })}</div>
                  <div className="wk-upload-progress-subtitle mt-0.5 text-xs leading-[18px] text-[#8a94a6]">{summary.completed === summary.total ? t('knowledgeList.uploadProgress.refreshing') : t('knowledgeList.uploadProgress.keepPageOpen')}</div>
                  {summary.hasError ? <div className="wk-upload-progress-subtitle wk-upload-progress-error mt-1 text-xs leading-[18px] text-[#e34d59]">{t('knowledgeList.uploadProgress.errorTip')}</div> : null}
                  <div className="wk-upload-progress-track mt-[10px] h-[6px] overflow-hidden rounded-full bg-[#f3f3f3]" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={summary.progress}><div className="wk-upload-progress-fill h-full rounded-full bg-[linear-gradient(90deg,#06b04d_0%,#07c05f_100%)] [transition:width_0.2s_ease]" style={{ width: `${summary.progress}%` }} /></div>
                </div>
              </div>)}
            </div> : null}
            {isLoading ? (
              <div className="kb-list-grid grid grid-cols-1 gap-3 motion-safe:animate-[kbListFadeIn_0.32s_ease-out] min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 min-[2200px]:grid-cols-6" aria-busy="true" aria-label={t('common.loading')}>
                {Array.from({ length: SKELETON_CARD_COUNT }, (_, index) => <div className="kb-list-skeleton h-[136px] rounded-lg border border-[#e3e7ee] bg-[linear-gradient(100deg,#f2f4f8_40%,#f8fafc_50%,#f2f4f8_60%)] [background-size:200%_100%] motion-safe:animate-[kbListShimmer_1.4s_ease_infinite]" key={index} />)}
              </div>
            ) : null}
            {emptyVisible && (space === 'all' || space === 'mine') ? (
              <div className="kb-list-empty flex flex-1 flex-col items-center justify-center px-5 py-[60px] text-center">
                <div className="kb-list-empty-img [&_svg]:mx-auto [&_svg]:mb-5 [&_svg]:block [&_svg]:h-[162px] [&_svg]:w-[162px]" aria-hidden="true" dangerouslySetInnerHTML={{ __html: KB_EMPTY_SVG }} />
                <span className="kb-list-empty-title mb-2 text-base font-semibold leading-[26px] text-[#8a94a6]">{t('knowledgeList.empty.title')}</span>
                <span className="kb-list-empty-desc text-sm font-normal leading-[22px] text-[#a5aebd]">{t('knowledgeList.empty.description')}</span>
                {viewer.isContributor ? <Button type="button" className="kb-list-empty-btn mt-5 border-0 bg-[linear-gradient(135deg,#07c05f_0%,#00a67e_100%)] text-white hover:bg-[linear-gradient(135deg,#07c05f_0%,#069155_100%)]" data-guide="kb-list-create" onClick={openCreate}>{t('knowledgeList.create')}</Button> : null}
              </div>
            ) : null}
            {/* Vue per-space empty state (KnowledgeBaseList.vue:674-678):
                shared title/description + illustration, never the create CTA */}
            {emptyVisible && isOrgScope(space) ? (
              <div className="kb-list-empty flex flex-1 flex-col items-center justify-center px-5 py-[60px] text-center">
                <div className="kb-list-empty-img [&_svg]:mx-auto [&_svg]:mb-5 [&_svg]:block [&_svg]:h-[162px] [&_svg]:w-[162px]" aria-hidden="true" dangerouslySetInnerHTML={{ __html: KB_EMPTY_SVG }} />
                <span className="kb-list-empty-title mb-2 text-base font-semibold leading-[26px] text-[#8a94a6]">{t('knowledgeList.empty.sharedTitle')}</span>
                <span className="kb-list-empty-desc text-sm font-normal leading-[22px] text-[#a5aebd]">{t('knowledgeList.empty.sharedDescription')}</span>
              </div>
            ) : null}
            {/* Vue favorites/recents empty states carry a scope hint, never the
                create CTA (KnowledgeBaseList.vue:645-659) */}
            {emptyVisible && (space === 'favorites' || space === 'recents') ? (
              <div className="kb-list-empty flex flex-1 flex-col items-center justify-center px-5 py-[60px] text-center">
                <span className="kb-list-empty-icon" aria-hidden="true"><KbIcon name={space === 'favorites' ? 'star' : 'history'} size={48} /></span>
                <span className="kb-list-empty-title mb-2 text-base font-semibold leading-[26px] text-[#8a94a6]">{t(space === 'favorites' ? 'knowledgeList.empty.favoritesTitle' : 'knowledgeList.empty.recentsTitle')}</span>
                <span className="kb-list-empty-desc text-sm font-normal leading-[22px] text-[#a5aebd]">{t(space === 'favorites' ? 'knowledgeList.empty.favoritesDescription' : 'knowledgeList.empty.recentsDescription')}</span>
              </div>
            ) : null}
            {listVisible ? (
              <div className="kb-list-grid grid grid-cols-1 gap-3 motion-safe:animate-[kbListFadeIn_0.32s_ease-out] min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 min-[2200px]:grid-cols-6">
                {rows.map((row) => {
                  if (row.kind === 'header') {
                    return (
                      <button
                        key={'sec-' + row.key}
                        type="button"
                        className="kb-list-section-header group/section col-span-full sticky top-0 z-[5] flex w-full cursor-pointer select-none items-center gap-1.5 border-0 bg-white p-1.5 pr-0 pl-0 text-left font-[inherit] text-[13px] font-semibold leading-5 text-[#646e74] shadow-[0_-8px_0_0_#fff,0_4px_0_0_#fff] hover:text-[#1d2129]"
                        aria-expanded={row.expanded}
                        onClick={() => toggleSection(row.key)}
                      >
                        <KbIcon name={row.key === 'tenantOthers' && !viewer.isAdmin ? 'browse' : SECTION_ICONS[row.key] ?? 'user'} size={14} />
                        {SECTION_SUBICONS[row.key] ? <span className="-ml-1 opacity-75" aria-hidden="true"><KbIcon name={SECTION_SUBICONS[row.key]} size={12} /></span> : null}
                        <span>{t(row.labelKey)}</span>
                        <span className="kb-list-section-count ml-0.5 rounded-lg bg-[#f2f4f8] px-1.5 text-[11px] font-medium leading-4 text-[#646e74]">{row.count}</span>
                        <span className="kb-list-section-toggle ml-1 opacity-70 group-hover/section:opacity-100" aria-hidden="true"><KbIcon name={row.expanded ? 'chevron-down' : 'chevron-right'} size={14} /></span>
                      </button>
                    );
                  }
                  const card = row.card;
                  const kb = card as Record<string, unknown>;
                  const initialized = isKnowledgeBaseInitialized(card as never);
                  const manageable = canManageKBCard(kb, { userId: viewer.userId, isAdmin: viewer.isAdmin });
                  const duplicable = canDuplicateKBCard(kb, { userId: viewer.userId, isContributor: viewer.isContributor });
                  const isWiki = (card as { indexing_strategy?: { wiki_enabled?: boolean } }).indexing_strategy?.wiki_enabled === true;
                  const isSharedCard = card.isMine === false;
                  const isFaq = card.type === 'faq';
                  const count = isFaq
                    ? (typeof card.chunk_count === 'number' ? card.chunk_count : 0)
                    : (typeof card.knowledge_count === 'number' ? card.knowledge_count : 0);
                  const extractEnabled = (card as { extract_config?: { enabled?: boolean } }).extract_config?.enabled === true;
                  const vlmEnabled = (card as { vlm_config?: { enabled?: boolean } }).vlm_config?.enabled === true;
                  const questionEnabled = (card as { question_generation_config?: { enabled?: boolean } }).question_generation_config?.enabled === true;
                  const shareCount = typeof card.share_count === 'number' ? card.share_count : 0;
                  const favorited = favorites.has(card.id);
                  // flash 动画的 border 覆盖用 ! 前缀（同属性 utilities 冲突时保证胜出）
                  const cardClasses = [
                    'kb-list-card group/card relative box-border flex h-[136px] min-h-[136px] cursor-pointer flex-col rounded-lg border px-[14px] py-3 shadow-[0_1px_3px_rgba(0,0,0,0.04)] transition-all duration-[250ms]',
                    isFaq
                      ? 'kb-list-card-faq border-[#e3e7ee] bg-[linear-gradient(135deg,#ffffff_0%,rgba(0,82,217,0.04)_100%)] after:pointer-events-none after:absolute after:right-0 after:top-0 after:z-0 after:h-[60px] after:w-[60px] after:rounded-[0_12px_0_100%] after:content-[""] after:bg-[linear-gradient(135deg,rgba(0,82,217,0.08)_0%,transparent_100%)] hover:border-[#07c05f] hover:shadow-[0_4px_12px_rgba(0,82,217,0.12)] hover:bg-[linear-gradient(135deg,#ffffff_0%,rgba(0,82,217,0.08)_100%)]'
                      : 'kb-list-card-document border-[#e3e7ee] bg-[linear-gradient(135deg,#ffffff_0%,rgba(7,192,95,0.04)_100%)] after:pointer-events-none after:absolute after:right-0 after:top-0 after:z-0 after:h-[60px] after:w-[60px] after:rounded-[0_12px_0_100%] after:content-[""] after:bg-[linear-gradient(135deg,rgba(7,192,95,0.08)_0%,transparent_100%)] hover:border-[#07c05f] hover:shadow-[0_4px_12px_rgba(7,192,95,0.12)] hover:bg-[linear-gradient(135deg,#ffffff_0%,rgba(7,192,95,0.08)_100%)]',
                    initialized ? '' : 'kb-list-card-uninitialized opacity-90',
                    highlightId === card.id ? 'kb-list-flash animate-[kbListFlash_0.6s_ease-in-out_3] border-[#07c05f]!' : '',
                  ].filter(Boolean).join(' ');
                  return (
                    <article key={card.id} data-kb-id={card.id} className={cardClasses} onClick={() => openCard(kb)}>
                      <button
                        type="button"
                        className={`kb-favorite-star absolute right-0 top-0 z-[3] flex h-6 w-6 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent transition-[opacity,background,color] duration-150 hover:bg-[#f2f4f8] hover:text-[#e37318] group-hover/card:opacity-100 ${favorited ? 'kb-favorite-star-active text-[#e37318] opacity-100' : 'text-[#8a94a6] opacity-0'}`}
                        aria-label={favorited ? t('knowledgeList.accessibility.unfavorite') : t('knowledgeList.accessibility.favorite')}
                        onClick={(event) => { event.stopPropagation(); toggleFavorite(card.id); }}
                      >
                        <KbIcon name={favorited ? 'star-filled' : 'star'} size={14} />
                      </button>
                      <div className="kb-list-card-head relative z-[1] mb-1.5 flex items-center gap-1">
                        <span className="kb-list-card-title flex min-w-0 flex-1 cursor-pointer items-center gap-1.5 overflow-hidden whitespace-nowrap border-0 bg-transparent p-0 text-left text-[15px] font-semibold leading-[22px] tracking-[0.01em] text-[#1d2129]" title={String(card.name ?? '')}>
                          {isWiki ? <span className="kb-list-card-wiki-chip shrink-0 text-[#646e74]" role="img" aria-label={t('knowledgeList.features.wiki')} title={t('knowledgeList.features.wiki')}><KbIcon name="wiki" size={15} /></span> : null}
                          <span className="kb-list-card-title-text">{String(card.name ?? '')}</span>
                        </span>
                        {/* Vue shared-detail-trigger (KnowledgeBaseList.vue:305-315):
                            info-circle 查看详情 entry on non-own shared cards,
                            replacing the three-dot as the detail affordance.
                            @click.stop keeps card navigation out of the way. */}
                        {isSharedCard ? (
                          <button
                            type="button"
                            className="kb-shared-detail-trigger flex h-7 w-7 shrink-0 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent p-0 text-[#07c05f] transition-colors duration-200 hover:bg-[rgba(7,192,95,0.08)]"
                            title={t('knowledgeList.menu.viewDetails')}
                            aria-label={t('knowledgeList.menu.viewDetails')}
                            onClick={(event) => { event.stopPropagation(); openSharedDetail(card); }}
                          >
                            <KbIcon name="info-circle" size={16} />
                          </button>
                        ) : null}
                        {/* Vue own-card three-dot menu (KnowledgeBaseList.vue) is absent on
                            shared cards (:304-315): the info-circle trigger above replaces it
                            as the only header affordance on non-own shared cards. */}
                        {isSharedCard ? null : (
                          <button
                            type="button"
                            className={`kb-list-card-more flex h-7 w-7 shrink-0 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent transition-all duration-200 group-hover/card:opacity-60 hover:bg-[#edf0f5] hover:opacity-100 hover:text-[#1d2129] ${menuFor === card.id ? 'kb-list-card-more-open bg-[#edf0f5] opacity-100 text-[#1d2129]' : 'opacity-0'}`}
                            aria-label={t('common.settings')}
                            aria-haspopup="menu"
                            aria-expanded={menuFor === card.id}
                            onClick={(event) => { event.stopPropagation(); setMenuFor((current) => (current === card.id ? null : card.id)); }}
                          >
                            <KbIcon name="dots" size={16} />
                          </button>
                        )}
                      </div>
                      {menuFor === card.id ? (
                        <div className="kb-list-more-menu absolute right-1.5 top-8 z-30 flex min-w-[132px] flex-col rounded-lg border border-[#e3e7ee] bg-white p-1.5 shadow-[0_8px_24px_rgba(23,35,61,0.12)]" role="menu" onClick={(event) => event.stopPropagation()}>
                          <button type="button" role="menuitem" onClick={() => { setMenuFor(null); void togglePin(kb); }}>
                            <KbIcon name={card.is_pinned ? 'pin-filled' : 'pin'} size={14} />{card.is_pinned ? t('knowledgeList.pin.unpin') : t('knowledgeList.pin.pin')}
                          </button>
                          {duplicable ? (
                            <button type="button" role="menuitem" onClick={() => { setMenuFor(null); void duplicate(kb); }}>
                              <KbIcon name="copy" size={14} />{t('knowledgeList.menu.duplicate')}
                            </button>
                          ) : null}
                          {manageable ? (
                            <>
                              <button type="button" role="menuitem" onClick={() => { setMenuFor(null); openKbSettings(kb); }}>
                                <KbIcon name="settings" size={14} />{t('common.settings')}
                              </button>
                              <button type="button" role="menuitem" className="kb-list-menu-danger" onClick={() => { setMenuFor(null); setDeletingKb({ id: card.id, name: String(card.name ?? '') }); }}>
                                <KbIcon name="trash" size={14} />{t('common.delete')}
                              </button>
                            </>
                          ) : null}
                        </div>
                      ) : null}
                      <p className="kb-list-card-desc relative z-[1] m-0 mb-1.5 min-h-0 flex-1 overflow-hidden [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2] [line-clamp:2] text-xs font-normal leading-[18px] text-[#646e74]">{String(card.description ?? '') || t('knowledgeBase.noDescription')}</p>
                      <div className="kb-list-card-bottom relative z-[1] mt-auto flex items-center justify-between gap-2 border-t-[0.5px] border-[#e3e7ee] pt-[6px]">
                        <div className="kb-list-badges flex min-w-0 items-center gap-1">
                          <span className={'kb-list-badge inline-flex h-[22px] items-center justify-center gap-[3px] rounded-[5px] px-1.5 text-[11px] font-medium leading-none ' + (isFaq ? 'bg-[rgba(0,82,217,0.08)] text-[#07c05f]' : 'bg-[rgba(7,192,95,0.08)] text-[#06b04d]')}>
                            <KbIcon name={isFaq ? 'chat' : 'folder'} size={14} />
                            {/* Vue KnowledgeBaseList.vue:336 shared card badge: `kb.knowledge_count || '-'`
                                (faq: chunk_count || '-'); own cards keep the numeric `|| 0` fallback (:259). */}
                            <span className="kb-list-badge-count">{isSharedCard ? (count || '-') : count}</span>
                          </span>
                          {extractEnabled ? <span className="kb-list-badge kb-list-badge-icon-only kb-list-badge-relation inline-flex h-[22px] w-[22px] items-center justify-center gap-[3px] rounded-[5px] p-0 text-[11px] font-medium leading-none bg-[rgba(124,77,255,0.08)] text-[#07c05f]" title={t('knowledgeList.features.knowledgeGraph')}><KbIcon name="relation" size={14} /></span> : null}
                          {vlmEnabled ? <span className="kb-list-badge kb-list-badge-icon-only kb-list-badge-multimodal inline-flex h-[22px] w-[22px] items-center justify-center gap-[3px] rounded-[5px] p-0 text-[11px] font-medium leading-none bg-[rgba(255,152,0,0.08)] text-[#ed7b2f]" title={t('knowledgeList.features.multimodal')}><KbIcon name="image" size={14} /></span> : null}
                          {questionEnabled ? <span className="kb-list-badge kb-list-badge-icon-only kb-list-badge-question inline-flex h-[22px] w-[22px] items-center justify-center gap-[3px] rounded-[5px] p-0 text-[11px] font-medium leading-none bg-[rgba(0,150,136,0.08)] text-[#00a870]" title={t('knowledgeList.features.questionGeneration')}><KbIcon name="help-circle" size={14} /></span> : null}
                          {shareCount > 0 ? <span className="kb-list-badge kb-list-badge-icon-only kb-list-badge-shared inline-flex h-[22px] w-[22px] items-center justify-center gap-[3px] rounded-[5px] p-0 text-[11px] font-medium leading-none bg-[rgba(0,82,217,0.08)] text-[#07c05f]" title={t('knowledgeList.sharedToOrgs', { count: shareCount })}><KbIcon name="share" size={14} /></span> : null}
                        </div>
                        {isSharedCard && typeof card.org_name === 'string' && card.org_name ? (
                          <span className="kb-list-org-chip inline-flex max-w-[140px] shrink-0 items-center gap-[5px] overflow-hidden whitespace-nowrap text-ellipsis rounded-md bg-[rgba(7,192,95,0.06)] px-2 py-[3px] text-xs font-medium text-[#646e74] [&_svg]:shrink-0 [&_svg]:text-[#07c05f]" title={card.org_name}><KbIcon name="workspace" size={14} />{card.org_name}</span>
                        ) : null}
                      </div>
                    </article>
                  );
                })}
              </div>
            ) : null}
          </div>
        </div>
      </div>

      <Dialog className="wk-kb-editor-dialog h-[min(85vh,750px)] w-[min(1000px,90vw)]! max-h-[min(750px,85vh)]!" open={dialogOpen} title={editingId ? t('knowledgeEditor.titleEdit') : t('knowledgeList.create')} closeLabel={editingId ? t('common.cancel') : undefined} onClose={() => setDialogOpen(false)}>
        {/* Vue KnowledgeBaseEditorModal.vue has no native form element: the
            settings body is a plain div and the footer buttons are @click
            handlers (:447-455). Keeping this wrapper a div (not a form)
            removes the form-in-form hydration error the share section used
            to trigger and drops the implicit Enter-submit contract Vue
            never had. */}
        <div className="wk-form mb-4 grid gap-4">
          <div className="grid min-h-[360px] grid-cols-[minmax(132px,0.34fr)_minmax(0,1fr)] gap-5 max-[680px]:grid-cols-1">
            <nav aria-label={t('common.settings')} data-guide="kb-editor-sidebar" className="flex flex-col gap-2 border-r border-line-soft pr-3 max-[680px]:border-r-0 max-[680px]:border-b max-[680px]:pb-3">
              {visibleKnowledgeEditorSections({ type, editing: Boolean(editingId) }).map((group) => <div key={group.key} className="grid gap-1">
                <p className="m-0 px-3 pt-1 text-[11px] font-semibold uppercase tracking-[0.06em] text-muted">{t(group.labelKey)}</p>
                {group.items.map((section) => <button key={section} type="button" data-guide={`kb-editor-nav-${section}`} onClick={() => { setEditorSection(section); if (section === 'activity' && editingId) void loadEditorActivity(editingId); }} className={`border-0 bg-transparent rounded-[6px] px-3 py-2 text-left text-[13px] font-medium transition-colors ${editorSection === section ? 'bg-[color-mix(in_srgb,var(--color-brand)_10%,transparent)] text-[var(--color-brand)]' : 'text-muted hover:bg-surface-muted'}`}>{t(section === 'basic' ? 'knowledgeEditor.basic.title' : section === 'models' ? 'knowledgeEditor.models.title' : section === 'vectorStore' ? 'knowledgeEditor.sidebar.vectorStore' : section === 'faq' ? 'knowledgeEditor.faq.title' : section === 'parser' ? 'kbSettings.parser.title' : section === 'chunking' ? 'knowledgeEditor.chunking.title' : section === 'multimodal' ? 'knowledgeEditor.sidebar.multimodal' : section === 'asr' ? 'knowledgeEditor.sidebar.asr' : section === 'graph' ? 'knowledgeEditor.sidebar.graph' : section === 'advanced' ? 'knowledgeEditor.advanced.title' : section === 'storage' ? 'knowledgeEditor.sidebar.storage' : section === 'datasource' ? 'knowledgeEditor.sidebar.datasource' : section === 'share' ? 'knowledgeEditor.sidebar.share' : 'knowledgeEditor.activity.title')}</button>)}
              </div>)}
            </nav>
            <div className="min-w-0">
              {editorSection === 'basic' ? <div className="grid gap-4">
                <div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.basic.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.basic.description')}</p></div>
                {editingId ? <div className="grid gap-1"><span className="text-[13px] font-medium text-ink">{t('knowledgeEditor.basic.kbId')}</span><span className="text-xs leading-[18px] text-muted">{t('knowledgeEditor.basic.kbIdDesc')}</span><code className="rounded-[6px] bg-surface-muted px-2 py-1 font-mono text-xs text-muted">{editingId}</code></div> : null}
                <div role="radiogroup" aria-label={t('knowledgeEditor.basic.typeLabel')} data-guide="kb-create-type" className="grid gap-1">
                  <span className="text-[15px] font-medium text-ink">{t('knowledgeEditor.basic.typeLabel')}<span className="ml-1 text-[#e34d59]">*</span></span>
                  {/* Vue KnowledgeBaseEditorModal.vue:67-74 t-radio-group outline +
                      theme.css checked fill. The frame is carried by the joined
                      buttons themselves (TDesign .t-radio-button: 1px #e7e7e7
                      border, 3px end radius, 30px height, 16px x-padding, no
                      tab gap); the checked button's right border stays brand
                      green so the shared divider reads green like Vue. */}
                  <div className="kb-create-type-frame inline-flex w-fit rounded-[3px]" onKeyDown={(event) => {
                    const forward = event.key === 'ArrowRight' || event.key === 'ArrowDown';
                    const backward = event.key === 'ArrowLeft' || event.key === 'ArrowUp';
                    if (!forward && !backward) return;
                    event.preventDefault();
                    const order = ['document', 'faq'] as const;
                    const next = order[(order.indexOf(type) + (forward ? 1 : order.length - 1)) % order.length];
                    setType(next);
                    setEditorSection(next === 'faq' ? 'faq' : 'basic');
                    (event.currentTarget.querySelector(`button[data-value="${next}"]`) as HTMLButtonElement | null)?.focus();
                  }}>
                    {(['document', 'faq'] as const).map((item, index, items) => <button key={item} type="button" role="radio" data-value={item} aria-checked={type === item} tabIndex={type === item ? 0 : -1} disabled={Boolean(editingId)} className={`h-[30px] cursor-pointer border-y px-4 py-0 text-sm [font:inherit] transition-colors disabled:cursor-not-allowed disabled:opacity-60 ${index === 0 ? 'rounded-l-[3px]' : ''} ${index === items.length - 1 ? 'rounded-r-[3px] border-r' : ''} ${index > 0 && type === items[index - 1] ? '' : 'border-l'} ${type === item ? 'z-[1] border-[var(--color-brand)] bg-accent text-white' : 'border-line-neutral bg-surface text-[rgba(0,0,0,0.9)] hover:border-accent hover:text-accent focus-visible:text-accent focus-visible:outline-none'}`} onClick={() => { setType(item); setEditorSection(item === 'faq' ? 'faq' : 'basic'); }}>{item === 'document' ? t('common.typeDocument') : t('common.typeFaq')}</button>)}
                  </div>
                  <span className="text-xs leading-[18px] text-muted">{t('knowledgeEditor.basic.typeDescription')}</span>
                </div>
                {type === 'document' ? <fieldset data-guide="kb-create-indexing" className="m-0 grid min-w-0 gap-2 border-0 p-0"><legend className="p-0 text-[15px] font-medium text-ink">{t('knowledgeEditor.indexing.title')}<span className="ml-1 text-[#e34d59]">*</span></legend><p className="m-0 text-xs leading-[18px] text-muted">{t('knowledgeEditor.indexing.description')}</p><div className={`grid gap-3 min-[720px]:grid-cols-2${Boolean(editingId) && editingHasFiles ? ' opacity-60' : ''}`}>
                  {/* R490 A4 (Vue isIndexingLocked, KnowledgeBaseEditorModal.vue:1011
                      + :114-116): an edit-mode KB with files disables both checks
                      (toggleVectorIndexing/toggleWikiIndexing early-return) and the
                      lockedTip renders below the card pair. */}
                  <label className={`grid gap-1 rounded-[8px] border p-3 transition-colors ${indexingStrategy.vector_enabled ? 'border-[var(--color-brand)] bg-[color-mix(in_srgb,var(--color-brand)_8%,transparent)]' : 'border-line-soft bg-surface'}`}><span className="flex items-start gap-2"><Checkbox checked={indexingStrategy.vector_enabled} disabled={Boolean(editingId) && editingHasFiles} onChange={(event) => { if (Boolean(editingId) && editingHasFiles) return; setIndexingStrategy((current) => ({ ...current, vector_enabled: event.target.checked, keyword_enabled: event.target.checked })); }} /><strong className="text-[13px] font-medium">{t('knowledgeEditor.indexing.searchTitle')}</strong></span><small className="pl-6 text-xs leading-[18px] text-muted">{t('knowledgeEditor.indexing.searchDesc')}</small></label>
                  <label className={`grid gap-1 rounded-[8px] border p-3 transition-colors ${indexingStrategy.wiki_enabled ? 'border-[var(--color-brand)] bg-[color-mix(in_srgb,var(--color-brand)_8%,transparent)]' : 'border-line-soft bg-surface'}`}><span className="flex items-start gap-2"><Checkbox checked={indexingStrategy.wiki_enabled} disabled={Boolean(editingId) && editingHasFiles} onChange={(event) => { if (Boolean(editingId) && editingHasFiles) return; setIndexingStrategy((current) => ({ ...current, wiki_enabled: event.target.checked })); }} />{/* Vue KnowledgeBaseEditorModal.vue:106-109 indexing-check-title + :1972-1984 indexing-new-badge */}
                    <strong className="inline-flex items-center gap-[6px] text-[13px] font-medium">{t('knowledgeEditor.indexing.wikiTitle')}<span className="kb-editor-new-badge inline-flex h-4 items-center rounded-[3px] bg-[var(--color-brand-light)] px-[6px] text-[10px] font-semibold leading-none tracking-[0.4px] text-accent">NEW</span></strong></span><small className="pl-6 text-xs leading-[18px] text-muted">{t('knowledgeEditor.indexing.wikiDesc')}</small></label>
                </div>{Boolean(editingId) && editingHasFiles ? <p className="m-0 text-xs leading-[18px] text-muted" data-indexing-locked-tip="">{t('knowledgeEditor.indexing.lockedTip')}</p> : null}</fieldset> : null}
                <label className="grid gap-1"><span>{t('knowledgeEditor.basic.nameLabel')}<span className="ml-1 text-[#e34d59]">*</span></span> {/* Vue t-input :165-169 is maxlength-only; blank names are blocked by the JS save pipeline */}<Input data-guide="kb-create-name" value={name} onChange={(event) => setName(event.target.value)} placeholder={t('knowledgeEditor.basic.namePlaceholder')} maxLength={50} className="rounded-control border border-line-strong p-[0.55rem]" /></label>
                <label className="grid gap-1">{t('knowledgeEditor.basic.descriptionLabel')} <Textarea value={description} maxLength={200} onChange={(event) => setDescription(event.target.value)} placeholder={t('knowledgeEditor.basic.descriptionPlaceholder')} rows={3} />{/* Vue KnowledgeBaseEditorModal.vue:171-179 t-textarea maxlength=200 → .t-textarea__limit counter (12px/20px, placeholder gray, right-aligned) */}
                  <span className="kb-editor-desc-count justify-self-end text-xs leading-5 text-[var(--color-text-placeholder)]" aria-live="polite">{description.length}/200</span></label>
              </div> : null}
              {/* R488 summary_model_id: the models section mirrors Vue KBModelConfig.vue —
                  LLM row first (always required), then the Embedding row (required while
                  RAG indexing runs). Both are ModelSelector dropdowns fed by GET /models
                  with the add-model entry, NOT free-text inputs whose 可选 placeholder
                  contradicted the submit-time summary_model_id requirement (K2). */}
              {editorSection === 'models' ? <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.models.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.models.description')}</p></div><div className="grid gap-1" data-guide="kb-create-llm"><label className="text-[15px] font-medium text-ink">{t('knowledgeEditor.models.llmLabel')}<span className="ml-1 text-[#e34d59]">*</span></label><p className="m-0 text-[13px] leading-[1.5] text-muted">{t('knowledgeEditor.models.llmDesc')}</p><ModelOptionSelect value={summaryModelId} options={chatModelOptions} addModelLabel={t('model.addModelInSettings')} onAddModel={() => navigate('/platform/settings?section=models&subsection=chat')} onChange={setSummaryModelId} /></div><div className="grid gap-1" data-guide="kb-create-embedding"><label className="text-[15px] font-medium text-ink">{t('knowledgeEditor.models.embeddingLabel')}{modelsEmbeddingRequired ? <span className="ml-1 text-[#e34d59]">*</span> : null}</label><p className="m-0 text-[13px] leading-[1.5] text-muted">{t('knowledgeEditor.models.embeddingDesc')}</p>{/* R490 A3② (Vue KBModelConfig.vue:41-46): a RAG-enabled KB that already has files locks the Embedding selector with the embeddingLocked warning — the model cannot be swapped once vectors exist. */}{modelsEmbeddingRequired && editingHasFiles ? <p className="m-0 text-[13px] leading-[1.5] text-muted" data-embedding-locked-tip="" style={{ margin: '0.5rem 0 0', fontSize: '0.85rem' }}>{t('knowledgeEditor.models.embeddingLocked')}</p> : null}<ModelOptionSelect value={embeddingModelId} options={embeddingModelOptions} disabled={modelsEmbeddingRequired && editingHasFiles} addModelLabel={t('model.addModelInSettings')} onAddModel={() => navigate('/platform/settings?section=models&subsection=embedding')} onChange={setEmbeddingModelId} /></div></div> : null}
              {editorSection === 'faq' && type === 'faq' ? <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.faq.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.faq.description')}</p></div><label className="grid gap-1">{t('knowledgeEditor.faq.indexModeLabel')}<Select value={editorConfig.faqConfig.indexMode} onChange={(event) => setEditorConfig((current) => ({ ...current, faqConfig: { ...current.faqConfig, indexMode: event.target.value as KnowledgeEditorConfig['faqConfig']['indexMode'] } }))}><option value="question_only">{t('knowledgeEditor.faq.modes.questionOnly')}</option><option value="question_answer">{t('knowledgeEditor.faq.modes.questionAnswer')}</option></Select></label><label className="grid gap-1">{t('knowledgeEditor.faq.questionIndexModeLabel')}<Select value={editorConfig.faqConfig.questionIndexMode} onChange={(event) => setEditorConfig((current) => ({ ...current, faqConfig: { ...current.faqConfig, questionIndexMode: event.target.value as KnowledgeEditorConfig['faqConfig']['questionIndexMode'] } }))}><option value="combined">{t('knowledgeEditor.faq.modes.combined')}</option><option value="separate">{t('knowledgeEditor.faq.modes.separate')}</option></Select></label><p className="m-0 text-xs leading-[18px] text-muted">{t('knowledgeEditor.faq.entryGuide')}</p></div> : null}
              {/* R490 A1 (Vue KBChunkingSettings.vue + KBChunkingDebug.vue): the
                  drawer mounts the shared Vue-form chunking section — strategy
                  select + 测试分块效果 trigger, size/overlap sliders with marks and
                  live value display, separator chips, parent-child switch and
                  sliders, collapsed 高级选项 (token limit + languages). The bare
                  number-input grid is gone; the table-metadata textarea moved to
                  the advanced section where Vue renders it (KBAdvancedSettings). */}
              {editorSection === 'chunking' && type === 'document' ? <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.chunking.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.chunking.description')}</p></div><ChunkingSettingsFields splitting={editorConfig.chunkingConfig} onPatch={(patch) => setEditorConfig((current) => ({ ...current, chunkingConfig: { ...current.chunkingConfig, ...patch } }))} client={client} t={t} /></div> : null}
              {editorSection === 'multimodal' && type === 'document' ? <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.multimodal.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.multimodal.description')}</p></div><label className="flex items-center gap-2 text-sm"><Checkbox checked={editorConfig.multimodalConfig.enabled} onChange={(event) => setEditorConfig((current) => ({ ...current, multimodalConfig: { ...current.multimodalConfig, enabled: event.target.checked } }))} />{t('knowledgeEditor.advanced.multimodal.label')}</label>{editorConfig.multimodalConfig.enabled ? <><label className="grid gap-1">{t('knowledgeEditor.advanced.multimodal.vllmLabel')}<Input value={editorConfig.multimodalConfig.vllmModelId} onChange={(event) => setEditorConfig((current) => ({ ...current, multimodalConfig: { ...current.multimodalConfig, vllmModelId: event.target.value } }))} placeholder={t('knowledgeEditor.advanced.multimodal.vllmPlaceholder')} /></label><label className="grid gap-1">{t('knowledgeEditor.advanced.multimodal.customInstructionsLabel')}<Textarea rows={4} maxLength={4000} value={editorConfig.multimodalConfig.customInstructions} onChange={(event) => setEditorConfig((current) => ({ ...current, multimodalConfig: { ...current.multimodalConfig, customInstructions: event.target.value } }))} placeholder={t('knowledgeEditor.advanced.multimodal.customInstructionsPlaceholder')} /></label></> : null}</div> : null}
              {editorSection === 'asr' && type === 'document' ? <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.asr.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.asr.description')}</p></div><label className="flex items-center gap-2 text-sm"><Checkbox checked={editorConfig.asrConfig.enabled} onChange={(event) => setEditorConfig((current) => ({ ...current, asrConfig: { ...current.asrConfig, enabled: event.target.checked } }))} />{t('knowledgeEditor.asr.label')}</label>{editorConfig.asrConfig.enabled ? <label className="grid gap-1">{t('knowledgeEditor.asr.modelLabel')}<Input value={editorConfig.asrConfig.modelId} onChange={(event) => setEditorConfig((current) => ({ ...current, asrConfig: { ...current.asrConfig, modelId: event.target.value } }))} placeholder={t('knowledgeEditor.asr.modelPlaceholder')} /></label> : null}</div> : null}
              {/* R490 A5 (Vue GraphSettings.vue): the drawer mounts the shared
                  graph settings — the graph-database warning alert with the
                  「如何启用知识图谱？」guide link (window.open to VITE_KG_GUIDE_URL,
                  Vue :589-596), the enable switch and the extraction form. */}
              {editorSection === 'graph' && type === 'document' ? <GraphSettings graphExtract={editorConfig.nodeExtractConfig as GraphExtractConfig} modelId={summaryModelId} client={client} canRunGraphExtract={viewer.isAdmin} onChange={(value) => setEditorConfig((current) => ({ ...current, nodeExtractConfig: value }))} onOpenGraphGuide={() => { window.open(((import.meta as { env?: Record<string, string | undefined> }).env?.VITE_KG_GUIDE_URL) || 'https://github.com/Tencent/WeKnora/blob/main/docs/KnowledgeGraph.md', '_blank', 'noopener'); }} /> : null}
              {editorSection === 'advanced' && type === 'document' ? <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.advanced.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.advanced.description')}</p></div><label className="flex items-center gap-2 text-sm"><Checkbox checked={editorConfig.questionGenerationConfig.enabled} onChange={(event) => setEditorConfig((current) => ({ ...current, questionGenerationConfig: { ...current.questionGenerationConfig, enabled: event.target.checked } }))} />{t('knowledgeEditor.advanced.questionGeneration.label')}</label>{editorConfig.questionGenerationConfig.enabled ? <label className="grid gap-1">{t('knowledgeEditor.advanced.questionGeneration.countLabel')}<Input type="number" min={1} max={10} value={editorConfig.questionGenerationConfig.questionCount} onChange={(event) => setEditorConfig((current) => ({ ...current, questionGenerationConfig: { ...current.questionGenerationConfig, questionCount: Number(event.target.value) } }))} /></label> : null}<label className="flex items-center gap-2 text-sm"><Checkbox checked={editorConfig.autoTagConfig.enabled} onChange={(event) => setEditorConfig((current) => ({ ...current, autoTagConfig: { ...current.autoTagConfig, enabled: event.target.checked } }))} />{t('knowledgeEditor.advanced.autoTag.label')}</label>{/* R490 A6 (Vue KBAdvancedSettings.vue:117-131): the table-metadata instructions textarea lives on the ADVANCED section (maxlength=4000 with the TDesign "0/4000" limit counter), not on chunking. */}<label className="grid gap-1">{t('knowledgeEditor.advanced.tableMetadataInstructions.label')}<Textarea rows={3} maxLength={4000} value={editorConfig.chunkingConfig.tableMetadataInstructions} placeholder={t('knowledgeEditor.advanced.tableMetadataInstructions.placeholder')} onChange={(event) => setEditorConfig((current) => ({ ...current, chunkingConfig: { ...current.chunkingConfig, tableMetadataInstructions: event.target.value } }))} /><span className="kb-editor-desc-count justify-self-end text-xs leading-5 text-[var(--color-text-placeholder)]" aria-live="polite" data-table-metadata-count="">{editorConfig.chunkingConfig.tableMetadataInstructions.length}/4000</span></label></div> : null}
              {editorSection === 'vectorStore' ? <div className="grid gap-4"><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.sidebar.vectorStore')}</h3><p className="m-0 text-sm leading-[22px] text-muted">{t('kbSettings.vectorStore.description')}</p><label className="grid gap-1">{t('kbSettings.vectorStore.engineLabel')}<Select value={editorConfig.vectorStoreId} disabled={Boolean(editingId) || editorOptions.loading} onChange={(event) => setEditorConfig((current) => ({ ...current, vectorStoreId: event.target.value }))}><option value="">{t('kbSettings.vectorStore.systemDefault')}</option>{editorOptions.vectorStores.map((store) => <option key={store.id} value={store.id}>{store.name} · {store.engine_type}</option>)}</Select></label><p className="m-0 text-xs leading-[18px] text-muted">{editingId ? t('kbSettings.vectorStore.immutableHint') : editorOptions.error ?? t('kbSettings.vectorStore.engineDesc')}</p></div> : null}
              {/* R490 A2 (Vue KBStorageSettings.vue): the instance list renders
                  INSIDE the select options (name + uppercased provider tag +
                  默认 tag on the tenant default), the selected instance shows the
                  endpoint/bucket/path hint, the migrate hint renders only while
                  the edit-mode KB has files (:disabled="!!hasFiles"), and the
                  管理存储实例 entry jumps to Settings → Storage (Vue
                  closeKBEditor + openSettings('storage')). */}
              {editorSection === 'storage' ? (() => { const storageLocked = Boolean(editingId) && editingHasFiles; const selectedBackend = editorOptions.storageBackends.find((candidate) => candidate.id === editorConfig.storageBackendId); const backendConfig = (selectedBackend?.config ?? {}) as Record<string, unknown>; const text = (value: unknown) => (typeof value === 'string' ? value : ''); const selectedHint = selectedBackend ? (text(backendConfig.endpoint) || text(backendConfig.bucket_name) || text(backendConfig.path_prefix) || t('kbSettings.storage.localStorage')) : ''; return <div className="grid gap-4"><div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.sidebar.storage')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('kbSettings.storage.selectDescription')}</p></div><div className="grid gap-1"><label className="text-[15px] font-medium text-ink">{t('kbSettings.storage.instanceLabel')}</label><p className="m-0 text-xs leading-[18px] text-muted">{t('kbSettings.storage.instanceDesc')}</p><Select value={editorConfig.storageBackendId} disabled={storageLocked || editorOptions.loading} aria-label={t('kbSettings.storage.instanceLabel')} onChange={(event) => { const backendId = event.target.value; const backend = editorOptions.storageBackends.find((candidate) => candidate.id === backendId); setEditorConfig((current) => ({ ...current, storageBackendId: backendId, storageProvider: backend?.provider ?? current.storageProvider })); }}>{editorOptions.storageBackends.map((backend) => <option key={backend.id} value={backend.id}>{backend.name} · {backend.provider.toUpperCase()}{backend.id === editorOptions.defaultStorageBackendId ? ` · ${t('kbSettings.storage.defaultTag')}` : ''}</option>)}{editorOptions.storageBackends.every((backend) => backend.id !== editorConfig.storageBackendId) && editorConfig.storageBackendId ? <option value={editorConfig.storageBackendId}>{editorConfig.storageBackendId}</option> : null}</Select>{storageLocked ? <p className="m-0 text-xs leading-[18px]" style={{ color: 'var(--color-warning-text, #b54708)' }} data-storage-migrate-hint="">{t('kbSettings.storage.migrateHint')}</p> : null}{!storageLocked && selectedHint ? <p className="m-0 text-xs leading-[18px] text-muted" data-storage-instance-hint="">{selectedHint}</p> : null}<a data-storage-manage-instances="" href="/platform/settings?section=storage" className="justify-self-start text-[13px]" style={{ color: 'var(--color-brand)' }} onClick={(event) => { event.preventDefault(); setDialogOpen(false); navigate('/platform/settings?section=storage'); }}>{t('kbSettings.storage.manageInstances')}</a></div></div>; })() : null}
              {editorSection === 'parser' ? <div className="grid gap-4"><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('kbSettings.parser.title')}</h3><p className="m-0 text-sm leading-[22px] text-muted">{t('kbSettings.parser.description')}</p><label className="grid gap-1">{t('kbSettings.parser.title')}<Select value={editorConfig.chunkingConfig.parserEngineRules[0]?.engine ?? ''} disabled={editorOptions.loading} onChange={(event) => setEditorConfig((current) => ({ ...current, chunkingConfig: { ...current.chunkingConfig, parserEngineRules: event.target.value ? [{ file_types: ['pdf'], engine: event.target.value }] : [] } }))}><option value="">{t('kbSettings.parser.default')}</option>{editorOptions.parserEngines.filter((engine) => engine.Available !== false).map((engine) => <option key={engine.Name} value={engine.Name}>{engine.Name}</option>)}</Select></label><p className="m-0 text-xs leading-[18px] text-muted">{editorOptions.error ?? t('kbSettings.parser.goConfig')}</p></div> : null}
              {editorSection === 'datasource' && editingId ? <div className="max-h-[34rem] overflow-auto"><DataSourcesPage client={client} knowledgeBaseId={editingId} canManage={viewer.isAdmin} /></div> : null}
              {editorSection === 'share' && editingId ? <KnowledgeBaseShareDialog client={client} knowledgeBaseId={editingId} knowledgeBaseName={name} open inline onClose={() => undefined} onChanged={() => setReloadToken((value) => value + 1)} /> : null}
              {editorSection === 'activity' && editingId ? <KnowledgeBaseActivityPanel client={client} knowledgeBaseId={editingId} /> : null}
            </div>
          </div>
          <div className="mt-3 flex justify-end gap-2">
            <Button type="button" onClick={() => setDialogOpen(false)}>{t('common.cancel')}</Button>
            {/* Vue :451-453 `<t-button @click="handleSubmit" :loading="saving" :disabled="loading">` —
                editorOptions.loading is the React counterpart of the Vue dialog-open `loading`
                (watch(visible) arms it, loadEditorOptions settles it), so save is disabled while
                the editor data is not ready, with saving's spinner/disable layered on top. */}
            <Button type="button" data-guide="kb-create-submit" loading={saving} disabled={editorOptions.loading} onClick={() => { void save(); }}>{editingId ? t('knowledgeEditor.buttons.saveAndClose') : t('knowledgeEditor.buttons.create')}</Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={deletingKb !== null}
        title={t('knowledgeList.delete.confirmTitle')}
        onClose={() => { if (!deleting) setDeletingKb(null); }}
        closeLabel={t('common.cancel')}
      >
        <p>{t('knowledgeList.delete.confirmMessage', { name: deletingKb?.name ?? '' })}</p>
        <div className="mt-3 flex justify-end gap-2">
          <Button type="button" className="text-[#dc2626]!" loading={deleting} onClick={() => void confirmDelete()}>{t('knowledgeList.delete.confirmButton')}</Button>
          <Button type="button" onClick={() => setDeletingKb(null)}>{t('common.cancel')}</Button>
        </div>
      </Dialog>

      <KnowledgeBaseShareDialog client={client} knowledgeBaseId={sharingKb?.id ?? ''} knowledgeBaseName={sharingKb?.name ?? ''} open={sharingKb !== null} onClose={() => setSharingKb(null)} onChanged={() => setReloadToken((value) => value + 1)} />

      {/* Vue right-side shared KB detail drawer (KnowledgeBaseList.vue:709-776) */}
      <SharedKnowledgeBaseDrawer
        open={sharedDetail !== null}
        shared={sharedDetail}
        onClose={() => setSharedDetail(null)}
        onGoToKb={(kbId) => { navigate(knowledgeBaseDetailPath(kbId)); setSharedDetail(null); }}
      />
    </main>
  );
}

export { BillingPage } from './commercial/BillingPage.tsx';
export { CheckoutPage } from './commercial/CheckoutPage.tsx';
export { RefundPage } from './commercial/RefundPage.tsx';
export { AdminCommercialPage } from './commercial/AdminCommercialPage.tsx';
export { AppsPage } from './appconnector/AppsPage.tsx';
export { ConnectionsPage } from './appconnector/ConnectionsPage.tsx';
export { AuthorizationPage } from './appconnector/AuthorizationPage.tsx';
export { ActionApproval } from './appconnector/ActionApproval.tsx';
export { TaskDetailCommercePanel } from './appconnector/TaskDetailCommercePanel.tsx';
export { TaskBudget } from './commercial/TaskBudget.tsx';
