import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode, type UIEvent } from 'react';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import type { ChatSession, createWeKnoraClient } from '@weknora/api-client';
import { sessionGroups } from '@weknora/domain/chat/session-state';
import { GlobalCommandPalette } from './GlobalCommandPalette.tsx';
import { SessionSidebarList, SessionSidebarShellContext, type SessionGroupView, type SessionSourceOption } from '../../../../packages/views/src/chat/session-sidebar.tsx';
import { chatSessionIdFromPath, SHELL_SESSION_ROUTE_EVENT } from '../chat/session-route.ts';
import { ContextualGuideHost } from '../../../../packages/views/src/guides/ContextualGuide.tsx';
import { NewUserGuide } from '../../../../packages/views/src/guides/NewUserGuide.tsx';
import { openNewUserGuide } from '../../../../packages/views/src/guides/new-user-guide.ts';
import {
  clearRecentQueries,
  consumeCmdkParam,
  decideGlobalShortcutAction,
  loadRecentQueries,
  pushRecentQuery,
  recentQueriesStorageKey,
} from './command-palette.ts';
import { readReactPlatformState } from './legacy-session.ts';
// Welcome-tour styles live with the component in @weknora/views; the package
// itself must stay css-import-free for the shared typecheck, so the shell
// pulls it in by relative path. (shell.css is gone — all rules became
// utilities in this file / session-sidebar.tsx.)
import '../../../../packages/views/src/guides/guides.css';
import weknoraLogo from '../auth/assets/weknora.png';

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

// .plat-shell__kb-filter (+ --active) → utilities. --active survives as a
// hook class for tests; active color/font swap here (the :hover gray keeps
// winning over the active blue, matching the deleted css cascade).
const kbFilterClass = (active: boolean): string =>
  'plat-shell__kb-filter block mx-[8px] pt-[6px] pr-[8px] pb-[6px] pl-[8px] rounded-[6px] text-[13px] no-underline hover:bg-[#eceff4] hover:text-[#3d4a5c] '
  + (active ? 'plat-shell__kb-filter--active text-[#2e6de6] font-semibold' : 'text-[#66758b]');

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
const SHELL_SESSION_PAGE_SIZE = 30;

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
    sessionLoadError: formatMessage(locale, 'common.error'),
    renameSession: formatMessage(locale, 'menu.renameSession'),
    newSession: formatMessage(locale, 'menu.newSession'),
  };

  const [pathname, setPathname] = useState(() => window.location.pathname);
  const [collapsed, setCollapsed] = useState(() => window.localStorage.getItem(COLLAPSE_STORAGE_KEY) === 'true');
  const [menuOpen, setMenuOpen] = useState(false);
  const userMenuRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!menuOpen) return;
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target;
      if (target instanceof Node && userMenuRef.current?.contains(target)) return;
      setMenuOpen(false);
    };
    document.addEventListener('click', handleClickOutside);
    return () => document.removeEventListener('click', handleClickOutside);
  }, [menuOpen]);
  // Welcome tour (Vue mounts NewUserGuide in platform/index.vue). The models
  // step opens the settings section in place; the shell remembers that it
  // navigated so leaving the step can return to the previous page, mirroring
  // Vue's uiStore.openSettings/closeSettings pair.
  const guideOpenedSettingsRef = useRef(false);
  const [user, setUser] = useState<{ id: string; name: string; email: string; avatar: string }>({ id: '', name: '', email: '', avatar: '' });
  // Vue menu.ts:72-81 — the organizations nav entry is gated on
  // hasRole('admin') (owner/admin pass; viewer/contributor manage nothing in
  // the shared space). Initial true = fail-open while identity resolves:
  // the shell mounts before auth/me lands, and the server route guard
  // remains the real boundary (same posture as OrganizationsPage's
  // canManageOrg).
  const [canSeeOrganizations, setCanSeeOrganizations] = useState(true);
  const [canSeeAdminSessionSources, setCanSeeAdminSessionSources] = useState(false);
  const authResolvedClientRef = useRef<Client | null>(null);

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
    authResolvedClientRef.current = null;
    void client.auth.me().then((me) => {
      if (!active) return;
      authResolvedClientRef.current = client;
      const record = me.user as Record<string, unknown>;
      setUser({
        id: typeof record.id === 'string' ? record.id : typeof record.id === 'number' ? String(record.id) : '',
        name: typeof record.username === 'string' && record.username ? record.username : '—',
        email: typeof record.email === 'string' ? record.email : '',
        avatar: typeof record.avatar === 'string' ? record.avatar : '',
      });
      // R017 RBAC self-resolution (OrganizationsPage parity, Vue
      // currentTenantRole): the active-tenant membership role — selected
      // tenant first, falling back to the home tenant — decides entry
      // visibility, with the can_access_all_tenants superuser flag passing
      // the admin gate. UI rendering only; the server route guard is the
      // real boundary. An unknown role ('' — membership data absent, e.g.
      // embedded/test mounts) fails open and keeps the entry visible.
      const selectedTenantId = readReactPlatformState(window.localStorage)?.tenantId ?? null;
      const homeTenantId = me.tenant && me.tenant.id !== null && me.tenant.id !== undefined ? String(me.tenant.id) : '';
      const tenantId = selectedTenantId ?? homeTenantId;
      let currentRole = '';
      for (const item of me.memberships ?? []) {
        if (!item || typeof item !== 'object') continue;
        const row = item as Record<string, unknown>;
        const id = row.tenant_id ?? row.tenantId;
        if (tenantId && String(id) === tenantId && typeof row.role === 'string') { currentRole = row.role; break; }
      }
      setCanSeeOrganizations(
        currentRole === '' || currentRole === 'admin' || currentRole === 'owner' || record.can_access_all_tenants === true,
      );
      setCanSeeAdminSessionSources(currentRole === 'admin' || currentRole === 'owner' || record.can_access_all_tenants === true);
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
  // chat page uses (client.sessions.list); later pages load as the sidebar is
  // scrolled, matching Vue menu.vue's bucket continuation behavior.
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [sessionSource, setSessionSource] = useState('web');
  const [sessionSourceOptions, setSessionSourceOptions] = useState<SessionSourceOption[]>([
    { value: 'web', label: labels.myChats },
  ]);
  const sessionSourceOptionsRef = useRef(sessionSourceOptions);
  sessionSourceOptionsRef.current = sessionSourceOptions;
  const sessionsClientRef = useRef<Client | null>(null);
  const sessionsScopeRef = useRef(canSeeAdminSessionSources);
  const [sessionsLoading, setSessionsLoading] = useState(true);
  const sessionsRef = useRef<ChatSession[]>([]);
  const sessionsTotalRef = useRef(0);
  const sessionsPageRef = useRef(0);
  const sessionsRequestRef = useRef(false);
  const sessionsMountedRef = useRef(false);
  const sessionsGenerationRef = useRef(0);
  const [sessionsLoadError, setSessionsLoadError] = useState(false);
  useEffect(() => {
    if (!sessionSourceOptions.some((option) => option.value === sessionSource)) setSessionSource('web');
  }, [sessionSource, sessionSourceOptions]);
  const loadShellSessionPage = useCallback(async (page: number, generation: number) => {
    if (!sessionsMountedRef.current || generation !== sessionsGenerationRef.current || sessionsRequestRef.current) return;
    // A bucket can disappear after an auth/client scope refresh. Let the
    // source effect restart from web rather than issuing a stale privileged
    // request during that render transition.
    if (!sessionSourceOptionsRef.current.some((option) => option.value === sessionSource)) {
      setSessionSource('web');
      return;
    }
    sessionsRequestRef.current = true;
    setSessionsLoading(true);
    try {
      const apiSource = sessionSource.startsWith('im:') ? sessionSource.slice('im:'.length) : sessionSource;
      const result = await client.sessions.list({ page, pageSize: SHELL_SESSION_PAGE_SIZE, source: apiSource });
      if (!sessionsMountedRef.current || generation !== sessionsGenerationRef.current) return;
      const incoming = page === 1
        ? result.data
        : [...sessionsRef.current, ...result.data.filter((session) => !sessionsRef.current.some((item) => item.id === session.id))];
      sessionsRef.current = incoming;
      sessionsTotalRef.current = result.total;
      sessionsPageRef.current = page;
      setSessionsLoadError(false);
      setSessions(incoming);
    } catch {
      if (sessionsMountedRef.current && generation === sessionsGenerationRef.current) setSessionsLoadError(true);
    } finally {
      if (generation === sessionsGenerationRef.current) sessionsRequestRef.current = false;
      if (sessionsMountedRef.current && generation === sessionsGenerationRef.current) setSessionsLoading(false);
    }
  }, [canSeeAdminSessionSources, client, sessionSource]);
  useEffect(() => {
    const clientChanged = sessionsClientRef.current !== null && sessionsClientRef.current !== client;
    const scopeChanged = sessionsClientRef.current !== null && sessionsScopeRef.current !== canSeeAdminSessionSources;
    sessionsClientRef.current = client;
    sessionsScopeRef.current = canSeeAdminSessionSources;
    if (clientChanged || scopeChanged) {
      setSessionSource('web');
      return () => { sessionsMountedRef.current = false; };
    }
    sessionsMountedRef.current = true;
    const generation = ++sessionsGenerationRef.current;
    sessionsRequestRef.current = false;
    sessionsRef.current = [];
    sessionsTotalRef.current = 0;
    sessionsPageRef.current = 0;
    setSessions([]);
    setSessionsLoadError(false);
    void loadShellSessionPage(1, generation);
    return () => {
      sessionsMountedRef.current = false;
    };
  }, [loadShellSessionPage]);

  // Vue menu.vue discovers configured IM/embed channels before building the
  // source filter. The React client exposes the same tenant-scoped endpoints;
  // only verified channel ids are surfaced, so an unavailable endpoint cannot
  // manufacture a bucket. Admin-only sources stay absent for viewers and
  // unknown/test mounts until auth.me proves the current tenant role.
  useEffect(() => {
    let active = true;
    const loadSourceOptions = async () => {
      const options: SessionSourceOption[] = [
        { value: 'web', label: labels.myChats },
      ];
      if (authResolvedClientRef.current !== client || !canSeeAdminSessionSources) {
        if (active) setSessionSourceOptions(options);
        return;
      }
      const embedApi = client.embed;
      const [embedResult, imResult] = await Promise.allSettled([
        embedApi?.channels?.listAll ? embedApi.channels.listAll() : Promise.reject(new Error('embed channels unavailable')),
        embedApi?.im?.listAll ? embedApi.im.listAll() : Promise.reject(new Error('im channels unavailable')),
      ]);
      if (!active) return;
      const candidates: Array<{ option: SessionSourceOption; apiSource: string }> = [
        { option: { value: 'api', label: t('menu.apiChats') }, apiSource: 'api' },
      ];
      if (embedResult.status === 'fulfilled') {
        for (const channel of embedResult.value) {
          if (!channel || typeof channel.id !== 'string' || channel.id.trim() === '') continue;
          candidates.push({ option: { value: `embed:${channel.id}`, label: typeof channel.name === 'string' && channel.name.trim() ? channel.name : channel.id }, apiSource: `embed:${channel.id}` });
        }
      }
      if (imResult.status === 'fulfilled') {
        const seen = new Set<string>();
        for (const channel of imResult.value) {
          if (!channel || typeof channel.platform !== 'string' || channel.platform.trim() === '' || seen.has(channel.platform)) continue;
          seen.add(channel.platform);
          candidates.push({ option: { value: `im:${channel.platform}`, label: channel.platform }, apiSource: channel.platform });
        }
      }
      const checked = await Promise.allSettled(candidates.map(async (candidate) => ({
        candidate,
        result: await client.sessions.list({ page: 1, pageSize: 1, source: candidate.apiSource }),
      })));
      if (!active) return;
      for (const item of checked) {
        if (item.status === 'fulfilled' && item.value.result.total > 0) options.push(item.value.candidate.option);
      }
      setSessionSourceOptions(options);
    };
    void loadSourceOptions();
    return () => { active = false; };
  }, [canSeeAdminSessionSources, client, labels.myChats, t]);

  const onSessionsScroll = useCallback((event: UIEvent<HTMLDivElement>) => {
    const element = event.currentTarget;
    if (sessionsRequestRef.current) return;
    if (element.scrollHeight - (element.scrollTop + element.clientHeight) > 80) return;
    const generation = sessionsGenerationRef.current;
    if (sessionsLoadError) {
      void loadShellSessionPage(1, generation);
      return;
    }
    if (sessionsTotalRef.current === 0 || sessionsRef.current.length >= sessionsTotalRef.current) return;
    void loadShellSessionPage(sessionsPageRef.current + 1, generation);
  }, [loadShellSessionPage, sessionsLoadError]);
  const retryShellSessions = useCallback(() => {
    setSessionsLoadError(false);
    void loadShellSessionPage(1, sessionsGenerationRef.current);
  }, [loadShellSessionPage]);

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

  async function renameShellSession(sessionId: string, title?: string): Promise<void> {
    const current = sessions.find((session) => session.id === sessionId);
    if (!current || !title || title === current.title) return;
    try {
      const updated = await client.sessions.update(sessionId, { title, description: current?.description });
      const nextSessions = sessionsRef.current.map((session) => session.id === sessionId ? updated : session);
      sessionsRef.current = nextSessions;
      setSessions(nextSessions);
    } catch (error) {
      throw error instanceof Error ? error : new Error('修改标题失败');
    }
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
    sessionsRef.current = sessionsRef.current.filter((session) => session.id !== sessionId);
    sessionsTotalRef.current = Math.max(0, sessionsTotalRef.current - 1);
    setSessions(sessionsRef.current);
    // Vue menu.vue: deleting the open session routes back to creatChat.
    if (chatSessionIdFromPath(window.location.pathname) === sessionId) {
      window.location.assign('/platform/creatChat');
    }
  }

  async function deleteShellSessions(sessionIds: readonly string[]): Promise<boolean> {
    await client.sessions.batchRemove(sessionIds);
    const selected = new Set(sessionIds);
    sessionsRef.current = sessionsRef.current.filter((session) => !selected.has(session.id));
    sessionsTotalRef.current = Math.max(0, sessionsTotalRef.current - sessionIds.length);
    setSessions(sessionsRef.current);
    if (chatSessionIdFromPath(window.location.pathname) && selected.has(chatSessionIdFromPath(window.location.pathname)!)) {
      window.location.assign('/platform/creatChat');
    }
    // Rebase the paginated window from page 1 after a destructive mutation so
    // rows shifted from later pages are not skipped by the old offset.
    sessionsRef.current = [];
    sessionsTotalRef.current = 0;
    sessionsPageRef.current = 0;
    setSessions([]);
    await loadShellSessionPage(1, sessionsGenerationRef.current);
    return true;
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
  // Vue menu.ts:76-78 — drop the organizations entry below admin. Filtering
  // after the build keeps the item table (labels/icons/guides) authoritative.
  const visibleNavItems = useMemo(
    () => navItems.filter((item) => item.key !== 'organizations' || canSeeOrganizations),
    [navItems, canSeeOrganizations],
  );

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

  // R017 organizations sub-filter: the Vue org rail (ListSpaceSidebar
  // mode="organization", OrganizationList.vue:3-4) carries 全部/我创建的/
  // 我加入的; the shell surfaces the same entries under the organizations nav
  // item on /platform/organizations and drives them through the shared
  // ?scope= convention (all|created|joined) that the page reads back.
  const orgListContext = pathname === '/platform/organizations';
  const currentOrgScope = (() => {
    const value = new URLSearchParams(window.location.search).get('scope');
    return value === 'created' || value === 'joined' ? value : 'all';
  })();

  const toggleCollapsed = () => {
    setCollapsed((current) => {
      window.localStorage.setItem(COLLAPSE_STORAGE_KEY, String(!current));
      return !current;
    });
  };

  const initial = (user.name || '?').charAt(0).toUpperCase();

  return (
    // shell.css → utilities: .plat-shell (flex row, full viewport), .plat-shell__aside
    // (+ collapsed state swaps width/padding values rather than layering overrides).
    <div className="flex items-stretch w-full h-screen min-w-[600px] bg-white">
      <aside className={collapsed
        ? 'box-border flex flex-col min-w-[60px] w-[60px] pt-[8px] px-[3px] pb-[6px] bg-[#f6f8fa] border-r border-[#e7ebf0] shadow-[1px_0_0_rgba(0,0,0,0.02)] overflow-hidden transition-[width,min-width] duration-[250ms] ease-[ease]'
        : 'box-border flex flex-col min-w-[260px] w-[260px] pt-[8px] px-[6px] pb-[6px] bg-[#f6f8fa] border-r border-[#e7ebf0] shadow-[1px_0_0_rgba(0,0,0,0.02)] overflow-hidden transition-[width,min-width] duration-[250ms] ease-[ease]'}>
        <div className="flex items-center justify-between h-[50px] shrink-0 pr-[10px] pl-[14px]">
          <a className="flex min-w-0 flex-1 items-center gap-[8px] overflow-hidden no-underline text-inherit" href="/platform/knowledge-bases" aria-label="WeKnora">
            {!collapsed && <img className="block h-auto w-[128px]" src={weknoraLogo} alt="" />}
          </a>
          {!collapsed && (
            <div className="flex shrink-0 items-center gap-1">
              <button type="button" className="inline-flex h-[26px] w-[26px] shrink-0 cursor-pointer items-center justify-center rounded-[6px] border-0 bg-transparent text-[#66758b] transition-colors hover:bg-[#eceff4]" onClick={() => { setPaletteQuery(''); setPaletteOpen(true); }} aria-label={t('menu.search')} title={t('menu.search')}>
                <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true">
                  <circle cx="11" cy="11" r="6.5" />
                  <path d="m16 16 4.5 4.5" />
                </svg>
              </button>
              <button type="button" className="inline-flex items-center justify-center w-[18px] h-[18px] border-none rounded-[4px] bg-transparent text-[#66758b] cursor-pointer hover:bg-[#eceff4]" onClick={toggleCollapsed} aria-label={t('menu.collapseSidebar')} title={t('menu.collapseSidebar')}>
              <svg viewBox="0 0 20 20" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.2" aria-hidden="true">
                <rect x="1.5" y="1.5" width="17" height="17" rx="3" />
                <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" />
                <line x1="5" y1="10" x2="3" y2="8" strokeLinecap="round" />
                <line x1="5" y1="10" x2="3" y2="12" strokeLinecap="round" />
              </svg>
              </button>
            </div>
          )}
        </div>
        {/* .plat-shell__item + --collapsed descendant override → utilities
            (collapsed form: centered, 4px side margins, 9px/0 padding). */}
        {collapsed && (
          <button type="button" className="plat-shell__toggle-item flex items-center justify-center gap-[8px] mx-[4px] py-[9px] px-0 rounded-[8px] no-underline text-[#3d4a5c] text-[14px] whitespace-nowrap hover:bg-[#eceff4]" onClick={toggleCollapsed} aria-label={t('menu.expandSidebar')} title={t('menu.expandSidebar')}>
            <svg viewBox="0 0 20 20" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.2" aria-hidden="true">
              <rect x="1.5" y="1.5" width="17" height="17" rx="3" />
              <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" />
              <line x1="4" y1="7.5" x2="4" y2="12.5" strokeLinecap="round" />
            </svg>
          </button>
        )}

        {/* .plat-shell__top / __nav → utilities. Item base styles from
            .plat-shell__item; --active (+ :hover pin) and the collapsed
            descendant override become state-swapped utilities keyed off
            active/collapsed (aria-current="page" already marks the active
            entry for semantics). --inset-x inlined: pl-[14px]. */}
        <div className="flex-1 min-h-0 overflow-y-auto pt-[6px]" onScroll={onSessionsScroll}>
          <nav className="flex flex-col gap-[2px]" aria-label="Platform">
            {visibleNavItems.map((item) => {
              const active = item.match(pathname);
              return (
                <a key={item.key} href={item.href} className={(collapsed
                  ? 'justify-center mx-[4px] px-0 '
                  : 'mx-[8px] px-[8px] ')
                  + 'flex items-center gap-[8px] py-[9px] rounded-[8px] no-underline text-[14px] whitespace-nowrap '
                  + (active
                    ? 'bg-[#f3f3f3] hover:bg-[#f3f3f3] text-[#07c05f] font-medium'
                    : 'text-[#3d4a5c] hover:bg-[#eceff4]')}
                  aria-current={active ? 'page' : undefined} title={collapsed ? item.label : undefined} data-guide={item.guide}>
                  <span className="inline-flex shrink-0">{item.icon}</span>
                  {!collapsed && <span className="overflow-hidden text-ellipsis">{item.label}</span>}
                </a>
              );
            })}
          </nav>

          {kbListContext && !collapsed && (
            <div className="flex flex-col gap-[2px] mt-[10px] mb-[4px] pt-[8px] border-t border-[#e7ebf0]" role="navigation" aria-label={t('common.knowledgeBases')}>
              <a href="/platform/knowledge-bases" className={kbFilterClass(currentScope === 'all')}>{t('common.all')}</a>
              <a href="/platform/knowledge-bases?scope=mine" className={kbFilterClass(currentScope === 'mine')}>{t('knowledgeList.sections.mine')}</a>
            </div>
          )}

          {/* R017: shared-space scope entries (Vue menu.vue has no organizations
              submenu — only the pending-requests badge on the nav entry — so
              the KB quick-filter block is the established shell form; counts
              stay on the page rail like Vue's ListSpaceSidebar). Class reuse
              is intentional: same visual language; the
              aria-label (menu.organizations) disambiguates the blocks. */}
          {orgListContext && !collapsed && (
            <div className="flex flex-col gap-[2px] mt-[10px] mb-[4px] pt-[8px] border-t border-[#e7ebf0]" role="navigation" aria-label={labels.organizations}>
              <a href="/platform/organizations" className={kbFilterClass(currentOrgScope === 'all')}>{t('common.all')}</a>
              <a href="/platform/organizations?scope=created" className={kbFilterClass(currentOrgScope === 'created')}>{t('organization.createdByMe')}</a>
              <a href="/platform/organizations?scope=joined" className={kbFilterClass(currentOrgScope === 'joined')}>{t('organization.joinedByMe')}</a>
            </div>
          )}

          {/* Vue menu.vue .submenu: the grouped session list lives in the
              sidebar on every protected page; collapsed sidebars hide it.
              The visible 我的对话 title mirrors Vue's session-area label
              (menu.myChats → SessionSourceFilter trigger, menu.vue:109-112/332). */}
          {/* .plat-shell__sessions / __sessions-title → utilities. The
              shell-context group-list indent (padding 0 6px on the session
              <ul>) rides along as an arbitrary-variant utility so the
              fallback chat sidebar (outside the shell) keeps its flush list.
              plat-shell__kb-filter--active is kept as a className hook for
              the subfilter tests (no styling of its own beyond the swapped
              color utilities below). */}
          {!collapsed && (
            <nav className="mt-[8px] mb-[4px] pt-[8px] border-t border-[#e7ebf0] [&_ul]:px-[6px]" aria-label={labels.myChats}>
              <h2 className="m-0 px-[14px] pb-[4px] text-[#8b97a8] text-[12px] font-semibold leading-[1.4]">{labels.myChats}</h2>
              {sessionsLoadError && !sessionsLoading ? <p className="mx-[14px] my-2 text-xs text-[#b42318]" role="status">
                {labels.sessionLoadError}{' '}<button type="button" className="cursor-pointer border-0 bg-transparent p-0 text-xs text-[#07c05f] underline" onClick={retryShellSessions}>{t('common.retry')}</button>
              </p> : null}
              <SessionSidebarList
                groups={sessionListGroups}
                selectedSessionId={activeChatId}
                loading={sessionsLoading}
                emptyLabel={sessionsLoadError ? undefined : labels.noSessions}
                untitledLabel={labels.newSession}
                source={sessionSource}
                sourceOptions={sessionSourceOptions.length > 1 ? sessionSourceOptions : undefined}
                onSourceChange={sessionSourceOptions.length > 1 ? setSessionSource : undefined}
                onSelect={openShellSession}
                onRename={renameShellSession}
                onTogglePin={toggleShellSessionPin}
                onClear={clearShellSessionMessages}
                onDelete={deleteShellSession}
                onBatchDelete={deleteShellSessions}
              />
            </nav>
          )}
        </div>

        {/* .plat-shell__bottom / __user(+--open, unused marker dropped) /
            __user-button / __avatar(+img) / __avatar-initial / __user-info /
            __user-name / __user-email / __dropdown / __dropdown-item
            (+ --danger swap) / __dropdown-divider → utilities. */}
        <div className="shrink-0 px-[2px] py-[4px]">
          <div ref={userMenuRef} className="relative">
            <button type="button" className="flex items-center gap-[6px] w-full px-[6px] py-[8px] border-none rounded-[8px] bg-transparent cursor-pointer text-left hover:bg-[#eceff4]" aria-haspopup="menu" aria-expanded={menuOpen}
              data-guide="user-menu"
              onClick={() => setMenuOpen((open) => !open)}>
              <span className="inline-flex items-center justify-center w-[24px] h-[24px] rounded-full overflow-hidden shrink-0 bg-[linear-gradient(135deg,#2e6de6_0%,#1f56c2_100%)]" aria-hidden="true">
                {user.avatar ? <img src={user.avatar} alt="" className="w-full h-full object-cover" /> : <span className="text-white text-[12px] font-semibold leading-[1]">{initial}</span>}
              </span>
              {!collapsed && (
                <span className="flex flex-col gap-[2px] min-w-0 flex-1">
                  <span className="text-[14px] font-medium text-[#1f2733] whitespace-nowrap overflow-hidden text-ellipsis">{user.name || '—'}</span>
                  <span className="text-[12px] text-[#66758b] whitespace-nowrap overflow-hidden text-ellipsis">{user.email}</span>
                </span>
              )}
            </button>
            {menuOpen && (
              <div className="absolute bottom-[calc(100%_+_6px)] left-[-4px] right-[-5px] bg-white border border-[#e7ebf0] rounded-[8px] shadow-[0_4px_20px_rgba(0,0,0,0.12)] overflow-hidden z-[1000]" role="menu">
                {/* Vue UserMenu.vue:45-50,501-504 — a help-circle entry labelled
                    $t('newUserGuide.reopen') re-opens the welcome tour by
                    dispatching weknora:open-new-user-guide; the NewUserGuide
                    host opens on that event even when the done-key is '1',
                    so the tour replays without touching the stored key. */}
                <button type="button" role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
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
                <a role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
                  href="/platform/settings?section=userprofile"
                  onClick={() => setMenuOpen(false)}>
                  {labels.personalSettings}
                </a>
                <div className="h-[1px] bg-[#e7ebf0] my-[3px]" aria-hidden="true" />
                <button type="button" role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#d54941] no-underline hover:bg-[#fbe9e8]"
                  onClick={() => { setMenuOpen(false); void onLogout(); }}>
                  {t('auth.logout')}
                </button>
              </div>
            )}
          </div>
        </div>
      </aside>
      {/* The shell owns the session list (Vue chat/index.vue has no sidebar
          of its own): chat pages under the shell suppress their in-page one.
          .plat-shell__outlet → utilities; the descendant page overrides from
          shell.css land here: children fill the column (min-height 0) and
          legacy .wk-page pages keep the scrollable full-height full-width
          treatment (max-w-none! must beat the unlayered styles.css
          .wk-page max-width, which otherwise wins over layered utilities). */}
      <div className="flex-1 min-w-0 min-h-0 flex flex-col overflow-hidden [&>*]:min-h-0 [&_.wk-page]:h-full [&_.wk-page]:overflow-y-auto [&_.wk-page]:max-w-none!">
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
