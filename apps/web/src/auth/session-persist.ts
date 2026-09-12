// Port of Vue Login.vue persistLoginResponse redirect/tenant-override logic
// (frontend/src/views/auth/Login.vue:547-600) as pure decisions.
import type { AuthSession } from '@weknora/api-client';

export interface AuthLanding {
  /** Tenant whose X-Tenant-ID should be applied going forward (active tenant). */
  activeTenantId: string | null;
  /** The user's immutable home tenant id (users.tenant_id). */
  homeTenantId: string | null;
  /** Set when the server honoured a remembered last-active-tenant preference. */
  tenantOverride: { tenantId: string; } | null;
  /** Where to navigate: onboarding when no valid tenant, else the next path. */
  target: string;
}

function tenantIdOf(value: unknown): string | null {
  if (!value || typeof value !== 'object') return null;
  const raw = (value as Record<string, unknown>).id;
  if (typeof raw === 'string' && raw.trim()) return raw;
  if (typeof raw === 'number' && Number.isSafeInteger(raw) && raw > 0) return String(raw);
  return null;
}

export function computeAuthLanding(session: AuthSession, nextPath?: string): AuthLanding {
  const activeTenantId = tenantIdOf(session.tenant);
  const userTenantRaw = session.user?.['tenant_id'];
  const homeTenantId = tenantIdOf({ id: userTenantRaw }) ?? activeTenantId;
  const differs = activeTenantId !== null && homeTenantId !== null && activeTenantId !== homeTenantId;
  const tenantOverride = differs ? { tenantId: activeTenantId! } : null;
  const target = activeTenantId === null ? '/onboarding/workspace' : (nextPath && nextPath.startsWith('/') && !nextPath.startsWith('//') ? nextPath : '/platform/knowledge-bases');
  return { activeTenantId, homeTenantId, tenantOverride, target };
}
