import assert from 'node:assert/strict';
import { test } from 'node:test';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScenarioResearchRemote, type ScenarioResearchRemoteScript } from './in-memory-research-remote.ts';
import { createTaskResearch, ResearchError, type ResearchBackendPort, type ResearchDraftsPort, type ResearchEvent } from './task-research.ts';

// ─── 测试夹具：真实可撤销 lease + 记账式 drafts + 可脚本化 remote ─────────────
// leaseActive 以 instanceof RuntimeScopeLease 判定（runtime/scope-lease.ts:26-27），
// 夹具必须用真类实例（同 task-office.test.ts:15 先例），普通对象伪造恒 false。
// 类型经 asScopeLease() 出手（同 task-office.test.ts:16 / task-material.test.ts:36 先例，
// ScopeLease 是 branded 不透明类型），revoke 走保留的 revocable 引用。

function newLease(): { lease: ScopeLease; revoke(): void } {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { lease: revocable.asScopeLease(), revoke: () => revocable.revoke() };
}

function codedError(code: string): Error {
  const error = new Error(code);
  (error as unknown as { code?: string }).code = code;
  return error;
}

function recordingDrafts(): ResearchDraftsPort & { rows: Map<string, unknown> } {
  const rows = new Map<string, unknown>();
  return {
    rows,
    async put(draft) { rows.set(draft.draftId, structuredClone(draft)); },
    async list() { return [...rows.values()] as never; },
    async remove(draftId) { rows.delete(draftId); },
  };
}

test('delegate validates scope and input before any network dispatch', async () => {
  const script: ScenarioResearchRemoteScript = { delegations: [] };
  const remote = createScenarioResearchRemote(script);
  let calls = 0;
  const counting: ResearchBackendPort = { ...remote, delegate: async (input) => { calls += 1; return remote.delegate(input); } };
  const activeGate = createTaskResearch({ remote: counting });
  const { lease, revoke } = newLease();
  const active = activeGate.open({ lease });

  // 非法输入（空 objective / 空 sources / 超量 sources / 空串源）→ 零网络。
  for (const bad of [
    { runId: 'r1', objective: '', sources: ['kb-1'] },
    { runId: 'r1', objective: 'x', sources: [] },
    { runId: 'r1', objective: 'x', sources: ['1', '2', '3', '4', '5', '6', '7', '8', '9'] },
    { runId: 'r1', objective: 'x', sources: ['kb-1', ' '] },
  ]) {
    await assert.rejects(active.delegate(bad), /RESEARCH_INVALID_INPUT/);
  }
  assert.equal(calls, 0);

  // 合法委派放行并返回行。
  const row = await active.delegate({ runId: 'r1', objective: 'survey', sources: ['kb-1'] });
  assert.equal(row.status, 'assigned');
  assert.equal(calls, 1);

  // 撤销后的迟到调用 → fail closed，零网络（Review Focus 5）。
  revoke();
  await assert.rejects(active.delegate({ runId: 'r1', objective: 'x', sources: ['kb-1'] }), /RESEARCH_SCOPE_CHANGED/);
  assert.equal(calls, 1);
});

test('annotations record, conflict-map and draft offline', async () => {
  const remote = createScenarioResearchRemote({ delegations: [] });
  const drafts = recordingDrafts();
  const events: ResearchEvent[] = [];
  const module = createTaskResearch({ remote, drafts });
  const handle = module.open({ lease: newLease().lease });
  handle.subscribe((event) => events.push(event));

  // 在线批注：直接 recorded。
  const recorded = await handle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b', body: '缺引用' });
  assert.equal(recorded.status, 'recorded');
  assert.equal(recorded.status === 'recorded' && recorded.annotation.baseVersion, '9a2f1c3d4e5f6a7b');

  // 版本冲突映射 RESEARCH_CONFLICT。
  await assert.rejects(
    handle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'stale', body: 'x' }),
    (error: unknown) => error instanceof ResearchError && error.code === 'RESEARCH_CONFLICT',
  );

  // 空批注体 → RESEARCH_INVALID_INPUT，零网络。
  await assert.rejects(handle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b', body: ' ' }), /RESEARCH_INVALID_INPUT/);

  // 离线批注：落加密草稿、零网络、可观测事件。
  const offlineGate = { status: async () => 'offline' as const, assertOnline: async () => { throw new Error('OFFLINE_ACTION_BLOCKED'); } };
  const offlineModule = createTaskResearch({ remote, gate: offlineGate, drafts });
  const offlineHandle = offlineModule.open({ lease: newLease().lease });
  offlineHandle.subscribe((event) => events.push(event)); // 事件挂在发起草稿的句柄上
  const drafted = await offlineHandle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b', body: '离线批注' });
  assert.equal(drafted.status, 'drafted');
  assert.match(drafted.status === 'drafted' ? drafted.draftId : '', /^research-ann-\d+$/);
  assert.equal((await offlineHandle.pendingDrafts()).length, 1);
  assert.ok(events.some((event) => event.type === 'annotation-drafted'));

  // 离线但无 drafts 端口 → RESEARCH_DRAFT_UNAVAILABLE（fail closed，不静默丢批注）。
  const noDrafts = createTaskResearch({ remote, gate: offlineGate });
  await assert.rejects(
    noDrafts.open({ lease: newLease().lease }).annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v', body: 'x' }),
    /RESEARCH_DRAFT_UNAVAILABLE/,
  );
});

test('flush replays drafts in order; conflict keeps the draft; revocation aborts the rest', async () => {
  let conflicts = 0;
  const base = createScenarioResearchRemote({ delegations: [] });
  const flaky: ResearchBackendPort = {
    ...base,
    annotate: async (input) => {
      if (input.body === 'stale') { conflicts += 1; throw new Error('RESEARCH_BASE_VERSION_CONFLICT'); }
      if (input.body === 'after-revoke') throw new Error('RESEARCH_SCOPE_CHANGED');
      return base.annotate(input);
    },
  };
  const drafts = recordingDrafts();
  const module = createTaskResearch({ remote: flaky, drafts });
  const offlineGate = { status: async () => 'offline' as const, assertOnline: async () => { throw new Error('x'); } };
  const handle = module.open({ lease: newLease().lease });
  const offline = createTaskResearch({ remote: flaky, gate: offlineGate, drafts });
  const offlineHandle = offline.open({ lease: newLease().lease });
  await offlineHandle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'ok-1' });
  await offlineHandle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'stale' });
  await offlineHandle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'ok-2' });
  assert.equal((await handle.pendingDrafts()).length, 3);

  const outcomes = await handle.flushAnnotationDrafts({ runId: 'r1' });
  assert.deepEqual(outcomes.map((o) => o.outcome), ['recorded', 'conflict', 'recorded']);
  assert.ok(conflicts >= 1);
  assert.equal((await handle.pendingDrafts()).length, 1, 'conflict 草稿保留待处理');

  // lease 撤销 → flush 中止且不发出其余草稿。
  const { lease, revoke } = newLease();
  const revocable = createTaskResearch({ remote: flaky, gate: offlineGate, drafts });
  const revocableOffline = createTaskResearch({ remote: flaky, gate: offlineGate, drafts });
  const rOffline = revocableOffline.open({ lease });
  await rOffline.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'ok-3' });
  await rOffline.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'after-revoke' });
  revoke();
  const rHandle = revocable.open({ lease });
  await assert.rejects(rHandle.flushAnnotationDrafts({ runId: 'r1' }), /RESEARCH_SCOPE_CHANGED/);
});

test('requestRevision composes a version-pinned text over the command port', async () => {
  const commands = {
    command: async (input: { text?: string; action: 'steer' | 'queue_next' | 'cancel' }) => {
      assert.equal(input.action, 'queue_next');
      assert.equal(input.text, '请基于版本 9a2f1c3d4e5f6a7b 修订材料 m1:0：补齐引用');
      return { runId: 'r1', action: input.action, nextRunId: 'r-next' };
    },
  };
  const module = createTaskResearch({
    remote: createScenarioResearchRemote({ delegations: [] }),
    commands,
  });
  const handle = module.open({ lease: newLease().lease });
  const receipt = await handle.requestRevision({
    runId: 'r1', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b',
    note: '补齐引用', action: 'queue_next', expectedRevision: 3,
  });
  assert.equal(receipt.outcome, 'accepted');
  assert.equal(receipt.nextRunId, 'r-next');

  // 无 commands → fail closed；命令冲突 → RESEARCH_COMMAND_CONFLICT。
  const bare = createTaskResearch({ remote: createScenarioResearchRemote({ delegations: [] }) });
  await assert.rejects(
    bare.open({ lease: newLease().lease }).requestRevision({ runId: 'r1', materialId: 'm', baseVersion: 'v', note: 'n', action: 'steer', expectedRevision: 1 }),
    /RESEARCH_COMMAND_UNAVAILABLE/,
  );
  const conflicting = createTaskResearch({
    remote: createScenarioResearchRemote({ delegations: [] }),
    // 真实 remote 的冲突形态：Error 携带 .code = 'TASK_COMMAND_CONFLICT'
    // （api-client task-office.ts:256-266 的 coded()，跨包契约码）。
    commands: { command: async () => { throw codedError('TASK_COMMAND_CONFLICT'); } },
  });
  await assert.rejects(
    conflicting.open({ lease: newLease().lease }).requestRevision({ runId: 'r1', materialId: 'm', baseVersion: 'v', note: 'n', action: 'steer', expectedRevision: 1 }),
    (error: unknown) => error instanceof ResearchError && error.code === 'RESEARCH_COMMAND_CONFLICT',
  );
});

test('closed handles reject every path with SCOPE_CHANGED and emit scope-closed', async () => {
  const events: ResearchEvent[] = [];
  const module = createTaskResearch({ remote: createScenarioResearchRemote({ delegations: [] }) });
  const handle = module.open({ lease: newLease().lease });
  handle.subscribe((event) => events.push(event));
  handle.close('navigate-away');
  await assert.rejects(handle.delegations('r1'), /RESEARCH_SCOPE_CHANGED/);
  await assert.rejects(handle.annotate({ runId: 'r1', materialId: 'm', baseVersion: 'v', body: 'b' }), /RESEARCH_SCOPE_CHANGED/);
  assert.ok(events.some((event) => event.type === 'scope-closed' && event.reason === 'navigate-away'));
});
