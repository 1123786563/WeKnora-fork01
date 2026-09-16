import * as React from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';
import { SessionView } from '@/-session/SessionView';
import { ConversationViewModelContext } from './context';
import { createRequestID, type ConversationViewModel } from './view-model';
export { ProductConversationMessages } from './ProductConversationMessages';

export interface ConversationScreenProps {
  sessionId: string;
  viewModel: ConversationViewModel;
  /** Injectable native renderer seam used by the RN interaction tests. */
  sessionRenderer?: React.ComponentType<{ id: string; viewModel: ConversationViewModel }>;
}

export function ConversationControlPanel({ viewModel }: { viewModel: ConversationViewModel }) {
  const [draft, setDraft] = React.useState('');
  const [busy, setBusy] = React.useState(false);
  const submit = async () => {
    if (!viewModel.send || !draft.trim() || busy) return;
    setBusy(true);
    try {
      await viewModel.send.submit(draft, createRequestID());
      setDraft('');
    } finally {
      setBusy(false);
    }
  };
  return (
    <View accessibilityLabel="conversation-controls">
      <TextInput accessibilityLabel="conversation-draft" value={draft} onChangeText={setDraft} />
      <Pressable accessibilityRole="button" accessibilityLabel="发送" disabled={busy} onPress={() => void submit()}>
        <Text>{busy ? '发送中' : '发送'}</Text>
      </Pressable>
      {viewModel.pendingInteractions.filter((item) => item.status === 'pending').map((item) => (
        <View key={item.id} accessibilityLabel={`pending-interaction-${item.id}`} style={{ paddingVertical: 6 }}>
          <Text>{item.label}</Text>
          {item.reason ? <Text>{item.reason}</Text> : null}
          {item.error ? <Text accessibilityRole="alert">{item.error}</Text> : null}
          <View style={{ flexDirection: 'row', gap: 8 }}>
            <Pressable accessibilityRole="button" accessibilityLabel={`批准 ${item.label}`} onPress={() => void Promise.resolve(viewModel.commands.approve?.(item.id, item.revision ?? 0)).catch(() => undefined)}>
              <Text>批准</Text>
            </Pressable>
            <Pressable accessibilityRole="button" accessibilityLabel={`拒绝 ${item.label}`} onPress={() => void Promise.resolve(viewModel.commands.reject?.(item.id, item.revision ?? 0)).catch(() => undefined)}>
              <Text>拒绝</Text>
            </Pressable>
            <Pressable accessibilityRole="button" accessibilityLabel={`刷新 ${item.label}`} onPress={() => void viewModel.commands.refreshPending?.(item.id)}>
              <Text>刷新</Text>
            </Pressable>
          </View>
        </View>
      ))}
    </View>
  );
}

/** Product-owned seam around the retained Happy renderer. */
export function ConversationScreen({ sessionId, viewModel, sessionRenderer: SessionRenderer = SessionView }: ConversationScreenProps) {
  const [, redraw] = React.useReducer((value: number) => value + 1, 0);
  React.useEffect(() => viewModel.subscribe?.(() => redraw()), [redraw, viewModel]);
  const executionNotice = viewModel.execution?.status === 'unknown'
    ? '连接状态未知，正在等待服务端确认'
    : viewModel.execution?.status === 'pending' || viewModel.execution?.status === 'dispatching'
      ? '任务正在排队'
      : null;
  return (
    <ConversationViewModelContext.Provider value={viewModel}>
      <View style={{ flex: 1 }}>
        {executionNotice && (
          <View accessibilityRole="alert" style={{ paddingHorizontal: 16, paddingVertical: 8 }}>
            <Text>{executionNotice}</Text>
          </View>
        )}
        <ConversationControlPanel viewModel={viewModel} />
        <SessionRenderer id={sessionId} viewModel={viewModel} />
      </View>
    </ConversationViewModelContext.Provider>
  );
}
