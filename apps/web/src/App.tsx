import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController, scopedKey, filterKnowledgeBases } from '@weknora/domain';
import {
  canDuplicateKBCard,
  canManageKBCard,
  isKnowledgeBaseInitialized,
  isSharedKbEditable,
  mergeAllScopeKnowledgeBases,
  type KnowledgeBaseCreatorFilter,
  type MergedKnowledgeBase,
} from '@weknora/domain';
import { formatMessage, isLocale, type Locale, type MessageValues } from '@weknora/i18n';
import { Button, Card, Dialog, Status } from '@weknora/ui';
import {
  createDeleteGuard,
  loadKnowledgeBaseListPage,
  saveKnowledgeBase,
  type KnowledgeBaseListPageState,
  type KnowledgeBaseSaveInput,
} from './knowledge-bases/list.ts';

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

function writeScopeToUrl(space: 'all' | 'mine'): void {
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
  const [modelsReady, setModelsReady] = useState<boolean | null>(null);
  const [space, setSpaceState] = useState<'all' | 'mine'>(readScopeFromUrl);
  const [query, setQuery] = useState(() => new URLSearchParams(window.location.search).get('q') ?? '');
  const [creator, setCreator] = useState<KnowledgeBaseCreatorFilter>('all');
  const [page, setPage] = useState(1);
  const [favorites, setFavorites] = useState<Set<string>>(readFavorites);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  // Create / edit dialog state. Prefills the full config when editing.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [type, setType] = useState<'document' | 'faq'>('document');
  const [embeddingModelId, setEmbeddingModelId] = useState('');
  const [summaryModelId, setSummaryModelId] = useState('');
  const [saving, setSaving] = useState(false);

  // Delete confirmation dialog state; DELETE fires only after confirm.
  const [deletingKb, setDeletingKb] = useState<{ id: string; name: string } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const deleteGuard = useRef(createDeleteGuard(client));

  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'knowledge-bases'), [scope.scope]);

  const setSpace = (next: 'all' | 'mine') => {
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
    void client.configuration.models.list().then((models) => {
      if (!active) return;
      setModelsReady(models.some((model) => model.type === 'llm'));
    }).catch(() => { if (active) setModelsReady(false); });
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

  const filtered = useMemo(() => filterKnowledgeBases(cards, {
    query,
    creator: space === 'mine' ? creator : 'all',
    currentUserId: viewer.userId || undefined,
    page,
    pageSize: 12,
  }), [cards, creator, page, query, space, viewer.userId]);

  useEffect(() => { setPage(1); }, [creator, query, space]);

  const hasUninitialized = useMemo(
    () => cards.some((kb) => !isKnowledgeBaseInitialized(kb as never)),
    [cards],
  );

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
      await saveKnowledgeBase(client, editingId, input);
      setDialogOpen(false);
      setEditingId(null);
      setReloadToken((value) => value + 1);
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

  function openCard(kb: Record<string, unknown>) {
    const id = String(kb.id);
    if (isKnowledgeBaseInitialized(kb as never)) {
      window.location.assign(`/knowledgeBase/${encodeURIComponent(id)}`);
      return;
    }
    // Uninitialized: configure models first — tenant model settings when the
    // tenant has no LLM configured, otherwise this KB's settings page.
    window.location.assign(modelsReady === false ? '/platform/settings' : `/knowledgeBase/${encodeURIComponent(id)}/settings`);
  }

  const isLoading = pageState.status === 'loading';

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <h1>{t('common.knowledgeBases')}</h1>
          <p className="wk-muted">{t('knowledgeList.subtitle')}</p>
        </div>
        {viewer.isContributor ? <Button type="button" onClick={openCreate}>+ {t('knowledgeList.create')}</Button> : null}
      </header>
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {hasUninitialized ? <Status tone="warning">{t('knowledgeList.uninitializedBanner')}</Status> : null}
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
            {viewer.isContributor ? <Button type="button" onClick={openCreate}>+ {t('knowledgeList.create')}</Button> : null}
          </div>
        ) : null}
        {pageState.status === 'success' && filtered.total > 0 ? (
          <>
            <div className="wk-kb-grid">
              {filtered.items.map((card) => {
                const kb = card as Record<string, unknown>;
                const initialized = isKnowledgeBaseInitialized(card as never);
                const manageable = canManageKBCard(card as Record<string, unknown>, { userId: viewer.userId, isAdmin: viewer.isAdmin });
                const duplicable = canDuplicateKBCard(card as Record<string, unknown>, { userId: viewer.userId, isContributor: viewer.isContributor });
                const isWiki = (card as { indexing_strategy?: { wiki_enabled?: boolean } }).indexing_strategy?.wiki_enabled === true;
                const isSharedCard = card.isMine === false;
                const sharedEditable = card.isMine === false && isSharedKbEditable(card.permission);
                const count = typeof card.knowledge_count === 'number' ? card.knowledge_count : 0;
                return (
                  <article key={card.id} className={initialized ? 'wk-kb-card' : 'wk-kb-card wk-kb-card-warning'}>
                    <div className="wk-kb-card-head">
                      <button type="button" className="wk-kb-card-title" onClick={() => openCard(kb)}>{String(card.name ?? '')}</button>
                      <button
                        type="button"
                        className={favorites.has(card.id) ? 'wk-kb-star wk-kb-star-active' : 'wk-kb-star'}
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
                          <Button type="button" onClick={() => window.location.assign(`/knowledgeBase/${encodeURIComponent(card.id)}/settings`)}>{t('common.settings')}</Button>
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
          <label>{t('common.name')} <input value={name} onChange={(event) => setName(event.target.value)} required /></label>
          <label>{'Type'}
            <select value={type} onChange={(event) => setType(event.target.value as 'document' | 'faq')}>
              <option value="document">{t('common.typeDocument')}</option>
              <option value="faq">{t('common.typeFaq')}</option>
            </select>
          </label>
          <label>{t('common.description')} <textarea value={description} onChange={(event) => setDescription(event.target.value)} rows={3} /></label>
          <label>{t('common.embeddingModel')} <input value={embeddingModelId} onChange={(event) => setEmbeddingModelId(event.target.value)} placeholder="embedding_model_id" /></label>
          <label>{t('common.summaryModel')} <input value={summaryModelId} onChange={(event) => setSummaryModelId(event.target.value)} placeholder="summary_model_id" /></label>
          <div className="wk-kb-dialog-actions">
            <Button type="submit" loading={saving}>{editingId ? t('common.saveChanges') : t('knowledgeList.create')}</Button>
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
    </main>
  );
}