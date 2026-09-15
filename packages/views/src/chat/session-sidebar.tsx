import { createContext, useContext, useEffect, useRef, useState } from 'react';
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
  source?: string;
  sourceOptions?: readonly SessionSourceOption[];
  onSourceChange?(source: string): void;
  onSelect(sessionId: string): void;
  onCreate(): void;
  onRename?(sessionId: string): Promise<void>;
  onTogglePin?(sessionId: string, pinned: boolean): Promise<void>;
  onDelete?(sessionId: string): Promise<void>;
  groups?: readonly SessionGroupView[];
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
  source?: string;
  sourceOptions?: readonly SessionSourceOption[];
  onSourceChange?(source: string): void;
  /** Empty-state copy (Vue menu.noSessions); omitted renders nothing. */
  emptyLabel?: string;
  /** Fallback row title (Vue mapSessionRow uses menu.newSession = 新会话). */
  untitledLabel?: string;
  onSelect(sessionId: string): void;
  onRename?(sessionId: string, title?: string): Promise<void> | void;
  onTogglePin?(sessionId: string, pinned: boolean): Promise<void> | void;
  /** 清空消息 (Vue menu.vue row menu → clearSession); confirm is the caller's. */
  onClear?(sessionId: string): Promise<void> | void;
  onDelete?(sessionId: string): Promise<void> | void;
  /** Batch delete selected rows; returning false means the caller cancelled. */
  onBatchDelete?(sessionIds: readonly string[]): Promise<boolean | void> | boolean | void;
}

/*
 * The grouped list body shared by the in-page chat sidebar and the platform
 * shell sidebar: time group headers (已置顶/今天/昨天/近7天/近30天/更早), full
 * titles, green active row, hover ⋯ menu (置顶/重命名会话/清空消息/删除会话).
 */
export function SessionSidebarList({ copy, sessions, groups, selectedSessionId, loading = false, emptyLabel, untitledLabel, onSelect, onRename, onTogglePin, onClear, onDelete, onBatchDelete, source, sourceOptions, onSourceChange }: SessionSidebarListProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState('');
  const [renameError, setRenameError] = useState<string | null>(null);
  const [batchMode, setBatchMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [batchBusy, setBatchBusy] = useState(false);
  const [batchError, setBatchError] = useState<string | null>(null);
  const selectAllRef = useRef<HTMLInputElement | null>(null);
  const renameSubmitting = useRef(false);
  const startRename = (session: ChatSession) => {
    setEditingSessionId(session.id);
    setEditingTitle(session.title ?? '');
    setRenameError(null);
  };
  const cancelRename = () => {
    if (renameSubmitting.current) return;
    setEditingSessionId(null);
    setEditingTitle('');
    setRenameError(null);
  };
  const submitRename = async (session: ChatSession) => {
    if (renameSubmitting.current || editingSessionId !== session.id) return;
    const title = editingTitle.trim().replace(/\s+/g, ' ').slice(0, 80);
    if (!title || title === (session.title ?? '')) {
      if (!title) setRenameError('标题不能为空');
      else cancelRename();
      return;
    }
    renameSubmitting.current = true;
    setRenameError(null);
    try {
      await onRename?.(session.id, title);
      setEditingSessionId(null);
      setEditingTitle('');
    } catch (error) {
      setRenameError(error instanceof Error ? error.message : '修改标题失败');
    } finally {
      renameSubmitting.current = false;
    }
  };
  const visibleGroups = groups ?? [{ key: 'all', items: sessions ?? [] }];
  const hasMenu = Boolean(onRename || onTogglePin || onClear || onDelete);
  const totalItems = visibleGroups.reduce((count, group) => count + group.items.length, 0);
  const visibleIds = visibleGroups.flatMap((group) => group.items.map((session) => session.id));
  useEffect(() => {
    setSelectedIds((current) => {
      const next = new Set([...current].filter((id) => visibleIds.includes(id)));
      return next.size === current.size ? current : next;
    });
  }, [visibleIds.join('\u0000')]);
  const allSelected = totalItems > 0 && visibleIds.every((id) => selectedIds.has(id));
  useEffect(() => {
    if (selectAllRef.current) selectAllRef.current.indeterminate = selectedIds.size > 0 && !allSelected;
  }, [allSelected, selectedIds.size]);
  const toggleBatchMode = () => {
    if (batchBusy) return;
    setBatchMode((current) => !current);
    setSelectedIds(new Set());
    setBatchError(null);
  };
  const toggleSelected = (sessionId: string) => {
    setSelectedIds((current) => {
      const next = new Set(current);
      if (next.has(sessionId)) next.delete(sessionId); else next.add(sessionId);
      return next;
    });
  };
  const toggleAll = () => setSelectedIds(allSelected ? new Set() : new Set(visibleIds));
  const submitBatchDelete = async () => {
    if (!onBatchDelete || selectedIds.size === 0 || batchBusy) return;
    const ids = [...selectedIds];
    if (!window.confirm(formatChatCopy(t, 'batchDeleteConfirm', { count: ids.length }))) return;
    setBatchBusy(true);
    setBatchError(null);
    try {
      const result = await onBatchDelete(ids);
      if (result !== false) {
        setSelectedIds(new Set());
        setBatchMode(false);
      }
    } catch (error) {
      setBatchError(error instanceof Error ? error.message : '批量删除失败');
    } finally {
      setBatchBusy(false);
    }
  };
  /*
   * shell.css → utilities. Effective values verified against the built css
   * bundle: for elements whose classes also matched styles.css rules, the
   * styles.css declaration won the unlayered cascade (bundle order) — the
   * group <h3> renders uppercase #66758b .78rem (styles.css) with the shell
   * css contributing only font-weight/line-height, so those are the values
   * encoded here. Row hover/active greens come from the deleted shell rules
   * (group/item hover keeps rgba(0,0,0,0.04); an active row stays green on
   * hover, matching the css source order). is-active / is-danger /
   * is-im|is-embed|is-api remain as state markers without css.
   * The shell-context ul indent (padding 0 6px) lives in PlatformShell's
   * sessions nav as [&_ul]:px-[6px]; this shared list stays flush outside.
   */
  return <>
    {!batchMode && sourceOptions && onSourceChange ? <label className="grid gap-[0.25rem] mx-[4px] my-[0.55rem] text-[rgba(0,0,0,0.4)] text-[12px]">{t.sourceLabel}<select aria-label={t.sourceSelectLabel} className="w-full box-border rounded-[6px] border border-[#cbd5e1] bg-white p-[0.45rem] text-[rgba(0,0,0,0.9)] text-[13px]" value={source ?? ''} onChange={(event) => onSourceChange(event.target.value)}>{sourceOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label> : null}
    {onBatchDelete && batchMode ? <div className="sticky bottom-0 z-10 flex items-center justify-between gap-[6px] mx-[4px] my-[6px] bg-[#f6f8fa]" role="toolbar" aria-label={formatChatCopy(t, 'batchManage')}>
      <>
        <button type="button" aria-label={formatChatCopy(t, 'batchCancel')} className="border-0 bg-transparent p-0 text-[12px] text-[#66758b] cursor-pointer hover:text-[#07c05f]" onClick={toggleBatchMode} disabled={batchBusy}>{formatChatCopy(t, 'batchCancel')}</button>
        <label className="inline-flex items-center gap-[4px] text-[12px] text-[#66758b]">
          <input ref={selectAllRef} type="checkbox" aria-label={formatChatCopy(t, 'batchSelectAll')} checked={allSelected} onChange={toggleAll} disabled={batchBusy || totalItems === 0} />{formatChatCopy(t, 'batchSelectAll')}
        </label>
        <button type="button" aria-label={formatChatCopy(t, 'batchDelete', { count: selectedIds.size })} className="border-0 bg-transparent p-0 text-[12px] text-[#e34d59] cursor-pointer disabled:cursor-not-allowed disabled:opacity-50" onClick={() => void submitBatchDelete()} disabled={batchBusy || selectedIds.size === 0}>{batchBusy ? formatChatCopy(t, 'batchDeleteBusy') : formatChatCopy(t, 'batchDelete', { count: selectedIds.size })}</button>
      </>
    </div> : null}
    {batchError ? <p role="alert" className="mx-[4px] my-[4px] text-[12px] text-[#e34d59]">{formatChatCopy(t, 'batchDeleteError', { message: batchError })} <button type="button" aria-label={formatChatCopy(t, 'batchRetry')} className="border-0 bg-transparent p-0 text-[12px] text-[#07c05f] underline cursor-pointer" onClick={() => void submitBatchDelete()} disabled={batchBusy}>{formatChatCopy(t, 'batchRetry')}</button></p> : null}
    {loading ? <p role="status">{t.loadingSessions}</p> : null}
    {!loading && totalItems === 0 && emptyLabel ? <p className="my-[10px] mx-[4px] text-[rgba(0,0,0,0.4)] text-[12px]" role="status">{emptyLabel}</p> : null}
    {visibleGroups.map((group) => <section key={group.key}>
      {group.label ? <h3 className="mt-[1rem] mx-0 mb-[0.35rem] text-[#66758b] text-[0.78rem] font-normal tracking-[0.04em] leading-[20px] uppercase">{sessionGroupLabel(t, group.label)}</h3> : null}
      <ul className="list-none m-0 p-0">{group.items.map((session) => {
        const badge = sessionSourceBadge(session);
        const active = session.id === selectedSessionId;
        return <li key={session.id} className={'group/item flex items-center rounded-[8px] relative' + (active ? ' is-active' : '')}>
          {batchMode ? <input type="checkbox" aria-label={formatChatCopy(t, 'batchSelectSession', { title: session.title || untitledLabel || t.untitledChat })} checked={selectedIds.has(session.id)} onChange={() => toggleSelected(session.id)} disabled={batchBusy} className="mx-[4px] shrink-0" /> : null}
          {editingSessionId === session.id ? <div className="flex min-w-0 flex-1 flex-col gap-[2px] px-[6px] py-[4px]">
            <input type="text" aria-label={t.renameSession} value={editingTitle} maxLength={80} autoFocus disabled={renameSubmitting.current}
              className="w-full min-w-0 box-border rounded-[5px] border border-[#07c05f] bg-white px-[7px] py-[4px] text-[14px] leading-[20px] outline-none"
              onChange={(event) => setEditingTitle(event.target.value)}
              onClick={(event) => event.stopPropagation()}
              onKeyDown={(event) => {
                if (event.key === 'Escape') { event.preventDefault(); cancelRename(); }
                if (event.key === 'Enter') { event.preventDefault(); void submitRename(session); }
              }}
              onBlur={() => { void submitRename(session); }} />
            {renameError ? <span role="alert" className="text-[11px] leading-[16px] text-[#e34d59]">{renameError}</span> : null}
          </div> : <button type="button" aria-current={active ? 'page' : undefined} onClick={() => { if (batchMode) toggleSelected(session.id); else onSelect(session.id); }}
            className={'flex flex-1 items-center min-w-0 gap-[6px] px-[10px] py-[7px] border-0 rounded-[8px] cursor-pointer text-left text-[14px] leading-[22px] overflow-hidden transition-[background-color,color] duration-[150ms] ease-[ease] '
            + (active ? 'bg-[#e9f8ec] text-[#07c05f] font-medium' : 'bg-transparent text-[rgba(0,0,0,0.9)] group-hover/item:bg-[rgba(0,0,0,0.04)]')}>
            {session.is_pinned ? <span className="shrink-0 text-[rgba(0,0,0,0.4)] text-[12px]" aria-hidden="true">★</span> : null}
            <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{session.title || untitledLabel || t.untitledChat}</span>
            {badge.kind ? <span className={badge.kind + ' shrink-0 text-[10px] font-semibold tracking-[0.03em] leading-[1.4] uppercase text-[rgba(0,0,0,0.4)] bg-[#eee] rounded-[4px] px-[4px]'} title="Session source">{badge.label}</span> : null}
          </button>}
          {!batchMode && hasMenu ? <details className="group/menu shrink-0 relative">
            <summary aria-label={t.moreActions} title={t.moreActions}
              className="inline-flex items-center justify-center h-[24px] w-[24px] rounded-[5px] text-[rgba(0,0,0,0.26)] cursor-pointer list-none opacity-0 transition-[opacity,background-color,color] duration-[150ms] ease-[ease] hover:bg-[rgba(0,0,0,0.06)] hover:text-[rgba(0,0,0,0.9)] group-hover/item:opacity-100 focus-visible:opacity-100 group-open/menu:opacity-100 [&::-webkit-details-marker]:hidden">
              <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><circle cx="8" cy="3" r="1.4" /><circle cx="8" cy="8" r="1.4" /><circle cx="8" cy="13" r="1.4" /></svg>
            </summary>
            <div className="absolute right-0 top-[26px] z-30 flex min-w-[120px] flex-col gap-[1px] rounded-[8px] border-[0.5px] border-[#e7e7e7] bg-white p-[4px] shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_6px_rgba(0,0,0,0.08)]" role="menu">
              {/* .wk-chat-session-menu-list button (+ .is-danger) → utilities. */}
              {onTogglePin ? <button type="button" role="menuitem" className="min-h-[30px] px-[10px] py-0 border-0 rounded-[5px] bg-transparent cursor-pointer text-left text-[13px] leading-[20px] whitespace-nowrap text-[rgba(0,0,0,0.9)] hover:bg-[#f3f3f3]" onClick={() => void onTogglePin(session.id, !session.is_pinned)}>{session.is_pinned ? t.unpin : t.pin}</button> : null}
              {onRename ? <button type="button" role="menuitem" className="min-h-[30px] px-[10px] py-0 border-0 rounded-[5px] bg-transparent cursor-pointer text-left text-[13px] leading-[20px] whitespace-nowrap text-[rgba(0,0,0,0.9)] hover:bg-[#f3f3f3]" onClick={() => startRename(session)}>{t.renameSession}</button> : null}
              {onClear ? <button type="button" role="menuitem" className="min-h-[30px] px-[10px] py-0 border-0 rounded-[5px] bg-transparent cursor-pointer text-left text-[13px] leading-[20px] whitespace-nowrap text-[rgba(0,0,0,0.9)] hover:bg-[#f3f3f3]" onClick={() => void onClear(session.id)}>{t.clearMessages}</button> : null}
              {onBatchDelete ? <button type="button" role="menuitem" aria-label={formatChatCopy(t, 'batchManage')} className="min-h-[30px] px-[10px] py-0 border-0 rounded-[5px] bg-transparent cursor-pointer text-left text-[13px] leading-[20px] whitespace-nowrap text-[rgba(0,0,0,0.9)] hover:bg-[#f3f3f3]" onClick={toggleBatchMode}>{formatChatCopy(t, 'batchManage')}</button> : null}
              {onDelete ? <button type="button" role="menuitem" className="is-danger min-h-[30px] px-[10px] py-0 border-0 rounded-[5px] bg-transparent cursor-pointer text-left text-[13px] leading-[20px] whitespace-nowrap text-[#e34d59] hover:bg-[#fdecee]" onClick={() => void onDelete(session.id)}>{t.deleteRecord}</button> : null}
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
  /*
   * Fallback in-page sidebar (never rendered under the shell — the shell
   * suppresses it via the context above). shell.css rules → utilities; where
   * styles.css also matched (flex heading, #cbd5e1 select/input chrome,
   * .55rem label margins), the styles.css values won the cascade in the
   * built bundle and are the ones encoded here. The loading p[role=status]
   * stays unstyled: its old styling came from a .wk-chat-sidebar-scoped rule
   * that only ever reached this fallback context.
   */
  return <aside className="flex min-w-0 flex-col gap-0 overflow-y-auto bg-[#f9f9f9] border-r border-[#e7e7e7] box-border p-[12px] pb-[16px]" aria-label={t.sidebarTitle}>
    <div className="flex items-center justify-between gap-[0.6rem] mb-[8px]">
      <button type="button" className="flex w-full box-border items-center gap-[8px] rounded-[8px] border-0 bg-transparent px-[10px] py-[8px] text-[#07c05f] text-[14px] font-medium cursor-pointer transition-[background-color] duration-[150ms] ease-[ease] hover:bg-[#e9f8ec]" onClick={onCreate}>
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" aria-hidden="true"><path d="M8 3v10M3 8h10" /></svg>
        <span>{t.newChat}</span>
      </button>
    </div>
    {sourceOptions && onSourceChange ? <label className="grid gap-[0.25rem] my-[0.55rem] text-[rgba(0,0,0,0.4)] text-[12px]">{t.sourceLabel}<select aria-label={t.sourceSelectLabel} className="w-full box-border rounded-[6px] border border-[#cbd5e1] bg-white p-[0.45rem] text-[rgba(0,0,0,0.9)] text-[13px]" value={source ?? ''} onChange={(event) => onSourceChange(event.target.value)}>{sourceOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label> : null}
    {onKeywordChange ? <label className="grid gap-[0.25rem] my-[0.55rem] text-[rgba(0,0,0,0.4)] text-[12px]">{t.searchSessions}<input className="w-full box-border rounded-[6px] border border-[#cbd5e1] bg-white p-[0.45rem] text-[rgba(0,0,0,0.9)] text-[13px] placeholder:text-[rgba(0,0,0,0.26)]" value={keyword ?? ''} onChange={(event) => onKeywordChange(event.target.value)} placeholder={t.searchSessions} /></label> : null}
    {onGroupModeChange ? <label className="grid gap-[0.25rem] my-[0.55rem] text-[rgba(0,0,0,0.4)] text-[12px]">{t.groupLabel}<select className="w-full box-border rounded-[6px] border border-[#cbd5e1] bg-white p-[0.45rem] text-[rgba(0,0,0,0.9)] text-[13px]" value={groupMode ?? 'none'} onChange={(event) => onGroupModeChange(event.target.value === 'date' ? 'date' : 'none')}><option value="none">{t.groupAll}</option><option value="date">{t.groupByDate}</option></select></label> : null}
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
    {onPageChange && pageCount > 1 ? <nav className="mt-auto flex items-center justify-center gap-[0.4rem] pt-[0.75rem] text-[rgba(0,0,0,0.4)] text-[12px]" aria-label="Conversation pages"><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[8px] py-[2px] text-[rgba(0,0,0,0.6)] text-[12px] disabled:cursor-not-allowed disabled:opacity-50" disabled={page <= 1 || loading} onClick={() => onPageChange(Math.max(1, page - 1))}>{t.previous}</button><span>{formatChatCopy(t, 'pageOf', { page, total: pageCount })}</span><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[8px] py-[2px] text-[rgba(0,0,0,0.6)] text-[12px] disabled:cursor-not-allowed disabled:opacity-50" disabled={page >= pageCount || loading} onClick={() => onPageChange(Math.min(pageCount, page + 1))}>{t.next}</button></nav> : null}
  </aside>;
}
