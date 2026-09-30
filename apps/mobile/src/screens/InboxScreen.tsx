import { Button, ScrollView, Text, View } from 'react-native';
import type { InboxItem, InboxView } from '@weknora/mobile-core';

export interface InboxScreenProps {
  view?: InboxView;
  loading: boolean;
  error?: string;
  notice?: string;
  onRefresh(): void;
  onLoadMore?(): void;
  onOpenNotification(item: InboxItem): void;
}

/**
 * 行动通知收件箱屏（#41）：只消费 Inbox 投影与回调（module-seams §10——Screen 不导入
 * api-client、不维护 cursor/未读状态）。深链安全裁决在 composition 的
 * openNotificationFromInbox（解析 + 重新鉴权 + 导航），屏内不解析深链。
 */
export function InboxScreen({ view, loading, error, notice, onRefresh, onLoadMore, onOpenNotification }: InboxScreenProps) {
  if (view === undefined && error !== undefined) {
    return (
      <View>
        <Text>无法读取行动通知</Text>
        <Text>{error}</Text>
        <Button title="重试" onPress={onRefresh} />
      </View>
    );
  }
  return (
    <ScrollView>
      <Text>{`未读 ${view?.unreadCount ?? 0}`}</Text>
      {notice !== undefined && <Text>{notice}</Text>}
      {/* 已有视图时刷新/加载更多失败不再静默：保留列表，就地渲染错误文案（唯一整屏错误分支在上方 view===undefined 早退）。 */}
      {error !== undefined && <Text>{error}</Text>}
      {loading && <Text>正在同步…</Text>}
      {(view?.items ?? []).map((item) => (
        <Button
          key={item.notificationId}
          title={`${item.kind} · ${item.title === '' ? item.notificationId : item.title}${item.read ? '' : ' · 未读'}`}
          onPress={() => { onOpenNotification(item); }}
        />
      ))}
      {view !== undefined && view.items.length === 0 && !loading && <Text>暂无行动通知</Text>}
      {view?.nextCursor !== undefined && onLoadMore !== undefined
        ? <Button title="Load more" onPress={onLoadMore} />
        : null}
      <Button title="刷新" onPress={onRefresh} />
    </ScrollView>
  );
}
