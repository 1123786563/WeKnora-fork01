import { useLayoutEffect, useRef } from 'react';
import type { ChatMessage, MessageSuggestionSet } from '@weknora/contracts';
import { hasSessionChanged, scrollTopAfterPrepend, shouldStickToBottom } from '@weknora/domain/chat/session-state';

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
  sessionId?: string | null;
}

export function MessageList({ messages, pending, onRetry, loadingOlder = false, hasMore = false, onLoadOlder, suggestions, onSuggestionClick, onRefreshSuggestions, onDismissSuggestions, sessionId = null }: MessageListProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const stickToBottom = useRef(true);
  const previousSessionId = useRef<string | null>(sessionId);
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

  function onScroll() {
    const container = containerRef.current;
    if (!container) return;
    stickToBottom.current = shouldStickToBottom({ scrollTop: container.scrollTop, scrollHeight: container.scrollHeight, clientHeight: container.clientHeight });
    if (container.scrollTop <= 0 && hasMore && !loadingOlder) onLoadOlder?.();
  }

  return <div ref={containerRef} className="wk-chat-message-scroll" onScroll={onScroll}>
    {hasMore ? <button type="button" disabled={loadingOlder} onClick={onLoadOlder}>{loadingOlder ? 'Loading history…' : 'Load older messages'}</button> : null}
    <ol className="wk-chat-messages" aria-label="Messages">
    {messages.map((message) => <li key={message.id} data-role={message.role}>
      <strong>{message.role}</strong>
      <p>{message.content}</p>
    </li>)}
    {pending ? <li data-role="user" data-status={pending.status}>
      <strong>user</strong>
      <p>{pending.content}</p>
      {pending.status === 'pending' ? <p role="status">Sending…</p> : <p role="alert">{pending.error ?? 'Message failed to send'}</p>}
      {pending.status === 'failed' && onRetry ? <button type="button" onClick={onRetry}>Retry</button> : null}
    </li> : null}
    </ol>
    {suggestions?.status === 'ready' && suggestions.questions.length > 0 ? <section className="wk-chat-suggestions" aria-label="Suggested questions">
      <div className="wk-chat-suggestions-heading"><h2>Suggested questions</h2><div><button type="button" onClick={onRefreshSuggestions} disabled={!suggestions.allow_regenerate}>Refresh</button><button type="button" onClick={onDismissSuggestions}>Dismiss</button></div></div>
      <div className="wk-chat-suggestions-grid">{suggestions.questions.map((question) => <button type="button" key={question.id} onClick={() => onSuggestionClick?.(question.id, question.text)}>{question.text}{question.source === 'faq' ? <small>FAQ</small> : null}</button>)}</div>
    </section> : null}
  </div>;
}
