import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import { router } from 'expo-router';
import type { InboxItem, InboxView, NotificationInbox } from '@weknora/mobile-core';
import { activeMobileRuntime, notificationInboxFor, openNotificationFromInbox } from '../composition.ts';
import { InboxScreen } from '../screens/InboxScreen.tsx';

interface InboxRouteState {
  view?: InboxView;
  loading: boolean;
  error?: string;
  notice?: string;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** /inbox 挂载生命周期宿主：subscribe 持续接收投影，page() 首次加载与手动刷新共用（与 /resources 同一模式）。 */
export function InboxRouteLifecycle({ inbox }: { inbox: NotificationInbox }) {
  const activeRuntime = activeMobileRuntime();
  const [state, setState] = useState<InboxRouteState>({ loading: true });
  useEffect(() => {
    let alive = true;
    const unsubscribe = inbox.subscribe((view) => {
      if (alive) setState({ view, loading: false, error: undefined });
    });
    void inbox.page().then(
      () => undefined,
      (failure) => { if (alive) setState((current) => ({ ...current, loading: false, error: errorMessage(failure) })); },
    );
    return () => {
      alive = false;
      unsubscribe();
    };
  }, [inbox]);
  const refresh = (): void => {
    setState((current) => ({ ...current, loading: true, error: undefined }));
    void inbox.page().then(
      () => undefined,
      (failure) => { setState((current) => ({ ...current, loading: false, error: errorMessage(failure) })); },
    );
  };
  const loadMore = state.view?.nextCursor !== undefined
    ? (): void => {
        void inbox.more().then(
          () => undefined,
          (failure) => { setState((current) => ({ ...current, error: errorMessage(failure) })); },
        );
      }
    : undefined;
  const openNotification = (item: InboxItem): void => {
    void (async () => {
      const outcome = await openNotificationFromInbox(inbox, activeRuntime.snapshot(), item, (path, params) => {
        router.push({ pathname: path, params });
      });
      setState((current) => ({
        ...current,
        notice: outcome === 'invalid-link'
          ? '该通知的链接无法安全打开。'
          : outcome === 'blocked-unauthorized'
            ? '请先登录并激活空间，再打开该任务。'
            : undefined,
      }));
    })();
  };
  return (
    <InboxScreen
      view={state.view}
      loading={state.loading}
      error={state.error}
      notice={state.notice}
      onRefresh={refresh}
      {...(loadMore === undefined ? {} : { onLoadMore: loadMore })}
      onOpenNotification={openNotification}
    />
  );
}

/** Expo Router 文件路由：/inbox。只消费行动通知 Inbox Interface；未授权面给出登录引导。 */
export default function InboxRoute() {
  const activeRuntime = activeMobileRuntime();
  const snapshot = activeRuntime.snapshot();
  const origin = snapshot.deployment?.origin;
  if (snapshot.surface !== 'authorized' || origin === undefined) {
    return (
      <View>
        <Text>请先登录并激活空间，再查看行动通知。</Text>
      </View>
    );
  }
  return <InboxRouteLifecycle inbox={notificationInboxFor(activeRuntime, origin)} />;
}
