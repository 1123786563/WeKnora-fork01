import { useEffect, useRef, useState, type ChangeEvent, type FormEvent, type KeyboardEvent } from 'react';
import { formatChatCopy, resolveChatCopy, resolveChatLocale, type ChatCopyTable } from './chat-copy.ts';
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

/*
 * R473-A2 — one chip per queued steer follow-up (Vue Input-field.vue
 * .steer-queue): the host owns the queue; the composer only renders it.
 * `pending` mirrors Vue item.pending (POST /steer in flight), `failed`
 * mirrors item.failed (retry affordance).
 */
export interface ChatSteerQueueChip {
  steerId: string;
  content: string;
  status: 'pending' | 'queued' | 'failed';
}

type ChatKeyboardEvent = Pick<KeyboardEvent, 'key' | 'keyCode' | 'shiftKey' | 'ctrlKey' | 'altKey' | 'metaKey'> & { isComposing?: boolean };

export function shouldSubmitFromKeyboard(event: ChatKeyboardEvent, canSteer: boolean): boolean {
  if (event.isComposing || event.keyCode === 229 || (event.key !== 'Enter' && event.keyCode !== 13)) return false;
  if (event.shiftKey || event.ctrlKey) return false;
  if (event.altKey && !event.metaKey && !canSteer) return false;
  return true;
}

/*
 * R474-A2 — Vue inject shortcut (frontend/src/utils/chatSubmitShortcut.ts):
 * ⌘Enter steers unconditionally; Alt+Enter only on a steer-capable turn.
 * Shift/Ctrl and IME composition never trigger it (newline / candidate keys).
 */
export function isSteerInjectShortcut(event: ChatKeyboardEvent, canSteer: boolean): boolean {
  if (!shouldSubmitFromKeyboard(event, canSteer)) return false;
  if (event.metaKey) return true;
  return event.altKey && canSteer;
}

/** Vue Input-field.vue steerShortcutLabel: ⌘ Enter on Apple platforms, Alt+Enter elsewhere. */
export function steerShortcutLabel(): string {
  const platform = typeof navigator === 'undefined' ? '' : navigator.platform ?? '';
  return /Mac|iPhone|iPad/.test(platform) ? '⌘ Enter' : 'Alt+Enter';
}

/*
 * R474-A2 — Vue Input-field.vue injectCurrentInput + firstQueuedSteer: the
 * ⌘Enter/Alt+Enter shortcut injects the current draft when it has content;
 * with an empty draft it promotes the first queued after-message (delivery
 * 'after', not pending/promoting/failed) via emit('promote-steer').
 */
export function resolveSteerInjectAction(input: {
  draft: string;
  steerQueue: readonly ChatSteerQueueChip[];
}): { kind: 'submit' } | { kind: 'promote'; steerId: string } | undefined {
  if (input.draft.trim()) return { kind: 'submit' };
  const first = input.steerQueue.find((item) => item.status === 'queued');
  return first ? { kind: 'promote', steerId: first.steerId } : undefined;
}

/*
 * R477-A2 — Vue Input-field.vue steer-path attachment gates, in Vue order:
 *   1. uploadedAttachments.some(item => item.status === 'uploading')
 *        → MessagePlugin.warning(input.messages.steerAttachmentPending)
 *   2. uploadedAttachments.length || uploadedImages.length
 *        → MessagePlugin.warning(input.messages.steerHasAttachments)
 * The React composer keeps one unified attachment list, so gate 2 is any
 * entry; gate 1 wins whenever an upload is still in flight. Returns the
 * copy key to toast (Vue carrier), or null when the steer may proceed.
 *
 * R478-A1 — pending mapping: Vue has no 'pending' attachment state.
 * AttachmentUpload.vue addFiles pushes
 * `status: props.sessionId ? 'uploading' : 'local'`, and a steer turn
 * always runs inside a session — a picked-but-not-yet-uploaded file is
 * 'uploading' from the very first tick (the HTTP request starts right
 * after). The React lazy upload parks that same "picked, upload not
 * started/finished, no attachmentId yet" window in 'pending', so by the
 * Vue contract it belongs to gate 1 (attachment not uploaded yet), never
 * to gate 2 (attachments on a running answer).
 */
export type SteerAttachmentWarning = 'steerAttachmentPending' | 'steerHasAttachments';

export function resolveSteerAttachmentWarning(attachments: readonly ChatAttachmentView[]): SteerAttachmentWarning | null {
  if (attachments.some((item) => item.status === 'uploading' || item.status === 'pending')) return 'steerAttachmentPending';
  if (attachments.length > 0) return 'steerHasAttachments';
  return null;
}

/*
 * R478-A1 — Vue Input-field.vue steer submit failure fallback (handleSteerMsg
 * catch): the server-provided Error message wins, the scenario copy
 * input.messages.steerFailed is the fallback — the send path's sendFailed
 * must never leak into the steer path. Extracted from SteerComposer.submit
 * (page.tsx) so tests pin the contract on behavior instead of a fixed-width
 * source window.
 */
export function resolveSteerSubmitFailure(copy: ChatCopyTable, cause: unknown): string {
  return cause instanceof Error && cause.message ? cause.message : copy.steerFailed;
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
  /** R490 B1 — Vue mentionEmptyHint: agent-compatibility empty state, shown
   *  instead of mentionNoAvailable when the filter emptied the list. */
  mentionEmptyHint?: string;
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
  selectedModelId?: string;
  onModelChange?(modelId: string): void;
  /** Vue control-right swaps send for stop while a reply is running (isReplying: dispatched through stream end, incl. the pre-stream window). */
  streaming?: boolean;
  /** Vue shows stop whenever the active session cannot accept a steer. */
  canSteer?: boolean;
  onStop?(): void;
  /** R473-A2 — queued steer follow-ups rendered as chips (Vue .steer-queue). */
  steerQueue?: readonly ChatSteerQueueChip[];
  /** Vue promote-steer: flip a queued after-message to inject (send now). */
  onSteerPromote?(steerId: string): void;
  /** Vue remove-steer: cancel a queued message (DELETE /steer/:id). */
  onSteerRemove?(steerId: string): void;
  /** Vue retry-steer: re-run the failed enqueue POST. */
  onSteerRetry?(steerId: string): void;
  /**
   * R484 D15 — Vue Input-field.vue:451-461 showWebSearchButton: the globe
   * toggle renders only while the host reports web-search readiness (tenant
   * default engine, or the selected agent's engine). The host owns the toggle
   * behaviour (Vue toggleWebSearch): toasts, the agent-disabled warning and
   * the not-configured prompt.
   */
  webSearchVisible?: boolean;
  /** Vue isWebSearchConfigured — false keeps the notConfigured title/look. */
  webSearchConfigured?: boolean;
  /** Vue settingsStore.isWebSearchEnabled — the persisted toggle state. */
  webSearchEnabled?: boolean;
  /** Vue toggleWebSearch click handler. */
  onWebSearchToggle?(): void;
  /** Resolved copy (chat-copy.ts); defaults to the app locale convention. */
  copy?: ChatCopyTable;
}

/*
 * Vue anatomy (frontend/src/components/Input-field.vue .rich-input-container):
 * rounded shell with the textarea on top and a control bar at the bottom —
 * left chips are the agent selector + attachment/@ buttons, right side holds
 * the model chip and the circular green send (or stop) button.
 */
export function ChatComposer({ draft, focusSignal = 0, disabled = false, onDraftChange, onSubmit, attachments = [], onAttachmentSelect, onRemoveAttachment, attachmentAccept, mentionOptions = [], mentionedItems = [], mentionOpen: initialMentionOpen = false, mentionLoading = false, mentionError, mentionEmptyHint, onMentionOpen, onMentionSelect, onMentionRemove, agents, selectedAgentId, onAgentChange, agentModels, onManageAgents, onConfigureAgent, onAgentNotReady, modelLabel, modelContext, modelContextIsDefault, modelOptions = [], selectedModelId, onModelChange, streaming = false, canSteer = false, onStop, steerQueue = [], onSteerPromote, onSteerRemove, onSteerRetry, webSearchVisible = false, webSearchConfigured = true, webSearchEnabled = false, onWebSearchToggle, copy }: ChatComposerProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const attachmentInputRef = useRef<HTMLInputElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);
  const draftRef = useRef<HTMLTextAreaElement>(null);
  // Vue Input-field.vue prefill consume: nextTick(() => textarea.focus()).
  useEffect(() => {
    if (focusSignal) draftRef.current?.focus();
  }, [focusSignal]);
  const [mentionOpen, setMentionOpen] = useState(initialMentionOpen);
  const [mentionQuery, setMentionQuery] = useState('');
  // Vue triggerMention（Input-field.vue:1640-1694）：弹层 fixed 定位锚定 textarea
  //（left=rect.left；上方优先，bottom=vh-rect.top+8，menuHeight 阈值 320）。
  const [mentionMenuStyle, setMentionMenuStyle] = useState<React.CSSProperties | null>(null);
  // Vue kb-btn t-tooltip（Input-field.vue:2756-2774，theme light / placement top）。
  const [kbTipOpen, setKbTipOpen] = useState(false);
  const kbTipTimer = useRef<number | null>(null);
  // Vue Input-field.vue:2737 附件按钮 t-tooltip（top/light，input-field-tooltip 覆盖）；
  // 与 kb-btn 同款 in-tree 手写 tooltip（300ms 显示 / 80ms 隐藏），文案带附件计数态。
  const [attachTipOpen, setAttachTipOpen] = useState(false);
  const attachTipTimer = useRef<number | null>(null);
  const [activeMentionIndex, setActiveMentionIndex] = useState(0);
  const [agentPanelOpen, setAgentPanelOpen] = useState(false);
  const agentChipRef = useRef<HTMLDivElement>(null);
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
    // R474-A2 — Vue injectCurrentInput: on a steer-capable running turn the
    // ⌘Enter/Alt+Enter shortcut injects the typed draft, or — with an empty
    // draft — promotes the first queued steer chip (firstQueuedSteer).
    if (streaming && canSteer && isSteerInjectShortcut(event, canSteer)) {
      const action = resolveSteerInjectAction({ draft, steerQueue });
      if (action?.kind === 'promote') {
        void promoteSteerChip(action.steerId);
        return;
      }
    }
    submitDraft();
  }

  /*
   * R474-A2 — Vue item.promoting: the chip promote action is disabled while
   * its POST /steer/:id/inject is in flight (double-click / shortcut guard on
   * top of the host-side idempotency check).
   */
  const [promotingSteerId, setPromotingSteerId] = useState<string | null>(null);
  function promoteSteerChip(steerId: string): void {
    const result = onSteerPromote?.(steerId);
    if (!result) return;
    setPromotingSteerId(steerId);
    void Promise.resolve(result).finally(() => setPromotingSteerId((current) => current === steerId ? null : current));
  }

  const showStop = streaming && (!canSteer || !draft.trim());
  function selectAttachments(event: ChangeEvent<HTMLInputElement>): void {
    const files = Array.from(event.target.files ?? []);
    event.target.value = '';
    for (const file of files) void onAttachmentSelect?.(file);
  }

  /*
   * R472-A2 (R470 reverse gap): Vue Input-field.vue renders a dedicated image
   * upload button only when the selected agent config has
   * image_upload_enabled === true (isImageUploadEnabledByAgent; quick-answer
   * has no config flag so the button stays hidden). On the authenticated web
   * client Vue image picks travel through the same temporary-attachment
   * transport as paperclip files (chat/index.vue: upload → attachment_ids),
   * so React reuses the onAttachmentSelect pipeline and only adds the gated
   * affordance; the chip strip doubles as the Vue image-preview bar.
   */
  const imageUploadEnabled = agents?.find((agent) => agent.id === selectedAgentId)?.config?.image_upload_enabled === true;
  const imageAttachmentCount = attachments.filter((attachment) => /\.(jpe?g|png|gif|webp|bmp|tiff)$/i.test(attachment.name)).length;

  const filteredMentionOptions = mentionOptions.filter((item) => item.name.toLocaleLowerCase().includes(mentionQuery.trim().toLocaleLowerCase()) && !mentionedItems.some((selected) => selected.id === item.id));
  function toggleMentions(): void {
    if (disabled) return;
    const next = !mentionOpen;
    setMentionOpen(next);
    if (next) {
      setMentionQuery('');
      setActiveMentionIndex(0);
      onMentionOpen?.();
      // Vue triggerMention：锚定 textarea 左缘，优先上方（8px 间距）。
      const textarea = draftRef.current;
      if (textarea) {
        const rect = textarea.getBoundingClientRect();
        const vh = window.innerHeight;
        const menuHeight = 320;
        const spaceAbove = rect.top;
        const spaceBelow = vh - rect.bottom;
        if (spaceAbove > menuHeight || spaceAbove > spaceBelow) {
          setMentionMenuStyle({ position: 'fixed', left: `${rect.left}px`, bottom: `${vh - rect.top + 8}px`, top: 'auto' });
        } else {
          setMentionMenuStyle({ position: 'fixed', left: `${rect.left}px`, top: `${rect.bottom + 8}px`, bottom: 'auto' });
        }
      }
      textarea?.focus();
    }
  }
  function closeMentions(): void {
    setMentionOpen(false);
    setMentionQuery('');
  }
  function handleMentionKeyDown(event: React.KeyboardEvent<HTMLElement>): void {
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

  return <form className="wk-chat-composer wk-vc-composer-1" onSubmit={submit}>
    {/* The textarea's own placeholder attribute carries the visible hint (Vue
        parity); the label is aria-only so the hidden text stays out of
        innerText — a clip-hidden text node still leaks into it. */}
    <label className="wk-chat-visually-hidden" htmlFor="wk-chat-draft" aria-hidden="true" style={{ display: 'none' }}>{t.composerPlaceholder}</label>
    <div data-guide="chat-input" className="wk-chat-input-shell wk-ease-ease wk-vc-composer-2">
      {/* R473-A2 — Vue Input-field.vue .steer-queue (~2599): one chip per queued
          after-message at the very top of the input shell. Waiting clock icon +
          truncated text (full text via title) + per-state actions; pending
          shows the spinner and hides actions, failed swaps them for retry. */}
      {steerQueue.length > 0 ? <ul className="wk-chat-steer-queue wk-vc-composer-3" role="list" aria-label={t.steerQueueWaiting}>
        {steerQueue.map((item, index) => {
          // R474-A2 — Vue Input-field.vue ~2610: only the first promotable chip
          // advertises the ⌘Enter/Alt+Enter shortcut in its send-now tooltip.
          const shortcutSuffix = index === steerQueue.findIndex((candidate) => candidate.status === 'queued')
            ? ` · ${steerShortcutLabel()}`
            : '';
          return <li key={item.steerId} role="listitem" data-steer-id={item.steerId} data-steer-status={item.status} data-steer-promoting={promotingSteerId === item.steerId ? 'true' : undefined} className="wk-chat-steer-queue-item wk-vc-composer-4">
          <svg className="wk-vc-composer-5" width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" aria-hidden="true"><circle cx="8" cy="8" r="6.2" /><path d="M8 4.8V8l2.2 1.6" strokeLinecap="round" /></svg>
          <span className="wk-vc-composer-6" title={item.content}>{item.content}</span>
          {item.status === 'failed' ? (onSteerRetry ? <button type="button" className="wk-vc-composer-7" aria-label={t.steerRetry} title={t.steerRetry} onClick={() => onSteerRetry(item.steerId)}>
            <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><path d="M13 8a5 5 0 1 1-1.5-3.5" strokeLinecap="round" /><path d="M13 2v3h-3" strokeLinecap="round" strokeLinejoin="round" /></svg>
          </button> : null) : item.status === 'pending' ? <span className="wk-chat-steer-sending wk-vc-composer-8" role="img" aria-label={t.loadingMessages}>…</span> : <>
            {onSteerPromote ? <button type="button" disabled={promotingSteerId === item.steerId} className="wk-vc-composer-9" aria-label={t.steerQueueSendNow} title={`${t.steerQueueSendNow}${shortcutSuffix}`} onClick={() => promoteSteerChip(item.steerId)}>
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M8 13V3" /><path d="M3.5 7.5L8 3l4.5 4.5" /></svg>
            </button> : null}
            {onSteerRemove ? <button type="button" className="wk-vc-composer-7" aria-label={t.remove} title={t.remove} onClick={() => onSteerRemove(item.steerId)}>×</button> : null}
          </>}
        </li>;
        })}
      </ul> : null}
      {attachments.length > 0 ? <ul className="wk-chat-attachments wk-vc-composer-10" aria-label={t.uploadAttachment}>
        {attachments.map((attachment) => <li key={attachment.id} data-attachment-status={attachment.status} className="wk-vc-composer-4" title={attachment.error || attachment.status}>
          <span className="wk-vc-composer-6">{attachment.name}</span>
          <span aria-label={attachmentStatusLabel(attachment)}>{attachment.status === 'ready' ? '✓' : attachment.status === 'failed' ? '!' : '…'} {attachmentStatusLabel(attachment)}</span>
          {onRemoveAttachment ? <button type="button" className="wk-vc-composer-7" aria-label={`${t.close}: ${attachment.name}`} onClick={() => void onRemoveAttachment(attachment.id)}>×</button> : null}
        </li>)}
      </ul> : null}
      {mentionedItems.length > 0 ? <ul className="wk-chat-mentions wk-vc-composer-10" aria-label={t.mentionKnowledge}>
        {mentionedItems.map((item) => <li key={item.id} data-mention-id={item.id} data-mention-type={item.type} className="wk-vc-composer-11">
          <span aria-hidden="true">{mentionMarker(item.type)}</span><span className="wk-vc-composer-6">{item.name}</span>
          {onMentionRemove ? <button type="button" className="wk-vc-composer-7" aria-label={`${t.close}: ${item.name}`} onClick={() => onMentionRemove(item.id)}>×</button> : null}
        </li>)}
      </ul> : null}
      <textarea
        id="wk-chat-draft"
        ref={draftRef}
        value={draft}
        onChange={(event) => onDraftChange(event.target.value)}
        onKeyDown={(event) => { if (mentionOpen) handleMentionKeyDown(event); handleDraftKeyDown(event); }}
        disabled={disabled}
        rows={2}
        placeholder={t.composerPlaceholder}
        className="wk-vc-composer-12"
      />
      {/* Vue .answers-input control row: 9px under the textarea, 13px to the
          shell bottom edge — the 8/12 pair left the whole composer 10px low
          and 2px short (y576 h124 on the Vue side). With the 30px agent chip
          the row is 38px tall in Vue (8px top pad) and the shell closes with
          12px + border, keeping the shell y576 h124 exactly. */}
      <div className="wk-chat-control-bar wk-vc-composer-13">
        <div className="wk-chat-control-left wk-vc-composer-14">
          {agents && onAgentChange ? (() => {
            const currentAgent = agents.find((agent) => agent.id === selectedAgentId);
            const chipLabel = currentAgent?.name ?? t.quickAnswer;
            const panelOpen = agentPanelOpen && typeof document !== 'undefined' && agentChipRef.current;
            return <>
              {/* Vue .control-btn.agent-mode-btn 是 div（无 tabindex/焦点）——点击不
                  落焦点，shell 不触发 :focus-within 品牌色描边。React 保持 div 同构
                  （button 会让 composer 外框出现 Vue 没有的绿边，px-chat-agent-selector
                  实证）。 */}
              <div
                ref={agentChipRef}
                id="wk-chat-agent"
                role="button"
                aria-label={t.selectAgent}
                aria-haspopup="dialog"
                aria-expanded={agentPanelOpen}
                aria-disabled={disabled || undefined}
                onClick={() => { if (!disabled) setAgentPanelOpen((open) => !open); }}
                className="wk-chat-agent-chip wk-vc-composer-15"
              >
                {/* Vue .agent-mode-text: margin 0 4px, on top of the control-btn flex gap 4 */}
                <span className="wk-vc-composer-16">{chipLabel}</span>
                <svg className="wk-vc-composer-17" width="10" height="10" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" /></svg>
              </div>
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
          {/* R484 D15 — Vue Input-field.vue:2690-2719: the web-search globe
              toggle sits between the agent chip and the image/attachment
              buttons, only while the host reports readiness. The active state
              (enabled && configured) mirrors the Vue .websearch-btn.active
              binding; the host toggle owns toasts and warnings. */}
          {webSearchVisible && onWebSearchToggle ? (() => {
            const configured = webSearchConfigured !== false;
            const active = webSearchEnabled === true && configured;
            const title = !configured ? t.webSearchNotConfigured : webSearchEnabled ? t.webSearchToggleOff : t.webSearchToggleOn;
            return <button
              type="button"
              data-web-search-toggle
              data-active={active ? 'true' : undefined}
              data-configured={configured ? undefined : 'false'}
              className="wk-chat-websearch-btn wk-vc-composer-18"
              aria-label={title}
              title={title}
              disabled={disabled}
              onClick={() => onWebSearchToggle()}
            >
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true" className={active ? 'wk-vc-composer-34' : 'wk-vc-composer-35'}>
                <circle cx="9" cy="9" r="7" stroke="currentColor" strokeWidth="1.2" fill="none" />
                <path d="M 9 2 A 3.5 7 0 0 0 9 16" stroke="currentColor" strokeWidth="1.2" fill="none" />
                <path d="M 9 2 A 3.5 7 0 0 1 9 16" stroke="currentColor" strokeWidth="1.2" fill="none" />
                <line x1="2.94" y1="5.5" x2="15.06" y2="5.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
                <line x1="2.94" y1="12.5" x2="15.06" y2="12.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
              </svg>
            </button>;
          })() : null}
          {imageUploadEnabled && onAttachmentSelect ? <>
            {/* Vue Input-field.vue ~2596: hidden image input accepts the four multimodal MIME types, multiple picks. */}
            <input ref={imageInputRef} type="file" accept="image/jpeg,image/png,image/gif,image/webp" multiple className="wk-vc-composer-19" tabIndex={-1} aria-hidden="true" onChange={selectAttachments} />
            <button type="button" data-image-count={imageAttachmentCount > 0 ? String(imageAttachmentCount) : undefined} data-active={imageAttachmentCount > 0 ? 'true' : undefined} className="wk-chat-control-icon wk-chat-image-upload-btn wk-vc-composer-20" aria-label={t.uploadImage} disabled={disabled} title={t.uploadImage} onClick={() => imageInputRef.current?.click()}>
              <svg width="18" height="18" viewBox="0 0 1024 1024" fill="currentColor" aria-hidden="true">
                <path d="M896 128H128c-35.3 0-64 28.7-64 64v640c0 35.3 28.7 64 64 64h768c35.3 0 64-28.7 64-64V192c0-35.3-28.7-64-64-64zM128 832V192h768l0.1 640H128z" />
                <path d="M352 448a96 96 0 1 0 0-192 96 96 0 0 0 0 192z" />
                <path d="M128 768l224-288 160 160 192-256L896 640v128H128z" />
              </svg>
              {imageAttachmentCount > 0 ? <span className="wk-chat-image-count wk-vc-composer-21">{imageAttachmentCount}</span> : null}
            </button>
          </> : null}
          <input ref={attachmentInputRef} type="file" accept={attachmentAccept?.join(',')} multiple className="wk-vc-composer-19" tabIndex={-1} aria-hidden="true" onChange={selectAttachments} />
          <div className="wk-chat-attach-btn-wrap wk-vc-composer-23"
            onMouseEnter={() => { if (attachTipTimer.current !== null) window.clearTimeout(attachTipTimer.current); attachTipTimer.current = window.setTimeout(() => setAttachTipOpen(true), 300); }}
            onMouseLeave={() => { if (attachTipTimer.current !== null) window.clearTimeout(attachTipTimer.current); attachTipTimer.current = window.setTimeout(() => setAttachTipOpen(false), 80); }}
          >
            <button type="button" className="wk-chat-control-icon wk-vc-composer-22" aria-label={t.uploadAttachment} disabled={disabled || !onAttachmentSelect} title={t.uploadAttachment} onClick={() => attachmentInputRef.current?.click()}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48" />
              </svg>
            </button>
            {attachTipOpen ? (
              <span className="t-popup t-tooltip t-tooltip--light wk-attach-tip" role="tooltip">
                <span className="t-popup__content">{attachments.length > 0 ? formatChatCopy(t, 'attachmentWithCount', { count: attachments.length }) : t.attachmentUploadTooltip}</span>
                <span className="wk-kb-tip__arrow" aria-hidden="true" />
              </span>
            ) : null}
          </div>
          <div className="wk-chat-kb-btn-wrap wk-vc-composer-23"
            onMouseEnter={() => { if (kbTipTimer.current !== null) window.clearTimeout(kbTipTimer.current); kbTipTimer.current = window.setTimeout(() => setKbTipOpen(true), 300); }}
            onMouseLeave={() => { if (kbTipTimer.current !== null) window.clearTimeout(kbTipTimer.current); kbTipTimer.current = window.setTimeout(() => setKbTipOpen(false), 80); }}
          >
          <button type="button" data-guide="chat-kb-mention" className="wk-chat-control-icon wk-vc-composer-24" aria-label={t.mentionKnowledge} aria-expanded={mentionOpen} aria-controls="wk-chat-mention-listbox" disabled={disabled} onClick={toggleMentions}>
            <svg width="18" height="18" viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <circle cx="10" cy="10" r="3.5" stroke="currentColor" strokeWidth="1.8" />
              <path d="M13.5 10V11.5C13.5 12.163 13.7634 12.7989 14.2322 13.2678C14.7011 13.7366 15.337 14 16 14C16.663 14 17.2989 13.7366 17.7678 13.2678C18.2366 12.7989 18.5 12.163 18.5 11.5V10C18.5 7.74566 17.6045 5.58365 16.0104 3.98959C14.4163 2.39553 12.2543 1.5 10 1.5C7.74566 1.5 5.58365 2.39553 3.98959 3.98959C2.39553 5.58365 1.5 7.74566 1.5 10C1.5 12.2543 2.39553 14.4163 3.98959 16.0104C5.58365 17.6045 7.74566 18.5 10 18.5H12" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
          {kbTipOpen ? (
            <span className="t-popup t-tooltip t-tooltip--light wk-kb-tip" role="tooltip">
              <span className="t-popup__content">{t.mentionKnowledge}</span>
              <span className="wk-kb-tip__arrow" aria-hidden="true" />
            </span>
          ) : null}
          {/* Vue MentionSelector.vue（Teleport body）：views 包无 react-dom（agent-selector
              同款 in-tree 先例），以 position:fixed 视口锚定达到同一几何/层叠。 */}
          {mentionOpen ? (
            <div id="wk-chat-mention-listbox" role="listbox" aria-label={t.mentionKnowledge} className="mention-menu" style={mentionMenuStyle ?? undefined} onClick={(event) => event.stopPropagation()}>
              <div className="mention-list">
                {mentionLoading ? (
                  <div className="empty" role="status">{t.loadingMessages}</div>
                ) : mentionError ? (
                  <div className="empty" role="alert">{mentionError}</div>
                ) : filteredMentionOptions.length > 0 ? (
                  <div className="mention-group" data-group-type="kb" id="wk-chat-mention-options">
                    {filteredMentionOptions.map((item, index) => (
                      <div
                        key={item.id}
                        id={`wk-chat-mention-option-${item.id}`}
                        role="option"
                        aria-selected={index === activeMentionIndex}
                        data-mention-id={item.id}
                        data-mention-type={item.type}
                        className={'mention-item wk-chat-mention-item' + (index === activeMentionIndex ? ' active' : '')}
                        onMouseEnter={() => setActiveMentionIndex(index)}
                        onClick={() => { onMentionSelect?.(item); closeMentions(); }}
                      >
                        <div className="icon-wrap">
                          <div className={'icon ' + (item.type === 'kb' ? (item.kbType === 'faq' ? 'faq-icon' : 'kb-icon') : `${item.type}-icon`)}>
                            <svg className="t-icon" viewBox="0 0 24 24" width="1em" height="1em" fill="none" aria-hidden="true"><use href={'#t-icon-' + (item.type === 'kb' ? (item.kbType === 'faq' ? 'chat-bubble-help' : 'folder') : item.type === 'file' ? 'file' : 'tools')} /></svg>
                          </div>
                        </div>
                        <div className="item-main">
                          <span className="name">{item.name}</span>
                          {item.type === 'kb' || item.type === 'mcp' ? <span className="count">{item.toolCount ?? 0}</span> : null}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="empty">{mentionEmptyHint || t.noResult}</div>
                )}
              </div>
            </div>
          ) : null}
          </div>
          {/* Vue Input-field.vue:2787-2795 — the model chip lives at the right
              edge of control-left (.model-display margin-left:auto), NOT in
              control-right; keeping it there shifts it ~8px left to x≈1022. */}
          <div className="wk-chat-model-display wk-vc-composer-25">
            {modelOptions.length > 0 && onModelChange ? <label className="wk-chat-model-chip wk-vc-composer-26"><span className="wk-vc-composer-27" aria-hidden="true">{t.modelChip}</span><span className="wk-chat-model-name wk-vc-composer-28">{selectedModelId ? (modelOptions.find((model) => model.id === selectedModelId)?.name ?? modelOptions[0]?.name ?? '') : (modelOptions[0]?.name ?? '')}</span><select aria-label={t.modelChip} value={selectedModelId ?? modelOptions[0]?.id ?? ''} onChange={(event) => onModelChange(event.target.value)} className="wk-vc-composer-29">{modelOptions.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}</select>{modelContext ? <span className={modelContextIsDefault ? 'wk-chat-model-ctx is-default wk-vc-composer-36' : 'wk-chat-model-ctx wk-vc-composer-37'}>{modelContext}</span> : null}<svg className="wk-chat-chip-arrow wk-vc-composer-30" width="10" height="10" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" /></svg></label> : <button type="button" className="wk-chat-model-chip wk-vc-composer-31" disabled aria-disabled="true" aria-label={modelLabel ?? t.modelChip} title={modelLabel ?? t.modelChip}>
              <span className="wk-chat-model-name wk-vc-composer-28">{modelLabel ?? t.modelChip}</span>
              {modelContext ? <span className={modelContextIsDefault ? 'wk-chat-model-ctx is-default wk-vc-composer-38' : 'wk-chat-model-ctx wk-vc-composer-39'}>{modelContext}</span> : null}
              <svg className="wk-chat-chip-arrow wk-vc-composer-30" width="10" height="10" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M2.5 4.5L6 8L9.5 4.5H2.5Z" /></svg>
            </button>}
            {modelPanelOpen && modelTriggerRef.current && onModelChange ? <ModelSelectorPanel
              copy={t}
              options={modelOptions}
              selectedModelId={selectedModelId}
              anchorRect={modelTriggerRef.current.getBoundingClientRect()}
              onSelect={(modelId) => { setModelPanelOpen(false); onModelChange(modelId); }}
              onAddModel={() => { setModelPanelOpen(false); onModelAdd?.(); }}
              onClose={() => setModelPanelOpen(false)}
            /> : null}
          </div>
        </div>
        <div className="wk-chat-control-right wk-vc-composer-32">
          {showStop && onStop ?<button type="button" className="wk-chat-stop wk-chat-send wk-ease-ease wk-outline-2 wk-vc-composer-33" aria-label={t.stopGeneration} title={t.stopGeneration} onClick={onStop}>
            <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor" aria-hidden="true"><rect x="2.5" y="2.5" width="9" height="9" rx="1.5" /></svg>
          </button> : (() => {
            // Vue steer-mode labelling: while replying on a steer-capable turn
            // the same circular button queues the follow-up (input.steerAfter),
            // so both the tooltip and aria-label swap away from input.send.
            const steerMode = streaming && canSteer;
            const actionLabel = steerMode ? t.steerQueued : t.send;
            return <button type="submit" data-guide="chat-send" className="wk-chat-send wk-ease-ease wk-outline-2 wk-vc-composer-33" disabled={disabled || !draft.trim()} aria-label={actionLabel} title={`${actionLabel} · Enter`}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M17.5 10.5 12 5l-5.5 5.5M12 6.25v13" stroke="currentColor" strokeWidth="2" strokeLinecap="square" /></svg>
            </button>;
          })()}
        </div>
      </div>
    </div>
  </form>;
}
