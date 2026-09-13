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
import { ArtifactPreview, artifactPreviewModel, type ArtifactPreviewPayload } from './artifact-preview.tsx';
import { conversationTimeLabels, resolveChatCopy, resolveChatLocale, type ChatCopyTable } from './chat-copy.ts';

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

export function renderMessageHtml(message: Pick<ChatMessage, 'content'>): string {
  return renderChatMarkdown(message.content);
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
  return <button type="button" className={copied ? 'wk-chat-copy is-copied' : 'wk-chat-copy'} onClick={() => void copy()} aria-label={copied ? copyTable.copied : copyTable.copy} title={copied ? copyTable.copied : copyTable.copy}>
    <CopyIcon />
  </button>;
}

function CopyIcon() {
  return <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
    <path d="M10.5 5.5v-2a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2" />
  </svg>;
}

function BookmarkAnswerButton({ copy: copyTable }: { copy: ChatCopyTable }) {
  // Vue botmsg.vue adds the answer to the knowledge manual editor; the React
  // shell has no manual-editor surface yet, so the icon renders disabled.
  return <button type="button" className="wk-chat-bookmark" aria-label={copyTable.addToKnowledgeBase} title={copyTable.addToKnowledgeBase} aria-disabled="true">
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 2.5h8a1 1 0 0 1 1 1V14l-5-2.6L3 14V3.5a1 1 0 0 1 1-1Z" />
      <path d="M8 5.5v4M6 7.5h4" />
    </svg>
  </button>;
}

function FallbackInfoButton({ copy: copyTable, message }: { copy: ChatCopyTable; message: ChatMessage }) {
  const fallback = (message as Record<string, unknown>).is_fallback === true;
  if (!fallback) return null;
  return <button type="button" className="wk-chat-request-info" aria-label={copyTable.fallbackHint} title={copyTable.fallbackHint}>
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" aria-hidden="true">
      <circle cx="8" cy="8" r="6.2" />
      <path d="M8 7.2v3.4" />
      <circle cx="8" cy="5" r="0.7" fill="currentColor" stroke="none" />
    </svg>
  </button>;
}

function TypingIndicator({ copy: copyTable }: { copy: ChatCopyTable }) {
  return <li className="wk-chat-typing" role="status" aria-label={copyTable.thinkingAlt}>
    <span className="wk-chat-typing-dots" aria-hidden="true"><i /><i /><i /></span>
  </li>;
}

function AssistantExtras(props: { copy: ChatCopyTable; message: ChatMessage }) {
  const extras = assistantMessageExtras(props.message);
  if (!extras.thinking && extras.toolCalls.length === 0) return null;
  return <details className='wk-chat-message-extras'>
    <summary>{props.copy.thinkingAndTools}</summary>
    {extras.thinking ? <pre>{extras.thinking}</pre> : null}
    {extras.toolCalls.length > 0 ? <ul className='wk-list'>{extras.toolCalls.map((call) => <li key={call.id}><strong>{call.name ?? call.id}</strong><small>{call.status}</small></li>)}</ul> : null}
  </details>;
}

function ArtifactList({ copy: copyTable, message, onDownload, onPreview }: { copy: ChatCopyTable; message: ChatMessage; onDownload?: MessageListProps['onArtifactDownload']; onPreview?: (messageId: string, artifactIndex: number) => void | Promise<void> }) {
  const artifacts = messageArtifactItems(message);
  if (artifacts.length === 0) return null;
  return <section className="wk-chat-artifacts" aria-label={copyTable.artifacts}>
    <h3>{copyTable.artifacts}</h3>
    <ul>{artifacts.map((artifact) => {
      const expired = isArtifactExpired(artifact);
      const previewable = artifactPreviewModel(artifact).kind !== 'download-only';
      return <li key={artifact.index}><span>{artifact.fileName}{artifact.version ? ` · v${artifact.version}` : ''}</span>{expired ? <small role="status">{copyTable.expired}</small> : <>{previewable && onPreview ? <button type="button" onClick={() => void onPreview(message.id, artifact.index)}>{copyTable.preview}</button> : null}{onDownload ? <button type="button" onClick={() => void onDownload(message.id, artifact.index)}>{copyTable.download}</button> : <small>{copyTable.available}</small>}</>}</li>;
    })}</ul>
  </section>;
}

export function MessageList({ copy, messages, pending, onRetry, loadingOlder = false, hasMore = false, onLoadOlder, suggestions, onSuggestionClick, onRefreshSuggestions, onDismissSuggestions, onCitationClick, onArtifactDownload, onArtifactPreview, sessionId = null, typingIndicator = false }: MessageListProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const timestampLabels = conversationTimeLabels(t);
  const containerRef = useRef<HTMLDivElement>(null);
  const stickToBottom = useRef(true);
  const previousSessionId = useRef<string | null>(sessionId);
  const previewRequestId = useRef(0);
  const [preview, setPreview] = useState<{ messageId: string; artifact: ChatArtifact; payload?: ArtifactPreviewPayload; loading: boolean; error?: string } | null>(null);
  const [showScrollToBottom, setShowScrollToBottom] = useState(false);
  useEffect(() => {
    previewRequestId.current += 1;
    setPreview(null);
  }, [sessionId]);
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
      if (requestId === previewRequestId.current) setPreview({ messageId, artifact, loading: false, error: cause instanceof Error ? cause.message : 'Unable to load artifact preview' });
    }
  }

  return <div ref={containerRef} className="wk-chat-message-scroll" onScroll={onScroll}>
    {showScrollToBottom ? <button type="button" className="wk-chat-scroll-bottom" aria-label="Scroll to bottom" onClick={scrollToBottom}>↓</button> : null}
    {hasMore ? <button type="button" className="wk-chat-load-older" disabled={loadingOlder} onClick={onLoadOlder}>{loadingOlder ? t.loadingHistory : t.loadOlder}</button> : null}
    <ol className="wk-chat-messages" aria-label="Messages">
    {messages.map((message, index) => {
      const isAssistant = message.role === 'assistant';
      const showSeparator = shouldShowConversationTimestamp(messages, index);
      return <Fragment key={message.id}>
        {showSeparator ? <li className="wk-chat-timestamp" role="separator">{formatConversationTimestampLabel(message.created_at, timestampLabels)}</li> : null}
        <li data-role={message.role} className={isAssistant ? 'wk-chat-message-row wk-chat-message-row--assistant' : 'wk-chat-message-row wk-chat-message-row--user'}>
        <div className="wk-chat-message-body">
          {isAssistant ? <div className="wk-chat-message-content" onClick={onContentClick} dangerouslySetInnerHTML={{ __html: renderMessageHtml(message) }} /> : <div className="wk-chat-message-bubble">{message.content}</div>}
          {isAssistant ? <div className="wk-chat-answer-toolbar">
            <CopyAnswerButton copy={t} message={message} />
            <BookmarkAnswerButton copy={t} />
            <FallbackInfoButton copy={t} message={message} />
          </div> : null}
          {isAssistant ? <AssistantExtras copy={t} message={message} /> : null}
          {isAssistant ? <ArtifactList copy={t} message={message} onDownload={onArtifactDownload} onPreview={onArtifactPreview ? openArtifactPreview : undefined} /> : null}
        </div>
      </li>
      </Fragment>;
    })}
    {pending ? <li data-role="user" data-status={pending.status} className="wk-chat-message-row wk-chat-message-row--user">
      <div className="wk-chat-message-body">
        <div className="wk-chat-message-bubble">
          <p>{pending.content}</p>
          {pending.status === 'pending' ? <p role="status" className="wk-chat-pending-state">{t.sending}</p> : <p role="alert" className="wk-chat-pending-state">{pending.error ?? t.sendFailed}</p>}
          {pending.status === 'failed' && onRetry ? <button type="button" className="wk-chat-retry" onClick={onRetry}>{t.retry}</button> : null}
        </div>
      </div>
    </li> : null}
    {typingIndicator ? <TypingIndicator copy={t} /> : null}
    </ol>
    {preview ? <ArtifactPreview artifact={preview.artifact} payload={preview.payload} loading={preview.loading} error={preview.error} onClose={() => { previewRequestId.current += 1; setPreview(null); }} onDownload={onArtifactDownload ? () => void onArtifactDownload(preview.messageId, preview.artifact.index) : undefined} /> : null}
    {suggestions?.status === 'ready' && suggestions.questions.length > 0 ? <section className="wk-chat-suggestions" aria-label={t.followUpQuestions}>
      <div className="wk-chat-suggestions-heading"><h2>{t.followUpQuestions}</h2><div><button type="button" onClick={onRefreshSuggestions} disabled={!suggestions.allow_regenerate}>{t.suggestedRefresh}</button><button type="button" onClick={onDismissSuggestions}>{t.dismiss}</button></div></div>
      <div className="wk-chat-suggestions-grid">{suggestions.questions.map((question) => <button type="button" key={question.id} onClick={() => onSuggestionClick?.(question.id, question.text)}>{question.text}{question.source === 'faq' ? <small>FAQ</small> : null}</button>)}</div>
    </section> : null}
  </div>;
}
