/**
 * 空间隔离查询键（MX-011）。
 * 缓存按规范化 origin/user/tenant 隔离：任何查询键都由此构造，页面不手拼数组；
 * 迟到响应以 generation acceptance 判定（配合 ProductScope.capture/accept），
 * 键隔离只保证缓存不串，不代替响应丢弃。
 */

export interface QueryScopeIdentity {
  origin: string;
  userId: string | null;
  tenantId: string | null;
}

/** 规范化：小写 host、去尾斜杠——防止同源不同写法分裂缓存。 */
export function normalizeOrigin(origin: string): string {
  const trimmed = origin.trim().replace(/\/+$/, '');
  try {
    const url = new URL(trimmed);
    return `${url.protocol}//${url.host.toLowerCase()}${url.pathname.replace(/\/+$/, '')}`;
  } catch {
    return trimmed.toLowerCase();
  }
}

export function queryScopeKey(identity: QueryScopeIdentity): readonly string[] {
  const origin = normalizeOrigin(identity.origin);
  const user = identity.userId?.trim() || 'anonymous';
  const tenant = identity.tenantId?.trim() || 'no-tenant';
  return ['weknora', origin, user, tenant] as const;
}

/** 空间内查询键：scope 前缀 + 业务段（业务段由调用方给稳定字面量）。 */
export function productQueryKey(identity: QueryScopeIdentity, ...parts: readonly string[]): readonly unknown[] {
  return [...queryScopeKey(identity), ...parts];
}

/** 迟到响应守卫：捕获时 generation 与提交时 scope 的 accept 判定结合。 */
export interface GenerationGuard {
  generation: number;
  /** 提交结果前调用：false = 已被切换取代，必须丢弃（不改写新空间任何状态）。 */
  isCurrent(): boolean;
}

export function createGenerationGuard(accept: (generation: number) => boolean, generation: number): GenerationGuard {
  return { generation, isCurrent: () => accept(generation) };
}
