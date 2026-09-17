import React, { useCallback, useMemo, useState } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Button } from '../ui/Button.tsx';
import { Sheet } from '../ui/Sheet.tsx';
import { StateView } from '../ui/StateView.tsx';
import { ToolApproval } from '../interactions/ToolApproval.tsx';
import {
  createInteractionController,
  type InteractionSnapshot,
} from '../interactions/controller.ts';
import type { InteractionRecord } from '@weknora/contracts';
import { useWeknoraTheme as useTheme } from '../ui/theme.ts';

/**
 * 操作审批页 M09（MX-019）：打开确认前强制刷新详情（Sheet 捕获 revision/generation）；
 * 陈旧确认零发送导向重新核对；ACK 只表示已记录（不宣告外部写入成功）；409/过期关闭旧确认。
 */
export interface InteractionScreenProps {
  runID: string;
  interactionID: string;
  /** 真实 interactions API（list/decide，经 MX-005 SDK） */
  ports: { list(runID: string): Promise<InteractionRecord[]>; decide(input: InteractionRecord): Promise<InteractionRecord> };
  /** scope generation（切空间后旧确认失效） */
  generation: { accept(): boolean };
  targetSummary: { tool: string; account?: string; argumentDigest: string; risk: 'low' | 'medium' | 'high' };
  testID?: string;
}

export function InteractionScreen({ runID, interactionID, ports, generation, targetSummary, testID }: InteractionScreenProps) {
  const { theme } = useTheme();
  const controller = useMemo(() => createInteractionController({ ...ports, now: () => new Date().toISOString() }), [ports]);
  const [snapshot, setSnapshot] = useState<InteractionSnapshot | null>(null);
  const [phase, setPhase] = useState<'loading' | 'ready' | 'refresh-needed' | 'acknowledged' | 'error'>('loading');
  const [confirming, setConfirming] = useState<'approve' | 'reject' | null>(null);
  const [errorMessage, setErrorMessage] = useState('');

  const refresh = useCallback(async () => {
    setPhase('loading');
    try {
      const next = await controller.openConfirmation(runID, interactionID);
      if (!next) {
        setPhase('error');
        setErrorMessage('交互不存在或已不可访问（可能已过期/撤权）');
        return;
      }
      setSnapshot(next);
      setPhase(next.record.decision_id === '' ? 'ready' : 'acknowledged');
    } catch (error) {
      setPhase('error');
      setErrorMessage(error instanceof Error ? error.message : '刷新失败');
    }
  }, [controller, interactionID, runID]);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const confirm = useCallback(async (decision: 'approve' | 'reject') => {
    if (!snapshot) return;
    setConfirming(null);
    try {
      const latest = (await controller.openConfirmation(runID, interactionID))?.record;
      if (!latest) {
        setPhase('refresh-needed');
        return;
      }
      const result = await controller.confirm(snapshot, latest, decision, generation);
      if (result.action === 'refresh') {
        setPhase('refresh-needed');
        return;
      }
      setPhase('acknowledged');
      void refresh();
    } catch (error) {
      setPhase('error');
      setErrorMessage(error instanceof Error ? error.message : '提交失败，请重新核对后重试');
    }
  }, [controller, generation, refresh, runID, interactionID, snapshot]);

  return (
    <ScrollView testID={testID} style={{ backgroundColor: theme.colors.bg }} contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[16] }}>
      {phase === 'loading' ? <StateView kind="loading" message="正在刷新审批详情" /> : null}
      {phase === 'error' ? <StateView kind="error" message="无法获取审批详情" detail={errorMessage} actionLabel="重试" onAction={() => void refresh()} /> : null}
      {phase === 'refresh-needed' ? (
        <StateView kind="error" message="审批内容已更新" detail="详情在确认前发生变化（他人已处理或版本前进）——旧确认已关闭，请重新核对" actionLabel="重新核对" onAction={() => void refresh()} />
      ) : null}
      {phase === 'acknowledged' && snapshot ? (
        <StateView kind="empty" message={snapshot.record.decision_id !== '' ? '该交互已有决定（以服务端记录为准）' : '决定已记录'} detail="批准记录不代表外部操作已执行；执行状态以任务事件流为准" actionLabel="刷新详情" onAction={() => void refresh()} />
      ) : null}
      {snapshot && (phase === 'ready' || phase === 'acknowledged') ? (
        <>
          <ToolApproval record={snapshot.record} targetSummary={targetSummary} fetchedAt={snapshot.fetchedAt} />
          {phase === 'ready' ? (
            <View style={{ gap: theme.spacing[12] }}>
              <Button label="批准" onPress={() => setConfirming('approve')} accessibilityLabel={`批准工具 ${targetSummary.tool} 调用（修订 ${snapshot.record.expected_revision}，需二次确认）`} />
              <Button label="拒绝" variant="danger" onPress={() => setConfirming('reject')} accessibilityLabel={`拒绝工具 ${targetSummary.tool} 调用（需二次确认）`} />
            </View>
          ) : null}
        </>
      ) : null}

      <Sheet
        visible={confirming !== null}
        title={confirming === 'approve' ? '确认批准' : '确认拒绝'}
        onClose={() => setConfirming(null)}
        footer={
          <View style={{ gap: theme.spacing[8] }}>
            <Button
              label={confirming === 'approve' ? '确认批准' : '确认拒绝'}
              variant={confirming === 'reject' ? 'danger' : 'primary'}
              accessibilityLabel={`${confirming === 'approve' ? '确认批准' : '确认拒绝'}工具 ${targetSummary.tool}（修订 ${snapshot?.record.expected_revision}）`}
              onPress={() => { if (confirming) void confirm(confirming); }}
            />
            <Button label="返回" variant="secondary" accessibilityLabel="返回不确认" onPress={() => setConfirming(null)} size="compact" />
          </View>
        }
      >
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
          {confirming === 'approve'
            ? `将以修订 ${snapshot?.record.expected_revision} 批准 ${targetSummary.tool} 的调用。批准记录后外部操作才会执行，执行结果以事件流为准。`
            : `将以修订 ${snapshot?.record.expected_revision} 拒绝 ${targetSummary.tool} 的调用。拒绝后该调用不会执行。`}
        </Text>
      </Sheet>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
void styles;
