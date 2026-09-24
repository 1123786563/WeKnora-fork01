import { useEffect, useRef, useState } from 'react';
import { Text, View } from 'react-native';
import { activeTaskOffice } from '../composition.ts';
import { createAttentionInboxController, type AttentionInboxController, type AttentionInboxViewState } from '../attention-inbox-view.ts';
import { AttentionInboxScreen } from '../screens/AttentionInboxScreen.tsx';

/** /inbox 挂载生命周期宿主：controller 在 effect 内创建，与 /tasks/detail 同一模式。 */
export function InboxRouteLifecycle() {
  const [state, setState] = useState<AttentionInboxViewState>({ loading: true, receipts: [] });
  const controllerRef = useRef<AttentionInboxController | undefined>(undefined);
  useEffect(() => {
    const office = activeTaskOffice();
    if (!office) {
      setState({ loading: false, error: '请先登录并激活空间，再打开收件箱。', receipts: [] });
      return;
    }
    const controller = createAttentionInboxController(office);
    controllerRef.current = controller;
    setState(controller.state());
    void controller.refresh();
    const unsubscribe = controller.subscribe(setState);
    return () => {
      unsubscribe();
      controllerRef.current = undefined;
    };
  }, []);
  if (!controllerRef.current) {
    return (
      <View>
        <Text>{state.error ?? '正在读取收件箱…'}</Text>
      </View>
    );
  }
  return (
    <AttentionInboxScreen
      state={state}
      onRefresh={() => { void controllerRef.current?.refresh(); }}
      onDecide={(item, action) => { void controllerRef.current?.decide(item, action); }}
    />
  );
}

/** Expo Router 文件路由：/inbox。只消费 Task Office Interface。 */
export default function Inbox() {
  return <InboxRouteLifecycle />;
}
