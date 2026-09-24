import test from 'node:test';
import assert from 'node:assert/strict';
import type { AgentOption, KnowledgeResource } from '@weknora/domain/mobile';
import { recommendLeadAgent } from '@weknora/domain/mobile';
import type { TaskOffice, TaskStartReceipt } from '@weknora/mobile-core';
import { createNewTaskController, type NewTaskDraftPersistence } from './new-task-view.ts';

function agents(): AgentOption[] {
  return [
    { id: 'a-coding', name: '编码', summary: '', kind: 'coding', capability: { state: 'supported', reason: '' } },
    { id: 'a-general', name: '通用', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } },
  ];
}

const knowledge: KnowledgeResource[] = [
  { id: 'kb-1', title: '团队知识库', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-24T00:00:00Z' },
];

function memoryDrafts(): NewTaskDraftPersistence & { saved: string[] } {
  const saved: string[] = [];
  let current: string | undefined;
  return {
    saved,
    async load() { return current === undefined ? undefined : JSON.parse(current); },
    async save(draft) { current = JSON.stringify(draft); saved.push(draft.text); },
  };
}

function officeDouble(startImpl?: (goal: unknown, options?: { requestId?: string }) => Promise<TaskStartReceipt>) {
  const calls: Array<{ goal: unknown; options?: { requestId?: string } }> = [];
  const double = {
    calls,
    async start(goal: never, options?: { requestId?: string }) {
      calls.push({ goal, options });
      return startImpl ? startImpl(goal, options) : { requestId: 'req-1', phase: 'bound', runId: 'run-1', dispatched: true };
    },
    async reconcilePending() { return []; },
  };
  return double as unknown as Pick<TaskOffice, 'start' | 'reconcilePending'> & { calls: typeof calls };
}

test('initialization adopts the recommended lead agent and surfaces the recommendation basis', async () => {
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => agents(), drafts: memoryDrafts(), newRequestId: () => 'req-9' });
  await controller.whenInitialized();
  const state = controller.state();
  assert.equal(state.loading, false);
  assert.deepEqual(state.recommendation, recommendLeadAgent(agents()));
  assert.equal(state.draft.agentId, 'a-general', 'a draft without an agent adopts the recommended lead');
  controller.dispose();
});

test('a stored draft overrides the recommendation (explicit choice wins)', async () => {
  const drafts = memoryDrafts();
  await drafts.save({ text: '既有草稿', agentId: 'a-coding', budgetUpper: 50, attachments: [], knowledgeIds: [] });
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => agents(), drafts, newRequestId: () => 'req-9' });
  await controller.whenInitialized();
  assert.equal(controller.state().draft.agentId, 'a-coding');
  assert.equal(controller.state().draft.text, '既有草稿');
  controller.dispose();
});

test('a not-ready submission (attachments scanning) never calls office.start and keeps the draft', async () => {
  const drafts = memoryDrafts();
  const office = officeDouble();
  const controller = createNewTaskController({ office, agents: async () => agents(), drafts, newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  controller.update({ text: '整理周报' });
  controller.setAttachments([{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }]);
  const receipt = await controller.submit();
  assert.equal(receipt, undefined);
  assert.equal(office.calls.length, 0, 'zero submissions for a not-ready draft');
  assert.equal(controller.state().draft.text, '整理周报', 'the draft survives (AC2)');
  assert.equal(drafts.saved[drafts.saved.length - 1], '整理周报');
  assert.equal(controller.state().readiness.reason, 'attachments_not_ready');
  controller.dispose();
});

test('a bound receipt clears the editable fields but keeps agent and budget for the next task', async () => {
  const drafts = memoryDrafts();
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => agents(), drafts, newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  controller.update({ text: '整理周报', budgetUpper: 300 });
  const receipt = await controller.submit();
  assert.equal(receipt?.phase, 'bound');
  const state = controller.state();
  assert.equal(state.draft.text, '');
  assert.equal(state.draft.agentId, 'a-general');
  assert.equal(state.draft.budgetUpper, 300);
  assert.equal(drafts.saved[drafts.saved.length - 1], '', 'the cleared draft is persisted');
  controller.dispose();
});

test('a failed submit keeps the draft and the intent request id; retry re-enters with the SAME id (D5)', async () => {
  const drafts = memoryDrafts();
  const office = officeDouble(async () => ({ requestId: 'req-first', phase: 'awaiting_reconciliation', dispatched: true }));
  const controller = createNewTaskController({ office, agents: async () => agents(), drafts, newRequestId: () => 'req-first' });
  await controller.whenInitialized();
  controller.update({ text: '离线目标' });
  await controller.submit();
  assert.equal(controller.state().draft.text, '离线目标', 'an unresolved submission keeps the draft (AC2 offline)');
  assert.deepEqual(office.calls.map((call) => call.options), [{}]);
  await controller.submit();
  assert.equal(office.calls[1]!.options?.requestId, 'req-first', 'the retry re-enters with the same request id');
  assert.equal(office.calls.length, 2);
  controller.dispose();
});

test('an input conflict surfaces the typed error and keeps the draft (zero replay)', async () => {
  const drafts = memoryDrafts();
  const office = officeDouble(async () => { throw new Error('TASK_OFFICE_SUBMISSION_CONFLICT'); });
  const controller = createNewTaskController({ office, agents: async () => agents(), drafts, newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  controller.update({ text: '冲突目标' });
  const receipt = await controller.submit();
  assert.equal(receipt, undefined);
  assert.equal(controller.state().error, 'TASK_OFFICE_SUBMISSION_CONFLICT');
  assert.equal(controller.state().draft.text, '冲突目标', 'the draft survives a conflict (AC2)');
  controller.dispose();
});

test('cancelKeepingDraft persists the draft explicitly', async () => {
  const drafts = memoryDrafts();
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => agents(), drafts, newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  controller.update({ text: '取消时的草稿' });
  await controller.cancelKeepingDraft();
  assert.equal(drafts.saved.includes('取消时的草稿'), true);
  controller.dispose();
});

test('attached knowledge references toggle into the draft and ride along to start', async () => {
  const office = officeDouble();
  const controller = createNewTaskController({ office, agents: async () => agents(), knowledge: async () => knowledge, drafts: memoryDrafts(), newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  assert.deepEqual(controller.state().knowledge.map((item) => item.id), ['kb-1'], 'the attachable knowledge list is projected');
  controller.update({ text: '整理周报' });
  controller.toggleKnowledge('kb-1');
  assert.deepEqual(controller.state().draft.knowledgeIds, ['kb-1']);
  controller.toggleKnowledge('kb-1');
  assert.deepEqual(controller.state().draft.knowledgeIds, [], 'toggling again detaches it');
  controller.toggleKnowledge('kb-1');
  await controller.submit();
  assert.deepEqual((office.calls[0]!.goal as { knowledgeIds?: string[] }).knowledgeIds, ['kb-1'], 'attached knowledge rides along to start(goal) — never into the Start body');
  controller.dispose();
});

test('unresolved submissions from before the restart are surfaced on initialization', async () => {
  const office = officeDouble();
  const recoverable = { ...office, reconcilePending: async () => [{ requestId: 'req-old', phase: 'awaiting_reconciliation', dispatched: false }] };
  const controller = createNewTaskController({ office: recoverable as unknown as Pick<TaskOffice, 'start' | 'reconcilePending'>, agents: async () => agents(), drafts: memoryDrafts(), newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  assert.deepEqual(controller.state().inFlight, { requestId: 'req-old', phase: 'awaiting_reconciliation', dispatched: false });
  controller.dispose();
});
