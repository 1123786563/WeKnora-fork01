/** T22 (#52)：代码交付只读投影。Screen 只消费 DeliveryReceiptView，
 * 不接触 wire 行、digest 或 action 生命周期。 */

export type DeliveryState = 'prepared' | 'dispatched' | 'pushed' | 'delivered' | 'failed' | 'unknown';

/** 与 contracts CodeDeliveryRecord / api-client MobileCodeDeliveryRemote 结构逐字一致
 * （结构可赋值由 apps/mobile typecheck 证明；mobile-core 不 import contracts）。 */
export interface DeliveryRemoteRecord {
  id: string;
  taskId: string;
  runId: string;
  state: DeliveryState;
  repo: string;
  baselineSha: string;
  branch: string;
  commitSha?: string;
  prNumber?: number;
  prUrl?: string;
  remoteLogin?: string;
  actionId: string;
  actionState: string;
  digest: string;
  approver?: string;
  failure?: string;
  files: number;
  createdAt: string;
  updatedAt: string;
}

export interface DeliveryReceiptView {
  deliveryId: string;
  taskId: string;
  runId: string;
  state: DeliveryState;
  repo: string;
  branch: string;
  baselineSha: string;
  commitSha?: string;
  prNumber?: number;
  prUrl?: string;
  remoteLogin?: string;
  approver?: string;
  failure?: string;
  /** 待 Owner 行动：待审批 / 部分完成待恢复 / 远端待收敛。 */
  attention: boolean;
  updatedAt: string;
}

export function deliveryViewOf(record: DeliveryRemoteRecord): DeliveryReceiptView {
  const attention = record.state === 'prepared' || record.state === 'pushed' || record.state === 'unknown';
  return {
    deliveryId: record.id, taskId: record.taskId, runId: record.runId, state: record.state,
    repo: record.repo, branch: record.branch, baselineSha: record.baselineSha,
    ...(record.commitSha === undefined ? {} : { commitSha: record.commitSha }),
    ...(record.prNumber === undefined ? {} : { prNumber: record.prNumber }),
    ...(record.prUrl === undefined ? {} : { prUrl: record.prUrl }),
    ...(record.remoteLogin === undefined ? {} : { remoteLogin: record.remoteLogin }),
    ...(record.approver === undefined ? {} : { approver: record.approver }),
    ...(record.failure === undefined || record.failure === '' ? {} : { failure: record.failure }),
    attention, updatedAt: record.updatedAt,
  };
}
