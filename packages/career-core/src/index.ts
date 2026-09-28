import type { CareerCommand, CareerEnvelope, CareerIntent, CareerIntentStore, CareerPage, CareerReceipt, CareerRemote, CareerScope } from '@weknora/contracts';

export type { CareerScope, CareerIntent, CareerEnvelope, CareerPage, CareerReceipt, CareerRemote, CareerIntentStore } from '@weknora/contracts';
export interface CareerDeskOptions<T, C extends CareerCommand = CareerCommand> {
  remote: CareerRemote<T, C>;
  intentStore: CareerIntentStore<C>;
  initialScope: CareerScope;
  createRequestId?: () => string;
}
function validScope(scope: CareerScope): CareerScope {
  if (!scope || typeof scope !== 'object' || Object.keys(scope).length !== 3 || Object.keys(scope).some(key => !['deploymentOrigin', 'tenantId', 'actorId'].includes(key))) throw new Error('Invalid Career scope');
  if (typeof scope.deploymentOrigin !== 'string' || typeof scope.tenantId !== 'string' || typeof scope.actorId !== 'string' || !scope.tenantId.trim() || !scope.actorId.trim()) throw new Error('Invalid Career scope');
  const origin = new URL(scope.deploymentOrigin);
  if (!['http:', 'https:'].includes(origin.protocol) || origin.origin !== scope.deploymentOrigin) throw new Error('Invalid Career scope');
  return { deploymentOrigin: origin.origin, tenantId: scope.tenantId, actorId: scope.actorId };
}
function validRevision(value: number): boolean { return Number.isSafeInteger(value) && value > 0; }
function sameScope(a: CareerScope, b: CareerScope): boolean { return a.deploymentOrigin === b.deploymentOrigin && a.tenantId === b.tenantId && a.actorId === b.actorId; }
function scopeError(): Error { return new Error('Career scope changed while operation was in flight'); }

export function createCareerDesk<T, C extends CareerCommand = CareerCommand>(options: CareerDeskOptions<T, C>) {
  let scope = validScope(options.initialScope);
  let generation = 0;
  let projection: CareerEnvelope<T> | undefined;
  let cursor: string | undefined;
  let stopObserving: (() => void) | undefined;
  const controllers = new Set<AbortController>();
  const makeId = options.createRequestId ?? (() => `career-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`);
  function begin() { const controller = new AbortController(); controllers.add(controller); return controller; }
  function finish(controller: AbortController) { controllers.delete(controller); }
  function ensureGeneration(expected: number, expectedScope: CareerScope) {
    if (generation !== expected || !sameScope(scope, expectedScope)) throw scopeError();
  }
  function acceptReceipt(intent: CareerIntent<C>, receipt: CareerReceipt<T>): CareerReceipt<T> {
    if (receipt.requestId !== intent.requestId) throw new Error('Career receipt request ID mismatch');
    if (receipt.kind === 'applied' || receipt.kind === 'conflict') {
      if (!validRevision(receipt.envelope.revision)) throw new Error('Career receipt has invalid revision');
      if (!projection || receipt.envelope.revision >= projection.revision) projection = receipt.envelope;
    }
    return receipt;
  }
  async function send(intent: CareerIntent<C>, persist: boolean): Promise<CareerReceipt<T>> {
    const expectedGeneration = generation, expectedScope = scope, controller = begin();
    try {
      if (!validRevision(intent.expectedRevision) || !intent.requestId.trim()) throw new Error('Invalid Career intent request ID or expected revision');
      if (persist) await options.intentStore.save(expectedScope, intent);
      ensureGeneration(expectedGeneration, expectedScope);
      let receipt: CareerReceipt<T>;
      try { receipt = await options.remote.act(expectedScope, intent, controller.signal); }
      catch { ensureGeneration(expectedGeneration, expectedScope); return { kind: 'unknown', requestId: intent.requestId }; }
      ensureGeneration(expectedGeneration, expectedScope);
      const accepted = acceptReceipt(intent, receipt);
      if (accepted.kind === 'applied' || accepted.kind === 'forbidden') await options.intentStore.remove(expectedScope, intent.requestId);
      ensureGeneration(expectedGeneration, expectedScope);
      return accepted;
    } catch (error) {
      if (generation !== expectedGeneration || !sameScope(scope, expectedScope)) {
        await options.intentStore.remove(expectedScope, intent.requestId);
        throw scopeError();
      }
      throw error;
    } finally { finish(controller); }
  }
  async function readOpen(): Promise<CareerEnvelope<T>> {
    const expectedGeneration = generation, expectedScope = scope, controller = begin();
    try {
      const result = await options.remote.open(expectedScope, controller.signal);
      ensureGeneration(expectedGeneration, expectedScope);
      if (!validRevision(result.revision)) throw new Error('Career response has invalid revision');
      projection = result; return result;
    } catch (error) {
      if (generation !== expectedGeneration || !sameScope(scope, expectedScope)) throw scopeError();
      throw error;
    } finally { finish(controller); }
  }
  async function readList(nextCursor?: string): Promise<CareerPage<T>> {
    const expectedGeneration = generation, expectedScope = scope, controller = begin();
    try {
      const result = await options.remote.list(expectedScope, nextCursor, controller.signal);
      ensureGeneration(expectedGeneration, expectedScope);
      if (!validRevision(result.revision)) throw new Error('Career response has invalid revision');
      cursor = result.cursor; return result;
    } catch (error) {
      if (generation !== expectedGeneration || !sameScope(scope, expectedScope)) throw scopeError();
      throw error;
    } finally { finish(controller); }
  }
  return {
    open: readOpen,
    list: (nextCursor?: string) => readList(nextCursor),
    act(command: C): Promise<CareerReceipt<T>> {
      const typed = command as C & { expectedRevision: number };
      if (!validRevision(typed.expectedRevision)) return Promise.reject(new Error('Invalid expected revision'));
      if (!projection) return Promise.reject(new Error('Open Career Desk before writing'));
      if (typed.expectedRevision !== projection.revision) return Promise.reject(new Error('Stale Career revision; refresh before writing'));
      const intent: CareerIntent<C> = { requestId: makeId(), expectedRevision: typed.expectedRevision, command };
      return send(intent, true);
    },
    submit(intent: CareerIntent<C>) { return send(intent, false); },
    async reconcilePending(): Promise<CareerReceipt<T>[]> {
      const expectedGeneration = generation, expectedScope = scope;
      const pending = await options.intentStore.list(expectedScope);
      ensureGeneration(expectedGeneration, expectedScope);
      const results: CareerReceipt<T>[] = [];
      for (const intent of pending) {
        ensureGeneration(expectedGeneration, expectedScope);
        const controller = begin();
        let receipt: CareerReceipt<T>;
        try {
          try { receipt = await options.remote.lookup(expectedScope, intent.requestId, controller.signal); }
          catch { ensureGeneration(expectedGeneration, expectedScope); results.push({ kind: 'unknown', requestId: intent.requestId }); continue; }
          ensureGeneration(expectedGeneration, expectedScope);
          if (receipt.requestId !== intent.requestId) throw new Error('Career lookup receipt request ID mismatch');
          if (receipt.kind === 'unknown') {
            try { receipt = await options.remote.act(expectedScope, intent, controller.signal); }
            catch { ensureGeneration(expectedGeneration, expectedScope); results.push({ kind: 'unknown', requestId: intent.requestId }); continue; }
            ensureGeneration(expectedGeneration, expectedScope);
          }
          const accepted = acceptReceipt(intent, receipt);
          if (accepted.kind === 'applied' || accepted.kind === 'forbidden') await options.intentStore.remove(expectedScope, intent.requestId);
          ensureGeneration(expectedGeneration, expectedScope); results.push(accepted);
        } finally { finish(controller); }
      }
      return results;
    },
    rebase(requestId: string, command: C): Promise<CareerIntent<C>> {
      return (async () => {
        const expectedScope = scope, expectedGeneration = generation;
      const old = (await options.intentStore.list(expectedScope)).find(intent => intent.requestId === requestId);
        ensureGeneration(expectedGeneration, expectedScope);
        if (!old) throw new Error('Career conflict intent is no longer pending');
        const expectedRevision = (command as C & { expectedRevision: number }).expectedRevision;
        if (!validRevision(expectedRevision) || expectedRevision <= old.expectedRevision || (projection && expectedRevision !== projection.revision)) throw new Error('Rebase requires a fresh, newer Career revision');
        const nextRequestId = makeId();
        if (!nextRequestId.trim() || nextRequestId === old.requestId) throw new Error('Rebase must create a new non-empty request ID');
        const rebased = { requestId: nextRequestId, expectedRevision, command };
        try {
          await options.intentStore.save(expectedScope, rebased);
          ensureGeneration(expectedGeneration, expectedScope);
        } catch (error) {
          if (generation !== expectedGeneration || !sameScope(scope, expectedScope)) {
            await options.intentStore.remove(expectedScope, rebased.requestId);
            throw scopeError();
          }
          throw error;
        }
        await options.intentStore.remove(expectedScope, requestId);
        ensureGeneration(expectedGeneration, expectedScope);
        return rebased;
      })();
    },
    observe(): void {
      stopObserving?.();
      const expectedGeneration = generation, expectedScope = scope;
      stopObserving = options.remote.observe(expectedScope, revision => {
        if (generation === expectedGeneration && sameScope(scope, expectedScope) && validRevision(revision)) void readOpen();
      });
    },
    snapshot() { return projection; },
    currentScope() { return scope; },
    currentCursor() { return cursor; },
    async changeScope(nextScope: CareerScope): Promise<void> {
      const oldScope = scope;
      const newScope = validScope(nextScope);
      if (sameScope(newScope, oldScope)) return;
      generation++;
      stopObserving?.(); stopObserving = undefined;
      for (const controller of controllers) controller.abort();
      projection = undefined; cursor = undefined; scope = newScope;
      const oldPending = await options.intentStore.list(oldScope);
      for (const intent of oldPending) await options.intentStore.remove(oldScope, intent.requestId);
    },
    dispose(): void {
      generation++; stopObserving?.(); stopObserving = undefined;
      for (const controller of controllers) controller.abort();
      controllers.clear(); projection = undefined; cursor = undefined;
    },
  };
}
