import test from 'node:test';
import assert from 'node:assert/strict';
import { projectExecutionEvent, projectExecutionSnapshot } from './execution-projection.ts';

const base = { schema_version: 1 as const, run_id: 'run-1', session_id: 's-1', revision: 3, driver: 'platform' as const, run_status: 'running' as const, execution_status: 'running', settlement_status: 'reserved', seq: 4, capabilities: {} };
const event = (seq: number, type: string, payload: Record<string, unknown>) => ({ schema_version: 1 as const, run_id: 'run-1', attempt_id: 'a-1', seq, type, occurred_at: '2026-09-16T00:00:00Z', payload });

test('projects persisted W09 snapshot events into text/tool/thinking and pending state', () => {
  const result = projectExecutionSnapshot({ execution: base, watermark: 4, events: [
    event(1, 'message.created', { message: { id: 'm1', role: 'assistant', blocks: [{ kind: 'text', text: 'hello' }, { kind: 'thinking', text: 'plan' }, { kind: 'tool', text: 'search' }] } }),
    event(2, 'approval.pending', { pending_interaction: { id: 'p1', label: 'Search', revision: 4 } }),
    event(3, 'message.created', { message: { id: 'm1', blocks: [{ kind: 'text', text: 'hello again' }] } }),
  ] });
  assert.equal(result.messages.length, 1);
  assert.equal(result.messages[0]?.blocks?.find((block) => block.kind === 'tool')?.text, 'search');
  assert.equal(result.messages[0]?.blocks?.find((block) => block.kind === 'thinking')?.text, 'plan');
  assert.equal(result.pendingInteractions[0]?.revision, 4);
  assert.equal(result.execution?.runID, 'run-1');
  assert.equal(result.watermark, 4);
});

test('merges stable message IDs and text deltas without duplicating entities', () => {
  const snapshot = projectExecutionSnapshot({ execution: base, watermark: 1, events: [
    event(1, 'message.created', { message: { id: 'm1', blocks: [{ id: 'b1', kind: 'text', text: 'hel' }] } }),
  ] });
  const next = projectExecutionEvent(snapshot, event(2, 'text.delta', { message_id: 'm1', blocks: [{ id: 'b1', kind: 'text', text: 'lo' }], delta: 'lo' }));
  const replay = projectExecutionEvent(next, event(2, 'text.delta', { message_id: 'm1', blocks: [{ id: 'b1', kind: 'text', text: 'lo' }], delta: 'lo' }));
  assert.equal(replay.messages.length, 1);
  assert.equal(replay.messages[0]?.text, 'hello');
  assert.equal(replay.messages[0]?.blocks?.[0]?.text, 'hello');
});

test('identity-less deltas are fail-closed instead of becoming one message per sequence', () => {
  const result = projectExecutionSnapshot({ execution: base, watermark: 2, events: [
    event(1, 'text.delta', { delta: 'a' }),
    event(2, 'text.delta', { delta: 'b' }),
  ] });
  assert.equal(result.messages.length, 0);
});

test('merges local W09 events with server snapshot exactly once by sequence', () => {
  const duplicate = event(1, 'message.created', { text: 'same' });
  const result = projectExecutionSnapshot({ execution: base, watermark: 1, events: [duplicate] }, [duplicate]);
  assert.equal(result.messages.length, 1);
});
