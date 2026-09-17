import React, { useCallback, useMemo, useState } from 'react';
import { KeyboardAvoidingView, Platform, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { ProductMessageList } from './ProductMessageList.tsx';
import { Button } from '../ui/Button.tsx';
import { Field } from '../ui/Field.tsx';
import { StatusBadge, type BadgeTone } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import type { ProductConversationViewModel, SendController } from './view-model.ts';

/**
 * 产品对话屏（MX-017 重写）：只依赖产品 VM/ports，零 Happy hooks（G07 页面半边）。
 * 输入端单飞发送——request_id 语义由持久提交协调器（MX-006）注入的 SendController
 * 承载；本屏不发明 ID、不 fetch、不调用任意 RPC。
 */
export interface ConversationScreenProps {
  viewModel: ProductConversationViewModel;
  send?: SendController;
  onSend?: (text: string) => Promise<void>;
  testID?: string;
}

const RUN_TONE: Record<string, BadgeTone> = {
  queued: 'neutral', running: 'brand', waiting_user: 'warning', reconciling: 'info',
  succeeded: 'neutral', failed: 'danger', canceled: 'neutral',
};

export function ConversationScreen({ viewModel, send, onSend, testID }: ConversationScreenProps) {
  const { theme } = useWeknoraTheme();
  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const [snapshotVersion, bumpSnapshot] = useState(0);
  const state = useMemo(() => viewModel.state(), [viewModel, snapshotVersion]);

  const submit = useCallback(async () => {
    if (draft.trim() === '' || sending) return;
    setSending(true);
    try {
      if (send) {
        await send.submit(draft, `persisted:${Date.now()}:${Math.random().toString(36).slice(2, 10)}`);
      }
      await onSend?.(draft);
      setDraft('');
    } finally {
      setSending(false);
      bumpSnapshot((n) => n + 1);
    }
  }, [draft, sending, send, onSend]);

  const cancelDisabled = !state.capabilities.canCancel;
  const terminal = ['succeeded', 'failed', 'canceled'].includes(state.runStatus);

  return (
    <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={[styles.container, { backgroundColor: theme.colors.bg }]} testID={testID}>
      <View style={[styles.header, { borderBottomColor: theme.colors.line, padding: theme.spacing[12] }]}>
        <StatusBadge tone={RUN_TONE[state.runStatus] ?? 'neutral'} label={state.runStatus} />
        <Text accessibilityLabel={`执行状态 ${state.executionStatus}，结算状态 ${state.settlementStatus}，修订 ${state.revision}`} style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginLeft: theme.spacing[8] }}>
          执行 {state.executionStatus} · 结算 {state.settlementStatus}
        </Text>
      </View>
      <View style={{ flex: 1 }}>
        {terminal && state.messages.length === 0 ? (
          <StateView kind="empty" message="该任务已结束且没有可显示的消息" />
        ) : (
          <ProductMessageList messages={state.messages} empty={state.messages.length === 0 && !terminal} />
        )}
      </View>
      <View style={[styles.inputRow, { borderTopColor: theme.colors.line, padding: theme.spacing[12], gap: theme.spacing[8] }]}>
        <View style={styles.grow}>
          <Field label="指令" value={draft} onChangeText={setDraft} placeholder="输入指令或补充要求" />
        </View>
        <Button label="发送" onPress={() => void submit()} loading={sending} disabled={draft.trim() === ''} accessibilityLabel="发送指令" size="compact" />
        <Button
          label="取消任务"
          variant="danger"
          accessibilityLabel="申请取消该任务（执行停止需服务端确认）"
          disabled={cancelDisabled}
          onPress={() => void viewModel.commands.cancel(viewModel.runID).finally(() => bumpSnapshot((n) => n + 1))}
          size="compact"
        />
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  header: { flexDirection: 'row', alignItems: 'center', borderBottomWidth: StyleSheet.hairlineWidth },
  inputRow: { flexDirection: 'row', alignItems: 'flex-end', borderTopWidth: StyleSheet.hairlineWidth },
  grow: { flex: 1 },
});
