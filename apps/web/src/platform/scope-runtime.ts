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
  key(resource: string, params?: unknown): readonly unknown[];
}

export function createWebScopeRuntime(origin: string, userId: string | null = null, tenantId: string | null = null, options: { liteMode?: boolean; edition?: string } = {}): WebScopeRuntime {
  const controller = createScopeController({ origin, userId, tenantId });
  let capabilitySnapshot: CapabilityMap = {};
  let liteMode = options.liteMode === true;
  let edition = options.edition;
  let systemAdmin = false;
  const hydrate = (authMe: AuthMe): ScopeHandle => {
    const nextUserId = typeof authMe.user.id === 'string' && authMe.user.id.trim() ? authMe.user.id : null;
    const rawTenant = authMe.tenant;
    const nextTenantId = rawTenant && (typeof rawTenant.id === 'string' || typeof rawTenant.id === 'number') ? String(rawTenant.id) : null;
    capabilitySnapshot = normalizeCapabilityMap(authMe.capabilities);
    systemAdmin = authMe.user.is_system_admin === true || authMe.user.isSystemAdmin === true;
    const maybeEdition = authMe.user.edition;
    if (typeof maybeEdition === 'string' && maybeEdition.trim()) edition = maybeEdition;
    return controller.switchScope(origin, nextUserId, nextTenantId);
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
    return controller.switchScope(origin, controller.current().scope.userId, tenantId);
  };
  return {
    controller,
    current: () => controller.current(),
    hydrate,
    setIdentity: (nextUserId, nextTenantId) => controller.switchScope(origin, nextUserId, nextTenantId),
    setTenant: (nextTenantId) => controller.switchScope(origin, controller.current().scope.userId, nextTenantId),
    switchTenant,
    logout: () => { capabilitySnapshot = {}; systemAdmin = false; controller.logout(); },
    requiresWorkspace: () => controller.current().scope.userId !== null && controller.current().scope.tenantId === null,
    can: (capability) => isCapabilitySupported(capabilitySnapshot, capability, { liteMode, edition }),
    capabilities: () => ({ ...capabilitySnapshot }),
    isSystemAdmin: () => systemAdmin,
    key: (resource, params = {}) => scopedKey(controller.current().scope, resource, params),
  };
}
