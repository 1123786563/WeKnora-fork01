import { useEffect, useState } from 'react';
import { Button, ScrollView, Text, View } from 'react-native';
import type { DeliveryReceiptView, DeliveryState, TaskDetailView } from '@weknora/mobile-core';
import { timelineKindLabel } from '@weknora/mobile-core';

export interface TaskDetailScreenProps {
  view?: TaskDetailView;
  loading: boolean;
  error?: string;
  onRefresh(): void;
  onOpenMaterials?: () => void;
  delivery?: DeliveryReceiptView;
}

const CONNECTION_LABELS: Record<TaskDetailView['connection'], string> = { syncing: '同步中', live: '已连接', interrupted: '连接中断，可恢复', drained: '已同步' };
const LIFECYCLE_LABELS: Record<TaskDetailView['lifecycle'], string> = { active: '进行中', completed: '已完成', canceled: '已取消', archived: '已归档' };
/** interruption 原因 → 用户文案（B2-F40：不得直出内部码；Task 10 会为 'stream-unavailable' 追加条目）。 */
const INTERRUPTION_COPY: Record<string, string> = { gap: '事件流出现缺口', 'cursor-expired': '同步游标过期', 'stream-error': '实时通道中断', 'stream-ended-nonterminal': '事件流提前结束', 'persist-failed': '本地保存失败', 'stream-unavailable': '此部署暂无实时通道，可手动刷新' };

/** 交付六态的如实中文文案：不粉饰部分完成（pushed）与不可观测（unknown）。 */
export const DELIVERY_STATE_COPY: Record<DeliveryState, string> = {
  prepared: '待审批：审阅 Diff 与候选提交后在行动收件箱批准',
  dispatched: '交付进行中：正在推送任务分支',
  pushed: '已推送，等待草稿 PR 恢复',
  delivered: '草稿 PR 已创建',
  failed: '交付失败',
  unknown: '远端结果待确认',
};

/** 交付回执区块：只读呈现服务端落账的追溯字段（仓库/分支/提交/PR/远端身份/批准人）。 */
function DeliveryReceiptSection({ delivery }: { delivery: DeliveryReceiptView }) {
  return (
    <View style={{ marginTop: 16, padding: 12, borderWidth: 1, borderColor: '#ccc', borderRadius: 8 }}>
      <Text style={{ fontWeight: '600' }}>代码交付</Text>
      <Text>{DELIVERY_STATE_COPY[delivery.state]}</Text>
      <Text numberOfLines={1}>仓库：{delivery.repo}</Text>
      <Text numberOfLines={1}>分支：{delivery.branch}</Text>
      {delivery.commitSha !== undefined ? <Text numberOfLines={1}>提交：{delivery.commitSha.slice(0, 12)}</Text> : null}
      {delivery.prUrl !== undefined ? <Text numberOfLines={1}>PR：{delivery.prUrl}</Text> : null}
      {delivery.remoteLogin !== undefined ? <Text numberOfLines={1}>远端身份：{delivery.remoteLogin}</Text> : null}
      {delivery.approver !== undefined ? <Text numberOfLines={1}>批准人：{delivery.approver}</Text> : null}
    </View>
  );
}

/** 结果优先详情屏：状态卡 + 三层状态 + attention 横幅在前，时间线事实流在后；原始证据默认折叠、按需展开。 */
export function TaskDetailScreen({ view, loading, error, onRefresh, onOpenMaterials, delivery }: TaskDetailScreenProps) {
  const [expanded, setExpanded] = useState<ReadonlySet<number>>(new Set());
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
      {delivery !== undefined ? <DeliveryReceiptSection delivery={delivery} /> : null}
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
