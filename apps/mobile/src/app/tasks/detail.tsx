import { useEffect, useRef, useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { DeliveryRecoveryError, TaskOfficeError, type DeliveryReceiptView } from '@weknora/mobile-core';
import { activeDeliveryReader, activeDeliveryRecovery, activeTaskOffice } from '../../composition.ts';
import { createTaskDetailController, TASK_OFFICE_ERROR_COPY, type TaskDetailController, type TaskDetailViewState } from '../../task-detail-view.ts';
import { DELIVERY_RECOVERY_COPY, TaskDetailScreen } from '../../screens/TaskDetailScreen.tsx';

type DeliveryRouteIdentity = Readonly<{ taskId: string; runId: string }>;
type DeliveryRecoveryInput = { runId: string; deliveryId: string };

function sameDeliveryRoute(left: DeliveryRouteIdentity, right: DeliveryRouteIdentity): boolean {
  return left.taskId === right.taskId && left.runId === right.runId;
}

/** Route-bound async seam: results may update presentation only while their originating route is current. */
export function createRouteBoundDeliveryRecoveryHandler(options: {
  identity: DeliveryRouteIdentity;
  receipt: DeliveryReceiptView;
  currentIdentity(): DeliveryRouteIdentity;
  recover(input: DeliveryRecoveryInput): Promise<DeliveryReceiptView>;
  setDelivery(receipt: DeliveryReceiptView): void;
  setRecoveryError(error?: string): void;
  errorCopy(error: unknown): string;
}): (input: DeliveryRecoveryInput) => Promise<DeliveryReceiptView> {
  return (input) => {
    const { identity, receipt } = options;
    if (!sameDeliveryRoute(options.currentIdentity(), identity)
      || receipt.taskId !== identity.taskId
      || receipt.runId !== identity.runId
      || input.runId !== identity.runId
      || input.deliveryId !== receipt.deliveryId) {
      return Promise.reject(new Error('delivery recovery route changed'));
    }
    options.setRecoveryError(undefined);
    return options.recover(input).then((recovered) => {
      if (!sameDeliveryRoute(options.currentIdentity(), identity)) return recovered;
      if (recovered.taskId !== identity.taskId || recovered.runId !== identity.runId) {
        throw new Error('delivery recovery response does not match the current route');
      }
      options.setDelivery(recovered);
      return recovered;
    }).catch((error: unknown) => {
      if (sameDeliveryRoute(options.currentIdentity(), identity)) options.setRecoveryError(options.errorCopy(error));
      throw error;
    });
  };
}

interface RouteDeliveryReceipt {
  identity: DeliveryRouteIdentity;
  receipt: DeliveryReceiptView;
}

interface RouteDeliveryError {
  identity: DeliveryRouteIdentity;
  message: string;
}

/** /tasks/detail 挂载生命周期宿主：handle 在 effect 内创建、卸载即 dispose——与 /resources 同一模式。 */
export function TaskDetailRouteLifecycle({ taskId, runId, onOpenMaterials, onOpenBudget, onOpenVoiceRoom }: { taskId: string; runId: string; onOpenMaterials?: () => void; onOpenBudget?: () => void; onOpenVoiceRoom?: () => void }) {
  const [state, setState] = useState<TaskDetailViewState>({ loading: true });
  const [deliveryState, setDeliveryState] = useState<RouteDeliveryReceipt | undefined>(undefined);
  const [recoveryErrorState, setRecoveryErrorState] = useState<RouteDeliveryError | undefined>(undefined);
  const identity: DeliveryRouteIdentity = { taskId, runId };
  const identityRef = useRef(identity);
  // Update during render so a completion in the render-to-effect interval already sees the new route.
  identityRef.current = identity;
  const controllerRef = useRef<TaskDetailController | undefined>(undefined);
  // 交付回执读一次（不阻塞详情渲染；失败静默——交付区块缺失是合法空态）。
  // 刷新路径 onRefresh 不拉交付：回执不因刷新而变，重进页面即重读。
  useEffect(() => {
    setDeliveryState(undefined);
    setRecoveryErrorState(undefined);
    const reader = activeDeliveryReader();
    if (reader === undefined) return;
    let cancelled = false;
    reader.read(runId)
      .then((receipt) => {
        if (cancelled || !sameDeliveryRoute(identityRef.current, identity)) return;
        if (receipt === undefined) return;
        if (receipt.taskId !== identity.taskId || receipt.runId !== identity.runId) return;
        setDeliveryState({ identity, receipt });
      })
      .catch(() => { /* 交付区块缺失是合法空态（无交付/未登录），不阻塞详情 */ });
    return () => { cancelled = true; };
  }, [taskId, runId]);
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
  const delivery = deliveryState !== undefined
    && sameDeliveryRoute(deliveryState.identity, identity)
    && deliveryState.receipt.taskId === taskId
    && deliveryState.receipt.runId === runId
    ? deliveryState.receipt
    : undefined;
  const recoveryError = recoveryErrorState !== undefined && sameDeliveryRoute(recoveryErrorState.identity, identity)
    ? recoveryErrorState.message
    : undefined;
  const recovery = delivery === undefined ? undefined : activeDeliveryRecovery();
  const onRecoverDelivery = recovery === undefined || delivery === undefined ? undefined : createRouteBoundDeliveryRecoveryHandler({
    identity,
    receipt: delivery,
    currentIdentity: () => identityRef.current,
    recover: (input) => recovery.recover(input),
    setDelivery: (receipt) => setDeliveryState({ identity, receipt }),
    setRecoveryError: (message) => setRecoveryErrorState(message === undefined ? undefined : { identity, message }),
    errorCopy: (error) => error instanceof DeliveryRecoveryError
      ? DELIVERY_RECOVERY_COPY.failed + (error.message ? `（${error.message}）` : '')
      : '恢复请求失败，请稍后重试',
  });
  return <TaskDetailScreen view={state.view} loading={state.loading} error={state.error} onRefresh={() => { void controllerRef.current?.refresh(); }} onOpenMaterials={onOpenMaterials} onOpenBudget={onOpenBudget} onOpenVoiceRoom={onOpenVoiceRoom} delivery={delivery} recoveryError={recoveryError} onRecoverDelivery={onRecoverDelivery} onAct={controllerRef.current === undefined ? undefined : (intent) => controllerRef.current!.act(intent)} />;
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
