import { Fragment, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { ChatMessage, MessageSuggestionSet } from '@weknora/contracts';
import { hasSessionChanged, scrollTopAfterPrepend, shouldStickToBottom } from '@weknora/domain/chat/session-state';
import { isArtifactExpired, normalizeArtifactList, type ChatArtifact } from '@weknora/domain/chat/artifacts';
import { assistantMessageExtras } from '@weknora/domain/chat/message-extras';
import {
  formatConversationTimestampLabel,
  shouldShowConversationTimestamp,
} from '@weknora/domain/chat/message-timestamps';
import { copyAnswerText } from '@weknora/domain/chat/copy-answer';
import { renderChatMarkdown } from './markdown.ts';
import { hydrateMermaidBlocksWithBrowserDefaults } from './mermaid.ts';
import {
  ArtifactPreview,
  artifactPreviewModel,
  ARTIFACT_PREVIEW_DEFAULT_WIDTH,
  clampArtifactPreviewWidth,
  formatArtifactDateTime,
  formatArtifactSize,
  type ArtifactPreviewPayload,
} from './artifact-preview.tsx';
import { conversationTimeLabels, resolveChatCopy, resolveChatLocale, type ChatCopyTable } from './chat-copy.ts';

/*
 * chat.css → utilities (Tailwind migration). The wk-* classes stay as
 * DOM/test anatomy hooks; conflicts with the legacy unlayered .wk-chat-*
 * rules in apps/web/src/styles.css keep a small guard block in chat.css
 * (utilities lose the @layer cascade) until the Orchestrator deletes it.
 */
/** .wk-chat-answer-toolbar button (+ :hover:not([aria-disabled]) / [aria-disabled]) */
const ANSWER_TOOL_BUTTON = "inline-flex h-[30px] w-[30px] shrink-0 cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] transition-[background-color,color] duration-[150ms] ease-[ease] hover:not-[aria-disabled='true']:bg-[#f3f3f3] hover:not-[aria-disabled='true']:text-[rgba(0,0,0,0.9)] [aria-disabled='true']:cursor-default [aria-disabled='true']:opacity-55";
/** .wk-chat-typing-dots i (+ nth-child delays via arbitrary animation-delay) */
const TYPING_DOT = "h-[12px] w-[12px] rounded-full border-[1.5px] border-[#c5c5c5] border-t-[rgba(0,0,0,0.6)] bg-[#dcdcdc] opacity-85 animate-[wk-chat-typing-bounce_1.2s_infinite_ease-in-out] motion-reduce:animate-none motion-reduce:opacity-60";
/** .wk-chat-message-row--user .wk-chat-message-bubble (Vue .user_msg pill) */
const USER_BUBBLE = "ml-auto box-border w-max max-w-[min(76%,820px)] rounded-[8px] bg-[#f3f3f3] px-[12px] py-[8px] text-left text-[16px] leading-[1.6] text-[rgba(0,0,0,0.9)] whitespace-pre-wrap break-words [overflow-wrap:anywhere]";
/** .wk-list li effective values (the styles.css rule wins the unlayered tie) */
export const TOOL_LIST_ITEM = "flex items-baseline justify-between gap-[1rem] border-b border-[#edf0f5] py-[0.9rem]";

export type AssistantTimelineItem =
  | { kind: 'tool'; id: string; name?: string; status: 'pending' | 'completed' | 'failed'; result?: unknown }
  | { kind: 'finish' };

export interface PendingChatMessage {
  content: string;
  status: 'pending' | 'failed';
  error?: string;
}

export interface MessageListProps {
  messages: readonly ChatMessage[];
  pending?: PendingChatMessage;
  onRetry?(): void;
  loadingOlder?: boolean;
  hasMore?: boolean;
  onLoadOlder?(): void;
  suggestions?: MessageSuggestionSet;
  onSuggestionClick?(questionId: string, text: string): void;
  onRefreshSuggestions?(): void;
  onDismissSuggestions?(): void;
  onCitationClick?(citationId: string): void;
  /** Host-owned Vue botmsg knowledge-base action; absent means unavailable. */
  onBookmark?(messageId: string): void | Promise<void>;
  /** Vue usermsg/botmsg 分叉 entry: fork this session at the given message. */
  onForkMessage?(messageId: string): void;
  /** Vue index.vue forkAffordanceOf gate; absent hides every fork button. */
  canForkMessage?(messageId: string): boolean;
  onArtifactDownload?(messageId: string, artifactIndex: number): Promise<void>;
  onArtifactPreview?(messageId: string, artifactIndex: number): Promise<ArtifactPreviewPayload>;
  sessionId?: string | null;
  /** True while the turn streams and no assistant content has arrived (Vue index.vue typing dots). */
  typingIndicator?: boolean;
  /** Resolved copy (chat-copy.ts); defaults to the app locale convention. */
  copy?: ChatCopyTable;
}

export function messageArtifactItems(message: Record<string, unknown>): ChatArtifact[] {
  return normalizeArtifactList(Array.isArray(message.artifacts) ? message.artifacts : undefined);
}

export function renderMessageHtml(message: Pick<ChatMessage, 'content'>, invalidImageLabel?: string): string {
  return renderChatMarkdown(message.content, invalidImageLabel ? { invalidImageLabel } : {});
}

export async function writeClipboardText(text: string, clipboard?: { writeText(t: string): Promise<void> }): Promise<void> {
  const target = clipboard ?? (typeof navigator !== 'undefined' ? navigator.clipboard : undefined);
  if (target?.writeText) {
    await target.writeText(text);
    return;
  }
  if (typeof document === 'undefined') throw new Error('Clipboard is unavailable');
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  try { document.execCommand('copy'); } finally { textarea.remove(); }
}

function CopyAnswerButton({ copy: copyTable, message }: { copy: ChatCopyTable; message: ChatMessage }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | null>(null);
  const text = copyAnswerText(message.content);
  useEffect(() => () => { if (timer.current !== null) window.clearTimeout(timer.current); }, []);
  if (!text) return null;
  async function copy() {
    try {
      await writeClipboardText(text);
      setCopied(true);
      if (timer.current !== null) window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard permission denied: the button simply stays in place.
    }
  }
  return <button type="button" className={`${ANSWER_TOOL_BUTTON}${copied ? ' wk-chat-copy is-copied text-[#07c05f]' : ' wk-chat-copy'}`} onClick={() => void copy()} aria-label={copied ? copyTable.copied : copyTable.copy} title={copied ? copyTable.copied : copyTable.copy}>
    <CopyIcon />
  </button>;
}

function CopyIcon() {
  return <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
    <path d="M10.5 5.5v-2a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2" />
  </svg>;
}

export function isBookmarkActionAvailable(onBookmark?: MessageListProps['onBookmark']): boolean {
  return Boolean(onBookmark);
}

function BookmarkAnswerButton({ copy: copyTable, messageId, onBookmark }: { copy: ChatCopyTable; messageId: string; onBookmark?: MessageListProps['onBookmark'] }) {
  // Keep the control truthful until the host provides the Vue manual-editor
  // action; consumers with that capability get the same enabled affordance.
  const unavailable = !isBookmarkActionAvailable(onBookmark);
  return <button type="button" className={`wk-chat-bookmark ${ANSWER_TOOL_BUTTON}`} aria-label={copyTable.addToKnowledgeBase} title={copyTable.addToKnowledgeBase} aria-disabled={unavailable ? 'true' : undefined} disabled={unavailable} onClick={() => void onBookmark?.(messageId)}>
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 2.5h8a1 1 0 0 1 1 1V14l-5-2.6L3 14V3.5a1 1 0 0 1 1-1Z" />
      <path d="M8 5.5v4M6 7.5h4" />
    </svg>
  </button>;
}

function FallbackInfoButton({ copy: copyTable, message }: { copy: ChatCopyTable; message: ChatMessage }) {
  const fallback = (message as Record<string, unknown>).is_fallback === true;
  if (!fallback) return null;
  return <button type="button" className={`wk-chat-request-info ${ANSWER_TOOL_BUTTON}`} aria-label={copyTable.fallbackHint} title={copyTable.fallbackHint}>
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" aria-hidden="true">
      <circle cx="8" cy="8" r="6.2" />
      <path d="M8 7.2v3.4" />
      <circle cx="8" cy="5" r="0.7" fill="currentColor" stroke="none" />
    </svg>
  </button>;
}

/*
 * Vue AgentStreamDisplay order: reasoning first, tool calls next, finish last.
 * R464: the reasoning item itself is gone — the Vue main chat face never
 * renders the `thinking` field or the persisted `agent_steps` reasoning text
 * (botmsg.vue only shows `<think>`-tag content via deepThink and the agent
 * timeline is a separate surface), so the simplified React timeline keeps
 * only the tool calls and the finish node.
 */
export function assistantTimelineItems(message: ChatMessage): AssistantTimelineItem[] {
  const extras = assistantMessageExtras(message);
  const items: AssistantTimelineItem[] = [];
  for (const call of extras.toolCalls) items.push({ kind: 'tool', ...call });
  if (message.is_completed === true && items.length > 0) items.push({ kind: 'finish' });
  return items;
}

function TypingIndicator({ copy: copyTable }: { copy: ChatCopyTable }) {
  return <li className="wk-chat-typing m-0 flex min-h-[28px] list-none items-center border-b border-[#edf0f5] py-[0.8rem]" role="status" aria-label={copyTable.thinkingAlt}>
    <span className="wk-chat-typing-dots inline-flex gap-[4px]" aria-hidden="true"><i className={TYPING_DOT} /><i className={`${TYPING_DOT} [animation-delay:150ms]`} /><i className={`${TYPING_DOT} [animation-delay:300ms]`} /></span>
  </li>;
}

function AssistantExtras(props: { copy: ChatCopyTable; message: ChatMessage }) {
  const items = assistantTimelineItems(props.message);
  if (items.length === 0) return null;
  return <section className='wk-chat-message-extras mt-[8px] rounded-[8px] border border-[#e7e7e7] text-[13px]' aria-label={props.copy.thinkingAndTools}>
    <ol className='wk-chat-agent-timeline m-0 list-none p-[6px]'>
      {items.map((item) => item.kind === 'tool' ? <li key={item.id} className={`wk-chat-agent-timeline-item flex items-baseline justify-between gap-[1rem] border-b border-[#edf0f5] py-[6px] last:border-b-0`} data-status={item.status}>
        <details className='min-w-0'>
          <summary className='cursor-pointer truncate'>{item.name ?? item.id}</summary>
          {item.result !== undefined ? <pre className='mt-[6px] max-h-[180px] overflow-auto whitespace-pre-wrap break-words rounded-[6px] bg-[#f9f9f9] p-[8px] text-[12px]'>{typeof item.result === 'string' ? item.result : JSON.stringify(item.result, null, 2)}</pre> : null}
        </details>
        <small className='shrink-0 text-[rgba(0,0,0,0.4)]'>{item.status}</small>
      </li> : <li key='finish' className='wk-chat-agent-timeline-item flex items-center gap-[6px] py-[6px] text-[rgba(0,0,0,0.6)]' data-status='completed' role='status'>✓ <span>{props.copy.approvalResolved}</span></li>)}
    </ol>
  </section>;
}

function ArtifactList({ copy: copyTable, message, onDownload, onPreview, onOpenList }: { copy: ChatCopyTable; message: ChatMessage; onDownload?: MessageListProps['onArtifactDownload']; onPreview?: (messageId: string, artifactIndex: number) => void | Promise<void>; onOpenList?: (messageId: string) => void }) {
  const artifacts = messageArtifactItems(message);
  if (artifacts.length === 0) return null;
  return <section className="wk-chat-artifacts mt-[0.7rem] border-t border-[#edf0f5] pt-[0.5rem]" aria-label={copyTable.artifacts}>
    <div className="mb-[0.35rem] flex items-center justify-between gap-[0.5rem]"><h3 className="m-0 text-[0.85rem] text-[rgba(0,0,0,0.6)]">{copyTable.artifacts}</h3>{onOpenList ? <button type="button" className="wk-chat-artifacts-open cursor-pointer border-0 bg-transparent p-0 text-[12px] text-[#245a9b] underline" onClick={() => onOpenList(message.id)}>{copyTable.artifacts}</button> : null}</div>
    <ul className="m-0 flex list-none flex-wrap gap-[0.4rem] p-0">{artifacts.map((artifact) => {
      const expired = isArtifactExpired(artifact);
      const previewable = artifactPreviewModel(artifact, copyTable).kind !== 'download-only';
      return <li key={artifact.index} className="flex items-center gap-[0.4rem] text-[13px] text-[rgba(0,0,0,0.9)]"><span>{artifact.fileName}{artifact.version ? ` · v${artifact.version}` : ''}</span>{expired ? <small role="status" className="text-[#66758b]">{copyTable.expired}</small> : <>{previewable && onPreview ? <button type="button" className="cursor-pointer rounded-[5px] border border-[#b9d1f2] bg-white px-[0.45rem] py-[0.15rem] text-[12px] text-[#245a9b]" onClick={() => void onPreview(message.id, artifact.index)}>{copyTable.preview}</button> : null}{onDownload ? <button type="button" className="cursor-pointer rounded-[5px] border border-[#b9d1f2] bg-white px-[0.45rem] py-[0.15rem] text-[12px] text-[#245a9b]" onClick={() => void onDownload(message.id, artifact.index)}>{copyTable.download}</button> : <small className="text-[#66758b]">{copyTable.available}</small>}</>}</li>;
    })}</ul>
  </section>;
}

export function MessageList({ copy, messages, pending, onRetry, loadingOlder = false, hasMore = false, onLoadOlder, suggestions, onSuggestionClick, onRefreshSuggestions, onDismissSuggestions, onCitationClick, onBookmark, onForkMessage, canForkMessage, onArtifactDownload, onArtifactPreview, sessionId = null, typingIndicator = false }: MessageListProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const timestampLabels = conversationTimeLabels(t);
  const containerRef = useRef<HTMLDivElement>(null);
  const stickToBottom = useRef(true);
  const previousSessionId = useRef<string | null>(sessionId);
  const previewRequestId = useRef(0);
  const [preview, setPreview] = useState<{ messageId: string; artifact?: ChatArtifact; payload?: ArtifactPreviewPayload; loading: boolean; error?: string } | null>(null);
  const [previewWidth, setPreviewWidth] = useState(ARTIFACT_PREVIEW_DEFAULT_WIDTH);
  const previewWidthRef = useRef(ARTIFACT_PREVIEW_DEFAULT_WIDTH);
  const artifactDrawerRef = useRef<HTMLElement | null>(null);
  const resizeCleanup = useRef<(() => void) | null>(null);
  const [showScrollToBottom, setShowScrollToBottom] = useState(false);
  useEffect(() => {
    previewRequestId.current += 1;
    setPreview(null);
  }, [sessionId]);
  useEffect(() => {
    try {
      const stored = Number(window.localStorage.getItem('weknora-chat-artifact-preview-width'));
      if (Number.isFinite(stored)) {
        const width = clampArtifactPreviewWidth(stored, window.innerWidth);
        previewWidthRef.current = width;
        setPreviewWidth(width);
      }
    } catch {
      // Storage is optional in embedded/webview contexts.
    }
    return () => resizeCleanup.current?.();
  }, []);
  useEffect(() => {
    if (preview) artifactDrawerRef.current?.focus();
  }, [preview?.messageId, preview?.artifact?.index]);
  const previousLayout = useRef<{ firstId?: string; length: number; height: number; top: number }>({ length: 0, height: 0, top: 0 });
  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    if (hasSessionChanged(previousSessionId.current, sessionId)) {
      previousSessionId.current = sessionId;
      stickToBottom.current = true;
      previousLayout.current = { length: 0, height: 0, top: 0 };
      container.scrollTop = 0;
      return;
    }
    const previous = previousLayout.current;
    const firstId = messages[0]?.id;
    const prepended = previous.length > 0 && messages.length > previous.length && firstId !== previous.firstId;
    if (prepended) container.scrollTop = scrollTopAfterPrepend(previous.top, previous.height, container.scrollHeight);
    else if (stickToBottom.current) container.scrollTop = container.scrollHeight;
    previousLayout.current = { firstId, length: messages.length, height: container.scrollHeight, top: container.scrollTop };
  }, [messages.length, messages[0]?.id, messages.at(-1)?.content, pending?.content, pending?.status]);

  useEffect(() => {
    const root = containerRef.current;
    if (!root || typeof window === 'undefined' || !root.querySelector('[data-markdown-diagram="mermaid"]')) return;
    let disposed = false;
    void (async () => {
      if (disposed) return;
      await hydrateMermaidBlocksWithBrowserDefaults(root);
    })().catch(() => {
      // The escaped Mermaid source remains visible when the optional renderer
      // or sanitizer cannot load in a particular WebView/runtime.
    });
    return () => { disposed = true; };
  }, [messages, pending?.content, pending?.status]);

  function onScroll() {
    const container = containerRef.current;
    if (!container) return;
    stickToBottom.current = shouldStickToBottom({ scrollTop: container.scrollTop, scrollHeight: container.scrollHeight, clientHeight: container.clientHeight });
    if (container.scrollTop <= 0 && hasMore && !loadingOlder) onLoadOlder?.();
    setShowScrollToBottom(container.scrollHeight - (container.scrollTop + container.clientHeight) > 200);
  }

  function scrollToBottom(): void {
    const container = containerRef.current;
    if (!container) return;
    const reducedMotion = typeof window !== 'undefined' && typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    container.scrollTo({ top: container.scrollHeight, behavior: reducedMotion ? 'auto' : 'smooth' });
  }

  function onContentClick(event: React.MouseEvent<HTMLDivElement>): void {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    const citation = target.closest<HTMLElement>('[data-citation-id]')?.dataset.citationId;
    if (citation) onCitationClick?.(citation);
  }

  function openArtifactList(messageId: string): void {
    const message = messages.find((item) => item.id === messageId);
    if (!message || messageArtifactItems(message).length === 0) return;
    previewRequestId.current += 1;
    setPreview({ messageId, loading: false });
  }

  async function openArtifactPreview(messageId: string, artifactIndex: number): Promise<void> {
    if (!onArtifactPreview) return;
    const message = messages.find((item) => item.id === messageId);
    const artifact = message ? messageArtifactItems(message).find((item) => item.index === artifactIndex) : undefined;
    if (!artifact) return;
    const requestId = ++previewRequestId.current;
    setPreview({ messageId, artifact, loading: true });
    try {
      const payload = await onArtifactPreview(messageId, artifactIndex);
      if (requestId === previewRequestId.current) setPreview({ messageId, artifact, payload, loading: false });
    } catch (cause) {
      if (requestId === previewRequestId.current) setPreview({ messageId, artifact, loading: false, error: cause instanceof Error ? cause.message : t.chatPreviewFailed });
    }
  }

  function closeArtifactDrawer(): void {
    previewRequestId.current += 1;
    setPreview(null);
  }

  function openArtifactFromList(artifactIndex: number): void {
    if (!preview) return;
    void openArtifactPreview(preview.messageId, artifactIndex);
  }

  function backToArtifactList(): void {
    if (!preview) return;
    previewRequestId.current += 1;
    setPreview({ messageId: preview.messageId, loading: false });
  }

  function startArtifactResize(event: React.PointerEvent<HTMLDivElement>): void {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = previewWidth;
    const move = (moveEvent: PointerEvent) => {
      const width = clampArtifactPreviewWidth(startWidth + startX - moveEvent.clientX, window.innerWidth);
      previewWidthRef.current = width;
      setPreviewWidth(width);
    };
    const end = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', end);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      resizeCleanup.current = null;
      try { window.localStorage.setItem('weknora-chat-artifact-preview-width', String(previewWidthRef.current)); } catch { /* optional */ }
    };
    resizeCleanup.current?.();
    resizeCleanup.current = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', end);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', end, { once: true });
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  const previewMessage = preview ? messages.find((message) => message.id === preview.messageId) : undefined;
  const previewArtifacts = previewMessage ? messageArtifactItems(previewMessage) : [];

  return <div ref={containerRef} className="wk-chat-message-scroll relative mx-auto min-h-0 w-full max-w-[960px] max-h-[62vh] max-[720px]:max-h-[55vh] flex-1 overflow-auto scroll-smooth p-[0.25rem] [scrollbar-width:auto]" onScroll={onScroll}>
    {showScrollToBottom ? <button type="button" className="wk-chat-scroll-bottom sticky bottom-[12px] z-[10] mx-auto mt-[-48px] mb-[12px] block h-[36px] w-[36px] cursor-pointer rounded-full border border-[#e7e7e7] bg-white text-[rgba(0,0,0,0.6)] shadow-[0_2px_8px_rgba(0,0,0,0.1)]" aria-label={t.chatScrollBottom} onClick={scrollToBottom}>↓</button> : null}
    {hasMore ? <button type="button" className="wk-chat-load-older mx-auto mb-[12px] block cursor-pointer rounded-[8px] border border-[#dcdcdc] bg-transparent px-[14px] py-[4px] text-[12px] text-[rgba(0,0,0,0.6)] disabled:cursor-not-allowed disabled:opacity-60" disabled={loadingOlder} onClick={onLoadOlder}>{loadingOlder ? t.loadingHistory : t.loadOlder}</button> : null}
    <ol className="wk-chat-messages m-0 flex list-none flex-col gap-[16px] p-0" aria-label={t.messagesLabel}>
    {messages.map((message, index) => {
      const isAssistant = message.role === 'assistant';
      const showSeparator = shouldShowConversationTimestamp(messages, index);
      return <Fragment key={message.id}>
        {showSeparator ? <li className="wk-chat-timestamp block list-none select-none border-b border-[#edf0f5] px-0 py-[0.8rem] text-center text-[12px] leading-[20px] text-[rgba(0,0,0,0.26)] tabular-nums" role="separator">{formatConversationTimestampLabel(message.created_at, timestampLabels)}</li> : null}
        <li data-role={message.role} className={isAssistant ? 'wk-chat-message-row wk-chat-message-row--assistant flex w-full flex-col border-b border-[#edf0f5] py-[0.8rem]' : 'wk-chat-message-row wk-chat-message-row--user flex w-full flex-col border-b border-[#edf0f5] py-[0.8rem]'}>
        <div className={isAssistant ? 'wk-chat-message-body flex min-w-0 max-w-full flex-col' : 'wk-chat-message-body flex min-w-0 max-w-full flex-col items-end'}>
          {isAssistant ? <div className="wk-chat-message-content m-0 text-[16px] leading-[1.6] text-[rgba(0,0,0,0.9)] break-words [overflow-wrap:anywhere]" onClick={onContentClick} dangerouslySetInnerHTML={{ __html: renderMessageHtml(message, t.invalidImageLink) }} /> : <div className={`wk-chat-message-bubble ${USER_BUBBLE}`}>{message.content}</div>}
          {!isAssistant && onForkMessage && canForkMessage?.(message.id) === true ? (
            <button type="button" className="mt-[4px] cursor-pointer rounded-[6px] border-0 bg-transparent px-[6px] py-[2px] text-[12px] text-[rgba(0,0,0,0.45)] hover:bg-[#f3f3f3] hover:text-[rgba(0,0,0,0.9)]" title={t.forkFromUserTooltip} aria-label={t.forkFromUserTooltip} onClick={() => onForkMessage(message.id)}>⑂</button>
          ) : null}
          {isAssistant ? <div className="wk-chat-answer-toolbar mt-[6px] ml-[-7px] flex min-h-[30px] items-center justify-start gap-[4px]">
            <CopyAnswerButton copy={t} message={message} />
            <BookmarkAnswerButton copy={t} messageId={message.id} onBookmark={onBookmark} />
            <FallbackInfoButton copy={t} message={message} />
            {onForkMessage && canForkMessage?.(message.id) === true ? (
              <button type="button" className={`${ANSWER_TOOL_BUTTON} wk-chat-fork`} title={t.forkFromAssistantTooltip} aria-label={t.forkFromAssistantTooltip} onClick={() => onForkMessage(message.id)}>
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><circle cx="4" cy="3.5" r="1.6" /><circle cx="12" cy="3.5" r="1.6" /><circle cx="8" cy="12.5" r="1.6" /><path d="M4 5.1v1.2a2.4 2.4 0 0 0 2.4 2.4h3.2A2.4 2.4 0 0 0 12 6.3V5.1" /><path d="M8 8.7v2.2" /></svg>
              </button>
            ) : null}
          </div> : null}
          {isAssistant ? <AssistantExtras copy={t} message={message} /> : null}
          {isAssistant ? <ArtifactList copy={t} message={message} onDownload={onArtifactDownload} onPreview={onArtifactPreview ? openArtifactPreview : undefined} onOpenList={onArtifactPreview || onArtifactDownload ? openArtifactList : undefined} /> : null}
        </div>
      </li>
      </Fragment>;
    })}
    {pending ? <li data-role="user" data-status={pending.status} className="wk-chat-message-row wk-chat-message-row--user flex w-full flex-col border-b border-[#edf0f5] py-[0.8rem]">
      <div className="wk-chat-message-body flex min-w-0 max-w-full flex-col items-end">
        <div className={`wk-chat-message-bubble ${USER_BUBBLE}`}>
          <p className="mt-[0.3rem] mb-0 [overflow-wrap:anywhere]">{pending.content}</p>
          {pending.status === 'pending' ? <p role="status" className="wk-chat-pending-state mt-[0.3rem] mb-0 [overflow-wrap:anywhere] text-[12px] text-[rgba(0,0,0,0.4)]">{t.sending}</p> : <p role="alert" className="wk-chat-pending-state mt-[0.3rem] mb-0 [overflow-wrap:anywhere] text-[12px] text-[rgba(0,0,0,0.4)]">{pending.error ?? t.sendFailed}</p>}
          {pending.status === 'failed' && onRetry ? <button type="button" className="wk-chat-retry mt-[4px] self-end cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[10px] py-[2px] text-[12px] text-[rgba(0,0,0,0.6)]" onClick={onRetry}>{t.retry}</button> : null}
        </div>
      </div>
    </li> : null}
    {typingIndicator ? <TypingIndicator copy={t} /> : null}
    </ol>
    {preview ? <>
      <button type="button" className="wk-chat-artifact-drawer-overlay" aria-label={t.close} onClick={closeArtifactDrawer} />
      <aside
        className="wk-chat-artifact-drawer"
        ref={artifactDrawerRef}
        style={{ width: preview.artifact ? `${previewWidth}px` : '440px' }}
        role="dialog"
        aria-modal="false"
        aria-label={preview.artifact ? `${preview.artifact.fileName} preview` : t.artifacts}
        tabIndex={-1}
        onKeyDown={(event) => { if (event.key === 'Escape') { event.preventDefault(); closeArtifactDrawer(); } }}
      >
        <header className="wk-chat-artifact-drawer-header">
          {preview.artifact ? <button type="button" className="wk-chat-artifact-drawer-back" aria-label={t.artifactPreviewBack} onClick={backToArtifactList}>‹</button> : null}
          <h2>{preview.artifact?.fileName ?? t.artifacts}</h2>
          {preview.artifact && onArtifactDownload ? <button type="button" className="wk-chat-artifact-drawer-action" onClick={() => void onArtifactDownload(preview.messageId, preview.artifact!.index)}>{t.download}</button> : null}
          <button type="button" className="wk-chat-artifact-drawer-close" aria-label={t.close} onClick={closeArtifactDrawer}>×</button>
        </header>
        {preview.artifact ? <div className="wk-chat-artifact-drawer-body"><ArtifactPreview artifact={preview.artifact} payload={preview.payload} loading={preview.loading} error={preview.error} copy={t} showHeader={false} onClose={backToArtifactList} /></div> : <ul className="wk-chat-artifact-drawer-list">{previewArtifacts.map((artifact) => {
          const expired = isArtifactExpired(artifact);
          return <li key={artifact.index} className="wk-chat-artifact-drawer-item">
            <button type="button" className="wk-chat-artifact-drawer-item-main" disabled={expired || !onArtifactPreview} onClick={() => openArtifactFromList(artifact.index)}>
              <span className="wk-chat-artifact-drawer-item-name" title={artifact.fileName}>{artifact.fileName}</span>
              <small>{formatArtifactSize(artifact.fileSize)} · {formatArtifactDateTime(artifact.createdAt ?? artifact.modTime)}{expired ? ` · ${t.expired}` : ''}</small>
            </button>
            {onArtifactDownload && !expired ? <button type="button" className="wk-chat-artifact-drawer-action" aria-label={`${t.download}: ${artifact.fileName}`} onClick={() => void onArtifactDownload(preview.messageId, artifact.index)}>{t.download}</button> : null}
          </li>;
        })}</ul>}
        {preview.artifact ? <div className="wk-chat-artifact-drawer-resize" role="separator" aria-orientation="vertical" aria-label={t.artifacts} onPointerDown={startArtifactResize} /> : null}
      </aside>
    </> : null}
    {suggestions?.status === 'ready' && suggestions.questions.length > 0 ? <section className="wk-chat-suggestions mx-0 my-[1rem] w-full max-w-[960px] rounded-[8px] border border-[#dce3ed] p-[0.8rem]" aria-label={t.followUpQuestions}>
      <div className="wk-chat-suggestions-heading flex items-center justify-between gap-[0.6rem]"><h2 className="mt-[0.35rem] mb-[0.35rem] text-[1rem] font-normal text-[rgba(0,0,0,0.4)]">{t.followUpQuestions}</h2><div><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[10px] py-[2px] text-[12px] text-[rgba(0,0,0,0.6)]" onClick={onRefreshSuggestions} disabled={!suggestions.allow_regenerate}>{t.suggestedRefresh}</button><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[10px] py-[2px] text-[12px] text-[rgba(0,0,0,0.6)]" onClick={onDismissSuggestions}>{t.dismiss}</button></div></div>
      <div className="wk-chat-suggestions-grid mt-[8px] grid grid-cols-[repeat(auto-fit,minmax(12rem,1fr))] gap-[0.5rem]">{suggestions.questions.map((question) => <button type="button" key={question.id} className="cursor-pointer overflow-hidden text-ellipsis whitespace-nowrap rounded-[6px] border border-[#dce3ed] bg-white p-[0.65rem] text-left text-[13px] leading-[1.5] text-[rgba(0,0,0,0.9)] shadow-[0_1px_2px_rgba(0,0,0,0.04)] hover:border-[rgba(0,0,0,0.1)] hover:shadow-[0_2px_6px_rgba(0,0,0,0.05)]" onClick={() => onSuggestionClick?.(question.id, question.text)}>{question.text}{question.source === 'faq' ? <small className="ml-[6px] mt-[0.25rem] block text-[#66758b]">FAQ</small> : null}</button>)}</div>
    </section> : null}
  </div>;
}
