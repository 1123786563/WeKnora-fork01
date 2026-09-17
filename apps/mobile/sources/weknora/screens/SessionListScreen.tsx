import React, { useEffect, useMemo, useRef, useState } from 'react';
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native';
import type { SessionListController, SessionFilter } from '@weknora/domain/mobile';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { StatusBadge, type BadgeTone } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import { Field } from '../ui/Field.tsx';

/**
 * 会话列表页 M04（MX-014）：分页（稳定 id 键）/搜索（去抖读）/筛选（进查询身份）。
 * 控制器为领域层（session-list.ts）；本屏只做视图与令牌视觉。
 */
const FILTERS: ReadonlyArray<{ id: SessionFilter; label: string }> = [
  { id: 'all', label: '全部' },
  { id: 'running', label: '进行中' },
  { id: 'waiting_user', label: '待处理' },
  { id: 'terminal', label: '已结束' },
];

const STATUS_TONE: Record<string, BadgeTone> = {
  running: 'brand', waiting_user: 'warning', queued: 'neutral', reconciling: 'info',
  succeeded: 'neutral', failed: 'danger', canceled: 'neutral',
};

export interface SessionListScreenProps {
  controller: SessionListController;
  onOpenSession?: (runId: string) => void;
  onNewTask?: () => void;
  testID?: string;
}

export function SessionListScreen({ controller, onOpenSession, onNewTask, testID }: SessionListScreenProps) {
  const { theme } = useWeknoraTheme();
  const [, force] = useState(0);
  const [search, setSearch] = useState('');
  const mounted = useRef(true);
  useEffect(() => () => { mounted.current = false; }, []);

  // 控制器状态变化驱动重绘（领域层不可变 state，指针比较即可）
  const statePtr = useRef(controller.state);
  useEffect(() => {
    const timer = setInterval(() => {
      if (statePtr.current !== controller.state) {
        statePtr.current = controller.state;
        if (mounted.current) force((n) => n + 1);
      }
    }, 50);
    return () => clearInterval(timer);
  }, [controller]);
  const state = controller.state;

  const items = useMemo(() => state.items, [state]);

  return (
    <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}>
      <View style={{ padding: theme.spacing[16], gap: theme.spacing[12] }}>
        <Field label="搜索" value={search} onChangeText={(text) => { setSearch(text); controller.setSearch(text); }} placeholder="搜索任务标题" />
        <View style={styles.filters} accessibilityRole="tablist">
          {FILTERS.map((filter) => {
            const selected = state.filter === filter.id;
            return (
              <Pressable
                key={filter.id}
                accessibilityRole="tab"
                accessibilityLabel={filter.label}
                accessibilityState={{ selected }}
                onPress={() => void controller.setFilter(filter.id)}
                style={[styles.chip, { borderColor: selected ? theme.colors.brand : theme.colors['control-line'], backgroundColor: selected ? theme.colors['brand-soft'] : theme.colors.surface, borderRadius: theme.radius.pill, paddingHorizontal: theme.spacing[12], paddingVertical: theme.spacing[6] }]}
              >
                <Text style={{ color: selected ? theme.colors.brand : theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, fontWeight: selected ? '600' : '400' }}>{filter.label}</Text>
              </Pressable>
            );
          })}
        </View>
      </View>
      {state.loading && items.length === 0 ? (
        <StateView kind="loading" message="正在加载会话列表" />
      ) : !state.loading && items.length === 0 ? (
        <StateView
          kind="empty"
          message={state.search || state.filter !== 'all' ? '没有匹配的会话' : '还没有会话'}
          detail={state.search || state.filter !== 'all' ? '调整搜索或筛选条件' : '发起第一个任务后在这里查看'}
          actionLabel="新建任务"
          onAction={onNewTask}
        />
      ) : (
        <FlatList
          data={items}
          keyExtractor={(item) => item.runId}
          onEndReached={() => void controller.loadNextPage()}
          onEndReachedThreshold={0.4}
          contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[12] }}
          renderItem={({ item }) => (
            <Card onPress={onOpenSession ? () => onOpenSession(item.runId) : undefined} accessibilityLabel={`打开会话 ${item.title || item.runId}，状态 ${item.runStatus}`} compact>
              <View style={styles.row}>
                <View style={styles.grow}>
                  <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>{item.title || `任务 ${item.runId}`}</Text>
                  <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>{item.updatedAt}</Text>
                </View>
                <StatusBadge tone={STATUS_TONE[item.runStatus] ?? 'neutral'} label={item.runStatus} />
              </View>
            </Card>
          )}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  filters: { flexDirection: 'row', gap: 8, flexWrap: 'wrap' },
  chip: { borderWidth: StyleSheet.hairlineWidth },
  row: { flexDirection: 'row', alignItems: 'center' },
  grow: { flex: 1 },
});
