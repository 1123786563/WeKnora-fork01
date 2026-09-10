import { createScopeController, scopedKey, type ScopeController, type ScopeHandle } from '@weknora/domain';

export interface WebScopeRuntime {
  controller: ScopeController;
  current(): ScopeHandle;
  setIdentity(userId: string | null, tenantId: string | null): ScopeHandle;
  setTenant(tenantId: string | null): ScopeHandle;
  key(resource: string, params?: unknown): readonly unknown[];
}

export function createWebScopeRuntime(origin: string, userId: string | null = null, tenantId: string | null = null): WebScopeRuntime {
  const controller = createScopeController({ origin, userId, tenantId });
  return {
    controller,
    current: () => controller.current(),
    setIdentity: (nextUserId, nextTenantId) => controller.switchScope(origin, nextUserId, nextTenantId),
    setTenant: (nextTenantId) => controller.switchScope(origin, controller.current().scope.userId, nextTenantId),
    key: (resource, params = {}) => scopedKey(controller.current().scope, resource, params),
  };
}
