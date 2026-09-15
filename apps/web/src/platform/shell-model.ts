export const PLATFORM_SIDEBAR_WIDTH = 260;
export const PLATFORM_SIDEBAR_COLLAPSED_WIDTH = 60;
export const PLATFORM_HEADER_HEIGHT = 50;
export const PLATFORM_MIN_LAYOUT_WIDTH = 600;

export interface PlatformNavItem {
  id: string;
  label: string;
  path: string;
  icon: string;
  children?: PlatformNavItem[];
}

/** Vue menu order and route ownership, kept independent from page components. */
export const platformNavItems: readonly PlatformNavItem[] = [
  { id: 'new-chat', label: 'New chat', path: '/platform/creatChat', icon: 'plus' },
  { id: 'knowledge-bases', label: 'Knowledge bases', path: '/platform/knowledge-bases', icon: 'knowledge' },
  { id: 'agents', label: 'Agents', path: '/platform/agents', icon: 'agent' },
  { id: 'organizations', label: 'Organizations', path: '/platform/organizations', icon: 'organization' },
  { id: 'settings', label: 'Settings', path: '/platform/settings', icon: 'settings' },
];

function pathMatches(pathname: string, itemPath: string): boolean {
  const path = pathname.split('?')[0]?.split('#')[0] || '/';
  return path === itemPath || path.startsWith(`${itemPath}/`);
}

export function isPlatformNavActive(pathname: string, item: PlatformNavItem): boolean {
  return pathMatches(pathname, item.path) || Boolean(item.children?.some((child) => pathMatches(pathname, child.path)));
}

export function activePlatformNavId(pathname: string, items: readonly PlatformNavItem[] = platformNavItems): string | null {
  // Prefer the most specific match so /knowledge-bases/:id does not lose to a parent.
  return items
    .flatMap((item) => [item, ...(item.children ?? [])])
    .filter((item) => pathMatches(pathname, item.path))
    .sort((a, b) => b.path.length - a.path.length)[0]?.id ?? null;
}

export interface TenantMembership {
  tenantId: string;
  name: string;
  role?: string;
  isHome?: boolean;
}

export function tenantDisplayName(membership: Pick<TenantMembership, 'tenantId' | 'name'>): string {
  const name = membership.name.trim();
  return name || `#${membership.tenantId}`;
}

export function tenantInitial(membership: Pick<TenantMembership, 'tenantId' | 'name'>): string {
  return (tenantDisplayName(membership).trim()[0] ?? '#').toUpperCase();
}

export interface PlatformGeometry {
  sidebarWidth: number;
  contentWidth: number;
  contentMinWidth: number;
  collapsed: boolean;
}

/** Geometry mirrors Vue's 260/60 sidebar and preserves a 600px desktop layout floor. */
export function platformGeometry(viewportWidth: number, collapsed: boolean): PlatformGeometry {
  const sidebarWidth = collapsed ? PLATFORM_SIDEBAR_COLLAPSED_WIDTH : PLATFORM_SIDEBAR_WIDTH;
  return {
    sidebarWidth,
    contentWidth: Math.max(0, viewportWidth - sidebarWidth),
    contentMinWidth: Math.max(0, PLATFORM_MIN_LAYOUT_WIDTH - sidebarWidth),
    collapsed,
  };
}

export interface CommandItem {
  id: string;
  label: string;
  hint?: string;
  keywords?: readonly string[];
  run: () => void;
}

export function filterCommandItems(items: readonly CommandItem[], query: string): CommandItem[] {
  const normalized = query.trim().toLocaleLowerCase();
  if (!normalized) return [...items];
  return items.filter((item) => [item.label, item.hint, ...(item.keywords ?? [])]
    .filter(Boolean)
    .some((value) => value!.toLocaleLowerCase().includes(normalized)));
}

export function nextCommandIndex(current: number, count: number, direction: 1 | -1): number {
  if (count <= 0) return -1;
  return (current + direction + count) % count;
}

export function userInitial(name: string): string {
  return name.trim()[0]?.toUpperCase() ?? '?';
}

export interface PlatformUserMenuItem {
  id: 'personal-settings' | 'workspace-settings' | 'members' | 'all-settings' | 'logout';
  label: string;
  path?: string;
  danger?: boolean;
}

/** The account menu keeps the same identity/context split as the Vue menu. */
export function platformUserMenuItems(options: {
  hasTenant: boolean;
  canManageMembers: boolean;
  canLogout: boolean;
}): PlatformUserMenuItem[] {
  const items: PlatformUserMenuItem[] = [
    { id: 'personal-settings', label: 'Personal settings', path: '/platform/settings?section=userprofile' },
  ];
  if (options.hasTenant) {
    items.push({ id: 'workspace-settings', label: 'Workspace settings', path: '/platform/settings?section=tenant' });
  }
  if (options.canManageMembers) {
    items.push({ id: 'members', label: 'Members', path: '/platform/settings?section=members' });
  }
  items.push({ id: 'all-settings', label: 'All settings', path: '/platform/settings' });
  if (options.canLogout) items.push({ id: 'logout', label: 'Log out', danger: true });
  return items;
}
