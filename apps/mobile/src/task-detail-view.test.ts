import test from 'node:test';
import assert from 'node:assert/strict';
import type { TaskDetailView, TaskHandle, TaskIntent } from '@weknora/mobile-core';
import { TaskOfficeError } from '@weknora/mobile-core';
import { createTaskDetailController, TASK_OFFICE_ERROR_COPY } from './task-detail-view.ts';

const view$ = (connection: TaskDetailView['connection'], seqs: number[] = [1]): TaskDetailView => ({
  taskId: 'task-1', runId: 'run-1', title: '报告', lifecycle: 'active', runStatus: 'running', attention: 'none',
  executionStatus: 'running', settlementStatus: 'pending', revision: 1, cursor: seqs.at(-1) ?? 0, incomplete: false,
  connection,
  timeline: seqs.map((seq) => ({ seq, occurredAt: '2026-09-23T00:00:00Z', kind: 'run_status' as const, type: 'run.started', summary: '任务已开始', evidence: { payload: {} } })),
  duplicateSeqs: [],
});

function handleFake(views: TaskDetailView[], options: { actError?: TaskOfficeError } = {}): TaskHandle & { resyncCount(): number; push(view: TaskDetailView): void; clear(): void; acts: TaskIntent[] } {
  let index = 0;
  let resyncs = 0;
  let listener: ((view: TaskDetailView | undefined) => void) | undefined;
  const acts: TaskIntent[] = [];
  return {
    push(view: TaskDetailView) { listener?.(view); },
    clear() { listener?.(undefined); },
    resyncCount: () => resyncs,
    acts,
    async hydrate() { return views[index++] ?? views[views.length - 1]!; },
    view: () => views[Math.max(index - 1, 0)],
    updates(next) { listener = next; return () => { listener = undefined; }; },
    async resync() { resyncs += 1; return views[Math.min(index, views.length - 1)]!; },
    async act(intent) {
      acts.push(intent);
      if (options.actError !== undefined) throw options.actError;
      return { intent, outcome: 'accepted', boundRunId: 'run-1', revision: 2, at: '2026-09-25T00:00:00Z' };
    },
    async flushQueuedIntents() {},
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

test('scope loss immediately clears the mounted controller view and exposes re-entry guidance', async () => {
  const fake = handleFake([view$('live')]);
  const controller = createTaskDetailController(fake);
  await controller.whenSettled();
  assert.equal(controller.state().view?.title, '报告');
  fake.clear();
  assert.equal(controller.state().view, undefined);
  assert.equal(controller.state().error, TASK_OFFICE_ERROR_COPY.TASK_OFFICE_SCOPE_CHANGED);
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
    async act() { return { intent: { kind: 'stop' }, outcome: 'accepted', boundRunId: 'run-1', revision: 1, at: '2026-09-25T00:00:00Z' }; },
    async flushQueuedIntents() {},
    close() {},
  };
  const controller = createTaskDetailController(failing);
  await controller.whenSettled();
  assert.equal(controller.state().error, '任务参数缺失（taskId/runId），请从任务列表重新进入。');
  controller.dispose();
});

/** 干预用例的构造器：既有 handleFake 已带 acts 记录与可注入 actError，这里只负责建控制器。 */
function newControllerWithCommands(options: { actError?: TaskOfficeError } = {}): {
  controller: ReturnType<typeof createTaskDetailController>;
  handle: ReturnType<typeof handleFake>;
} {
  const handle = handleFake([view$('live')], options);
  return { controller: createTaskDetailController(handle), handle };
}

test('controller.act forwards intents to the handle and surfaces receipts through state', async () => {
  const { controller, handle } = newControllerWithCommands();
  const receipt = await controller.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'accepted');
  assert.equal(handle.acts.length, 1);
  assert.deepEqual(handle.acts[0], { kind: 'stop' });
  controller.dispose();
});

test('controller.act maps TaskOfficeError codes to user copy instead of raw codes', async () => {
  const { controller } = newControllerWithCommands({ actError: new TaskOfficeError('TASK_OFFICE_COMMAND_UNKNOWN') });
  await assert.rejects(() => controller.act({ kind: 'stop' }));
  // 文案映射表必须覆盖新码（Screen 兜底渲染用）。
  assert.match(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_COMMAND_UNKNOWN, /核对/);
  assert.equal(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_COMMAND_UNAVAILABLE, '当前部署未提供运行干预通道。');
  assert.equal(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_NO_SNAPSHOT, '任务快照尚未同步，请稍候再试。');
  assert.equal('TASK_OFFICE_COMMAND_CONFLICT' in TASK_OFFICE_ERROR_COPY, false, '死码不得回潮：该码从未有抛出点（R1-F9），冲突走跨包契约码 TASK_COMMAND_CONFLICT');
  controller.dispose();
});
