import { useEffect, useRef, useState } from 'react';
import { Button, Text, View } from 'react-native';
import type { ResourceShelfHandle } from '@weknora/mobile-core';
import { createResourceShelfController, type ResourceShelfViewState } from '../resources-view.ts';
import { ResourcesScreen } from './ResourcesScreen.tsx';

export interface ReadOnlyScreenProps {
  deploymentLabel?: string;
  handle?: ResourceShelfHandle;
  onSignOut(): Promise<void>;
}

/**
 * 有限只读降级面（spec：Missing security-critical capabilities produce an explanation or
 * limited read-only mode, not optimistic calls）。只渲染说明 + Resource Shelf 只读投影；
 * 不挂 Task Office，不提供任何授权控制。
 */
export function ReadOnlyScreen({ deploymentLabel, handle, onSignOut }: ReadOnlyScreenProps) {
  const controllerRef = useRef<ReturnType<typeof createResourceShelfController> | undefined>(undefined);
  // 初始投影是纯计算：与 app/resources.tsx 的 ResourcesRouteLifecycle 相同的 commit 后副作用纪律。
  const [state, setState] = useState<ResourceShelfViewState>(handle ? { loading: true } : { loading: false });
  useEffect(() => {
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
  return (
    <View>
      <Text>Limited read-only mode</Text>
      <Text>{deploymentLabel ? `${deploymentLabel} is behind this version of WeKnora.` : 'This deployment is behind this version of WeKnora.'}</Text>
      <Text>Task commands are disabled. Ask the deployment administrator to upgrade, or switch to another deployment.</Text>
      <Button title="Sign out" onPress={() => { void onSignOut(); }} />
      {handle
        ? <ResourcesScreen page={state.page} loading={state.loading} error={state.error} onRefresh={() => { controllerRef.current?.refresh(); }} />
        : <Text>Read-only browsing is unavailable.</Text>}
    </View>
  );
}
