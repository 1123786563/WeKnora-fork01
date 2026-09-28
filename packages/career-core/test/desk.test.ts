import assert from 'node:assert/strict';
import test from 'node:test';
import { createCareerDesk } from '../src/index.ts';
import type { CareerCommand, CareerIntentStore, CareerRemote, CareerScope, CareerWorkspace } from '@weknora/contracts';

const scope: CareerScope = { deploymentOrigin: 'https://desk.example', tenantId: 'tenant-1', actorId: 'actor-1' };

test('unknown write recovery first looks up the same durable request ID', async () => {
  const order: string[] = [];
  let stored: any[] = [];
  let requestCount = 0;
  const remote: CareerRemote<CareerWorkspace, CareerCommand> = {
    open: async () => ({ revision: 1, value: { opportunities: [], applications: [] } }),
    list: async () => ({ revision: 1, value: [] }),
    act: async (_scope, intent) => { order.push(`act:${intent.requestId}`); requestCount++; if (requestCount === 1) throw new Error('network outcome unknown'); return { kind: 'applied', requestId: intent.requestId, envelope: { revision: 2, value: { opportunities: [], applications: [] } } }; },
    lookup: async (_scope, requestId) => { order.push(`lookup:${requestId}`); return { kind: 'unknown', requestId }; },
    observe: () => () => undefined,
  };
  const store: CareerIntentStore<CareerCommand> = { save: async (_scope, intent) => { stored = [intent]; }, list: async () => stored, remove: async (_scope, id) => { stored = stored.filter(item => item.requestId !== id); } };
  const desk = createCareerDesk({ remote, intentStore: store, initialScope: scope, createRequestId: () => 'request-1' });
  await desk.open();
  assert.equal((await desk.act({ type: 'updateProfile', expectedRevision: 1, payload: { facts: [] } })).kind, 'unknown');
  assert.equal((await desk.reconcilePending())[0].kind, 'applied');
  assert.deepEqual(order, ['act:request-1', 'lookup:request-1', 'act:request-1']);
  assert.equal(stored.length, 0);
});
