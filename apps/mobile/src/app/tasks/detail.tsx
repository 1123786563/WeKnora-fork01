import { useEffect, useRef, useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { DeliveryRecoveryError, TaskOfficeError, type DeliveryReceiptView, type DeliveryRecovery } from '@weknora/mobile-core';
import { activeDeliveryReader, activeDeliveryRecovery, activeTaskOffice } from '../../composition.ts';
import { createTaskDetailController, TASK_OFFICE_ERROR_COPY, type TaskDetailController, type TaskDetailViewState } from '../../task-detail-view.ts';
import { DELIVERY_RECOVERY_COPY, TaskDetailScreen } from '../../screens/TaskDetailScreen.tsx';

/** Recovery action is bound to the run and authorized module captured by its render. */
export function createDeliveryRecoveryAction(options: {
  runId: string;
  capturedRecovery: DeliveryRecovery;
  currentRecovery(): DeliveryRecovery | undefined;
  isCurrentRun(): boolean;
  setDelivery(view: DeliveryReceiptView): void;
  clearError(): void;
  setError(message: string): void;
}): (input: { runId: string; deliveryId: string }) => Promise<DeliveryReceiptView> {
  return async (input) => {
    if (!options.isCurrentRun() || input.runId !== options.runId) {
      options.setError('任务已切换，请重新进入详情后重试');
      throw new Error('task detail run changed');
    }
    options.clearError();
    if (options.currentRecovery() !== options.capturedRecovery) {
      const message = '授权或活动空间已变化，请重新进入任务详情后重试';
      options.setError(message);
      throw new Error('delivery recovery authorization changed');
    }
    try {
      const view = await options.capturedRecovery.recover(input);
      if (options.isCurrentRun() && options.currentRecovery() === options.capturedRecovery) options.setDelivery(view);
      return view;
    } catch (error: unknown) {
      if (!options.isCurrentRun()) throw error;
      if (options.currentRecovery() !== options.capturedRecovery) {
        options.setError('授权或活动空间已变化，请重新进入任务详情后重试');
      } else {
        options.setError(error instanceof DeliveryRecoveryError ? (DELIVERY_RECOVERY_COPY.failed + (error.message ? `（${error.message}）` : '')) : '恢复请求失败，请稍后重试');
      }
      throw error;
    }
  };
}

export function deliveryForRoute(delivery: DeliveryReceiptView | undefined, taskId: string, runId: string): DeliveryReceiptView | undefined {
  return delivery?.taskId === taskId && delivery.runId === runId ? delivery : undefined;
}

export function recoveryErrorForRoute(issue: { taskId: string; runId: string; message: string } | undefined, taskId: string, runId: string): string | undefined {
  return issue?.taskId === taskId && issue.runId === runId ? issue.message : undefined;
}

/** /tasks/detail 挂载生命周期宿主：handle 在 effect 内创建、卸载即 dispose——与 /resources 同一模式。 */
export function TaskDetailRouteLifecycle({ taskId, runId, onOpenMaterials, onOpenBudget, onOpenVoiceRoom }: { taskId: string; runId: string; onOpenMaterials?: () => void; onOpenBudget?: () => void; onOpenVoiceRoom?: () => void }) {
  const [state, setState] = useState<TaskDetailViewState>({ loading: true });
  const [delivery, setDelivery] = useState<DeliveryReceiptView | undefined>(undefined);
  const [recoveryIssue, setRecoveryIssue] = useState<{ taskId: string; runId: string; message: string } | undefined>(undefined);
  const currentIdentityRef = useRef({ taskId, runId });
  if (currentIdentityRef.current.taskId !== taskId || currentIdentityRef.current.runId !== runId) {
    currentIdentityRef.current = { taskId, runId };
  }
  const routeIdentity = currentIdentityRef.current;
  const controllerRef = useRef<TaskDetailController | undefined>(undefined);
  // 交付回执读一次（不阻塞详情渲染；失败静默——交付区块缺失是合法空态）。
  // 刷新路径 onRefresh 不拉交付：回执不因刷新而变，重进页面即重读。
  useEffect(() => {
    setDelivery(undefined);
    setRecoveryIssue(undefined);
  }, [taskId, runId]);
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
  const activeRecovery = activeDeliveryRecovery();
  const onRecoverDelivery = activeRecovery === undefined ? undefined : createDeliveryRecoveryAction({
    runId, capturedRecovery: activeRecovery, currentRecovery: activeDeliveryRecovery,
    isCurrentRun: () => currentIdentityRef.current === routeIdentity,
    setDelivery,
    clearError: () => setRecoveryIssue(undefined),
    setError: (message) => setRecoveryIssue({ taskId, runId, message }),
  });
  return <TaskDetailScreen view={state.view} loading={state.loading} error={state.error} onRefresh={() => { void controllerRef.current?.refresh(); }} onOpenMaterials={onOpenMaterials} onOpenBudget={onOpenBudget} onOpenVoiceRoom={onOpenVoiceRoom} delivery={deliveryForRoute(delivery, taskId, runId)} recoveryError={recoveryErrorForRoute(recoveryIssue, taskId, runId)} onRecoverDelivery={onRecoverDelivery} onAct={controllerRef.current === undefined ? undefined : (intent) => controllerRef.current!.act(intent)} />;
}

/** Expo Router 文件路由：/tasks/detail?taskId=..&runId=..。只消费 Task Office Interface。 */
export default function TaskDetailRoute() {
  const params = useLocalSearchParams<{ taskId?: string; runId?: string }>();
  return (
    <TaskDetailRouteLifecycle
      taskId={String(params.taskId ?? '')}
      runId={String(params.runId ?? '')}
      onOpenMaterials={() => { router.push({ pathname: '/tasks/materials', params: { runId: String(params.runId ?? '') } }); }}
      onOpenBudget={() => { router.push({ pathname: '/tasks/budget', params: { taskId: String(params.taskId ?? '') } }); }}
      onOpenVoiceRoom={() => { router.push({ pathname: '/tasks/voice', params: { taskId: String(params.taskId ?? ''), runId: String(params.runId ?? '') } }); }}
    />
  );
}
