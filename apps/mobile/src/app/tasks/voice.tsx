import { useEffect, useRef, useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { TaskOfficeError } from '@weknora/mobile-core';
import { activeTaskOffice, activeVoiceRoom } from '../../composition.ts';
import { createTaskDetailController, TASK_OFFICE_ERROR_COPY, type TaskDetailController } from '../../task-detail-view.ts';
import { createVoiceRoomController, type VoiceRoomController, type VoiceRoomViewState } from '../../voice-room-view.ts';
import { VoiceRoomScreen } from '../../screens/VoiceRoomScreen.tsx';

/** /tasks/voice 挂载生命周期宿主：语音房句柄与 Task 详情句柄都在 effect 内创建、卸载即
 * dispose（dispose → leave()：服务端 stop/settle 明确结束）。确认文字经同一 Task Office
 * 的 act 通道提交（spec §8.1）；语音模块本身零提交。 */
export function VoiceRoomRouteLifecycle({ taskId, runId }: { taskId: string; runId?: string }) {
  const [state, setState] = useState<VoiceRoomViewState>({ phase: 'idle', taskId, turns: [] });
  const controllerRef = useRef<VoiceRoomController | undefined>(undefined);
  const detailRef = useRef<TaskDetailController | undefined>(undefined);
  useEffect(() => {
    let voiceController: VoiceRoomController | undefined;
    let detailController: TaskDetailController | undefined;
    try {
      const room = activeVoiceRoom();
      const office = activeTaskOffice();
      if (room === undefined || office === undefined) {
        setState({ phase: 'idle', taskId, turns: [], lastSubmitError: room === undefined ? '当前部署或设备不支持语音房（需要麦克风与授权通道）。' : '请先登录并激活空间。' });
        return;
      }
      const voiceHandle = room.join({ taskId, ...(runId === undefined ? {} : { runId }) });
      const detailHandle = office.open({ taskId, runId: runId ?? taskId });
      detailController = createTaskDetailController(detailHandle);
      detailRef.current = detailController;
      voiceController = createVoiceRoomController({
        handle: voiceHandle,
        onConfirmIntent: (intent) => detailController!.act(intent),
      });
      controllerRef.current = voiceController;
      setState(voiceController.state());
      const unsubscribe = voiceController.subscribe(setState);
      return () => {
        unsubscribe();
        voiceController?.dispose();
        detailController?.dispose();
        controllerRef.current = undefined;
        detailRef.current = undefined;
      };
    } catch (error) {
      const fallback = error instanceof TaskOfficeError ? (TASK_OFFICE_ERROR_COPY[error.code] ?? error.code) : '语音房不可用：请从任务详情重新进入。';
      setState({ phase: 'idle', taskId, turns: [], lastSubmitError: fallback });
      return;
    }
  }, [taskId, runId]);
  return (
    <VoiceRoomScreen
      state={state}
      onBeginTurn={() => { void controllerRef.current?.beginTurn(); }}
      onEndTurn={() => { void controllerRef.current?.endTurn(); }}
      onEditTranscript={(text) => { controllerRef.current?.editTranscript(text); }}
      onConfirmTranscript={() => { void controllerRef.current?.confirmTranscript(); }}
      onRetrySubmit={() => { void controllerRef.current?.retrySubmit(); }}
      onDiscardTurn={() => { controllerRef.current?.discardTurn(); }}
      onResume={() => { void controllerRef.current?.resume(); }}
      onLeave={() => { void controllerRef.current?.leave().then(() => { router.back(); }, () => { router.back(); }); }}
    />
  );
}

/** Expo Router 文件路由：/tasks/voice?taskId=..&runId=..。只消费 Voice Room + Task Office Interface。 */
export default function VoiceRoomRoute() {
  const params = useLocalSearchParams<{ taskId?: string; runId?: string }>();
  const taskId = String(params.taskId ?? '');
  const runId = params.runId === undefined ? undefined : String(params.runId);
  // 缺 taskId 的深链：文案归因于缺参数（引导从任务详情页进入），不误报为登录问题。
  if (taskId === '') {
    return (
      <VoiceRoomScreen
        state={{ phase: 'idle', taskId: '', turns: [], lastSubmitError: '链接缺少 taskId：语音房需从任务详情页进入。' }}
      />
    );
  }
  return <VoiceRoomRouteLifecycle taskId={taskId} runId={runId} />;
}
