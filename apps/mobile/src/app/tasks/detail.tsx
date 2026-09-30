import { useCallback, useEffect, useRef, useState } from 'react';
import { router, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { DeliveryRecoveryError, TaskOfficeError, type DeliveryReceiptView } from '@weknora/mobile-core';
import { activeDeliveryReader, activeDeliveryRecovery, activeTaskOffice } from '../../composition.ts';
import { createTaskDetailController, TASK_OFFICE_ERROR_COPY, type TaskDetailController, type TaskDetailViewState } from '../../task-detail-view.ts';
import { DELIVERY_RECOVERY_COPY, TaskDetailScreen } from '../../screens/TaskDetailScreen.tsx';

type DeliveryRouteIdentity = Readonly<{ taskId: string; runId: string }>;
type DeliveryRouteEntry = Readonly<{ identity: DeliveryRouteIdentity; generation: number }>;
type DeliveryRecoveryInput = { runId: string; deliveryId: string };

function sameDeliveryRoute(left: DeliveryRouteIdentity, right: DeliveryRouteIdentity): boolean {
  return left.taskId === right.taskId && left.runId === right.runId;
}

function sameDeliveryRouteEntry(left: DeliveryRouteEntry, right: DeliveryRouteEntry): boolean {
  return left.generation === right.generation && sameDeliveryRoute(left.identity, right.identity);
}

/** Delivery reads are accepted only for the route entry that initiated them. */
export function createRouteBoundDeliveryReadHandler(options: {
  entry: DeliveryRouteEntry;
  currentEntry(): DeliveryRouteEntry;
  setDelivery(receipt: DeliveryReceiptView): void;
}): (receipt: DeliveryReceiptView | undefined) => void {
  return (receipt) => {
    const { entry } = options;
    if (receipt === undefined
      || !sameDeliveryRouteEntry(options.currentEntry(), entry)
      || receipt.taskId !== entry.identity.taskId
      || receipt.runId !== entry.identity.runId) return;
    options.setDelivery(receipt);
  };
}

/** Route-bound async seam: results may update presentation only while their originating route is current. */
export function createRouteBoundDeliveryRecoveryHandler(options: {
  entry: DeliveryRouteEntry;
  receipt: DeliveryReceiptView;
  currentEntry(): DeliveryRouteEntry;
  recover(input: DeliveryRecoveryInput): Promise<DeliveryReceiptView>;
  setDelivery(receipt: DeliveryReceiptView): void;
  setRecoveryError(error?: string): void;
  errorCopy(error: unknown): string;
}): (input: DeliveryRecoveryInput) => Promise<DeliveryReceiptView> {
  return (input) => {
    const { entry, receipt } = options;
    if (!sameDeliveryRouteEntry(options.currentEntry(), entry)
      || receipt.taskId !== entry.identity.taskId
      || receipt.runId !== entry.identity.runId
      || input.runId !== entry.identity.runId
      || input.deliveryId !== receipt.deliveryId) {
      return Promise.reject(new Error('delivery recovery route changed'));
    }
    options.setRecoveryError(undefined);
    return options.recover(input).then((recovered) => {
      if (!sameDeliveryRouteEntry(options.currentEntry(), entry)) return recovered;
      if (recovered.taskId !== entry.identity.taskId || recovered.runId !== entry.identity.runId) {
        throw new Error('delivery recovery response does not match the current route');
      }
      options.setDelivery(recovered);
      return recovered;
    }).catch((error: unknown) => {
      if (sameDeliveryRouteEntry(options.currentEntry(), entry)) options.setRecoveryError(options.errorCopy(error));
      throw error;
    });
  };
}

interface RouteDeliveryReceipt {
  entry: DeliveryRouteEntry;
  receipt: DeliveryReceiptView;
}

interface RouteDeliveryError {
  entry: DeliveryRouteEntry;
  message: string;
}

/** /tasks/detail 挂载生命周期宿主：handle 在 effect 内创建、卸载即 dispose——与 /resources 同一模式。 */
export function TaskDetailRouteLifecycle({ taskId, runId, onOpenMaterials, onOpenBudget, onOpenVoiceRoom }: { taskId: string; runId: string; onOpenMaterials?: () => void; onOpenBudget?: () => void; onOpenVoiceRoom?: () => void }) {
  const [state, setState] = useState<TaskDetailViewState>({ loading: true });
  const [deliveryState, setDeliveryState] = useState<RouteDeliveryReceipt | undefined>(undefined);
  const [recoveryErrorState, setRecoveryErrorState] = useState<RouteDeliveryError | undefined>(undefined);
  const identity: DeliveryRouteIdentity = { taskId, runId };
  const routeEntryRef = useRef<DeliveryRouteEntry>({ identity, generation: 0 });
  // A new task/run pair starts a new entry; returning to an old pair increments again.
  if (!sameDeliveryRoute(routeEntryRef.current.identity, identity)) {
    routeEntryRef.current = { identity, generation: routeEntryRef.current.generation + 1 };
  }
  const routeEntry = routeEntryRef.current;
  const controllerRef = useRef<TaskDetailController | undefined>(undefined);
  // Stack may retain this route while another screen is pushed. Every focus gets
  // a distinct entry; blur invalidates it synchronously before stale promises settle.
  const onFocus = useCallback(() => {
    const focusedIdentity: DeliveryRouteIdentity = { taskId, runId };
    const focusedEntry: DeliveryRouteEntry = {
      identity: focusedIdentity,
      generation: routeEntryRef.current.generation + 1,
    };
    routeEntryRef.current = focusedEntry;
    setDeliveryState(undefined);
    setRecoveryErrorState(undefined);
    let cancelled = false;
    const reader = activeDeliveryReader();
    if (reader !== undefined) {
      const applyRead = createRouteBoundDeliveryReadHandler({
        entry: focusedEntry,
        currentEntry: () => routeEntryRef.current,
        setDelivery: (receipt) => setDeliveryState({ entry: focusedEntry, receipt }),
      });
      reader.read(runId)
        .then((receipt) => { if (!cancelled) applyRead(receipt); })
        .catch(() => { /* 交付区块缺失是合法空态（无交付/未登录），不阻塞详情 */ });
    }
    return () => {
      cancelled = true;
      if (sameDeliveryRouteEntry(routeEntryRef.current, focusedEntry)) {
        routeEntryRef.current = { identity: focusedIdentity, generation: focusedEntry.generation + 1 };
      }
    };
  }, [taskId, runId]);
  useFocusEffect(onFocus);
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
    && sameDeliveryRouteEntry(deliveryState.entry, routeEntry)
    && deliveryState.receipt.taskId === taskId
    && deliveryState.receipt.runId === runId
    ? deliveryState.receipt
    : undefined;
  const recoveryError = recoveryErrorState !== undefined && sameDeliveryRouteEntry(recoveryErrorState.entry, routeEntry)
    ? recoveryErrorState.message
    : undefined;
  const recovery = delivery === undefined ? undefined : activeDeliveryRecovery();
  const onRecoverDelivery = recovery === undefined || delivery === undefined ? undefined : createRouteBoundDeliveryRecoveryHandler({
    entry: routeEntry,
    receipt: delivery,
    currentEntry: () => routeEntryRef.current,
    recover: (input) => recovery.recover(input),
    setDelivery: (receipt) => setDeliveryState({ entry: routeEntry, receipt }),
    setRecoveryError: (message) => setRecoveryErrorState(message === undefined ? undefined : { entry: routeEntry, message }),
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
