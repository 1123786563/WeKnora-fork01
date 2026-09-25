import type { AttentionDecisionReceipt } from '@weknora/mobile-core';
import type { AttentionInboxItem as InboxItem } from '@weknora/mobile-core';
import type { TaskConnectionState, TaskInterruptionReason } from '@weknora/mobile-core';
import type { MaterialEntry } from '@weknora/mobile-core';
import { formatBytes, formatTime } from '../core/format.ts';

export const runStatusLabels: Record<string, string> = {
  queued: '排队中', running: '运行中', waiting_user: '待你确认', reconciling: '核对状态中',
  succeeded: '已完成', failed: '失败', canceled: '已取消',
};

export function runStatusBadgeTone(runStatus: string): 'success' | 'warning' | 'info' | 'neutral' {
  if (runStatus === 'succeeded') return 'success';
  if (runStatus === 'waiting_user' || runStatus === 'failed') return 'warning';
  if (runStatus === 'running' || runStatus === 'reconciling' || runStatus === 'queued') return 'info';
  return 'neutral';
}

export function connectionLabel(connection: TaskConnectionState): string {
  return { syncing: '同步中', live: '已连接', interrupted: '连接中断，可恢复', drained: '已同步' }[connection] ?? connection;
}

export function interruptionNotice(reason: TaskInterruptionReason): string {
  const copy: Record<TaskInterruptionReason, string> = {
    'gap': '事件出现缺口，正在自动重新同步；不会重放已完成的工作。',
    'cursor-expired': '服务端游标已过期裁剪，正在从快照重新同步。',
    'stream-error': '连接中断；页面保留已同步状态，可手动重新同步。',
    'stream-ended-nonterminal': '连接在非终态结束，正在核对最新状态。',
    'persist-failed': '本机缓存写入失败；已提交的服务端状态不受影响。',
    'stream-unavailable': '当前部署未提供流式通道；只能整段刷新快照。',
  };
  return copy[reason];
}

/** 收件箱过滤：源是跨 run 的 inbox()；按 run 进入时仅做视图过滤，不改权威列表。 */
export function inboxVisibleItems(items: InboxItem[], runId?: string): InboxItem[] {
  return runId ? items.filter(item => item.runId === runId) : items;
}

/** 四态回执如实文案：recorded 只表示决定已记录，绝不解释为外部派发完成（#38 AC2）。 */
export function decisionReceiptText(receipt: AttentionDecisionReceipt): string {
  switch (receipt.status) {
    case 'recorded': return '决定已记录。外部动作是否已派发完成以执行状态为准。';
    case 'delivery-unknown': return '结果不确定：服务端未确认该决定是否送达，请稍后重查，不要盲目重复提交。';
    case 'superseded': return '该事项已被取代（存在更新的决定）。';
    case 'gone': return '该事项已失效（不存在或已过期）。';
  }
}

export function materialEntryRow(entry: MaterialEntry): { title: string; subtitle: string } {
  const kindLabel = entry.kind === 'test-report' ? '测试报告' : entry.kind === 'diff' ? 'Diff' : '产物';
  return { title: entry.name || `${kindLabel} #${entry.index}`, subtitle: `${kindLabel} · ${entry.mime} · ${formatBytes(entry.size)} · 版本 ${entry.version.slice(0, 8)}` };
}
