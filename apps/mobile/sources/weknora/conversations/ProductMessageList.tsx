import React from 'react';
import { FlatList, StyleSheet, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { MessageBlock } from './MessageBlock.tsx';
import { StateView } from '../ui/StateView.tsx';
import type { ConversationMessage } from './view-model.ts';

/**
 * 产品消息列表（MX-017）：倒序（最新在底）、稳定 message id 键；
 * 分段 append 更新由数据层保证（同 id upsert 不重建）；上滑不抢滚动
 * （仅在尾部追加分段，maintainVisibleContentPosition 语义由 inverted 列表承接）。
 */
export function ProductMessageList({ messages, empty }: { messages: readonly ConversationMessage[]; empty?: boolean }) {
  const { theme } = useWeknoraTheme();
  if (messages.length === 0) {
    return (
      <View style={[styles.container, { backgroundColor: theme.colors.bg }]}>
        <StateView kind={empty ? 'empty' : 'loading'} message={empty ? '还没有消息，发送第一条指令' : '正在同步消息'} />
      </View>
    );
  }
  return (
    <FlatList
      data={[...messages]}
      keyExtractor={(item) => item.id}
      inverted
      contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[12] }}
      renderItem={({ item }) => <MessageBlock message={item} />}
      style={{ backgroundColor: theme.colors.bg }}
      accessibilityLabel={`消息列表，共 ${messages.length} 条`}
    />
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
});
