import type { ChatSession } from '@weknora/contracts';

export interface SessionGroupView {
  key: string;
  label?: string;
  items: readonly ChatSession[];
}

export interface SessionSourceOption {
  value: string;
  label: string;
}

export interface SessionSidebarProps {
  sessions: readonly ChatSession[];
  selectedSessionId: string | null;
  loading?: boolean;
  onSelect(sessionId: string): void;
  onCreate(): void;
  onRename?(sessionId: string): Promise<void>;
  onTogglePin?(sessionId: string, pinned: boolean): Promise<void>;
  onDelete?(sessionId: string): Promise<void>;
  groups?: readonly SessionGroupView[];
  source?: string;
  sourceOptions?: readonly SessionSourceOption[];
  onSourceChange?(source: string): void;
  groupMode?: 'none' | 'date';
  onGroupModeChange?(mode: 'none' | 'date'): void;
  keyword?: string;
  onKeywordChange?(keyword: string): void;
}

export function SessionSidebar({ sessions, selectedSessionId, loading = false, onSelect, onCreate, onRename, onTogglePin, onDelete, groups, source, sourceOptions, onSourceChange, groupMode, onGroupModeChange, keyword, onKeywordChange }: SessionSidebarProps) {
  const visibleGroups = groups ?? [{ key: 'all', items: sessions }];
  return <aside className="wk-chat-sidebar" aria-label="Sessions">
    <div className="wk-chat-sidebar-heading"><h2>Conversations</h2><button type="button" onClick={onCreate}>New chat</button></div>
    {sourceOptions && onSourceChange ? <label>Source<select value={source ?? ''} onChange={(event) => onSourceChange(event.target.value)}>{sourceOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label> : null}
    {onKeywordChange ? <label>Search<input value={keyword ?? ''} onChange={(event) => onKeywordChange(event.target.value)} placeholder="Search conversations" /></label> : null}
    {onGroupModeChange ? <label>Group<select value={groupMode ?? 'none'} onChange={(event) => onGroupModeChange(event.target.value === 'date' ? 'date' : 'none')}><option value="none">All</option><option value="date">By date</option></select></label> : null}
    {loading ? <p role="status">Loading sessions…</p> : null}
    {visibleGroups.map((group) => <section key={group.key} className="wk-chat-session-group">
      {group.label ? <h3>{group.label}</h3> : null}
      <ul>{group.items.map((session) => <li key={session.id}>
        <div className="wk-chat-session-row"><button type="button" aria-current={session.id === selectedSessionId ? 'page' : undefined} onClick={() => onSelect(session.id)}>{session.is_pinned ? '★ ' : ''}{session.title || 'Untitled chat'}</button>{onRename ? <button type="button" aria-label={`Rename ${session.id}`} onClick={() => void onRename(session.id)}>Rename</button> : null}{onTogglePin ? <button type="button" aria-label={`${session.is_pinned ? 'Unpin' : 'Pin'} ${session.id}`} onClick={() => void onTogglePin(session.id, !session.is_pinned)}>{session.is_pinned ? 'Unpin' : 'Pin'}</button> : null}{onDelete ? <button type="button" aria-label={`Delete ${session.id}`} onClick={() => void onDelete(session.id)}>Delete</button> : null}</div>
      </li>)}</ul>
    </section>)}
  </aside>;
}
