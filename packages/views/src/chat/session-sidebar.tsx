import type { ChatSession } from '@weknora/contracts';

export interface SessionSidebarProps {
  sessions: readonly ChatSession[];
  selectedSessionId: string | null;
  loading?: boolean;
  onSelect(sessionId: string): void;
  onCreate(): void;
}

export function SessionSidebar({ sessions, selectedSessionId, loading = false, onSelect, onCreate }: SessionSidebarProps) {
  return <aside className="wk-chat-sidebar" aria-label="Sessions">
    <button type="button" onClick={onCreate}>New chat</button>
    {loading ? <p role="status">Loading sessions…</p> : null}
    <ul>
      {sessions.map((session) => <li key={session.id}>
        <button
          type="button"
          aria-current={session.id === selectedSessionId ? 'page' : undefined}
          onClick={() => onSelect(session.id)}
        >
          {session.is_pinned ? '★ ' : ''}{session.title || 'Untitled chat'}
        </button>
      </li>)}
    </ul>
  </aside>;
}
