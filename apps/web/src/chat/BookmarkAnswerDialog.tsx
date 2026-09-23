/**
 * R490 B3 (R489 N2) — the 添加到知识库 answer-toolbar target.
 * R491 P2 — deep sub-features ported from the Vue manual editor.
 *
 * Vue botmsg.vue handleAddToKnowledge (373-390) prefills the global manual
 * knowledge editor (uiStore.openManualEditor, mode 'create', status 'draft')
 * with formatManualTitle(userQuery) / buildManualMarkdown(userQuery, content)
 * and toasts chat.editorOpened. The React shell has no global editor store,
 * so this dialog is the same-behavior surface: editable title + content
 * prefilled exactly like the Vue editor, a document-KB picker, and a
 * 暂存草稿 (save-draft) action posting to POST /knowledge-bases/:id/
 * knowledge/manual with status 'draft' — the same endpoint the Vue editor
 * saves through (frontend/src/api/knowledge-base/index.ts:254).
 *
 * R491 P2 additions, all anchored on the Vue sources:
 * - Tag multi-select (manual-knowledge-editor.vue manualTagIds + the manual
 *   upload-confirm tags section): per-KB options via documents.tagsPage with
 *   the Vue listKnowledgeTags params { page: 1, page_size: 1000 }
 *   (UploadConfirmDialog.vue loadTags:1296-1310), selection cleared on KB
 *   switch, tag_ids always part of the createManual payload
 *   (manual-knowledge-editor.vue:638).
 * - Preview toggle (manual-knowledge-editor.vue view.editLabel/view.
 *   previewLabel + previewHTML): markdown rendered through the domain-safe
 *   chat renderer, empty placeholder preview.empty.
 * - 发布入库 publish action (manualEditor.actions.publish): Vue validateForm
 *   order KB → title → content → publish content >= 10 trimmed chars
 *   (warning.contentTooShort), then createManual with status 'publish'.
 *   The Vue flow additionally collects process_config through the manual
 *   upload-confirm dialog; the React documents-domain confirm dialog has no
 *   process-config surface yet, so publishing posts without process_config
 *   (backend defaults apply) — recorded as a documents-domain gap.
 */
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { formatMessage, type Locale } from '@weknora/i18n';
import type { ChatCopyTable } from '@weknora/views/chat/chat-copy';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import './chat-u.css';

/** Vue formatManualTitle (chatMessageShared.ts:69-78): collapse whitespace,
 *  truncate at 40 chars with '...', fall back to the session-excerpt label. */
export function formatManualBookmarkTitle(question: string | undefined, excerptLabel: string): string {
  if (!question) return excerptLabel;
  const condensed = question.replace(/\s+/g, ' ').trim();
  if (!condensed) return excerptLabel;
  return condensed.length > 40 ? `${condensed.slice(0, 40)}...` : condensed;
}

/** Vue buildManualMarkdown (chatMessageShared.ts:80-83): the trimmed answer
 *  with the no-answer placeholder fallback. */
export function buildManualBookmarkContent(answer: string, noAnswerLabel: string): string {
  return answer?.trim() || noAnswerLabel;
}

/** manualEditor strings used by this dialog, ported byte-exact from the Vue
 *  locales (frontend/src/i18n/locales/*.ts manualEditor section — the keys
 *  are not in @weknora/i18n yet, so they live here until backfilled). */
export interface ManualEditorStrings {
  publish: string;
  publishedToast: string;
  contentTooShortWarning: string;
  enterContentWarning: string;
  editLabel: string;
  previewLabel: string;
  previewEmpty: string;
}

const MANUAL_EDITOR_STRINGS: Record<Locale, ManualEditorStrings> = {
  'zh-CN': {
    publish: '发布入库',
    publishedToast: '知识已发布并开始索引',
    contentTooShortWarning: '内容过短，建议补充更多信息后再发布',
    enterContentWarning: '请输入知识内容',
    editLabel: '返回编辑',
    previewLabel: '预览内容',
    previewEmpty: '暂无内容',
  },
  'en-US': {
    publish: 'Publish',
    publishedToast: 'Knowledge published and indexing started',
    contentTooShortWarning: 'Content is too short. Please add more information before publishing',
    enterContentWarning: 'Please enter knowledge content',
    editLabel: 'Back to edit',
    previewLabel: 'Preview content',
    previewEmpty: 'No content yet',
  },
  'ja-JP': {
    publish: '公開',
    publishedToast: 'ナレッジを公開し、インデックス作成を開始しました',
    contentTooShortWarning: '内容が短すぎます。公開する前に情報を追加してください',
    enterContentWarning: 'ナレッジの内容を入力してください',
    editLabel: '編集に戻る',
    previewLabel: '内容をプレビュー',
    previewEmpty: 'まだ内容がありません',
  },
  'ko-KR': {
    publish: '게시하기',
    publishedToast: '지식이 게시되고 인덱싱이 시작되었습니다',
    contentTooShortWarning: '내용이 너무 짧습니다. 더 많은 정보를 추가한 후 게시하는 것을 권장합니다',
    enterContentWarning: '지식 내용을 입력해주세요',
    editLabel: '편집으로 돌아가기',
    previewLabel: '내용 미리보기',
    previewEmpty: '내용 없음',
  },
  'ru-RU': {
    publish: 'Опубликовать',
    publishedToast: 'Знание опубликовано и начата индексация',
    contentTooShortWarning: 'Контент слишком короткий. Добавьте больше информации перед публикацией',
    enterContentWarning: 'Введите содержимое знания',
    editLabel: 'Вернуться к редактированию',
    previewLabel: 'Предпросмотр',
    previewEmpty: 'Пока нет содержимого',
  },
};

/** Resolve the ported manualEditor strings for a locale (zh-CN fallback). */
export function manualEditorStrings(locale: Locale | undefined): ManualEditorStrings {
  return MANUAL_EDITOR_STRINGS[locale ?? 'zh-CN'] ?? MANUAL_EDITOR_STRINGS['zh-CN'];
}

interface BookmarkTagOption {
  id: string;
  name: string;
}

export interface BookmarkAnswerDialogProps {
  client: WeKnoraClient;
  copy: ChatCopyTable;
  open: boolean;
  initialTitle: string;
  initialContent: string;
  /** UI locale for the manualEditor/uploadConfirm strings (zh-CN default). */
  locale?: Locale;
  onClose: () => void;
  /** Fired after a successful save with the action's status; the host toasts
   * bookmarkDraftSaved (draft) or publishedToast (publish). */
  onSaved?: (status: 'draft' | 'publish') => void;
}

export function BookmarkAnswerDialog({ client, copy, open, initialTitle, initialContent, locale = 'zh-CN', onClose, onSaved }: BookmarkAnswerDialogProps): ReactNode {
  const strings = manualEditorStrings(locale);
  const [title, setTitle] = useState(initialTitle);
  const [content, setContent] = useState(initialContent);
  const [kbOptions, setKbOptions] = useState<Array<{ id: string; name: string }>>([]);
  const [kbId, setKbId] = useState('');
  const [loadState, setLoadState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [tagOptions, setTagOptions] = useState<BookmarkTagOption[]>([]);
  const [tagLoadState, setTagLoadState] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');
  const [tagIds, setTagIds] = useState<string[]>([]);
  const [view, setView] = useState<'edit' | 'preview'>('edit');
  const [saving, setSaving] = useState(false);
  const [warning, setWarning] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setTitle(initialTitle);
    setContent(initialContent);
    setKbId('');
    setTagOptions([]);
    setTagIds([]);
    setTagLoadState('idle');
    setView('edit');
    setWarning('');
    setError('');
  }, [open, initialTitle, initialContent]);

  useEffect(() => {
    if (!open) return;
    let active = true;
    setLoadState('loading');
    // The Vue manual editor lists document-type KBs only (manual-knowledge-
    // editor.vue), so faq KBs are filtered out here too.
    void client.knowledgeBases.list()
      .then((bases) => {
        if (!active) return;
        setKbOptions(bases.filter((base) => base.type !== 'faq').map((base) => ({ id: base.id, name: base.name })));
        setLoadState('ready');
      })
      .catch(() => {
        if (!active) return;
        setKbOptions([]);
        setLoadState('error');
      });
    return () => { active = false; };
  }, [client, open]);

  // Vue loads the tag options for the publish confirm from the selected KB
  // (UploadConfirmDialog.vue loadTags); here the picker is in-dialog, so the
  // load follows kbId and the selection resets on every switch.
  useEffect(() => {
    if (!open || !kbId) {
      setTagOptions([]);
      setTagLoadState('idle');
      return;
    }
    let active = true;
    setTagLoadState('loading');
    void client.knowledgeBases.documents.tagsPage(kbId, { page: 1, page_size: 1000 })
      .then((response) => {
        if (!active) return;
        setTagOptions(response.data.map((tag) => ({ id: tag.id, name: tag.name })));
        setTagLoadState('ready');
      })
      .catch(() => {
        if (!active) return;
        setTagOptions([]);
        setTagLoadState('error');
      });
    return () => { active = false; };
  }, [client, open, kbId]);

  const previewHtml = useMemo(() => (view === 'preview' && content.trim() ? renderChatMarkdown(content) : ''), [view, content]);

  if (!open) return null;

  function toggleTag(tagId: string): void {
    setTagIds((prev) => prev.includes(tagId) ? prev.filter((id) => id !== tagId) : [...prev, tagId]);
  }

  // Vue manual-knowledge-editor.vue validateForm (600-618): KB → title →
  // content, then the publish-only minimum length (>= 10 trimmed chars).
  function validate(targetStatus: 'draft' | 'publish'): boolean {
    if (!kbId) {
      setWarning(copy.bookmarkSelectKbWarning);
      return false;
    }
    if (!title.trim()) {
      setWarning(copy.bookmarkEnterTitleWarning);
      return false;
    }
    if (!content.trim()) {
      setWarning(strings.enterContentWarning);
      return false;
    }
    if (targetStatus === 'publish' && content.trim().length < 10) {
      setWarning(strings.contentTooShortWarning);
      return false;
    }
    return true;
  }

  async function save(targetStatus: 'draft' | 'publish') {
    if (saving || !validate(targetStatus)) {
      return;
    }
    setSaving(true);
    setWarning('');
    setError('');
    try {
      await client.knowledgeBases.documents.createManual(kbId, {
        title: title.trim(),
        content,
        status: targetStatus,
        // Vue always sends tag_ids (manual-knowledge-editor.vue:638).
        tag_ids: [...tagIds],
      });
      onSaved?.(targetStatus);
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : copy.operationFailed);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="wk-bookmark-dialog-backdrop wk-bad-1" role="presentation" onClick={onClose}>
      <div
        className="wk-bookmark-dialog wk-bad-2"
        role="dialog"
        aria-modal="true"
        aria-label={copy.bookmarkEditorTitle}
        onClick={(event) => event.stopPropagation()}
      >
        <h2 className="wk-bad-3">{copy.bookmarkEditorTitle}</h2>
        <label className="wk-bad-4">
          {copy.bookmarkKbLabel}
          <select
            className="wk-bad-5"
            value={kbId}
            aria-label={copy.bookmarkKbLabel}
            onChange={(event) => { setKbId(event.target.value); setTagIds([]); setWarning(''); }}
          >
            <option value="">{copy.bookmarkKbPlaceholder}</option>
            {kbOptions.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
          </select>
        </label>
        <label className="wk-bad-4">
          {copy.bookmarkTitleLabel}
          <input
            className="wk-bad-5"
            value={title}
            maxLength={100}
            aria-label={copy.bookmarkTitleLabel}
            onChange={(event) => setTitle(event.target.value)}
          />
        </label>
        <div className="wk-bad-6">
          <div className="wk-bad-7">
            <span className="wk-bad-8">{copy.bookmarkContentLabel}</span>
            <button
              type="button"
              className="wk-bad-9"
              disabled={saving}
              aria-label={view === 'edit' ? strings.previewLabel : strings.editLabel}
              onClick={() => setView((prev) => (prev === 'edit' ? 'preview' : 'edit'))}
            >
              {view === 'edit' ? strings.previewLabel : strings.editLabel}
            </button>
          </div>
          {view === 'edit' ? (
            <textarea
              className="wk-bad-10"
              value={content}
              rows={10}
              aria-label={copy.bookmarkContentLabel}
              onChange={(event) => setContent(event.target.value)}
            />
          ) : previewHtml ? (
            <div
              className="wk-bookmark-preview wk-bad-11"
              role="region"
              aria-label={strings.previewLabel}
              dangerouslySetInnerHTML={{ __html: previewHtml }}
            />
          ) : (
            <div
              className="wk-bookmark-preview wk-bad-11"
              role="region"
              aria-label={strings.previewLabel}
            >
              <p className="wk-bookmark-preview-empty wk-bad-12">{strings.previewEmpty}</p>
            </div>
          )}
        </div>
        {kbId && tagLoadState === 'error' ? (
          <p role="alert" className="wk-bad-13">{formatMessage(locale, 'uploadConfirm.tagsLoadFailed')}</p>
        ) : null}
        {kbId && tagLoadState === 'ready' && tagOptions.length === 0 ? (
          <p className="wk-bad-14">{formatMessage(locale, 'uploadConfirm.tagsEmpty')}</p>
        ) : null}
        {kbId && tagLoadState === 'ready' && tagOptions.length > 0 ? (
          <fieldset className="wk-bad-15">
            <legend className="wk-bad-16">{formatMessage(locale, 'uploadConfirm.tabTags')}</legend>
            <span className="wk-bad-17">{formatMessage(locale, 'uploadConfirm.tabTags')}</span>
            {tagOptions.map((tag) => (
              <label key={tag.id} className="wk-bad-18">
                <input
                  type="checkbox"
                  className="wk-bad-accent"
                  checked={tagIds.includes(tag.id)}
                  aria-label={tag.name}
                  onChange={() => toggleTag(tag.id)}
                />
                {tag.name}
              </label>
            ))}
          </fieldset>
        ) : null}
        {loadState === 'error' ? <p role="alert" className="wk-bad-13">{copy.bookmarkNoDocumentKbs}</p> : null}
        {loadState === 'ready' && kbOptions.length === 0 ? <p className="wk-bad-14">{copy.bookmarkNoDocumentKbs}</p> : null}
        {warning ? <p role="alert" className="wk-bad-13">{warning}</p> : null}
        {error ? <p role="alert" className="wk-bad-13">{error}</p> : null}
        <div className="wk-bad-19">
          <button type="button" className="wk-bad-20" disabled={saving} onClick={onClose}>
            {copy.bookmarkCancel}
          </button>
          <button
            type="button"
            className="wk-bad-21"
            disabled={saving}
            onClick={() => void save('draft')}
          >
            {copy.bookmarkSaveDraft}
          </button>
          <button
            type="button"
            className="wk-bad-22"
            disabled={saving}
            onClick={() => void save('publish')}
          >
            {strings.publish}
          </button>
        </div>
      </div>
    </div>
  );
}
