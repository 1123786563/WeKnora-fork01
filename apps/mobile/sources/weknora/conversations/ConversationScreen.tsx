import * as React from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';
import { SessionView } from '@/-session/SessionView';
import { ConversationViewModelContext } from './context';
import type { ConversationViewModel } from './view-model';

export interface ConversationScreenProps {
  sessionId: string;
  viewModel: ConversationViewModel;
}

export function ConversationControlPanel({ viewModel }: { viewModel: ConversationViewModel }) {
  const [draft, setDraft] = React.useState('');
  const [busy, setBusy] = React.useState(false);
  const submit = async () => {
    if (!viewModel.send || !draft.trim() || busy) return;
    setBusy(true);
    try {
      await viewModel.send.submit(draft, `mobile:${viewModel.scope.userId ?? 'anonymous'}:${draft}`);
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
        <Pressable key={item.id} accessibilityRole="button" accessibilityLabel={`审批 ${item.label}`} onPress={() => void viewModel.commands.refreshPending?.(item.id)}>
          <Text>{item.label}</Text>
        </Pressable>
      ))}
    </View>
  );
}

/** Product-owned seam around the retained Happy renderer. */
export function ConversationScreen({ sessionId, viewModel }: ConversationScreenProps) {
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
        <SessionView id={sessionId} />
      </View>
    </ConversationViewModelContext.Provider>
  );
}
