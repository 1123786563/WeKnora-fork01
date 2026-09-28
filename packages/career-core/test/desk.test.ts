import assert from 'node:assert/strict';
import test from 'node:test';
import { createCareerDesk } from '../src/index.ts';

test('Career Desk persists the same request ID before writes and reconciles unknown outcomes', async () => {
  const intents: any[] = [];
  let calls = 0;
  const desk = createCareerDesk({
    api: { act: async (intent: any) => { calls++; if (calls === 1) throw new Error('unknown'); return { requestId: intent.requestId, revision: 2, data: 'ok' }; }, list: async () => [] },
    intentStore: { save: async (intent: any) => { if (!intents.some(item => item.requestId === intent.requestId)) intents.push(intent); }, loadPending: async () => intents, remove: async () => undefined },
    initialScope: 'tenant-a',
  });
  await assert.rejects(desk.act({ type: 'confirmProfile', expectedRevision: 1 }));
  const requestId = intents[0].requestId;
  await desk.reconcilePending();
  assert.equal(intents[1]?.requestId ?? intents[0].requestId, requestId);
});

test('Career Desk requires explicit conflict rebase and drops late replies after scope change', async () => {
  let finish!: (value: any) => void;
  const desk = createCareerDesk({
    api: { act: () => new Promise(resolve => { finish = resolve; }), list: async () => [] },
    intentStore: { save: async () => undefined, loadPending: async () => [], remove: async () => undefined },
    initialScope: 'tenant-a',
  });
  const action = desk.act({ type: 'updateProfile', expectedRevision: 1 });
  await new Promise(resolve => setImmediate(resolve));
  await desk.changeScope('tenant-b');
  finish({ requestId: 'r', revision: 2, data: 'private' });
  await assert.rejects(action, /scope/i);
});
