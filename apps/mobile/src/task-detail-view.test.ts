import test from 'node:test';
import assert from 'node:assert/strict';
import type { TaskDetailView, TaskHandle } from '@weknora/mobile-core';
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
