import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode, type UIEvent } from 'react';
import type { KeyboardEvent as ReactKeyboardEvent, MouseEvent as ReactMouseEvent } from 'react';
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
// SP13 Task 8 — 侧栏「分享」弹窗（ChatRoutePage 同组件；不经过 @weknora/ui
// 以免 theme.css 拖进 shell 的 node 测试模块图）。
import { SessionShareDialog } from '../chat/SessionShareDialog.tsx';
import { InvitationInbox } from './InvitationInbox.tsx';
import { navigate, subscribeNavigation } from './navigation.ts';
import {
  loadPaletteDeploymentCapabilities,
  paletteAccessFromCapabilities,
  type PaletteDeploymentCapabilities,
} from './deployment-capabilities.ts';
import { PaletteRetrievalSettings } from './retrieval-settings-panel.tsx';
// Vue sessionActivity 同构纯核心（侧栏会话行 running spinner 标记的转移规则）。
import {
  detectRunningMessageId,
  refreshSessionActivityEntry,
  refreshSessionActivityError,
  type SessionActivityEntries,
} from './session-activity.ts';
// Welcome-tour styles live with the component in @weknora/views; the package
// itself must stay css-import-free for the shared typecheck, so the shell
// pulls it in by relative path. (shell.css is gone — all rules became
// utilities in this file / session-sidebar.tsx.)
import '../../../../packages/views/src/guides/guides.css';
// Task 9.5 — shell 层同构平移样式（menu.vue / UserMenu.vue / SessionSidebarRow.vue
// 平移，见 platform-shell.td.css 头注）。Vue 端图标走 <img src> 资产（渲染为
// 黑色 filled glyph、激活态换 -green.svg 变体），资产从 frontend/src/assets/img
// 复制到 ./assets/img 保持逐字节一致。
import './platform-shell.td.css';
import { Icon as TIcon } from 'tdesign-icons-react';

// Vue menu.vue getImgSrc 同款解析（new URL(..., import.meta.url)）：vite 资产
// 管线在 dev/build 均支持；node 直算 href 不加载文件，测试无需 svg 拦截。
const getImgSrc = (url: string): string => new URL(`./assets/img/${url}`, import.meta.url).href;
const searchIconUrl = getImgSrc('search.svg');
const weknoraLogo = getImgSrc('weknora.png');

type Client = ReturnType<typeof createWeKnoraClient>;

// Union param so the role="button" account card can reuse the anchor guard for
// its Enter keydown; `button` is mouse-only, so read it through a narrowing cast
// (identical expression after erasure — keyboard events keep the early return).
function handleInternalLink(event: ReactMouseEvent<Element> | ReactKeyboardEvent<Element>, path: string, afterNavigate?: () => void): void {
  if (event.defaultPrevented || (event as ReactMouseEvent<Element>).button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
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
  /** NAV_ICON_URLS key — the <img> asset pair (default/-green) per Vue menu.vue getIcon. */
  icon: string;
  /**
   * React-only rail entries (experts / market / analytics) have no Vue menu
   * asset to mirror — they keep their own inline svg glyphs here instead of
   * borrowing another entry's <img> (which would erase their visual
   * identity). Rendered when set; otherwise the Vue <img> pair is used.
   */
  iconNode?: ReactNode;
  match: (pathname: string) => boolean;
  /** Anchor for the welcome-tour spotlight (Vue menu.vue data-guide attrs). */
  guide?: string;
}

// React-only rail glyphs (pre-Task-9.5 geometry, restored): 24×24 stroke
// drawings in the same visual family as the platform rail — these have no
// Vue asset counterpart and must not fall back to a Vue entry's icon.
function ReactOnlyNavIcon({ paths }: { paths: string[] }): ReactNode {
  return (
    <svg className="plat-shell__icon" viewBox="0 0 24 24" width="18" height="18" fill="none"
      stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {paths.map((d, index) => <path key={`${d}-${index}`} d={d} />)}
    </svg>
  );
}

const REACT_ONLY_NAV_ICONS = {
  // SP11 analytics entry — bar-chart glyph (stroke = currentColor).
  chart: [
    'M5 20V11',
    'M12 20V5',
    'M19 20V14',
    'M3.5 20H20.5',
  ],
  // M2 experts entry — sparkles glyph (experts = preset templates that
  // "spark" a new agent).
  sparkles: [
    'M12 3L13.7 7.8L18.5 9.5L13.7 11.2L12 16L10.3 11.2L5.5 9.5L10.3 7.8L12 3Z',
    'M18.5 14.5L19.4 16.6L21.5 17.5L19.4 18.4L18.5 20.5L17.6 18.4L15.5 17.5L17.6 16.6L18.5 14.5Z',
  ],
  // M4 skills-market entry — shopping-bag glyph (the market = a bag of
  // installable skills).
  bag: [
    'M6.3 8.2H17.7L18.9 19.1C19 20 18.3 20.8 17.4 20.8H6.6C5.7 20.8 5 20 5.1 19.1L6.3 8.2Z',
    'M9 10.2V6.6C9 4.7 10.3 3.2 12 3.2C13.7 3.2 15 4.7 15 6.6V10.2',
  ],
};

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

/**
 * R464-A1 — seed the command palette's KB scope chip from the KB detail
 * route, mirroring Vue GlobalCommandPalette.vue's `route.params.kbId`
 * inference on open. The palette resolves the display name from the KB list
 * once it loads; the raw id is the fallback (Vue parity).
 */
// Both path forms reach the KB detail surface: the React-native
// /knowledgeBase/:id(…) routes and the Vue-form alias
// /platform/knowledge-bases/:id — the scope chip must seed on the user's
// primary path too, not only the alias (R464 A4 live finding).
export function kbScopeFromLocation(): { id: string; name: string } | null {
  const match = window.location.pathname.match(/^\/(?:knowledgeBase|platform\/knowledge-bases)\/([^/]+)/);
  if (!match) return null;
  try {
    const id = decodeURIComponent(match[1]!).trim();
    return id ? { id, name: id } : null;
  } catch {
    return null;
  }
}

// Task 9.5 — Vue menu.vue renders nav icons as <img> assets from
// frontend/src/assets/img (menu.vue:87-89): the svg files carry their own
// fill/stroke (currentColor → black in the <img> image context), and the
// active section swaps to the -green.svg variant (menu.vue getIcon). The
// assets are byte-identical copies; the rendered glyph is therefore the
// same rasterization path as Vue (<img> of the same svg bytes).
const NAV_ICON_URLS: Record<string, { default: string; active: string }> = {
  creatChat: { default: getImgSrc('prefixIcon.svg'), active: getImgSrc('prefixIcon-green.svg') },
  'knowledge-bases': { default: getImgSrc('zhishiku.svg'), active: getImgSrc('zhishiku-green.svg') },
  agents: { default: getImgSrc('agent.svg'), active: getImgSrc('agent-green.svg') },
  organizations: { default: getImgSrc('organization.svg'), active: getImgSrc('organization-green.svg') },
};

export function buildNavItems(t: (key: string) => string, labels: Record<string, string>): NavItem[] {
  return [
    { key: 'newChat', href: '/platform/creatChat', label: labels.newChat, icon: 'creatChat', match: (p: string) => p === '/platform/creatChat', guide: 'nav-creatChat' },
    { key: 'knowledgeBases', href: '/platform/knowledge-bases', label: t('common.knowledgeBases'), icon: 'knowledge-bases', match: KB_ACTIVE, guide: 'nav-knowledge-bases' },
    { key: 'agents', href: '/platform/agents', label: labels.agents, icon: 'agents', match: (p: string) => p === '/platform/agents' || p.startsWith('/platform/agents/') || p === '/platform/configuration', guide: 'nav-agents' },
    // M2 expert templates — a creation surface next to agents; unconditional
    // (the GET /experts list is tenant-scoped, no admin gate). React-only
    // entry: no Vue asset counterpart, keeps its own sparkles glyph.
    { key: 'experts', href: '/platform/experts', label: labels.experts, icon: 'experts', iconNode: <ReactOnlyNavIcon paths={REACT_ONLY_NAV_ICONS.sparkles} />, match: (p: string) => p.startsWith('/platform/experts') },
    // M4 skills market — remote SkillHub search/rankings plus the
    // tenant-internal published list; visible to every member (Viewer+ reads
    // — routes_skill_market.go / routes_tenant_skill_market.go), the Admin+
    // install/publish affordances gate inside the page. React-only entry:
    // no Vue asset counterpart, keeps its own bag glyph.
    { key: 'market', href: '/platform/market', label: labels.market, icon: 'market', iconNode: <ReactOnlyNavIcon paths={REACT_ONLY_NAV_ICONS.bag} />, match: (p: string) => p.startsWith('/platform/market') },
    { key: 'organizations', href: '/platform/organizations', label: labels.organizations, icon: 'organizations', match: (p: string) => p.startsWith('/platform/organizations'), guide: 'nav-organizations' },
    // SP11 analytics — React-only entry: no Vue asset counterpart, keeps its
    // own bar-chart glyph.
    { key: 'analytics', href: '/platform/analytics', label: labels.analytics, icon: 'analytics', iconNode: <ReactOnlyNavIcon paths={REACT_ONLY_NAV_ICONS.chart} />, match: (p: string) => p.startsWith('/platform/analytics') },
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

// Vue composables/useRoleLabel.ts ROLE_ICONS — 角色前缀图标映射（受限集合，
// 只收录已随包发行的图标名，避免冷门角色触发 icon-not-found）。
const ROLE_ICONS: Record<string, string> = {
  owner: 'secured',
  admin: 'user-circle',
  contributor: 'edit',
  viewer: 'browse',
};

// Vue menu.vue:977 — sessionActivity 轮询周期（setInterval(..., 5000)）。
const SESSION_ACTIVITY_REFRESH_MS = 5000;

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
    // M2 expert templates rail entry (menu.* key registered React-side in
    // scripts/parity/backfill-i18n-keys.mjs EXPERTS_VALUES — the Vue menu
    // table has no experts key).
    experts: formatMessage(locale, 'menu.experts'),
    // M4 skills-market rail entry (menu.market, same script-owned registration
    // via MARKET_VALUES — React-only surface, no Vue menu baseline).
    market: formatMessage(locale, 'menu.market'),
    organizations: formatMessage(locale, 'menu.organizations'),
    // SP11 analytics dashboard (admin-only entry, gated below).
    analytics: formatMessage(locale, 'menu.analytics'),
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

  // R452-A2 — Vue menu.vue:986-991: after mount the shell additionally
  // probes GET /api/v1/system/info and upgrades the lite gating when the
  // response reports edition 'lite'. The upgrade persists the durable key
  // exactly like Vue authStore.setLiteMode(true) (stores/auth.ts:411-418);
  // a non-lite edition never downgrades it and a failed probe is swallowed
  // (menu.vue:991 `.catch(() => { })`).
  const [liteEditionProbed, setLiteEditionProbed] = useState(false);
  useEffect(() => {
    let active = true;
    // Defensive lookup: bare test fakes and embed mounts may not provide
    // the settings namespace at all (same posture as organizations above).
    const systemApi = (client as unknown as {
      settings?: { system?: { info?: (signal?: AbortSignal) => Promise<{ edition?: string }> } };
    }).settings?.system;
    const systemInfo = systemApi?.info?.bind(systemApi);
    if (!systemInfo) return;
    void systemInfo().then((info) => {
      if (!active) return;
      if (info?.edition === 'lite') {
        window.localStorage.setItem('weknora_lite_mode', 'true');
        setLiteEditionProbed(true);
      }
    }).catch(() => { /* Vue menu.vue:991 — silent */ });
    return () => { active = false; };
  }, [client]);

  const activeTenantId = readReactPlatformState(window.localStorage)?.tenantId ?? user.tenantId;
  // Vue menu.vue:7 renders a literal "Lite" edition mark next to the logo
  // when the edition flag is set; stores/auth.ts:538 sources it from the
  // durable localStorage key (same one main.tsx seeds the shell with).
  // R450-A2 — the same flag also gates the user menu / rail entries below.
  // R452-A2 — Vue menu.vue:986-991 ORs in the system-info edition probe:
  // localStorage decides first, then a lite server edition upgrades the
  // gating (and persists the key) even on a first session where the key was
  // never written; a non-lite probe never downgrades it.
  const isLiteEdition = liteEditionProbed || window.localStorage.getItem('weknora_lite_mode') === 'true';
  // R465-A2 — deployment capabilities for the command palette (Vue
  // deploymentCapabilities store): the open-agents / open-organizations
  // quick actions and the palette's agent search group follow
  // GET /api/v1/system/capabilities. Fail-open like Vue: a missing
  // administration namespace (bare test fakes / embed mounts) or a failed
  // probe leaves everything visible; the backend still guards the routes.
  const [paletteCapabilities, setPaletteCapabilities] = useState<PaletteDeploymentCapabilities | null>(null);
  useEffect(() => {
    let active = true;
    const adminApi = (client as unknown as {
      administration?: { capabilities?: (signal?: AbortSignal) => Promise<PaletteDeploymentCapabilities> };
    }).administration;
    const fetchCapabilities = adminApi?.capabilities?.bind(adminApi);
    if (!fetchCapabilities) return;
    void loadPaletteDeploymentCapabilities(fetchCapabilities).then((probed) => {
      if (active) setPaletteCapabilities(probed);
    });
    return () => { active = false; };
  }, [client]);
  const paletteAccess = useMemo(
    () => paletteAccessFromCapabilities(paletteCapabilities, {
      liteMode: isLiteEdition,
      // Vue authStore.hasRole('admin'): owner also passes.
      isAdmin: user.role === 'admin' || user.role === 'owner',
    }),
    [paletteCapabilities, isLiteEdition, user.role],
  );
  const paletteRetrievalSettings = useMemo(
    () => <PaletteRetrievalSettings client={client} locale={locale} />,
    [client, locale],
  );
  // Vue UserMenu.vue:375-382 — showTenantSwitcher 只看 memberships 数（>=1 即
  // 显示切换 glyph：单空间用户也要能从子菜单看到当前空间/创建入口）。它不看
  // canAccessAllTenants——超管额外走侧栏 TenantSelector（Vue-only 挂载），不收紧
  // 此处门控。shouldShowTenantSwitcher 仍保留给未来的侧栏 TenantSelector 挂载。
  const tenantSwitcherVisible = !isLiteEdition && user.memberships.length > 0;
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

  const activeChatId = chatSessionIdFromPath(pathname);

  // Vue sessionActivity 通路（menu.vue + stores/sessionActivity）：会话行的
  // running spinner 标记。Vue 由 chat/index.vue 的 isReplying watcher 写入
  // store（update(id, true)），menu.vue 只负责 5s refresh 与渲染；React chat
  // 域（禁改域）既无写入方也无对外事件，故 shell 承担最小对齐：活跃会话
  // 切换时跑 chat 视图同款「末位未完成 assistant」检测（session-activity.ts
  // 纯核心，复刻 sessionActivityState 的转移规则），此后按 menu.vue:977 的
  // 5s 口径轮询条目直到消息完成/消失/连续失败清除。已知残差：同会话内发
  // 起新一轮生成（路由不变）时 React 侧标记要等下次检测，不像 Vue 的
  // watcher 即时——补齐它需要 chat 域暴露生成态事件，超出本壳层范围。
  const [sessionActivityEntries, setSessionActivityEntries] = useState<SessionActivityEntries>({});
  const sessionActivityRef = useRef<SessionActivityEntries>({});
  sessionActivityRef.current = sessionActivityEntries;
  // Client/tenant identity change → clear (Vue store watches
  // [auth.user.id, auth.effectiveTenantId] with flush:'sync').
  const sessionActivityClientRef = useRef<Client | null>(null);
  useEffect(() => {
    const clientChanged = sessionActivityClientRef.current !== null && sessionActivityClientRef.current !== client;
    sessionActivityClientRef.current = client;
    if (clientChanged) setSessionActivityEntries({});
  }, [client]);
  // Active chat switch → one-shot detection (chat/index.vue onAfterMsgList).
  // Defensive probe: host clients without the chat history API keep the shell
  // mounted without the marker (same convention as the queryHistory probing).
  // Memoized: a fresh function per render would re-arm the effects below into
  // a fetch/render loop.
  const sessionsMessagesApi = useMemo(
    () => typeof client.sessions.messages === 'function' ? client.sessions.messages.bind(client.sessions) : null,
    [client],
  );
  useEffect(() => {
    if (!activeChatId || !sessionsMessagesApi) return;
    let active = true;
    sessionsMessagesApi(activeChatId, { limit: 20 }).then((messages) => {
      if (!active) return;
      const messageId = detectRunningMessageId(messages);
      setSessionActivityEntries((prev) => {
        if (messageId) {
          const current = prev[activeChatId];
          if (current && current.messageId === messageId && current.failures === 0) return prev;
          return { ...prev, [activeChatId]: { messageId, failures: 0 } };
        }
        if (!(activeChatId in prev)) return prev;
        const next = { ...prev };
        delete next[activeChatId];
        return next;
      });
    }).catch(() => { /* transient failure: the poll below re-checks */ });
    return () => { active = false; };
  }, [activeChatId, sessionsMessagesApi]);
  // menu.vue:977 — unconditional 5s refresh while mounted; empty entries are
  // a no-op (sessionActivityState.refresh only iterates entries).
  useEffect(() => {
    if (!sessionsMessagesApi) return;
    const timer = window.setInterval(() => {
      const entries = sessionActivityRef.current;
      const ids = Object.keys(entries);
      if (ids.length === 0) return;
      void (async () => {
        const settled = await Promise.all(ids.map(async (sessionId) => {
          try {
            const messages = await sessionsMessagesApi(sessionId, { limit: 20 });
            return [sessionId, refreshSessionActivityEntry(entries[sessionId], messages)] as const;
          } catch (cause) {
            const status = (cause as { status?: number })?.status;
            return [sessionId, refreshSessionActivityError(entries[sessionId], status)] as const;
          }
        }));
        setSessionActivityEntries((prev) => {
          const next = { ...prev };
          let changed = false;
          for (const [sessionId, entry] of settled) {
            if (!entry) {
              if (sessionId in next) { delete next[sessionId]; changed = true; }
            } else {
              const current = next[sessionId];
              if (!current || current.messageId !== entry.messageId || current.failures !== entry.failures) {
                next[sessionId] = entry;
                changed = true;
              }
            }
          }
          return changed ? next : prev;
        });
      })();
    }, SESSION_ACTIVITY_REFRESH_MS);
    return () => { window.clearInterval(timer); };
  }, [sessionsMessagesApi]);

  // Vue menu.vue groups the list by date unconditionally (groupSessionsByDate
  // → 已置顶/今天/昨天/近7天/近30天/更早), with the route as the selection.
  // Session rows additionally carry the sessionActivity running marker
  // (SessionSidebarRow.vue :running — see sessionActivityEntries above).
  const sessionListGroups: readonly SessionGroupView[] = useMemo(
    () => sessionGroups(
      Object.keys(sessionActivityEntries).length === 0
        ? sessions
        : sessions.map((session) => sessionActivityEntries[session.id] ? { ...session, running: true } : session),
      new Date(),
      'date',
    ),
    [sessionActivityEntries, sessions],
  );

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
    // Vue menu.vue handleSessionMutation — messagesCleared → sessionActivity.update(id, false).
    setSessionActivityEntries((prev) => {
      if (!(sessionId in prev)) return prev;
      const next = { ...prev };
      delete next[sessionId];
      return next;
    });
  }

  async function deleteShellSession(sessionId: string): Promise<void> {
    try { await client.sessions.remove(sessionId); } catch { return; }
    // Vue menu.vue handleSessionMutation — removed → sessionActivity.update(id, false).
    setSessionActivityEntries((prev) => {
      if (!(sessionId in prev)) return prev;
      const next = { ...prev };
      delete next[sessionId];
      return next;
    });
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
    setSessionActivityEntries((prev) => {
      const next = { ...prev };
      let changed = false;
      for (const sessionId of selected) {
        if (sessionId in next) { delete next[sessionId]; changed = true; }
      }
      return changed ? next : prev;
    });
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

  // SP13 Task 8 — 会话分享：侧栏 ⋯ 菜单「分享」→ SessionShareDialog（mint
  // 只读链接 + 复制 + 撤销）。能力开关沿用 shell 对可选命名空间的防御式
  // 探测（organizations/settings 同惯例）：测试替身与 embed 挂载没有
  // queryHistory 域时不渲染入口。
  const [shareSessionId, setShareSessionId] = useState<string | null>(null);
  const [shellShareToast, setShellShareToast] = useState<string | null>(null);
  const shellShareToastTimer = useRef<number | null>(null);
  useEffect(() => () => { if (shellShareToastTimer.current !== null) window.clearTimeout(shellShareToastTimer.current); }, []);
  function showShellShareToast(message: string) {
    setShellShareToast(message);
    if (shellShareToastTimer.current !== null) window.clearTimeout(shellShareToastTimer.current);
    shellShareToastTimer.current = window.setTimeout(() => setShellShareToast(null), 2400);
  }
  const shareApi = (client as unknown as {
    queryHistory?: { share?: unknown; unshare?: unknown };
  }).queryHistory;
  const canShareSessions = Boolean(
    typeof shareApi?.share === 'function' && typeof shareApi?.unshare === 'function',
  );

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
    // SP11 — the analytics entry follows the canViewChannelSessions gate
    // (systemAdmin || owner || admin; canSeeAdminSessionSources mirrors it
    // in applyAuthMe) and stays fail-closed until auth/me resolves.
    // Round-22 parity (2026-09-19): Vue's rail (stores/menu.ts) has no
    // analytics or experts entry on any surface, so both sidebar links drop
    // to keep the two rails byte-identical; the SP11 /platform/analytics and
    // M2 /platform/experts routes stay reachable by URL — restore by removing
    // the clause below. Round-23 parity (2026-09-21, user-authorized): the
    // skills-market entry leaves the rail too — Vue's rail (stores/menu.ts)
    // has no market link, and the extra 38px row pushed the session list
    // ~40px down on every page. The /platform/market route and its page stay
    // reachable by URL; only the sidebar entry is removed.
    () => navItems.filter((item) => (item.key !== 'organizations' || (canSeeOrganizations && !isLiteEdition)) && item.key !== 'analytics' && item.key !== 'experts' && item.key !== 'market'),
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

  // Vue menu.vue:1151-1169 onDragHandleMouseDown — collapsed-rail drag handle:
  // track the drag, expand once the pointer moves >40px to the right.
  const onDragHandleMouseDown = (event: ReactMouseEvent<HTMLElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const expandThreshold = 40;
    const onMouseMove = (moveEvent: MouseEvent) => {
      if (moveEvent.clientX - startX > expandThreshold) {
        setCollapsed(false);
        window.localStorage.setItem(COLLAPSE_STORAGE_KEY, 'false');
        cleanup();
      }
    };
    const onMouseUp = () => cleanup();
    const cleanup = () => {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
    };
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  };

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
  // Vue composables/useRoleLabel.ts ROLE_ICONS — 角色标签的 12px 前缀图标
  // （owner=secured/admin=user-circle/contributor=edit/viewer=browse），颜色随
  // 所在行文本（user-tenant-meta-icon / dropdown-tenant-panel-role-icon 均
  // color:inherit）。
  const roleIcon = user.role && ROLE_ICONS[user.role] ? ROLE_ICONS[user.role] : '';

  return (
    // shell.css → utilities: .plat-shell (flex row, full viewport), .plat-shell__aside
    // (+ collapsed state swaps width/padding values rather than layering overrides).
    // Task 9.5 — shell DOM 同构平移：Vue views/platform/index.vue .main >
    // components/menu.vue .aside_box（logo_row / menu_top / menu_bottom）。
    // 类名与结构 1:1（样式 platform-shell.td.css）；根容器与右侧 outlet 的
    // 布局 utilities 维持原值（与 Vue .main/.platform-route-outlet 计算值一致）。
    <div className="flex items-stretch w-full h-screen min-w-[600px] bg-white">
      <aside className={collapsed ? 'aside_box aside_box--collapsed' : 'aside_box'}>
        {/* 展开时：Logo + 搜索/折叠按钮同行（Vue menu.vue logo_row）。 */}
        {!collapsed ? <div className="logo_row">
          <a className="logo_box" style={{ cursor: 'pointer' }} href="/platform/knowledge-bases" aria-label="WeKnora" onClick={(event) => {
            if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
            event.preventDefault();
            navigate('/platform/knowledge-bases');
          }}>
            <img className="logo" src={weknoraLogo} alt="" />
            {/* Vue menu.vue:7 `<sup class="lite-badge">Lite</sup>` — edition mark, untranslated. */}
            {isLiteEdition ? <sup className="lite-badge">Lite</sup> : null}
          </a>
          <div className="logo_actions">
            {/* Vue menu.vue:10-21 t-tooltip(.cmdk-tip) > .header-icon-btn > img search.svg。
                tooltip 为 hover 态 DOM（静态不渲染），title 属性承接展开态提示。 */}
            <button type="button" className="header-icon-btn" onClick={() => { setPaletteQuery(''); setPaletteOpen(true); }} aria-label={t('menu.search')} title={t('menu.search')}>
              <img className="header-icon-img" src={searchIconUrl} alt="" />
            </button>
            <button type="button" className="sidebar-toggle" onClick={toggleCollapsed} aria-label={t('menu.collapseSidebar')} title={t('menu.collapseSidebar')}>
              <svg viewBox="0 0 20 20" width="18" height="18" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <rect x="1.5" y="1.5" width="17" height="17" rx="3" stroke="currentColor" strokeWidth="1.2" />
                <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" stroke="currentColor" strokeWidth="1.2" />
                <line x1="4" y1="7.5" x2="4" y2="12.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
              </svg>
            </button>
          </div>
        </div> : <>
          {/* 折叠时：展开按钮（Vue menu.vue:33-50 t-tooltip > .menu_item.sidebar-toggle-item）。 */}
          <button type="button" className="menu_item sidebar-toggle-item" onClick={toggleCollapsed} aria-label={t('menu.expandSidebar')} title={t('menu.expandSidebar')}>
            <span className="menu_item-box">
              <span className="menu_icon">
                <svg className="icon" viewBox="0 0 20 20" width="20" height="20" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                  <rect x="1.5" y="1.5" width="17" height="17" rx="3" stroke="currentColor" strokeWidth="1.2" />
                  <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" stroke="currentColor" strokeWidth="1.2" />
                  <line x1="5" y1="10" x2="3" y2="8" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
                  <line x1="5" y1="10" x2="3" y2="12" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
                </svg>
              </span>
            </span>
          </button>
          {/* 折叠时右侧拖拽展开手柄（Vue menu.vue:56 + onDragHandleMouseDown:1151-1169：
              右拖 >40px 展开侧栏）。 */}
          <div className="sidebar-drag-handle" onMouseDown={onDragHandleMouseDown} />
        </>}

        {/* 上半部分：新对话吸顶 + 知识库/智能体/共享空间/历史会话随滚动一起滚走
            （Vue menu.vue .menu_top）。 */}
        <div className="menu_top" onScroll={onSessionsScroll}>
          {/* 全局搜索入口：折叠态保留为图标项（Vue menu.vue:62-78 .menu_box--cmdk）。 */}
          {collapsed ? <div className="menu_box menu_box--cmdk">
            <button type="button" className="menu_item menu_item--cmdk" onClick={() => { setPaletteQuery(''); setPaletteOpen(true); }} aria-label={t('menu.search')} title={`${t('menu.search')} ${platformModKeyLabel(navigator.platform)}K`}>
              <span className="menu_item-box">
                <span className="menu_icon"><img className="icon" src={searchIconUrl} alt="" /></span>
              </span>
            </button>
          </div> : null}
          <nav className="flex flex-col" aria-label="Platform">
            {visibleNavItems.map((item) => {
              const active = item.match(pathname);
              // Vue menu.vue:79-84 — creatChat 带 children（childrenPath 'chat'）：
              // 会话详情页给 menu_item_c_active（无底色、文字主色），本区首页给
              // menu_item_active（底色 + 品牌色）。
              const chatDetailActive = item.key === 'newChat' && /^\/platform\/chat\//.test(pathname);
              const iconPair = NAV_ICON_URLS[item.icon];
              return (
                <div key={item.key} className={item.key === 'newChat' && !collapsed ? 'menu_box menu_box--sticky' : 'menu_box'}>
                  <a href={item.href} onClick={(event) => {
                    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
                    event.preventDefault();
                    navigate(item.href);
                  }} className={'menu_item'
                    + (chatDetailActive && !active ? ' menu_item_c_active' : '')
                    + (active ? ' menu_item_active' : '')}
                    aria-current={active ? 'page' : undefined}
                    title={collapsed ? item.label : undefined}
                    data-guide={item.guide}>
                    <span className="menu_item-box">
                      <span className="menu_icon">
                        {item.iconNode ?? <img className="icon" src={iconPair ? (active ? iconPair.active : iconPair.default) : ''} alt="" />}
                      </span>
                      {!collapsed ? <>
                        <span className="menu_title" title={item.label}>{item.label}</span>
                        {/* Vue menu.vue:93-98 — amber pending-join pill on the
                            organizations entry, expanded rail only, raw count. */}
                        {item.key === 'organizations' && orgPendingJoinRequestCount > 0 ? (
                          <span data-testid="org-pending-badge" title={t('organization.settings.pendingJoinRequestsBadge')} className="menu-pending-badge">
                            {orgPendingJoinRequestCount}
                          </span>
                        ) : null}
                      </> : null}
                    </span>
                  </a>
                </div>
              );
            })}
          </nav>

          {/* Vue menu.vue .submenu：历史会话按日期分组（折叠态隐藏）。
              SessionSidebarList 已按 SessionSidebarRow.vue 同构（packages/views）。 */}
          {!collapsed ? (
            <nav className="submenu" aria-label={labels.myChats}>
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
                onShareSession={canShareSessions ? setShareSessionId : undefined}
              />
            </nav>
          ) : null}
        </div>

        {/* 下半部分：用户菜单（Vue menu.vue .menu_bottom > UserMenu.vue）。 */}
        <div className="menu_bottom">
          <div ref={userMenuRef} className={collapsed ? 'user-menu user-menu--collapsed' : 'user-menu'}>
            {/* Vue UserMenu.vue .user-button：头像 + (空间名/角色 | 昵称/邮箱) + chevron。 */}
            <button type="button" className="user-button" data-guide="user-menu" aria-haspopup="menu" aria-expanded={menuOpen}
              onClick={() => setMenuOpen((open) => !open)}>
              <span className="user-avatar" aria-hidden="true">
                {user.avatar ? <img src={user.avatar} alt="" onError={(event) => { const img = event.currentTarget; if (!img.dataset.faviconFallback) { img.dataset.faviconFallback = '1'; img.src = '/favicon.ico'; } }} /> : <span className="avatar-placeholder">{initial}</span>}
              </span>
              {!collapsed ? <>
                <span className="user-info">
                  {showTenantIdentityLine ? <>
                    <span className="user-tenant-name" title={user.tenantName || user.name || undefined}>{user.tenantName || user.name || '—'}</span>
                    <span className="user-tenant-meta">
                      {user.name && user.name !== user.tenantName ? <span className="user-tenant-meta-name">{user.name}</span> : null}
                      {user.name && user.name !== user.tenantName && roleLabel ? <span aria-hidden="true" className="user-tenant-meta-sep">·</span> : null}
                      {roleIcon ? <TIcon name={roleIcon} size="12px" className="user-tenant-meta-icon" aria-hidden="true" /> : null}
                      {roleLabel ? <span className="user-tenant-meta-role">{roleLabel}</span> : null}
                    </span>
                  </> : <>
                    <span className="user-name">{user.name || '—'}</span>
                    <span className="user-email">{user.email}</span>
                  </>}
                </span>
                <TIcon name={menuOpen ? 'chevron-up' : 'chevron-down'} className="dropdown-icon" />
              </> : null}
            </button>
            {menuOpen ? (
              <div className="user-dropdown" role="menu" onClick={(event) => event.stopPropagation()}>
                {/* Vue UserMenu.vue:35-54 — 账号区（头像 + 昵称 + 重开引导按钮 + 邮箱），
                    整卡可点跳 userprofile。 */}
                <div role="button" tabIndex={0} className="dropdown-user-header is-clickable"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=userprofile', () => setMenuOpen(false))}
                  onKeyDown={(event) => { if (event.key === 'Enter') handleInternalLink(event, '/platform/settings?section=userprofile', () => setMenuOpen(false)); }}>
                  <span className="dropdown-user-avatar" aria-hidden="true">
                    {user.avatar ? <img src={user.avatar} alt="" onError={(event) => { const img = event.currentTarget; if (!img.dataset.faviconFallback) { img.dataset.faviconFallback = '1'; img.src = '/favicon.ico'; } }} /> : <span className="dropdown-user-avatar-placeholder">{initial}</span>}
                  </span>
                  <span className="dropdown-user-meta">
                    <span className="dropdown-user-name-row">
                      <span className="dropdown-user-name">{user.name || '—'}</span>
                      <button type="button" data-testid="plat-shell-guide-reopen" className="dropdown-guide-btn" aria-label={labels.reopenGuide} title={labels.reopenGuide}
                        onClick={(event) => { event.stopPropagation(); setMenuOpen(false); openNewUserGuide(); }}>
                        <TIcon name="help-circle" size="14px" />
                      </button>
                    </span>
                    {user.email ? <span className="dropdown-user-email">{user.email}</span> : null}
                  </span>
                </div>
                {/* Vue UserMenu.vue:56-74 — 当前工作区行（system-sum 图标 + 名称/角色
                    + 切换 glyph）；租户子列表为 React 内联 listbox（行为见既有测试）。 */}
                {!isLiteEdition ? <div role="group" aria-label={t('tenant.switcher.menuLabel')} className={'dropdown-tenant-panel' + (tenantSwitcherVisible ? ' is-clickable' : '') + (tenantMenuOpen ? ' is-open' : '')}>
                  <TIcon name="system-sum" className="menu-icon" aria-hidden="true" />
                  <span className="dropdown-tenant-panel-main">
                    <span className="dropdown-tenant-panel-name" title={user.tenantName || user.name || undefined}>{user.tenantName || user.name || '—'}</span>
                    {roleLabel ? <span className="dropdown-tenant-panel-role">
                      {roleIcon ? <TIcon name={roleIcon} size="12px" className="dropdown-tenant-panel-role-icon" aria-hidden="true" /> : null}
                      {roleLabel}
                    </span> : null}
                  </span>
                  {tenantSwitcherVisible ? <button type="button" aria-label={t('tenant.switcher.menuLabel')} title={t('tenant.switcher.menuLabel')} aria-expanded={tenantMenuOpen}
                    className="dropdown-tenant-panel-trail-btn" onClick={toggleTenantSubmenu}>
                    <TIcon name="swap" className="dropdown-tenant-panel-trail" />
                  </button> : null}
                </div> : null}
                {tenantMenuOpen ? <div role="listbox" aria-label={t('tenant.switcher.menuLabel')} className="tenant-submenu-inline">
                  {user.memberships.map((membership) => <button key={membership.tenantId} type="button" role="option" aria-selected={membership.tenantId === activeTenantId} disabled={tenantSwitchPending !== null} onClick={() => void switchTenant(membership.tenantId)}>
                    <span className="tenant-submenu-item-name">{membership.tenantName}</span><span className="tenant-submenu-role">{membership.tenantId === activeTenantId ? '当前' : membership.role}</span>
                  </button>)}
                </div> : null}
                <div className="menu-divider" aria-hidden="true" />
                <a role="menuitem" className="menu-item" href="/platform/settings?section=userprofile"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=userprofile', () => setMenuOpen(false))}>
                  <TIcon name="user" className="menu-icon" />
                  <span>{labels.personalSettings}</span>
                </a>
                {/* R450-A2 — Vue UserMenu.vue:81 gates the 「空间设置」 quick link
                    with !isLiteMode; lite deployments have no tenant surface. */}
                {!isLiteEdition ? <a role="menuitem" className="menu-item" href="/platform/settings?section=tenant"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=tenant', () => setMenuOpen(false))}>
                  <TIcon name="user-circle" className="menu-icon" />
                  <span>{labels.workspaceSettings}</span>
                </a> : null}
                {canSeeAdminSessionSources && !isLiteEdition ? <a role="menuitem" className="menu-item" href="/platform/settings?section=members"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=members', () => setMenuOpen(false))}>
                  <TIcon name="usergroup" className="menu-icon" />
                  <span>{labels.membersSettings}</span>
                </a> : null}
                {canSeeAdminSessionSources && !isLiteEdition ? <a role="menuitem" className="menu-item" href="/platform/settings?section=models"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=models', () => setMenuOpen(false))}>
                  <TIcon name="control-platform" className="menu-icon" />
                  <span>{labels.modelsSettings}</span>
                </a> : null}
                {canSeeAdminSessionSources && !isLiteEdition ? <a role="menuitem" className="menu-item" href="/platform/settings?section=skills"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=skills', () => setMenuOpen(false))}>
                  <TIcon name="system-code" className="menu-icon" />
                  <span>{labels.skillsSettings}</span>
                </a> : null}
                {/* Vue UserMenu.vue:99-103 — a divider closes the section quick-link
                    group, then the unconditional 「全部设置」 entry (no section query). */}
                <div className="menu-divider" aria-hidden="true" />
                <a role="menuitem" className="menu-item" href="/platform/settings"
                  onClick={(event) => handleInternalLink(event, '/platform/settings', () => setMenuOpen(false))}>
                  <TIcon name="setting" className="menu-icon" />
                  <span>{labels.allSettings}</span>
                </a>
                {/* R449-A2 — Vue UserMenu.vue:110-113 renders 「系统管理」 only for
                    is_system_admin users. UI gating only; server middleware is the
                    real boundary. */}
                {user.isSystemAdmin ? <a role="menuitem" className="menu-item" href="/platform/settings?section=system-global"
                  onClick={(event) => handleInternalLink(event, '/platform/settings?section=system-global', () => setMenuOpen(false))}>
                  <TIcon name="server" className="menu-icon" />
                  <span>{labels.systemAdministration}</span>
                </a> : null}
                <div className="menu-divider" aria-hidden="true" />
                <a role="menuitem" className="menu-item" href="https://github.com/Tencent/WeKnora/tree/main/docs" target="_blank" rel="noreferrer"
                  onClick={() => setMenuOpen(false)}>
                  <TIcon name="help-circle" className="menu-icon" />
                  <span className="menu-text-with-icon">
                    <span>{labels.helpAndDocs}</span>
                    <svg className="menu-external-icon" viewBox="0 0 16 16" aria-hidden="true">
                      <path fill="currentColor" d="M12.667 8a.667.667 0 0 1 .666.667v4a2.667 2.667 0 0 1-2.666 2.666H4.667a2.667 2.667 0 0 1-2.667-2.666V5.333a2.667 2.667 0 0 1 2.667-2.666h4a.667.667 0 1 1 0 1.333h-4a1.333 1.333 0 0 0-1.333 1.333v7.334A1.333 1.333 0 0 0 4.667 13.333h6a1.333 1.333 0 0 0 1.333-1.333v-4A.667.667 0 0 1 12.667 8Zm2.666-6.667v4a.667.667 0 0 1-1.333 0V3.276l-5.195 5.195a.667.667 0 0 1-.943-.943l5.195-5.195h-2.057a.667.667 0 0 1 0-1.333h4a.667.667 0 0 1 .666.666Z" />
                    </svg>
                  </span>
                </a>
                <a role="menuitem" className="menu-item" href="https://github.com/Tencent/WeKnora" target="_blank" rel="noreferrer" title={labels.githubStarTip}
                  onClick={() => setMenuOpen(false)}>
                  <TIcon name="logo-github" className="menu-icon" />
                  <span className="menu-text-with-icon">
                    <span>{labels.github}</span>
                    <TIcon name="star-filled" className="menu-github-star-icon" size="16px" aria-hidden="true" />
                    <svg className="menu-external-icon" viewBox="0 0 16 16" aria-hidden="true">
                      <path fill="currentColor" d="M12.667 8a.667.667 0 0 1 .666.667v4a2.667 2.667 0 0 1-2.666 2.666H4.667a2.667 2.667 0 0 1-2.667-2.666V5.333a2.667 2.667 0 0 1 2.667-2.666h4a.667.667 0 1 1 0 1.333h-4a1.333 1.333 0 0 0-1.333 1.333v7.334A1.333 1.333 0 0 0 4.667 13.333h6a1.333 1.333 0 0 0 1.333-1.333v-4A.667.667 0 0 1 12.667 8Zm2.666-6.667v4a.667.667 0 0 1-1.333 0V3.276l-5.195 5.195a.667.667 0 0 1-.943-.943l5.195-5.195h-2.057a.667.667 0 0 1 0-1.333h4a.667.667 0 0 1 .666.666Z" />
                    </svg>
                  </span>
                </a>
                {/* R450-A2 — Vue UserMenu.vue:136-144 keeps the divider and the
                    logout item behind !isLiteMode. */}
                {!isLiteEdition ? <>
                  <div className="menu-divider" aria-hidden="true" />
                  <button type="button" role="menuitem" className="menu-item danger"
                    onClick={() => { setMenuOpen(false); void runShellLogout(onLogout); }}>
                    <TIcon name="logout" className="menu-icon" />
                    <span>{t('auth.logout')}</span>
                  </button>
                </> : null}
              </div>
            ) : null}
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
        searchClient={client}
        initialKbScope={kbScopeFromLocation()}
        access={paletteAccess}
        agentsEnabled={paletteAccess.canOpenAgents}
        retrievalSettings={paletteRetrievalSettings}
      />
      {/* 带遮罩层的新手引导：首次进入自动开启 (Vue platform/index.vue:18). */}
      <NewUserGuide locale={locale} actions={guideActions} />
      {/* Contextual guides (Vue mounts ContextualGuide/KbCreateContextualGuide/
          AgentCreateContextualGuide/TenantModelsGuide per page): one host in
          the shell is fed by openContextualGuide(tour) trigger calls from the
          pages, so page wiring stays a one-liner. */}
      <ContextualGuideHost locale={locale} actions={guideActions} />
      <InvitationInbox client={client} />
      {/* SP13 Task 8 — 侧栏分享弹窗 + 结果 toast（ChatRoutePage agentToast 同
          形态；shell 侧没有聊天宿主的 toast 通道，就地挂一个）。 */}
      {shareSessionId ? (
        <SessionShareDialog
          client={client}
          sessionId={shareSessionId}
          sessionTitle={sessions.find((session) => session.id === shareSessionId)?.title}
          locale={locale}
          onClose={() => setShareSessionId(null)}
          onToast={showShellShareToast}
        />
      ) : null}
      {shellShareToast ? (
        <div role="status" aria-live="polite" className="fixed bottom-[76px] left-1/2 z-[10050] -translate-x-1/2 rounded-[8px] bg-[rgba(0,0,0,0.78)] px-[14px] py-[8px] text-[13px] text-white shadow-[0_4px_12px_rgba(0,0,0,0.2)]">
          {shellShareToast}
        </div>
      ) : null}
    </div>
  );
}
