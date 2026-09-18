import React, { useMemo, useState } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { Sheet } from '../ui/Sheet.tsx';
import { StatusBadge, type BadgeTone } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import {
  presentCancelAcknowledged,
  presentExecution,
  shouldRequestCancel,
  type ExecutionObservation,
} from '@weknora/domain/mobile';

/**
 * 执行详情页 M08（MX-018）：三态原值+观察时间；保守取消（确认 Sheet→ACK→stop_pending）；
 * unknown 专用查询；无 revision 禁发命令；结算 pending 可见且不显示自动退款。
 */
export interface ExecutionScreenProps {
  observation: ExecutionObservation;
  /** 取消命令端口（真实 command API；ACK 只表示请求记录） */
  onCancelRequest?: (expectedRevision: number) => Promise<void>;
  onViewArtifacts?: () => void;
  testID?: string;
}

const STATUS_TONE: Record<string, BadgeTone> = {
  running: 'brand', waiting_user: 'warning', queued: 'neutral', reconciling: 'info',
  succeeded: 'neutral', failed: 'danger', canceled: 'neutral',
};

export function ExecutionScreen({ observation, onCancelRequest, onViewArtifacts, testID }: ExecutionScreenProps) {
  const { theme } = useWeknoraTheme();
  const [pendingAck, setPendingAck] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const base = useMemo(() => presentExecution(observation), [observation]);
  const presentation = pendingAck ? presentCancelAcknowledged(observation) : base;
  const cancelGuard = useMemo(() => shouldRequestCancel(pendingAck ? { ...observation, executionStatus: 'stop_pending' } : observation), [observation, pendingAck]);
  const terminal = ['succeeded', 'failed', 'canceled'].includes(observation.runStatus);

  const confirmCancel = async () => {
    setConfirming(false);
    if (!onCancelRequest || !cancelGuard.allowed) return;
    await onCancelRequest(observation.revision);
    // ACK：申请已记录——等待服务端真实确认（不假装已停止）
    setPendingAck(true);
  };

  return (
    <ScrollView testID={testID} style={{ backgroundColor: theme.colors.bg }} contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[16] }}>
      <Card>
        <View style={styles.row}>
          <StatusBadge tone={STATUS_TONE[observation.runStatus] ?? 'neutral'} label={observation.runStatus} />
          <Text accessibilityLabel={`执行状态 ${presentation.executionStatus}，标签 ${presentation.label}，结算 ${observation.settlementStatus}，观察时间 ${observation.observedAt}`} style={{ color: theme.colors.muted, flex: 1, marginLeft: theme.spacing[8], fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
            {presentation.label} · 观察于 {observation.observedAt}
          </Text>
        </View>
      </Card>

      <Card>
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>状态详情</Text>
        {([
          ['任务状态 (run)', observation.runStatus],
          ['执行观察 (execution)', presentation.executionStatus],
          ['结算状态 (settlement)', observation.settlementStatus === 'pending' ? 'pending（未最终）' : observation.settlementStatus],
          ['修订号 (revision)', String(observation.revision)],
        ] as const).map(([label, value]) => (
          <View key={label} style={[styles.row, { marginTop: theme.spacing[8] }]}>
            <Text style={{ color: theme.colors.muted, flex: 1, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>{label}</Text>
            <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, fontWeight: '600' }}>{value}</Text>
          </View>
        ))}
        <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[12] }}>
          取消为申请语义：批准记录不等于执行已停止，也不产生退款。
        </Text>
      </Card>

      {terminal ? (
        <Card>
          <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
            任务已结束，结果可查看；结算完成以服务端为准。
          </Text>
          {onViewArtifacts ? <View style={{ marginTop: theme.spacing[12] }}><Button label="查看任务成果" variant="secondary" onPress={onViewArtifacts} accessibilityLabel="查看任务成果" size="compact" /></View> : null}
        </Card>
      ) : (
        <Button
          label="申请取消任务"
          variant="danger"
          disabled={!cancelGuard.allowed || !onCancelRequest}
          onPress={() => setConfirming(true)}
          accessibilityLabel={cancelGuard.allowed ? '申请取消该任务（需确认；停止需服务端确认）' : `无法申请取消：${cancelGuard.reason === 'no_revision' ? '尚无快照修订号' : cancelGuard.reason === 'already_pending' ? '已有停止请求待确认' : '任务已结束'}`}
        />
      )}

      {pendingAck ? (
        <StateView kind="empty" message="停止请求已提交，等待服务端确认" detail="执行停止以事件流为准；不产生自动退款" />
      ) : null}

      <Sheet
        visible={confirming}
        title="确认申请取消"
        onClose={() => setConfirming(false)}
        footer={
          <View style={{ gap: theme.spacing[8] }}>
            <Button label="确认申请取消" variant="danger" accessibilityLabel="确认申请取消该任务（不是停止，也不是退款）" onPress={() => void confirmCancel()} />
            <Button label="返回" variant="secondary" accessibilityLabel="返回不取消" onPress={() => setConfirming(false)} size="compact" />
          </View>
        }
      >
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
          将以修订号 {observation.revision} 发送取消请求。这是「申请取消」：批准记录不代表执行已停止，也不会退款。
        </Text>
      </Sheet>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
