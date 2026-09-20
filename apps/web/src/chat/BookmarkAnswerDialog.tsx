/**
 * R490 B3 (R489 N2) — the 添加到知识库 answer-toolbar target.
 *
 * Vue botmsg.vue handleAddToKnowledge (373-390) prefills the global manual
 * knowledge editor (uiStore.openManualEditor, mode 'create', status 'draft')
 * with formatManualTitle(userQuery) / buildManualMarkdown(userQuery, content)
 * and toasts chat.editorOpened. The React shell has no global editor store,
 * so this dialog is the minimal same-behavior surface: editable title +
 * content prefilled exactly like the Vue editor, a document-KB picker, and a
 * 暂存草稿 (save-draft) action posting to POST /knowledge-bases/:id/
 * knowledge/manual with status 'draft' — the same endpoint the Vue editor
 * saves through (frontend/src/api/knowledge-base/index.ts:254).
 */
import { useEffect, useState, type ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { ChatCopyTable } from '@weknora/views/chat/chat-copy';

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

export interface BookmarkAnswerDialogProps {
  client: WeKnoraClient;
  copy: ChatCopyTable;
  open: boolean;
  initialTitle: string;
  initialContent: string;
  onClose: () => void;
  /** Fired after a successful draft save (host toasts bookmarkDraftSaved). */
  onSaved?: () => void;
}

export function BookmarkAnswerDialog({ client, copy, open, initialTitle, initialContent, onClose, onSaved }: BookmarkAnswerDialogProps): ReactNode {
  const [title, setTitle] = useState(initialTitle);
  const [content, setContent] = useState(initialContent);
  const [kbOptions, setKbOptions] = useState<Array<{ id: string; name: string }>>([]);
  const [kbId, setKbId] = useState('');
  const [loadState, setLoadState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [saving, setSaving] = useState(false);
  const [warning, setWarning] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setTitle(initialTitle);
    setContent(initialContent);
    setKbId('');
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

  if (!open) return null;

  async function saveDraft() {
    // Vue manual-editor warnings: KB first (warning.selectKnowledgeBase),
    // then the title (warning.enterTitle).
    if (!kbId) {
      setWarning(copy.bookmarkSelectKbWarning);
      return;
    }
    if (!title.trim()) {
      setWarning(copy.bookmarkEnterTitleWarning);
      return;
    }
    if (saving) return;
    setSaving(true);
    setWarning('');
    setError('');
    try {
      await client.knowledgeBases.documents.createManual(kbId, {
        title: title.trim(),
        content,
        status: 'draft',
      });
      onSaved?.();
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : copy.operationFailed);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="wk-bookmark-dialog-backdrop fixed inset-0 z-[1200] flex items-center justify-center bg-[rgba(15,23,42,0.45)]" role="presentation" onClick={onClose}>
      <div
        className="wk-bookmark-dialog flex max-h-[85vh] w-[min(720px,92vw)] flex-col gap-3 overflow-auto rounded-[10px] bg-white p-[20px] shadow-[0_12px_40px_rgba(15,23,42,0.2)]"
        role="dialog"
        aria-modal="true"
        aria-label={copy.bookmarkEditorTitle}
        onClick={(event) => event.stopPropagation()}
      >
        <h2 className="m-0 text-[16px] font-semibold text-[#101828]">{copy.bookmarkEditorTitle}</h2>
        <label className="flex flex-col gap-1 text-[13px] text-[#344054]">
          {copy.bookmarkKbLabel}
          <select
            className="h-8 rounded-[6px] border border-[#e4e7ec] bg-transparent px-2 text-[13px] text-[#101828] outline-none focus:border-[#07c05f]"
            value={kbId}
            aria-label={copy.bookmarkKbLabel}
            onChange={(event) => { setKbId(event.target.value); setWarning(''); }}
          >
            <option value="">{copy.bookmarkKbPlaceholder}</option>
            {kbOptions.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-[13px] text-[#344054]">
          {copy.bookmarkTitleLabel}
          <input
            className="h-8 rounded-[6px] border border-[#e4e7ec] bg-transparent px-2 text-[13px] text-[#101828] outline-none focus:border-[#07c05f]"
            value={title}
            maxLength={100}
            aria-label={copy.bookmarkTitleLabel}
            onChange={(event) => setTitle(event.target.value)}
          />
        </label>
        <label className="flex min-h-0 flex-1 flex-col gap-1 text-[13px] text-[#344054]">
          {copy.bookmarkContentLabel}
          <textarea
            className="min-h-[220px] flex-1 resize-y rounded-[6px] border border-[#e4e7ec] bg-transparent px-2 py-1.5 font-mono text-[12px] leading-[1.6] text-[#101828] outline-none focus:border-[#07c05f]"
            value={content}
            rows={10}
            aria-label={copy.bookmarkContentLabel}
            onChange={(event) => setContent(event.target.value)}
          />
        </label>
        {loadState === 'error' ? <p role="alert" className="m-0 text-[12px] text-[#d54941]">{copy.bookmarkNoDocumentKbs}</p> : null}
        {loadState === 'ready' && kbOptions.length === 0 ? <p className="m-0 text-[12px] text-[rgba(0,0,0,0.45)]">{copy.bookmarkNoDocumentKbs}</p> : null}
        {warning ? <p role="alert" className="m-0 text-[12px] text-[#d54941]">{warning}</p> : null}
        {error ? <p role="alert" className="m-0 text-[12px] text-[#d54941]">{error}</p> : null}
        <div className="flex items-center justify-end gap-2 border-t border-[#eef1f5] pt-3">
          <button type="button" className="cursor-pointer rounded-[6px] border border-[#e4e7ec] bg-transparent px-[14px] py-[6px] text-[13px] text-[#344054]" disabled={saving} onClick={onClose}>
            {copy.bookmarkCancel}
          </button>
          <button type="button" className="cursor-pointer rounded-[6px] border border-[#07c05f] bg-[#07c05f] px-[14px] py-[6px] text-[13px] text-white disabled:cursor-not-allowed disabled:opacity-60" disabled={saving} onClick={() => void saveDraft()}>
            {copy.bookmarkSaveDraft}
          </button>
        </div>
      </div>
    </div>
  );
}
