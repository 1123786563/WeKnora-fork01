import test from 'node:test';
import assert from 'node:assert/strict';
import type { TaskDetailView, TaskHandle } from '@weknora/mobile-core';
import { TaskOfficeError } from '@weknora/mobile-core';
import { createTaskDetailController } from './task-detail-view.ts';

const view$ = (connection: TaskDetailView['connection'], seqs: number[] = [1]): TaskDetailView => ({
  taskId: 'task-1', runId: 'run-1', title: '报告', lifecycle: 'active', runStatus: 'running', attention: 'none',
  executionStatus: 'running', settlementStatus: 'pending', revision: 1, cursor: seqs.at(-1) ?? 0, incomplete: false,
  connection,
  timeline: seqs.map((seq) => ({ seq, occurredAt: '2026-09-23T00:00:00Z', kind: 'run_status' as const, type: 'run.started', summary: '任务已开始', evidence: { payload: {} } })),
  duplicateSeqs: [],
});

function handleFake(views: TaskDetailView[]): TaskHandle & { resyncCount(): number; push(view: TaskDetailView): void } {
  let index = 0;
  let resyncs = 0;
  let listener: ((view: TaskDetailView) => void) | undefined;
  return {
    push(view: TaskDetailView) { listener?.(view); },
    resyncCount: () => resyncs,
    async hydrate() { return views[index++] ?? views[views.length - 1]!; },
    view: () => views[Math.max(index - 1, 0)],
    updates(next) { listener = next; return () => { listener = undefined; }; },
    async resync() { resyncs += 1; return views[Math.min(index, views.length - 1)]!; },
    close() {},
  };
}

test('the controller publishes the hydrated view, streams updates and refresh resyncs the handle', async () => {
  const fake = handleFake([view$('live'), view$('drained', [1, 2])]);
  const controller = createTaskDetailController(fake);
  await controller.whenSettled();
  assert.deepEqual(controller.state().view?.timeline.map((entry) => entry.seq), [1]);
  assert.equal(controller.state().loading, false);
  fake.push(view$('live', [1, 2])); // updates 增量
  assert.deepEqual(controller.state().view?.timeline.map((entry) => entry.seq), [1, 2]);
  await controller.refresh();
  assert.equal(controller.state().view?.connection, 'drained');
  assert.equal(fake.resyncCount(), 1);
  controller.dispose();
});

test('messageOf renders TaskOfficeError codes as human copy', async () => {
  const { TASK_OFFICE_ERROR_COPY, createTaskDetailController } = await import('./task-detail-view.ts');
  assert.equal(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_INVALID_INPUT, '任务参数缺失（taskId/runId），请从任务列表重新进入。');
  assert.equal(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_DETAIL_UNAVAILABLE, '当前部署未提供任务详情通道。');
  assert.equal(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_SCOPE_CHANGED, '登录状态或活动空间已变化，请重新进入。');
  // 行为级：hydrate 失败抛 TaskOfficeError 时，state.error 是文案而非裸错误码
  // （顶层静态导入与被测模块的 require 走同一 mobile-core 实例，instanceof 才成立）
  const failing: TaskHandle = {
    async hydrate() { throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT'); },
    view: () => undefined,
    updates() { return () => {}; },
    async resync() { return view$('live'); },
    close() {},
  };
  const controller = createTaskDetailController(failing);
  await controller.whenSettled();
  assert.equal(controller.state().error, '任务参数缺失（taskId/runId），请从任务列表重新进入。');
  controller.dispose();
});
