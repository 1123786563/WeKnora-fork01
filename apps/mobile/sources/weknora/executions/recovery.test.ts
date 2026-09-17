import test from 'node:test';
import assert from 'node:assert/strict';
import { recoverRun } from './recovery.ts';
test('completed run restores history without opening stream',async()=>{
 const calls:string[]=[];
 await recoverRun({status:async()=>{calls.push('status');return {terminal:true};},subscribe:async()=>{calls.push('stream');},refreshHistory:async()=>{calls.push('history');}});
 assert.deepEqual(calls,['status','history']);
});

import { createExecutionRecovery, isRecoveryNotFound, subscribeFromLastCommittedCursor, type RecoveryPorts } from './recovery.ts';
import { projectExecutionSnapshot, projectExecutionEvent } from '../conversations/execution-projection.ts';
import type { ExecutionEvent, ExecutionSnapshot } from '@weknora/contracts';

test('active run restores history and then opens the stream', async () => {
  const calls: string[] = [];
  await recoverRun({
    status: async () => { calls.push('status'); return { terminal: false }; },
    subscribe: async () => { calls.push('stream'); },
    refreshHistory: async () => { calls.push('history'); },
  });
  assert.deepEqual(calls, ['status', 'history', 'stream']);
});

test('concurrent active events are single-flighted onto one recovery pass', async () => {
  const calls: string[] = [];
  let release!: () => void;
  const gate = new Promise<void>((done) => { release = done; });
  const ports: RecoveryPorts = {
    status: async () => { calls.push('status'); await gate; return { terminal: true }; },
    subscribe: async () => { calls.push('stream'); },
    refreshHistory: async () => { calls.push('history'); },
  };
  const controller = createExecutionRecovery({ ports });
  const first = controller.recover();
  const second = controller.recover();
  const third = controller.recover();
  release();
  await Promise.all([first, second, third]);
  assert.deepEqual(calls, ['status', 'history']);
});

test('scope switch cancels the in-flight recovery and never resumes its steps', async () => {
  const calls: string[] = [];
  let release!: () => void;
  const gate = new Promise<void>((done) => { release = done; });
  let generation = 1;
  const scope = {
    capture: () => ({ generation, signal: new AbortController().signal }),
    accept: (candidate: number) => candidate === generation,
    subscribe: (_listener: () => void) => () => undefined,
  };
  const ports: RecoveryPorts = {
    status: async () => { calls.push('status'); await gate; return { terminal: false }; },
    subscribe: async () => { calls.push('stream'); },
    refreshHistory: async () => { calls.push('history'); },
  };
  const controller = createExecutionRecovery({ ports, scope });
  const recovery = controller.recover();
  generation += 1; // identity transition: late responses must not keep flowing
  release();
  await recovery;
  assert.deepEqual(calls, ['status']);
});

test('HTTP 404 marks the recovery failed and never auto-creates a session', async () => {
  const calls: string[] = [];
  const ports: RecoveryPorts = {
    status: async () => { calls.push('status'); const error = new Error('execution stream HTTP 404') as Error & { status?: number }; error.status = 404; throw error; },
    subscribe: async () => { calls.push('stream'); },
    refreshHistory: async () => { calls.push('history'); },
  };
  const states: Array<{ status: string; notFound: boolean }> = [];
  const controller = createExecutionRecovery({ ports, onStatus: (status, error) => states.push({ status, notFound: isRecoveryNotFound(error) }) });
  await controller.recover();
  assert.deepEqual(calls, ['status']);
  assert.equal(states.at(-1)?.status, 'failed');
  assert.equal(states.at(-1)?.notFound, true);
});

test('background only closes the subscription; it never cancels the run', async () => {
  const calls: string[] = [];
  let closeStream!: () => void;
  const ports: RecoveryPorts = {
    status: async () => { calls.push('status'); return { terminal: false }; },
    refreshHistory: async () => { calls.push('history'); },
    subscribe: () => new Promise<void>((_resolve, reject) => { calls.push('stream'); closeStream = () => reject(new Error('RECOVERY_ABORTED')); }),
  };
  const controller = createExecutionRecovery({ ports });
  const recovery = controller.recover();
  await new Promise<void>((done) => { setImmediate(done); });
  controller.appStateChange('background');
  closeStream();
  await recovery; // the abort is absorbed: it is a lifecycle stop, not a failure
  assert.deepEqual(calls, ['status', 'history', 'stream']);
  assert.equal(controller.getState().state, 'idle');
  assert.equal(controller.getState().error, undefined);
  // Returning to the foreground replays the full controller sequence.
  const calls2: string[] = [];
  const second = createExecutionRecovery({ ports: {
    status: async () => { calls2.push('status'); return { terminal: true }; },
    subscribe: async () => { calls2.push('stream'); },
    refreshHistory: async () => { calls2.push('history'); },
  } });
  second.appStateChange('active');
  await new Promise<void>((done) => { setImmediate(done); });
  assert.deepEqual(calls2, ['status', 'history']);
});

test('isRecoveryNotFound recognizes handler-shaped 404 errors', () => {
  assert.equal(isRecoveryNotFound(new Error('execution stream HTTP 404')), true);
  const status = new Error('gone') as Error & { status?: number };
  status.status = 404;
  assert.equal(isRecoveryNotFound(status), true);
  assert.equal(isRecoveryNotFound(new Error('execution stream HTTP 500')), false);
  assert.equal(isRecoveryNotFound(undefined), false);
});

test('subscription resumes from the last committed W09 cursor, not from zero', async () => {
  const committed: ExecutionEvent[] = [
    { schema_version: 1, run_id: 'run-1', attempt_id: 'a1', seq: 3, type: 'text.delta', occurred_at: '2026-09-16T00:00:03Z', payload: { message_id: 'm1', delta: ' world' } },
    { schema_version: 1, run_id: 'run-1', attempt_id: 'a1', seq: 1, type: 'message.created', occurred_at: '2026-09-16T00:00:01Z', payload: { message: { id: 'm1', role: 'assistant', text: 'hello' } } },
    { schema_version: 1, run_id: 'run-1', attempt_id: 'a1', seq: 2, type: 'text.delta', occurred_at: '2026-09-16T00:00:02Z', payload: { message_id: 'm1', delta: ' brave' } },
  ];
  const cursors: string[] = [];
  const subscribe = subscribeFromLastCommittedCursor({
    read: async () => committed,
    stream: async (runID: string, lastEventID: string | undefined) => { cursors.push(`${runID}@${lastEventID ?? 'none'}`); },
  });
  await subscribe('run-1');
  assert.deepEqual(cursors, ['run-1@3']);
  // With no durable events yet the stream must start from the beginning.
  const cursors2: string[] = [];
  const subscribe2 = subscribeFromLastCommittedCursor({
    read: async () => [],
    stream: async (runID: string, lastEventID: string | undefined) => { cursors2.push(`${runID}@${lastEventID ?? 'none'}`); },
  });
  await subscribe2('run-2');
  assert.deepEqual(cursors2, ['run-2@none']);
});

// Harness chain 1: a knowledge-resource execution is answered with citations;
// after relaunch the restored history shows each message exactly once.
test('harness: knowledge-resource run recovers without duplicating history', async () => {
  const events: ExecutionEvent[] = [
    { schema_version: 1, run_id: 'run-kb', attempt_id: 'a1', seq: 1, type: 'message.created', occurred_at: '2026-09-16T00:00:01Z', payload: { message: { id: 'u1', role: 'user', text: '总结这份资料' } } },
    { schema_version: 1, run_id: 'run-kb', attempt_id: 'a1', seq: 2, type: 'message.created', occurred_at: '2026-09-16T00:00:02Z', payload: { message: { id: 'a1', role: 'assistant', text: '资料要点如下', blocks: [{ id: 'b1', kind: 'text', text: '资料要点如下' }, { id: 'b2', kind: 'text', text: '[1] 引用来源 knowledge-7' }] } } },
    { schema_version: 1, run_id: 'run-kb', attempt_id: 'a1', seq: 3, type: 'execution.succeeded', occurred_at: '2026-09-16T00:00:03Z', payload: { status: 'succeeded' } },
  ];
  const snapshot: ExecutionSnapshot = {
    execution: { run_id: 'run-kb', execution_status: 'succeeded', revision: 3 },
    events,
    watermark: 3,
  } as unknown as ExecutionSnapshot;
  // First visit: live projection consumes the events one by one.
  let live = projectExecutionSnapshot({ ...snapshot, events: [] } as unknown as ExecutionSnapshot);
  for (const event of events) live = projectExecutionEvent(live, event);
  // Relaunch: the controller treats the run as terminal, replays the durable
  // snapshot, and must not open a second stream over the same history.
  const calls: string[] = [];
  let restored = projectExecutionSnapshot(snapshot);
  await recoverRun({
    status: async () => { calls.push('status'); return { terminal: true }; },
    refreshHistory: async () => { calls.push('history'); restored = projectExecutionSnapshot(snapshot); },
    subscribe: async () => { calls.push('stream'); },
  });
  assert.deepEqual(calls, ['status', 'history']);
  assert.equal(restored.messages.length, live.messages.length);
  assert.deepEqual(restored.messages.map((message) => message.id), live.messages.map((message) => message.id));
  const citation = restored.messages.flatMap((message) => message.blocks ?? []).find((block) => block.text.includes('引用来源'));
  assert.ok(citation, 'cited knowledge answer stays in restored history');
});

// Harness chain 2: a no-KB custom agent run pauses on a controlled tool
// approval; recovery keeps the pending interaction addressable so the
// approve/reject decision still lands on the original run.
test('harness: controlled-tool run recovers its pending approval', async () => {
  const events: ExecutionEvent[] = [
    { schema_version: 1, run_id: 'run-agent', attempt_id: 'a1', seq: 1, type: 'tool.approval.requested', occurred_at: '2026-09-16T00:00:01Z', payload: { pending_interaction: { id: 'pi-1', kind: 'permission', label: '运行外部检索', revision: 4 } } },
    { schema_version: 1, run_id: 'run-agent', attempt_id: 'a1', seq: 2, type: 'message.created', occurred_at: '2026-09-16T00:00:02Z', payload: { message: { id: 'w1', role: 'tool', text: 'waiting for user decision' } } },
  ];
  const snapshot: ExecutionSnapshot = { execution: { run_id: 'run-agent', execution_status: 'waiting_user', revision: 2 }, events, watermark: 2 } as unknown as ExecutionSnapshot;
  const decisions: string[] = [];
  const calls: string[] = [];
  let projection = projectExecutionSnapshot(snapshot);
  await recoverRun({
    status: async () => { calls.push('status'); return { terminal: false }; },
    refreshHistory: async () => { calls.push('history'); projection = projectExecutionSnapshot(snapshot); },
    subscribe: async () => {
      calls.push('stream');
      // The user approves after recovery; the artifact message then lands on
      // the same run identity instead of a fresh session.
      projection = projectExecutionEvent(projection, { schema_version: 1, run_id: 'run-agent', attempt_id: 'a1', seq: 3, type: 'tool.approval.resolved', occurred_at: '2026-09-16T00:00:03Z', payload: { pending_interaction: { id: 'pi-1', kind: 'permission', status: 'approved', revision: 4 } } });
      decisions.push(`approve:pi-1@run-agent`);
      projection = projectExecutionEvent(projection, { schema_version: 1, run_id: 'run-agent', attempt_id: 'a1', seq: 4, type: 'message.created', occurred_at: '2026-09-16T00:00:04Z', payload: { message: { id: 'artifact-1', role: 'tool', text: '产物：检索结果已生成' } } });
    },
  });
  assert.deepEqual(calls, ['status', 'history', 'stream']);
  const pending = projection.pendingInteractions.find((item) => item.id === 'pi-1');
  assert.equal(pending?.status, 'approved');
  assert.deepEqual(decisions, ['approve:pi-1@run-agent']);
  const artifact = projection.messages.find((message) => message.id === 'artifact-1');
  assert.ok(artifact, 'tool artifact lands on the recovered run');
  assert.equal(projection.execution?.status, 'waiting_user');
});

// Reconciliation semantics: a start response dropped in flight recovers via
// lookup onto the original run; the client never issues a second start.
test('harness: dropped start response recovers via lookup onto the same run', async () => {
  const starts: string[] = [];
  const lookups: string[] = [];
  const runs = new Map<string, string>([['request-1', 'run-9']]);
  const statusCalls: string[] = [];
  const ports: RecoveryPorts = {
    status: async () => {
      statusCalls.push('status');
      lookups.push('request-1');
      return { terminal: true };
    },
    refreshHistory: async () => { statusCalls.push('history'); },
    subscribe: async () => { statusCalls.push('stream'); starts.push('start:again'); },
  };
  await recoverRun(ports);
  assert.deepEqual(starts, [], 'recovery never re-issues start');
  assert.deepEqual(statusCalls, ['status', 'history']);
  assert.equal(runs.get('request-1'), 'run-9');
});
