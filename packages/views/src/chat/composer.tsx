import type { FormEvent } from 'react';
import { resolveChatCopy, resolveChatLocale, type ChatCopyTable } from './chat-copy.ts';

export interface ChatSubmission {
  content: string;
  status: 'pending';
}

export function createChatSubmission(draft: string): ChatSubmission {
  const content = draft.trim();
  if (!content) throw new Error('message must not be empty');
  return { content, status: 'pending' };
}

export interface ChatComposerProps {
  draft: string;
  disabled?: boolean;
  onDraftChange(value: string): void;
  onSubmit(submission: ChatSubmission): void;
  /** Agent chip (Vue AgentSelector trigger): select options + current value. */
  agents?: readonly { id: string; name: string; disabled?: boolean }[];
  selectedAgentId?: string;
  onAgentChange?(agentId: string): void;
  /** Display-only chat model chip label (Vue model-selector-trigger). */
  modelLabel?: string;
  /** Compact context suffix next to the label (Vue model-selector-ctx, e.g. 200K). */
  modelContext?: string;
  /** True when the model has no explicit context window (Vue model-selector-ctx is-default). */
  modelContextIsDefault?: boolean;
  /** Vue control-right swaps send for stop while a reply is streaming. */
  streaming?: boolean;
  onStop?(): void;
  /** Resolved copy (chat-copy.ts); defaults to the app locale convention. */
  copy?: ChatCopyTable;
}

/*
 * Vue anatomy (frontend/src/components/Input-field.vue .rich-input-container):
 * rounded shell with the textarea on top and a control bar at the bottom —
 * left chips are the agent selector + attachment/@ buttons, right side holds
 * the model chip and the circular green send (or stop) button.
 */
export function ChatComposer({ draft, disabled = false, onDraftChange, onSubmit, agents, selectedAgentId, onAgentChange, modelLabel, modelContext, modelContextIsDefault, streaming = false, onStop, copy }: ChatComposerProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit(createChatSubmission(draft));
  }

  const showStop = streaming && !draft.trim();

  return <form className="wk-chat-composer" onSubmit={submit}>
    <label className="wk-chat-visually-hidden" htmlFor="wk-chat-draft">{t.composerPlaceholder}</label>
    <div className="wk-chat-input-shell">
      <textarea
        id="wk-chat-draft"
        value={draft}
        onChange={(event) => onDraftChange(event.target.value)}
        disabled={disabled}
        rows={2}
        placeholder={t.composerPlaceholder}
      />
      <div className="wk-chat-control-bar">
        <div className="wk-chat-control-left">
          {agents && onAgentChange ? <span className="wk-chat-agent-chip">
            <select
              id="wk-chat-agent"
              aria-label={t.selectAgent}
              value={selectedAgentId ?? ''}
              onChange={(event) => onAgentChange(event.target.value)}
            >
              <option value="">{t.quickAnswer}</option>
              {agents.map((agent) => <option key={agent.id} value={agent.id} disabled={agent.disabled}>{agent.name}{agent.disabled ? ` · ${t.disabledAgentSuffix}` : ''}</option>)}
            </select>
            <svg className="wk-chat-chip-arrow" width="12" height="12" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" /></svg>
          </span> : null}
          <button type="button" className="wk-chat-control-icon" aria-label={t.uploadAttachment} disabled={disabled} title={t.uploadAttachment}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48" />
            </svg>
          </button>
          <button type="button" className="wk-chat-control-icon" aria-label={t.mentionKnowledge} disabled={disabled} title={t.mentionKnowledge}>
            <svg width="18" height="18" viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <circle cx="10" cy="10" r="3.5" stroke="currentColor" strokeWidth="1.8" />
              <path d="M13.5 10V11.5C13.5 12.163 13.7634 12.7989 14.2322 13.2678C14.7011 13.7366 15.337 14 16 14C16.663 14 17.2989 13.7366 17.7678 13.2678C18.2366 12.7989 18.5 12.163 18.5 11.5V10C18.5 7.74566 17.6045 5.58365 16.0104 3.98959C14.4163 2.39553 12.2543 1.5 10 1.5C7.74566 1.5 5.58365 2.39553 3.98959 3.98959C2.39553 5.58365 1.5 7.74566 1.5 10C1.5 12.2543 2.39553 14.4163 3.98959 16.0104C5.58365 17.6045 7.74566 18.5 10 18.5H12" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
        </div>
        <div className="wk-chat-control-right">
          <span className="wk-chat-model-chip" role="note" aria-label={modelLabel ?? t.modelChip} title={modelLabel ?? t.modelChip}>
            <span className="wk-chat-model-name">{modelLabel ?? t.modelChip}</span>
            {modelContext ? <span className={modelContextIsDefault ? 'wk-chat-model-ctx is-default' : 'wk-chat-model-ctx'}>{modelContext}</span> : null}
            <svg className="wk-chat-chip-arrow" width="10" height="10" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" /></svg>
          </span>
          {showStop && onStop ? <button type="button" className="wk-chat-stop wk-chat-send" aria-label={t.stopGeneration} title={t.stopGeneration} onClick={onStop}>
            <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor" aria-hidden="true"><rect x="2.5" y="2.5" width="9" height="9" rx="1.5" /></svg>
          </button> : <button type="submit" className="wk-chat-send" disabled={disabled || !draft.trim()} aria-label={t.send} title={`${t.send} · Enter`}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M8 13V3" /><path d="M3.5 7.5L8 3l4.5 4.5" /></svg>
          </button>}
        </div>
      </div>
    </div>
  </form>;
}
