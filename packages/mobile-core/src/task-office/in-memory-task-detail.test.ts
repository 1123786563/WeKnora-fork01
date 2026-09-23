import test from 'node:test';
import assert from 'node:assert/strict';
import { createInMemoryTaskProjectionStore, createScenarioTaskDetailBackend, createScriptedTaskStream } from './in-memory-task-detail.ts';
import type { TaskBackendDetail, TaskBackendEvent } from './task-detail.ts';

const event$ = (seq: number): TaskBackendEvent => ({ runId: 'run-1', seq, type: 'text.delta', occurredAt: '2026-09-23T00:00:00Z', payload: {} });
const detail$ = (): TaskBackendDetail => ({
  taskId: 'task-1', runId: 'run-1', title: 't', attention: 'none',
  execution: { runStatus: 'running', executionStatus: 'running', settlementStatus: 'pending', revision: 1, seq: 2 },
  watermark: 2, incomplete: false, events: [],
});

test('the projection store round-trips per run and overwrites on save', async () => {
  const store = createInMemoryTaskProjectionStore();
  assert.equal(await store.load('run-1'), undefined);
  await store.save({ taskId: 'task-1', runId: 'run-1', cursor: 2, events: [event$(1)], savedAt: '2026-09-23T00:00:00Z' });
  await store.save({ taskId: 'task-1', runId: 'run-1', cursor: 3, events: [event$(1), event$(2)], savedAt: '2026-09-23T00:00:01Z' });
  const loaded = await store.load('run-1');
  assert.equal(loaded?.cursor, 3);
  assert.equal(store.snapshot().length, 1);
});

test('a scripted stream delivers frames until end, fail, or abort', async () => {
  const first = createScriptedTaskStream();
  const events: number[] = [];
  const controls: string[] = [];
  await new Promise<void>((resolve, reject) => {
    const controller = new AbortController();
    first.attach({ signal: controller.signal, onEvent: (event) => events.push(event.seq), onControl: (frame) => controls.push(frame.code), resolve, reject });
    first.opened.push({ runId: 'run-1', cursor: 0 });
    first.emit(event$(1));
    first.control({ code: 'stream_error', message: 'x' });
    first.end();
  });
  assert.deepEqual(events, [1]);
  assert.deepEqual(controls, ['stream_error']);

  const second = createScriptedTaskStream();
  const failure = new Promise<void>((resolve, reject) => {
    const controller = new AbortController();
    second.attach({ signal: controller.signal, onEvent: () => {}, onControl: () => {}, resolve, reject });
    second.fail(new Error('HTTP 503'));
  });
  await assert.rejects(failure, /HTTP 503/);

  const third = createScriptedTaskStream();
  let abortedSeen = 0;
  await new Promise<void>((resolve) => {
    const controller = new AbortController();
    third.attach({ signal: controller.signal, onEvent: () => { abortedSeen += 1; }, onControl: () => {}, resolve, reject: () => {} });
    controller.abort();
    third.emit(event$(9)); // abort 之后静默
  });
  assert.equal(abortedSeen, 0, 'frames after an abort are never delivered');
});

test('the scenario backend records detail calls and hands out scripted streams', async () => {
  const scripted = createScriptedTaskStream();
  const backend = createScenarioTaskDetailBackend({ detail: async () => detail$(), stream: () => scripted });
  assert.deepEqual(await backend.detail('run-1'), detail$());
  assert.deepEqual(backend.detailCalls, ['run-1']);
  const streamPromise = backend.stream({ runId: 'run-1', cursor: 5, signal: new AbortController().signal, onEvent: () => {}, onControl: () => {} });
  assert.equal(backend.streams[0], scripted);
  assert.deepEqual(scripted.opened, [{ runId: 'run-1', cursor: 5 }]);
  scripted.end();
  await streamPromise;
});
