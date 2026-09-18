// MX-018 probe · 取消≠停止观察器
// frozen 场景：cancel-ack-no-process-exit。
// 真实 execution-presentation：取消 ACK 后展示 stop_pending（停止待确认），
// 不改写 run 原值、不显示退款；无 revision 禁发命令。
import {
  presentCancelAcknowledged,
  presentExecution,
  shouldRequestCancel,
} from '../../../packages/domain/src/mobile/execution-presentation.ts';

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  executionStatus: string;
  label: string;
  refundRequested: boolean;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'cancel-ack-no-process-exit') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  const observation = {
    runStatus: 'running',
    executionStatus: 'executing',
    settlementStatus: 'pending',
    revision: 4,
    observedAt: '2026-09-18T08:00:00Z',
  };

  // 取消前置守卫：有 revision、非终态、未在等待 → 允许申请
  const guard = shouldRequestCancel(observation);
  if (!guard.allowed) throw new Error(`precondition: cancel must be requestable, got ${guard.reason}`);

  // ACK：服务端已记录取消请求，但进程未退出（run 仍 running）
  const acked = presentCancelAcknowledged(observation);
  if (acked.stopObservation !== 'stop_pending') throw new Error('ack must present stop_pending');
  // run 原值不被 ACK 改写（presentation 之外观察字段由调用方持有；此处验证执行展示不坍缩）
  if (acked.executionStatus !== 'stop_pending' || acked.label !== '停止待确认') {
    throw new Error(`unexpected ack presentation: ${JSON.stringify(acked)}`);
  }
  // 已 pending 后再次申请被拒
  const pendingGuard = shouldRequestCancel({ ...observation, executionStatus: 'stop_pending' });
  if (pendingGuard.allowed || pendingGuard.reason !== 'already_pending') {
    throw new Error('repeated cancel while pending must be rejected');
  }
  // 无 revision（无快照）禁发
  const noRevision = shouldRequestCancel({ ...observation, revision: 0 });
  if (noRevision.allowed || noRevision.reason !== 'no_revision') throw new Error('cancel without revision must be rejected');
  // 终态禁发
  const terminal = shouldRequestCancel({ ...observation, runStatus: 'succeeded' });
  if (terminal.allowed || terminal.reason !== 'terminal') throw new Error('cancel on terminal run must be rejected');

  // unknown 不显示为失败/完成
  const unknown = presentExecution({ ...observation, executionStatus: '', runStatus: 'reconciling' });
  if (unknown.executionStatus !== 'unknown_lookup') throw new Error('empty execution status must show unknown_lookup, not failed/succeeded');

  // 退款语义：任何展示都不含退款
  const base = presentExecution(observation);
  if (base.refundRequested || acked.refundRequested || unknown.refundRequested) {
    throw new Error('refund must never be requested or implied');
  }

  return {
    executionStatus: acked.executionStatus,
    label: acked.label,
    refundRequested: acked.refundRequested,
  };
}
