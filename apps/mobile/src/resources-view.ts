import type { ResourcePage, ResourceShelfHandle } from '@weknora/mobile-core';

export interface ResourceShelfViewState {
  page?: ResourcePage;
  loading: boolean;
  error?: string;
}

export interface ResourceShelfController {
  state(): ResourceShelfViewState;
  subscribe(listener: (state: ResourceShelfViewState) => void): () => void;
  refresh(): Promise<void>;
  whenSettled(): Promise<void>;
  dispose(): void;
}

/** Resources 页控制器：失效事件触发重取；迟到结果按代次丢弃（不从缓存回填旧投影——AC1）。 */
export function createResourceShelfController(handle: ResourceShelfHandle): ResourceShelfController {
  let state: ResourceShelfViewState = { loading: true };
  let generation = 0;
  let tail: Promise<void> = Promise.resolve();
  const listeners = new Set<(state: ResourceShelfViewState) => void>();

  const publish = (next: ResourceShelfViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const load = (): Promise<void> => {
    const run = ++generation;
    publish({ ...state, loading: true });
    const attempt = (async (): Promise<void> => {
      try {
        const page = await handle.browse();
        if (run === generation) publish({ page, loading: false });
      } catch (cause) {
        if (run === generation) publish({ page: undefined, loading: false, error: cause instanceof Error ? cause.message : String(cause) });
      }
    })();
    tail = attempt;
    return attempt;
  };
  const unsubscribe = handle.subscribe((event) => {
    if (event.type === 'authorization-revoked' || event.type === 'scope-closed') void load();
  });
  void load();
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    refresh: load,
    whenSettled: () => tail,
    dispose() { unsubscribe(); listeners.clear(); generation += 1; },
  };
}
