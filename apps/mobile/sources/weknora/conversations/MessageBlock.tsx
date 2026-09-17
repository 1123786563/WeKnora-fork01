import React from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import type { ConversationMessage } from './view-model.ts';

/**
 * 消息块（MX-017）：稳定 id 渲染、角色语义色（不只靠对齐区分）、
 * 文本主通道；工具/审批卡片形态随后续消息类型任务扩展。
 */
export function MessageBlock({ message }: { message: ConversationMessage }) {
  const { theme } = useWeknoraTheme();
  const isUser = message.role === 'user';
  return (
    <View
      accessibilityLabel={`${isUser ? '用户' : '助手'}消息：${message.text}`}
      style={[
        styles.bubble,
        {
          backgroundColor: isUser ? theme.colors['brand-soft'] : theme.colors.surface,
          borderColor: theme.colors.line,
          borderRadius: theme.radius.control,
          padding: theme.spacing[12],
        },
        isUser ? styles.userAlign : styles.assistantAlign,
      ]}
    >
      <Text style={{ color: isUser ? theme.colors.brand : theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, fontWeight: '600' }}>
        {isUser ? '用户' : '助手'}
      </Text>
      <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, marginTop: theme.spacing[4] }}>
        {message.text}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  bubble: { borderWidth: StyleSheet.hairlineWidth, maxWidth: '85%' },
  userAlign: { alignSelf: 'flex-end' },
  assistantAlign: { alignSelf: 'flex-start' },
});
