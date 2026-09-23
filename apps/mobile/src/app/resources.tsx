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
  // 初始投影是纯计算：handle 存在时与 controller 的初始 state（{ loading: true }，
  // resources-view.ts）一致；handle 缺失时无 controller 可言。effect 提交时以
  // controller.state() 校准。
  const [state, setState] = useState<ResourceShelfViewState>(handle ? { loading: true } : { loading: false });
  useEffect(() => {
    // controller 只在 effect 内创建：createResourceShelfController 会订阅长寿命 shelf
    // handle 并触发首次 load（副作用）。concurrent 渲染中被丢弃的 render 没有 effect 提交，
    // 若在 render 相创建，会留下一个永不清理的订阅——副作用必须收敛到 commit 之后。
    if (!handle) return;
    const controller = createResourceShelfController(handle);
    controllerRef.current = controller;
    setState(controller.state());
    const unsubscribe = controller.subscribe(setState);
    return () => {
      unsubscribe();
      controller.dispose();
      controllerRef.current = undefined;
    };
  }, [handle]);
  if (!handle) {
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
