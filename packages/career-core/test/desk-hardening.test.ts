import assert from 'node:assert/strict';
import test from 'node:test';
import { createCareerDesk } from '../src/index.ts';
import type { CareerRemote, CareerScope, CareerIntentStore, CareerCommand, CareerWorkspace } from '@weknora/contracts';

const scopeA: CareerScope = { deploymentOrigin: 'https://a.example', tenantId: 'tenant-a', actorId: 'actor-a' };
const scopeB: CareerScope = { deploymentOrigin: 'https://b.example', tenantId: 'tenant-b', actorId: 'actor-b' };
const applied = (requestId: string, revision = 2) => ({ kind: 'applied' as const, requestId, envelope: { revision, value: { profile: { id: 'p', revision, facts: [] }, opportunities: [], applications: [] } } });

test('Desk persists before dispatch, binds every store and remote operation to structured scope, and applies receipts', async () => {
  const log: string[] = [];
  const remote: CareerRemote<CareerWorkspace, CareerCommand> = {
    open: async scope => { log.push(`open:${scope.tenantId}`); return { revision: 1, value: { opportunities: [], applications: [] } }; },
    list: async scope => ({ revision: 1, value: [] }),
    act: async (scope, intent) => { log.push(`act:${scope.tenantId}:${intent.requestId}`); return applied(intent.requestId); },
    lookup: async (scope, requestId) => { log.push(`lookup:${scope.tenantId}:${requestId}`); return { kind: 'unknown', requestId }; },
    observe: () => () => undefined,
  };
  const store: CareerIntentStore<CareerCommand> = {
    save: async (scope, intent) => { log.push(`save:${scope.tenantId}:${intent.requestId}`); },
    list: async scope => { log.push(`store-list:${scope.tenantId}`); return []; },
    remove: async (scope, requestId) => { log.push(`remove:${scope.tenantId}:${requestId}`); },
  };
  const desk = createCareerDesk({ remote, intentStore: store, initialScope: scopeA, createRequestId: () => 'r1' });
  await desk.open();
  const receipt = await desk.act({ type: 'updateProfile', expectedRevision: 1, payload: { facts: [] } });
  assert.equal(receipt.kind, 'applied');
  assert.ok(log.indexOf('save:tenant-a:r1') < log.indexOf('act:tenant-a:r1'));
});

test('unknown outcomes are looked up by the original ID before retry; conflict rebase always gets a new ID', async () => {
  const ids = ['original', 'rebased'];
  const calls: string[] = [];
  let phase = 0;
  const remote: CareerRemote<CareerWorkspace, CareerCommand> = {
    open: async () => ({ revision: 1, value: { opportunities: [], applications: [] } }), list: async () => ({ revision: 1, value: [] }),
    act: async (_scope, intent) => { calls.push(`act:${intent.requestId}`); phase++; return phase === 1 ? { kind: 'unknown', requestId: intent.requestId } : phase === 2 ? { kind: 'conflict', requestId: intent.requestId, envelope: { revision: 3, value: { opportunities: [], applications: [] } } } : { kind: 'applied', requestId: intent.requestId, envelope: { revision: 4, value: { opportunities: [], applications: [] } } }; },
    lookup: async (_scope, requestId) => { calls.push(`lookup:${requestId}`); return { kind: 'unknown', requestId }; }, observe: () => () => undefined,
  };
  const pending = new Map<string, any>();
  const store: CareerIntentStore<CareerCommand> = { save: async (_s, i) => { pending.set(i.requestId, i); }, list: async () => [...pending.values()], remove: async (_s, id) => { pending.delete(id); } };
  const desk = createCareerDesk({ remote, intentStore: store, initialScope: scopeA, createRequestId: () => ids.shift()! });
  await desk.open();
  assert.equal((await desk.act({ type: 'updateProfile', expectedRevision: 1, payload: { facts: [] } })).kind, 'unknown');
  assert.equal((await desk.reconcilePending())[0].kind, 'conflict');
  assert.deepEqual(calls, ['act:original', 'lookup:original', 'act:original']);
  const rebased = await desk.rebase('original', { type: 'updateProfile', expectedRevision: 3, payload: { facts: [] } });
  assert.equal(rebased.requestId, 'rebased');
  assert.equal(rebased.expectedRevision, 3);
  assert.equal((await desk.reconcilePending())[0].kind, 'applied');
  assert.deepEqual(calls, ['act:original', 'lookup:original', 'act:original', 'lookup:rebased', 'act:rebased']);
});

test('unknown write remains pending and cannot be rebased after a newer read until same-ID conflict is authoritative', async () => {
  const pending = new Map<string, any>();
  let openRevision = 1;
  const remote: CareerRemote<CareerWorkspace, CareerCommand> = {
    open: async () => ({ revision: openRevision, value: { opportunities: [], applications: [] } }),
    list: async () => ({ revision: 1, value: [] }),
    act: async (_scope, intent) => ({ kind: 'unknown', requestId: intent.requestId }),
    lookup: async (_scope, requestId) => ({ kind: 'unknown', requestId }),
    observe: () => () => undefined,
  };
  const store: CareerIntentStore<CareerCommand> = {
    save: async (_s, i) => { pending.set(i.requestId, i); }, list: async () => [...pending.values()],
    remove: async (_s, id) => { pending.delete(id); },
  };
  const desk = createCareerDesk({ remote, intentStore: store, initialScope: scopeA, createRequestId: () => 'original' });
  await desk.open();
  await desk.act({ type: 'updateProfile', expectedRevision: 1, payload: { facts: [] } });
  openRevision = 2;
  await desk.open();
  await assert.rejects(desk.rebase('original', { type: 'updateProfile', expectedRevision: 2, payload: { facts: [] } }), /conflict/i);
  assert.equal(pending.has('original'), true);
  assert.equal((await desk.reconcilePending())[0]?.kind, 'unknown');
  assert.equal(pending.has('original'), true);
});

test('out-of-order same-scope open reads cannot regress the projection', async () => {
  const deferred: Array<(value: any) => void> = [];
  const remote: CareerRemote<CareerWorkspace, CareerCommand> = {
    open: async () => new Promise(resolve => { deferred.push(resolve); }), list: async () => ({ revision: 1, value: [] }),
    act: async (_s, i) => ({ kind: 'applied', requestId: i.requestId, envelope: { revision: 4, value: { opportunities: [], applications: [] } } }),
    lookup: async (_s, id) => ({ kind: 'unknown', requestId: id }), observe: () => () => undefined,
  };
  const store: CareerIntentStore<CareerCommand> = { save: async () => undefined, list: async () => [], remove: async () => undefined };
  const desk = createCareerDesk({ remote, intentStore: store, initialScope: scopeA });
  const first = desk.open(); const second = desk.open();
  deferred[1]!({ revision: 3, value: { opportunities: [], applications: [] } }); await second;
  deferred[0]!({ revision: 2, value: { opportunities: [], applications: [] } }); await first;
  assert.equal(desk.snapshot()?.revision, 3);
  await assert.rejects(desk.act({ type: 'updateProfile', expectedRevision: 2, payload: { facts: [] } }), /stale/i);
});

test('Desk has no public submit path that can dispatch an unpersisted request', () => {
  const desk = createCareerDesk({
    remote: { open: async () => ({ revision: 1, value: { opportunities: [], applications: [] } }), list: async () => ({ revision: 1, value: [] }), act: async (_s, i) => ({ kind: 'unknown', requestId: i.requestId }), lookup: async (_s, id) => ({ kind: 'unknown', requestId: id }), observe: () => () => undefined },
    intentStore: { save: async () => undefined, list: async () => [], remove: async () => undefined }, initialScope: scopeA,
  });
  assert.equal('submit' in desk, false);
});

test('revision-hint refresh rejection is reported and scope-switch refresh rejection is contained', async () => {
  let hint!: (revision: number) => void;
  const errors: unknown[] = [];
  const remote: CareerRemote<CareerWorkspace, CareerCommand> = {
    open: async () => { throw new Error('refresh failed'); }, list: async () => ({ revision: 1, value: [] }),
    act: async (_s, i) => ({ kind: 'unknown', requestId: i.requestId }), lookup: async (_s, id) => ({ kind: 'unknown', requestId: id }),
    observe: (_s, callback) => { hint = callback; return () => undefined; },
  };
  const desk = createCareerDesk({ remote, intentStore: { save: async () => undefined, list: async () => [], remove: async () => undefined }, initialScope: scopeA, onRefreshError: error => errors.push(error) });
  desk.observe(); hint(2); await new Promise(resolve => setImmediate(resolve));
  assert.equal(errors.length, 1);
  await desk.changeScope(scopeB); hint(3); await new Promise(resolve => setImmediate(resolve));
  assert.equal(errors.length, 1);
});

test('forbidden receipt is terminal and stale expected revisions are rejected before sending', async () => {
  let calls = 0;
  const remote: CareerRemote<CareerWorkspace, CareerCommand> = {
    open: async () => ({ revision: 2, value: { opportunities: [], applications: [] } }), list: async () => ({ revision: 2, value: [] }),
    act: async (_s, i) => { calls++; return { kind: 'forbidden', requestId: i.requestId }; }, lookup: async (_s, requestId) => ({ kind: 'forbidden', requestId }), observe: () => () => undefined,
  };
  const store: CareerIntentStore<CareerCommand> = { save: async () => undefined, list: async () => [], remove: async () => undefined };
  const desk = createCareerDesk({ remote, intentStore: store, initialScope: scopeA, createRequestId: () => 'r' });
  await desk.open();
  await assert.rejects(desk.act({ type: 'updateProfile', expectedRevision: 1, payload: { facts: [] } }), /stale/i);
  assert.equal(calls, 0);
  assert.equal((await desk.act({ type: 'updateProfile', expectedRevision: 2, payload: { facts: [] } })).kind, 'forbidden');
  assert.equal(calls, 1);
});

test('scope changes during persistence, dispatch, lookup, and reads reject old replies and clear only old-scope intents', async () => {
  const removed: string[] = [];
  let releaseSave!: () => void;
  let releaseAct!: (receipt: any) => void;
  let releaseLookup!: (receipt: any) => void;
  let releaseRead!: (value: any) => void;
  let deferRead = false;
  const remote: CareerRemote<CareerWorkspace, CareerCommand> = {
    open: () => deferRead ? new Promise(resolve => { releaseRead = resolve; }) : Promise.resolve({ revision: 1, value: { opportunities: [], applications: [] } }), list: async () => ({ revision: 1, value: [] }),
    act: async () => new Promise(resolve => { releaseAct = resolve; }), lookup: async (_s, requestId) => new Promise(resolve => { releaseLookup = resolve; }), observe: () => () => undefined,
  };
  let stored: any[] = [{ scope: scopeB, intent: { requestId: 'new-scope-intent', expectedRevision: 1, command: { type: 'updateProfile', expectedRevision: 1, payload: { facts: [] } } } }];
  let deferFirstSave = true;
  const store: CareerIntentStore<CareerCommand> = { save: async (scope, intent) => { if (deferFirstSave) { deferFirstSave = false; await new Promise<void>(resolve => { releaseSave = resolve; }); } stored.push({ scope, intent }); }, list: async scope => stored.filter(row => row.scope.tenantId === scope.tenantId).map(row => row.intent), remove: async (scope, id) => { removed.push(`${scope.tenantId}:${id}`); stored = stored.filter(row => !(row.scope.tenantId === scope.tenantId && row.intent.requestId === id)); } };
  const desk = createCareerDesk({ remote, intentStore: store, initialScope: scopeA, createRequestId: () => 'r1' });
  await desk.open();
  const pending = desk.act({ type: 'updateProfile', expectedRevision: 1, payload: { facts: [] } });
  await new Promise(resolve => setImmediate(resolve));
  await desk.changeScope(scopeB);
  assert.ok(stored.some(row => row.scope.tenantId === 'tenant-b' && row.intent.requestId === 'new-scope-intent'));
  releaseSave();
  await assert.rejects(pending, /scope/i);
  assert.equal(releaseAct, undefined);
  assert.deepEqual(removed, ['tenant-a:r1']);

  await desk.open();
  deferRead = true;
  const read = desk.open(); await new Promise(resolve => setImmediate(resolve)); await desk.changeScope(scopeA);
  releaseRead({ revision: 1, value: { opportunities: [], applications: [] } });
  await assert.rejects(read, /scope/i);
  deferRead = false;

  await desk.open();
  const write = desk.act({ type: 'updateProfile', expectedRevision: 1, payload: { facts: [] } }); await new Promise(resolve => setImmediate(resolve));
  await desk.changeScope(scopeB); (releaseAct as unknown as (receipt: any) => void)(applied('r1'));
  await assert.rejects(write, /scope/i);

  await desk.open();
  const unknownWrite = desk.act({ type: 'updateProfile', expectedRevision: 1, payload: { facts: [] } }); await new Promise(resolve => setImmediate(resolve));
  (releaseAct as unknown as (receipt: any) => void)({ kind: 'unknown', requestId: 'r1' });
  assert.equal((await unknownWrite).kind, 'unknown');
  const pendingLookup = desk.reconcilePending(); await new Promise(resolve => setImmediate(resolve));
  await desk.changeScope(scopeA); releaseLookup({ kind: 'unknown', requestId: 'r1' });
  await assert.rejects(pendingLookup, /scope/i);
});
