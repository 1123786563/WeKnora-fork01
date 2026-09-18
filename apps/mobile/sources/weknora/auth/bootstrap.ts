import type { AuthApi, AuthMe } from '@weknora/api-client';

/**
 * 冷启动身份引导（MX-010，关闭 G04 身份面）。
 * 规则：恢复凭据 ≠ 恢复身份——有效凭据且 scope 缺失（userId/tenantId 未知）时，
 * 必须以真实身份端点（A 类 GET /api/v1/auth/me）取得 user + memberships 再进 scope；
 * 不从 Token 本地臆造空间归属。失败保留凭据、scope 维持未定（登录门仍可达）。
 */

export interface MembershipSummary {
  tenantId: string;
  tenantName?: string;
  role?: string;
  status?: string;
}

export interface ScopeBootstrap {
  userId: string;
  tenantId: string | null;
  memberships: MembershipSummary[];
  tenantRequired: boolean;
}

function membershipRow(value: unknown): MembershipSummary | null {
  if (typeof value !== 'object' || value === null) return null;
  const row = value as Record<string, unknown>;
  const rawId = row.tenant_id ?? row.tenantId;
  if (typeof rawId !== 'number' && typeof rawId !== 'string') return null;
  return {
    tenantId: String(rawId),
    ...(typeof (row.tenant_name ?? row.tenantName) === 'string' ? { tenantName: String(row.tenant_name ?? row.tenantName) } : {}),
    ...(typeof row.role === 'string' ? { role: row.role } : {}),
    ...(typeof row.status === 'string' ? { status: row.status } : {}),
  };
}

/** 纯解析：AuthMe → scope 事实。空间选择优先级：调用方记忆的选择 > 服务端当前 tenant > 首个成员关系。 */
export function resolveScopeFromMe(me: AuthMe, preferredTenantId?: string | null): ScopeBootstrap {
  const userId = String((me.user as { id?: unknown }).id ?? '').trim();
  if (!userId) throw new Error('bootstrap: auth me user.id is required');
  const memberships = (me.memberships ?? []).map(membershipRow).filter((row): row is MembershipSummary => row !== null);
  const active = memberships.filter((row) => !row.status || row.status === 'active');
  let tenantId: string | null = null;
  if (preferredTenantId && (active.length === 0 || active.some((row) => row.tenantId === preferredTenantId))) {
    tenantId = preferredTenantId;
  } else if (me.tenant && typeof (me.tenant as { id?: unknown }).id !== 'undefined') {
    tenantId = String((me.tenant as { id: unknown }).id);
  } else if (active.length > 0) {
    tenantId = active[0]!.tenantId;
  }
  return { userId, tenantId, memberships, tenantRequired: me.tenant_required === true };
}

export interface BootstrapPort {
  /** 拉取当前身份并解析 scope；网络/协议失败上抛（调用方保留凭据、不臆造 scope）。 */
  run(preferredTenantId?: string | null): Promise<ScopeBootstrap>;
}

export function createBootstrapPort(authApi: AuthApi): BootstrapPort {
  return {
    async run(preferredTenantId) {
      const me = await authApi.me();
      return resolveScopeFromMe(me, preferredTenantId);
    },
  };
}
