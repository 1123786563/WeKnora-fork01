import { useEffect, useRef, useState, type ChangeEvent, type FormEvent, type KeyboardEvent } from 'react';
import { resolveChatCopy, resolveChatLocale, type ChatCopyTable } from './chat-copy.ts';
import { AgentSelectorPanel, type AgentSelectorAgent, type AgentSelectorModel } from './agent-selector.tsx';

export interface ChatSubmission {
  content: string;
  status: 'pending';
  modelId?: string;
}

export type ChatAttachmentStatus = 'pending' | 'uploading' | 'uploaded' | 'processing' | 'ready' | 'failed';

export interface ChatAttachmentView {
  id: string;
  name: string;
  status: ChatAttachmentStatus;
  attachmentId?: string;
  error?: string;
}

export interface ChatMentionView {
  id: string;
  name: string;
  type: 'kb' | 'file' | 'tag' | 'mcp' | 'skill';
  kbType?: 'document' | 'faq';
  kbId?: string;
  kbName?: string;
  skillName?: string;
  description?: string;
  toolCount?: number;
  catalogStale?: boolean;
}

function mentionMarker(type: ChatMentionView['type']): string {
  return type === 'file' ? '▧' : type === 'tag' ? '#' : type === 'mcp' ? '⚒' : type === 'skill' ? '✦' : '@';
}

export function createChatSubmission(draft: string): ChatSubmission {
  const content = draft.trim();
  if (!content) throw new Error('message must not be empty');
  return { content, status: 'pending' };
}

type ChatKeyboardEvent = Pick<KeyboardEvent, 'key' | 'keyCode' | 'shiftKey' | 'ctrlKey' | 'altKey' | 'metaKey'> & { isComposing?: boolean };

export function shouldSubmitFromKeyboard(event: ChatKeyboardEvent, canSteer: boolean): boolean {
  if (event.isComposing || event.keyCode === 229 || (event.key !== 'Enter' && event.keyCode !== 13)) return false;
  if (event.shiftKey || event.ctrlKey) return false;
  if (event.altKey && !event.metaKey && !canSteer) return false;
  return true;
}

export interface ChatComposerProps {
  draft: string;
  /**
   * R466-A2 — Vue prefillQuery focus pulse: when the number changes to a
   * truthy value the textarea is focused (Input-field.vue consumes the
   * prefill then focuses in nextTick). 0/undefined keeps default behavior.
   */
  focusSignal?: number;
  disabled?: boolean;
  onDraftChange(value: string): void;
  onSubmit(submission: ChatSubmission): void;
  attachments?: readonly ChatAttachmentView[];
  onAttachmentSelect?(file: File): void | Promise<void>;
  onRemoveAttachment?(id: string): void | Promise<void>;
  attachmentAccept?: readonly string[];
  /** Resource-level @ mentions loaded by the application client. */
  mentionOptions?: readonly ChatMentionView[];
  mentionedItems?: readonly ChatMentionView[];
  /** Test/host initial state; interactive toggling remains local. */
  mentionOpen?: boolean;
  mentionLoading?: boolean;
  mentionError?: string;
  onMentionOpen?(): void;
  onMentionSelect?(item: ChatMentionView): void;
  onMentionRemove?(id: string): void;
  /** Agent chip (Vue AgentSelector trigger): selector panel options + current value. */
  agents?: readonly { id: string; name: string; disabled?: boolean; description?: string; is_builtin?: boolean; config?: Record<string, unknown> }[];
  selectedAgentId?: string;
  onAgentChange?(agentId: string): void;
  /** Models for the chat-readiness gate (upstream AgentSelector allModels). */
  agentModels?: readonly { id: string; type?: string }[];
  /** Opens the agents management page (upstream selector header entry). */
  onManageAgents?(): void;
  /** Opens the agent editor at the section fixing missing config. */
  onConfigureAgent?(agent: { id: string }, section: string, highlight?: 'summary_model' | 'rerank_model'): void;
  /** Vue Input-field surfaces the not-ready block as a toast. */
  onAgentNotReady?(agent: { id: string; name: string }, labels: string[]): void;
  /** Display-only chat model chip label (Vue model-selector-trigger). */
  modelLabel?: string;
  /** Compact context suffix next to the label (Vue model-selector-ctx, e.g. 200K). */
  modelContext?: string;
  /** True when the model has no explicit context window (Vue model-selector-ctx is-default). */
  modelContextIsDefault?: boolean;
  modelOptions?: readonly { id: string; name: string }[];
  selectedModelId?: string;
  onModelChange?(modelId: string): void;
  /** Vue control-right swaps send for stop while a reply is running (isReplying: dispatched through stream end, incl. the pre-stream window). */
  streaming?: boolean;
  /** Vue shows stop whenever the active session cannot accept a steer. */
  canSteer?: boolean;
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
export function ChatComposer({ draft, focusSignal = 0, disabled = false, onDraftChange, onSubmit, attachments = [], onAttachmentSelect, onRemoveAttachment, attachmentAccept, mentionOptions = [], mentionedItems = [], mentionOpen: initialMentionOpen = false, mentionLoading = false, mentionError, onMentionOpen, onMentionSelect, onMentionRemove, agents, selectedAgentId, onAgentChange, agentModels, onManageAgents, onConfigureAgent, onAgentNotReady, modelLabel, modelContext, modelContextIsDefault, modelOptions = [], selectedModelId, onModelChange, streaming = false, canSteer = false, onStop, copy }: ChatComposerProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const attachmentInputRef = useRef<HTMLInputElement>(null);
  const mentionSearchRef = useRef<HTMLInputElement>(null);
  const draftRef = useRef<HTMLTextAreaElement>(null);
  // Vue Input-field.vue prefill consume: nextTick(() => textarea.focus()).
  useEffect(() => {
    if (focusSignal) draftRef.current?.focus();
  }, [focusSignal]);
  const [mentionOpen, setMentionOpen] = useState(initialMentionOpen);
  const [mentionQuery, setMentionQuery] = useState('');
  const [activeMentionIndex, setActiveMentionIndex] = useState(0);
  const [agentPanelOpen, setAgentPanelOpen] = useState(false);
  const agentChipRef = useRef<HTMLButtonElement>(null);
  function submitDraft(): void {
    if (!draft.trim()) return;
    onSubmit({ ...createChatSubmission(draft), ...(selectedModelId ? { modelId: selectedModelId } : {}) });
    // Vue Input-field.vue createSession → clearvalue(): the query is cleared
    // the moment the message is emitted (send and steer path alike), before
    // the turn resolves. With the draft cleared, the control-right stop
    // condition `isReplying && (!canSteer || !draft.trim())` can actually win
    // while a reply runs.
    onDraftChange('');
  }
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    submitDraft();
  }
  function handleDraftKeyDown(event: KeyboardEvent<HTMLTextAreaElement>): void {
    if (disabled || !shouldSubmitFromKeyboard(event, canSteer)) return;
    event.preventDefault();
    submitDraft();
  }

  const showStop = streaming && (!canSteer || !draft.trim());
  function selectAttachments(event: ChangeEvent<HTMLInputElement>): void {
    const files = Array.from(event.target.files ?? []);
    event.target.value = '';
    for (const file of files) void onAttachmentSelect?.(file);
  }

  const filteredMentionOptions = mentionOptions.filter((item) => item.name.toLocaleLowerCase().includes(mentionQuery.trim().toLocaleLowerCase()) && !mentionedItems.some((selected) => selected.id === item.id));
  function toggleMentions(): void {
    if (disabled) return;
    const next = !mentionOpen;
    setMentionOpen(next);
    if (next) {
      setMentionQuery('');
      setActiveMentionIndex(0);
      onMentionOpen?.();
      window.setTimeout(() => mentionSearchRef.current?.focus(), 0);
    }
  }
  function closeMentions(): void {
    setMentionOpen(false);
    setMentionQuery('');
  }
  function handleMentionKeyDown(event: KeyboardEvent<HTMLInputElement>): void {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeMentions();
      return;
    }
    if (event.key === 'ArrowDown' && filteredMentionOptions.length > 0) {
      event.preventDefault();
      setActiveMentionIndex((current) => Math.min(current + 1, filteredMentionOptions.length - 1));
      return;
    }
    if (event.key === 'ArrowUp' && filteredMentionOptions.length > 0) {
      event.preventDefault();
      setActiveMentionIndex((current) => Math.max(current - 1, 0));
      return;
    }
    if (event.key === 'Enter' && filteredMentionOptions.length > 0) {
      event.preventDefault();
      onMentionSelect?.(filteredMentionOptions[Math.min(activeMentionIndex, filteredMentionOptions.length - 1)]);
      closeMentions();
    }
  }

  function attachmentStatusLabel(attachment: ChatAttachmentView): string {
    if (attachment.status === 'pending') return t.uploadAttachment;
    if (attachment.status === 'uploading') return t.sending;
    if (attachment.status === 'ready') return t.available;
    if (attachment.status === 'failed') return attachment.error || t.sendFailed;
    return t.attachmentProcessing;
  }

  return <form className="wk-chat-composer relative mx-auto w-full max-w-[960px] shrink-0" onSubmit={submit}>
    <label className="wk-chat-visually-hidden absolute h-[1px] w-[1px] overflow-hidden whitespace-nowrap [clip:rect(0_0_0_0)] [clip-path:inset(50%)]" htmlFor="wk-chat-draft">{t.composerPlaceholder}</label>
    <div data-guide="chat-input" className="wk-chat-input-shell w-full rounded-[12px] border border-[#dcdcdc] bg-white shadow-[0_2px_8px_rgba(0,0,0,0.04),0_8px_16px_-4px_rgba(0,0,0,0.06)] transition-[border-color] duration-[150ms] ease-[ease] focus-within:border-[#07c05f]">
      {attachments.length > 0 ? <ul className="wk-chat-attachments m-0 flex flex-wrap gap-[6px] px-[14px] pt-[10px]" aria-label={t.uploadAttachment}>
        {attachments.map((attachment) => <li key={attachment.id} data-attachment-status={attachment.status} className="inline-flex max-w-full items-center gap-[6px] rounded-[6px] border border-[#e7e7e7] bg-[#fafafa] px-[8px] py-[4px] text-[12px] text-[rgba(0,0,0,0.65)]" title={attachment.error || attachment.status}>
          <span className="max-w-[220px] overflow-hidden text-ellipsis whitespace-nowrap">{attachment.name}</span>
          <span aria-label={attachmentStatusLabel(attachment)}>{attachment.status === 'ready' ? '✓' : attachment.status === 'failed' ? '!' : '…'} {attachmentStatusLabel(attachment)}</span>
          {onRemoveAttachment ? <button type="button" className="cursor-pointer border-0 bg-transparent p-0 text-[rgba(0,0,0,0.4)] hover:text-[rgba(0,0,0,0.9)]" aria-label={`${t.close}: ${attachment.name}`} onClick={() => void onRemoveAttachment(attachment.id)}>×</button> : null}
        </li>)}
      </ul> : null}
      {mentionedItems.length > 0 ? <ul className="wk-chat-mentions m-0 flex flex-wrap gap-[6px] px-[14px] pt-[10px]" aria-label={t.mentionKnowledge}>
        {mentionedItems.map((item) => <li key={item.id} data-mention-id={item.id} data-mention-type={item.type} className="inline-flex max-w-full items-center gap-[6px] rounded-[6px] border border-[#d9f2e2] bg-[#f2fbf5] px-[8px] py-[4px] text-[12px] text-[rgba(0,0,0,0.65)]">
          <span aria-hidden="true">{mentionMarker(item.type)}</span><span className="max-w-[220px] overflow-hidden text-ellipsis whitespace-nowrap">{item.name}</span>
          {onMentionRemove ? <button type="button" className="cursor-pointer border-0 bg-transparent p-0 text-[rgba(0,0,0,0.4)] hover:text-[rgba(0,0,0,0.9)]" aria-label={`${t.close}: ${item.name}`} onClick={() => onMentionRemove(item.id)}>×</button> : null}
        </li>)}
      </ul> : null}
      <textarea
        id="wk-chat-draft"
        ref={draftRef}
        value={draft}
        onChange={(event) => onDraftChange(event.target.value)}
        onKeyDown={handleDraftKeyDown}
        disabled={disabled}
        rows={2}
        placeholder={t.composerPlaceholder}
        className="block box-border h-[72px] w-full min-h-[72px] resize-none overflow-auto border-0 bg-transparent px-[16px] pt-[16px] pb-[8px] [font:inherit] text-[16px] leading-[24px] text-[rgba(0,0,0,0.9)] outline-none placeholder:text-[rgba(0,0,0,0.26)] disabled:cursor-not-allowed disabled:bg-transparent disabled:text-[rgba(0,0,0,0.4)]"
      />
      <div className="wk-chat-control-bar relative mx-[16px] mb-[12px] mt-0 flex flex-wrap items-center justify-between gap-[8px] pt-[8px]">
        <div className="wk-chat-control-left flex min-w-0 flex-1 flex-wrap items-center gap-[8px]">
          {agents && onAgentChange ? (() => {
            const currentAgent = agents.find((agent) => agent.id === selectedAgentId);
            const chipLabel = currentAgent?.name ?? t.quickAnswer;
            const panelOpen = agentPanelOpen && typeof document !== 'undefined' && agentChipRef.current;
            return <>
              <button
                type="button"
                ref={agentChipRef}
                id="wk-chat-agent"
                aria-label={t.selectAgent}
                aria-haspopup="dialog"
                aria-expanded={agentPanelOpen}
                disabled={disabled}
                onClick={() => setAgentPanelOpen((open) => !open)}
                className="wk-chat-agent-chip relative inline-flex h-[28px] cursor-pointer items-center gap-[2px] rounded-[6px] border-[0.5px] border-[#e7e7e7] bg-transparent px-[8px] py-0 text-[13px] font-medium text-[rgba(0,0,0,0.6)] hover:bg-[#f7f7f7] disabled:cursor-not-allowed disabled:opacity-50"
              >
                <span className="max-w-[140px] overflow-hidden text-ellipsis whitespace-nowrap">{chipLabel}</span>
                <svg className="shrink-0 text-[rgba(0,0,0,0.26)]" width="12" height="12" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" /></svg>
              </button>
              {panelOpen && agentChipRef.current ? <AgentSelectorPanel
                copy={t}
                currentAgentId={selectedAgentId ?? ''}
                agents={agents.filter((agent) => !agent.disabled) as readonly AgentSelectorAgent[]}
                models={(agentModels ?? []) as readonly AgentSelectorModel[]}
                anchorRect={agentChipRef.current.getBoundingClientRect()}
                onSelect={(agentId) => { setAgentPanelOpen(false); onAgentChange(agentId); }}
                onNotReady={(agent, labels) => { onAgentNotReady?.(agent, labels); }}
                onManage={() => { onManageAgents?.(); }}
                onConfigureAgent={(agent, section, highlight) => { onConfigureAgent?.(agent, section, highlight); }}
                onClose={() => setAgentPanelOpen(false)}
              /> : null}
            </>;
          })() : null}
          <input ref={attachmentInputRef} type="file" accept={attachmentAccept?.join(',')} multiple className="absolute h-px w-px overflow-hidden opacity-0" tabIndex={-1} aria-hidden="true" onChange={selectAttachments} />
          <button type="button" className="wk-chat-control-icon flex h-[28px] w-[28px] shrink-0 cursor-pointer items-center justify-center rounded-[6px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] transition-[background,color] duration-[120ms] enabled:hover:bg-[#eee] enabled:hover:text-[rgba(0,0,0,0.9)] disabled:cursor-not-allowed disabled:opacity-50" aria-label={t.uploadAttachment} disabled={disabled || !onAttachmentSelect} title={t.uploadAttachment} onClick={() => attachmentInputRef.current?.click()}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48" />
            </svg>
          </button>
          <div className="relative">
          <button type="button" data-guide="chat-kb-mention" className="wk-chat-control-icon flex h-[28px] w-[28px] shrink-0 cursor-pointer items-center justify-center rounded-[6px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] transition-[background,color] duration-[120ms] enabled:hover:bg-[#eee] enabled:hover:text-[rgba(0,0,0,0.9)] disabled:cursor-not-allowed disabled:opacity-50" aria-label={t.mentionKnowledge} aria-expanded={mentionOpen} aria-controls="wk-chat-mention-listbox" disabled={disabled} title={t.mentionKnowledge} onClick={toggleMentions}>
            <svg width="18" height="18" viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <circle cx="10" cy="10" r="3.5" stroke="currentColor" strokeWidth="1.8" />
              <path d="M13.5 10V11.5C13.5 12.163 13.7634 12.7989 14.2322 13.2678C14.7011 13.7366 15.337 14 16 14C16.663 14 17.2989 13.7366 17.7678 13.2678C18.2366 12.7989 18.5 12.163 18.5 11.5V10C18.5 7.74566 17.6045 5.58365 16.0104 3.98959C14.4163 2.39553 12.2543 1.5 10 1.5C7.74566 1.5 5.58365 2.39553 3.98959 3.98959C2.39553 5.58365 1.5 7.74566 1.5 10C1.5 12.2543 2.39553 14.4163 3.98959 16.0104C5.58365 17.6045 7.74566 18.5 10 18.5H12" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
          {mentionOpen ? <div id="wk-chat-mention-listbox" role="listbox" aria-label={t.mentionKnowledge} className="absolute bottom-[36px] left-0 z-20 w-[280px] rounded-[8px] border border-[#e7e7e7] bg-white p-[8px] shadow-[0_8px_24px_rgba(0,0,0,0.12)]">
            <input ref={mentionSearchRef} value={mentionQuery} onChange={(event) => { setMentionQuery(event.target.value); setActiveMentionIndex(0); }} onKeyDown={handleMentionKeyDown} aria-label={t.composerPlaceholder} aria-activedescendant={filteredMentionOptions.length > 0 ? `wk-chat-mention-option-${filteredMentionOptions[Math.min(activeMentionIndex, filteredMentionOptions.length - 1)].id}` : undefined} aria-controls="wk-chat-mention-options" placeholder={t.composerPlaceholder} className="mb-[6px] box-border w-full rounded-[6px] border border-[#e7e7e7] px-[8px] py-[6px] text-[12px] outline-none focus:border-[#07c05f]" />
            {mentionLoading ? <p role="status" className="m-0 px-[8px] py-[8px] text-[12px] text-[rgba(0,0,0,0.45)]">{t.loadingMessages}</p> : mentionError ? <p role="alert" className="m-0 px-[8px] py-[8px] text-[12px] text-[#d54941]">{mentionError}</p> : filteredMentionOptions.length > 0 ? <div className="max-h-[220px] overflow-y-auto">
              <div id="wk-chat-mention-options">
              {filteredMentionOptions.map((item, index) => <button key={item.id} id={`wk-chat-mention-option-${item.id}`} type="button" role="option" aria-selected={index === activeMentionIndex} data-mention-id={item.id} data-mention-type={item.type} className={index === activeMentionIndex ? 'flex w-full cursor-pointer items-center gap-[8px] rounded-[6px] border-0 bg-[#f3f3f3] px-[8px] py-[7px] text-left text-[13px] text-[rgba(0,0,0,0.75)] focus:outline-none' : 'flex w-full cursor-pointer items-center gap-[8px] rounded-[6px] border-0 bg-transparent px-[8px] py-[7px] text-left text-[13px] text-[rgba(0,0,0,0.75)] hover:bg-[#f3f3f3] focus:bg-[#f3f3f3] focus:outline-none'} onMouseEnter={() => setActiveMentionIndex(index)} onClick={() => { onMentionSelect?.(item); closeMentions(); }}><span aria-hidden="true">{mentionMarker(item.type)}</span><span className="overflow-hidden text-ellipsis whitespace-nowrap">{item.name}</span></button>)}
              </div>
            </div> : <p className="m-0 px-[8px] py-[8px] text-[12px] text-[rgba(0,0,0,0.45)]">{mentionQuery ? t.mentionNoResults : t.mentionNoAvailable}</p>}
          </div> : null}
          </div>
        </div>
        <div className="wk-chat-control-right flex items-center gap-[8px]">
          {modelOptions.length > 0 && onModelChange ? <label className="wk-chat-model-chip relative flex h-[22px] min-w-[100px] items-center rounded-[6px] border-[0.5px] border-[#e7e7e7] bg-transparent px-[8px] py-[2px] text-left"><span className="sr-only">{t.modelChip}</span><select aria-label={t.modelChip} value={selectedModelId ?? modelOptions[0]?.id ?? ''} onChange={(event) => onModelChange(event.target.value)} className="h-full w-full cursor-pointer appearance-none border-0 bg-transparent pr-[14px] text-[12px] font-medium text-[rgba(0,0,0,0.6)] outline-none">{modelOptions.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}</select><svg className="wk-chat-chip-arrow pointer-events-none absolute right-[8px] shrink-0 text-[rgba(0,0,0,0.26)]" width="10" height="10" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" /></svg></label> : <button type="button" className="wk-chat-model-chip flex h-[22px] min-w-[100px] cursor-not-allowed items-center gap-[6px] rounded-[6px] border-[0.5px] border-[#e7e7e7] bg-transparent px-[8px] py-[2px] text-left opacity-75" disabled aria-disabled="true" aria-label={modelLabel ?? t.modelChip} title={modelLabel ?? t.modelChip}>
            <span className="wk-chat-model-name min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-[12px] font-medium text-[rgba(0,0,0,0.6)]">{modelLabel ?? t.modelChip}</span>
            {modelContext ? <span className={modelContextIsDefault ? 'wk-chat-model-ctx is-default shrink-0 text-[11px] font-normal text-[rgba(0,0,0,0.45)] opacity-85' : 'wk-chat-model-ctx shrink-0 text-[11px] font-normal text-[rgba(0,0,0,0.45)]'}>{modelContext}</span> : null}
            <svg className="wk-chat-chip-arrow static shrink-0 text-[rgba(0,0,0,0.26)]" width="10" height="10" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" /></svg>
          </button>}
          {showStop && onStop ? <button type="button" className="wk-chat-stop wk-chat-send flex h-[28px] w-[28px] shrink-0 cursor-pointer items-center justify-center rounded-full border-0 bg-[#07c05f] p-0 text-[16px] leading-none text-white transition-[background-color,opacity] duration-[150ms] ease-[ease] enabled:hover:bg-[#08dd6e] disabled:cursor-not-allowed disabled:bg-[#8ce0af] focus-visible:outline-[2px] focus-visible:outline-[#07c05f] focus-visible:outline-offset-2" aria-label={t.stopGeneration} title={t.stopGeneration} onClick={onStop}>
            <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor" aria-hidden="true"><rect x="2.5" y="2.5" width="9" height="9" rx="1.5" /></svg>
          </button> : (() => {
            // Vue steer-mode labelling: while replying on a steer-capable turn
            // the same circular button queues the follow-up (input.steerAfter),
            // so both the tooltip and aria-label swap away from input.send.
            const steerMode = streaming && canSteer;
            const actionLabel = steerMode ? t.steerQueued : t.send;
            return <button type="submit" data-guide="chat-send" className="wk-chat-send flex h-[28px] w-[28px] shrink-0 cursor-pointer items-center justify-center rounded-full border-0 bg-[#07c05f] p-0 text-[16px] leading-none text-white transition-[background-color,opacity] duration-[150ms] ease-[ease] enabled:hover:bg-[#08dd6e] disabled:cursor-not-allowed disabled:bg-[#8ce0af] focus-visible:outline-[2px] focus-visible:outline-[#07c05f] focus-visible:outline-offset-2" disabled={disabled || !draft.trim()} aria-label={actionLabel} title={`${actionLabel} · Enter`}>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M8 13V3" /><path d="M3.5 7.5L8 3l4.5 4.5" /></svg>
            </button>;
          })()}
        </div>
      </div>
    </div>
  </form>;
}
