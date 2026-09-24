import test from 'node:test';
import assert from 'node:assert/strict';
import type { TaskOffice } from '@weknora/mobile-core';
import { legacyTaskGates } from '@weknora/mobile-core';
import { createLegacyTasksController } from './legacy-tasks-view.ts';

type LegacyOffice = Pick<TaskOffice, 'legacyTasks' | 'moreLegacyTasks' | 'legacyHistory' | 'followUp'>;

function fakeLegacyOffice(calls: string[], options: { failFollowUp?: boolean } = {}): LegacyOffice {
  const office = {
    async legacyTasks() {
      calls.push('legacyTasks');
      return {
        items: [{ taskId: 'lg-1', title: '旧聊天', attention: 'none' as const, updatedAt: '2026-09-20T08:00:00Z', kind: 'legacy' as const, gates: legacyTaskGates() }],
        duplicateTaskIds: [],
      };
    },
    async moreLegacyTasks() {
      calls.push('moreLegacyTasks');
      return { items: [], duplicateTaskIds: [] };
    },
    async legacyHistory(taskId: string) {
      calls.push(`history:${taskId}`);
      return [{ messageId: 'm1', role: 'user' as const, content: '第一问' }];
    },
    async followUp(input: { taskId: string; question: string }) {
      if (options.failFollowUp) throw new Error('HTTP_409: another turn is already running');
      calls.push(`followUp:${input.taskId}:${input.question}`);
    },
  };
  return office as LegacyOffice;
}

test('the controller loads the legacy page, opens history and submits a follow-up', async () => {
  const calls: string[] = [];
  const controller = createLegacyTasksController(fakeLegacyOffice(calls));
  await controller.whenSettled();
  let state = controller.state();
  assert.equal(state.loading, false);
  assert.equal(state.items[0]!.taskId, 'lg-1');
  assert.equal(state.hasMore, false);

  await controller.openHistory('lg-1');
  state = controller.state();
  assert.equal(state.history?.taskId, 'lg-1');
  assert.equal(state.history?.messages[0]!.content, '第一问');

  await controller.submitFollowUp('lg-1', '继续这个话题');
  state = controller.state();
  assert.equal(state.followUpState, 'sent');
  assert.deepEqual(calls, ['legacyTasks', 'history:lg-1', 'followUp:lg-1:继续这个话题', 'legacyTasks'], 'a successful follow-up reloads the page');
  controller.dispose();
});

test('the controller surfaces follow-up failures without faking success', async () => {
  const calls: string[] = [];
  const controller = createLegacyTasksController(fakeLegacyOffice(calls, { failFollowUp: true }));
  await controller.whenSettled();
  await controller.submitFollowUp('lg-1', 'boom');
  const state = controller.state();
  assert.equal(state.followUpState, 'idle');
  assert.match(state.followUpError ?? '', /409/);
  assert.deepEqual(calls, ['legacyTasks'], 'no reload happens after a failed follow-up');
  controller.dispose();
});

test('the controller skips empty follow-up input', async () => {
  const calls: string[] = [];
  const controller = createLegacyTasksController(fakeLegacyOffice(calls));
  await controller.whenSettled();
  await controller.submitFollowUp('lg-1', '   ');
  const state = controller.state();
  assert.equal(state.followUpState, 'idle');
  assert.equal(calls.filter((call) => call.startsWith('followUp')).length, 0);
  controller.dispose();
});
