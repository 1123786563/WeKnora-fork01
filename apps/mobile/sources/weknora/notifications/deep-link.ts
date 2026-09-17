/**
 * 安全深链（MX-021 / M10）。
 * 冻结规则：
 * - 深链只携资源标识（kind+id），绝不携 token/审批正文/决定；
 * - 处理顺序固定：先认证（未登录→登录页，登录后回跳原链）→ 空间确认（tenant 匹配）→
 *   重查权限（打开目标页时走各自 openConfirmation/authorize——通知数据不作授权依据）；
 * - 跨账户深链（intent 的 tenant 与当前 scope 不符）：不投递给当前用户（导向空间选择）。
 */

export type DeepLinkKind = 'inbox' | 'run' | 'interaction' | 'artifact';

export interface DeepLinkIntent {
  kind: DeepLinkKind;
  /** 资源标识（run_id / interaction_id / artifact_id / notification_id） */
  id: string;
  /** 目标空间（服务端通知元数据携带；客户端核对后消费） */
  tenantId: string;
}

export interface DeepLinkScope {
  authenticated: boolean;
  currentTenantId: string | null;
}

export type DeepLinkResolution =
  | { action: 'login'; intent: DeepLinkIntent }
  | { action: 'switch-space'; intent: DeepLinkIntent }
  | { action: 'open'; target: { kind: DeepLinkKind; id: string } };

const KIND_SEGMENTS = ['inbox', 'runs', 'interactions', 'artifacts'] as const;
const KIND_BY_SEGMENT: Record<string, DeepLinkKind> = {
  inbox: 'inbox', runs: 'run', interactions: 'interaction', artifacts: 'artifact',
};
const ID_CHARS = /[A-Za-z0-9_-]/;

/** 解析 weknora:// 链接：只接受「前缀 + 白名单段 + 标识符」，标识符逐字符白名单校验。 */
export function parseDeepLink(url: string): DeepLinkIntent | null {
  const trimmed = url.trim();
  if (!trimmed.startsWith('weknora://')) return null;
  const segment = trimmed.slice('weknora://'.length);
  const slash = segment.indexOf('/');
  if (slash <= 0) return null;
  const kindSegment = segment.slice(0, slash);
  const id = segment.slice(slash + 1);
  if (!KIND_SEGMENTS.includes(kindSegment as (typeof KIND_SEGMENTS)[number])) return null;
  if (id.length === 0 || id.length > 128) return null;
  for (const char of id) {
    if (!ID_CHARS.test(char)) return null;
  }
  return { kind: KIND_BY_SEGMENT[kindSegment] ?? 'inbox', id, tenantId: '' };
}

/** 恢复顺序裁决：认证 → 空间 → 打开（打开时重查权限）。 */
export function resolveDeepLink(intent: DeepLinkIntent, scope: DeepLinkScope, notificationTenant: string): DeepLinkResolution {
  if (!scope.authenticated) {
    return { action: 'login', intent };
  }
  if (scope.currentTenantId === null || scope.currentTenantId !== notificationTenant) {
    return { action: 'switch-space', intent: { ...intent, tenantId: notificationTenant } };
  }
  return { action: 'open', target: { kind: intent.kind, id: intent.id } };
}

/** 跨账户投递守卫：通知归属租户/用户 ≠ 当前会话 → 不投递（device-A-logout-B-login 场景）。 */
export function shouldDeliverNotification(notificationTenant: string, notificationUser: string, current: { tenantId: string | null; userId: string | null }): boolean {
  if (current.tenantId === null || current.userId === null) return false;
  return notificationTenant === current.tenantId && notificationUser === current.userId;
}
