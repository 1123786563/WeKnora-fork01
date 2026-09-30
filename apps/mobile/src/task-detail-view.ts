import type { TaskDetailView, TaskHandle } from '@weknora/mobile-core';

export interface TaskDetailViewState {
  view?: TaskDetailView;
  loading: boolean;
  error?: string;
}

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
  const messageOf = (failure: unknown): string => (failure instanceof Error ? failure.message : String(failure));
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
