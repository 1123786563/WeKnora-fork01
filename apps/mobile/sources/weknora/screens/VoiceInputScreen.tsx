import React, { useCallback, useMemo, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import {
  createDictationStateMachine,
  type DictationController,
} from '../voice/dictation.ts';

/**
 * 语音输入页 M16（MX-028）：确认式听写——识别进草稿可编辑；确认提交用户编辑后的
 * 最新文本；「结束听写」「取消听写」「取消任务」三个动作分离。
 */
export interface VoiceInputScreenProps {
  controller?: DictationController;
  onStartListening?: () => void;
  onStopListening?: () => void;
  onSubmitDraft?: (draft: string) => Promise<void> | void;
  onCancelTask?: () => void;
  testID?: string;
}

export function VoiceInputScreen({ controller: injected, onStartListening, onStopListening, onSubmitDraft, onCancelTask, testID }: VoiceInputScreenProps) {
  const { theme } = useWeknoraTheme();
  const controller = useMemo(() => injected ?? createDictationStateMachine(), [injected]);
  const [session, setSession] = useState(controller.state);
  const [submittedCount, setSubmittedCount] = useState(0);

  const sync = useCallback((next: ReturnType<DictationController['dispatch']>) => {
    setSession(controller.state);
    if (next.type === 'confirmed') setSubmittedCount((n) => n + 1);
  }, [controller]);

  const confirm = () => {
    sync(controller.dispatch({ type: 'confirm', submitVia: async (draft) => { await onSubmitDraft?.(draft); } }));
  };

  return (
    <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}>
      <Card>
        <View style={styles.row}>
          <StatusBadge tone={session.listening ? 'brand' : 'neutral'} label={session.listening ? '正在听写' : '未在听写'} />
          <Text accessibilityLabel={session.listening ? '正在听写，识别内容会进入下方草稿' : '听写未开始'} style={{ color: theme.colors.muted, flex: 1, marginLeft: theme.spacing[8], fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
            识别内容进入草稿，确认前可自由编辑
          </Text>
        </View>
      </Card>

      <Card>
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>
          草稿（编辑后将以你的最终文本提交）
        </Text>
        <Pressable
          accessibilityRole="text"
          accessibilityLabel={`听写草稿，当前内容：${session.draft || '（空）'}`}
          style={[styles.draftBox, { borderColor: theme.colors['control-line'], borderRadius: theme.radius.control, padding: theme.spacing[12], marginTop: theme.spacing[12] }]}
        >
          <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight }}>
            {session.draft || '（开始听写或直接输入）'}
          </Text>
        </Pressable>
      </Card>

      {session.draft.trim() === '' ? (
        <StateView kind="empty" message="还没有听写内容" detail="开始听写或取消返回" />
      ) : null}

      <View style={{ gap: theme.spacing[12] }}>
        <Button
          label={session.listening ? '结束听写' : '开始听写'}
          onPress={() => {
            if (session.listening) {
              onStopListening?.();
              sync(controller.dispatch({ type: 'stop_listening' }));
            } else {
              onStartListening?.();
              sync(controller.dispatch({ type: 'start' }));
            }
          }}
          accessibilityLabel={session.listening ? '结束听写（识别停止，草稿保留）' : '开始听写'}
        />
        <Button
          label="确认并放入会话草稿"
          disabled={session.draft.trim() === ''}
          onPress={confirm}
          accessibilityLabel="确认听写内容，以你编辑后的最终文本放入会话草稿"
        />
        <Button
          label="取消听写（丢弃本段）"
          variant="secondary"
          onPress={() => sync(controller.dispatch({ type: 'cancel' }))}
          accessibilityLabel="取消听写并丢弃本段识别（不影响已提交内容）"
          size="compact"
        />
        {onCancelTask ? (
          <Button label="取消任务（独立操作）" variant="danger" onPress={onCancelTask} accessibilityLabel="取消任务——与取消听写是不同操作" size="compact" />
        ) : null}
      </View>
      {submittedCount > 0 ? (
        <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
          已确认 {submittedCount} 段（进入会话草稿，未直接发送）
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 16 },
  row: { flexDirection: 'row', alignItems: 'center' },
  draftBox: { borderWidth: StyleSheet.hairlineWidth, minHeight: 96 },
});
