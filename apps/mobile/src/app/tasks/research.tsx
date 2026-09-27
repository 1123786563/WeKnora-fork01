import { useEffect, useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { activeMobileRuntime, activeTaskMaterial, activeTaskResearch } from '../../composition.ts';
import { createResearchController, type ResearchController, type ResearchViewState } from '../../research-view.ts';
import { ResearchScreen } from '../../screens/ResearchScreen.tsx';

/** /tasks/research 挂载生命周期宿主：handle 在 effect 内开、卸载即 close——与
 *  /tasks/materials 同一模式。材料索引用于批注表单的版本身份点选。 */
export function ResearchRouteLifecycle({ runId }: { runId: string }) {
  const [state, setState] = useState<ResearchViewState>({ loading: true });
  const [controller, setController] = useState<ResearchController | undefined>(undefined);
  useEffect(() => {
    const runtime = activeMobileRuntime();
    const research = activeTaskResearch();
    const material = activeTaskMaterial();
    const lease = runtime.scopeLease();
    if (!research || !lease || runId.trim() === '') {
      setState({ loading: false, error: '请先登录并激活空间，再查看研究面。' });
      return;
    }
    let next: ResearchController | undefined;
    try {
      next = createResearchController(research.open({ lease }), {
        runId,
        materials: async () => {
          if (material === undefined) return [];
          return (await material.open({ lease }).index({ runId })).materials;
        },
      });
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
  return createElementScreen(controller, state, runId);
}

function createElementScreen(controller: ResearchController | undefined, state: ResearchViewState, runId: string) {
  return (
    <ResearchScreen
      state={state}
      onDelegate={(objective, sources) => { void controller?.delegate({ objective, sources }); }}
      onAnnotate={(input) => { void controller?.annotate(input); }}
      onFlushDrafts={() => { void controller?.flushDrafts(); }}
      onRequestRevision={(input) => { void controller?.requestRevision(input); }}
      onRefresh={() => { void controller?.load(); }}
      onBack={() => router.back()}
    />
  );
}

/** Expo Router 文件路由：/tasks/research?runId=..。只消费 TaskResearch Interface。 */
export default function TaskResearchRoute() {
  const params = useLocalSearchParams<{ runId?: string }>();
  return <ResearchRouteLifecycle runId={String(params.runId ?? '')} />;
}
