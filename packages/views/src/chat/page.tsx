import { useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { ChatMessage, ChatSession, FeedbackRating, MessageSuggestionSet } from '@weknora/contracts';
import { shouldShowTypingIndicator } from '@weknora/domain/chat/session-state';
import { ChatComposer, isSteerInjectShortcut, resolveSteerAttachmentWarning, resolveSteerInjectAction, resolveSteerSubmitFailure, shouldSubmitFromKeyboard, type ChatAttachmentView, type ChatMentionView, type ChatSteerQueueChip, type ChatSubmission } from './composer.tsx';
import { MessageList, TOOL_LIST_ITEM, type PendingChatMessage } from './message-list.tsx';
import { SessionSidebar } from './session-sidebar.tsx';
import { ReferenceList } from './reference-list.tsx';
import { ToolResultView } from './tool-result.tsx';
import { ToolApprovalCard } from './tool-approval.tsx';
import type { ArtifactPreviewPayload } from './artifact-preview.tsx';
import { splitLiveThinking, type LiveThinkingState } from './live-thinking.ts';
import { resolveChatCopy, resolveChatLocale, type ChatCopyTable } from './chat-copy.ts';

// Re-exported for route hosts on the mapped ./chat/page subpath: the
// ChatRoutePage transient row strips `<think>` content with the same Vue
// processStreamChunk split that drives the live deepThink block.
export { splitLiveThinking };

function mentionMarker(type: ChatMentionView['type']): string {
  switch (type) {
    case 'file': return '▧';
    case 'tag': return '#';
    case 'mcp': return '⚒';
    case 'skill': return '✦';
    default: return '@';
  }
}

export interface ChatAgentOption {
  id: string;
  name: string;
  disabled?: boolean;
  description?: string;
  is_builtin?: boolean;
  config?: Record<string, unknown>;
}

export interface ChatToolApprovalPrompt {
  pendingId: string;
  toolName?: string;
  status: 'pending' | 'resolved';
  decision?: string;
  /** Original tool call arguments, rendered editable in the approval card. */
  arguments?: Record<string, unknown>;
  /** Unix seconds the approval was requested (SSE tool_approval_required). */
  requestedAt?: number;
  /** Approval timeout in seconds; Vue ToolApprovalCard defaults to 600. */
  timeoutSeconds?: number;
}

export interface ChatOAuthApprovalPrompt {
  pendingId: string;
  serviceId?: string;
  serviceName?: string;
  toolName?: string;
  status: 'pending' | 'resolved';
  authorized?: boolean;
  reason?: string;
}

export interface ChatToolCallView {
  id: string;
  name?: string;
  status: 'pending' | 'completed' | 'failed';
  result?: unknown;
}

export interface ChatStreamPresentation {
  phase: 'idle' | 'streaming' | 'completed' | 'stopped' | 'error';
  thinking: string;
  /**
   * Raw accumulated answer content. Vue's main face drives its deepThink
   * streaming indicator from `<think>` tags in this buffer, not from the SSE
   * `thinking` field (agent timeline only).
   */
  answer?: string;
  references: readonly unknown[];
  toolCalls: readonly ChatToolCallView[];
  artifactsPending?: boolean;
}

export interface ChatTerminalView {
  status: string;
  output: string;
}

export interface ChatPageProps {
  sessions: readonly ChatSession[];
  selectedSessionId: string | null;
  messages: readonly ChatMessage[];
  draft: string;
  /**
   * R466-A2 — Vue prefillQuery focus pulse: bump the number (e.g. 0 → 1) to
   * focus the composer textarea (Input-field.vue nextTick(textarea.focus)
   * after consumePrefillQuery fills the draft). 0 keeps the default blur.
   */
  composerFocusSignal?: number;
  /** UI locale (chat-copy.ts tables); defaults to the app locale convention. */
  locale?: string;
  loadingSessions?: boolean;
  loadingMessages?: boolean;
  error?: string;
  onSelectSession(sessionId: string): void;
  onCreateSession(): void;
  onDraftChange(value: string): void;
  send(submission: ChatSubmission): Promise<void>;
  attachments?: readonly ChatAttachmentView[];
  onAttachmentSelect?(file: File): void | Promise<void>;
  onRemoveAttachment?(id: string): void | Promise<void>;
  attachmentAccept?: readonly string[];
  mentionOptions?: readonly ChatMentionView[];
  mentionedItems?: readonly ChatMentionView[];
  mentionLoading?: boolean;
  mentionError?: string;
  /** R490 B1 — Vue mentionEmptyHint: agent-compatibility empty state of the
   *  @ popup, shown instead of the generic empty label. */
  mentionEmptyHint?: string;
  onMentionOpen?(): void;
  onMentionSelect?(item: ChatMentionView): void;
  onMentionRemove?(id: string): void;
  agents?: readonly ChatAgentOption[];
  selectedAgentId?: string;
  onAgentChange?(agentId: string): void;
  /** Chat-readiness models + agent-selector host actions (upstream AgentSelector). */
  agentModels?: readonly { id: string; type?: string }[];
  onManageAgents?(): void;
  onConfigureAgent?(agent: { id: string }, section: string, highlight?: 'summary_model' | 'rerank_model'): void;
  onAgentNotReady?(agent: { id: string; name: string }, labels: string[]): void;
  /** Empty-state suggested questions for the new-conversation view. */
  starterQuestions?: readonly string[];
  /** True while the agent suggested-questions request is in flight (skeleton chips). */
  starterQuestionsLoading?: boolean;
  /** Vue creatChat refresh action for the current starter-question set. */
  onRefreshStarterQuestions?(): void;
  onStarterQuestionClick?(question: string): void;
  toolApprovals?: readonly ChatToolApprovalPrompt[];
  oauthApprovals?: readonly ChatOAuthApprovalPrompt[];
  onResolveToolApproval?(pendingId: string, decision: 'approve' | 'reject', modifiedArgs?: Record<string, unknown>, reason?: string): Promise<void>;
  onAuthorizeOAuth?(pendingId: string, serviceId: string): Promise<void>;
  onCancelOAuth?(pendingId: string): Promise<void>;
  /**
   * Steer dispatch. R474-A2: the optional delivery mirrors Vue
   * handleSteerMsg(query, mentions, delivery) — 'inject' (⌘Enter/Alt+Enter
   * shortcut) surfaces the message in the running turn immediately, 'after'
   * (default, plain Enter) queues it as a follow-up.
   */
  onSteer?(content: string, mentionedItems?: readonly ChatMentionView[], delivery?: 'after' | 'inject'): Promise<void>;
  /**
   * R477-A2 — toast channel for the two steer attachment warnings
   * (steerAttachmentPending / steerHasAttachments). Vue carries both via
   * MessagePlugin.warning; hosts with a toast surface pass it through.
   */
  onSteerWarning?(message: string): void;
  /** R473-A2 — queued steer chips shown by the composer (Vue .steer-queue). */
  steerQueue?: readonly ChatSteerQueueChip[];
  /** Vue promote-steer (inject a queued after-message now). */
  onSteerPromote?(steerId: string): void | Promise<void>;
  /** Vue remove-steer (cancel one queued message). */
  onSteerRemove?(steerId: string): void | Promise<void>;
  /** Vue retry-steer (re-run a failed enqueue). */
  onSteerRetry?(steerId: string): void | Promise<void>;
  /**
   * R471-A1 — Vue canSteer parity (chat/index.vue :canSteer="isAgentStreamSession()"):
   * only an agent-pipeline session has a loop that accepts a mid-run message.
   * The presence of onSteer used to stand in for this, but hosts wire a steer
   * handler unconditionally (its idle branch falls back to a plain send), which
   * advertised steer capability on quick-answer turns and let the stop button
   * (`streaming && (!canSteer || !draft)`) never win with a non-empty draft.
   * Hosts pass the real per-session capability; the Boolean(onSteer) fallback
   * preserves hosts that never differentiate.
   */
  canSteer?: boolean;
  onStopStream?(): void;
  stream?: ChatStreamPresentation;
  onRenameSession?(sessionId: string, title?: string): Promise<void>;
  onToggleSessionPin?(sessionId: string, pinned: boolean): Promise<void>;
  onDeleteSession?(sessionId: string): Promise<void>;
  /**
   * SP13 Task 8 — 侧栏 ⋯ 菜单「分享」入口（能力开关：缺省即隐藏）。宿主负责
   * mint 分享 token（client.queryHistory.share）并渲染分享窗。
   */
  onShareSession?(sessionId: string): void;
  sessionGroups?: readonly { key: string; label?: string; items: readonly ChatSession[] }[];
  sessionSource?: string;
  sessionSourceOptions?: readonly { value: string; label: string }[];
  onSessionSourceChange?(source: string): void;
  sessionGroupMode?: 'none' | 'date';
  onSessionGroupModeChange?(mode: 'none' | 'date'): void;
  sessionKeyword?: string;
  onSessionKeywordChange?(keyword: string): void;
  sessionPage?: number;
  sessionPageCount?: number;
  onSessionPageChange?(page: number): void;
  onClearSession?(): Promise<void>;
  loadingOlderMessages?: boolean;
  hasMoreMessages?: boolean;
  onLoadOlderMessages?(): void;
  suggestions?: MessageSuggestionSet;
  onSuggestionClick?(questionId: string, text: string): void;
  onRefreshSuggestions?(): void;
  onDismissSuggestions?(): void;
  onCitationClick?(citationId: string): void;
  /** Host-owned Vue botmsg knowledge-base action; absent means unavailable. */
  onBookmark?(messageId: string): void | Promise<void>;
  /** SP11 message feedback (like/dislike on assistant bubbles); absent hides the pair. */
  onRateMessage?(messageId: string, rating: FeedbackRating): void;
  /** SP11 toggle-off: clears the persisted rating for a message. */
  onRemoveRating?(messageId: string): void;
  /** SP11 current rating lookup for the pressed (aria-pressed) state. */
  ratingOf?(messageId: string): FeedbackRating | undefined;
  /** Vue usermsg/botmsg 分叉 entry (A11 phase 4). */
  onForkMessage?(messageId: string): void;
  canForkMessage?(messageId: string): boolean;
  onArtifactDownload?(messageId: string, artifactIndex: number): Promise<void>;
  onArtifactPreview?(messageId: string, artifactIndex: number): Promise<ArtifactPreviewPayload>;
  terminal?: ChatTerminalView;
  /** Initial sandbox drawer state (Vue keeps the panel closed until toggled). */
  terminalOpen?: boolean;
  onOpenTerminal?(): Promise<void>;
  onTerminalInput?(input: string): Promise<void>;
  onTerminalResize?(cols: number, rows: number): void;
  onCloseTerminal?(): void;
  /** Display-only chat model chip label in the composer control bar. */
  modelLabel?: string;
  /** Context spec rendered as the Vue .model-selector-ctx suffix span. */
  modelContext?: string;
  /** True when the context spec is the 200K default (dims the suffix). */
  modelContextIsDefault?: boolean;
  modelOptions?: readonly { id: string; name: string }[];
  selectedModelId?: string;
  onModelChange?(modelId: string): void;
  /**
   * R484 D15 — Vue Input-field.vue web-search toggle, forwarded to the
   * composer globe button. Visibility gates on readiness (tenant default
   * engine, or the selected agent's engine); the host owns the toggle.
   */
  webSearchVisible?: boolean;
  webSearchConfigured?: boolean;
  webSearchEnabled?: boolean;
  onWebSearchToggle?(): void;
  /**
   * R483 D16 — Vue ChatHeader utility block rendered between 修改标题 and
   * 清空消息 (ChatHeader.vue:61-77): copy session id / copy link / copy as
   * Markdown / open in new window, framed by the two Vue menu dividers.
   * Hosts own the actions (clipboard, window.open, message paging); labels
   * arrive pre-localized so the shared copy table stays untouched.
   */
  headerUtilityItems?: readonly { id: string; label: string; onActivate(): void }[];
  /**
   * 会话视图头部件注入（Vue ChatHeader.vue 同构）：本包不带 tdesign 依赖，
   * 真实 t-popup 菜单由宿主（apps/web/src/chat/chat-header.tsx）以 ReactNode
   * 注入。缺省回退为结构面（.chat-header 标题 + ⋯ 按钮，无弹层）。
   */
  headerSlot?: ReactNode;
  /** Vue index.vue .sandbox-header-toggle 注入（宿主 t-tooltip 版）；缺省回退为结构面。 */
  sandboxToggleSlot?: ReactNode;
}

export function messageReferenceValues(messages: readonly ChatMessage[]): unknown[] {
  const references: unknown[] = [];
  for (const message of messages) {
    const row = message as Record<string, unknown>;
    for (const key of ['knowledge_references', 'references']) {
      const value = row[key];
      if (Array.isArray(value)) references.push(...value);
    }
  }
  return references;
}

/*
 * Vue deepThink.vue live indicator: while the streamed answer holds an open
 * `<think>` tag the header pulses with chat.thinking「思考中...」 and the
 * reasoning streams inline (forced open, answer held back); once the tag
 * closes the block auto-folds under chat.deepThoughtCompleted「已深度思考」.
 * The SSE `thinking` field itself never renders here — on the Vue main face
 * it only feeds the agent timeline surface.
 */
function LiveThinking({ copy, live }: { copy: ChatCopyTable; live: LiveThinkingState }) {
  if (!live.showThink) return null;
  if (live.thinking) {
    return <section className="wk-chat-live-think wk-vc-page-1" aria-label={copy.thinkingAlt}>
      <p role="status" className="wk-vc-page-2">
        <span className="wk-vc-page-3 wk-mr-none" aria-hidden="true" />
        {copy.thinking}
      </p>
      {live.thinkContent ? <p className="wk-vc-page-4">{live.thinkContent}</p> : null}
    </section>;
  }
  return <details className="wk-chat-live-think wk-vc-page-5">
    <summary className="wk-vc-page-6">{copy.deepThoughtCompleted}</summary>
    {live.thinkContent ? <p className="wk-vc-page-4">{live.thinkContent}</p> : null}
  </details>;
}

function LiveResponse({ copy, stream, onStopStream }: { copy: ChatCopyTable; stream: ChatStreamPresentation; onStopStream?: () => void }) {
  const live = splitLiveThinking(stream.answer ?? '');
  if (stream.phase !== 'streaming' && !stream.thinking && stream.toolCalls.length === 0) return null;
  return <section aria-label={copy.streamStatus} className="wk-chat-live-response wk-vc-page-7">
    <p role="status" className="wk-vc-page-8">{copy.streamStatus}: {stream.phase}</p>
    {stream.phase === 'streaming' && stream.artifactsPending ? <p role="status" className="wk-chat-artifacts-pending wk-vc-page-8">{copy.artifactsPending}</p> : null}
    {stream.phase === 'streaming' ? <LiveThinking copy={copy} live={live} /> : null}
    {stream.toolCalls.length > 0 ? <div><h2 className="wk-vc-page-9">{copy.toolCallsTitle}</h2><ul className="wk-list wk-vc-page-10">{stream.toolCalls.map((tool) => <li key={tool.id} className={TOOL_LIST_ITEM}><strong>{tool.name ?? tool.id}</strong><small className="wk-vc-page-11">{tool.status}</small>{tool.result === undefined ? null : <ToolResultView toolCall={tool} copy={copy} />}</li>)}</ul></div> : null}
  </section>;
}

function ChatActionCards(props: Pick<ChatPageProps, 'toolApprovals' | 'oauthApprovals' | 'onResolveToolApproval' | 'onAuthorizeOAuth' | 'onCancelOAuth'> & { copy: ChatCopyTable }) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const toolApprovals = props.toolApprovals ?? [];
  const oauthApprovals = props.oauthApprovals ?? [];
  if (toolApprovals.length === 0 && oauthApprovals.length === 0) return null;

  async function run(key: string, action: () => Promise<void>) {
    setBusy(key); setError(null);
    try { await action(); } catch (cause) { setError(cause instanceof Error ? cause.message : props.copy.sendFailed); } finally { setBusy(null); }
  }

  return <section aria-label={props.copy.chatActionsTitle} className="wk-chat-actions wk-vc-page-12">
    <h2 className="wk-vc-page-13">{props.copy.chatActionsTitle}</h2>
    {error ? <p role="alert">{error}</p> : null}
    {toolApprovals.map((approval) => <ToolApprovalCard
      key={`tool-${approval.pendingId}`}
      approval={approval}
      busy={busy !== null}
      onResolve={props.onResolveToolApproval
        ? (pendingId, decision, modifiedArgs, reason) => run(pendingId, () => props.onResolveToolApproval!(pendingId, decision, modifiedArgs, reason))
        : undefined}
      copy={props.copy}
    />)}
    {oauthApprovals.map((approval) => <div key={`oauth-${approval.pendingId}`} className="wk-chat-action-card wk-vc-page-14">
      <strong className="wk-vc-page-15">{props.copy.oauthTitle}: {approval.serviceName ?? approval.serviceId ?? props.copy.oauthTool}</strong>
      {approval.toolName ? <small className="wk-vc-page-11">{props.copy.oauthTool}: {approval.toolName}</small> : null}
      {approval.status === 'pending' && approval.serviceId && props.onAuthorizeOAuth && props.onCancelOAuth ? <div className="wk-list-actions wk-vc-page-16"><button type="button" className="wk-vc-page-17" disabled={busy !== null} onClick={() => void run(approval.pendingId, () => props.onAuthorizeOAuth!(approval.pendingId, approval.serviceId!))}>{props.copy.oauthAuthorize} {approval.serviceName ?? approval.serviceId}</button><button type="button" className="wk-vc-page-17" disabled={busy !== null} onClick={() => void run(approval.pendingId, () => props.onCancelOAuth!(approval.pendingId))}>{props.copy.oauthCancel}</button></div> : <small className="wk-vc-page-11">{approval.authorized ? props.copy.oauthAuthorized : approval.reason ?? props.copy.approvalResolved}</small>}
    </div>)}
  </section>;
}

function SteerComposer({ copy, onSteer, steerQueue = [], onSteerPromote, mentionOptions = [], mentionedItems = [], attachments = [], onSteerWarning, onMentionOpen, onMentionSelect, onMentionRemove }: {
  copy: ChatCopyTable;
  onSteer: (content: string, mentionedItems: readonly ChatMentionView[], delivery?: 'after' | 'inject') => Promise<void>;
  /** R474-A2 — the ⌘Enter/Alt+Enter inject shortcut promotes the first queued chip when the steer draft is empty (Vue injectCurrentInput). */
  steerQueue?: readonly ChatSteerQueueChip[];
  onSteerPromote?(steerId: string): void | Promise<void>;
  mentionOptions?: readonly ChatMentionView[];
  mentionedItems?: readonly ChatMentionView[];
  attachments?: readonly ChatAttachmentView[];
  /**
   * R477-A2 — the two steer attachment warnings travel as toasts in Vue
   * (Input-field.vue MessagePlugin.warning). Hosts that own a toast channel
   * pass it here; without one the message degrades to the inline alert.
   */
  onSteerWarning?(message: string): void;
  onMentionOpen?(): void;
  onMentionSelect?(item: ChatMentionView): void;
  onMentionRemove?(id: string): void;
}) {
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [mentionOpen, setMentionOpen] = useState(false);
  const [mentionQuery, setMentionQuery] = useState('');
  const [activeMentionIndex, setActiveMentionIndex] = useState(0);

  async function submit(event?: React.FormEvent<HTMLFormElement>, delivery: 'after' | 'inject' = 'after'): Promise<void> {
    event?.preventDefault();
    const content = draft.trim();
    if (!content) return;
    // R477-A2 — the two Vue steer attachment warnings gate the path (see
    // resolveSteerAttachmentWarning); hosts own the toast carrier.
    const attachmentWarning = resolveSteerAttachmentWarning(attachments);
    if (attachmentWarning) { const message = copy[attachmentWarning]; onSteerWarning ? onSteerWarning(message) : setError(message); return; }
    setBusy(true); setError(null);
    // R476-A2/R478-A1 — a rejected enqueue surfaces input.messages.steerFailed
    // (Vue handleSteerMsg catch toasts the scenario copy, not the send
    // fallback); the fallback contract lives in the exported predicate.
    try { await onSteer(content, mentionedItems, delivery); setDraft(''); } catch (cause) { setError(resolveSteerSubmitFailure(copy, cause)); } finally { setBusy(false); }
  }

  /*
   * R474-A2 — Vue Input-field.vue onKeydown + injectCurrentInput: plain Enter
   * queues the follow-up ('after'); ⌘Enter/Alt+Enter injects the typed draft
   * ('inject'), or — with an empty draft — promotes the first queued chip.
   */
  function handleDraftKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>): void {
    if (busy || !shouldSubmitFromKeyboard(event, true)) return;
    event.preventDefault();
    if (isSteerInjectShortcut(event, true)) {
      const action = resolveSteerInjectAction({ draft, steerQueue });
      if (action?.kind === 'promote') {
        void onSteerPromote?.(action.steerId);
        return;
      }
      void submit(undefined, 'inject');
      return;
    }
    void submit(undefined, 'after');
  }

  const availableMentions = mentionOptions.filter((item) => item.name.toLocaleLowerCase().includes(mentionQuery.trim().toLocaleLowerCase()) && !mentionedItems.some((selected) => selected.id === item.id));
  function openMentions(): void {
    if (busy) return;
    const next = !mentionOpen;
    setMentionOpen(next);
    setMentionQuery('');
    setActiveMentionIndex(0);
    if (next) onMentionOpen?.();
  }

  function closeMentions(): void {
    setMentionOpen(false);
    setMentionQuery('');
    setActiveMentionIndex(0);
  }

  function handleMentionKeyDown(event: React.KeyboardEvent<HTMLInputElement>): void {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeMentions();
      return;
    }
    if (event.key === 'ArrowDown' && availableMentions.length > 0) {
      event.preventDefault();
      setActiveMentionIndex((current) => Math.min(current + 1, availableMentions.length - 1));
      return;
    }
    if (event.key === 'ArrowUp' && availableMentions.length > 0) {
      event.preventDefault();
      setActiveMentionIndex((current) => Math.max(current - 1, 0));
      return;
    }
    if (event.key === 'Enter' && availableMentions.length > 0) {
      event.preventDefault();
      const item = availableMentions[Math.min(activeMentionIndex, availableMentions.length - 1)];
      if (item) {
        onMentionSelect?.(item);
        closeMentions();
      }
    }
  }

  return <form className="wk-chat-steer wk-vc-page-18" onSubmit={(event) => void submit(event)}>
    <label htmlFor="wk-chat-steer-draft" className="wk-vc-page-19">{copy.steerCurrent}</label>
    {mentionedItems.length > 0 ? <ul className="wk-vc-page-20" aria-label={copy.mentionKnowledge}>{mentionedItems.map((item) => <li key={item.id} data-mention-id={item.id} data-mention-type={item.type} className="wk-vc-page-21"><span aria-hidden="true">{mentionMarker(item.type)}</span><span>{item.name}</span><button type="button" aria-label={`${copy.close}: ${item.name}`} className="wk-vc-page-22" disabled={busy} onClick={() => onMentionRemove?.(item.id)}>×</button></li>)}</ul> : null}
    <textarea id="wk-chat-steer-draft" rows={2} value={draft} onChange={(event) => setDraft(event.target.value)} onKeyDown={handleDraftKeyDown} disabled={busy} className="wk-vc-page-23" />
    <div className="wk-vc-page-24"><button id="wk-chat-steer-mention" type="button" aria-label={copy.mentionKnowledge} aria-expanded={mentionOpen} disabled={busy} className="wk-vc-page-25" onClick={openMentions}>@</button>{mentionOpen ? <div role="listbox" aria-label={copy.mentionKnowledge} className="wk-vc-page-26"><input autoFocus value={mentionQuery} onChange={(event) => { setMentionQuery(event.target.value); setActiveMentionIndex(0); }} onKeyDown={handleMentionKeyDown} aria-label={copy.composerPlaceholder} aria-activedescendant={availableMentions.length > 0 ? `wk-chat-steer-mention-option-${availableMentions[Math.min(activeMentionIndex, availableMentions.length - 1)].id}` : undefined} aria-controls="wk-chat-steer-mention-options" placeholder={copy.composerPlaceholder} className="wk-vc-page-27" />{availableMentions.length > 0 ? <div id="wk-chat-steer-mention-options">{availableMentions.map((item, index) => <button key={item.id} id={`wk-chat-steer-mention-option-${item.id}`} type="button" role="option" aria-selected={index === activeMentionIndex} data-mention-id={item.id} data-mention-type={item.type} className="wk-vc-page-28" onClick={() => { onMentionSelect?.(item); closeMentions(); }}><span aria-hidden="true">{mentionMarker(item.type)}</span><span>{item.name}</span></button>)}</div> : <p className="wk-vc-page-29">{copy.mentionNoAvailable}</p>}</div> : null}</div>
    {error ? <p role="alert">{error}</p> : null}
    <button type="submit" disabled={busy || !draft.trim()} className="wk-vc-page-30">{copy.steerQueued}</button>
  </form>;
}

function TerminalPanel(props: { copy: ChatCopyTable } & Pick<ChatPageProps, 'terminal' | 'onOpenTerminal' | 'onTerminalInput' | 'onTerminalResize' | 'onCloseTerminal'>) {
  const copy = props.copy;
  const panelRef = useRef<HTMLElement>(null);
  const [input, setInput] = useState('');
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!props.terminal || !props.onTerminalResize || typeof ResizeObserver === 'undefined' || !panelRef.current) return;
    const notify = () => {
      const element = panelRef.current;
      if (!element) return;
      props.onTerminalResize!(Math.max(1, Math.floor(element.clientWidth / 8)), Math.max(1, Math.floor(element.clientHeight / 16)));
    };
    const observer = new ResizeObserver(notify);
    observer.observe(panelRef.current);
    notify();
    return () => observer.disconnect();
  }, [props.onTerminalResize, props.terminal?.status]);
  async function sendInput(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!input.trim() || !props.onTerminalInput) return;
    setBusy(true);
    try { await props.onTerminalInput(input); setInput(''); } finally { setBusy(false); }
  }
  /* .wk-chat-sandbox-drawer .wk-chat-terminal scoped values folded in: the
     terminal only ever renders inside the drawer (page.tsx below). */
  return <section ref={panelRef} aria-label={`${props.copy.sandboxPanelTitle} (Sandbox terminal)`} className="wk-chat-terminal wk-vc-page-31">
    <div className="wk-settings-panel-heading wk-vc-page-32"><span role="status">{props.terminal?.status ?? 'idle'}</span></div>
    {props.terminal?.output ? <pre className="wk-vc-page-33">{props.terminal.output}</pre> : null}
    {!props.terminal?.output && props.onOpenTerminal ? <p className="wk-chat-terminal-hint wk-vc-page-34">{copy.startTerminal}</p> : null}
    {props.onOpenTerminal && !props.terminal?.output ? <button type="button" className="wk-chat-terminal-open wk-vc-page-35" onClick={() => void props.onOpenTerminal!()}>{copy.startTerminal}</button> : null}
    {props.terminal && props.onTerminalInput ? <form onSubmit={(event) => void sendInput(event)} className="wk-vc-page-36"><label htmlFor="wk-chat-terminal-input" className="wk-vc-page-37">{copy.terminalInput}</label><input id="wk-chat-terminal-input" value={input} onChange={(event) => setInput(event.target.value)} disabled={busy} className="wk-vc-page-38" /><button type="submit" disabled={busy || !input.trim()} className="wk-vc-page-39">{copy.sendInput}</button></form> : null}
    {props.terminal && props.onCloseTerminal ? <button type="button" className="wk-chat-terminal-close wk-vc-page-40" onClick={props.onCloseTerminal}>{copy.closeTerminal}</button> : null}
  </section>;
}

export function ChatPage(props: ChatPageProps) {
  // Chat copy resolves per locale: explicit prop wins, otherwise the app
  // convention (localStorage 'locale' set by the language switch, then
  // navigator.language). The switch dispatches 'weknora:locale-changed'
  // (GeneralPreferencesPanel); re-resolve so the page flips in place.
  const [locale, setLocale] = useState(() => props.locale ?? resolveChatLocale());
  useEffect(() => { if (props.locale) setLocale(props.locale); }, [props.locale]);
  useEffect(() => {
    if (props.locale || typeof window === 'undefined') return;
    const onLocaleChanged = () => setLocale(resolveChatLocale());
    window.addEventListener('weknora:locale-changed', onLocaleChanged);
    return () => window.removeEventListener('weknora:locale-changed', onLocaleChanged);
  }, [props.locale]);
  const copy = useMemo(() => resolveChatCopy(locale), [locale]);
  const [pending, setPending] = useState<PendingChatMessage | undefined>();
  const [sending, setSending] = useState(false);
  const sendingRef = useRef(false);
  const [activeCitationId, setActiveCitationId] = useState<string | null>(null);
  // Vue sandbox panel: closed until the header toggle opens it.
  const [terminalOpen, setTerminalOpen] = useState(props.terminalOpen ?? false);
  // Vue chat references live behind the collapsed 检索完成 summary
  // (ChatReferencesDrawer); the shared panel stays closed until that opens it.
  const [referencesOpen, setReferencesOpen] = useState(false);
  const references = useMemo(() => [...messageReferenceValues(props.messages), ...(props.stream?.references ?? [])], [props.messages, props.stream?.references]);

  useEffect(() => { setActiveCitationId(null); }, [props.selectedSessionId]);

  function activateCitation(referenceId: string): void {
    setActiveCitationId(referenceId);
    props.onCitationClick?.(referenceId);
  }

  useEffect(() => { setPending(undefined); }, [props.selectedSessionId]);

  async function send(submission: ChatSubmission) {
    if (sendingRef.current) return;
    sendingRef.current = true;
    setSending(true);
    setPending(submission);
    try {
      await props.send(submission);
      setPending(undefined);
    } catch (error) {
      setPending({
        ...submission,
        status: 'failed',
        error: error instanceof Error ? error.message : 'Message failed to send',
      });
    } finally {
      sendingRef.current = false;
      setSending(false);
    }
  }

  const headerTitle = (() => {
    const selected = props.sessions.find((session) => session.id === props.selectedSessionId);
    return selected?.title || (props.selectedSessionId ? copy.newSession : copy.newChat);
  })();
  const selectedSession = props.sessions.find((session) => session.id === props.selectedSessionId) ?? null;
  // Vue index.vue：回底按钮常驻 DOM（v-show），点击滚回 .chat_scroll_box 底部。
  const scrollBoxRef = useRef<HTMLDivElement | null>(null);
  const [userScrolledUp, setUserScrolledUp] = useState(false);
  const sandboxAvailable = Boolean(props.terminal || props.onOpenTerminal);
  const streaming = props.stream?.phase === 'streaming';
  // R471-A1: Vue isAgentStreamSession() parity — see the canSteer prop doc.
  const canSteer = props.canSteer ?? Boolean(props.onSteer);
  // Vue parity: once deepThink streams the typing dots are replaced by the
  // live thinking block (shouldShowGlobalTypingIndicator turns false when the
  // assistant message exists).
  const liveThinking = splitLiveThinking(props.stream?.answer ?? '');

  /* main.wk-chat-page utilities carry the chat.css parity values; the
     retained guard block in chat.css keeps beating the legacy styles.css
     .wk-chat-page rule until the Orchestrator deletes that block. */
  /* Vue creatChat.vue 空态：composer 属于 .dialogue-answers 列（title → 推荐问题 → 输入区）；
   * 会话视图仍由 conversation 流承载。Vue isReplying（Input-field.vue）在消息派发即翻转，
   * 不等首个 SSE 事件——composer 的停止换位须覆盖发送前窗口。 */
  const composerNode = (
    <ChatComposer
      copy={copy}
      draft={props.draft}
      focusSignal={props.composerFocusSignal}
      disabled={sending || pending !== undefined || streaming}
      onDraftChange={props.onDraftChange}
      onSubmit={(submission) => void send(submission)}
      attachments={props.attachments}
      onAttachmentSelect={props.onAttachmentSelect}
      onRemoveAttachment={props.onRemoveAttachment}
      attachmentAccept={props.attachmentAccept}
      mentionOptions={props.mentionOptions}
      mentionedItems={props.mentionedItems}
      mentionLoading={props.mentionLoading}
      mentionError={props.mentionError}
      mentionEmptyHint={props.mentionEmptyHint}
      onMentionOpen={props.onMentionOpen}
      onMentionSelect={props.onMentionSelect}
      onMentionRemove={props.onMentionRemove}
      agents={props.agents}
      selectedAgentId={props.selectedAgentId}
      onAgentChange={props.onAgentChange}
      agentModels={props.agentModels}
      onManageAgents={props.onManageAgents}
      onConfigureAgent={props.onConfigureAgent}
      onAgentNotReady={props.onAgentNotReady}
      modelLabel={props.modelLabel}
      modelContext={props.modelContext}
      modelContextIsDefault={props.modelContextIsDefault}
      modelOptions={props.modelOptions}
      selectedModelId={props.selectedModelId}
      onModelChange={props.onModelChange}
      webSearchVisible={props.webSearchVisible}
      webSearchConfigured={props.webSearchConfigured}
      webSearchEnabled={props.webSearchEnabled}
      onWebSearchToggle={props.onWebSearchToggle}
      streaming={streaming || sending}
      canSteer={canSteer}
      onStop={props.onStopStream}
      steerQueue={props.steerQueue}
      onSteerPromote={props.onSteerPromote ? (steerId) => { void props.onSteerPromote!(steerId); } : undefined}
      onSteerRemove={props.onSteerRemove ? (steerId) => { void props.onSteerRemove!(steerId); } : undefined}
      onSteerRetry={props.onSteerRetry ? (steerId) => { void props.onSteerRetry!(steerId); } : undefined}
    />
  );

  /* 会话视图：Vue index.vue 外壳（.chat > ChatHeader + .chat_thread + 回底按钮 +
   * .input-container）。空态（creatchat）保留 Task 11b 现状结构，勿重迁。 */
  if (props.selectedSessionId) {
    return <div
      className={'chat'
        + (referencesOpen ? ' has-references-panel' : '')
        + (terminalOpen ? ' has-sandbox-panel' : '')}
      style={{ '--sandbox-panel-width': '420px' } as React.CSSProperties}>
      {props.headerSlot ?? (
        /* 结构回退面（无 tdesign 弹层）：.chat-header + 标题 + ⋯ 按钮。 */
        <header className="chat-header">
          <h1 className="chat-header__title" title={headerTitle}>
            {selectedSession?.is_pinned === true ? <svg className="t-icon t-icon-pin chat-header__pin" viewBox="0 0 24 24" width="12px" height="12px" fill="none" aria-hidden="true"><use href="#t-icon-pin" /></svg> : null}
            <span className="chat-header__title-text">{headerTitle}</span>
          </h1>
          <button type="button" className="chat-header__menu-btn wk-chat-header-menu" aria-label={copy.moreActions}>
            <svg className="t-icon t-icon-ellipsis" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-ellipsis" /></svg>
          </button>
        </header>
      )}
      {/* 沙箱面板收起时：图标镜像会话左上角三个点（Vue index.vue sandbox-header-toggle）。 */}
      {sandboxAvailable && !terminalOpen ? (props.sandboxToggleSlot ?? (
        <div className="sandbox-header-toggle">
          <button type="button" className="sandbox-header-toggle__btn" aria-label={copy.openSandboxPanel} onClick={() => setTerminalOpen(true)}>
            <svg viewBox="0 0 20 20" width="18" height="18" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
              <rect x="1.5" y="1.5" width="17" height="17" rx="3" stroke="currentColor" strokeWidth="1.2" />
              <line x1="12.5" y1="1.5" x2="12.5" y2="18.5" stroke="currentColor" strokeWidth="1.2" />
              <line x1="16" y1="7.5" x2="16" y2="12.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
            </svg>
          </button>
        </div>
      )) : null}
      <div className="chat_thread">
        <ChatActionCards {...props} copy={copy} />
        {props.stream ? <LiveResponse copy={copy} stream={props.stream} /> : null}
        {props.error ? <p role="alert">{props.error}</p> : null}
        {props.loadingMessages ? <p role="status">{copy.loadingMessages}</p> : null}
        <MessageList
          copy={copy}
          messages={props.messages}
          pending={pending}
          onRetry={pending?.status === 'failed' ? () => void send({ content: pending.content, status: 'pending' }) : undefined}
          loadingOlder={props.loadingOlderMessages}
          hasMore={props.hasMoreMessages}
          onLoadOlder={props.onLoadOlderMessages}
          sessionId={props.selectedSessionId}
          typingIndicator={streaming && !props.stream!.thinking && !liveThinking.thinking && props.stream!.toolCalls.length === 0 && shouldShowTypingIndicator(props.messages, true)}
          suggestions={props.suggestions}
          onSuggestionClick={props.onSuggestionClick}
          onRefreshSuggestions={props.onRefreshSuggestions}
          onDismissSuggestions={props.onDismissSuggestions}
          onCitationClick={activateCitation}
          onToggleReferences={() => setReferencesOpen((open) => !open)}
          referencesOpen={referencesOpen}
          onBookmark={props.onBookmark}
          onRateMessage={props.onRateMessage}
          onRemoveRating={props.onRemoveRating}
          ratingOf={props.ratingOf}
          onForkMessage={props.onForkMessage}
          canForkMessage={props.canForkMessage}
          onArtifactDownload={props.onArtifactDownload}
          onArtifactPreview={props.onArtifactPreview}
          scrollContainerRef={scrollBoxRef}
          onScrolledUpChange={setUserScrolledUp}
        />
      </div>
      <div
        className="scroll-to-bottom-btn wk-chat-scroll-bottom"
        style={{ display: userScrolledUp ? undefined : 'none' }}
        onClick={() => {
          const box = scrollBoxRef.current;
          if (box) box.scrollTo({ top: box.scrollHeight });
        }}
      >
        <svg className="t-icon t-icon-chevron-down" viewBox="0 0 24 24" width="20px" height="20px" fill="none" aria-hidden="true"><use href="#t-icon-chevron-down" /></svg>
      </div>
      <div className="input-container">
        {props.onSteer && canSteer && streaming ? <SteerComposer copy={copy} onSteer={props.onSteer} steerQueue={props.steerQueue} onSteerPromote={props.onSteerPromote} mentionOptions={props.mentionOptions} mentionedItems={props.mentionedItems} attachments={props.attachments} onSteerWarning={props.onSteerWarning} onMentionOpen={props.onMentionOpen} onMentionSelect={props.onMentionSelect} onMentionRemove={props.onMentionRemove} /> : null}
        {composerNode}
      </div>
      {sandboxAvailable && terminalOpen ? <aside className="wk-chat-sandbox-drawer wk-vc-page-41" role="complementary" aria-label={copy.sandboxPanelTitle}>
        <div className="wk-chat-sandbox-drawer-head wk-vc-page-42">
          <span>{copy.sandboxPanelTitle}</span>
          <button type="button" className="wk-chat-sandbox-drawer-close wk-vc-page-43" aria-label={copy.close} onClick={() => setTerminalOpen(false)}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" aria-hidden="true"><path d="M3.5 3.5l9 9M12.5 3.5l-9 9" /></svg>
          </button>
        </div>
        <TerminalPanel copy={copy} terminal={props.terminal} onOpenTerminal={props.onOpenTerminal} onTerminalInput={props.onTerminalInput} onTerminalResize={props.onTerminalResize} onCloseTerminal={props.onCloseTerminal} />
      </aside> : null}
    </div>;
  }

  return <main className="wk-chat-page wk-vc-page-44">
    <SessionSidebar
      copy={copy}
      sessions={props.sessions}
      selectedSessionId={props.selectedSessionId}
      loading={props.loadingSessions}
      onSelect={props.onSelectSession}
      onCreate={props.onCreateSession}
      onRename={props.onRenameSession}
      onTogglePin={props.onToggleSessionPin}
      onClear={props.onClearSession}
      onDelete={props.onDeleteSession}
      onShareSession={props.onShareSession}
      groups={props.sessionGroups}
      source={props.sessionSource}
      sourceOptions={props.sessionSourceOptions}
      onSourceChange={props.onSessionSourceChange}
      groupMode={props.sessionGroupMode}
      onGroupModeChange={props.onSessionGroupModeChange}
      keyword={props.sessionKeyword}
      onKeywordChange={props.onSessionKeywordChange}
      page={props.sessionPage}
      pageCount={props.sessionPageCount}
      onPageChange={props.onSessionPageChange}
    />
    <section className="wk-chat-main wk-vc-page-45" aria-label={copy.streamStatus}>
      <div className="wk-chat-conversation wk-chat-conversation--empty wk-vc-page-46">
        {/* Vue creatChat.vue 空态簇（Task 11b 平移）：.dialogue-wrap 居中 →
            .dialogue-answers 列（gap 24，100%/max 960）→ .dialogue-title +
            .suggested-questions-container + 输入区。整数 gap 几何取代旧
            48.8px 分数 padding（其 0.5px 偏移一路放大成字形相位差）。 */}
        {!props.selectedSessionId ? (
      <div className="dialogue-wrap wk-vc-page-47">
      <div className="dialogue-answers wk-vc-page-48">
      <h1 className="dialogue-title" style={{ '--wails-draggable': 'drag' } as React.CSSProperties}><span style={{ '--wails-draggable': 'drag' } as React.CSSProperties}>{copy.createChatTitle}</span></h1>
      <div className={'suggested-questions-container' + (props.starterQuestionsLoading ? ' wk-chat-starters--loading' : '')} aria-label={(props.starterQuestionsLoading || (props.starterQuestions?.length ?? 0) > 0) ? copy.suggestedQuestions : copy.streamStatus} aria-busy={props.starterQuestionsLoading || undefined}>
            {props.starterQuestionsLoading && (props.starterQuestions?.length ?? 0) === 0 ? (
              <ul className="wk-chat-starters-grid wk-vc-page-49">
                {[0, 1, 2].map((index) => <li key={index}><span className="wk-chat-starter-skeleton wk-mr-none wk-vc-page-50" aria-hidden="true" /></li>)}
              </ul>
            ) : (props.starterQuestions?.length ?? 0) > 0 ? (
              <>
                <p className="wk-chat-starters-caption wk-vc-page-51">
                  <span>{copy.suggestedQuestions}</span>
                  {props.onRefreshStarterQuestions ? <button
                    type="button"
                    className="wk-chat-starter-refresh wk-ease-ease wk-vc-page-52"
                    disabled={props.starterQuestionsLoading}
                    title={copy.refreshSuggestedQuestions}
                    aria-label={copy.refreshSuggestedQuestions}
                    onClick={props.onRefreshStarterQuestions}
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={props.starterQuestionsLoading ? 'wk-chat-starter-refresh-icon wk-mr-none wk-vc-page-54' : 'wk-chat-starter-refresh-icon'}>
                      <path d="M10 3.5V1.5M10 1.5H8" /><path d="M10 1.5A4.5 4.5 0 1 0 10.7 7" />
                    </svg>
                  </button> : null}
                </p>
                <ul className="wk-chat-starters-grid wk-vc-page-49">
                  {props.starterQuestions!.map((question, index) => (
                    <li key={index}>
                      <button type="button" className="wk-chat-starter-card wk-ease-ease wk-vc-page-53" onClick={() => props.onStarterQuestionClick?.(question)}>{question}</button>
                    </li>
                  ))}
                </ul>
              </>
            ) : null}
      </div>
      {composerNode}
      </div>
      </div>
        ) : null}
        <ChatActionCards {...props} copy={copy} />
        {props.stream ? <LiveResponse copy={copy} stream={props.stream} /> : null}
        {referencesOpen && references.length > 0 ? <ReferenceList references={references} activeId={activeCitationId} onActivate={activateCitation} copy={copy} /> : null}
        {props.error ? <p role="alert">{props.error}</p> : null}
        {props.loadingMessages ? <p role="status">{copy.loadingMessages}</p> : null}
        {/* Vue creatChat.vue renders no message list in the empty new-chat
            view; without this guard the flex:1 scroll pushes the centered
            composer cluster apart. */}
        {!props.selectedSessionId && (props.messages?.length ?? 0) === 0 ? null : <MessageList
          copy={copy}
          messages={props.messages}
          pending={pending}
          onRetry={pending?.status === 'failed' ? () => void send({ content: pending.content, status: 'pending' }) : undefined}
          loadingOlder={props.loadingOlderMessages}
          hasMore={props.hasMoreMessages}
          onLoadOlder={props.onLoadOlderMessages}
          sessionId={props.selectedSessionId}
          typingIndicator={streaming && !props.stream!.thinking && !liveThinking.thinking && props.stream!.toolCalls.length === 0 && shouldShowTypingIndicator(props.messages, true)}
          suggestions={props.suggestions}
          onSuggestionClick={props.onSuggestionClick}
          onRefreshSuggestions={props.onRefreshSuggestions}
          onDismissSuggestions={props.onDismissSuggestions}
          onCitationClick={activateCitation}
          onToggleReferences={() => setReferencesOpen((open) => !open)}
          referencesOpen={referencesOpen}
          onBookmark={props.onBookmark}
          onRateMessage={props.onRateMessage}
          onRemoveRating={props.onRemoveRating}
          ratingOf={props.ratingOf}
          onForkMessage={props.onForkMessage}
          canForkMessage={props.canForkMessage}
          onArtifactDownload={props.onArtifactDownload}
          onArtifactPreview={props.onArtifactPreview}
        />}
      </div>
      {sandboxAvailable && terminalOpen ? <aside className="wk-chat-sandbox-drawer wk-vc-page-41" role="complementary" aria-label={copy.sandboxPanelTitle}>
        <div className="wk-chat-sandbox-drawer-head wk-vc-page-42">
          <span>{copy.sandboxPanelTitle}</span>
          <button type="button" className="wk-chat-sandbox-drawer-close wk-vc-page-43" aria-label={copy.close} onClick={() => setTerminalOpen(false)}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" aria-hidden="true"><path d="M3.5 3.5l9 9M12.5 3.5l-9 9" /></svg>
          </button>
        </div>
        <TerminalPanel copy={copy} terminal={props.terminal} onOpenTerminal={props.onOpenTerminal} onTerminalInput={props.onTerminalInput} onTerminalResize={props.onTerminalResize} onCloseTerminal={props.onCloseTerminal} />
      </aside> : null}
    </section>
  </main>;
}
