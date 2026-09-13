import type { ChatSession } from '@weknora/contracts';
import { sessionSourceBadge } from '@weknora/domain/chat/session-grouping';
import { chatCopy, sessionGroupLabel } from './chat-copy.ts';

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
  page?: number;
  pageCount?: number;
  onPageChange?(page: number): void;
}

/*
 * Vue session list anatomy (platform sidebar conversation area): time group
 * headers, full-width session titles, green-tinted active row, hover ⋯ menu.
 * The Vue list lives in the platform sidebar; React keeps its own in-page
 * sidebar until shell integration lands (recorded as an open item).
 */
export function SessionSidebar({ sessions, selectedSessionId, loading = false, onSelect, onCreate, onRename, onTogglePin, onDelete, groups, source, sourceOptions, onSourceChange, groupMode, onGroupModeChange, keyword, onKeywordChange, page = 1, pageCount = 1, onPageChange }: SessionSidebarProps) {
  const visibleGroups = groups ?? [{ key: 'all', items: sessions }];
  return <aside className="wk-chat-sidebar" aria-label={chatCopy('sidebarTitle')}>
    <div className="wk-chat-sidebar-heading">
      <button type="button" className="wk-chat-new-chat" onClick={onCreate}>
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" aria-hidden="true"><path d="M8 3v10M3 8h10" /></svg>
        <span>{chatCopy('newChat')}</span>
      </button>
    </div>
    {sourceOptions && onSourceChange ? <label className="wk-chat-sidebar-filter">{chatCopy('sourceLabel')}<select value={source ?? ''} onChange={(event) => onSourceChange(event.target.value)}>{sourceOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label> : null}
    {onKeywordChange ? <label className="wk-chat-sidebar-filter">{chatCopy('searchSessions')}<input value={keyword ?? ''} onChange={(event) => onKeywordChange(event.target.value)} placeholder={chatCopy('searchSessions')} /></label> : null}
    {onGroupModeChange ? <label className="wk-chat-sidebar-filter">{chatCopy('groupLabel')}<select value={groupMode ?? 'none'} onChange={(event) => onGroupModeChange(event.target.value === 'date' ? 'date' : 'none')}><option value="none">{chatCopy('groupAll')}</option><option value="date">{chatCopy('groupByDate')}</option></select></label> : null}
    {loading ? <p role="status">{chatCopy('loadingSessions')}</p> : null}
    {visibleGroups.map((group) => <section key={group.key} className="wk-chat-session-group">
      {group.label ? <h3>{sessionGroupLabel(group.label)}</h3> : null}
      <ul>{group.items.map((session) => {
        const badge = sessionSourceBadge(session);
        const active = session.id === selectedSessionId;
        return <li key={session.id} className={active ? 'wk-chat-session-item is-active' : 'wk-chat-session-item'}>
          <button type="button" className="wk-chat-session-title" aria-current={active ? 'page' : undefined} onClick={() => onSelect(session.id)}>
            {session.is_pinned ? <span className="wk-chat-session-pin" aria-hidden="true">★</span> : null}
            <span className="wk-chat-session-title-text">{session.title || chatCopy('untitledChat')}</span>
            {badge.kind ? <span className={`wk-chat-session-source ${badge.kind}`} title="Session source">{badge.label}</span> : null}
          </button>
          {onRename || onTogglePin || onDelete ? <details className="wk-chat-session-menu">
            <summary aria-label={chatCopy('moreActions')} title={chatCopy('moreActions')}>
              <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><circle cx="8" cy="3" r="1.4" /><circle cx="8" cy="8" r="1.4" /><circle cx="8" cy="13" r="1.4" /></svg>
            </summary>
            <div className="wk-chat-session-menu-list" role="menu">
              {onTogglePin ? <button type="button" role="menuitem" onClick={() => void onTogglePin(session.id, !session.is_pinned)}>{session.is_pinned ? chatCopy('unpin') : chatCopy('pin')}</button> : null}
              {onRename ? <button type="button" role="menuitem" onClick={() => void onRename(session.id)}>{chatCopy('renameSession')}</button> : null}
              {onDelete ? <button type="button" role="menuitem" className="is-danger" onClick={() => void onDelete(session.id)}>{chatCopy('deleteSession')}</button> : null}
            </div>
          </details> : null}
        </li>;
      })}</ul>
    </section>)}
    {onPageChange && pageCount > 1 ? <nav className="wk-chat-session-pagination" aria-label="Conversation pages"><button type="button" disabled={page <= 1 || loading} onClick={() => onPageChange(Math.max(1, page - 1))}>{chatCopy('previous')}</button><span>{chatCopy('pageOf', { page, total: pageCount })}</span><button type="button" disabled={page >= pageCount || loading} onClick={() => onPageChange(Math.min(pageCount, page + 1))}>{chatCopy('next')}</button></nav> : null}
  </aside>;
}
