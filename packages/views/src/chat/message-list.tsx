import type { ChatMessage } from '@weknora/contracts';

export interface PendingChatMessage {
  content: string;
  status: 'pending' | 'failed';
  error?: string;
}

export interface MessageListProps {
  messages: readonly ChatMessage[];
  pending?: PendingChatMessage;
  onRetry?(): void;
}

export function MessageList({ messages, pending, onRetry }: MessageListProps) {
  return <ol className="wk-chat-messages" aria-label="Messages">
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
  </ol>;
}
