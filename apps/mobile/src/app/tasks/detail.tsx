import { useEffect, useRef, useState } from 'react';
import { useLocalSearchParams } from 'expo-router';
import { activeTaskOffice } from '../../composition.ts';
import { createTaskDetailController, type TaskDetailController, type TaskDetailViewState } from '../../task-detail-view.ts';
import { TaskDetailScreen } from '../../screens/TaskDetailScreen.tsx';

/** /tasks/detail 挂载生命周期宿主：handle 在 effect 内创建、卸载即 dispose——与 /resources 同一模式。 */
export function TaskDetailRouteLifecycle({ taskId, runId }: { taskId: string; runId: string }) {
  const [state, setState] = useState<TaskDetailViewState>({ loading: true });
  const controllerRef = useRef<TaskDetailController | undefined>(undefined);
  useEffect(() => {
    let controller: TaskDetailController | undefined;
    try {
      const handle = activeTaskOffice()?.open({ taskId, runId });
      controller = handle === undefined ? undefined : createTaskDetailController(handle);
    } catch {
      controller = undefined;
    }
    if (controller === undefined) {
      setState({ loading: false, error: '请先登录并激活空间，再打开任务详情。' });
      return;
    }
    controllerRef.current = controller;
    setState(controller.state());
    const unsubscribe = controller.subscribe(setState);
    return () => {
      unsubscribe();
      controller?.dispose();
      controllerRef.current = undefined;
    };
  }, [taskId, runId]);
  return <TaskDetailScreen view={state.view} loading={state.loading} error={state.error} onRefresh={() => { void controllerRef.current?.refresh(); }} />;
}

/** Expo Router 文件路由：/tasks/detail?taskId=..&runId=..。只消费 Task Office Interface。 */
export default function TaskDetailRoute() {
  const params = useLocalSearchParams<{ taskId?: string; runId?: string }>();
  return <TaskDetailRouteLifecycle taskId={String(params.taskId ?? '')} runId={String(params.runId ?? '')} />;
}
