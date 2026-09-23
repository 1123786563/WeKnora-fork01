import { useEffect, useRef, useState } from 'react';
import { Text, View } from 'react-native';
import { activeMobileRuntime } from '../composition.ts';
import { createResourceShelfController, type ResourceShelfViewState } from '../resources-view.ts';
import { ResourcesScreen } from '../screens/ResourcesScreen.tsx';

/** Expo Router 文件路由：/resources。只消费 Resource Shelf Interface（AC2）。 */
export default function ResourcesRoute() {
  const controllerRef = useRef<ReturnType<typeof createResourceShelfController> | undefined>(undefined);
  if (!controllerRef.current) {
    const handle = activeMobileRuntime().resourceShelf();
    controllerRef.current = handle ? createResourceShelfController(handle) : undefined;
  }
  const [state, setState] = useState<ResourceShelfViewState>(controllerRef.current?.state() ?? { loading: false });
  useEffect(() => {
    const controller = controllerRef.current;
    if (!controller) return;
    setState(controller.state());
    return controller.subscribe(setState);
  }, []);
  if (!controllerRef.current) {
    return (
      <View>
        <Text>Sign in to browse tenant resources.</Text>
      </View>
    );
  }
  return <ResourcesScreen page={state.page} loading={state.loading} error={state.error} onRefresh={() => { controllerRef.current?.refresh(); }} />;
}
