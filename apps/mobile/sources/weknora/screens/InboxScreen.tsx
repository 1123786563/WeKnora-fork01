import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { FlatList, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { StatusBadge, type BadgeTone } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import { resolveDeepLink, type DeepLinkIntent, type DeepLinkScope } from '../notifications/deep-link.ts';

/**
 * 收件箱页 M10（MX-021）：通知只提示——点击后恢复身份→空间确认→重查权限；
 * 已读为幂等本地动作（不执行通知描述的操作）。
 */
export interface InboxItemView {
  notificationId: string;
  kind: string;
  title: string;
  body: string;
  createdAt: string;
  read: boolean;
  deepLink?: string;
  tenantId: string;
}

export interface InboxScreenProps {
  items: readonly InboxItemView[];
  unreadCount: number;
  loading?: boolean;
  error?: string | null;
  scope: DeepLinkScope;
  onOpenItem: (item: InboxItemView) => void;
  onMarkRead: (notificationId: string) => void;
  onRetry?: () => void;
  testID?: string;
}

const KIND_TONE: Record<string, BadgeTone> = {
  approval: 'warning', run_update: 'brand', usage: 'info', system: 'neutral',
};

export function InboxScreen({ items, unreadCount, loading, error, scope, onOpenItem, onMarkRead, onRetry, testID }: InboxScreenProps) {
  const { theme } = useWeknoraTheme();

  if (loading) {
    return <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}><StateView kind="loading" message="正在加载收件箱" /></View>;
  }
  if (error) {
    return (
      <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}>
        <StateView kind="error" message="收件箱加载失败" detail={error} actionLabel="重试" onAction={onRetry} />
      </View>
    );
  }
  if (items.length === 0) {
    return (
      <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}>
        <StateView kind="empty" message="暂无通知" detail="审批与任务进展会在这里提示" />
      </View>
    );
  }

  return (
    <FlatList
      testID={testID}
      style={{ backgroundColor: theme.colors.bg }}
      data={[...items]}
      keyExtractor={(item) => item.notificationId}
      contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[12] }}
      ListHeaderComponent={
        unreadCount > 0 ? (
          <Text accessibilityLabel={`${unreadCount} 条未读通知`} style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, paddingHorizontal: theme.spacing[4] }}>
            {unreadCount} 条未读
          </Text>
        ) : null
      }
      renderItem={({ item }) => {
        const open = () => {
          onMarkRead(item.notificationId);
          const intent: DeepLinkIntent = { kind: item.kind === 'approval' ? 'interaction' : 'run', id: item.deepLink ?? item.notificationId, tenantId: item.tenantId };
          onOpenItem({ ...item });
          return intent;
        };
        void open;
        return (
          <Card
            onPress={() => {
              onMarkRead(item.notificationId);
              onOpenItem(item);
            }}
            accessibilityLabel={`${item.read ? '已读' : '未读'}通知：${item.title}。${item.body}。点击后恢复身份、确认空间并重新验证权限。`}
            tone={item.read ? 'surface' : 'alt'}
            compact
          >
            <View style={styles.row}>
              <View style={styles.grow}>
                <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, fontWeight: item.read ? '400' : '600' }}>
                  {item.title}
                </Text>
                <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
                  {item.body}
                </Text>
                <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
                  {item.createdAt}
                </Text>
              </View>
              <StatusBadge tone={item.read ? 'neutral' : KIND_TONE[item.kind] ?? 'neutral'} label={item.read ? '已读' : '未读'} />
            </View>
          </Card>
        );
      }}
    />
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  row: { flexDirection: 'row', alignItems: 'center' },
  grow: { flex: 1 },
});

export { resolveDeepLink };
