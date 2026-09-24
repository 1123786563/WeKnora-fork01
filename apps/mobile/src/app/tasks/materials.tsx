import { useEffect, useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { activeMobileRuntime, activeTaskMaterial } from '../../composition.ts';
import { createMaterialsController, type MaterialsController, type MaterialsViewState } from '../../materials-view.ts';
import { MaterialsScreen } from '../../screens/MaterialsScreen.tsx';

/** /tasks/materials 挂载生命周期宿主：handle 在 effect 内开、卸载即 close——与 /tasks/detail 同一模式。 */
export function MaterialsRouteLifecycle({ runId }: { runId: string }) {
  const [state, setState] = useState<MaterialsViewState>({ loading: true });
  const [controller, setController] = useState<MaterialsController | undefined>(undefined);
  useEffect(() => {
    const runtime = activeMobileRuntime();
    const material = activeTaskMaterial();
    const lease = runtime.scopeLease();
    if (!material || !lease || runId.trim() === '') {
      setState({ loading: false, error: '请先登录并激活空间，再查看任务材料。' });
      return;
    }
    let next: MaterialsController | undefined;
    try {
      next = createMaterialsController(material.open({ lease }), { runId });
    } catch {
      setState({ loading: false, error: '登录状态或活动空间已变化，请重新进入。' });
      return;
    }
    setController(next);
    setState(next.state());
    const unsubscribe = next.subscribe(setState);
    void next.load();
    return () => {
      unsubscribe();
      next?.dispose();
    };
  }, [runId]);
  return (
    <MaterialsScreen
      index={state.index}
      view={state.view}
      loading={state.loading}
      error={state.error}
      grant={state.grant}
      onOpenMaterial={(materialId) => { void controller?.openMaterial(materialId); }}
      onOpenTerminal={() => { void controller?.openTerminal(); }}
      onOpenEvidence={() => { void controller?.openEvidence(); }}
      onDownload={(materialId) => { void controller?.download(materialId); }}
      onShare={(materialId) => { void controller?.share(materialId); }}
      onRefresh={() => { void controller?.load(); }}
      onBack={() => router.back()}
    />
  );
}

/** Expo Router 文件路由：/tasks/materials?runId=..。只消费 Task Material Interface。 */
export default function TaskMaterialsRoute() {
  const params = useLocalSearchParams<{ runId?: string }>();
  return <MaterialsRouteLifecycle runId={String(params.runId ?? '')} />;
}
