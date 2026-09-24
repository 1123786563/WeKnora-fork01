import test from 'node:test';
import assert from 'node:assert/strict';
import { emptyOverview, createScenarioTaskBackend } from '../task-office/in-memory-task-backend.ts';
import type { TaskBackendPort, TaskBackendStartInput } from '../task-office/task-office.ts';
import type { InteractionBackendPort, InteractionActionValue, InteractionKindValue, InboxItem, ResolvedDecisionRecord } from '../task-office/attention-inbox.ts';
import type { LegacyBackendTask, LegacyFollowUpInput, LegacyMessage, LegacyTaskBackendPort, LegacyTaskBackendPage } from '../task-office/legacy-tasks.ts';
import { createOfflineGate, OfflineGateError, type NetworkStatusPort } from './offline-gate.ts';
import { guardInteractionBackend, guardLegacyTaskBackend, guardTaskBackend } from './guarded-ports.ts';

const startInput: TaskBackendStartInput = {
  request_id: 'req-1', session_id: 'session-1', agent_id: 'agent-1', target_id: 'platform', workspace_ref: '', text: 'goal', budget_upper: 10,
};

const item: InboxItem = { interactionId: 'ix-1', runId: 'run-1', kind: 'tool_approval', argsHash: 'h', expectedRevision: 1, createdAt: '2026-09-24T00:00:00Z' };

const record: ResolvedDecisionRecord = {
  interactionId: 'ix-1', runId: 'run-1', kind: 'tool_approval', decisionId: 'dec-1', action: 'approve', argsHash: 'h', expectedRevision: 1,
};

const legacyPage = (): LegacyTaskBackendPage => ({ items: [{ taskId: 'legacy-1', title: '旧会话', attention: 'none', updatedAt: '2026-09-24T00:00:00Z' } as LegacyBackendTask] });

function counters() {
  const log: string[] = [];
  const backend: TaskBackendPort = {
    overview: async () => { log.push('overview'); return emptyOverview(); },
    list: async () => { log.push('list'); return { items: [] }; },
    archive: async (taskId: string) => { log.push(`archive:${taskId}`); },
    restore: async (taskId: string) => { log.push(`restore:${taskId}`); },
    createSession: async (input: { title: string }) => { log.push(`createSession:${input.title}`); return { sessionId: 'session-1' }; },
    start: async (input: TaskBackendStartInput) => { log.push(`start:${input.request_id}`); return { run_id: 'run-1', request_id: input.request_id, status: 'running' }; },
    lookup: async (requestId: string) => { log.push(`lookup:${requestId}`); return { state: 'unknown' as const }; },
  };
  const interactions: InteractionBackendPort = {
    inbox: async () => { log.push('inbox'); return []; },
    decide: async (input: { item: InboxItem; decisionId: string; action: InteractionActionValue }) => {
      log.push(`decide:${input.decisionId}`);
      return { ...record, decisionId: input.decisionId, action: input.action };
    },
  };
  const legacy: LegacyTaskBackendPort = {
    list: async () => { log.push('legacy:list'); return legacyPage(); },
    history: async (taskId: string) => { log.push(`legacy:history:${taskId}`); return [{ messageId: 'm-1', role: 'user', content: 'q' } as LegacyMessage]; },
    followUp: async (input: LegacyFollowUpInput) => { log.push(`legacy:followUp:${input.taskId}`); },
  };
  return { log, backend, interactions, legacy };
}

const online: NetworkStatusPort = { online: async () => true };
const offline: NetworkStatusPort = { online: async () => false };

test('offline start is refused before the underlying backend is touched (zero dispatch)', async () => {
  const { log, backend } = counters();
  const guarded = guardTaskBackend(backend, createOfflineGate(offline));
  await assert.rejects(guarded.start(startInput), (error: unknown) => error instanceof OfflineGateError && error.action === 'run');
  assert.deepEqual(log, [], 'an offline dangerous action must never reach the backend');
});

test('online start passes through; reads and archive/restore are never gated even offline', async () => {
  const { log, backend } = counters();
  const onlineGuarded = guardTaskBackend(backend, createOfflineGate(online));
  const ack = await onlineGuarded.start(startInput);
  assert.equal(ack.run_id, 'run-1');
  assert.deepEqual(log, ['start:req-1']);
  const offlineGuarded = guardTaskBackend(backend, createOfflineGate(offline)); // 全程离线
  await offlineGuarded.overview();
  await offlineGuarded.archive('task-1');
  await offlineGuarded.restore('task-1');
  assert.deepEqual(log, ['start:req-1', 'overview', 'archive:task-1', 'restore:task-1'], 'approved reads and archive lifecycle are not dangerous actions');
});

test('offline decide is refused as approval before dispatch; inbox reads pass', async () => {
  const { log, interactions } = counters();
  const guarded = guardInteractionBackend(interactions, createOfflineGate(offline));
  const view = await guarded.inbox({ limit: 10 });
  assert.deepEqual(view, []);
  await assert.rejects(
    guarded.decide({ item, decisionId: 'dec-1', action: 'approve' }),
    (error: unknown) => error instanceof OfflineGateError && error.action === 'approval',
  );
  assert.deepEqual(log, ['inbox'], 'the offline decision must be the only blocked call');
});

test('offline legacy follow-up is refused as run; list/history reads pass', async () => {
  const { log, legacy } = counters();
  const guarded = guardLegacyTaskBackend(legacy, createOfflineGate(offline));
  const page = await guarded.list({});
  assert.equal(page.items.length, 1);
  await guarded.history('legacy-1');
  await assert.rejects(
    guarded.followUp({ taskId: 'legacy-1', question: '追问' }),
    (error: unknown) => error instanceof OfflineGateError && error.action === 'run',
  );
  assert.deepEqual(log, ['legacy:list', 'legacy:history:legacy-1']);
});

test('guards preserve the scenario backend structure (structural compatibility with the port)', async () => {
  const scenario = createScenarioTaskBackend({});
  const guarded = guardTaskBackend(scenario, createOfflineGate(online));
  const methods: Array<keyof TaskBackendPort> = ['overview', 'list', 'archive', 'restore', 'createSession', 'start', 'lookup'];
  for (const method of methods) assert.equal(typeof guarded[method], 'function', `${String(method)} must survive the guard`);
});
