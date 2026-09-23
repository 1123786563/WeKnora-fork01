import { useState } from 'react';
import { Button, ScrollView, Text, View } from 'react-native';
import type { TaskDetailView } from '@weknora/mobile-core';
import { timelineKindLabel } from '@weknora/mobile-core';

export interface TaskDetailScreenProps {
  view?: TaskDetailView;
  loading: boolean;
  error?: string;
  onRefresh(): void;
}

const CONNECTION_LABELS: Record<TaskDetailView['connection'], string> = { syncing: '同步中', live: '已连接', interrupted: '连接中断，可恢复', drained: '已同步' };
const LIFECYCLE_LABELS: Record<TaskDetailView['lifecycle'], string> = { active: '进行中', completed: '已完成', canceled: '已取消', archived: '已归档' };

/** 结果优先详情屏：状态卡 + 三层状态 + attention 横幅在前，时间线事实流在后；原始证据默认折叠、按需展开。 */
export function TaskDetailScreen({ view, loading, error, onRefresh }: TaskDetailScreenProps) {
  const [expanded, setExpanded] = useState<ReadonlySet<number>>(new Set());
  if (view === undefined) {
    return (
      <View>
        <Text>{loading ? '正在读取服务端快照…' : '无法读取该任务'}</Text>
        {error !== undefined && <Text>{error}</Text>}
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
        <Text>{CONNECTION_LABELS[view.connection]}{view.interruption !== undefined ? ` · ${view.interruption.reason}` : ''}</Text>
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
      <Button title="重新同步快照" onPress={onRefresh} />
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
