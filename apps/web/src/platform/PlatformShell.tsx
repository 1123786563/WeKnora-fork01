import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import type { ChatSession, createWeKnoraClient } from '@weknora/api-client';
import { sessionGroups } from '@weknora/domain/chat/session-state';
import { GlobalCommandPalette } from './GlobalCommandPalette.tsx';
import { SessionSidebarList, SessionSidebarShellContext, type SessionGroupView } from '@weknora/views';
import { chatSessionIdFromPath, SHELL_SESSION_ROUTE_EVENT } from '../chat/session-route.ts';
import { ContextualGuideHost, NewUserGuide, openNewUserGuide } from '@weknora/views';
import {
  clearRecentQueries,
  consumeCmdkParam,
  decideGlobalShortcutAction,
  loadRecentQueries,
  pushRecentQuery,
  recentQueriesStorageKey,
} from './command-palette.ts';
import { readReactPlatformState } from './legacy-session.ts';
import './shell.css';
// Welcome-tour styles live with the component in @weknora/views; the package
// itself must stay css-import-free for the shared typecheck, so the shell
// (which already imports shell.css) pulls it in by relative path.
import '../../../../packages/views/src/guides/guides.css';

type Client = ReturnType<typeof createWeKnoraClient>;

export interface PlatformShellProps {
  client: Client;
  onLogout: () => void | Promise<void>;
  children: ReactNode;
}

// Locale resolution mirrors App.tsx / SettingsPage: navigator.language with a
// package-supported fallback.
function resolveLocale(): Locale {
  const language = typeof navigator !== 'undefined' ? navigator.language : 'en-US';
  return isLocale(language) ? language : isLocale(language.split('-')[0] ?? '') ? (language.split('-')[0] as Locale) : 'en-US';
}

// Nav labels migrated to packages/i18n menu.* (auto-ported from Vue locales).

interface NavItem {
  key: string;
  href: string;
  label: string;
  icon: ReactNode;
  match: (pathname: string) => boolean;
  /** Anchor for the welcome-tour spotlight (Vue menu.vue data-guide attrs). */
  guide?: string;
}

// Vue menu.vue:300-304 — platform-aware modifier label for shortcut hints
// (⌘ on Apple platforms, Ctrl+ elsewhere). Groundwork for the logo-row ⌘K
// search entry (menu.vue:10-21) and the palette-scoped ⌘1-9 chips.
export function platformModKeyLabel(platform: string): '⌘' | 'Ctrl+' {
  return /Mac|iPod|iPhone|iPad/.test(platform) ? '⌘' : 'Ctrl+';
}

const KB_ACTIVE = (pathname: string): boolean =>
  pathname === '/platform/knowledge-bases' ||
  /^\/platform\/knowledge-bases\/[^/]+/.test(pathname) ||
  /^\/knowledgeBase(\/|$)/.test(pathname);

function Icon({ path }: { path: string }): ReactNode {
  return (
    <svg className="plat-shell__icon" viewBox="0 0 24 24" width="18" height="18" fill="none"
      stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d={path} />
    </svg>
  );
}

const ICONS = {
  chat: 'M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z',
  book: 'M4 19.5A2.5 2.5 0 0 1 6.5 17H20M4 19.5A2.5 2.5 0 0 0 6.5 22H20V2H6.5A2.5 2.5 0 0 0 4 4.5v15z',
  bot: 'M12 8V4m0 0a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3zM4 7h16a1 1 0 0 1 1 1v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a1 1 0 0 1 1-1zm5 5h.01M15 12h.01M9 17h6',
  users: 'M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2m22-2v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0z',
};

function buildNavItems(t: (key: string) => string, labels: Record<string, string>): NavItem[] {
  return [
    { key: 'newChat', href: '/platform/creatChat', label: labels.newChat, icon: <Icon path={ICONS.chat} />, match: (p: string) => p === '/platform/creatChat' || p.startsWith('/platform/chat/') , guide: 'nav-creatChat' },
    { key: 'knowledgeBases', href: '/platform/knowledge-bases', label: t('common.knowledgeBases'), icon: <Icon path={ICONS.book} />, match: KB_ACTIVE, guide: 'nav-knowledge-bases' },
    { key: 'agents', href: '/platform/agents', label: labels.agents, icon: <Icon path={ICONS.bot} />, match: (p: string) => p === '/platform/agents' || p.startsWith('/platform/agents/') || p === '/platform/configuration', guide: 'nav-agents' },
    { key: 'organizations', href: '/platform/organizations', label: labels.organizations, icon: <Icon path={ICONS.users} />, match: (p: string) => p.startsWith('/platform/organizations') },
  ];
}

const COLLAPSE_STORAGE_KEY = 'weknora_sidebar_collapsed';

// Vue chatHeader.clearConfirmBody / deleteConfirmBody (zh-CN). The chat.*
// domain does not exist in @weknora/i18n yet, so the confirm copy is inlined
// here; reported as missing keys in the slice evidence.
const CLEAR_SESSION_CONFIRM = '确认清空当前对话的全部消息？对话本身会保留，此操作无法恢复。';
const DELETE_SESSION_CONFIRM = '确认删除当前对话？删除后将无法恢复。';

export function PlatformShell({ client, onLogout, children }: PlatformShellProps): ReactNode {
  const locale = useMemo(resolveLocale, []);
  const t = useCallback((key: string) => formatMessage(locale, key), [locale]);
  const labels = {
    newChat: formatMessage(locale, 'menu.newChat'),
    agents: formatMessage(locale, 'menu.agents'),
    organizations: formatMessage(locale, 'menu.organizations'),
    personalSettings: formatMessage(locale, 'general.personalSettings'),
    // Vue UserMenu.vue:45 uses $t('newUserGuide.reopen') for the reopen entry.
    reopenGuide: formatMessage(locale, 'newUserGuide.reopen'),
    // Session-list copy (Vue menu.vue uses the same menu.* keys).
    myChats: formatMessage(locale, 'menu.myChats'),
    noSessions: formatMessage(locale, 'menu.noSessions'),
    renameSession: formatMessage(locale, 'menu.renameSession'),
    newSession: formatMessage(locale, 'menu.newSession'),
  };

  const [pathname, setPathname] = useState(() => window.location.pathname);
  const [collapsed, setCollapsed] = useState(() => window.localStorage.getItem(COLLAPSE_STORAGE_KEY) === 'true');
  const [menuOpen, setMenuOpen] = useState(false);
  // Welcome tour (Vue mounts NewUserGuide in platform/index.vue). The models
  // step opens the settings section in place; the shell remembers that it
  // navigated so leaving the step can return to the previous page, mirroring
  // Vue's uiStore.openSettings/closeSettings pair.
  const guideOpenedSettingsRef = useRef(false);
  const [user, setUser] = useState<{ id: string; name: string; email: string; avatar: string }>({ id: '', name: '', email: '', avatar: '' });

  // Global command palette (⌘K / Ctrl+K) — R011/N003. See GlobalCommandPalette.tsx.
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [paletteQuery, setPaletteQuery] = useState('');
  const [recentQueries, setRecentQueries] = useState<string[]>([]);
  const recentQueriesKey = recentQueriesStorageKey(user.id || null, readReactPlatformState(window.localStorage)?.tenantId ?? null);

  useEffect(() => {
    // Route transitions in this app mostly use full navigations and popstate
    // reloads the page. Track pushState/replaceState so in-place navigations
    // (e.g. settings section links) still update the active highlight.
    const update = () => setPathname(window.location.pathname);
    window.addEventListener('popstate', update);
    const originalPush = window.history.pushState.bind(window.history);
    const originalReplace = window.history.replaceState.bind(window.history);
    window.history.pushState = (...args: Parameters<typeof originalPush>) => { const r = originalPush(...args); update(); return r; };
    window.history.replaceState = (...args: Parameters<typeof originalReplace>) => { const r = originalReplace(...args); update(); return r; };
    return () => {
      window.removeEventListener('popstate', update);
      window.history.pushState = originalPush;
      window.history.replaceState = originalReplace;
    };
  }, []);

  useEffect(() => {
    let active = true;
    void client.auth.me().then((me) => {
      if (!active) return;
      const record = me.user as Record<string, unknown>;
      setUser({
        id: typeof record.id === 'string' ? record.id : typeof record.id === 'number' ? String(record.id) : '',
        name: typeof record.username === 'string' && record.username ? record.username : '—',
        email: typeof record.email === 'string' ? record.email : '',
        avatar: typeof record.avatar === 'string' ? record.avatar : '',
      });
    }).catch(() => { /* menu falls back to placeholders; the page still works */ });
    return () => { active = false; };
  }, [client]);

  // Recent ⌘K searches are namespaced per (user, tenant); reload whenever
  // that identity resolves (mirrors Vue commandPaletteStore's auth watcher).
  useEffect(() => {
    setRecentQueries(loadRecentQueries(window.localStorage, recentQueriesKey));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [recentQueriesKey]);

  // Session list (Vue menu.vue .submenu): the platform sidebar lists the
  // tenant's web conversations on every protected page. Same API surface the
  // chat page uses (client.sessions.list); first page only — the Vue list
  // pages deeper buckets in on scroll, which the shell does not do yet.
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(true);
  useEffect(() => {
    let active = true;
    setSessionsLoading(true);
    void client.sessions.list({ page: 1, pageSize: 30, source: 'web' }).then(
      (result) => {
        if (!active) return;
        setSessions(result.data);
        setSessionsLoading(false);
      },
      () => { if (active) setSessionsLoading(false); },
    );
    return () => { active = false; };
  }, [client]);

  // Vue menu.vue groups the list by date unconditionally (groupSessionsByDate
  // → 已置顶/今天/昨天/近7天/近30天/更早), with the route as the selection.
  const sessionListGroups: readonly SessionGroupView[] = useMemo(
    () => sessionGroups(sessions, new Date(), 'date'),
    [sessions],
  );
  const activeChatId = chatSessionIdFromPath(pathname);

  const openShellSession = useCallback((sessionId: string) => {
    const current = window.location.pathname;
    if (current === '/platform/creatChat' || chatSessionIdFromPath(current)) {
      // ChatRoutePage owns stream teardown: it performs the in-place switch
      // and pushes the new /platform/chat/:id route, which the shell's
      // history patch mirrors into the active-row highlight.
      window.dispatchEvent(new CustomEvent(SHELL_SESSION_ROUTE_EVENT, { detail: { sessionId } }));
    } else {
      window.location.assign(`/platform/chat/${encodeURIComponent(sessionId)}`);
    }
  }, []);

  async function renameShellSession(sessionId: string): Promise<void> {
    const current = sessions.find((session) => session.id === sessionId);
    const title = window.prompt(labels.renameSession, current?.title ?? '')?.trim();
    if (!title || title === current?.title) return;
    try {
      const updated = await client.sessions.update(sessionId, { title, description: current?.description });
      setSessions((items) => items.map((session) => session.id === sessionId ? updated : session));
    } catch { /* keep the prior title on failure */ }
  }

  async function toggleShellSessionPin(sessionId: string, pinned: boolean): Promise<void> {
    try {
      await (pinned ? client.sessions.pin(sessionId) : client.sessions.unpin(sessionId));
      setSessions((items) => items.map((session) => session.id === sessionId ? { ...session, is_pinned: pinned } : session));
    } catch { /* keep the prior pin state on failure */ }
  }

  async function clearShellSessionMessages(sessionId: string): Promise<void> {
    if (!window.confirm(CLEAR_SESSION_CONFIRM)) return;
    try { await client.sessions.clear(sessionId); } catch { /* messages reload on the next visit */ }
  }

  async function deleteShellSession(sessionId: string): Promise<void> {
    if (!window.confirm(DELETE_SESSION_CONFIRM)) return;
    try { await client.sessions.remove(sessionId); } catch { return; }
    setSessions((items) => items.filter((session) => session.id !== sessionId));
    // Vue menu.vue: deleting the open session routes back to creatChat.
    if (chatSessionIdFromPath(window.location.pathname) === sessionId) {
      window.location.assign('/platform/creatChat');
    }
  }

  // `/platform/knowledge-search?q=...` redirects to `?cmdk=...` (routes.tsx).
  // Consume it once on mount, open the palette, and strip the param so
  // Back/Refresh doesn't reopen it (mirrors platform/index.vue's route.query.cmdk watcher).
  useEffect(() => {
    const { query, remainingSearch } = consumeCmdkParam(window.location.search);
    if (query === null) return;
    setPaletteQuery(query);
    setPaletteOpen(true);
    window.history.replaceState(window.history.state, '', `${window.location.pathname}${remainingSearch}${window.location.hash}`);
    // Only ever consume the initial load's query string, matching Vue's
    // one-shot behavior on the redirect landing.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Global ⌘K / Ctrl+K shortcut, plus bare "/" when nothing editable is
  // focused (mirrors GlobalCommandPalette.vue onGlobalKey()).
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const tag = (target?.tagName ?? '').toUpperCase();
      const isEditingTarget = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || !!target?.isContentEditable;
      const action = decideGlobalShortcutAction(event, { open: paletteOpen, isEditingTarget });
      if (action === 'none') return;
      event.preventDefault();
      if (action === 'toggle') setPaletteOpen((current) => !current);
      else if (action === 'open') { setPaletteQuery(''); setPaletteOpen(true); }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [paletteOpen]);

  const closePalette = useCallback(() => setPaletteOpen(false), []);
  const navigateFromPalette = useCallback((path: string) => { window.location.assign(path); }, []);
  const recordPaletteSearch = useCallback((query: string) => {
    setRecentQueries(pushRecentQuery(window.localStorage, recentQueriesKey, query));
  }, [recentQueriesKey]);
  const clearPaletteRecent = useCallback(() => {
    clearRecentQueries(window.localStorage, recentQueriesKey);
    setRecentQueries([]);
  }, [recentQueriesKey]);

  const navItems = useMemo(() => buildNavItems(t, labels), [t, labels]);

  // Welcome-tour shell callbacks (Vue: uiStore.expandSidebar / openSettings('models')).
  const guideActions = useMemo(() => ({
    expandSidebar: () => setCollapsed(false),
    openModelsSettings: () => {
      window.history.pushState({}, '', '/platform/settings?section=models');
      guideOpenedSettingsRef.current = true;
    },
    closeGuideSettings: () => {
      if (!guideOpenedSettingsRef.current) return;
      guideOpenedSettingsRef.current = false;
      window.history.back();
    },
  }), []);

  // KB-list quick filters: the React KB list page reads ?scope=all|mine from
  // the URL. 收藏/最近 from the Vue rail have no backing in the React page
  // this round, so only the two supported scopes are rendered.
  const kbListContext = pathname === '/platform/knowledge-bases' || pathname === '/platform';
  const currentScope = new URLSearchParams(window.location.search).get('scope') === 'mine' ? 'mine' : 'all';

  const toggleCollapsed = () => {
    setCollapsed((current) => {
      window.localStorage.setItem(COLLAPSE_STORAGE_KEY, String(!current));
      return !current;
    });
  };

  const initial = (user.name || '?').charAt(0).toUpperCase();

  return (
    <div className="plat-shell">
      <aside className={`plat-shell__aside${collapsed ? ' plat-shell__aside--collapsed' : ''}`}>
        <div className="plat-shell__logo-row">
          <a className="plat-shell__logo" href="/platform/knowledge-bases" aria-label="WeKnora">
            <span className="plat-shell__logo-mark" aria-hidden="true">W</span>
            {!collapsed && <span className="plat-shell__logo-text">WeKnora</span>}
          </a>
          {!collapsed && (
            <button type="button" className="plat-shell__toggle" onClick={toggleCollapsed} aria-label="Collapse sidebar" title="Collapse sidebar">
              <svg viewBox="0 0 20 20" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.2" aria-hidden="true">
                <rect x="1.5" y="1.5" width="17" height="17" rx="3" />
                <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" />
                <line x1="5" y1="10" x2="3" y2="8" strokeLinecap="round" />
                <line x1="5" y1="10" x2="3" y2="12" strokeLinecap="round" />
              </svg>
            </button>
          )}
        </div>
        {collapsed && (
          <button type="button" className="plat-shell__item plat-shell__toggle-item" onClick={toggleCollapsed} aria-label="Expand sidebar">
            <svg viewBox="0 0 20 20" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.2" aria-hidden="true">
              <rect x="1.5" y="1.5" width="17" height="17" rx="3" />
              <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" />
              <line x1="4" y1="7.5" x2="4" y2="12.5" strokeLinecap="round" />
            </svg>
          </button>
        )}

        <div className="plat-shell__top">
          <nav className="plat-shell__nav" aria-label="Platform">
            {navItems.map((item) => {
              const active = item.match(pathname);
              return (
                <a key={item.key} href={item.href} className={`plat-shell__item${active ? ' plat-shell__item--active' : ''}`}
                  aria-current={active ? 'page' : undefined} title={collapsed ? item.label : undefined} data-guide={item.guide}>
                  <span className="plat-shell__item-icon">{item.icon}</span>
                  {!collapsed && <span className="plat-shell__item-label">{item.label}</span>}
                </a>
              );
            })}
          </nav>

          {kbListContext && !collapsed && (
            <div className="plat-shell__kb-filters" role="navigation" aria-label={t('common.knowledgeBases')}>
              <a href="/platform/knowledge-bases" className={`plat-shell__kb-filter${currentScope === 'all' ? ' plat-shell__kb-filter--active' : ''}`}>{t('common.all')}</a>
              <a href="/platform/knowledge-bases?scope=mine" className={`plat-shell__kb-filter${currentScope === 'mine' ? ' plat-shell__kb-filter--active' : ''}`}>{t('knowledgeList.sections.mine')}</a>
            </div>
          )}

          {/* Vue menu.vue .submenu: the grouped session list lives in the
              sidebar on every protected page; collapsed sidebars hide it.
              The visible 我的对话 title mirrors Vue's session-area label
              (menu.myChats → SessionSourceFilter trigger, menu.vue:109-112/332). */}
          {!collapsed && (
            <nav className="plat-shell__sessions" aria-label={labels.myChats}>
              <h2 className="plat-shell__sessions-title">{labels.myChats}</h2>
              <SessionSidebarList
                groups={sessionListGroups}
                selectedSessionId={activeChatId}
                loading={sessionsLoading}
                emptyLabel={labels.noSessions}
                untitledLabel={labels.newSession}
                onSelect={openShellSession}
                onRename={renameShellSession}
                onTogglePin={toggleShellSessionPin}
                onClear={clearShellSessionMessages}
                onDelete={deleteShellSession}
              />
            </nav>
          )}
        </div>

        <div className="plat-shell__bottom">
          <div className={`plat-shell__user${menuOpen ? ' plat-shell__user--open' : ''}`}>
            <button type="button" className="plat-shell__user-button" aria-haspopup="menu" aria-expanded={menuOpen}
              data-guide="user-menu"
              onClick={() => setMenuOpen((open) => !open)}>
              <span className="plat-shell__avatar" aria-hidden="true">
                {user.avatar ? <img src={user.avatar} alt="" /> : <span className="plat-shell__avatar-initial">{initial}</span>}
              </span>
              {!collapsed && (
                <span className="plat-shell__user-info">
                  <span className="plat-shell__user-name">{user.name || '—'}</span>
                  <span className="plat-shell__user-email">{user.email}</span>
                </span>
              )}
            </button>
            {menuOpen && (
              <div className="plat-shell__dropdown" role="menu">
                {/* Vue UserMenu.vue:45-50,501-504 — a help-circle entry labelled
                    $t('newUserGuide.reopen') re-opens the welcome tour by
                    dispatching weknora:open-new-user-guide; the NewUserGuide
                    host opens on that event even when the done-key is '1',
                    so the tour replays without touching the stored key. */}
                <button type="button" role="menuitem" className="plat-shell__dropdown-item"
                  data-testid="plat-shell-guide-reopen"
                  aria-label={labels.reopenGuide}
                  onClick={() => { setMenuOpen(false); openNewUserGuide(); }}>
                  <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor"
                    strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <circle cx="12" cy="12" r="9" />
                    <path d="M9.4 9.4a2.6 2.6 0 1 1 3.7 2.4c-.8.4-1.1.9-1.1 1.7" />
                    <line x1="12" y1="16.6" x2="12" y2="16.7" />
                  </svg>
                  {labels.reopenGuide}
                </button>
                <a role="menuitem" className="plat-shell__dropdown-item"
                  href="/platform/settings?section=userprofile"
                  onClick={() => setMenuOpen(false)}>
                  {labels.personalSettings}
                </a>
                <div className="plat-shell__dropdown-divider" aria-hidden="true" />
                <button type="button" role="menuitem" className="plat-shell__dropdown-item plat-shell__dropdown-item--danger"
                  onClick={() => { setMenuOpen(false); void onLogout(); }}>
                  {t('auth.logout')}
                </button>
              </div>
            )}
          </div>
        </div>
      </aside>
      {/* The shell owns the session list (Vue chat/index.vue has no sidebar
          of its own): chat pages under the shell suppress their in-page one. */}
      <div className="plat-shell__outlet">
        <SessionSidebarShellContext.Provider value={true}>{children}</SessionSidebarShellContext.Provider>
      </div>
      <GlobalCommandPalette
        open={paletteOpen}
        initialQuery={paletteQuery}
        recentQueries={recentQueries}
        locale={locale}
        onClose={closePalette}
        onNavigate={navigateFromPalette}
        onSearch={recordPaletteSearch}
        onClearRecent={clearPaletteRecent}
      />
      {/* 带遮罩层的新手引导：首次进入自动开启 (Vue platform/index.vue:18). */}
      <NewUserGuide locale={locale} actions={guideActions} />
      {/* Contextual guides (Vue mounts ContextualGuide/KbCreateContextualGuide/
          AgentCreateContextualGuide/TenantModelsGuide per page): one host in
          the shell is fed by openContextualGuide(tour) trigger calls from the
          pages, so page wiring stays a one-liner. */}
      <ContextualGuideHost locale={locale} actions={guideActions} />
    </div>
  );
}