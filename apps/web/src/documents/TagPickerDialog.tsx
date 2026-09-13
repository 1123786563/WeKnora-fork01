// Tag surfaces for the documents page, ported from the Vue baseline:
// - TagPickerDialog  ← BatchTagDialog.vue + TagEditDialog.vue (batch & single modes)
// - TagFilterPanel   ← the tag-filter popup in KnowledgeBase.vue L2465-2556
// Chips, sections and footer copy mirror the Vue dialogs; state stays local.
import { useState, type ReactNode } from 'react';
import { Dialog } from '@weknora/ui';
import type { KnowledgeTag } from '@weknora/api-client';
import { filterTagOptions, paginateTagOptions, TAG_PANEL_PAGE_SIZE } from './tags.ts';
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
    // Vue handleAddNewTag: an existing name just selects the tag.
    const existing = tags.find((tag) => tag.name === trimmed);
    if (existing) {
      toggleTag(existing.id);
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
    } finally {
      setCreatingTag(false);
    }
  };

  return (
    <Dialog open title={t(copy.heading)} onClose={onClose} closeLabel={t('common.cancel')}>
      <div className={mode === 'batch' ? 'batch-tag-body' : 'tag-edit-body'}>
        {mode === 'batch' && typeof count === 'number' ? (
          <p className="batch-tag-subtitle">{t('knowledgeBase.batchTagSubtitle', { count })}</p>
        ) : null}
        <section className="wk-tag-section">
          <div className="wk-tag-section-head">
            <h4>{t(copy.selected)}</h4>
            {selectedSet.size > 0 ? (
              <button type="button" className="wk-tag-link" onClick={() => setSelectedSet(new Set())}>
                {t('knowledgeBase.tagClearAction')}
              </button>
            ) : null}
          </div>
          {selectedTags.length > 0 ? (
            <div className="batch-tag-chips">
              {selectedTags.map((tag) => (
                <button
                  key={tag.id}
                  type="button"
                  className="batch-tag-chip is-selected"
                  title={tag.name}
                  onClick={() => toggleTag(tag.id)}
                >
                  {tag.name}
                </button>
              ))}
            </div>
          ) : (
            <p className="wk-tag-section-empty">{t(copy.noSelected)}</p>
          )}
        </section>
        <section className="wk-tag-section">
          <div className="wk-tag-section-head">
            <h4>{t(copy.available)}</h4>
          </div>
          <input
            type="search"
            className="wk-tag-search"
            value={searchQuery}
            placeholder={t('knowledgeBase.tagEditSearch')}
            aria-label={t('knowledgeBase.tagEditSearch')}
            onChange={(event) => setSearchQuery(event.target.value)}
          />
          {availableTags.length > 0 ? (
            <div className="batch-tag-chips">
              {availableTags.map((tag) => (
                <button
                  key={tag.id}
                  type="button"
                  className="batch-tag-chip"
                  title={tag.knowledge_count !== undefined ? `${tag.name} (${tag.knowledge_count})` : tag.name}
                  onClick={() => toggleTag(tag.id)}
                >
                  {tag.name}
                </button>
              ))}
            </div>
          ) : (
            <div className="wk-tag-section-empty wk-tag-section-empty--row">
              <span>{searchQuery.trim() ? t('knowledgeBase.tagEmptyResult') : t('knowledgeBase.noTags')}</span>
              {canManage && createTag && searchQuery.trim() ? (
                <button
                  type="button"
                  className="wk-tag-link"
                  disabled={creatingTag}
                  onClick={() => void addNewTag(searchQuery)}
                >
                  {t('knowledgeBase.tagCreateAction')} &quot;{searchQuery.trim()}&quot;
                </button>
              ) : null}
            </div>
          )}
          {canManage && createTag ? (
            <input
              type="text"
              className="wk-tag-create-input"
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
        </section>
      </div>
      <div className="batch-tag-footer">
        <span className="batch-tag-selected-count">
          {t('knowledgeBase.tagSelectedCount', { count: selectedSet.size })}
        </span>
        <div className="batch-tag-footer-right">
          <button type="button" className="wk-tag-btn" disabled={confirmLoading} onClick={onClose}>
            {t('common.cancel')}
          </button>
          <button
            type="button"
            className="wk-tag-btn wk-tag-btn--primary"
            disabled={confirmLoading}
            onClick={() => onConfirm(Array.from(selectedSet))}
          >
            {t('common.confirm')}
          </button>
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
}

/** The Vue tag-filter popup body (KnowledgeBase.vue L2468-2523). */
export function TagFilterPanel({
  t,
  tags,
  selectedIds,
  onToggle,
  onClear,
  onClose,
}: TagFilterPanelProps): ReactNode {
  const [searchQuery, setSearchQuery] = useState('');
  const [visibleCount, setVisibleCount] = useState(TAG_PANEL_PAGE_SIZE);

  const query = searchQuery.trim().toLowerCase();
  const matched = query
    ? tags.filter((tag) => (tag.name || '').toLowerCase().includes(query))
    : [...tags];
  // Vue sidebarTags: selections missing from the current page stay visible.
  const missing = selectedIds
    .filter((id) => !matched.some((tag) => tag.id === id))
    .map((id) => tags.find((tag) => tag.id === id))
    .filter((tag): tag is KnowledgeTag => Boolean(tag));
  const ordered = [...missing, ...matched];
  const visible = paginateTagOptions(ordered, visibleCount);
  const hasMore = ordered.length > visible.length;

  return (
    <div className="tag-filter-panel" role="group" aria-label={t('knowledgeBase.tagFilterTitle')}>
      <div className="tag-filter-panel__header">
        <span className="tag-filter-panel__title">{t('knowledgeBase.tagFilterTitle')}</span>
        <span className="tag-filter-panel__count">({tags.length})</span>
        <button type="button" className="wk-tag-link wk-tag-panel-close" aria-label={t('common.cancel')} onClick={onClose}>
          ×
        </button>
      </div>
      <div className="tag-filter-panel__search">
        <input
          type="search"
          value={searchQuery}
          placeholder={t('knowledgeBase.tagSearchPlaceholder')}
          aria-label={t('knowledgeBase.tagSearchPlaceholder')}
          onChange={(event) => setSearchQuery(event.target.value)}
        />
      </div>
      <div className="tag-filter-panel__body">
        {visible.length > 0 ? (
          <div className="tag-filter-chips">
            {visible.map((tag) => (
              <button
                key={tag.id}
                type="button"
                className={selectedIds.includes(tag.id) ? 'tag-filter-chip is-active' : 'tag-filter-chip'}
                title={`${tag.name} (${tag.knowledge_count || 0})`}
                onClick={() => onToggle(tag.id)}
              >
                <span className="tag-filter-chip__label">{tag.name}</span>
                <span className="tag-filter-chip__count">{tag.knowledge_count || 0}</span>
              </button>
            ))}
          </div>
        ) : (
          <p className="wk-tag-section-empty">{t('knowledgeBase.tagEmptyResult')}</p>
        )}
        {hasMore ? (
          <button
            type="button"
            className="wk-tag-link wk-tag-load-more"
            onClick={() => setVisibleCount((value) => value + TAG_PANEL_PAGE_SIZE)}
          >
            {t('tenant.loadMore')}
          </button>
        ) : null}
      </div>
      {selectedIds.length > 0 ? (
        <div className="tag-filter-panel__footer">
          <button type="button" className="wk-tag-link" onClick={onClear}>
            {t('knowledgeBase.tagClearAction')}
          </button>
        </div>
      ) : null}
    </div>
  );
}
