import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController, scopedKey, filterKnowledgeBases } from '@weknora/domain';
import {
  canDuplicateKBCard,
  canManageKBCard,
  isKnowledgeBaseInitialized,
  isSharedKbEditable,
  groupKnowledgeBaseSections,
  mergeAllScopeKnowledgeBases,
  type KnowledgeBaseCreatorFilter,
  type MergedKnowledgeBase,
} from '@weknora/domain';
import { formatMessage, isLocale, type Locale, type MessageValues } from '@weknora/i18n';
import { Button, Card, Dialog, Status } from '@weknora/ui';
import {
  isContextualGuideDone,
  markContextualGuideDone,
  openContextualGuide,
} from '@weknora/views';
import {
  createDeleteGuard,
  loadKnowledgeBaseListPage,
  saveKnowledgeBase,
  type KnowledgeBaseListPageState,
  type KnowledgeBaseSaveInput,
} from './knowledge-bases/list.ts';
import { KnowledgeBaseShareDialog } from './knowledge-bases/KnowledgeBaseShareDialog.tsx';
import { findUploadTargetPage, patchUploadTask, summarizeUploadTasks, upsertUploadTask, type UploadTaskState } from './knowledge-bases/upload-progress.ts';

interface KnowledgeBasesPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
}

interface Viewer {
  userId: string;
  isAdmin: boolean;
  isContributor: boolean;
}

const SKELETON_CARD_COUNT = 6;

function resolveLocale(): Locale {
  const language = typeof navigator !== 'undefined' ? navigator.language : 'en-US';
  return isLocale(language) ? language : isLocale(language.split('-')[0] ?? '') ? (language.split('-')[0] as Locale) : 'en-US';
}

function readScopeFromUrl(): 'all' | 'mine' {
  const value = new URLSearchParams(window.location.search).get('scope');
  return value === 'mine' ? 'mine' : 'all';
}

function writeScopeToUrl(space: 'all' | 'mine' | 'favorites' | 'recents'): void {
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

export function KnowledgeBasesPage({ client, scopeController }: KnowledgeBasesPageProps) {
  const locale = useMemo(resolveLocale, []);
  const t = useCallback((key: string, values: MessageValues = {}) => formatMessage(locale, key, values), [locale]);

  const [reloadToken, setReloadToken] = useState(0);
  const [pageState, setPageState] = useState<KnowledgeBaseListPageState>({ status: 'loading' });
  const [viewer, setViewer] = useState<Viewer>({ userId: '', isAdmin: false, isContributor: false });
  const [space, setSpaceState] = useState<'all' | 'mine' | 'favorites' | 'recents'>(readScopeFromUrl);
  const [query, setQuery] = useState(() => new URLSearchParams(window.location.search).get('q') ?? '');
  const [creator, setCreator] = useState<KnowledgeBaseCreatorFilter>('all');
  const [page, setPage] = useState(1);
  const [favorites, setFavorites] = useState<Set<string>>(readFavorites);
  const [recents, setRecents] = useState<Set<string>>(() => {
    try { return new Set(JSON.parse(window.localStorage.getItem('wk-kb-recents') ?? '[]') as string[]); } catch { return new Set(); }
  });
  const [collapsedSections, setCollapsedSections] = useState<ReadonlySet<string>>(new Set());
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
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [type, setType] = useState<'document' | 'faq'>('document');
  const [embeddingModelId, setEmbeddingModelId] = useState('');
  const [summaryModelId, setSummaryModelId] = useState('');
  const [sharingKb, setSharingKb] = useState<{ id: string; name: string } | null>(null);
  const [saving, setSaving] = useState(false);

  // Delete confirmation dialog state; DELETE fires only after confirm.
  const [deletingKb, setDeletingKb] = useState<{ id: string; name: string } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const deleteGuard = useRef(createDeleteGuard(client));

  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'knowledge-bases'), [scope.scope]);

  const setSpace = (next: 'all' | 'mine' | 'favorites' | 'recents') => {
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

  useEffect(() => {
    let active = true;
    setPageState({ status: 'loading' });
    void loadKnowledgeBaseListPage(client, scope.signal, { creator }).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setPageState(next);
    }).catch(() => { /* aborted */ });
    return () => { active = false; };
  }, [client, creator, reloadToken, scopeController, scope.scope, scope.signal]);

  const cards = useMemo<MergedKnowledgeBase[]>(() => {
    if (pageState.status !== 'success') return [];
    if (space === 'mine') return pageState.owned.map((kb) => ({ ...kb, isMine: true as const }));
    return mergeAllScopeKnowledgeBases(pageState.owned, pageState.shared, viewer.userId || undefined);
  }, [pageState, space, viewer.userId]);

  const scopedCards = useMemo(() => {
    if (space === 'favorites') return cards.filter((c) => favorites.has(c.id));
    if (space === 'recents') return cards.filter((c) => recents.has(c.id));
    return cards;
  }, [cards, space, favorites, recents]);

  const filtered = useMemo(() => filterKnowledgeBases(scopedCards, {
    query,
    creator: space === 'mine' ? creator : 'all',
    currentUserId: viewer.userId || undefined,
    page,
    pageSize: 12,
  }), [scopedCards, creator, page, query, space, favorites, recents, viewer.userId]);

  // Vue KnowledgeBaseList.vue:97-185 — collapsible sections in the all-scope view.
  const sections = useMemo(() => {
    if (space !== 'all' || pageState.status !== 'success') return [];
    return groupKnowledgeBaseSections(cards, viewer.userId || undefined);
  }, [space, pageState, cards, viewer.userId]);

  useEffect(() => { setPage(1); }, [creator, query, space]);

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
    const match = cards.find((card) => String(card.id) === kbId);
    return match ? String(match.name ?? '') : t('knowledgeList.uploadProgress.unknownKb', { id: kbId });
  }), [cards, t, uploadTasks]);

  useEffect(() => {
    if (!highlightId || pageState.status !== 'success') return;
    const highlightCards = filterKnowledgeBases(scopedCards, {
      query,
      creator: space === 'mine' ? creator : 'all',
      currentUserId: viewer.userId || undefined,
      page: 1,
      pageSize: Math.max(scopedCards.length, 1),
    }).items;
    const targetPage = findUploadTargetPage(highlightCards, highlightId);
    if (targetPage !== null && targetPage !== page) {
      setPage(targetPage);
      return;
    }
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
  }, [creator, highlightId, page, pageState.status, query, scopedCards, space, viewer.userId]);

  const hasUninitialized = useMemo(
    () => cards.some((kb) => !isKnowledgeBaseInitialized(kb as never)),
    [cards],
  );

  // Vue KnowledgeBaseList.vue:1195-1197 — the kbList tour's trigger condition
  // (contributor-visible empty list, editor closed). The shell-level guide
  // host applies the dismissal + welcome-tour gates.
  const kbListGuideWhen = pageState.status === 'success'
    && viewer.isContributor
    && !dialogOpen
    && (space === 'all' || space === 'mine')
    && cards.length === 0;
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

  function openCreate() {
    setEditingId(null);
    setName('');
    setDescription('');
    setType('document');
    setEmbeddingModelId('');
    setSummaryModelId('');
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
    setDialogOpen(true);
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return; // double-submit guard
    if (!name.trim()) return; // blank names are blocked
    setSaving(true);
    setError(null);
    const input: KnowledgeBaseSaveInput = {
      name: name.trim(),
      type,
      ...(description.trim() ? { description: description.trim() } : {}),
      ...(embeddingModelId ? { embedding_model_id: embeddingModelId } : {}),
      ...(summaryModelId ? { summary_model_id: summaryModelId } : {}),
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
        window.location.assign(`/knowledgeBase/${encodeURIComponent(String(record.id))}`);
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

  // Vue handleSettingsById → goSettings unconditionally
  // (KnowledgeBaseList.vue:1394-1396): the settings action opens this KB's
  // settings without a tenant-model gate (Audit #4 superseded by source).
  function openKbSettings(kb: Record<string, unknown>) {
    const id = String(kb.id);
    window.location.assign(`/knowledgeBase/${encodeURIComponent(id)}/settings`);
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
      window.location.assign(`/knowledgeBase/${encodeURIComponent(id)}`);
      return;
    }
    // Vue handleCardClick else-branch: uninitialized card click opens this
    // KB's settings (goSettings) without a tenant-model detour.
    window.location.assign(`/knowledgeBase/${encodeURIComponent(id)}/settings`);
  }

  const isLoading = pageState.status === 'loading';

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <h1>{t('common.knowledgeBases')}</h1>
          <p className="wk-muted">{t('knowledgeList.subtitle')}</p>
        </div>
        {viewer.isContributor ? <Button type="button" data-guide="kb-list-create" onClick={openCreate}>+ {t('knowledgeList.create')}</Button> : null}
      </header>
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {hasUninitialized ? <Status tone="warning">{t('knowledgeList.uninitializedBanner')}</Status> : null}
      {uploadSummaries.length ? <div className="wk-upload-progress-panel" aria-live="polite">
        {uploadSummaries.map((summary) => <div className="wk-upload-progress-item" key={summary.kbId}>
          <div className="wk-upload-progress-icon" aria-hidden="true">{summary.completed === summary.total ? '✓' : '↑'}</div>
          <div className="wk-upload-progress-content">
            <div className="wk-upload-progress-title">{summary.completed === summary.total ? t('knowledgeList.uploadProgress.completedTitle', { name: summary.kbName }) : t('knowledgeList.uploadProgress.uploadingTitle', { name: summary.kbName })}</div>
            <div className="wk-upload-progress-subtitle">{summary.completed === summary.total ? t('knowledgeList.uploadProgress.completedDetail', { total: summary.total }) : t('knowledgeList.uploadProgress.detail', { completed: summary.completed, total: summary.total })}</div>
            <div className="wk-upload-progress-subtitle">{summary.completed === summary.total ? t('knowledgeList.uploadProgress.refreshing') : t('knowledgeList.uploadProgress.keepPageOpen')}</div>
            {summary.hasError ? <div className="wk-upload-progress-subtitle wk-upload-progress-error">{t('knowledgeList.uploadProgress.errorTip')}</div> : null}
            <div className="wk-upload-progress-track" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={summary.progress}><div className="wk-upload-progress-fill" style={{ width: `${summary.progress}%` }} /></div>
          </div>
        </div>)}
      </div> : null}
      <Card>
        <div className="wk-toolbar" role="search">
          <input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t('knowledgeList.subtitle')}
            aria-label={t('common.search')}
          />
          <select value={creator} onChange={(event) => setCreator(event.target.value as KnowledgeBaseCreatorFilter)} aria-label={t('common.creator')}>
            <option value="all">{t('common.all')}</option>
            <option value="mine">{t('common.mine')}</option>
            <option value="others">{t('common.others')}</option>
          </select>
          <span className="wk-debug">{t('common.itemCount', { count: filtered.total })}</span>
        </div>
        <div className="wk-kb-scope" role="tablist" aria-label={t('common.knowledgeBases')}>
          <button type="button" role="tab" aria-selected={space === 'all'} className={space === 'all' ? 'wk-kb-scope-tab wk-kb-scope-tab-active' : 'wk-kb-scope-tab'} onClick={() => setSpace('all')}>{t('common.all')}</button>
          <button type="button" role="tab" aria-selected={space === 'mine'} className={space === 'mine' ? 'wk-kb-scope-tab wk-kb-scope-tab-active' : 'wk-kb-scope-tab'} onClick={() => setSpace('mine')}>{t('knowledgeList.sections.mine')}</button>
          <button type="button" role="tab" aria-selected={space === 'favorites'} className={space === 'favorites' ? 'wk-kb-scope-tab wk-kb-scope-tab-active' : 'wk-kb-scope-tab'} onClick={() => setSpace('favorites')}>{t('common.favorite')}</button>
          <button type="button" role="tab" aria-selected={space === 'recents'} className={space === 'recents' ? 'wk-kb-scope-tab wk-kb-scope-tab-active' : 'wk-kb-scope-tab'} onClick={() => setSpace('recents')}>{t('knowledgeList.empty.recentsTitle')}</button>
        </div>
        {isLoading ? (
          <div className="wk-kb-grid" aria-busy="true" aria-label={t('common.loading')}>
            {Array.from({ length: SKELETON_CARD_COUNT }, (_, index) => <div className="wk-kb-skeleton" key={index} />)}
          </div>
        ) : null}
        {pageState.status === 'error' ? (
          <>
            <Status tone="error">{pageState.message}</Status>
            <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>{t('common.retry')}</Button>
          </>
        ) : null}
        {pageState.status === 'success' && filtered.total === 0 ? (
          <div className="wk-kb-empty">
            <h2>{t('knowledgeList.empty.title')}</h2>
            <p>{space === 'mine' ? t('knowledgeList.empty.description') : t('knowledgeList.empty.sharedDescription')}</p>
            {viewer.isContributor ? <Button type="button" className="empty-state-btn" data-guide="kb-list-create" onClick={openCreate}>+ {t('knowledgeList.create')}</Button> : null}
          </div>
        ) : null}
        {pageState.status === 'success' && filtered.total > 0 ? (
          <>
            {space === 'all' && sections.length > 0 ? (
              <div className="wk-kb-sections" role="list">
                {sections.map((section) => {
                  const collapsed = collapsedSections.has(section.key);
                  return (
                    <button
                      key={section.key}
                      type="button"
                      className="wk-kb-section-toggle"
                      aria-expanded={!collapsed}
                      onClick={() => setCollapsedSections((current) => {
                        const next = new Set(current);
                        if (next.has(section.key)) next.delete(section.key); else next.add(section.key);
                        return next;
                      })}
                    >
                      {t(section.labelKey)} · {section.items.length}
                      <span aria-hidden="true">{collapsed ? '+' : '−'}</span>
                    </button>
                  );
                })}
              </div>
            ) : null}
            <div className="wk-kb-grid">
              {(space === 'all' && sections.length > 0
                ? sections.flatMap((section) => (collapsedSections.has(section.key) ? [] : section.items))
                : filtered.items
              ).slice((page - 1) * 12, page * 12).map((card) => {
                const kb = card as Record<string, unknown>;
                const initialized = isKnowledgeBaseInitialized(card as never);
                const manageable = canManageKBCard(card as Record<string, unknown>, { userId: viewer.userId, isAdmin: viewer.isAdmin });
                const duplicable = canDuplicateKBCard(card as Record<string, unknown>, { userId: viewer.userId, isContributor: viewer.isContributor });
                const isWiki = (card as { indexing_strategy?: { wiki_enabled?: boolean } }).indexing_strategy?.wiki_enabled === true;
                const isSharedCard = card.isMine === false;
                const sharedEditable = card.isMine === false && isSharedKbEditable(card.permission);
                const count = typeof card.knowledge_count === 'number' ? card.knowledge_count : 0;
                return (
                  <article key={card.id} data-kb-id={card.id} className={`${initialized ? 'wk-kb-card' : 'wk-kb-card wk-kb-card-warning'}${highlightId === card.id ? ' wk-kb-flash' : ''}`}>
                    <div className="wk-kb-card-head">
                      <button type="button" className="wk-kb-card-title" onClick={() => openCard(kb)}>{String(card.name ?? '')}</button>
                      <button
                        type="button"
                        className={favorites.has(card.id) ? 'wk-kb-star wk-kb-star-active' : 'wk-kb-star'} data-highlight={highlightId === card.id || undefined}
                        aria-label={t('common.favorite')}
                        onClick={() => toggleFavorite(card.id)}
                      >{favorites.has(card.id) ? '★' : '☆'}</button>
                    </div>
                    <p className="wk-kb-card-desc">{String(card.description ?? '')}</p>
                    <div className="wk-kb-badges">
                      <span className="wk-kb-badge">{card.type === 'faq' ? t('common.typeFaq') : t('common.typeDocument')} · {count}</span>
                      {isWiki ? <span className="wk-kb-badge wk-kb-badge-wiki">{t('knowledgeList.features.wiki')}</span> : null}
                      {isSharedCard ? (
                        <span className="wk-kb-badge">{sharedEditable ? t('knowledgeList.sections.sharedEditable') : t('knowledgeList.sections.sharedReadonly')}</span>
                      ) : null}
                      {typeof card.share_count === 'number' && card.share_count > 0 ? (
                        <span className="wk-kb-badge">{t('knowledgeList.sharedToOrgs', { count: card.share_count })}</span>
                      ) : null}
                      {!initialized ? <span className="wk-kb-badge wk-kb-badge-warning">⚠</span> : null}
                    </div>
                    <div className="wk-kb-card-actions">
                      <Button type="button" onClick={() => void togglePin(kb)}>{card.is_pinned ? t('knowledgeList.pin.unpin') : t('knowledgeList.pin.pin')}</Button>
                      {duplicable ? <Button type="button" onClick={() => void duplicate(kb)}>{t('knowledgeList.menu.duplicate')}</Button> : null}
                      {manageable ? (
                        <>
                          <Button type="button" onClick={() => openEdit(kb)}>{t('common.edit')}</Button>
                          <Button type="button" onClick={() => openKbSettings(kb)}>{t('common.settings')}</Button>
                          <Button type="button" onClick={() => setSharingKb({ id: card.id, name: String(card.name ?? '') })}>{t('common.share')}</Button>
                          <Button type="button" className="wk-kb-danger" onClick={() => setDeletingKb({ id: card.id, name: String(card.name ?? '') })}>{t('common.delete')}</Button>
                        </>
                      ) : null}
                    </div>
                  </article>
                );
              })}
            </div>
            {filtered.pageCount > 1 ? (
              <nav className="wk-pagination" aria-label={t('common.knowledgeBases')}>
                <Button type="button" disabled={filtered.page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}>{t('common.previous')}</Button>
                <span>{t('common.pageOf', { page: filtered.page, total: filtered.pageCount })}</span>
                <Button type="button" disabled={filtered.page >= filtered.pageCount} onClick={() => setPage((value) => value + 1)}>{t('common.next')}</Button>
              </nav>
            ) : null}
          </>
        ) : null}
      </Card>

      <Dialog open={dialogOpen} title={editingId ? t('common.edit') + ' · ' + t('common.knowledgeBases') : t('knowledgeList.create')} onClose={() => setDialogOpen(false)}>
        <form className="wk-form" onSubmit={save}>
          <label>{t('common.name')} <input data-guide="kb-create-name" value={name} onChange={(event) => setName(event.target.value)} required /></label>
          <label>{'Type'}
            <select data-guide="kb-create-type" value={type} onChange={(event) => setType(event.target.value as 'document' | 'faq')}>
              <option value="document">{t('common.typeDocument')}</option>
              <option value="faq">{t('common.typeFaq')}</option>
            </select>
          </label>
          <label>{t('common.description')} <textarea value={description} onChange={(event) => setDescription(event.target.value)} rows={3} /></label>
          <label>{t('common.embeddingModel')} <input data-guide="kb-create-embedding" value={embeddingModelId} onChange={(event) => setEmbeddingModelId(event.target.value)} placeholder="embedding_model_id" /></label>
          <label>{t('common.summaryModel')} <input data-guide="kb-create-llm" value={summaryModelId} onChange={(event) => setSummaryModelId(event.target.value)} placeholder="summary_model_id" /></label>
          <div className="wk-kb-dialog-actions">
            <Button type="submit" data-guide="kb-create-submit" loading={saving}>{editingId ? t('common.saveChanges') : t('knowledgeList.create')}</Button>
            <Button type="button" onClick={() => setDialogOpen(false)}>{t('common.cancel')}</Button>
          </div>
        </form>
      </Dialog>

      <Dialog
        open={deletingKb !== null}
        title={t('knowledgeList.delete.confirmTitle')}
        onClose={() => { if (!deleting) setDeletingKb(null); }}
        closeLabel={t('common.cancel')}
      >
        <p>{t('knowledgeList.delete.confirmMessage', { name: deletingKb?.name ?? '' })}</p>
        <div className="wk-kb-dialog-actions">
          <Button type="button" className="wk-kb-danger" loading={deleting} onClick={() => void confirmDelete()}>{t('knowledgeList.delete.confirmButton')}</Button>
          <Button type="button" onClick={() => setDeletingKb(null)}>{t('common.cancel')}</Button>
        </div>
      </Dialog>

      <KnowledgeBaseShareDialog client={client} knowledgeBaseId={sharingKb?.id ?? ''} knowledgeBaseName={sharingKb?.name ?? ''} open={sharingKb !== null} onClose={() => setSharingKb(null)} onChanged={() => setReloadToken((value) => value + 1)} />
    </main>
  );
}
