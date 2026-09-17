import React, { useMemo, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';
import type { VoiceSessionController, VoiceSessionState } from '../voice/realtime.ts';

/**
 * 实时语音面板（MX-029 / M07 语音区）：三种停止操作显式分离——
 * 停止播报（只停输出）/ 结束语音（会话终止）/ 取消任务（独立命令）；
 * 短期令牌过期提示重授权；语音无审批权提示。
 */
export interface VoiceSessionPanelProps {
  controller: VoiceSessionController;
  session: VoiceSessionState | null;
  onStart: () => void;
  onCancelRun: () => void;
  testID?: string;
}

export function VoiceSessionPanel({ controller, session, onStart, onCancelRun, testID }: VoiceSessionPanelProps) {
  const { theme } = useWeknoraTheme();
  const [busy, setBusy] = useState(false);
  const tokenExpired = useMemo(() => session !== null && new Date(session.mediaToken.expiresAt).getTime() < Date.now(), [session]);

  const stop = async (action: 'interrupt_output' | 'end_session') => {
    if (!session || busy) return;
    setBusy(true);
    try {
      await controller.stop(action, { expectedRevision: 0 });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card testID={testID}>
      <View style={styles.row}>
        <StatusBadge tone={session === null ? 'neutral' : session.outputActive ? 'brand' : 'warning'} label={session === null ? '语音未开始' : session.outputActive ? '播报中' : '已停止播报'} />
        <Text accessibilityLabel={session === null ? '实时语音未开始' : `实时语音会话 ${session.sessionID}，绑定任务 ${session.runID}，${session.outputActive ? '正在播报' : '播报已停止'}`} style={{ color: theme.colors.muted, flex: 1, marginLeft: theme.spacing[8], fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
          {session === null ? '经产品 API 授权后开始（短期媒体令牌）' : `任务 ${session.runID}`}
        </Text>
      </View>
      {tokenExpired ? (
        <Text accessibilityRole="alert" style={{ color: theme.colors.warning, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, marginTop: theme.spacing[8] }}>
          媒体令牌已过期——重新授权后继续（不下发长期密钥）。
        </Text>
      ) : null}
      <View style={{ marginTop: theme.spacing[12], gap: theme.spacing[8] }}>
        {session === null ? (
          <Button label="开始实时语音" onPress={onStart} accessibilityLabel="开始实时语音会话（短期媒体令牌，经产品 API 授权）" />
        ) : (
          <>
            <Button label="停止播报（语音继续）" variant="secondary" disabled={busy || !session.outputActive} onPress={() => void stop('interrupt_output')} accessibilityLabel="停止播报——只停止媒体输出，语音会话与任务都不受影响" size="compact" />
            <Button label="结束语音会话" variant="secondary" disabled={busy} onPress={() => void stop('end_session')} accessibilityLabel="结束语音会话——令牌失效并关闭流，任务不受影响" size="compact" />
            <Button label="取消任务（独立操作）" variant="danger" disabled={busy} onPress={onCancelRun} accessibilityLabel="取消任务——走任务取消命令，与停止播报和结束语音是不同操作" size="compact" />
          </>
        )}
      </View>
      <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[8] }}>
        语音通道不携带审批权——危险操作审批仍在审批界面完成。
      </Text>
    </Card>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
