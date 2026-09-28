export interface CareerIntent { requestId: string; type: string; expectedRevision: number; payload?: unknown; scope: string }
export interface CareerDeskOptions<T> {
  api: { act(intent: CareerIntent): Promise<{ requestId: string; revision: number; data: T }>; list(): Promise<T[]> };
  intentStore: { save(intent: CareerIntent): Promise<void>; loadPending(): Promise<CareerIntent[]>; remove(requestId: string): Promise<void> };
  initialScope: string;
  createRequestId?: () => string;
}
export function createCareerDesk<T>(options: CareerDeskOptions<T>) {
  let scope = options.initialScope;
  let generation = 0;
  const id = options.createRequestId ?? (() => `career-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`);
  async function send(intent: CareerIntent, persist: boolean): Promise<T> {
    const currentGeneration = generation;
    if (persist) await options.intentStore.save(intent);
    if (generation !== currentGeneration || scope !== intent.scope) {
      await options.intentStore.remove(intent.requestId);
      throw new Error('Career scope changed before request dispatch');
    }
    const receipt = await options.api.act(intent);
    if (generation !== currentGeneration || scope !== intent.scope) throw new Error('Career scope changed while request was in flight');
    if (receipt.requestId !== intent.requestId) throw new Error('Career receipt request ID mismatch');
    await options.intentStore.remove(intent.requestId);
    return receipt.data;
  }
  async function read(): Promise<T[]> {
    const currentGeneration = generation;
    const result = await options.api.list();
    if (generation !== currentGeneration) throw new Error('Career scope changed while read was in flight');
    return result;
  }
  return {
    async open() { return read(); },
    async list() { return read(); },
    async observe() { return read(); },
    async act(input: { type: string; expectedRevision: number; payload?: unknown }) {
      const intent = { ...input, requestId: id(), scope };
      return send(intent, true);
    },
    async reconcilePending() {
      const pending = await options.intentStore.loadPending();
      const results: T[] = [];
      for (const intent of pending) {
        if (intent.scope !== scope) { await options.intentStore.remove(intent.requestId); continue; }
        results.push(await send(intent, false));
      }
      return results;
    },
    async changeScope(nextScope: string) {
      if (nextScope === scope) return;
      scope = nextScope; generation++;
      for (const intent of await options.intentStore.loadPending()) if (intent.scope !== scope) await options.intentStore.remove(intent.requestId);
    },
    async rebase(intent: CareerIntent, expectedRevision: number) {
      if (intent.scope !== scope) throw new Error('Cannot rebase an intent outside the active scope');
      const rebased = { ...intent, expectedRevision };
      await options.intentStore.save(rebased);
      return rebased;
    },
  };
}
