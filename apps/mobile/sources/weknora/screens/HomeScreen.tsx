import React, { useCallback, useEffect, useState } from 'react';
import { RefreshControl, ScrollView, StyleSheet, Text, View } from 'react-native';
import type { WorkbenchOverview } from '@weknora/contracts';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { StatusBadge, type BadgeTone } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import { productQueryKey } from '@weknora/domain/mobile';

/**
 * 工作台首页 M03（MX-013）。数据一次聚合加载（GET /workbench/overview，零逐会话补读）；
 * 加载/空/失败/离线态齐备；as_of（上次同步）可见；进行中/待审批真实入口待路由任务接线。
 */
export interface HomeScreenProps {
  loader: { overview: () => Promise<WorkbenchOverview> };
  identity: { origin: string; userId: string | null; tenantId: string | null };
  offline: () => boolean;
  onOpenRun?: (runId: string) => void;
  onOpenApprovals?: () => void;
  testID?: string;
}

const RUN_STATUS_TONE: Record<string, BadgeTone> = {
  queued: 'neutral', running: 'brand', waiting_user: 'warning', reconciling: 'info',
};

export function HomeScreen({ loader, identity, offline, onOpenRun, onOpenApprovals, testID }: HomeScreenProps) {
  const { theme } = useWeknoraTheme();
  const [state, setState] = useState<{ kind: 'loading' } | { kind: 'ready'; overview: WorkbenchOverview } | { kind: 'error'; message: string }>({ kind: 'loading' });
  // 查询键按规范化 scope 隔离（MX-011）：空间切换后旧数据不可见
  const queryKey = productQueryKey(identity, 'overview').join('/');

  const load = useCallback(async () => {
    setState({ kind: 'loading' });
    try {
      const overview = await loader.overview();
      setState({ kind: 'ready', overview });
    } catch (error) {
      setState({ kind: 'error', message: error instanceof Error ? error.message : 'overview failed' });
    }
  }, [loader]);

  useEffect(() => {
    void load();
  }, [load, queryKey]);

  return (
    <ScrollView
      testID={testID}
      style={[styles.container, { backgroundColor: theme.colors.bg }]}
      contentContainerStyle={{ padding: theme.spacing[20], gap: theme.spacing[16] }}
      refreshControl={<RefreshControl refreshing={state.kind === 'loading'} onRefresh={() => void load()} tintColor={theme.colors.brand} />}
    >
      {state.kind === 'loading' ? (
        <StateView kind="loading" message="正在加载工作台" />
      ) : state.kind === 'error' ? (
        offline() ? (
          <StateView kind="offline" message="当前离线" detail="恢复网络后下拉刷新" actionLabel="重试" onAction={() => void load()} />
        ) : (
          <StateView kind="error" message="工作台加载失败" detail={state.message} actionLabel="重试" onAction={() => void load()} />
        )
      ) : (
        <>
          <Card tone="hero">
            <Text style={{ color: theme.colors['hero-ink'], fontSize: theme.typography.title.fontSize, lineHeight: theme.typography.title.lineHeight, fontWeight: '600' }}>工作台</Text>
            <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
              {state.overview.counts.active_runs > 0 ? `${state.overview.counts.active_runs} 个任务进行中` : '当前没有进行中的任务'}
            </Text>
            <Text accessibilityLabel={`数据更新于 ${state.overview.as_of}`} style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
              上次同步 {state.overview.as_of}
            </Text>
          </Card>

          {state.overview.counts.pending_interactions > 0 ? (
            <Card onPress={onOpenApprovals} accessibilityLabel={`查看 ${state.overview.counts.pending_interactions} 项待审批`}>
              <View style={styles.row}>
                <StatusBadge tone="warning" label={`待审批 ${state.overview.counts.pending_interactions}`} />
                <Text style={{ color: theme.colors.muted, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, marginLeft: theme.spacing[8], flex: 1 }}>
                  有操作等待你的确认
                </Text>
              </View>
            </Card>
          ) : null}

          {state.overview.in_progress.length === 0 ? (
            <StateView kind="empty" message="还没有进行中的任务" detail="从会话页发起第一个任务" />
          ) : (
            state.overview.in_progress.map((run) => (
              <Card key={run.run_id} onPress={onOpenRun ? () => onOpenRun(run.run_id) : undefined} accessibilityLabel={`打开任务 ${run.title || run.run_id}，状态 ${run.run_status}`}>
                <View style={styles.row}>
                  <View style={styles.grow}>
                    <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>
                      {run.title || `任务 ${run.run_id}`}
                    </Text>
                    <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
                      更新于 {run.updated_at}
                    </Text>
                  </View>
                  <StatusBadge tone={RUN_STATUS_TONE[run.run_status] ?? 'neutral'} label={run.run_status} />
                </View>
              </Card>
            ))
          )}
        </>
      )}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  row: { flexDirection: 'row', alignItems: 'center' },
  grow: { flex: 1 },
});
