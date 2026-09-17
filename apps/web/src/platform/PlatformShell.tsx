import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode, type UIEvent } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { formatMessage, type Locale } from '@weknora/i18n';
import { usePreferredLocale } from '../locale.ts';
import type { ChatSession, createWeKnoraClient } from '@weknora/api-client';
import { sessionGroups } from '@weknora/domain/chat/session-state';
import { GlobalCommandPalette } from './GlobalCommandPalette.tsx';
import { SessionSidebarList, SessionSidebarShellContext, type SessionGroupView, type SessionSourceOption } from '../../../../packages/views/src/chat/session-sidebar.tsx';
import { resolveChatCopy } from '../../../../packages/views/src/chat/chat-copy.ts';
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
import { InvitationInbox } from './InvitationInbox.tsx';
import { navigate, subscribeNavigation } from './navigation.ts';
// Welcome-tour styles live with the component in @weknora/views; the package
// itself must stay css-import-free for the shared typecheck, so the shell
// pulls it in by relative path. (shell.css is gone — all rules became
// utilities in this file / session-sidebar.tsx.)
import '../../../../packages/views/src/guides/guides.css';
import weknoraLogo from '../auth/assets/weknora.png';

type Client = ReturnType<typeof createWeKnoraClient>;

function handleInternalLink(event: ReactMouseEvent<HTMLAnchorElement>, path: string, afterNavigate?: () => void): void {
  if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
  event.preventDefault();
  afterNavigate?.();
  navigate(path);
}

export interface PlatformShellProps {
  client: Client;
  onLogout: () => void | Promise<void>;
  onTenantSwitch?: (tenantId: string) => Promise<void>;
  children: ReactNode;
}

// R446 D7 — the user menu 「退出」 item must ALWAYS land the user on the login
// page, matching Vue UserMenu.vue:518-536 handleLogout: the logout API call may
// fail (Vue swallows the error) but local cleanup plus navigation still win.
// A rejected onLogout means the parent chain died before it could navigate, and
// a promise that never settles (hung transport / refresh coordinator) strands
// the user in the shell forever — both cases fall back to a hard navigation.
export const SHELL_LOGOUT_FALLBACK_MS = 4000;

export async function runShellLogout(
  onLogout: () => void | Promise<void>,
  nav: (url: string) => void = (url) => window.location.assign(url),
  fallbackMs: number = SHELL_LOGOUT_FALLBACK_MS,
): Promise<void> {
  // Object holder: the callbacks below mutate async, and TS's control-flow
  // analysis would wrongly narrow a `let` scalar to its initializer.
  const outcome = { status: 'timeout' as 'ok' | 'failed' | 'timeout' };
  await Promise.race([
    Promise.resolve()
      .then(onLogout)
      .then(() => { outcome.status = 'ok'; }, () => { outcome.status = 'failed'; }),
    new Promise<void>((resolve) => setTimeout(resolve, fallbackMs)),
  ]);
  if (outcome.status !== 'ok') nav('/login');
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

function Icon({ path }: { path: string | string[] }): ReactNode {
  const paths = Array.isArray(path) ? path : [path];
  return (
    <svg className="plat-shell__icon" viewBox="0 0 24 24" width="18" height="18" fill="none"
      stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {paths.map((d, index) => <path key={`${d}-${index}`} d={d} />)}
    </svg>
  );
}

const ICONS = {
  // These paths are ports of frontend/src/assets/img/{prefixIcon,zhishiku,agent,organization}.svg.
  // Vue's filled/outlined geometry is the visual authority for the platform rail.
  chat: [
    'M1.875 2.5C1.875 2.15483 2.15482 1.875 2.5 1.875H17.5C17.8451 1.875 18.125 2.15483 18.125 2.5V13.75C18.125 14.0951 17.8451 14.375 17.5 14.375H9.04274L6.10514 17.9001C5.93669 18.1023 5.65965 18.1773 5.41224 18.0876C5.16481 17.9981 5 17.7631 5 17.5V14.375H2.5C2.15482 14.375 1.875 14.0951 1.875 13.75V2.5Z',
    'M3.125 3.125V13.125H5.625C5.97017 13.125 6.25 13.4049 6.25 13.75V15.7738L8.26986 13.3499C8.38861 13.2074 8.56451 13.125 8.75 13.125H16.875V3.125H3.125Z',
    'M9.375 5H10.625V8.44961L13.5154 10.762L12.7346 11.738L9.375 9.05039V5Z',
  ],
  book: [
    'M9.17736 1.28207C9.32257 1.23367 9.4805 1.24024 9.6212 1.30054L18.3713 5.05054C18.5619 5.13222 18.6995 5.30316 18.7388 5.50681C18.778 5.71045 18.7136 5.9203 18.567 6.06694C18.5138 6.12012 18.4366 6.24314 18.3744 6.46076C18.3146 6.66994 18.2812 6.92361 18.2812 7.1875V13.4375C18.2812 13.7014 18.3146 13.955 18.3744 14.1642C18.4366 14.3819 18.5138 14.5049 18.567 14.558C18.709 14.7001 18.7741 14.9017 18.7419 15.1001C18.7097 15.2984 18.5843 15.4691 18.4045 15.559L12.1545 18.684C11.9886 18.767 11.7944 18.772 11.6245 18.6976L1.62449 14.3226C1.397 14.223 1.25 13.9983 1.25 13.75V4.375C1.25 4.106 1.42215 3.86715 1.67736 3.78207L9.17736 1.28207Z',
    'M2.5 5.33064L10.9868 9.04362C10.826 9.58006 10.7812 9.95139 10.7812 10.3125V16.5625C10.7812 16.6986 10.7876 16.8361 10.8007 16.9728L2.5 13.3413V5.33064Z',
    'M12.1275 17.3C12.0646 17.08 12.0312 16.8264 12.0312 16.5625V15.8358L17.0432 13.1146C17.0312 13.3304 17.076 13.7986 17.1725 14.5076L12.1275 17.3Z',
  ],
  bot: [
    'M10 3L10.8 6.2C10.9 6.7 11.3 7.1 11.8 7.2L15 8L11.8 8.8C11.3 8.9 10.9 9.3 10.8 9.8L10 13L9.2 9.8C9.1 9.3 8.7 8.9 8.2 8.8L5 8L8.2 7.2C8.7 7.1 9.1 6.7 9.2 6.2L10 3Z',
    'M15.5 4L15.8 5.2C15.85 5.45 16.05 5.65 16.3 5.7L17.5 6L16.3 6.3C16.05 6.35 15.85 6.55 15.8 6.8L15.5 8L15.2 6.8C15.15 6.55 14.95 6.35 14.7 6.3L13.5 6L14.7 5.7C14.95 5.65 15.15 5.45 15.2 5.2L15.5 4Z',
    'M4.5 13L4.8 14.2C4.85 14.45 5.05 14.65 5.3 14.7L6.5 15L5.3 15.3C5.05 15.35 4.85 15.55 4.8 15.8L4.5 17L4.2 15.8C4.15 15.55 3.95 15.35 3.7 15.3L2.5 15L3.7 14.7C3.95 14.45 4.15 14.45 4.2 14.2L4.5 13Z',
  ],
  users: 'M10 10C8.8 7.5 7.8 3.8 4.8 3.8C2.2 3.8 0.8 6.8 0.8 10C0.8 13.2 2.2 16.2 4.8 16.2C7.8 16.2 8.8 12.5 10 10C11.2 7.5 12.5 5.5 14.5 5.5C16.5 5.5 18 7.5 18 10C18 12.5 16.5 14.5 14.5 14.5C12.5 14.5 11.2 12.5 10 10Z',
};

export function buildNavItems(t: (key: string) => string, labels: Record<string, string>): NavItem[] {
  return [
    { key: 'newChat', href: '/platform/creatChat', label: labels.newChat, icon: <Icon path={ICONS.chat} />, match: (p: string) => p === '/platform/creatChat', guide: 'nav-creatChat' },
    { key: 'knowledgeBases', href: '/platform/knowledge-bases', label: t('common.knowledgeBases'), icon: <Icon path={ICONS.book} />, match: KB_ACTIVE, guide: 'nav-knowledge-bases' },
    { key: 'agents', href: '/platform/agents', label: labels.agents, icon: <Icon path={ICONS.bot} />, match: (p: string) => p === '/platform/agents' || p.startsWith('/platform/agents/') || p === '/platform/configuration', guide: 'nav-agents' },
    { key: 'organizations', href: '/platform/organizations', label: labels.organizations, icon: <Icon path={ICONS.users} />, match: (p: string) => p.startsWith('/platform/organizations') },
  ];
}

/** Vue platform/index.vue mounts TenantSelector only for all-tenant access. */
export function shouldShowTenantSwitcher(options: {
  canAccessAllTenants: boolean;
  collapsed: boolean;
  hasSwitchHandler: boolean;
}): boolean {
  return options.hasSwitchHandler && options.canAccessAllTenants && !options.collapsed;
}

// Vue stores/ui.ts:23,123-126 persists the collapsed rail under the Vue-era
// key `sidebar_collapsed` and re-reads it on boot; keep the same key so the
// preference survives reloads and stays interchangeable with the Vue artifact
// (the key is also listed in legacy-session.ts LEGACY_PREFERENCE_KEYS).
const COLLAPSE_STORAGE_KEY = 'sidebar_collapsed';

const SHELL_SESSION_PAGE_SIZE = 30;

type TenantMembership = { tenantId: string; tenantName: string; role: string };

export function PlatformShell({ client, onLogout, onTenantSwitch, children }: PlatformShellProps): ReactNode {
  const locale = usePreferredLocale();
  const t = useCallback((key: string) => formatMessage(locale, key), [locale]);
  const labels = {
    newChat: formatMessage(locale, 'menu.newChat'),
    agents: formatMessage(locale, 'menu.agents'),
    organizations: formatMessage(locale, 'menu.organizations'),
    personalSettings: formatMessage(locale, 'general.personalSettings'),
    // R449-A2 — Vue UserMenu.vue:83 labels the tenant quick link with
    // $t('settings.workspaceSettings') (「空间设置」), not settings.tenantInfo
    // (「空间信息」, which names the settings section header). Both target
    // ?section=tenant; the menu label follows the Vue key.
    workspaceSettings: formatMessage(locale, 'settings.workspaceSettings'),
    membersSettings: formatMessage(locale, 'tenantMember.title'),
    modelsSettings: formatMessage(locale, 'settings.modelManagement'),
    skillsSettings: formatMessage(locale, 'settings.skills.title'),
    // Vue UserMenu.vue:96-100 renders the catch-all settings entry with
    // $t('general.allSettings') below a divider that closes the section
    // quick-link group.
    allSettings: formatMessage(locale, 'general.allSettings'),
    // R449-A2 — Vue UserMenu.vue:112 labels the system-admin entry with
    // $t('settings.navGroups.systemAdministration') (「系统管理」).
    systemAdministration: formatMessage(locale, 'settings.navGroups.systemAdministration'),
    // Vue UserMenu.vue:45 uses $t('newUserGuide.reopen') for the reopen entry.
    reopenGuide: formatMessage(locale, 'newUserGuide.reopen'),
    // Vue UserMenu.vue:115-134 keeps these external help/community entries
    // in the account menu, independent of the current tenant capabilities.
    helpAndDocs: formatMessage(locale, 'general.helpAndDocs'),
    github: formatMessage(locale, 'common.github'),
    githubStarTip: formatMessage(locale, 'common.githubStarTip'),
    // Session-list copy (Vue menu.vue uses the same menu.* keys).
    myChats: formatMessage(locale, 'menu.myChats'),
    noSessions: formatMessage(locale, 'menu.noSessions'),
    sessionLoadError: formatMessage(locale, 'common.error'),
    renameSession: formatMessage(locale, 'menu.renameSession'),
    newSession: formatMessage(locale, 'menu.newSession'),
  };
  // Vue menu.vue renders session-list copy in the app locale (stored
  // preference, zh-CN default). SessionSidebarList without a copy prop falls
  // back to resolveChatLocale(), which also consults navigator.language and
  // rendered English chat copy ("Loading...") in en-US browsers; pass the
  // shell's resolved copy so the sidebar follows the app locale like the
  // chat page does (ChatRoutePage: resolveChatCopy(readStoredLocale())).
  const shellSidebarCopy = useMemo(() => resolveChatCopy(locale), [locale]);

  const [pathname, setPathname] = useState(() => window.location.pathname);
  const [collapsed, setCollapsed] = useState(() => window.localStorage.getItem(COLLAPSE_STORAGE_KEY) === 'true');
  const [menuOpen, setMenuOpen] = useState(false);
  const [tenantMenuOpen, setTenantMenuOpen] = useState(false);
  const [tenantSwitchPending, setTenantSwitchPending] = useState<string | null>(null);
  const userMenuRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!menuOpen) return;
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target;
      if (target && userMenuRef.current?.contains(target as Node)) return;
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
  const [user, setUser] = useState<{ id: string; name: string; email: string; avatar: string; tenantId: string; tenantName: string; role: string; memberships: TenantMembership[]; membershipsCount: number; canAccessAllTenants: boolean; isSystemAdmin: boolean }>({ id: '', name: '', email: '', avatar: '', tenantId: '', tenantName: '', role: '', memberships: [], membershipsCount: 0, canAccessAllTenants: false, isSystemAdmin: false });
  // Vue menu.ts:72-81 — the organizations nav entry is gated on
  // hasRole('admin') (owner/admin pass; viewer/contributor manage nothing in
  // the shared space). Initial true = fail-open while identity resolves:
  // the shell mounts before auth/me lands, and the server route guard
  // remains the real boundary (same posture as OrganizationsPage's
  // canManageOrg).
  const [canSeeOrganizations, setCanSeeOrganizations] = useState(true);
  const [canSeeAdminSessionSources, setCanSeeAdminSessionSources] = useState(false);
  const authResolvedClientRef = useRef<Client | null>(null);

  // Shared /auth/me reconciliation. The mount bootstrap and the tenant
  // submenu's throttled refresh both apply the same payload so memberships
  // and role gating stay consistent (Vue stores/auth.ts refreshFromAuthMe
  // reconciles user / home tenant / memberships wholesale).
  const applyAuthMe = useCallback((me: Awaited<ReturnType<Client['auth']['me']>>) => {
    const record = me.user as Record<string, unknown>;
    setUser({
      id: typeof record.id === 'string' ? record.id : typeof record.id === 'number' ? String(record.id) : '',
      name: typeof record.username === 'string' && record.username ? record.username : '—',
      email: typeof record.email === 'string' ? record.email : '',
      avatar: typeof record.avatar === 'string' ? record.avatar : '',
      tenantId: me.tenant && me.tenant.id !== null && me.tenant.id !== undefined ? String(me.tenant.id) : '',
      tenantName: typeof (me.tenant as unknown as { name?: unknown } | null | undefined)?.name === 'string' ? String((me.tenant as unknown as { name: string }).name) : '',
      role: '',
      memberships: (me.memberships ?? []).flatMap((item) => {
        if (!item || typeof item !== 'object') return [];
        const row = item as Record<string, unknown>;
        const rawId = row.tenant_id ?? row.tenantId;
        if (rawId === undefined || rawId === null || String(rawId).trim() === '') return [];
        return [{ tenantId: String(rawId), tenantName: typeof row.tenant_name === 'string' && row.tenant_name.trim() ? row.tenant_name : `#${String(rawId)}`, role: typeof row.role === 'string' ? row.role : '' }];
      }),
      membershipsCount: Array.isArray(me.memberships) ? me.memberships.length : 0,
      canAccessAllTenants: record.can_access_all_tenants === true,
      // R449-A2 — Vue stores/auth.ts:121 isSystemAdmin (User.IsSystemAdmin):
      // platform-wide flag, independent of per-tenant roles. Resolution
      // mirrors scope-runtime.ts:75 (snake_case primary, camelCase tolerated).
      // UI gating only; the server-side RequireSystemAdmin middleware is the
      // real boundary.
      isSystemAdmin: record.is_system_admin === true || record.isSystemAdmin === true,
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
    setUser((current) => ({ ...current, role: currentRole }));
  }, []);

  // Global command palette (⌘K / Ctrl+K) — R011/N003. See GlobalCommandPalette.tsx.
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [paletteQuery, setPaletteQuery] = useState('');
  const [recentQueries, setRecentQueries] = useState<string[]>([]);
  const recentQueriesKey = recentQueriesStorageKey(user.id || null, readReactPlatformState(window.localStorage)?.tenantId ?? null);

  useEffect(() => {
    // Keep the active menu in sync with browser history and app navigation.
    const update = () => setPathname(window.location.pathname);
    const unsubscribe = subscribeNavigation(update);
    return () => {
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    let active = true;
    authResolvedClientRef.current = null;
    void client.auth.me().then((me) => {
      if (!active) return;
      authResolvedClientRef.current = client;
      applyAuthMe(me);
    }).catch(() => { /* menu falls back to placeholders; the page still works */ });
    return () => { active = false; };
  }, [client, applyAuthMe]);

  // Vue menu.vue:1004-1006 fetches the organizations list once on mount
  // (when not already loaded) purely to power the sidebar pending-join
  // badge; stores/organization.ts:100-102 totals each org's
  // pending_join_request_count. Failures degrade to a hidden badge (the
  // Vue store keeps last-known data and the total starts at 0) — no toast.
  const [orgPendingJoinRequestCount, setOrgPendingJoinRequestCount] = useState(0);
  useEffect(() => {
    let active = true;
    // Defensive lookup: bare test fakes and embed mounts may not provide
    // the organizations namespace at all.
    const organizationsApi = (client as unknown as {
      organizations?: { list?: (signal?: AbortSignal) => Promise<{ items?: ReadonlyArray<Record<string, unknown>> }> };
    }).organizations;
    const listOrganizations = organizationsApi?.list?.bind(organizationsApi);
    if (!listOrganizations) return;
    void listOrganizations().then((page) => {
      if (!active) return;
      const total = (page.items ?? []).reduce((sum, org) => {
        const pending = org.pending_join_request_count;
        return sum + (typeof pending === 'number' && Number.isFinite(pending) && pending > 0 ? Math.floor(pending) : 0);
      }, 0);
      setOrgPendingJoinRequestCount(total);
    }).catch(() => { /* keep 0 → badge stays hidden */ });
    return () => { active = false; };
  }, [client]);

  const activeTenantId = readReactPlatformState(window.localStorage)?.tenantId ?? user.tenantId;
  // Vue menu.vue:7 renders a literal "Lite" edition mark next to the logo
  // when the edition flag is set; stores/auth.ts:538 sources it from the
  // durable localStorage key (same one main.tsx seeds the shell with).
  // R450-A2 — the same flag also gates the user menu / rail entries below.
  const isLiteEdition = window.localStorage.getItem('weknora_lite_mode') === 'true';
  const tenantSwitcherVisible = !isLiteEdition && shouldShowTenantSwitcher({
    canAccessAllTenants: user.canAccessAllTenants,
    collapsed,
    hasSwitchHandler: Boolean(onTenantSwitch),
  });
  const switchTenant = useCallback(async (tenantId: string) => {
    if (!onTenantSwitch || tenantSwitchPending) return;
    // Vue UserMenu.switchToTenant closes both the account menu and its
    // tenant submenu before handling either a no-op current-tenant click or
    // an async tenant navigation.
    setMenuOpen(false);
    setTenantMenuOpen(false);
    if (tenantId === activeTenantId) return;
    setTenantSwitchPending(tenantId);
    try {
      await onTenantSwitch(tenantId);
    } catch {
      setTenantSwitchPending(null);
    }
  }, [activeTenantId, onTenantSwitch, tenantSwitchPending]);

  // Vue UserMenu.vue:430-443 — opening the tenant submenu re-fetches
  // /auth/me at most once per 2s (timestamp throttle) so membership
  // invites/revokes surface without a reload. Failures are swallowed and
  // the last-known membership list keeps rendering (refreshFromAuthMe
  // returns false without a toast on the Vue side).
  const lastTenantSubmenuMembershipRefreshRef = useRef(0);
  const TENANT_SUBMENU_MEMBERSHIP_REFRESH_MS = 2000;
  const toggleTenantSubmenu = useCallback(() => {
    const next = !tenantMenuOpen;
    setTenantMenuOpen(next);
    if (!next) return;
    const now = Date.now();
    if (now - lastTenantSubmenuMembershipRefreshRef.current < TENANT_SUBMENU_MEMBERSHIP_REFRESH_MS) return;
    lastTenantSubmenuMembershipRefreshRef.current = now;
    void client.auth.me().then((me) => {
      // A stale client's late response must not clobber the fresh one.
      if (authResolvedClientRef.current !== client) return;
      applyAuthMe(me);
    }).catch(() => { /* keep last-known memberships; degrade silently */ });
  }, [applyAuthMe, client, tenantMenuOpen]);

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
    sessionsRequestRef.current = true;
    setSessionsLoading(true);
    // A bucket can disappear after an auth/client scope refresh. Let the
    // source effect restart from web rather than issuing a stale privileged
    // request during that render transition. The loading flag is raised
    // before this early return so the reset frame keeps the skeleton (not
    // the empty state); the effect run triggered by the source change resets
    // sessionsRequestRef before reloading.
    if (!sessionSourceOptionsRef.current.some((option) => option.value === sessionSource)) {
      setSessionSource('web');
      return;
    }
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
    // A bucket can disappear after an auth/client scope refresh. Restart from
    // web rather than issuing a stale privileged request during that render
    // transition. When the source is already web there is nothing to reset —
    // falling through to the reload below is required: an early return here
    // leaves sessionsMountedRef false so the in-flight request's finally
    // never resets sessionsLoading and the sidebar strands on "Loading..."
    // (R428 parity bug: the first admin auth/me flips the scope after mount).
    if ((clientChanged || scopeChanged) && sessionSource !== 'web') {
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
      navigate(`/platform/chat/${encodeURIComponent(sessionId)}`);
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
    try { await client.sessions.clear(sessionId); } catch { /* messages reload on the next visit */ }
  }

  async function deleteShellSession(sessionId: string): Promise<void> {
    try { await client.sessions.remove(sessionId); } catch { return; }
    sessionsRef.current = sessionsRef.current.filter((session) => session.id !== sessionId);
    sessionsTotalRef.current = Math.max(0, sessionsTotalRef.current - 1);
    setSessions(sessionsRef.current);
    // Vue menu.vue: deleting the open session routes back to creatChat.
    if (chatSessionIdFromPath(window.location.pathname) === sessionId) {
      navigate('/platform/creatChat');
    }
  }

  async function deleteShellSessions(sessionIds: readonly string[]): Promise<boolean> {
    await client.sessions.batchRemove(sessionIds);
    const selected = new Set(sessionIds);
    sessionsRef.current = sessionsRef.current.filter((session) => !selected.has(session.id));
    sessionsTotalRef.current = Math.max(0, sessionsTotalRef.current - sessionIds.length);
    setSessions(sessionsRef.current);
    if (chatSessionIdFromPath(window.location.pathname) && selected.has(chatSessionIdFromPath(window.location.pathname)!)) {
      navigate('/platform/creatChat');
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
  const navigateFromPalette = useCallback((path: string) => { navigate(path); }, []);
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
    // R450-A2 — Vue stores/menu.ts:64,73 adds 'organizations' (and the
    // sidebar logout) to liteHiddenPaths; the React rail only owns the
    // organizations entry, so it drops out under lite mode too.
    () => navItems.filter((item) => item.key !== 'organizations' || (canSeeOrganizations && !isLiteEdition)),
    [navItems, canSeeOrganizations, isLiteEdition],
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

  const toggleCollapsed = () => {
    setCollapsed((current) => {
      window.localStorage.setItem(COLLAPSE_STORAGE_KEY, String(!current));
      return !current;
    });
  };

  const initial = (user.name || '?').charAt(0).toUpperCase();
  // R450-A2 — Vue UserMenu.vue:247-253 gates the tenant identity line with
  // !isLiteMode ("Lite 模式下没有 RBAC 概念，统一隐藏"); the same panel owns
  // the tenant switcher, so the switcher collapses with it (UserMenu.vue:56).
  const showTenantIdentityLine = !isLiteEdition && (!collapsed && !user.canAccessAllTenants && user.membershipsCount > 1 || (!collapsed && user.canAccessAllTenants));
  const roleLabel = user.role ? formatMessage(locale, `tenantMember.role.${user.role}`) : '';

  return (
    // shell.css → utilities: .plat-shell (flex row, full viewport), .plat-shell__aside
    // (+ collapsed state swaps width/padding values rather than layering overrides).
    <div className="flex items-stretch w-full h-screen min-w-[600px] bg-white">
      <aside className={collapsed
        ? 'box-border flex flex-col min-w-[60px] w-[60px] pt-[8px] px-[3px] pb-[6px] bg-[#f6f8fa] border-r border-[#e7ebf0] shadow-[1px_0_0_rgba(0,0,0,0.02)] overflow-hidden transition-[width,min-width] duration-[250ms] ease-[ease]'
        : 'box-border flex flex-col min-w-[260px] w-[260px] pt-[8px] px-[6px] pb-[6px] bg-[#f6f8fa] border-r border-[#e7ebf0] shadow-[1px_0_0_rgba(0,0,0,0.02)] overflow-hidden transition-[width,min-width] duration-[250ms] ease-[ease]'}>
        <div className="flex items-center justify-between h-[42px] shrink-0 pr-[10px] pl-[14px]">
                <a className="flex min-w-0 flex-1 items-center gap-[8px] overflow-hidden no-underline text-inherit" href="/platform/knowledge-bases" aria-label="WeKnora" onClick={(event) => {
                  if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
                  event.preventDefault();
                  navigate('/platform/knowledge-bases');
                }}>
            {!collapsed && <img className="block h-auto w-[128px]" src={weknoraLogo} alt="" />}
            {/* Vue menu.vue:7 `<sup class="lite-badge">Lite</sup>` — edition
                mark, untranslated; styles port menu.vue:1289-1297. */}
            {!collapsed && isLiteEdition && (
              <sup className="ml-[2px] mt-[2px] shrink-0 self-start select-none whitespace-nowrap text-[9px] font-semibold leading-none text-[var(--wk-color-text-placeholder,rgba(0,0,0,0.4))]">Lite</sup>
            )}
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

        {/* Vue menu.vue keeps the global search entry available in the
            collapsed rail (the expanded logo row is not mounted there). */}
        {collapsed && (
          <button type="button" className="mx-[4px] flex h-[38px] items-center justify-center rounded-[4px] border-0 bg-transparent p-0 text-[#3d4a5c] hover:bg-[#eceff4]" onClick={() => { setPaletteQuery(''); setPaletteOpen(true); }} aria-label={t('menu.search')} title={`${t('menu.search')} ${platformModKeyLabel(navigator.platform)}K`}>
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true">
              <circle cx="11" cy="11" r="6.5" />
              <path d="m16 16 4.5 4.5" />
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
                <a key={item.key} href={item.href} onClick={(event) => {
                  if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
                  event.preventDefault();
                  navigate(item.href);
                }} className={(collapsed
                  ? 'justify-center mx-[4px] px-0 '
                  : 'mx-0 px-[14px] ')
                  + 'box-border flex h-[38px] items-center gap-[8px] rounded-[4px] py-[8px] no-underline text-[14px] font-semibold whitespace-nowrap '
                  + (active
                    ? 'bg-[#f3f3f3] hover:bg-[#f3f3f3] text-[#07c05f]'
                    : 'text-[#3d4a5c] hover:bg-[#eceff4]')}
                  aria-current={active ? 'page' : undefined} title={collapsed ? item.label : undefined} data-guide={item.guide}>
                  <span className="inline-flex shrink-0">{item.icon}</span>
                  {!collapsed && <span className="overflow-hidden text-ellipsis">{item.label}</span>}
                  {/* Vue menu.vue:93-98 — amber pending-join pill on the
                      organizations entry, expanded rail only, raw count. */}
                  {!collapsed && item.key === 'organizations' && orgPendingJoinRequestCount > 0 && (
                    <span data-testid="org-pending-badge" title={t('organization.settings.pendingJoinRequestsBadge')}
                      className="inline-flex h-[18px] min-w-[18px] shrink-0 items-center justify-center rounded-[9px] bg-[rgba(250,173,20,0.2)] px-[5px] text-[12px] font-semibold leading-[18px] text-[#e37318]">
                      {orgPendingJoinRequestCount}
                    </span>
                  )}
                </a>
              );
            })}
          </nav>

          {/* Vue menu.vue .submenu: the grouped session list lives in the
              sidebar on every protected page; collapsed sidebars hide it.
              Keep the region label semantic-only; Vue renders dates and rows
              here without a visible 我的对话 heading. */}
          {/* .plat-shell__sessions / __sessions-title → utilities. The
              shell-context group-list indent (padding 0 6px on the session
              <ul>) rides along as an arbitrary-variant utility so the
              fallback chat sidebar (outside the shell) keeps its flush list. */}
          {!collapsed && (
            <nav className="mb-[4px] pt-[8px] border-t border-[#e7ebf0] [&_h3]:mt-0 [&_ul]:px-[6px] [&_li>button]:py-[5px]" aria-label={labels.myChats}>
              {sessionsLoadError && !sessionsLoading ? <p className="mx-[14px] my-2 text-xs text-[#b42318]" role="status">
                {labels.sessionLoadError}{' '}<button type="button" className="cursor-pointer border-0 bg-transparent p-0 text-xs text-[#07c05f] underline" onClick={retryShellSessions}>{t('common.retry')}</button>
              </p> : null}
              <SessionSidebarList
                copy={shellSidebarCopy}
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
                <span className="flex min-w-0 flex-1 flex-col gap-[2px]">
                  {showTenantIdentityLine ? <>
                    <span className="overflow-hidden text-ellipsis whitespace-nowrap text-[14px] font-semibold tracking-[-0.01em] text-[#1f2733]">{user.tenantName || user.name || '—'}</span>
                    <span className="flex min-w-0 items-center gap-1 overflow-hidden text-ellipsis whitespace-nowrap text-[12px] leading-[1.35] text-[#66758b]">
                      {user.name && user.name !== user.tenantName ? <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{user.name}</span> : null}
                      {user.name && user.name !== user.tenantName && roleLabel ? <span aria-hidden="true" className="text-[#8b97a8]">·</span> : null}
                      {roleLabel ? <span className="shrink-0">{roleLabel}</span> : null}
                    </span>
                  </> : <>
                    <span className="overflow-hidden text-ellipsis whitespace-nowrap text-[14px] font-medium text-[#1f2733]">{user.name || '—'}</span>
                    <span className="overflow-hidden text-ellipsis whitespace-nowrap text-[12px] text-[#66758b]">{user.email}</span>
                  </>}
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
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=userprofile', () => setMenuOpen(false))}>
                  {labels.personalSettings}
                </a>
                {/* R450-A2 — Vue UserMenu.vue:81 gates the 「空间设置」
                    quick link with !isLiteMode; lite deployments have no
                    tenant surface to manage. */}
                {!isLiteEdition && <a role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
                  href="/platform/settings?section=tenant"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=tenant', () => setMenuOpen(false))}>
                  {labels.workspaceSettings}
                </a>}
                {tenantSwitcherVisible ? <div className="border-t border-[#eef1f5] px-[8px] py-[6px]" role="group" aria-label={t('tenant.switcher.menuLabel')}>
                  <button type="button" className="flex items-center justify-between gap-2 w-full border-0 bg-transparent px-[4px] py-[5px] text-left text-[12px] font-semibold text-[#66758b] cursor-pointer" aria-expanded={tenantMenuOpen} onClick={toggleTenantSubmenu}>
                    <span>{t('tenant.switcher.menuLabel')}</span><span aria-hidden="true">{tenantMenuOpen ? '⌃' : '⌄'}</span>
                  </button>
                  {tenantMenuOpen ? <div role="listbox" aria-label={t('tenant.switcher.menuLabel')} className="mt-[2px] max-h-[180px] overflow-y-auto">
                    {user.memberships.map((membership) => <button key={membership.tenantId} type="button" role="option" aria-selected={membership.tenantId === activeTenantId} disabled={tenantSwitchPending !== null} className="flex items-center justify-between gap-2 w-full border-0 bg-transparent px-[4px] py-[7px] text-left text-[13px] text-[#1f2733] cursor-pointer hover:bg-[#f2f5f9] disabled:cursor-wait disabled:opacity-60" onClick={() => void switchTenant(membership.tenantId)}>
                      <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{membership.tenantName}</span><span className="shrink-0 text-[11px] text-[#8b97a8]">{membership.tenantId === activeTenantId ? '当前' : membership.role}</span>
                    </button>)}
                  </div> : null}
                </div> : null}
                {canSeeAdminSessionSources && !isLiteEdition ? <a role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
                  href="/platform/settings?section=members"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=members', () => setMenuOpen(false))}>
                  {labels.membersSettings}
                </a> : null}
                {canSeeAdminSessionSources && !isLiteEdition ? <a role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
                  href="/platform/settings?section=models"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=models', () => setMenuOpen(false))}>
                  {labels.modelsSettings}
                </a> : null}
                {canSeeAdminSessionSources && !isLiteEdition ? <a role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
                  href="/platform/settings?section=skills"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=skills', () => setMenuOpen(false))}>
                  {labels.skillsSettings}
                </a> : null}
                {/* Vue UserMenu.vue:96-100 — a divider closes the section
                    quick-link group, then the unconditional 「全部设置」 entry
                    opens the settings surface WITHOUT a section query
                    (handleSettings → router.push('/platform/settings')). It
                    renders for every role so viewer-only users keep a path to
                    the read-only rosters and model lists. */}
                <div className="h-[1px] bg-[#e7ebf0] my-[3px]" aria-hidden="true" />
                <a role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
                  href="/platform/settings"
                  onClick={(event) => handleInternalLink(event, '/platform/settings', () => setMenuOpen(false))}>
                  {labels.allSettings}
                </a>
                {/* R449-A2 — Vue UserMenu.vue:104-113 renders 「系统管理」 only
                    for is_system_admin users, between 全部设置 and a divider
                    that precedes the docs entry. handleSystemAdmin lands on
                    the settings modal opened at the system-global group
                    (?section=system-global). UI gating only; the server-side
                    RequireSystemAdmin middleware is the real boundary. */}
                {user.isSystemAdmin ? <a role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
                  href="/platform/settings?section=system-global"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=system-global', () => setMenuOpen(false))}>
                  {labels.systemAdministration}
                </a> : null}
                <div className="h-[1px] bg-[#e7ebf0] my-[3px]" aria-hidden="true" />
                <a role="menuitem" className="flex items-center gap-[10px] w-full border-none bg-transparent px-[12px] py-[9px] text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
                  href="https://github.com/Tencent/WeKnora/tree/main/docs" target="_blank" rel="noreferrer"
                  onClick={() => setMenuOpen(false)}>
                  <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor"
                    strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <circle cx="12" cy="12" r="9" />
                    <path d="M9.4 9.4a2.6 2.6 0 1 1 3.7 2.4c-.8.4-1.1.9-1.1 1.7" />
                    <line x1="12" y1="16.6" x2="12" y2="16.7" />
                  </svg>
                  <span className="flex min-w-0 flex-1 items-center justify-between gap-[10px]">
                    <span>{labels.helpAndDocs}</span>
                    <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor" aria-hidden="true">
                      <path d="M12.667 8a.667.667 0 0 1 .666.667v4a2.667 2.667 0 0 1-2.666 2.666H4.667A2.667 2.667 0 0 1 2 12.667V5.333a2.667 2.667 0 0 1 2.667-2.666h4a.667.667 0 1 1 0 1.333h-4a1.333 1.333 0 0 0-1.333 1.333v7.334a1.333 1.333 0 0 0 1.333 1.333h6a1.333 1.333 0 0 0 1.333-1.333v-4a.667.667 0 0 1 .667-.667Z" />
                      <path d="M10 1.333h4a.667.667 0 0 1 .667.667v4a.667.667 0 0 1-1.334 0V3.609L8.138 8.805a.667.667 0 1 1-.943-.943l5.195-5.195H10a.667.667 0 1 1 0-1.334Z" />
                    </svg>
                  </span>
                </a>
                <a role="menuitem" className="flex items-center gap-[10px] w-full border-none bg-transparent px-[12px] py-[9px] text-[14px] text-[#1f2733] no-underline hover:bg-[#f2f5f9]"
                  href="https://github.com/Tencent/WeKnora" target="_blank" rel="noreferrer" title={labels.githubStarTip}
                  onClick={() => setMenuOpen(false)}>
                  <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" aria-hidden="true">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
                  </svg>
                  <span className="flex min-w-0 flex-1 items-center justify-between gap-[10px]">
                    <span className="flex min-w-0 items-center gap-[6px]">
                      <span>{labels.github}</span>
                      <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor" aria-hidden="true">
                        <path d="m12 2.4 2.98 6.04 6.66.97-4.82 4.7 1.14 6.63L12 17.63l-5.96 3.14 1.14-6.63-4.82-4.7 6.66-.97L12 2.4Z" />
                      </svg>
                    </span>
                    <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor" aria-hidden="true">
                      <path d="M12.667 8a.667.667 0 0 1 .666.667v4a2.667 2.667 0 0 1-2.666 2.666H4.667A2.667 2.667 0 0 1 2 12.667V5.333a2.667 2.667 0 0 1 2.667-2.666h4a.667.667 0 1 1 0 1.333h-4a1.333 1.333 0 0 0-1.333 1.333v7.334a1.333 1.333 0 0 0 1.333 1.333h6a1.333 1.333 0 0 0 1.333-1.333v-4a.667.667 0 0 1 .667-.667Z" />
                      <path d="M10 1.333h4a.667.667 0 0 1 .667.667v4a.667.667 0 0 1-1.334 0V3.609L8.138 8.805a.667.667 0 1 1-.943-.943l5.195-5.195H10a.667.667 0 1 1 0-1.334Z" />
                    </svg>
                  </span>
                </a>
                {/* R450-A2 — Vue UserMenu.vue:136-144 keeps the divider and
                    the logout item behind !isLiteMode: lite editions have no
                    account session to end (Vue stores/auth.ts logout also
                    clears the weknora_lite_mode key). */}
                {!isLiteEdition && <>
                  <div className="h-[1px] bg-[#e7ebf0] my-[3px]" aria-hidden="true" />
                  <button type="button" role="menuitem" className="flex items-center gap-[10px] w-full px-[12px] py-[9px] border-none bg-transparent cursor-pointer text-[14px] text-[#d54941] no-underline hover:bg-[#fbe9e8]"
                    onClick={() => { setMenuOpen(false); void runShellLogout(onLogout); }}>
                    {t('auth.logout')}
                  </button>
                </>}
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
      <InvitationInbox client={client} />
    </div>
  );
}
