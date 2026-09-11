import { useState } from 'react';
import type { ChatMessage, ChatSession } from '@weknora/contracts';
import { ChatComposer, type ChatSubmission } from './composer.tsx';
import { MessageList, type PendingChatMessage } from './message-list.tsx';
import { SessionSidebar } from './session-sidebar.tsx';

export interface ChatAgentOption {
  id: string;
  name: string;
  disabled?: boolean;
}

export interface ChatPageProps {
  sessions: readonly ChatSession[];
  selectedSessionId: string | null;
  messages: readonly ChatMessage[];
  draft: string;
  loadingSessions?: boolean;
  loadingMessages?: boolean;
  error?: string;
  onSelectSession(sessionId: string): void;
  onCreateSession(): void;
  onDraftChange(value: string): void;
  send(submission: ChatSubmission): Promise<void>;
  agents?: readonly ChatAgentOption[];
  selectedAgentId?: string;
  onAgentChange?(agentId: string): void;
}

export function ChatPage(props: ChatPageProps) {
  const [pending, setPending] = useState<PendingChatMessage | undefined>();

  async function send(submission: ChatSubmission) {
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
    }
  }

  return <main className="wk-chat-page">
    <SessionSidebar
      sessions={props.sessions}
      selectedSessionId={props.selectedSessionId}
      loading={props.loadingSessions}
      onSelect={props.onSelectSession}
      onCreate={props.onCreateSession}
    />
    <section className="wk-chat-main" aria-label="Chat">
      <h1>{props.selectedSessionId ? 'Conversation' : 'New conversation'}</h1>
      {props.agents && props.onAgentChange ? <label htmlFor="wk-chat-agent">Agent<select id="wk-chat-agent" value={props.selectedAgentId ?? ''} onChange={(event) => props.onAgentChange?.(event.target.value)}><option value="">Knowledge chat</option>{props.agents.map((agent) => <option key={agent.id} value={agent.id} disabled={agent.disabled}>{agent.name}{agent.disabled ? ' · disabled' : ''}</option>)}</select></label> : null}
      {props.error ? <p role="alert">{props.error}</p> : null}
      {props.loadingMessages ? <p role="status">Loading messages…</p> : null}
      <MessageList messages={props.messages} pending={pending} onRetry={pending?.status === 'failed' ? () => void send({ content: pending.content, status: 'pending' }) : undefined} />
      <ChatComposer draft={props.draft} onDraftChange={props.onDraftChange} onSubmit={(submission) => void send(submission)} />
    </section>
  </main>;
}
