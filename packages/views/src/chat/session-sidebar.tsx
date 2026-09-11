import type { ChatSession } from '@weknora/contracts';

export interface SessionSidebarProps {
  sessions: readonly ChatSession[];
  selectedSessionId: string | null;
  loading?: boolean;
  onSelect(sessionId: string): void;
  onCreate(): void;
  onRename?(sessionId: string): Promise<void>;
  onTogglePin?(sessionId: string, pinned: boolean): Promise<void>;
  onDelete?(sessionId: string): Promise<void>;
}

export function SessionSidebar({ sessions, selectedSessionId, loading = false, onSelect, onCreate, onRename, onTogglePin, onDelete }: SessionSidebarProps) {
  return <aside className="wk-chat-sidebar" aria-label="Sessions">
    <button type="button" onClick={onCreate}>New chat</button>
    {loading ? <p role="status">Loading sessions…</p> : null}
    <ul>
      {sessions.map((session) => <li key={session.id}>
        <div className="wk-chat-session-row"><button type="button" aria-current={session.id === selectedSessionId ? 'page' : undefined} onClick={() => onSelect(session.id)}>{session.is_pinned ? '★ ' : ''}{session.title || 'Untitled chat'}</button>{onRename ? <button type="button" aria-label={`Rename ${session.id}`} onClick={() => void onRename(session.id)}>Rename</button> : null}{onTogglePin ? <button type="button" aria-label={`${session.is_pinned ? 'Unpin' : 'Pin'} ${session.id}`} onClick={() => void onTogglePin(session.id, !session.is_pinned)}>{session.is_pinned ? 'Unpin' : 'Pin'}</button> : null}{onDelete ? <button type="button" aria-label={`Delete ${session.id}`} onClick={() => void onDelete(session.id)}>Delete</button> : null}</div>
      </li>)}
    </ul>
  </aside>;
}
