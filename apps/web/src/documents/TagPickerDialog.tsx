// Tag surfaces for the documents page, ported from the Vue baseline:
// - TagPickerDialog  ← BatchTagDialog.vue + TagEditDialog.vue (batch & single modes)
// - TagFilterPanel   ← the tag-filter popup in KnowledgeBase.vue L2465-2556
// - TagManageDialog  ← KbTagManageDrawer.vue (R490 B2: the 管理标签… entry)
// Chips, sections and footer copy mirror the Vue dialogs; state stays local.
import { useEffect, useState, type ReactNode } from 'react';
import { Button, Dialog, Input } from '@weknora/ui';
import { Button as TdButton, Input as TdInput } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import type { KnowledgeTag } from '@weknora/api-client';
import { filterTagOptions, selectTagId, tagCreateFailureMessage } from './tags.ts';
import type { TagSurfaceT } from './tags-locale.ts';

interface TagPickerDialogProps {
  open: boolean;
  t: TagSurfaceT;
  tags: readonly KnowledgeTag[];
  /** batch = Vue BatchTagDialog copy; single = Vue TagEditDialog copy. */
  mode: 'batch' | 'single';
  /** Selection size for the batch subtitle (Vue :count). */
  count?: number;
  preSelectedIds?: readonly string[];
  /** Vue :can-manage — gates the create-tag affordances. */
  canManage?: boolean;
  confirmLoading?: boolean;
  /** POST /knowledge-bases/:id/tags (Vue createKnowledgeBaseTag). */
  createTag?: (name: string) => Promise<KnowledgeTag>;
  onConfirm: (tagIds: string[]) => void;
  onClose: () => void;
  /** Vue @tag-created — lets the page reload its tag list. */
  onTagCreated?: () => void;
}

const COPY = {
  batch: {
    heading: 'knowledgeBase.batchTagDialogHeading',
    selected: 'knowledgeBase.batchTagSelectedSection',
    available: 'knowledgeBase.batchTagAvailableSection',
    noSelected: 'knowledgeBase.batchTagNoSelected',
  },
  single: {
    heading: 'knowledgeBase.tagEditDialogHeading',
    selected: 'knowledgeBase.tagEditSelectedSection',
    available: 'knowledgeBase.tagEditAvailableSection',
    noSelected: 'knowledgeBase.tagEditNoSelected',
  },
} as const;

export function TagPickerDialog({
  open,
  t,
  tags,
  mode,
  count,
  preSelectedIds,
  canManage,
  confirmLoading,
  createTag,
  onConfirm,
  onClose,
  onTagCreated,
}: TagPickerDialogProps): ReactNode {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedSet, setSelectedSet] = useState<ReadonlySet<string>>(new Set(preSelectedIds ?? []));
  const [newTagName, setNewTagName] = useState('');
  const [creatingTag, setCreatingTag] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const copy = COPY[mode];

  if (!open) return null;

  // Vue BatchTagDialog watch(visible): re-seed the selection from preSelectedIds
  // is handled by the parent remounting with fresh preSelectedIds (dialog key).

  const toggleTag = (tagId: string) => {
    setSelectedSet((current) => {
      const next = new Set(current);
      if (next.has(tagId)) next.delete(tagId);
      else next.add(tagId);
      return next;
    });
  };

  const selectedTags = Array.from(selectedSet)
    .map((id) => tags.find((tag) => tag.id === id))
    .filter((tag): tag is KnowledgeTag => Boolean(tag));
  const availableTags = filterTagOptions(tags, Array.from(selectedSet), searchQuery);

  const addNewTag = async (name: string) => {
    const trimmed = name.trim();
    if (!trimmed || !createTag) return;
    setCreateError(null);
    // Vue handleAddNewTag: an existing name just selects the tag.
    const existing = tags.find((tag) => tag.name === trimmed);
    if (existing) {
      setSelectedSet((current) => new Set(selectTagId(Array.from(current), existing.id)));
      setNewTagName('');
      return;
    }
    setCreatingTag(true);
    try {
      const created = await createTag(trimmed);
      setSelectedSet((current) => new Set(current).add(created.id));
      setNewTagName('');
      setSearchQuery('');
      onTagCreated?.();
    } catch (error) {
      setCreateError(tagCreateFailureMessage(error, t('common.operationFailed')));
    } finally {
      setCreatingTag(false);
    }
  };

  return (
    <Dialog open title={t(copy.heading)} onClose={onClose} closeLabel={t('common.cancel')}>
      <div className={mode === 'batch' ? 'batch-tag-body' : 'tag-edit-body'}>
        {mode === 'batch' && typeof count === 'number' ? (
          <p className="batch-tag-subtitle m-0 mb-1 text-[12px] text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.batchTagSubtitle', { count })}</p>
        ) : null}
        <section className="wk-tag-section mt-3 flex flex-col gap-2 border-b border-[var(--wk-border,#e4e7ec)] px-0 py-[10px] first-of-type:border-t-0">
          <div className="wk-tag-section-head flex items-center justify-between gap-2">
            <h4 className="m-0 text-[13px] font-semibold">{t(copy.selected)}</h4>
            {selectedSet.size > 0 ? (
              <button type="button" className="wk-tag-link cursor-pointer border-none bg-transparent p-0 text-[12px] text-[var(--wk-muted,#98a2b8)] hover:text-[var(--wk-brand,#07c05f)]" onClick={() => setSelectedSet(new Set())}>
                {t('knowledgeBase.tagClearAction')}
              </button>
            ) : null}
          </div>
          {selectedTags.length > 0 ? (
            <div className="batch-tag-chips flex max-h-[min(160px,24vh)] flex-wrap gap-[6px] overflow-y-auto">
              {selectedTags.map((tag) => (
                <button
                  key={tag.id}
                  type="button"
                  className="batch-tag-chip is-selected inline-flex min-h-[22px] max-w-full cursor-pointer items-center gap-1 overflow-hidden rounded-[4px] border border-[var(--wk-border,#e4e7ec)] px-2 py-0 text-[11px] text-ellipsis whitespace-nowrap text-[var(--wk-muted,#667085)] bg-transparent border-transparent bg-[var(--wk-surface-strong,#f2f4f7)] font-medium text-[var(--wk-text,#344054)]"
                  title={tag.name}
                  onClick={() => toggleTag(tag.id)}
                >
                  {tag.name}
                </button>
              ))}
            </div>
          ) : (
            <p className="wk-tag-section-empty m-0 min-h-[22px] text-[12px] text-[var(--wk-muted,#98a2b8)]">{t(copy.noSelected)}</p>
          )}
        </section>
        <section className="wk-tag-section mt-3 flex flex-col gap-2 border-b border-[var(--wk-border,#e4e7ec)] px-0 py-[10px] first-of-type:border-t-0">
          <div className="wk-tag-section-head flex items-center justify-between gap-2">
            <h4 className="m-0 text-[13px] font-semibold">{t(copy.available)}</h4>
          </div>
          <Input
            type="search"
            className="wk-tag-search w-full min-h-[28px] my-2 border border-[var(--wk-border,#e4e7ec)] rounded-[6px] bg-transparent px-2 py-0 text-[12px] text-[var(--wk-text,#344054)]"
            value={searchQuery}
            placeholder={t('knowledgeBase.tagEditSearch')}
            aria-label={t('knowledgeBase.tagEditSearch')}
            onChange={(event) => setSearchQuery(event.target.value)}
          />
          {availableTags.length > 0 ? (
            <div className="batch-tag-chips flex max-h-[min(160px,24vh)] flex-wrap gap-[6px] overflow-y-auto">
              {availableTags.map((tag) => (
                <button
                  key={tag.id}
                  type="button"
                  className="batch-tag-chip inline-flex min-h-[22px] max-w-full cursor-pointer items-center gap-1 overflow-hidden rounded-[4px] border border-[var(--wk-border,#e4e7ec)] px-2 py-0 text-[11px] text-ellipsis whitespace-nowrap text-[var(--wk-muted,#667085)] bg-transparent"
                  title={tag.knowledge_count !== undefined ? `${tag.name} (${tag.knowledge_count})` : tag.name}
                  onClick={() => toggleTag(tag.id)}
                >
                  {tag.name}
                </button>
              ))}
            </div>
          ) : (
            <div className="wk-tag-section-empty wk-tag-section-empty--row m-0 flex min-h-[22px] items-center justify-between gap-2 text-[12px] text-[var(--wk-muted,#98a2b8)]">
              <span>{searchQuery.trim() ? t('knowledgeBase.tagEmptyResult') : t('knowledgeBase.noTags')}</span>
              {canManage && createTag && searchQuery.trim() ? (
                <button
                  type="button"
                  className="wk-tag-link cursor-pointer border-none bg-transparent p-0 text-[12px] text-[var(--wk-muted,#98a2b8)] hover:text-[var(--wk-brand,#07c05f)]"
                  disabled={creatingTag}
                  onClick={() => void addNewTag(searchQuery)}
                >
                  {t('knowledgeBase.tagCreateAction')} &quot;{searchQuery.trim()}&quot;
                </button>
              ) : null}
            </div>
          )}
          {canManage && createTag ? (
            <Input
              type="text"
              className="wk-tag-create-input w-full min-h-[28px] my-2 border border-[var(--wk-border,#e4e7ec)] rounded-[6px] bg-transparent px-2 py-0 text-[12px] text-[var(--wk-text,#344054)] border-dashed"
              value={newTagName}
              maxLength={40}
              disabled={creatingTag}
              placeholder={t('knowledgeBase.tagNewPlaceholder')}
              aria-label={t('knowledgeBase.tagNewPlaceholder')}
              onChange={(event) => setNewTagName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  void addNewTag(newTagName);
                }
              }}
            />
          ) : null}
          {createError ? <p className="wk-tag-create-error m-0 text-[12px] text-danger" role="alert">{createError}</p> : null}
        </section>
      </div>
      <div className="batch-tag-footer mt-[14px] flex items-center justify-between gap-3 border-t border-[var(--wk-border,#e4e7ec)] pt-3">
        <span className="batch-tag-selected-count text-[12px] text-[var(--wk-muted,#98a2b8)]">
          {t('knowledgeBase.tagSelectedCount', { count: selectedSet.size })}
        </span>
        <div className="batch-tag-footer-right flex gap-2">
          <Button type="button" className="wk-tag-btn min-h-[30px] cursor-pointer rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-transparent px-[14px] py-0 text-[13px] text-[var(--wk-text,#344054)]" disabled={confirmLoading} onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button
            type="button"
            className="wk-tag-btn wk-tag-btn--primary min-h-[30px] cursor-pointer rounded-[6px] border border-[var(--wk-brand,#07c05f)] bg-[var(--wk-brand,#07c05f)] px-[14px] py-0 text-[13px] text-white"
            disabled={confirmLoading}
            onClick={() => onConfirm(Array.from(selectedSet))}
          >
            {t('common.confirm')}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

interface TagFilterPanelProps {
  t: TagSurfaceT;
  tags: readonly KnowledgeTag[];
  selectedIds: readonly string[];
  onToggle: (tagId: string) => void;
  onClear: () => void;
  onClose: () => void;
  searchQuery?: string;
  onSearch?: (value: string) => void;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  total?: number;
  /** R490 B2 — Vue canEdit gate on the tag-filter footer 管理标签… entry. */
  canManage?: boolean;
  /** Opens the tag manage dialog (Vue openTagManageDrawer closes the panel first). */
  onManage?: () => void;
}

/** The Vue tag-filter popup body (KnowledgeBase.vue L2531-2586). */
export function TagFilterPanel({
  t,
  tags,
  selectedIds,
  onToggle,
  onClear,
  onClose,
  searchQuery = '',
  onSearch = () => undefined,
  hasMore = false,
  loadingMore = false,
  onLoadMore = () => undefined,
  total,
  canManage = false,
  onManage,
}: TagFilterPanelProps): ReactNode {
  // Vue sidebarTags: selections missing from the current page stay visible.
  const missing = selectedIds
    .filter((id) => !tags.some((tag) => tag.id === id))
    .map((id) => tags.find((tag) => tag.id === id))
    .filter((tag): tag is KnowledgeTag => Boolean(tag));
  const visible = [...missing, ...tags];

  return (
    <div className="tag-filter-panel" role="group" aria-label={t('knowledgeBase.tagFilterTitle')} onClick={(event) => event.stopPropagation()}>
      <div className="tag-filter-panel__header">
        <div className="tag-filter-panel__title">
          <span>{t('knowledgeBase.tagFilterTitle')}</span>
          <span className="tag-filter-panel__count">({total ?? tags.length})</span>
        </div>
      </div>
      <div className="tag-search-bar">
        <TdInput
          size="small"
          value={searchQuery}
          placeholder={t('knowledgeBase.tagSearchPlaceholder')}
          clearable
          aria-label={t('knowledgeBase.tagSearchPlaceholder')}
          onChange={(value) => onSearch(String(value).trim())}
          prefixIcon={<TIcon name="search" size="14px" />}
        />
      </div>
      <div className="tag-filter-panel__body">
        <div className="tag-filter-chips">
          {visible.map((tag) => (
            <button
              key={tag.id}
              type="button"
              className={'tag-filter-chip' + (selectedIds.includes(tag.id) ? ' active' : '')}
              title={`${tag.name} (${tag.knowledge_count || 0})`}
              onClick={() => onToggle(tag.id)}
            >
              <span className="tag-filter-chip__label">{tag.name}</span>
              <span className="tag-filter-chip__count">{tag.knowledge_count || 0}</span>
            </button>
          ))}
        </div>
        {!visible.length ? (
          <div className="tag-empty-state">{t('knowledgeBase.tagEmptyResult')}</div>
        ) : null}
        {hasMore ? (
          <div className="tag-load-more">
            <TdButton variant="text" size="small" loading={loadingMore} onClick={onLoadMore}>
              {t('tenant.loadMore')}
            </TdButton>
          </div>
        ) : null}
      </div>
      {canManage && onManage ? (
        <div className="tag-filter-panel__footer">
          <TdButton variant="text" size="small" className="tag-manage-link" onClick={onManage}>
            {t('knowledgeBase.tagManageLink')}
          </TdButton>
        </div>
      ) : null}
    </div>
  );
}

export interface TagManageDialogProps {
  t: TagSurfaceT;
  open: boolean;
  tags: readonly KnowledgeTag[];
  /** POST /knowledge-bases/:id/tags (Vue createKnowledgeBaseTag). */
  createTag: (name: string) => Promise<unknown>;
  /** PUT tag rename (Vue updateKnowledgeBaseTag). */
  updateTag: (tagId: string, name: string) => Promise<unknown>;
  /** DELETE tag by seq_id with force (Vue deleteKnowledgeBaseTag force:true). */
  deleteTag: (tag: KnowledgeTag) => Promise<unknown>;
  onClose: () => void;
  /** Vue @changed — reload the page tag data after each mutation; the
   *  deletedTagId payload mirrors KbTagManageDrawer's emit. */
  onChanged?: (payload?: { deletedTagId?: string }) => void;
}

/**
 * R490 B2 — minimal-form port of KbTagManageDrawer.vue (the 管理标签… target):
 * a dialog listing the KB tags with search + create + rename + delete, the
 * exact affordances the Vue drawer ships. Local list filtering mirrors the
 * React FAQ counterpart (FAQTagManageDialog); the drawer's server-side paging
 * rides on the page-provided tag list instead of a second pager.
 */
export function TagManageDialog({
  t,
  open,
  tags,
  createTag,
  updateTag,
  deleteTag,
  onClose,
  onChanged,
}: TagManageDialogProps): ReactNode {
  const [query, setQuery] = useState('');
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const visible = tags.filter((tag) => !query.trim() || (tag.name || '').toLowerCase().includes(query.trim().toLowerCase()));

  // Vue KbTagManageDrawer resetLocalState on close.
  useEffect(() => {
    if (!open) return;
    setQuery('');
    setCreating(false);
    setDraft('');
    setEditingId(null);
    setEditingName('');
    setError('');
  }, [open]);

  if (!open) return null;

  const failureMessage = (cause: unknown): string =>
    typeof cause === 'object' && cause !== null && 'message' in cause && typeof (cause as { message?: unknown }).message === 'string' && (cause as { message: string }).message.trim()
      ? (cause as { message: string }).message
      : t('common.operationFailed');

  async function submitCreate() {
    const name = draft.trim();
    if (!name) {
      setError(t('knowledgeBase.tagNameRequired'));
      return;
    }
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      await createTag(name);
      setCreating(false);
      setDraft('');
      onChanged?.();
    } catch (cause) {
      setError(failureMessage(cause));
    } finally {
      setBusy(false);
    }
  }

  async function submitRename() {
    if (!editingId) return;
    const name = editingName.trim();
    if (!name) {
      setError(t('knowledgeBase.tagNameRequired'));
      return;
    }
    // Vue submitEditTag: an unchanged name just cancels the editor.
    const current = tags.find((tag) => tag.id === editingId);
    if (current && name === current.name) {
      setEditingId(null);
      return;
    }
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      await updateTag(editingId, name);
      setEditingId(null);
      onChanged?.();
    } catch (cause) {
      setError(failureMessage(cause));
    } finally {
      setBusy(false);
    }
  }

  async function removeTag(tag: KnowledgeTag) {
    if (busy) return;
    if (!window.confirm(t('knowledgeBase.tagDeleteDescDoc', { name: tag.name }))) return;
    setBusy(true);
    setError('');
    try {
      await deleteTag(tag);
      setCreating(false);
      setEditingId(null);
      onChanged?.({ deletedTagId: tag.id });
    } catch (cause) {
      setError(failureMessage(cause));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} title={t('knowledgeBase.tagManageTitle')} onClose={onClose} closeLabel={t('common.cancel')}>
      <p className="tag-manage-description m-0 mt-1 text-[12px] text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.tagManageDescription')}</p>
      {error ? <p className="tag-manage-error m-0 mt-2 text-[12px]" role="alert" style={{ color: 'var(--wk-danger,#d54941)' }}>{error}</p> : null}
      <div className="tag-manage-toolbar mt-3 flex items-center gap-2">
        <Input
          type="search"
          className="min-w-0 flex-1"
          value={query}
          placeholder={t('knowledgeBase.tagSearchPlaceholder')}
          aria-label={t('knowledgeBase.tagSearchPlaceholder')}
          onChange={(event) => setQuery(event.target.value)}
        />
        <Button
          type="button"
          className="shrink-0"
          disabled={busy || creating}
          onClick={() => {
            setCreating(true);
            setEditingId(null);
          }}
        >
          {t('knowledgeBase.tagCreateAction')}
        </Button>
      </div>
      {creating ? (
        <div className="tag-manage-create mt-2 flex items-center gap-2">
          <Input
            autoFocus
            maxLength={40}
            className="min-w-0 flex-1"
            value={draft}
            placeholder={t('knowledgeBase.tagNamePlaceholder')}
            aria-label={t('knowledgeBase.tagNamePlaceholder')}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') void submitCreate();
              if (event.key === 'Escape') setCreating(false);
            }}
          />
          <Button type="button" loading={busy} onClick={() => void submitCreate()}>{t('common.create')}</Button>
          <Button type="button" disabled={busy} onClick={() => setCreating(false)}>{t('common.cancel')}</Button>
        </div>
      ) : null}
      <ul className="tag-manage-list m-0 mt-3 grid list-none gap-2 p-0">
        {visible.map((tag) => editingId === tag.id ? (
          <li key={tag.id} className="tag-manage-row flex items-center justify-between gap-2 border-b border-[var(--wk-border,#e7e7ec)] py-2">
            <Input
              autoFocus
              maxLength={40}
              className="min-w-0 flex-1"
              value={editingName}
              placeholder={t('knowledgeBase.tagNamePlaceholder')}
              aria-label={t('knowledgeBase.tagNamePlaceholder')}
              onChange={(event) => setEditingName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void submitRename();
                if (event.key === 'Escape') setEditingId(null);
              }}
            />
            <Button type="button" loading={busy} onClick={() => void submitRename()}>{t('common.save')}</Button>
            <Button type="button" disabled={busy} onClick={() => setEditingId(null)}>{t('common.cancel')}</Button>
          </li>
        ) : (
          <li key={tag.id} className="tag-manage-row flex items-center justify-between gap-2 border-b border-[var(--wk-border,#e7e7ec)] py-2">
            <span className="grid min-w-0 flex-1 gap-0.5">
              <strong className="truncate text-[13px] font-semibold">{tag.name}</strong>
              <small className="text-[11px] leading-[1.5] text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.tagManageDocCount', { count: tag.knowledge_count || 0 })}</small>
            </span>
            <Button
              type="button"
              disabled={busy}
              onClick={() => {
                setEditingId(tag.id);
                setEditingName(tag.name);
                setCreating(false);
              }}
            >
              {t('knowledgeBase.tagEditAction')}
            </Button>
            <Button type="button" disabled={busy || !Number.isSafeInteger(tag.seq_id)} onClick={() => void removeTag(tag)}>{t('knowledgeBase.tagDeleteAction')}</Button>
          </li>
        ))}
        {visible.length === 0 ? <li className="tag-manage-empty py-4 text-center text-[12px] text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.tagEmptyResult')}</li> : null}
      </ul>
    </Dialog>
  );
}
