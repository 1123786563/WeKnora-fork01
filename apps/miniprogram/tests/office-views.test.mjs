import test from 'node:test';
import assert from 'node:assert/strict';
const views = await import('../src/services/office-views.ts');

test('run status labels cover the six states and fall back to the raw value', () => {
  for (const status of ['queued', 'running', 'waiting_user', 'reconciling', 'succeeded', 'failed', 'canceled']) {
    assert.equal(typeof views.runStatusLabels[status], 'string');
  }
  assert.equal(views.runStatusLabels.weird ?? 'weird', 'weird');
});
test('badge tone maps attention-carrying states to warning, success only for succeeded', () => {
  assert.equal(views.runStatusBadgeTone('waiting_user'), 'warning');
  assert.equal(views.runStatusBadgeTone('succeeded'), 'success');
  assert.equal(views.runStatusBadgeTone('running'), 'info');
  assert.equal(views.runStatusBadgeTone('whatever'), 'neutral');
});
test('inbox filtering: optional run filter keeps cross-run inbox as the source of truth', () => {
  const items = [
    { interactionId: 'i1', runId: 'run-1', kind: 'tool_approval', argsHash: 'h', expectedRevision: 2, createdAt: '2026-09-24T00:00:00Z' },
    { interactionId: 'i2', runId: 'run-2', kind: 'budget', argsHash: 'h', expectedRevision: 1, createdAt: '2026-09-24T00:00:01Z' },
  ];
  assert.equal(views.inboxVisibleItems(items).length, 2);
  assert.equal(views.inboxVisibleItems(items, 'run-1').length, 1);
  assert.equal(views.inboxVisibleItems(items, 'run-1')[0].interactionId, 'i1');
});
test('decision receipt copy is honest for all four states', () => {
  assert.match(views.decisionReceiptText({ status: 'recorded', record: { interactionId: 'i1', runId: 'r', kind: 'tool_approval', decisionId: 'd1', action: 'reject', argsHash: 'h', expectedRevision: 2 } }), /已记录/);
  assert.match(views.decisionReceiptText({ status: 'delivery-unknown', interactionId: 'i1', decisionId: 'd1' }), /不确定/);
  assert.match(views.decisionReceiptText({ status: 'superseded', interactionId: 'i1' }), /已被取代/);
  assert.match(views.decisionReceiptText({ status: 'gone', interactionId: 'i1' }), /已失效/);
});
test('interruption notices name the reason without inventing recovery promises', () => {
  assert.match(views.interruptionNotice('gap'), /缺口|重新同步/);
  assert.match(views.interruptionNotice('cursor-expired'), /游标/);
  assert.match(views.interruptionNotice('stream-error'), /连接/);
});
