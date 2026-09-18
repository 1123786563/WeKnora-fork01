/**
 * 空间用量展示领域模型（MX-031 / M18）。
 * 冻结规则：
 * - 只读商业视图：数据全部来自服务端（OpenMeter 模型/商业域 A 类事实），客户端不建钱包、
 *   不硬编码示例价格、不发起购买/退款（paymentRequests 结构性为 0）；
 * - 金额按服务端单位与 as_of 展示；reserved/pending/settled 三桶分离，
 *   pending 标注「未最终」（结算最终性以服务端为准）；
 * - 账单账户作用域（billing account scope）：不混用其他空间余额；
 * - BYOK 模型费与平台服务费分开（不假定全任务免费）。
 */

export interface UsageAmount {
  /** 服务端原值（整数最小单位）；展示层换算由 as_of 单位说明承载 */
  value: number;
  currency: string;
  unitLabel: string;
}

export interface TenantUsageSnapshot {
  tenantId: string;
  billingAccountScope: string;
  reserved: UsageAmount;
  pending: UsageAmount;
  settled: UsageAmount;
  /** BYOK 模型费（自带密钥）与平台服务费分列 */
  breakdown: { byokModelFees: UsageAmount | null; platformServiceFees: UsageAmount };
  asOf: string;
  viewerRole: 'owner' | 'admin' | 'contributor' | 'viewer';
}

export interface UsagePresentation {
  reserved: number;
  pending: number;
  settled: number;
  pendingIsFinal: boolean;
  scopeLabel: string;
  asOfLabel: string;
  /** 展示层不发起任何支付动作 */
  paymentRequests: 0;
  canManageBilling: boolean;
}

/** 只读展示投影：三桶分离+未最终标注+作用域标签；不做任何换算/合计发明。 */
export function presentUsage(snapshot: TenantUsageSnapshot): UsagePresentation {
  return {
    reserved: snapshot.reserved.value,
    pending: snapshot.pending.value,
    settled: snapshot.settled.value,
    pendingIsFinal: false, // pending 按定义未最终；最终性以服务端 settled 为准
    scopeLabel: `${snapshot.billingAccountScope}（空间 ${snapshot.tenantId} 专属账单账户，不与其他空间混用）`,
    asOfLabel: `数据截至 ${snapshot.asOf}（单位：${snapshot.settled.unitLabel}，以服务端结算为准）`,
    paymentRequests: 0,
    canManageBilling: snapshot.viewerRole === 'owner',
  };
}

/** 敏感边界：viewer/contributor 不展示管理入口（普通 Admin 与 billing admin 分权）。 */
export function canViewBillingActions(role: TenantUsageSnapshot['viewerRole']): boolean {
  return role === 'owner';
}
