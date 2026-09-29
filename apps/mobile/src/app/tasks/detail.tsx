import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { DeliveryRecoveryError, TaskOfficeError, type DeliveryReceiptView, type DeliveryReader, type DeliveryRecovery, type MobileRuntime, type RuntimeSnapshot, type ScopeLease } from '@weknora/mobile-core';
import { activeDeliveryReader, activeDeliveryRecovery, activeMobileRuntime, activeTaskOffice } from '../../composition.ts';
import { createTaskDetailController, TASK_OFFICE_ERROR_COPY, type TaskDetailController, type TaskDetailViewState } from '../../task-detail-view.ts';
import type { TaskOffice } from '@weknora/mobile-core';
import { DELIVERY_RECOVERY_COPY, TaskDetailScreen } from '../../screens/TaskDetailScreen.tsx';

type RuntimePort = Pick<MobileRuntime, 'snapshot' | 'subscribe' | 'scopeLease'>;
export interface TaskDetailRouteIdentity {
  deploymentOrigin: string;
  userId: string;
  tenantId: string;
  lease: ScopeLease;
  taskId: string;
  runId: string;
}

export function taskDetailRouteIdentity(snapshot: RuntimeSnapshot, lease: ScopeLease | undefined, taskId: string, runId: string): TaskDetailRouteIdentity | undefined {
  if (snapshot.surface !== 'authorized' || !snapshot.deployment?.origin || !snapshot.identity?.userId || !snapshot.identity.activeTenantId || lease === undefined) return undefined;
  return { deploymentOrigin: snapshot.deployment.origin, userId: snapshot.identity.userId, tenantId: snapshot.identity.activeTenantId, lease, taskId, runId };
}

function sameTaskDetailRouteIdentity(left: TaskDetailRouteIdentity | undefined, right: TaskDetailRouteIdentity | undefined): boolean {
  return left === right || (left !== undefined && right !== undefined && left.deploymentOrigin === right.deploymentOrigin && left.userId === right.userId && left.tenantId === right.tenantId && left.lease === right.lease && left.taskId === right.taskId && left.runId === right.runId);
}
const SCOPE_CHANGED_COPY = '登录状态或活动空间已变化，请重新进入任务详情后重试';

/** Recovery action is bound to the run and authorized module captured by its render. */
export function createDeliveryRecoveryAction(options: {
  runId: string;
  capturedRecovery: DeliveryRecovery;
  currentRecovery(): DeliveryRecovery | undefined;
  capturedIdentity?: TaskDetailRouteIdentity;
  currentIdentity?(): TaskDetailRouteIdentity | undefined;
  isCurrentRun(): boolean;
  setDelivery(view: DeliveryReceiptView): void;
  clearError(): void;
  setError(message: string): void;
  setCurrentScopeError?(): void;
}): (input: { runId: string; deliveryId: string }) => Promise<DeliveryReceiptView> {
  return async (input) => {
    if (!options.isCurrentRun() || input.runId !== options.runId) {
      options.setError('任务已切换，请重新进入详情后重试');
      throw new Error('task detail run changed');
    }
    options.clearError();
    if (options.currentRecovery() !== options.capturedRecovery || !sameTaskDetailRouteIdentity(options.currentIdentity?.(), options.capturedIdentity)) {
      if (options.setCurrentScopeError !== undefined) options.setCurrentScopeError();
      else options.setError(SCOPE_CHANGED_COPY);
      throw new Error('delivery recovery authorization changed');
    }
    try {
      const view = await options.capturedRecovery.recover(input);
      if (options.isCurrentRun() && options.currentRecovery() === options.capturedRecovery && sameTaskDetailRouteIdentity(options.currentIdentity?.(), options.capturedIdentity)) options.setDelivery(view);
      return view;
    } catch (error: unknown) {
      if (!options.isCurrentRun()) throw error;
      if (options.currentRecovery() !== options.capturedRecovery || !sameTaskDetailRouteIdentity(options.currentIdentity?.(), options.capturedIdentity)) {
        if (options.setCurrentScopeError !== undefined) options.setCurrentScopeError();
        else options.setError(SCOPE_CHANGED_COPY);
      } else {
        options.setError(error instanceof DeliveryRecoveryError ? (DELIVERY_RECOVERY_COPY.failed + (error.message ? `（${error.message}）` : '')) : '恢复请求失败，请稍后重试');
      }
      throw error;
    }
  };
}

export function deliveryForRoute(delivery: { identity: TaskDetailRouteIdentity; view: DeliveryReceiptView } | undefined, identity: TaskDetailRouteIdentity | undefined): DeliveryReceiptView | undefined {
  return identity !== undefined && delivery !== undefined && sameTaskDetailRouteIdentity(delivery.identity, identity) && delivery.view.taskId === identity.taskId && delivery.view.runId === identity.runId ? delivery.view : undefined;
}

export function recoveryErrorForRoute(issue: { identity: TaskDetailRouteIdentity; message: string } | undefined, identity: TaskDetailRouteIdentity | undefined): string | undefined {
  return identity !== undefined && issue !== undefined && sameTaskDetailRouteIdentity(issue.identity, identity) ? issue.message : undefined;
}

/** /tasks/detail 挂载生命周期宿主：handle 在 effect 内创建、卸载即 dispose——与 /resources 同一模式。 */
export function TaskDetailRouteLifecycle({ taskId, runId, onOpenMaterials, onOpenBudget, onOpenVoiceRoom, runtime: injectedRuntime, recoveryProvider, deliveryReader, taskOfficeProvider }: { taskId: string; runId: string; onOpenMaterials?: () => void; onOpenBudget?: () => void; onOpenVoiceRoom?: () => void; runtime?: RuntimePort; recoveryProvider?: () => DeliveryRecovery | undefined; deliveryReader?: () => DeliveryReader | undefined; taskOfficeProvider?: () => TaskOffice | undefined }) {
  const runtime = injectedRuntime ?? activeMobileRuntime();
  const snapshot = useSyncExternalStore((notify) => runtime.subscribe(() => notify()), () => runtime.snapshot(), () => runtime.snapshot());
  const currentRouteRef = useRef({ taskId, runId });
  if (currentRouteRef.current.taskId !== taskId || currentRouteRef.current.runId !== runId) currentRouteRef.current = { taskId, runId };
  const currentRouteIdentity = () => taskDetailRouteIdentity(snapshot, runtime.scopeLease(), taskId, runId);
  const currentIdentity = currentRouteIdentity();
  const currentIdentityRef = useRef<TaskDetailRouteIdentity | undefined>(currentIdentity);
  if (!sameTaskDetailRouteIdentity(currentIdentityRef.current, currentIdentity)) currentIdentityRef.current = currentIdentity;
  const routeIdentity = currentIdentityRef.current;
  const [state, setState] = useState<{ identity: TaskDetailRouteIdentity | undefined; value: TaskDetailViewState }>({ identity: routeIdentity, value: { loading: true } });
  const [delivery, setDelivery] = useState<{ identity: TaskDetailRouteIdentity; view: DeliveryReceiptView } | undefined>(undefined);
  const [recoveryIssue, setRecoveryIssue] = useState<{ identity: TaskDetailRouteIdentity; message: string } | undefined>(undefined);
  const controllerRef = useRef<TaskDetailController | undefined>(undefined);
  // 交付回执读一次（不阻塞详情渲染；失败静默——交付区块缺失是合法空态）。
  // 刷新路径 onRefresh 不拉交付：回执不因刷新而变，重进页面即重读。
  useEffect(() => {
    setState({ identity: routeIdentity, value: { loading: true } });
    setDelivery(undefined);
    setRecoveryIssue(undefined);
  }, [taskId, runId, routeIdentity]);
  useEffect(() => {
    const reader = deliveryReader?.() ?? (injectedRuntime === undefined ? activeDeliveryReader() : undefined);
    if (reader === undefined || routeIdentity === undefined) return;
    let cancelled = false;
    reader.read(runId)
      .then((view) => { if (!cancelled && view !== undefined && sameTaskDetailRouteIdentity(currentIdentityRef.current, routeIdentity)) setDelivery({ identity: routeIdentity, view }); })
      .catch(() => { /* 交付区块缺失是合法空态（无交付/未登录），不阻塞详情 */ });
    return () => { cancelled = true; };
  }, [runId, taskId, routeIdentity]);
  useEffect(() => {
    if (routeIdentity === undefined) {
      setState({ identity: undefined, value: { loading: false, error: '请先登录并激活空间，再打开任务详情。' } });
      return;
    }
    let controller: TaskDetailController | undefined;
    try {
      const taskOffice = taskOfficeProvider?.() ?? (injectedRuntime === undefined ? activeTaskOffice() : undefined);
      const handle = taskOffice?.open({ taskId, runId });
      controller = handle === undefined ? undefined : createTaskDetailController(handle);
    } catch (error) {
      // 按错误码分流（B2-F13）：缺参（INVALID_INPUT）与详情端口缺失（DETAIL_UNAVAILABLE）时用户
      // 往往已授权，不得折叠为「请先登录」；其余未知异常保持既有登录引导兜底。
      controller = undefined;
      const fallback = error instanceof TaskOfficeError ? (TASK_OFFICE_ERROR_COPY[error.code] ?? error.code) : '请先登录并激活空间，再打开任务详情。';
      setState({ identity: routeIdentity, value: { loading: false, error: fallback } });
      return;
    }
    if (controller === undefined) {
      setState({ identity: routeIdentity, value: { loading: false, error: '请先登录并激活空间，再打开任务详情。' } });
      return;
    }
    controllerRef.current = controller;
    setState({ identity: routeIdentity, value: controller.state() });
    const unsubscribe = controller.subscribe((value) => {
      if (sameTaskDetailRouteIdentity(currentIdentityRef.current, routeIdentity)) setState({ identity: routeIdentity, value });
    });
    return () => {
      unsubscribe();
      controller?.dispose();
      controllerRef.current = undefined;
    };
  }, [taskId, runId, routeIdentity]);
  const getRecovery = recoveryProvider ?? activeDeliveryRecovery;
  const activeRecovery = routeIdentity === undefined ? undefined : getRecovery();
  const onRecoverDelivery = activeRecovery === undefined ? undefined : createDeliveryRecoveryAction({
    runId, capturedRecovery: activeRecovery, currentRecovery: getRecovery,
    capturedIdentity: routeIdentity,
    currentIdentity: () => taskDetailRouteIdentity(runtime.snapshot(), runtime.scopeLease(), taskId, runId),
    isCurrentRun: () => currentRouteRef.current.taskId === taskId && currentRouteRef.current.runId === runId,
    setDelivery: (view) => { if (routeIdentity !== undefined) setDelivery({ identity: routeIdentity, view }); },
    clearError: () => setRecoveryIssue(undefined),
    setError: (message) => { if (routeIdentity !== undefined) setRecoveryIssue({ identity: routeIdentity, message }); },
    setCurrentScopeError: () => {
      const identity = taskDetailRouteIdentity(runtime.snapshot(), runtime.scopeLease(), taskId, runId);
      setState({ identity, value: { loading: false, error: SCOPE_CHANGED_COPY } });
    },
  });
  const visibleState = sameTaskDetailRouteIdentity(state.identity, routeIdentity) ? state.value : { loading: true };
  return <TaskDetailScreen view={visibleState.view} loading={visibleState.loading} error={visibleState.error} onRefresh={() => { void controllerRef.current?.refresh(); }} onOpenMaterials={onOpenMaterials} onOpenBudget={onOpenBudget} onOpenVoiceRoom={onOpenVoiceRoom} delivery={deliveryForRoute(delivery, routeIdentity)} recoveryError={recoveryErrorForRoute(recoveryIssue, routeIdentity)} onRecoverDelivery={onRecoverDelivery} onAct={controllerRef.current === undefined ? undefined : (intent) => controllerRef.current!.act(intent)} />;
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
