import test from 'node:test';
import assert from 'node:assert/strict';
import { createTaskForm, EMPTY_DRAFT, evaluateSubmitReadiness, toStartInput, type TaskFormSubmitPorts } from './task-form.ts';

function ports(log: string[] = []): TaskFormSubmitPorts {
  return {
    submit: async (input) => { log.push(`start:${input.request_id}`); return { dispatched: true }; },
    saveDraft: async (draft) => { log.push(`save:${draft.text}|${draft.agentId ?? ''}`); },
  };
}

test('readiness blocks empty text, missing agent, invalid budget and not-ready attachments', () => {
  assert.deepEqual(evaluateSubmitReadiness(EMPTY_DRAFT), { ready: false, reason: 'text_required', blockingAttachments: [] });
  assert.equal(evaluateSubmitReadiness({ ...EMPTY_DRAFT, text: '整理周报' }).reason, 'agent_required');
  assert.equal(evaluateSubmitReadiness({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', budgetUpper: -1 }).reason, 'budget_invalid');
  assert.equal(evaluateSubmitReadiness({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', budgetUpper: 1.5 }).reason, 'budget_invalid');
  const blocked = evaluateSubmitReadiness({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }] });
  assert.equal(blocked.reason, 'attachments_not_ready');
  assert.deepEqual(blocked.blockingAttachments.map((item) => item.id), ['f-1']);
});

test('attemptSubmit with a not-ready attachment keeps the draft and never touches the network', async () => {
  const log: string[] = [];
  const form = createTaskForm({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'pending' }] }, ports(log));
  const result = await form.attemptSubmit({ requestID: 'req-1', sessionId: 's-1', targetId: 'platform', workspaceRef: '' });
  assert.equal(result.submitted, false);
  assert.equal(result.readiness.reason, 'attachments_not_ready');
  assert.equal(log.filter((entry) => entry.startsWith('start:')).length, 0, 'zero submissions');
  assert.equal(form.draft.text, '整理周报', 'the draft survives');
});

test('toStartInput produces exactly the frozen seven fields; attachments and knowledge never enter the body', () => {
  const input = toStartInput(
    { text: '  整理周报  ', agentId: 'a-1', budgetUpper: 200, attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'ready' }], knowledgeIds: ['kb-1'] },
    { requestID: 'req-1', sessionId: 's-1', targetId: 'platform', workspaceRef: 'ws' },
  );
  assert.deepEqual(Object.keys(input).sort(), ['agent_id', 'budget_upper', 'request_id', 'session_id', 'target_id', 'text', 'workspace_ref']);
  assert.equal(input.text, '整理周报');
});

test('a successful dispatch clears the editable fields but keeps agent and budget; a failed one keeps everything', async () => {
  const log: string[] = [];
  const form = createTaskForm({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', budgetUpper: 200 }, ports(log));
  await form.attemptSubmit({ requestID: 'req-1', sessionId: 's-1', targetId: 'platform', workspaceRef: '' });
  assert.equal(form.draft.text, '');
  assert.equal(form.draft.agentId, 'a-1');
  assert.equal(form.draft.budgetUpper, 200);
  const failing = createTaskForm({ ...EMPTY_DRAFT, text: '离线目标', agentId: 'a-1' }, { submit: async () => ({ dispatched: false }), saveDraft: async () => {} });
  await failing.attemptSubmit({ requestID: 'req-2', sessionId: 's-2', targetId: 'platform', workspaceRef: '' });
  assert.equal(failing.draft.text, '离线目标', 'an unresolved submission keeps the draft');
});

test('cancelKeepingDraft persists the draft explicitly', async () => {
  const log: string[] = [];
  const form = createTaskForm({ ...EMPTY_DRAFT, text: '草稿', agentId: null }, ports(log));
  await form.cancelKeepingDraft();
  assert.equal(log.some((entry) => entry === 'save:草稿|'), true);
});
