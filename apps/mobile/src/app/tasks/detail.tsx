import { useEffect, useRef, useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { TaskOfficeError, type DeliveryReceiptView } from '@weknora/mobile-core';
import { activeDeliveryReader, activeTaskOffice } from '../../composition.ts';
import { createTaskDetailController, TASK_OFFICE_ERROR_COPY, type TaskDetailController, type TaskDetailViewState } from '../../task-detail-view.ts';
import { TaskDetailScreen } from '../../screens/TaskDetailScreen.tsx';

/** /tasks/detail 挂载生命周期宿主：handle 在 effect 内创建、卸载即 dispose——与 /resources 同一模式。 */
export function TaskDetailRouteLifecycle({ taskId, runId, onOpenMaterials }: { taskId: string; runId: string; onOpenMaterials?: () => void }) {
  const [state, setState] = useState<TaskDetailViewState>({ loading: true });
  const [delivery, setDelivery] = useState<DeliveryReceiptView | undefined>(undefined);
  const controllerRef = useRef<TaskDetailController | undefined>(undefined);
  // 交付回执读一次（不阻塞详情渲染；失败静默——交付区块缺失是合法空态）。
  // 刷新路径 onRefresh 不拉交付：回执不因刷新而变，重进页面即重读。
  useEffect(() => {
    const reader = activeDeliveryReader();
    if (reader === undefined) return;
    let cancelled = false;
    reader.read(runId)
      .then((view) => { if (!cancelled) setDelivery(view); })
      .catch(() => { /* 交付区块缺失是合法空态（无交付/未登录），不阻塞详情 */ });
    return () => { cancelled = true; };
  }, [runId]);
  useEffect(() => {
    let controller: TaskDetailController | undefined;
    try {
      const handle = activeTaskOffice()?.open({ taskId, runId });
      controller = handle === undefined ? undefined : createTaskDetailController(handle);
    } catch (error) {
      // 按错误码分流（B2-F13）：缺参（INVALID_INPUT）与详情端口缺失（DETAIL_UNAVAILABLE）时用户
      // 往往已授权，不得折叠为「请先登录」；其余未知异常保持既有登录引导兜底。
      controller = undefined;
      const fallback = error instanceof TaskOfficeError ? (TASK_OFFICE_ERROR_COPY[error.code] ?? error.code) : '请先登录并激活空间，再打开任务详情。';
      setState({ loading: false, error: fallback });
      return;
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
  return <TaskDetailScreen view={state.view} loading={state.loading} error={state.error} onRefresh={() => { void controllerRef.current?.refresh(); }} onOpenMaterials={onOpenMaterials} delivery={delivery} />;
}

/** Expo Router 文件路由：/tasks/detail?taskId=..&runId=..。只消费 Task Office Interface。 */
export default function TaskDetailRoute() {
  const params = useLocalSearchParams<{ taskId?: string; runId?: string }>();
  return (
    <TaskDetailRouteLifecycle
      taskId={String(params.taskId ?? '')}
      runId={String(params.runId ?? '')}
      onOpenMaterials={() => { router.push({ pathname: '/tasks/materials', params: { runId: String(params.runId ?? '') } }); }}
    />
  );
}
