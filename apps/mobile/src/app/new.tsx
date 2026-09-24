import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import type { AgentOption, KnowledgeResource } from '@weknora/domain/mobile';
import { activeMobileRuntime, activeDictation, activeTaskOffice, openScopedDraftStore } from '../composition.ts';
import { createNativeRequestId } from '../adapters/request-id.ts';
import { createScopedNewTaskDrafts } from '../new-task-drafts.ts';
import { createNewTaskController, type NewTaskController, type NewTaskViewState } from '../new-task-view.ts';
import { NewTaskScreen, GOAL_TEXT_MAX_LENGTH } from '../screens/NewTaskScreen.tsx';
import { applyConfirmedDictation } from '../dictation-view.ts';
import type { DictationState } from '@weknora/mobile-core';

/** /new 的挂载生命周期宿主：controller 与加密草稿在 effect 内创建，卸载时 dispose。 */
export function NewTaskRouteLifecycle({ office }: { office: NonNullable<ReturnType<typeof activeTaskOffice>> }) {
  const runtime = activeMobileRuntime();
  const [state, setState] = useState<NewTaskViewState | undefined>(undefined);
  const [controller, setController] = useState<NewTaskController | undefined>(undefined);
  const dictation = activeDictation();
  const [dictationState, setDictationState] = useState<DictationState | undefined>(dictation?.state());
  useEffect(() => (dictation === undefined ? undefined : dictation.subscribe(setDictationState)), [dictation]);
  // 卸载兜底：录音/转写在途时停止麦克风与在途派发（迟到结果由模块代次守卫丢弃）；
  // review 转写草稿留在记忆化模块实例中，返回 /new 可继续编辑确认（不丢已转写文本）。
  useEffect(() => () => {
    if (dictation === undefined) return;
    const phase = dictation.state().phase;
    if (phase === 'recording' || phase === 'transcribing') void dictation.cancel();
  }, [dictation]);
  useEffect(() => {
    let disposed = false;
    // cleanup 必须能拿到异步创建完成后的 controller：在 effect 作用域持有引用，
    // 而不是读取首帧渲染的 state（那是 undefined 的 stale closure）。
    let createdController: NewTaskController | undefined;
    void (async () => {
      const draftsStore = await openScopedDraftStore();
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
      dictation={dictationState}
      onDictationBegin={() => { if (dictation !== undefined) void dictation.begin(); }}
      onDictationFinish={() => { if (dictation !== undefined) void dictation.finish(); }}
      onDictationCancel={() => { if (dictation !== undefined) void dictation.cancel(); }}
      onDictationEditTranscript={(text) => { dictation?.editTranscript(text); }}
      onDictationRetryTranscription={() => { if (dictation !== undefined) void dictation.retryTranscription(); }}
      onDictationConfirmTranscript={() => {
        if (dictation === undefined) return;
        const confirmed = dictation.confirmTranscript();
        if (confirmed === undefined) return;
        controller.update({ text: applyConfirmedDictation(controller.state().draft.text, confirmed, GOAL_TEXT_MAX_LENGTH) });
      }}
      onDictationDiscardTranscript={() => { dictation?.discardTranscript(); }}
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
