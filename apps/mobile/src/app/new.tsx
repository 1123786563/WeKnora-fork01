import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import type { AgentOption, KnowledgeResource } from '@weknora/domain/mobile';
import { activeMobileRuntime, activeTaskOffice, openScopedDraftStore } from '../composition.ts';
import { createNativeNetworkStatusIfAvailable } from '../adapters/network-status.ts';
import { createNativeRequestId } from '../adapters/request-id.ts';
import { createScopedNewTaskDrafts } from '../new-task-drafts.ts';
import { createNewTaskController, type NewTaskController, type NewTaskViewState } from '../new-task-view.ts';
import { NewTaskScreen } from '../screens/NewTaskScreen.tsx';

/** /new 的挂载生命周期宿主：controller 与加密草稿在 effect 内创建，卸载时 dispose。 */
export function NewTaskRouteLifecycle({ office }: { office: NonNullable<ReturnType<typeof activeTaskOffice>> }) {
  const runtime = activeMobileRuntime();
  const [state, setState] = useState<NewTaskViewState | undefined>(undefined);
  const [controller, setController] = useState<NewTaskController | undefined>(undefined);
  useEffect(() => {
    let disposed = false;
    // cleanup 必须能拿到异步创建完成后的 controller：在 effect 作用域持有引用，
    // 而不是读取首帧渲染的 state（那是 undefined 的 stale closure）。
    let createdController: NewTaskController | undefined;
    void (async () => {
      const draftsStore = await openScopedDraftStore();
      const network = createNativeNetworkStatusIfAvailable();
      const created = createNewTaskController({
        office,
        agents: async () => {
          const handle = runtime.resourceShelf();
          if (!handle) return [];
          return (await handle.browse()).agents as AgentOption[];
        },
        knowledge: async () => {
          const handle = runtime.resourceShelf();
          if (!handle) return [];
          return (await handle.browse()).knowledge as KnowledgeResource[];
        },
        ...(draftsStore === undefined ? {} : { drafts: createScopedNewTaskDrafts(draftsStore.drafts) }),
        ...(network === undefined ? {} : { network }),
        newRequestId: createNativeRequestId(),
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
    <NewTaskScreen
      state={state}
      onUpdate={(patch) => { controller.update(patch); }}
      onSetAttachments={(attachments) => { controller.setAttachments(attachments); }}
      onToggleKnowledge={(knowledgeId) => { controller.toggleKnowledge(knowledgeId); }}
      onSubmit={() => { void controller.submit(); }}
      onCancel={() => { void controller.cancelKeepingDraft(); }}
      onRefreshAgents={() => { void controller.refreshAgents(); }}
    />
  );
}

/** Expo Router 文件路由：/new（统一 New 入口）。只消费 Task Office 与 Resource Shelf Interface。 */
export default function NewTaskRoute() {
  const office = activeTaskOffice();
  if (!office) {
    return (
      <View>
        <Text>Sign in to create a task.</Text>
      </View>
    );
  }
  return <NewTaskRouteLifecycle office={office} />;
}
