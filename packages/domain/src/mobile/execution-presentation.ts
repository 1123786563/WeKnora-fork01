/**
 * 执行状态展示领域模型（MX-018 / M08）。
 * 冻结规则：
 * - run/execution/settlement 三个原值分别保留 + 观察时间（as_of）；
 * - 「申请取消」ACK 后展示 stop_pending（停止待确认）——批准已记录≠执行停止≠退款；
 * - unknown 状态使用专用查询值（unknown_lookup），不默认显示为失败或完成；
 * - 无 revision（无快照）不发送 command（调用方守卫）；终态结果可查看但结算可仍 pending；
 * - refundRequested 永远不为 true——客户端不发起、不展示、不暗示自动退款。
 */

export type StopObservation = 'idle' | 'stop_pending' | 'stop_confirmed' | 'unknown_lookup';

export interface ExecutionObservation {
  runStatus: string;
  executionStatus: string;
  settlementStatus: string;
  revision: number;
  observedAt: string;
}

export interface CancelPresentation {
  /** 展示用执行状态（专用值，不坍缩三态） */
  executionStatus: string;
  /** 中文标签（文字+语义双通道） */
  label: string;
  /** 客户端从不请求退款 */
  refundRequested: boolean;
  /** 取消按钮可用性（无 revision 即禁用） */
  canRequestCancel: boolean;
  stopObservation: StopObservation;
}

/** 申请取消 ACK 后的保守展示：等待服务端真实确认（run_status 仍是原值）。 */
export function presentCancelAcknowledged(observation: ExecutionObservation): CancelPresentation {
  return {
    executionStatus: 'stop_pending',
    label: '停止待确认',
    refundRequested: false,
    canRequestCancel: false,
    stopObservation: 'stop_pending',
  };
}

/** 常规展示：三态原值+观察时间；unknown 不改写为 failed/succeeded。 */
export function presentExecution(observation: ExecutionObservation): CancelPresentation {
  const terminal = ['succeeded', 'failed', 'canceled'].includes(observation.runStatus);
  return {
    executionStatus: observation.executionStatus === '' ? 'unknown_lookup' : observation.executionStatus,
    label: terminal ? `任务已${observation.runStatus === 'succeeded' ? '完成' : observation.runStatus === 'failed' ? '失败' : '取消'}` : observation.executionStatus || '观察中',
    refundRequested: false,
    canRequestCancel: observation.revision > 0 && !terminal,
    stopObservation: 'idle',
  };
}

/**
 * 取消请求守卫：无 revision（无快照）拒绝发送 command——返回 false 并给出原因。
 * ACK 只代表请求已记录；run_status 的真实变化以事件流为准。
 */
export function shouldRequestCancel(observation: ExecutionObservation): { allowed: boolean; reason?: 'no_revision' | 'terminal' | 'already_pending' } {
  if (observation.revision <= 0) return { allowed: false, reason: 'no_revision' };
  if (['succeeded', 'failed', 'canceled'].includes(observation.runStatus)) return { allowed: false, reason: 'terminal' };
  if (observation.executionStatus === 'stop_pending') return { allowed: false, reason: 'already_pending' };
  return { allowed: true };
}
