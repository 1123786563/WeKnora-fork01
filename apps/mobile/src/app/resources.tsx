import { useEffect, useRef, useState } from 'react';
import { Text, View } from 'react-native';
import type { ResourceShelfHandle } from '@weknora/mobile-core';
import { activeMobileRuntime } from '../composition.ts';
import { createResourceShelfController, type ResourceShelfController, type ResourceShelfViewState } from '../resources-view.ts';
import { ResourcesScreen } from '../screens/ResourcesScreen.tsx';

/**
 * /resources 的挂载生命周期宿主（handle 经 props 注入）：卸载时 dispose controller，
 * 把订阅从每 scope 唯一的长寿命 shelf handle 上摘除——失效事件不得在已卸载页面触发 browse。
 */
export function ResourcesRouteLifecycle({ handle }: { handle?: ResourceShelfHandle }) {
  const controllerRef = useRef<ResourceShelfController | undefined>(undefined);
  if (!controllerRef.current && handle) {
    controllerRef.current = createResourceShelfController(handle);
  }
  const [state, setState] = useState<ResourceShelfViewState>(controllerRef.current?.state() ?? { loading: false });
  useEffect(() => {
    if (!controllerRef.current && handle) {
      controllerRef.current = createResourceShelfController(handle);
    }
    const controller = controllerRef.current;
    if (!controller) return;
    setState(controller.state());
    const unsubscribe = controller.subscribe(setState);
    return () => {
      unsubscribe();
      controller.dispose();
      controllerRef.current = undefined;
    };
  }, [handle]);
  if (!controllerRef.current) {
    return (
      <View>
        <Text>Sign in to browse tenant resources.</Text>
      </View>
    );
  }
  return <ResourcesScreen page={state.page} loading={state.loading} error={state.error} onRefresh={() => { controllerRef.current?.refresh(); }} />;
}

/** Expo Router 文件路由：/resources。只消费 Resource Shelf Interface（AC2）。 */
export default function ResourcesRoute() {
  return <ResourcesRouteLifecycle handle={activeMobileRuntime().resourceShelf()} />;
}
