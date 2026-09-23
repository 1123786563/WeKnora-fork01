import { TaskOfficeError } from '@weknora/mobile-core';
import type { TaskDetailView, TaskHandle } from '@weknora/mobile-core';

export interface TaskDetailViewState {
  view?: TaskDetailView;
  loading: boolean;
  error?: string;
}

/** TaskOfficeError 错误码 → 用户文案（TaskOfficeError 的 message 即裸错误码，B2-F16）。 */
export const TASK_OFFICE_ERROR_COPY: Record<string, string> = {
  TASK_OFFICE_INVALID_INPUT: '任务参数缺失（taskId/runId），请从任务列表重新进入。',
  TASK_OFFICE_DETAIL_UNAVAILABLE: '当前部署未提供任务详情通道。',
  TASK_OFFICE_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
  TASK_OFFICE_BACKEND: '服务端暂时不可用，请稍后重试。',
  TASK_OFFICE_DETAIL_CLOSED: '该任务详情已关闭。',
};

const messageOf = (failure: unknown): string => {
  if (failure instanceof TaskOfficeError) return TASK_OFFICE_ERROR_COPY[failure.code] ?? failure.code;
  return failure instanceof Error ? failure.message : String(failure);
};

export interface TaskDetailController {
  state(): TaskDetailViewState;
  subscribe(listener: (state: TaskDetailViewState) => void): () => void;
  refresh(): Promise<void>;
  whenSettled(): Promise<void>;
  dispose(): void;
}

/** 详情页控制器：hydrate 驱动首帧，updates 驱动增量，refresh 走显式 resync；dispose 摘除订阅并关闭句柄。 */
export function createTaskDetailController(handle: TaskHandle): TaskDetailController {
  let state: TaskDetailViewState = { loading: true };
  let disposed = false;
  const listeners = new Set<(state: TaskDetailViewState) => void>();
  const publish = (next: TaskDetailViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const unsubscribe = handle.updates((view) => { if (!disposed) publish({ view, loading: false }); });
  const tail: Promise<void> = handle.hydrate().then(
    (view) => { if (!disposed) publish({ view, loading: false }); },
    (failure: unknown) => {
      if (!disposed) publish({ view: undefined, loading: false, error: messageOf(failure) });
    },
  );
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    refresh(): Promise<void> {
      publish({ ...state, loading: true, error: undefined });
      const attempt = handle.resync().then(
        (view) => { if (!disposed) publish({ view, loading: false }); },
        (failure: unknown) => {
          if (!disposed) publish({ view: handle.view(), loading: false, error: messageOf(failure) });
        },
      );
      return attempt;
    },
    whenSettled: () => tail,
    dispose() {
      if (disposed) return;
      disposed = true;
      unsubscribe();
      listeners.clear();
      handle.close('controller-disposed');
    },
  };
}
