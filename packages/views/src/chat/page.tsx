import { useEffect, useMemo, useRef, useState } from 'react';
import type { ChatMessage, ChatSession, MessageSuggestionSet } from '@weknora/contracts';
import { shouldShowTypingIndicator } from '@weknora/domain/chat/session-state';
import { ChatComposer, type ChatAttachmentView, type ChatMentionView, type ChatSubmission } from './composer.tsx';
import { MessageList, TOOL_LIST_ITEM, type PendingChatMessage } from './message-list.tsx';
import { SessionSidebar } from './session-sidebar.tsx';
import { ReferenceList } from './reference-list.tsx';
import { ToolResultView } from './tool-result.tsx';
import { ToolApprovalCard } from './tool-approval.tsx';
import type { ArtifactPreviewPayload } from './artifact-preview.tsx';
import { resolveChatCopy, resolveChatLocale, type ChatCopyTable } from './chat-copy.ts';

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
}

export interface ChatToolApprovalPrompt {
  pendingId: string;
  toolName?: string;
  status: 'pending' | 'resolved';
  decision?: string;
  /** Original tool call arguments, rendered editable in the approval card. */
  arguments?: Record<string, unknown>;
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
  onMentionOpen?(): void;
  onMentionSelect?(item: ChatMentionView): void;
  onMentionRemove?(id: string): void;
  agents?: readonly ChatAgentOption[];
  selectedAgentId?: string;
  onAgentChange?(agentId: string): void;
  /** Empty-state suggested questions for the new-conversation view. */
  starterQuestions?: readonly string[];
  /** True while the agent suggested-questions request is in flight (skeleton chips). */
  starterQuestionsLoading?: boolean;
  onStarterQuestionClick?(question: string): void;
  toolApprovals?: readonly ChatToolApprovalPrompt[];
  oauthApprovals?: readonly ChatOAuthApprovalPrompt[];
  onResolveToolApproval?(pendingId: string, decision: 'approve' | 'reject', modifiedArgs?: Record<string, unknown>): Promise<void>;
  onAuthorizeOAuth?(pendingId: string, serviceId: string): Promise<void>;
  onCancelOAuth?(pendingId: string): Promise<void>;
  onSteer?(content: string, mentionedItems?: readonly ChatMentionView[]): Promise<void>;
  onStopStream?(): void;
  stream?: ChatStreamPresentation;
  onRenameSession?(sessionId: string, title?: string): Promise<void>;
  onToggleSessionPin?(sessionId: string, pinned: boolean): Promise<void>;
  onDeleteSession?(sessionId: string): Promise<void>;
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

function LiveResponse({ copy, stream, onStopStream }: { copy: ChatCopyTable; stream: ChatStreamPresentation; onStopStream?: () => void }) {
  if (stream.phase !== 'streaming' && !stream.thinking && stream.toolCalls.length === 0) return null;
  return <section aria-label={copy.streamStatus} className="wk-chat-live-response mx-auto mb-[12px] w-full max-w-[960px] rounded-[8px] border border-[#e7e7e7] px-[12px] py-[8px] text-[13px]">
    <p role="status" className="mt-0 mb-[6px] text-[rgba(0,0,0,0.6)]">{copy.streamStatus}: {stream.phase}</p>
    {stream.phase === 'streaming' && stream.artifactsPending ? <p role="status" className="wk-chat-artifacts-pending mt-0 mb-[6px] text-[rgba(0,0,0,0.6)]">{copy.artifactsPending}</p> : null}
    {stream.phase === 'streaming' && stream.thinking ? <details open className="my-[6px]"><summary className="cursor-pointer text-[rgba(0,0,0,0.6)]">{copy.thinkingAlt}</summary><p className="mt-0 mb-[6px] text-[rgba(0,0,0,0.6)]">{stream.thinking}</p></details> : null}
    {stream.toolCalls.length > 0 ? <div><h2 className="mt-[8px] mb-[4px] text-[13px]">{copy.toolCallsTitle}</h2><ul className="wk-list m-0 list-none p-0">{stream.toolCalls.map((tool) => <li key={tool.id} className={TOOL_LIST_ITEM}><strong>{tool.name ?? tool.id}</strong><small className="text-[rgba(0,0,0,0.4)]">{tool.status}</small>{tool.result === undefined ? null : <ToolResultView toolCall={tool} copy={copy} />}</li>)}</ul></div> : null}
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

  return <section aria-label={props.copy.chatActionsTitle} className="wk-chat-actions mx-auto mb-[12px] w-full max-w-[960px] rounded-[8px] border border-[#e7e7e7] px-[12px] py-[8px]">
    <h2 className="m-0 mb-[6px] text-[13px] text-[rgba(0,0,0,0.6)]">{props.copy.chatActionsTitle}</h2>
    {error ? <p role="alert">{error}</p> : null}
    {toolApprovals.map((approval) => <ToolApprovalCard
      key={`tool-${approval.pendingId}`}
      approval={approval}
      busy={busy !== null}
      onResolve={props.onResolveToolApproval
        ? (pendingId, decision, modifiedArgs) => run(pendingId, () => props.onResolveToolApproval!(pendingId, decision, modifiedArgs))
        : undefined}
      copy={props.copy}
    />)}
    {oauthApprovals.map((approval) => <div key={`oauth-${approval.pendingId}`} className="wk-chat-action-card mb-[8px] flex flex-col gap-[6px] rounded-[8px] border border-[#e7e7e7] px-[10px] py-[8px]">
      <strong className="text-[13px]">{props.copy.oauthTitle}: {approval.serviceName ?? approval.serviceId ?? props.copy.oauthTool}</strong>
      {approval.toolName ? <small className="text-[rgba(0,0,0,0.4)]">{props.copy.oauthTool}: {approval.toolName}</small> : null}
      {approval.status === 'pending' && approval.serviceId && props.onAuthorizeOAuth && props.onCancelOAuth ? <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-white px-[12px] py-[4px] text-[13px] text-[rgba(0,0,0,0.9)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-55" disabled={busy !== null} onClick={() => void run(approval.pendingId, () => props.onAuthorizeOAuth!(approval.pendingId, approval.serviceId!))}>{props.copy.oauthAuthorize} {approval.serviceName ?? approval.serviceId}</button><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-white px-[12px] py-[4px] text-[13px] text-[rgba(0,0,0,0.9)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-55" disabled={busy !== null} onClick={() => void run(approval.pendingId, () => props.onCancelOAuth!(approval.pendingId))}>{props.copy.oauthCancel}</button></div> : <small className="text-[rgba(0,0,0,0.4)]">{approval.authorized ? props.copy.oauthAuthorized : approval.reason ?? props.copy.approvalResolved}</small>}
    </div>)}
  </section>;
}

function SteerComposer({ copy, onSteer, mentionOptions = [], mentionedItems = [], attachments = [], onMentionOpen, onMentionSelect, onMentionRemove }: {
  copy: ChatCopyTable;
  onSteer: (content: string, mentionedItems: readonly ChatMentionView[]) => Promise<void>;
  mentionOptions?: readonly ChatMentionView[];
  mentionedItems?: readonly ChatMentionView[];
  attachments?: readonly ChatAttachmentView[];
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

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const content = draft.trim();
    if (!content) return;
    if (attachments.length > 0) {
      setError(copy.steerAttachmentsBlocked);
      return;
    }
    setBusy(true); setError(null);
    try { await onSteer(content, mentionedItems); setDraft(''); } catch (cause) { setError(cause instanceof Error ? cause.message : copy.sendFailed); } finally { setBusy(false); }
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

  return <form className="wk-chat-steer mx-auto grid w-full max-w-[960px] gap-[6px] rounded-[10px_10px_0_0] border border-b-0 border-[#dcdcdc] px-[12px] py-[8px]" onSubmit={(event) => void submit(event)}>
    <label htmlFor="wk-chat-steer-draft" className="text-[12px] text-[rgba(0,0,0,0.6)]">{copy.steerCurrent}</label>
    {mentionedItems.length > 0 ? <ul className="m-0 flex flex-wrap gap-[6px] p-0" aria-label={copy.mentionKnowledge}>{mentionedItems.map((item) => <li key={item.id} data-mention-id={item.id} data-mention-type={item.type} className="inline-flex items-center gap-[5px] rounded-[6px] border border-[#d9f2e2] bg-[#f2fbf5] px-[7px] py-[3px] text-[12px] text-[rgba(0,0,0,0.65)]"><span aria-hidden="true">{mentionMarker(item.type)}</span><span>{item.name}</span><button type="button" aria-label={`${copy.close}: ${item.name}`} className="border-0 bg-transparent p-0" disabled={busy} onClick={() => onMentionRemove?.(item.id)}>×</button></li>)}</ul> : null}
    <textarea id="wk-chat-steer-draft" rows={2} value={draft} onChange={(event) => setDraft(event.target.value)} disabled={busy} className="min-h-[40px] resize-none rounded-[6px] border-0 px-[8px] py-[6px] [font:inherit] text-[13px]" />
    <div className="relative flex items-center gap-[6px]"><button id="wk-chat-steer-mention" type="button" aria-label={copy.mentionKnowledge} aria-expanded={mentionOpen} disabled={busy} className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-white px-[8px] py-[4px] text-[12px] disabled:cursor-not-allowed disabled:opacity-50" onClick={openMentions}>@</button>{mentionOpen ? <div role="listbox" aria-label={copy.mentionKnowledge} className="absolute bottom-[34px] left-0 z-20 w-[260px] rounded-[8px] border border-[#e7e7e7] bg-white p-[8px] shadow-[0_8px_24px_rgba(0,0,0,0.12)]"><input autoFocus value={mentionQuery} onChange={(event) => { setMentionQuery(event.target.value); setActiveMentionIndex(0); }} onKeyDown={handleMentionKeyDown} aria-label={copy.composerPlaceholder} aria-activedescendant={availableMentions.length > 0 ? `wk-chat-steer-mention-option-${availableMentions[Math.min(activeMentionIndex, availableMentions.length - 1)].id}` : undefined} aria-controls="wk-chat-steer-mention-options" placeholder={copy.composerPlaceholder} className="mb-[6px] box-border w-full rounded-[6px] border border-[#e7e7e7] px-[8px] py-[5px] text-[12px]" />{availableMentions.length > 0 ? <div id="wk-chat-steer-mention-options">{availableMentions.map((item, index) => <button key={item.id} id={`wk-chat-steer-mention-option-${item.id}`} type="button" role="option" aria-selected={index === activeMentionIndex} data-mention-id={item.id} data-mention-type={item.type} className="block w-full rounded-[6px] border-0 bg-transparent px-[8px] py-[6px] text-left text-[12px] hover:bg-[#f3f3f3]" onClick={() => { onMentionSelect?.(item); closeMentions(); }}><span aria-hidden="true">{mentionMarker(item.type)}</span><span>{item.name}</span></button>)}</div> : <p className="m-0 px-[8px] py-[6px] text-[12px] text-[rgba(0,0,0,0.45)]">{copy.mentionNoAvailable}</p>}</div> : null}</div>
    {error ? <p role="alert">{error}</p> : null}
    <button type="submit" disabled={busy || !draft.trim()} className="cursor-pointer self-end rounded-[6px] border-0 bg-[#07c05f] px-[12px] py-[5px] text-[13px] text-white disabled:cursor-not-allowed disabled:opacity-50">{copy.steerQueued}</button>
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
  return <section ref={panelRef} aria-label={`${props.copy.sandboxPanelTitle} (Sandbox terminal)`} className="wk-chat-terminal m-0 flex min-h-0 w-full max-w-none flex-1 flex-col gap-[8px] overflow-auto rounded-none border-0 p-[12px] text-[12px]">
    <div className="wk-settings-panel-heading m-0 flex items-center justify-between gap-[8px] border-b border-[#eef1f5] pb-[1rem]"><span role="status">{props.terminal?.status ?? 'idle'}</span></div>
    {props.terminal?.output ? <pre className="m-0 min-h-[120px] flex-1 overflow-auto whitespace-pre-wrap break-words rounded-[6px] bg-[#1f2430] p-[10px] text-[12px] leading-[1.5] text-[#d5dded]">{props.terminal.output}</pre> : null}
    {!props.terminal?.output && props.onOpenTerminal ? <p className="wk-chat-terminal-hint m-0 leading-[1.6] text-[rgba(0,0,0,0.4)]">{copy.startTerminal}</p> : null}
    {props.onOpenTerminal && !props.terminal?.output ? <button type="button" className="wk-chat-terminal-open cursor-pointer self-start rounded-[6px] border-0 bg-[#07c05f] px-[14px] py-[6px] text-[13px] text-white" onClick={() => void props.onOpenTerminal!()}>{copy.startTerminal}</button> : null}
    {props.terminal && props.onTerminalInput ? <form onSubmit={(event) => void sendInput(event)} className="flex items-center gap-[6px]"><label htmlFor="wk-chat-terminal-input" className="shrink-0 text-[rgba(0,0,0,0.4)]">{copy.terminalInput}</label><input id="wk-chat-terminal-input" value={input} onChange={(event) => setInput(event.target.value)} disabled={busy} className="flex-1 rounded-[6px] border border-[#dcdcdc] px-[8px] py-[5px] text-[13px]" /><button type="submit" disabled={busy || !input.trim()} className="cursor-pointer rounded-[6px] border-0 bg-[#07c05f] px-[10px] py-[5px] text-[13px] text-white disabled:cursor-not-allowed disabled:opacity-50">{copy.sendInput}</button></form> : null}
    {props.terminal && props.onCloseTerminal ? <button type="button" className="wk-chat-terminal-close cursor-pointer self-start rounded-[6px] border border-[#dcdcdc] bg-white px-[14px] py-[6px] text-[13px] text-[rgba(0,0,0,0.6)]" onClick={props.onCloseTerminal}>{copy.closeTerminal}</button> : null}
  </section>;
}

function ChatHeaderMenu(props: { copy: ChatCopyTable } & Pick<ChatPageProps, 'selectedSessionId' | 'onRenameSession' | 'onToggleSessionPin' | 'onDeleteSession' | 'onClearSession' | 'sessions'>) {
  const copy = props.copy;
  const session = props.sessions.find((item) => item.id === props.selectedSessionId) ?? null;
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameValue, setRenameValue] = useState('');
  const [renameError, setRenameError] = useState<string | null>(null);
  const [renameBusy, setRenameBusy] = useState(false);
  const renameInputRef = useRef<HTMLInputElement | null>(null);
  const renameEditorRef = useRef<HTMLDivElement | null>(null);
  const renameSubmittingRef = useRef(false);
  const renameDetailsRef = useRef<HTMLDetailsElement | null>(null);
  const renameTriggerRef = useRef<HTMLElement | null>(null);
  if (!session) return null;
  const pinned = session.is_pinned === true;
  const openRename = () => {
    setRenameValue(session.title ?? '');
    setRenameError(null);
    renameDetailsRef.current?.removeAttribute('open');
    setRenameOpen(true);
  };
  const closeRename = () => {
    setRenameOpen(false);
    setRenameError(null);
    window.setTimeout(() => renameTriggerRef.current?.focus(), 0);
  };
  useEffect(() => {
    if (!renameOpen) return;
    const frame = window.requestAnimationFrame(() => renameInputRef.current?.select());
    return () => window.cancelAnimationFrame(frame);
  }, [renameOpen]);
  const submitRename = async () => {
    if (renameSubmittingRef.current || !props.onRenameSession) return;
    const title = renameValue.trim().replace(/\s+/g, ' ').slice(0, 80);
    if (!title) {
      setRenameError(copy.renameTitleRequired);
      renameInputRef.current?.focus();
      return;
    }
    const currentTitle = (session.title ?? '').trim().replace(/\s+/g, ' ').slice(0, 80);
    if (title === currentTitle) { closeRename(); return; }
    renameSubmittingRef.current = true;
    setRenameBusy(true);
    setRenameError(null);
    try {
      await props.onRenameSession(session.id, title);
      closeRename();
    } catch {
      setRenameError(copy.renameTitleFailed);
    } finally {
      renameSubmittingRef.current = false;
      setRenameBusy(false);
    }
  };
  /* .wk-chat-header-menu / -list → utilities (Vue ChatHeader ⋯ menu). */
  const menuItem = 'min-h-[30px] cursor-pointer whitespace-nowrap rounded-[5px] border-0 bg-transparent px-[10px] py-0 text-left text-[13px] leading-[20px] text-[rgba(0,0,0,0.9)] hover:bg-[#f3f3f3]';
  return <>
    <details ref={renameDetailsRef} className="wk-chat-header-menu relative">
    <summary ref={renameTriggerRef} aria-label={copy.moreActions} title={copy.moreActions} className="inline-flex h-[24px] w-[24px] cursor-pointer list-none items-center justify-center rounded-[5px] border-0 text-[rgba(0,0,0,0.26)] transition-[background-color,color] duration-[150ms] ease-[ease] hover:bg-[#f3f3f3] hover:text-[rgba(0,0,0,0.9)] [&::-webkit-details-marker]:hidden">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><circle cx="8" cy="3" r="1.4" /><circle cx="8" cy="8" r="1.4" /><circle cx="8" cy="13" r="1.4" /></svg>
    </summary>
    <div className="wk-chat-header-menu-list absolute left-0 top-full z-[30] mt-[2px] flex min-w-[132px] flex-col gap-[1px] rounded-[8px] border-[0.5px] border-[#e7e7e7] bg-white p-[4px] shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_6px_rgba(0,0,0,0.08)]" role="menu">
      {props.onToggleSessionPin ? <button type="button" role="menuitem" className={menuItem} onClick={() => void props.onToggleSessionPin!(session.id, !pinned)}>{pinned ? copy.unpin : copy.pin}</button> : null}
      {props.onRenameSession ? <button type="button" role="menuitem" className={menuItem} onClick={openRename}>{copy.renameSession}</button> : null}
      {props.onClearSession ? <button type="button" role="menuitem" className={menuItem} onClick={() => void props.onClearSession!()}>{copy.clearMessages}</button> : null}
      {props.onDeleteSession ? <button type="button" role="menuitem" className={menuItem + ' text-[#e34d59] hover:bg-[#fdecee]'} onClick={() => void props.onDeleteSession!(session.id)}>{copy.deleteSession}</button> : null}
    </div>
    </details>
    {renameOpen ? <div ref={renameEditorRef} className="inline-flex min-w-0 items-center gap-[4px]" role="group" aria-label={copy.renameTitle}>
      <input ref={renameInputRef} type="text" value={renameValue} placeholder={copy.renameTitlePlaceholder} maxLength={80} autoFocus disabled={renameBusy} aria-label={copy.renameTitle} aria-invalid={renameError ? 'true' : undefined} className="box-border min-w-0 w-[min(240px,50vw)] rounded-[5px] border border-[#07c05f] bg-white px-[7px] py-[3px] text-[14px] leading-[20px] outline-none" onChange={(event) => setRenameValue(event.target.value)} onBlur={(event) => { const next = event.relatedTarget; if (next instanceof Node && renameEditorRef.current?.contains(next)) return; void submitRename(); }} onKeyDown={(event) => { if (event.key === 'Escape') { event.preventDefault(); closeRename(); } if (event.key === 'Enter') { event.preventDefault(); void submitRename(); } }} />
      <button type="button" aria-label={copy.renameConfirm} className="h-[24px] cursor-pointer rounded-[5px] border-0 bg-[#07c05f] px-[7px] text-[12px] text-white disabled:opacity-50" onClick={() => void submitRename()} disabled={renameBusy}>{renameBusy ? copy.renameSaving : copy.renameConfirm}</button>
      <button type="button" aria-label={copy.renameCancel} className="h-[24px] cursor-pointer rounded-[5px] border-0 bg-transparent px-[5px] text-[12px] text-[rgba(0,0,0,0.55)] hover:bg-[#f3f3f3]" onClick={closeRename} disabled={renameBusy}>{copy.renameCancel}</button>
      {renameError ? <span role="alert" className="text-[11px] leading-[16px] text-[#e34d59]">{renameError}</span> : null}
    </div> : null}
  </>;
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
  const sandboxAvailable = Boolean(props.terminal || props.onOpenTerminal);
  const streaming = props.stream?.phase === 'streaming';

  /* main.wk-chat-page utilities carry the chat.css parity values; the
     retained guard block in chat.css keeps beating the legacy styles.css
     .wk-chat-page rule until the Orchestrator deletes that block. */
  return <main className="wk-chat-page grid h-screen items-stretch gap-0 m-0 max-w-none p-0 grid-cols-[minmax(0,1fr)]">
    <SessionSidebar
      copy={copy}
      sessions={props.sessions}
      selectedSessionId={props.selectedSessionId}
      loading={props.loadingSessions}
      onSelect={props.onSelectSession}
      onCreate={props.onCreateSession}
      onRename={props.onRenameSession}
      onTogglePin={props.onToggleSessionPin}
      onDelete={props.onDeleteSession}
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
    <section className="wk-chat-main flex min-h-0 min-w-0 flex-col" aria-label={copy.streamStatus}>
      {props.selectedSessionId ? <header className="wk-chat-header pointer-events-none absolute inset-x-[12px] top-0 z-[6] flex shrink-0 items-center justify-between gap-[8px] border-b-0 bg-transparent px-[12px] pt-[10px] pb-0">
        <div className="wk-chat-header-titles pointer-events-auto inline-flex items-center gap-[2px] max-w-[min(320px,100%)] rounded-[8px] bg-[rgba(255,255,255,0.88)] p-[2px] pl-[8px] backdrop-blur-[8px]">
          <h1 title={headerTitle} className="m-0 min-w-0 cursor-default overflow-hidden text-ellipsis whitespace-nowrap text-[14px] font-medium leading-[20px] text-[rgba(0,0,0,0.6)]">{headerTitle}</h1>
          <ChatHeaderMenu
            copy={copy}
            selectedSessionId={props.selectedSessionId}
            sessions={props.sessions}
            onRenameSession={props.onRenameSession}
            onToggleSessionPin={props.onToggleSessionPin}
            onDeleteSession={props.onDeleteSession}
            onClearSession={props.onClearSession}
          />
        </div>
        <div className="wk-chat-header-actions pointer-events-auto inline-flex items-center gap-[8px] rounded-[8px] bg-[rgba(255,255,255,0.88)] p-[2px] backdrop-blur-[8px]">
            {sandboxAvailable ? <button type="button" className="wk-chat-sandbox-toggle inline-flex h-[24px] w-[24px] cursor-pointer items-center justify-center rounded-[5px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.26)] transition-[background-color,color] duration-[150ms] ease-[ease] hover:bg-[#f3f3f3] hover:text-[rgba(0,0,0,0.9)]" aria-label={copy.openSandboxPanel} title={copy.openSandboxPanel} aria-expanded={terminalOpen} onClick={() => setTerminalOpen((open) => !open)}>
              <svg width="18" height="18" viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <rect x="1.5" y="1.5" width="17" height="17" rx="3" stroke="currentColor" strokeWidth="1.2" />
                <line x1="12.5" y1="1.5" x2="12.5" y2="18.5" stroke="currentColor" strokeWidth="1.2" />
                <line x1="16" y1="7.5" x2="16" y2="12.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
              </svg>
            </button> : null}
          </div>
        </header> : null}
      <div className={props.selectedSessionId ? 'wk-chat-conversation flex min-h-0 flex-1 flex-col px-[16px] pb-[12px] pt-0' : 'wk-chat-conversation wk-chat-conversation--empty flex min-h-0 flex-1 flex-col justify-center px-[16px] pb-[12px] pt-0'}>
        {/* Vue creatChat.vue: the welcome heading is always part of the empty
            state; suggested-question cards load per selected agent. The empty
            view centers the welcome+composer cluster (.dialogue-wrap) and must
            not render the flex:1 message scroll that pins the composer down. */}
        {!props.selectedSessionId ? (
          <section className={props.starterQuestionsLoading ? 'wk-chat-starters wk-chat-starters--loading mx-auto w-full max-w-[960px] animate-[wk-content-fade-in_0.3s_ease-out] motion-reduce:animate-none' : 'wk-chat-starters mx-auto w-full max-w-[960px] px-0 pt-0 pb-[56px] animate-[wk-content-fade-in_0.3s_ease-out] motion-reduce:animate-none'} aria-label={(props.starterQuestionsLoading || (props.starterQuestions?.length ?? 0) > 0) ? copy.suggestedQuestions : copy.streamStatus} aria-busy={props.starterQuestionsLoading || undefined}>
            {/* Empty-view starters always sit inside .wk-chat-conversation--empty,
                whose padding override (0 0 24px) replaces the base 48px padding. */}
            <h1 className="wk-chat-welcome m-0 text-center text-[28px] font-semibold leading-[1.4] text-[rgba(0,0,0,0.9)]">{copy.createChatTitle}</h1>
            {props.starterQuestionsLoading ? (
              <ul className="wk-chat-starters-grid flex w-full flex-wrap justify-center gap-[10px] list-none m-0 px-[16px] py-0">
                {[0, 1, 2].map((index) => <li key={index}><span className="wk-chat-starter-skeleton block h-[37px] w-[180px] rounded-[10px] bg-[linear-gradient(90deg,#eceef1_25%,#f6f7f8_50%,#eceef1_75%)] bg-[length:200%_100%] animate-[wk-chat-skeleton_1.4s_infinite_ease] motion-reduce:animate-none" aria-hidden="true" /></li>)}
              </ul>
            ) : (props.starterQuestions?.length ?? 0) > 0 ? (
              <>
                <p className="wk-chat-starters-caption m-0 text-center text-[13px] tracking-[0.01em] text-[rgba(0,0,0,0.26)]">
                  <span>{copy.suggestedQuestions}</span>
                </p>
                <ul className="wk-chat-starters-grid flex w-full flex-wrap justify-center gap-[10px] list-none m-0 px-[16px] py-0">
                  {props.starterQuestions!.map((question, index) => (
                    <li key={index}>
                      <button type="button" className="wk-chat-starter-card box-border max-w-full cursor-pointer overflow-hidden text-ellipsis whitespace-nowrap rounded-[10px] border border-[#e7e7e7] bg-white px-[14px] py-[8px] text-[13px] leading-[1.5] text-[rgba(0,0,0,0.9)] shadow-[0_1px_2px_rgba(0,0,0,0.04)] transition-[border-color,box-shadow,background] duration-200 ease-[ease] hover:border-[rgba(0,0,0,0.1)] hover:shadow-[0_2px_6px_rgba(0,0,0,0.05)] focus-visible:border-[rgba(0,0,0,0.1)] focus-visible:shadow-[0_2px_6px_rgba(0,0,0,0.05)]" onClick={() => props.onStarterQuestionClick?.(question)}>{question}</button>
                    </li>
                  ))}
                </ul>
              </>
            ) : null}
          </section>
        ) : null}
        <ChatActionCards {...props} copy={copy} />
        {props.stream ? <LiveResponse copy={copy} stream={props.stream} /> : null}
        <ReferenceList references={references} activeId={activeCitationId} onActivate={activateCitation} copy={copy} />
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
          typingIndicator={streaming && !props.stream!.thinking && props.stream!.toolCalls.length === 0 && shouldShowTypingIndicator(props.messages, true)}
          suggestions={props.suggestions}
          onSuggestionClick={props.onSuggestionClick}
          onRefreshSuggestions={props.onRefreshSuggestions}
          onDismissSuggestions={props.onDismissSuggestions}
          onCitationClick={activateCitation}
          onArtifactDownload={props.onArtifactDownload}
          onArtifactPreview={props.onArtifactPreview}
        />}
        {/* A follow-up queue only makes sense while a turn is actually running;
            when idle the main composer handles the message (a steer would 409). */}
        {props.selectedSessionId && props.onSteer && streaming ? <SteerComposer copy={copy} onSteer={props.onSteer} mentionOptions={props.mentionOptions} mentionedItems={props.mentionedItems} attachments={props.attachments} onMentionOpen={props.onMentionOpen} onMentionSelect={props.onMentionSelect} onMentionRemove={props.onMentionRemove} /> : null}
        <ChatComposer
          copy={copy}
          draft={props.draft}
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
          onMentionOpen={props.onMentionOpen}
          onMentionSelect={props.onMentionSelect}
          onMentionRemove={props.onMentionRemove}
          agents={props.agents}
          selectedAgentId={props.selectedAgentId}
          onAgentChange={props.onAgentChange}
          modelLabel={props.modelLabel}
          modelContext={props.modelContext}
          modelContextIsDefault={props.modelContextIsDefault}
          modelOptions={props.modelOptions}
          selectedModelId={props.selectedModelId}
          onModelChange={props.onModelChange}
          streaming={streaming}
          onStop={props.onStopStream}
        />
      </div>
      {sandboxAvailable && terminalOpen ? <aside className="wk-chat-sandbox-drawer absolute bottom-0 right-0 top-0 z-[40] flex w-[min(420px,100%)] max-w-[100vw] flex-col border-l border-[#e7e7e7] bg-white shadow-[-8px_0_24px_rgba(0,0,0,0.06)]" role="complementary" aria-label={copy.sandboxPanelTitle}>
        <div className="wk-chat-sandbox-drawer-head flex shrink-0 items-center justify-between border-b border-[#e7e7e7] px-[12px] py-[8px] text-[13px] font-medium text-[rgba(0,0,0,0.9)]">
          <span>{copy.sandboxPanelTitle}</span>
          <button type="button" className="wk-chat-sandbox-drawer-close inline-flex h-[32px] w-[32px] shrink-0 cursor-pointer items-center justify-center rounded-[8px] border-0 bg-[#f3f3f3] text-[rgba(0,0,0,0.6)] hover:bg-[#eee] hover:text-[rgba(0,0,0,0.9)]" aria-label={copy.close} onClick={() => setTerminalOpen(false)}>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" aria-hidden="true"><path d="M3.5 3.5l9 9M12.5 3.5l-9 9" /></svg>
          </button>
        </div>
        <TerminalPanel copy={copy} terminal={props.terminal} onOpenTerminal={props.onOpenTerminal} onTerminalInput={props.onTerminalInput} onTerminalResize={props.onTerminalResize} onCloseTerminal={props.onCloseTerminal} />
      </aside> : null}
    </section>
  </main>;
}
