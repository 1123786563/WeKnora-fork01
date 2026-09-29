import { useEffect, useState } from 'react';
import { Button, ScrollView, Text, TextInput, View } from 'react-native';
import type { DeliveryReceiptView, DeliveryState, InterventionReceipt, StopPhase, TaskDetailView, TaskIntent } from '@weknora/mobile-core';
import { timelineKindLabel } from '@weknora/mobile-core';

export interface TaskDetailScreenProps {
  view?: TaskDetailView;
  loading: boolean;
  error?: string;
  onRefresh(): void;
  onOpenMaterials?: () => void;
  /** T07：受控干预入口；未提供时整个干预区不渲染（通道缺失 fail closed 的呈现面）。 */
  onAct?: (intent: TaskIntent) => Promise<InterventionReceipt>;
  onOpenBudget?: () => void;
  /** T27 语音房入口；未提供时不渲染（组合根未装配或无捕获能力的呈现面）。 */
  onOpenVoiceRoom?: () => void;
  delivery?: DeliveryReceiptView;
  /** T25 恢复入口；未提供时操作不渲染（通道缺失 fail closed 的呈现面）。 */
  onRecoverDelivery?: (input: { runId: string; deliveryId: string }) => Promise<DeliveryReceiptView>;
  recoveryError?: string;
}

const CONNECTION_LABELS: Record<TaskDetailView['connection'], string> = { syncing: '同步中', live: '已连接', interrupted: '连接中断，可恢复', drained: '已同步' };
const LIFECYCLE_LABELS: Record<TaskDetailView['lifecycle'], string> = { active: '进行中', completed: '已完成', canceled: '已取消', archived: '已归档' };
/** interruption 原因 → 用户文案（B2-F40：不得直出内部码；Task 10 会为 'stream-unavailable' 追加条目）。 */
const INTERRUPTION_COPY: Record<string, string> = { gap: '事件流出现缺口', 'cursor-expired': '同步游标过期', 'stream-error': '实时通道中断', 'stream-ended-nonterminal': '事件流提前结束', 'persist-failed': '本地保存失败', 'stream-unavailable': '此部署暂无实时通道，可手动刷新', offline: '当前离线：展示最近一次同步的加密缓存内容' };
/** 停止三态卡文案（Issue #37 AC1）：requested=202 已受理未观察终态；confirmed=已观察到取消；unknown=投递结果未知。 */
const STOP_PHASE_COPY: Record<StopPhase, string> = {
  requested: '停止请求已发出，等待运行确认停止',
  confirmed: '停止已确认：运行已取消',
  unknown: '停止结果未知：正在与服务端核对，核对完成前不能下达新指令',
};
/** 停止卡最终文案：「停止已确认：运行已取消」只有在模块未附加 note 时才可断言——
 * stopProjection 在自然终态获胜（run ended as X before the stop landed）时返回
 * confirmed+note，那次取消并未发生（Spec Story 23：停止不得隐藏后果）。带 note 的
 * confirmed 用中性陈述 + 模块给的事实，绝不硬编码一次未发生的取消。 */
function stopCardCopy(stop: NonNullable<TaskDetailView['stop']>): string {
  if (stop.phase === 'confirmed' && stop.note !== undefined) {
    return `停止流程已结束：${stop.note}`;
  }
  return STOP_PHASE_COPY[stop.phase];
};
const OUTCOME_COPY: Record<InterventionReceipt['outcome'], string> = {
  accepted: '已受理', parked: '已排队（等待当前 Run 结束后发出）', conflict: '状态冲突，请刷新后重试', unknown: '结果未知，核对中',
};

/** 交付六态的如实中文文案：不粉饰部分完成（pushed）与不可观测（unknown）。
 * PR/MR 中性措辞：GitLab 草稿 MR 与 GitHub 草稿 PR 经同一统一回执呈现（T24 #54）。 */
export const DELIVERY_STATE_COPY: Record<DeliveryState, string> = {
  prepared: '待审批：审阅 Diff 与候选提交后在行动收件箱批准',
  dispatched: '交付进行中：正在推送任务分支',
  pushed: '已推送，等待草稿 PR/MR 恢复',
  delivered: '草稿 PR/MR 已创建',
  failed: '交付失败',
  unknown: '远端结果待确认',
};

/** T25 (#55) 恢复操作文案：按状态给恰一个动作，失败态错误不粉饰。 */
export const DELIVERY_RECOVERY_COPY = {
  pushed: '恢复创建草稿 PR/MR',
  unknown: '核对远端结果',
  failed: '当前状态无法恢复：请刷新后查看最新交付状态',
} as const;

/** 交付回执区块：只读呈现服务端落账的追溯字段（仓库/分支/提交/PR·MR/远端身份/批准人）。 */
function DeliveryReceiptSection({ delivery, onRecover, recoveryError }: { delivery: DeliveryReceiptView; onRecover?: () => void; recoveryError?: string }) {
  return (
    <View style={{ marginTop: 16, padding: 12, borderWidth: 1, borderColor: '#ccc', borderRadius: 8 }}>
      <Text style={{ fontWeight: '600' }}>代码交付</Text>
      <Text>{DELIVERY_STATE_COPY[delivery.state]}</Text>
      {delivery.state === 'pushed' && onRecover !== undefined ? <Button title={DELIVERY_RECOVERY_COPY.pushed} onPress={onRecover} /> : null}
      {delivery.state === 'unknown' && onRecover !== undefined ? <Button title={DELIVERY_RECOVERY_COPY.unknown} onPress={onRecover} /> : null}
      {delivery.state === 'failed' ? <Text>{DELIVERY_RECOVERY_COPY.failed}</Text> : null}
      {recoveryError !== undefined ? <Text>{recoveryError}</Text> : null}
      <Text numberOfLines={1}>仓库：{delivery.repo}</Text>
      <Text numberOfLines={1}>分支：{delivery.branch}</Text>
      {delivery.commitSha !== undefined ? <Text numberOfLines={1}>提交：{delivery.commitSha.slice(0, 12)}</Text> : null}
      {delivery.prUrl !== undefined ? <Text numberOfLines={1}>PR/MR：{delivery.prUrl}</Text> : null}
      {delivery.remoteLogin !== undefined ? <Text numberOfLines={1}>远端身份：{delivery.remoteLogin}</Text> : null}
      {delivery.approver !== undefined ? <Text numberOfLines={1}>批准人：{delivery.approver}</Text> : null}
    </View>
  );
}

/** 结果优先详情屏：状态卡 + 三层状态 + attention 横幅在前，干预区随后，时间线事实流在后；原始证据默认折叠、按需展开；预算入口与交付回执在底部操作区。 */
export function TaskDetailScreen({ view, loading, error, onRefresh, onOpenMaterials, onOpenBudget, onOpenVoiceRoom, onAct, delivery, onRecoverDelivery, recoveryError }: TaskDetailScreenProps) {
  const [expanded, setExpanded] = useState<ReadonlySet<number>>(new Set());
  const [draft, setDraft] = useState('');
  // B2-F41：expanded 以 runId 隔离——切换任务（组件复用）时不携带上一个任务的展开状态。
  const runKey = view?.runId ?? '';
  useEffect(() => { setExpanded(new Set()); }, [runKey]);
  if (view === undefined) {
    return (
      <View>
        <Text>{loading ? '正在读取服务端快照…' : '无法读取该任务'}</Text>
        {error !== undefined && <Text>{error}</Text>}
        {/* B2-F38：controller.refresh() 支持从无 view 状态恢复，瞬时故障后必须留就地重试入口 */}
        <Button title="重试" onPress={onRefresh} />
      </View>
    );
  }
  const toggle = (seq: number): void => {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(seq)) next.delete(seq);
      else next.add(seq);
      return next;
    });
  };
  // 干预发出：受理即清空草稿；失败保留草稿（回执区/错误文案呈现冲突与未知，不静默吞掉）。
  const run = (intent: TaskIntent): void => {
    if (onAct === undefined) return;
    void onAct(intent).then(() => setDraft(''), () => undefined);
  };
  return (
    <ScrollView>
      <View>
        <Text>{view.title === '' ? view.taskId : view.title}</Text>
        <Text>{CONNECTION_LABELS[view.connection]}{view.interruption !== undefined ? ` · ${INTERRUPTION_COPY[view.interruption.reason] ?? view.interruption.reason}` : ''}</Text>
        <View>
          <Text>任务：{LIFECYCLE_LABELS[view.lifecycle]}</Text>
          <Text>运行：{view.runStatus}</Text>
          <Text>关注：{view.attention === 'required' ? '需要你处理' : '无需处理'}</Text>
          <Text>执行：{view.executionStatus} · 结算：{view.settlementStatus} · 已接收事件：{view.cursor}</Text>
        </View>
        {view.attention === 'required' && <Text>任务需要你的确认，请查看时间线中的审批条目。</Text>}
      </View>
      {view.stop !== undefined && <Text>{stopCardCopy(view.stop)}</Text>}
      {onAct !== undefined && (
        <View>
          <Text>运行干预</Text>
          <TextInput value={draft} onChangeText={setDraft} placeholder="调整或排队下一 Run 的指令" />
          <Button title="调整当前运行" onPress={() => run({ kind: 'steer', text: draft })} disabled={draft.trim() === ''} />
          <Button title="排队下一 Run" onPress={() => run({ kind: 'queue-next', text: draft })} disabled={draft.trim() === ''} />
          <Button title="停止运行" onPress={() => run({ kind: 'stop' })} />
        </View>
      )}
      {(view.interventions ?? []).slice(-5).reverse().map((receipt, index) => (
        <View key={`${receipt.at}-${index}`}>
          <Text>{OUTCOME_COPY[receipt.outcome]} · 绑定 Run {receipt.boundRunId}{receipt.nextRunId === undefined ? '' : ` · 下一 Run ${receipt.nextRunId}`}</Text>
          {(receipt.intent.kind === 'steer' || receipt.intent.kind === 'queue-next') && <Text numberOfLines={2}>指令：{receipt.intent.text}</Text>}
        </View>
      ))}
      {(view.queuedNext ?? []).length > 0 && <Text>已排队待发：{view.queuedNext!.map((q) => q.text).join('；')}</Text>}
      <Text>任务时间线</Text>
      {view.timeline.map((entry) => (
        <View key={entry.seq}>
          <Text>{entry.summary}</Text>
          <Text>{timelineKindLabel(entry.kind)} · {formatTime(entry.occurredAt)} · #{entry.seq}</Text>
          <Button title={expanded.has(entry.seq) ? '收起证据' : '展开证据'} onPress={() => toggle(entry.seq)} />
          {expanded.has(entry.seq) && (
            <Text>{Object.entries(entry.evidence.payload).map(([key, value]) => `${key}: ${safeText(value)}`).join('\n')}</Text>
          )}
        </View>
      ))}
      {view.duplicateSeqs.length > 0 && <Text>已忽略重复事件：{view.duplicateSeqs.join(', ')}</Text>}
      {onOpenMaterials !== undefined && <Button title="任务材料" onPress={onOpenMaterials} />}
      {onOpenBudget !== undefined && <Button title="任务预算" onPress={onOpenBudget} />}
      {onOpenVoiceRoom !== undefined && <Button title="语音房" onPress={onOpenVoiceRoom} />}
      {delivery !== undefined ? <DeliveryReceiptSection delivery={delivery} recoveryError={recoveryError} onRecover={onRecoverDelivery === undefined ? undefined : () => { void onRecoverDelivery({ runId: view.runId, deliveryId: delivery.deliveryId }).catch(() => undefined); }} /> : null}
      <Button title="重新同步快照" onPress={onRefresh} disabled={loading} />
      {error !== undefined && <Text>{error}</Text>}
    </ScrollView>
  );
}

function formatTime(value: string): string {
  return value.replace('T', ' ').replace('Z', ' UTC');
}

/** 原始证据只按文本呈现键值；不执行、不猜测结构。 */
function safeText(value: unknown): string {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || value === null) return String(value);
  try { return JSON.stringify(value) ?? String(value); } catch { return '[unserializable]'; }
}
