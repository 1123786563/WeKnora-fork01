import * as React from 'react';
import { ScrollView, Text, View } from 'react-native';
import type { ConversationViewModel } from './view-model';

/** Product message projection. It deliberately consumes only VM messages. */
export function ProductConversationMessages({ viewModel }: { viewModel: ConversationViewModel }) {
  return (
    <ScrollView accessibilityLabel="product-conversation-messages" contentContainerStyle={{ padding: 16, gap: 10 }}>
      {viewModel.messages.map((message) => (
        <View key={message.id} accessibilityLabel={`message-${message.id}`}>
          <Text accessibilityLabel={`message-role-${message.id}`}>{message.role}</Text>
          <Text>{message.text}</Text>
        </View>
      ))}
      {viewModel.execution ? (
        <View accessibilityLabel="execution-state">
          <Text>{viewModel.execution.status}</Text>
          {viewModel.execution.reason ? <Text>{viewModel.execution.reason}</Text> : null}
        </View>
      ) : null}
    </ScrollView>
  );
}
