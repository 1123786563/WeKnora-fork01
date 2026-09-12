import { Fragment, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { ChatMessage, MessageSuggestionSet } from '@weknora/contracts';
import { hasSessionChanged, scrollTopAfterPrepend, shouldStickToBottom } from '@weknora/domain/chat/session-state';
import { isArtifactExpired, normalizeArtifactList, type ChatArtifact } from '@weknora/domain/chat/artifacts';
import { assistantMessageExtras } from '@weknora/domain/chat/message-extras';
import {
  formatConversationTimestampLabel,
  formatMessageTimestamp,
  shouldShowConversationTimestamp,
} from '@weknora/domain/chat/message-timestamps';
import { copyAnswerText } from '@weknora/domain/chat/copy-answer';
import { renderChatMarkdown } from './markdown.ts';
import { hydrateMermaidBlocksWithBrowserDefaults } from './mermaid.ts';
import { ArtifactPreview, artifactPreviewModel, type ArtifactPreviewPayload } from './artifact-preview.tsx';

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
}

// TODO(migration): replace with @weknora/i18n chat labels once chat migrates
// off local label maps.
const TIMESTAMP_LABELS = {
  today: 'Today',
  yesterday: 'Yesterday',
  thisYear: (model: { month: number; day: number }) => `${model.month}/${model.day}`,
  otherYear: (model: { year: number; month: number; day: number }) => `${model.year}/${model.month}/${model.day}`,
};

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

function CopyAnswerButton({ message }: { message: ChatMessage }) {
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
  return <button type="button" className={copied ? 'wk-chat-copy is-copied' : 'wk-chat-copy'} onClick={() => void copy()} aria-label={copied ? 'Copied' : 'Copy answer'}>
    {copied ? 'Copied' : 'Copy'}
  </button>;
}

function TypingIndicator() {
  return <li className="wk-chat-typing" role="status" aria-label="Assistant is typing">
    <span className="wk-chat-avatar" aria-hidden="true">AI</span>
    <span className="wk-chat-typing-dots" aria-hidden="true"><i /><i /><i /></span>
  </li>;
}

function AssistantExtras(props: { message: ChatMessage }) {
  const extras = assistantMessageExtras(props.message);
  if (!extras.thinking && extras.toolCalls.length === 0) return null;
  return <details className='wk-chat-message-extras'>
    <summary>Thinking &amp; tools</summary>
    {extras.thinking ? <pre>{extras.thinking}</pre> : null}
    {extras.toolCalls.length > 0 ? <ul className='wk-list'>{extras.toolCalls.map((call) => <li key={call.id}><strong>{call.name ?? call.id}</strong><small>{call.status}</small></li>)}</ul> : null}
  </details>;
}

function ArtifactList({ message, onDownload, onPreview }: { message: ChatMessage; onDownload?: MessageListProps['onArtifactDownload']; onPreview?: (messageId: string, artifactIndex: number) => void | Promise<void> }) {
  const artifacts = messageArtifactItems(message);
  if (artifacts.length === 0) return null;
  return <section className="wk-chat-artifacts" aria-label="Artifacts">
    <h3>Artifacts</h3>
    <ul>{artifacts.map((artifact) => {
      const expired = isArtifactExpired(artifact);
      const previewable = artifactPreviewModel(artifact).kind !== 'download-only';
      return <li key={artifact.index}><span>{artifact.fileName}{artifact.version ? ` · v${artifact.version}` : ''}</span>{expired ? <small role="status">Expired</small> : <>{previewable && onPreview ? <button type="button" onClick={() => void onPreview(message.id, artifact.index)}>Preview</button> : null}{onDownload ? <button type="button" onClick={() => void onDownload(message.id, artifact.index)}>Download</button> : <small>Available</small>}</>}</li>;
    })}</ul>
  </section>;
}

export function MessageList({ messages, pending, onRetry, loadingOlder = false, hasMore = false, onLoadOlder, suggestions, onSuggestionClick, onRefreshSuggestions, onDismissSuggestions, onCitationClick, onArtifactDownload, onArtifactPreview, sessionId = null, typingIndicator = false }: MessageListProps) {
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
    {hasMore ? <button type="button" disabled={loadingOlder} onClick={onLoadOlder}>{loadingOlder ? 'Loading history…' : 'Load older messages'}</button> : null}
    <ol className="wk-chat-messages" aria-label="Messages">
    {messages.map((message, index) => {
      const isAssistant = message.role === 'assistant';
      const showSeparator = shouldShowConversationTimestamp(messages, index);
      return <Fragment key={message.id}>
        {showSeparator ? <li className="wk-chat-timestamp" role="separator" aria-label={formatConversationTimestampLabel(message.created_at, TIMESTAMP_LABELS)}>{formatConversationTimestampLabel(message.created_at, TIMESTAMP_LABELS)}</li> : null}
        <li data-role={message.role} className={`wk-chat-message-row wk-chat-message-row--${message.role}`}>
        {isAssistant ? <span className="wk-chat-avatar" aria-hidden="true">AI</span> : null}
        <div className="wk-chat-message-body">
          <strong className="wk-chat-message-role">{message.role}</strong>
          <div className="wk-chat-message-bubble">
            <div className="wk-chat-message-content" onClick={onContentClick} dangerouslySetInnerHTML={{ __html: renderMessageHtml(message) }} />
            <span className="wk-chat-message-time">{formatMessageTimestamp(message.created_at)}</span>
          </div>
          {isAssistant ? <CopyAnswerButton message={message} /> : null}
          {isAssistant ? <AssistantExtras message={message} /> : null}
          <ArtifactList message={message} onDownload={onArtifactDownload} onPreview={onArtifactPreview ? openArtifactPreview : undefined} />
        </div>
      </li>
      </Fragment>;
    })}
    {pending ? <li data-role="user" data-status={pending.status} className="wk-chat-message-row wk-chat-message-row--user">
      <div className="wk-chat-message-body">
        <strong className="wk-chat-message-role">user</strong>
        <div className="wk-chat-message-bubble">
          <p>{pending.content}</p>
          {pending.status === 'pending' ? <p role="status">Sending…</p> : <p role="alert">{pending.error ?? 'Message failed to send'}</p>}
          {pending.status === 'failed' && onRetry ? <button type="button" onClick={onRetry}>Retry</button> : null}
        </div>
      </div>
    </li> : null}
    {typingIndicator ? <TypingIndicator /> : null}
    </ol>
    {preview ? <ArtifactPreview artifact={preview.artifact} payload={preview.payload} loading={preview.loading} error={preview.error} onClose={() => { previewRequestId.current += 1; setPreview(null); }} onDownload={onArtifactDownload ? () => void onArtifactDownload(preview.messageId, preview.artifact.index) : undefined} /> : null}
    {suggestions?.status === 'ready' && suggestions.questions.length > 0 ? <section className="wk-chat-suggestions" aria-label="Suggested questions">
      <div className="wk-chat-suggestions-heading"><h2>Suggested questions</h2><div><button type="button" onClick={onRefreshSuggestions} disabled={!suggestions.allow_regenerate}>Refresh</button><button type="button" onClick={onDismissSuggestions}>Dismiss</button></div></div>
      <div className="wk-chat-suggestions-grid">{suggestions.questions.map((question) => <button type="button" key={question.id} onClick={() => onSuggestionClick?.(question.id, question.text)}>{question.text}{question.source === 'faq' ? <small>FAQ</small> : null}</button>)}</div>
    </section> : null}
  </div>;
}