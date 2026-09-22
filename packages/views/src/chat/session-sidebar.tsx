import { createContext, Fragment, useContext, useEffect, useRef, useState } from 'react';
import type { ChatSession } from '@weknora/contracts';
import { formatChatCopy, resolveChatCopy, resolveChatLocale, sessionGroupLabel, type ChatCopyTable } from './chat-copy.ts';

/**
 * Vue SessionSidebarRow.vue apiOwnerTag（合成主体徽标）：api_external_user /
 * api_tenant_key 前缀的会话在标题旁渲染提问人小徽标；普通账号不渲染。
 */
const API_EXTERNAL_USER_PREFIX = 'api_external_user:';
const API_TENANT_KEY_PREFIX = 'api_tenant_key:';
function apiOwnerTagOf(session: ChatSession): { kind: 'user' | 'key'; label: string; full: string } | null {
  const uid = (session as { user_id?: string }).user_id || '';
  if (uid.startsWith(API_EXTERNAL_USER_PREFIX)) {
    const tail = uid.slice(API_EXTERNAL_USER_PREFIX.length);
    const segments = tail.split(':').filter(Boolean);
    const label = segments.length ? segments[segments.length - 1]! : tail;
    return label ? { kind: 'user', label, full: uid } : null;
  }
  if (uid.startsWith(API_TENANT_KEY_PREFIX)) {
    return { kind: 'key', label: 'API', full: uid };
  }
  return null;
}

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
  onClear?(sessionId: string): Promise<void>;
  onDelete?(sessionId: string): Promise<void>;
  /**
   * SP13 Task 8 — 会话分享入口（宿主能力开关：缺省即隐藏菜单项）。
   * 宿主负责 mint 分享 token 并弹分享窗（apps/web SessionShareDialog）。
   */
  onShareSession?(sessionId: string): void;
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
  /** SP13 Task 8 — 分享菜单项能力开关：缺省隐藏（isFeedbackAvailable 模式）。 */
  onShareSession?(sessionId: string): void;
  /** Batch delete selected rows; returning false means the caller cancelled. */
  onBatchDelete?(sessionIds: readonly string[]): Promise<boolean | void> | boolean | void;
}

/**
 * SP13 Task 8 — 分享菜单项能力开关（message-list isFeedbackAvailable 模式）：
 * 宿主未提供 onShareSession 时侧栏 ⋯ 菜单不渲染分享入口。
 */
export function isShareActionAvailable(onShareSession?: SessionSidebarListProps['onShareSession']): boolean {
  return typeof onShareSession === 'function';
}

/*
 * The grouped list body shared by the in-page chat sidebar and the platform
 * shell sidebar: time group headers (已置顶/今天/昨天/近7天/近30天/更早), full
 * titles, green active row, hover ⋯ menu (置顶/重命名会话/分享/清空消息/删除会话).
 *
 * Task 9.5 — DOM is a 1:1 port of the Vue anatomy (frontend/src/components/
 * menu.vue .submenu + SessionSidebarRow.vue): the visible chrome uses the Vue
 * class vocabulary (.timeline_header / .submenu_item_p.session-chat-row /
 * .session-list-row(--flat) / .submenu_item / .submenu_title), with the shell
 * porting the styles in apps/web/src/platform/platform-shell.td.css. React
 * keeps semantic hooks the tests rely on: role/aria anchors, the <details>
 * ⋯ menu, and the wk-* hook classes; the package stays css-import-free.
 */
export function SessionSidebarList({ copy, sessions, groups, selectedSessionId, loading = false, emptyLabel, untitledLabel, onSelect, onRename, onTogglePin, onClear, onDelete, onShareSession, onBatchDelete, source, sourceOptions, onSourceChange }: SessionSidebarListProps) {
  const t = copy ?? resolveChatCopy(resolveChatLocale());
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState('');
  const [renameError, setRenameError] = useState<string | null>(null);
  const [batchMode, setBatchMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [batchBusy, setBatchBusy] = useState(false);
  const [batchError, setBatchError] = useState<string | null>(null);
  const [sessionDangerAction, setSessionDangerAction] = useState<{ type: 'clear' | 'delete'; sessionId: string } | null>(null);
  const [sessionDangerBusy, setSessionDangerBusy] = useState(false);
  const [sessionDangerError, setSessionDangerError] = useState<string | null>(null);
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
  const hasMenu = Boolean(onRename || onTogglePin || onClear || onDelete || isShareActionAvailable(onShareSession));
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
  const submitSessionDangerAction = async () => {
    if (!sessionDangerAction || sessionDangerBusy) return;
    const callback = sessionDangerAction.type === 'clear' ? onClear : onDelete;
    if (!callback) return;
    setSessionDangerBusy(true);
    setSessionDangerError(null);
    try {
      await callback(sessionDangerAction.sessionId);
      setSessionDangerAction(null);
    } catch (error) {
      setSessionDangerError(error instanceof Error ? error.message : t.operationFailed);
    } finally {
      setSessionDangerBusy(false);
    }
  };
  /*
   * Vue 事实源 DOM（menu.vue .submenu + SessionSidebarRow.vue），样式由
   * apps/web/src/platform/platform-shell.td.css 平移承载。React 侧保留的
   * 语义钩点：role/aria 锚点、details ⋯ 菜单、wk-* hook 类、input aria-label。
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
    {/* Vue menu.vue:113-131 renders four gradient skeleton rows while the
        first bucket loads (never a text "Loading..." label); menu.vue:162-167
        shows a small spinner below the rows while a later page streams in.
        The i18n copy stays as the sr-only announcement for screen readers. */}
    {loading && totalItems === 0 ? <div role="status">
      <span className="sr-only">{t.loadingSessions}</span>
      {[0, 1, 2, 3].map((row) => <div key={row} aria-hidden="true" className="submenu_item_p session-chat-row">
        <div className="session-list-row session-list-row--flat">
          <div className="session-list-row__body t-skeleton t-skeleton--animate">
            <div className="t-skeleton__row"><div className="t-skeleton__col t-skeleton--type-text t-skeleton--animation-gradient" style={{ width: '100%', height: '14px' }} /></div>
          </div>
        </div>
      </div>)}
    </div> : null}
    {loading && totalItems > 0 ? <div className="session-list-loading session-list-row session-list-row--flat" role="status">
      <span className="sr-only">{t.loadingSessions}</span>
      <span className="session-list-row__body" aria-hidden="true">
        <span className="t-loading t-loading--default t-size-s wk-chat-session-loading">
          <span className="t-loading__spinner wk-chat-session-loading-spinner" />
        </span>
      </span>
    </div> : null}
    {!loading && totalItems === 0 && emptyLabel ? <p className="submenu_empty" role="status">{emptyLabel}</p> : null}
    <div className="session-filtered-list">
      {visibleGroups.map((group) => <Fragment key={group.key}>
        {/* Vue menu.vue .timeline_header：11px/16px 分组标题，随源类平移。 */}
        {group.label ? <div className="timeline_header session-list-row session-list-row--flat">
          <span className="session-list-row__body"><span className="timeline_header-label">{sessionGroupLabel(t, group.label)}</span></span>
        </div> : null}
        {group.items.map((session) => {
          const active = session.id === selectedSessionId;
          return <div key={session.id} className={'submenu_item_p session-chat-row'
            + (!batchMode && active ? ' session-chat-row--active' : '')
            + (batchMode && selectedIds.has(session.id) ? ' session-chat-row--selected' : '')}>
            <div className="session-list-row session-list-row--flat">
              <div className="session-list-row__body">
                {/* Vue SessionSidebarRow .submenu_item（div + @click；React 保留
                    role/tabindex/aria-current 语义锚点）。 */}
                <div
                  className={'submenu_item' + (active ? ' submenu_item_active' : '') + (batchMode ? ' submenu_item_batch' : '')}
                  role="button" tabIndex={0} aria-current={active ? 'page' : undefined}
                  onClick={() => { if (batchMode) toggleSelected(session.id); else onSelect(session.id); }}
                  onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); if (batchMode) toggleSelected(session.id); else onSelect(session.id); } }}>
                  {editingSessionId === session.id ? <form className="session-title-edit" onSubmit={(event) => { event.preventDefault(); void submitRename(session); }} onClick={(event) => event.stopPropagation()}>
                    <input aria-label={t.renameSession} className="session-title-edit__input" value={editingTitle} maxLength={80} autoFocus disabled={renameSubmitting.current}
                      onChange={(event) => setEditingTitle(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Escape') { event.preventDefault(); cancelRename(); }
                        if (event.key === 'Enter') { event.preventDefault(); void submitRename(session); }
                      }}
                      onBlur={() => { void submitRename(session); }} />
                    {renameError ? <span role="alert" className="text-[11px] leading-[16px] text-[#e34d59]">{renameError}</span> : null}
                  </form> : <>
                    {batchMode ? <input type="checkbox" className="batch-checkbox" aria-label={formatChatCopy(t, 'batchSelectSession', { title: session.title || untitledLabel || t.untitledChat })} checked={selectedIds.has(session.id)} onChange={() => toggleSelected(session.id)} onClick={(event) => event.stopPropagation()} disabled={batchBusy} /> : null}
                    <span className={batchMode ? 'submenu_title submenu_title--batch' : 'submenu_title'} title={session.title || untitledLabel || t.untitledChat}>
                      {session.is_pinned ? <svg className="t-icon submenu_pin_icon" viewBox="0 0 24 24" width="1em" height="1em" fill="none" aria-hidden="true"><use href="#t-icon-pin" /></svg> : null}
                      <span className="submenu_title-text">{session.title || untitledLabel || t.untitledChat}</span>
                      {apiOwnerTagOf(session) ? <span className={'session-owner-tag session-owner-tag--' + apiOwnerTagOf(session)!.kind} title={apiOwnerTagOf(session)!.full}>{apiOwnerTagOf(session)!.label}</span> : null}
                    </span>
                  </>}
                  {session.running === true ? <span className="session-running-indicator wk-chat-session-running" role="status" aria-label={t.sessionInProgress} title={t.sessionInProgress}><span className="session-running-indicator__spinner wk-chat-session-running-spinner" aria-hidden="true" /></span> : null}
                  {!batchMode && hasMenu ? <div className="session-row-menu-wrap" onClick={(event) => event.stopPropagation()}>
                    <details className="session-row-menu">
                      {/* Vue SessionSidebarRow.vue:26 leaves the row ⋯ button
                          without an accessible name (aria-haspopup only). */}
                      <summary className="menu-more-wrap" aria-haspopup="menu">
                        <svg className="t-icon t-icon-ellipsis menu-more" viewBox="0 0 24 24" width="1em" height="1em" style={{ fontSize: '13.3333px' }} fill="none" aria-hidden="true"><use href="#t-icon-ellipsis" /></svg>
                      </summary>
                      <div className="session-action-menu-panel" role="menu">
                        <div className="session-action-menu">
                          {onTogglePin ? <button type="button" role="menuitem" className="session-action-menu__item" onClick={() => void onTogglePin(session.id, !session.is_pinned)}>
                            <span className="session-action-menu__icon"><svg className="t-icon" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href={session.is_pinned ? '#t-icon-pin-filled' : '#t-icon-pin'} /></svg></span>
                            <span>{session.is_pinned ? t.unpin : t.pin}</span>
                          </button> : null}
                          {onRename ? <button type="button" role="menuitem" className="session-action-menu__item" onClick={() => startRename(session)}>
                            <span className="session-action-menu__icon"><svg className="t-icon" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-edit-1" /></svg></span>
                            <span>{t.renameSession}</span>
                          </button> : null}
                          {onShareSession ? <button type="button" role="menuitem" data-share-session={session.id} className="session-action-menu__item" onClick={() => onShareSession(session.id)}>
                            <span className="session-action-menu__icon"><svg className="t-icon" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-share" /></svg></span>
                            <span>{t.shareSession}</span>
                          </button> : null}
                          {onClear ? <>
                            <div className="session-action-menu__divider" />
                            <button type="button" role="menuitem" className="session-action-menu__item" onClick={() => { setSessionDangerAction({ type: 'clear', sessionId: session.id }); setSessionDangerError(null); }}>
                              <span className="session-action-menu__icon"><svg className="t-icon" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-clear" /></svg></span>
                              <span>{t.clearMessages}</span>
                            </button>
                          </> : null}
                          {onBatchDelete ? <button type="button" role="menuitem" aria-label={formatChatCopy(t, 'batchManage')} className="session-action-menu__item" onClick={toggleBatchMode}>
                            <span className="session-action-menu__icon"><svg className="t-icon" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-queue" /></svg></span>
                            <span>{formatChatCopy(t, 'batchManage')}</span>
                          </button> : null}
                          {onDelete ? <>
                            <div className="session-action-menu__divider" />
                            <button type="button" role="menuitem" className="session-action-menu__item is-danger" onClick={() => { setSessionDangerAction({ type: 'delete', sessionId: session.id }); setSessionDangerError(null); }}>
                              <span className="session-action-menu__icon"><svg className="t-icon" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-delete" /></svg></span>
                              <span>{t.deleteRecord}</span>
                            </button>
                          </> : null}
                          {sessionDangerAction?.sessionId === session.id ? <div className="session-action-confirm wk-chat-session-confirm" role="dialog" aria-label={sessionDangerAction.type === 'clear' ? t.clearMessages : t.deleteSession}>
                            <div className="session-action-confirm__title">{sessionDangerAction.type === 'clear' ? t.clearConfirmTitle : t.deleteConfirmTitle}</div>
                            <div className="session-action-confirm__body">{sessionDangerAction.type === 'clear' ? t.clearConfirmBody : t.deleteConfirmBody}</div>
                            {sessionDangerError ? <p role="alert" className="m-0 px-[6px] pb-[4px] text-[11px] text-[#e34d59]">{sessionDangerError}</p> : null}
                            <div className="session-action-confirm__footer">
                              <button type="button" className="session-action-confirm__btn" onClick={() => setSessionDangerAction(null)} disabled={sessionDangerBusy}>{t.renameCancel}</button>
                              <button type="button" className="session-action-confirm__btn is-danger" onClick={() => void submitSessionDangerAction()} disabled={sessionDangerBusy}>{sessionDangerAction.type === 'clear' ? t.clearConfirmAction : t.deleteConfirmAction}</button>
                            </div>
                          </div> : null}
                        </div>
                      </div>
                    </details>
                  </div> : null}
                </div>
              </div>
            </div>
          </div>;
        })}
      </Fragment>)}
    </div>
  </>;
}

export function SessionSidebar({ copy, sessions, selectedSessionId, loading = false, onSelect, onCreate, onRename, onTogglePin, onClear, onDelete, onShareSession, groups, source, sourceOptions, onSourceChange, groupMode, onGroupModeChange, keyword, onKeywordChange, page = 1, pageCount = 1, onPageChange }: SessionSidebarProps) {
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
      onClear={onClear}
      onDelete={onDelete}
      onShareSession={onShareSession}
    />
    {onPageChange && pageCount > 1 ? <nav className="mt-auto flex items-center justify-center gap-[0.4rem] pt-[0.75rem] text-[rgba(0,0,0,0.4)] text-[12px]" aria-label={t.conversationPagesLabel}><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[8px] py-[2px] text-[rgba(0,0,0,0.6)] text-[12px] disabled:cursor-not-allowed disabled:opacity-50" disabled={page <= 1 || loading} onClick={() => onPageChange(Math.max(1, page - 1))}>{t.previous}</button><span>{formatChatCopy(t, 'pageOf', { page, total: pageCount })}</span><button type="button" className="cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-transparent px-[8px] py-[2px] text-[rgba(0,0,0,0.6)] text-[12px] disabled:cursor-not-allowed disabled:opacity-50" disabled={page >= pageCount || loading} onClick={() => onPageChange(Math.min(pageCount, page + 1))}>{t.next}</button></nav> : null}
  </aside>;
}
