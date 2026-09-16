import * as React from 'react';
import { Text, View } from 'react-native';
import { SessionView } from '@/-session/SessionView';
import { ConversationViewModelContext, type ConversationViewModel } from './context';

export interface ConversationScreenProps {
  sessionId: string;
  viewModel: ConversationViewModel;
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
        <SessionView id={sessionId} />
      </View>
    </ConversationViewModelContext.Provider>
  );
}
