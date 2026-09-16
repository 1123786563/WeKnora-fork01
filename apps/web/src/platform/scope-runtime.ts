import { createScopeController, isCapabilitySupported, normalizeCapabilityMap, scopedKey, type CapabilityMap, type ScopeController, type ScopeHandle } from '@weknora/domain';
import type { AuthMe, AuthSession } from '@weknora/api-client';

export interface WebScopeRuntime {
  controller: ScopeController;
  current(): ScopeHandle;
  hydrate(authMe: AuthMe): ScopeHandle;
  setIdentity(userId: string | null, tenantId: string | null): ScopeHandle;
  setTenant(tenantId: string | null): ScopeHandle;
  switchTenant(
    tenantId: string,
    switcher: (tenantId: number, refreshToken?: string) => Promise<AuthSession>,
    persistSession: (session: AuthSession) => Promise<void> | void,
    refreshToken?: string,
  ): Promise<ScopeHandle>;
  logout(): void;
  requiresWorkspace(): boolean;
  can(capability?: string): boolean;
  capabilities(): CapabilityMap;
  isSystemAdmin(): boolean;
  canViewChannelSessions(): boolean;
  /** Active membership role in the current tenant scope ('viewer'|'contributor'|'admin'|'owner'). */
  role(): WebTenantRole;
  key(resource: string, params?: unknown): readonly unknown[];
}

type WebTenantRole = 'owner' | 'admin' | 'contributor' | 'viewer';

function membershipRole(memberships: unknown[] | undefined, tenantId: string | null): WebTenantRole {
  if (!tenantId) return 'viewer';
  for (const item of memberships ?? []) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) continue;
    const row = item as Record<string, unknown>;
    const id = row.tenant_id ?? row.tenantId;
    const role = row.role;
    if (String(id) === tenantId && (role === 'owner' || role === 'admin' || role === 'contributor' || role === 'viewer')) return role;
  }
  return 'viewer';
}

export function createWebScopeRuntime(origin: string, userId: string | null = null, tenantId: string | null = null, options: { liteMode?: boolean; edition?: string; persistTenant?: (tenantId: string | null) => void } = {}): WebScopeRuntime {
  const controller = createScopeController({ origin, userId, tenantId });
  let capabilitySnapshot: CapabilityMap = {};
  let liteMode = options.liteMode === true;
  let edition = options.edition;
  let systemAdmin = false;
  let activeRole: WebTenantRole = 'viewer';
  const commitScope = (nextUserId: string | null, nextTenantId: string | null): ScopeHandle => {
    const handle = controller.switchScope(origin, nextUserId, nextTenantId);
    options.persistTenant?.(nextTenantId);
    return handle;
  };
  const hydrate = (authMe: AuthMe): ScopeHandle => {
    const nextUserId = typeof authMe.user.id === 'string' && authMe.user.id.trim() ? authMe.user.id : null;
    const rawTenant = authMe.tenant;
    const nextTenantId = rawTenant && (typeof rawTenant.id === 'string' || typeof rawTenant.id === 'number') ? String(rawTenant.id) : null;
    const currentScope = controller.current().scope;
    // Vue's TenantSelector/UserMenu keeps the selected-tenant override in
    // local storage and sends it as X-Tenant-ID. During a reload, /auth/me
    // can still describe the JWT home tenant before that override is reflected
    // in the response. Keep the override for the same user (or the initial
    // pre-hydration scope), but never carry it into a tenantless response or a
    // different user session.
    const tenantId = nextTenantId !== null
      && currentScope.tenantId !== null
      && (currentScope.userId === null || currentScope.userId === nextUserId)
      ? currentScope.tenantId
      : nextTenantId;
    capabilitySnapshot = normalizeCapabilityMap(authMe.capabilities);
    systemAdmin = authMe.user.is_system_admin === true || authMe.user.isSystemAdmin === true;
    const maybeEdition = authMe.user.edition;
    if (typeof maybeEdition === 'string' && maybeEdition.trim()) edition = maybeEdition;
    activeRole = membershipRole(authMe.memberships, tenantId);
    return commitScope(nextUserId, tenantId);
  };
  const switchTenant = async (
    tenantId: string,
    switcher: (nextTenantId: number, refreshToken?: string) => Promise<AuthSession>,
    persistSession: (session: AuthSession) => Promise<void> | void,
    refreshToken?: string,
  ): Promise<ScopeHandle> => {
    const numericTenantId = Number(tenantId);
    if (!Number.isSafeInteger(numericTenantId) || numericTenantId <= 0) throw new Error('tenantId must be a positive safe integer');
    const session = await switcher(numericTenantId, refreshToken);
    await persistSession(session);
    activeRole = membershipRole(session.memberships, tenantId);
    return commitScope(controller.current().scope.userId, tenantId);
  };
  return {
    controller,
    current: () => controller.current(),
    hydrate,
    setIdentity: commitScope,
    setTenant: (nextTenantId) => commitScope(controller.current().scope.userId, nextTenantId),
    switchTenant,
    logout: () => { capabilitySnapshot = {}; systemAdmin = false; activeRole = 'viewer'; controller.logout(); options.persistTenant?.(null); },
    requiresWorkspace: () => controller.current().scope.userId !== null && controller.current().scope.tenantId === null,
    can: (capability) => isCapabilitySupported(capabilitySnapshot, capability, { liteMode, edition }),
    capabilities: () => ({ ...capabilitySnapshot }),
    isSystemAdmin: () => systemAdmin,
    canViewChannelSessions: () => systemAdmin || activeRole === 'owner' || activeRole === 'admin',
    role: () => activeRole,
    key: (resource, params = {}) => scopedKey(controller.current().scope, resource, params),
  };
}
