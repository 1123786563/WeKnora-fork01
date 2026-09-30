import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import type { KnowledgeResource } from '@weknora/domain/mobile';
import { activeMobileRuntime, activeTaskOffice } from '../composition.ts';
import { createKnowledgeQAController, type KnowledgeQAController, type KnowledgeQAViewState } from '../knowledge-qa-view.ts';
import { KnowledgeQAScreen } from '../screens/KnowledgeQAScreen.tsx';

/** /ask 的挂载生命周期宿主：controller 在 effect 内创建，卸载时 dispose。 */
export function KnowledgeQARouteLifecycle({ office }: { office: NonNullable<ReturnType<typeof activeTaskOffice>> }) {
  const runtime = activeMobileRuntime();
  const [state, setState] = useState<KnowledgeQAViewState | undefined>(undefined);
  const [controller, setController] = useState<KnowledgeQAController | undefined>(undefined);
  useEffect(() => {
    let disposed = false;
    let createdController: KnowledgeQAController | undefined;
    void (async () => {
      const created = createKnowledgeQAController({
        office,
        knowledge: async () => {
          const handle = runtime.resourceShelf();
          if (!handle) return [];
          return (await handle.browse()).knowledge as KnowledgeResource[];
        },
      });
      if (disposed) { created.dispose(); return; }
      createdController = created;
      setController(created);
      setState(created.state());
    })();
    return () => {
      disposed = true;
      createdController?.dispose();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- office/runtime 是 app 生命周期单例
  }, [office, runtime]);
  useEffect(() => (controller === undefined ? undefined : controller.subscribe(setState)), [controller]);
  if (controller === undefined || state === undefined) {
    return (
      <View>
        <Text>Loading</Text>
      </View>
    );
  }
  return (
    <KnowledgeQAScreen
      state={state}
      onUpdate={(patch) => { controller.update(patch); }}
      onToggleKnowledge={(knowledgeId) => { controller.toggleKnowledge(knowledgeId); }}
      onAsk={() => { void controller.ask(); }}
      onRetry={() => { void controller.retry(); }}
      onRefreshKnowledge={() => { void controller.refreshKnowledge(); }}
    />
  );
}

/** Expo Router 文件路由：/ask（快速知识问答——答案即 Task，自动进入历史）。只消费 Task Office 与 Resource Shelf Interface。 */
export default function KnowledgeQARoute() {
  const office = activeTaskOffice();
  if (!office) {
    return (
      <View>
        <Text>Sign in to ask a knowledge question.</Text>
      </View>
    );
  }
  return <KnowledgeQARouteLifecycle office={office} />;
}
