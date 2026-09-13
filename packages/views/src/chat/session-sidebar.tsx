import { createContext, useContext } from 'react';
import type { ChatSession } from '@weknora/contracts';
import { sessionSourceBadge } from '@weknora/domain/chat/session-grouping';
import { formatChatCopy, resolveChatCopy, resolveChatLocale, sessionGroupLabel, type ChatCopyTable } from './chat-copy.ts';

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
  /** Resolved copy (chat-copy.ts); defaults to the app locale convention. */
  copy?: ChatCopyTable;
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
 *
 * Vue mounts the list inside the platform sidebar (frontend/src/components/
 * menu.vue .submenu) so it is visible on every protected page, and the chat
 * view (frontend/src/views/chat/index.vue) has no sidebar of its own. The
 * platform shell therefore renders <SessionSidebarList> itself and marks the
 * context below: a SessionSidebar mounted under it renders nothing instead of
 * duplicating the list.
 */
export const SessionSidebarShellContext = createContext(false);

export interface SessionSidebarListProps {
  /** Resolved copy (chat-copy.ts); defaults to the app locale convention. */
  copy?: ChatCopyTable;
  /** Flat fallback list; ignored when groups are provided. */
  sessions?: readonly ChatSession[];
  groups?: readonly SessionGroupView[];
  selectedSessionId: string | null;
  loading?: boolean;
  /** Empty-state copy (Vue menu.noSessions); omitted renders nothing. */
  emptyLabel?: string;
  /** Fallback row title (Vue mapSessionRow uses menu.newSession = 新会话). */
  untitledLabel?: string;
  onSelect(sessionId: string): void;
  onRename?(sessionId: string): Promise<void> | void;
  onTogglePin?(sessionId: string, pinned: boolean): Promise<void> | void;
  /** 清空消息 (Vue menu.vue row menu → clearSession); confirm is the caller's. */
  onClear?(sessionId: string): Promise<void> | void;
  onDelete?(sessionId: string): Promise<void> | void;
}

/*
 * The grouped list body shared by the in-page chat sidebar and the platform
 * shell sidebar: time group headers (已置顶/今天/昨天/近7天/近30天/更早), full
 * titles, green active row, hover ⋯ menu (置顶/重命名会话/清空消息/删除会话).
 */
export function SessionSidebarList({ copy, sessions, groups, selectedSessionId, loading = false, emptyLabel, untitledLabel, onSelect, onRename, onTogglePin, onClear, onDelete }: SessionSidebarListProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const visibleGroups = groups ?? [{ key: 'all', items: sessions ?? [] }];
  const hasMenu = Boolean(onRename || onTogglePin || onClear || onDelete);
  const totalItems = visibleGroups.reduce((count, group) => count + group.items.length, 0);
  return <>
    {loading ? <p role="status">{t.loadingSessions}</p> : null}
    {!loading && totalItems === 0 && emptyLabel ? <p className="wk-chat-sidebar-empty" role="status">{emptyLabel}</p> : null}
    {visibleGroups.map((group) => <section key={group.key} className="wk-chat-session-group">
      {group.label ? <h3>{sessionGroupLabel(t, group.label)}</h3> : null}
      <ul>{group.items.map((session) => {
        const badge = sessionSourceBadge(session);
        const active = session.id === selectedSessionId;
        return <li key={session.id} className={active ? 'wk-chat-session-item is-active' : 'wk-chat-session-item'}>
          <button type="button" className="wk-chat-session-title" aria-current={active ? 'page' : undefined} onClick={() => onSelect(session.id)}>
            {session.is_pinned ? <span className="wk-chat-session-pin" aria-hidden="true">★</span> : null}
            <span className="wk-chat-session-title-text">{session.title || untitledLabel || t.untitledChat}</span>
            {badge.kind ? <span className={`wk-chat-session-source ${badge.kind}`} title="Session source">{badge.label}</span> : null}
          </button>
          {hasMenu ? <details className="wk-chat-session-menu">
            <summary aria-label={t.moreActions} title={t.moreActions}>
              <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><circle cx="8" cy="3" r="1.4" /><circle cx="8" cy="8" r="1.4" /><circle cx="8" cy="13" r="1.4" /></svg>
            </summary>
            <div className="wk-chat-session-menu-list" role="menu">
              {onTogglePin ? <button type="button" role="menuitem" onClick={() => void onTogglePin(session.id, !session.is_pinned)}>{session.is_pinned ? t.unpin : t.pin}</button> : null}
              {onRename ? <button type="button" role="menuitem" onClick={() => void onRename(session.id)}>{t.renameSession}</button> : null}
              {onClear ? <button type="button" role="menuitem" onClick={() => void onClear(session.id)}>{t.clearMessages}</button> : null}
              {onDelete ? <button type="button" role="menuitem" className="is-danger" onClick={() => void onDelete(session.id)}>{t.deleteRecord}</button> : null}
            </div>
          </details> : null}
        </li>;
      })}</ul>
    </section>)}
  </>;
}

export function SessionSidebar({ copy, sessions, selectedSessionId, loading = false, onSelect, onCreate, onRename, onTogglePin, onDelete, groups, source, sourceOptions, onSourceChange, groupMode, onGroupModeChange, keyword, onKeywordChange, page = 1, pageCount = 1, onPageChange }: SessionSidebarProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const shellProvidesSessionList = useContext(SessionSidebarShellContext);
  // The platform shell already renders the grouped list next to the nav
  // (Vue menu.vue); an in-page duplicate would show two lists on chat routes.
  if (shellProvidesSessionList) return null;
  return <aside className="wk-chat-sidebar" aria-label={t.sidebarTitle}>
    <div className="wk-chat-sidebar-heading">
      <button type="button" className="wk-chat-new-chat" onClick={onCreate}>
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" aria-hidden="true"><path d="M8 3v10M3 8h10" /></svg>
        <span>{t.newChat}</span>
      </button>
    </div>
    {sourceOptions && onSourceChange ? <label className="wk-chat-sidebar-filter">{t.sourceLabel}<select value={source ?? ''} onChange={(event) => onSourceChange(event.target.value)}>{sourceOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label> : null}
    {onKeywordChange ? <label className="wk-chat-sidebar-filter">{t.searchSessions}<input value={keyword ?? ''} onChange={(event) => onKeywordChange(event.target.value)} placeholder={t.searchSessions} /></label> : null}
    {onGroupModeChange ? <label className="wk-chat-sidebar-filter">{t.groupLabel}<select value={groupMode ?? 'none'} onChange={(event) => onGroupModeChange(event.target.value === 'date' ? 'date' : 'none')}><option value="none">{t.groupAll}</option><option value="date">{t.groupByDate}</option></select></label> : null}
    <SessionSidebarList
      copy={t}
      sessions={sessions}
      groups={groups}
      selectedSessionId={selectedSessionId}
      loading={loading}
      onSelect={onSelect}
      onRename={onRename}
      onTogglePin={onTogglePin}
      onDelete={onDelete}
    />
    {onPageChange && pageCount > 1 ? <nav className="wk-chat-session-pagination" aria-label="Conversation pages"><button type="button" disabled={page <= 1 || loading} onClick={() => onPageChange(Math.max(1, page - 1))}>{t.previous}</button><span>{formatChatCopy(t, 'pageOf', { page, total: pageCount })}</span><button type="button" disabled={page >= pageCount || loading} onClick={() => onPageChange(Math.min(pageCount, page + 1))}>{t.next}</button></nav> : null}
  </aside>;
}
