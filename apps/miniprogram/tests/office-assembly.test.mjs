import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
// 平台边界替换与脚手架（hook/me/capabilities/backend/authRoutes/freshLogin/until）共享自
// helpers/assembly-harness.mjs（最终审查修复 F5）；本文件只保留 Task Office 场景数据与场景本身。
import { loadAssemblyHarness } from './helpers/assembly-harness.mjs';

const { stub, runtime: runtimeModule, office, harness } = await loadAssemblyHarness({ withOffice: true });
const { backend, authRoutes, freshLogin, settle, until, setActiveTenant } = harness;

// activateTenant 以切换后的 me() 为身份权威（mobile-runtime.ts:426-429）：activeTenant 随路由翻转。
const runRow = (runId, sessionId, status, attention = 'none') => ({ run_id: runId, session_id: sessionId, title: `任务 ${runId}`, status, run_status: status, attention, created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z' });
// overview 行走 parseWorkbenchOverview.executionSummary（read-models.ts:118-129）：
// run_status/execution_status/settlement_status 皆必填——与 list 行（parseExecutionItem 的 status/created_at）不同 wire 模型。
const overviewRow = (runId, sessionId, status, attention = 'none') => ({ run_id: runId, session_id: sessionId, title: `任务 ${runId}`, run_status: status, execution_status: status, settlement_status: 'pending', attention, updated_at: '2026-09-24T01:00:00Z' });
const execDto = (seq = 5) => ({ schema_version: 1, run_id: 'run-1', session_id: 's-1', revision: 3, driver: 'platform', run_status: 'running', execution_status: 'running', settlement_status: 'pending', seq, capabilities: {} });
const execEvent = seq => ({ schema_version: 1, run_id: 'run-1', attempt_id: 'a-1', seq, type: 'progress', occurred_at: '2026-09-18T00:00:00Z', payload: { summary: 'working' } });

const overviewRoutes = () => ({
  'GET /api/v1/workbench/overview': call => stub.succeed(call, { data: { success: true, data: {
    pending_interactions: [{ id: 'i-1', kind: 'tool_approval', created_at: '2026-09-24T00:00:00Z' }],
    in_progress: [overviewRow('run-1', 's-1', 'running')],
    recently_completed: [overviewRow('run-9', 's-9', 'succeeded')],
    // counts 三键皆为 parseWorkbenchOverview 硬校验（contracts read-models.ts:135-137）。
    counts: { active_runs: 1, pending_interactions: 1, unread_notifications: 3 }, recent_artifacts: [], as_of: '2026-09-24T01:00:00Z',
  } } }),
});

test('scenario: home() aggregates three sections through TaskOffice (GET /workbench/overview)', async () => {
  await freshLogin(overviewRoutes());
  const o = office.requireTaskOffice();
  const home = await o.home();
  assert.equal(home.needsMe.length, 1);
  assert.equal(home.running[0].runId, 'run-1');
  assert.equal(home.recentlyCompleted[0].runId, 'run-9');
  assert.equal(home.unreadNotifications, 3);
});

test('scenario: tasks()/moreTasks() own the cursor and suppress duplicate runIds across pages', async () => {
  let page = 0;
  await freshLogin({
    'GET /api/v1/workbench/executions': call => {
      page += 1;
      const items = page === 1 ? [runRow('run-1', 's-1', 'running'), runRow('run-2', 's-2', 'waiting_user', 'required')] : [runRow('run-2', 's-2', 'waiting_user', 'required'), runRow('run-3', 's-3', 'succeeded')];
      stub.succeed(call, { data: { success: true, data: { items, next_cursor: page === 1 ? 'c2' : undefined } } });
    },
  });
  const o = office.requireTaskOffice();
  const first = await o.tasks({});
  assert.equal(first.items.length, 2);
  assert.equal(first.nextCursor, 'c2');
  const second = await o.moreTasks();
  assert.deepEqual(second.duplicateRunIds, ['run-2'], 'duplicate run across pages is observable, never rendered twice');
  assert.equal(second.items.filter(item => item.runId === 'run-2').length, 0);
  assert.equal(second.items.length, 1);
});

test('scenario: start() persists intent before dispatch and resubmits the SAME request_id after an unknown lookup', async () => {
  const starts = [], lookups = [];
  let failFirst = true;
  await freshLogin({
    // 会话 DTO 解析要求 is_pinned 布尔（api-client chat/sessions create 校验）。
    'POST /api/v1/sessions': call => stub.succeed(call, { data: { success: true, data: { id: 's-new', title: '', is_pinned: false } } }),
    'POST /api/v1/workbench/executions': call => {
      starts.push(call.options.data);
      if (failFirst) { failFirst = false; call.options.fail({ errMsg: 'request lost' }); return; }
      stub.succeed(call, { statusCode: 202, data: { success: true, data: { run_id: 'run-9', request_id: call.options.data.request_id, status: 'admitted' } } });
    },
    'GET /api/v1/workbench/executions/requests/': call => {
      lookups.push(new URL(call.options.url).pathname.split('/').pop());
      stub.succeed(call, { data: { success: true, data: { state: 'unknown' } } });
    },
  });
  const o = office.requireTaskOffice();
  const goal = { text: '整理本周工作，生成一份周报', agentId: 'agent-1', budgetUpper: 200 };
  // 差异记录（vs 本任务 brief）：brief 原断言期待第一次 start() reject NETWORK_ERROR；
  // 但已合并 domain coordinator（submission.ts:149-160）的 submit() 吞网络错误 → lookup
  // → 落 awaiting_reconciliation → **resolve**（dispatched:true 表「已尝试派发」），
  // 不盲目重发。按 brief 兜底条款以已合并语义为准：断言回执如实携带对账态 phase。
  const ambiguous = await o.start(goal);
  assert.equal(ambiguous.dispatched, true, 'dispatch was attempted');
  assert.equal(ambiguous.phase, 'awaiting_reconciliation', 'ambiguous outcome lands in reconciliation, never fabricated as bound');
  assert.equal(ambiguous.runId, undefined, 'no run id is invented while the outcome is unknown');
  assert.equal(starts.length, 1, 'no blind resubmission while the outcome is ambiguous');
  // 重入协议：同一意图的重试必须传回原 request_id（TaskOffice.start 的 options.requestId）；
  // 不传即是新意图（新 ID 是正确行为，非缺陷）。
  const receipt = await o.start(goal, { requestId: ambiguous.requestId });
  assert.equal(receipt.dispatched, true);
  assert.equal(receipt.runId, 'run-9');
  assert.equal(starts.length, 2, 'unknown lookup authorizes resubmission');
  assert.equal(starts[1].request_id, starts[0].request_id, 'the SAME intent keeps the SAME request_id (durable identity)');
  assert.equal(starts[1].session_id, 's-new', 'reentry reuses the persisted session, never creates a second one');
});

test('scenario: open() hydrates from the snapshot and appends SSE events past the watermark', async () => {
  let eventStreams = 0;
  await freshLogin({
    'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: { execution: execDto(5), task: { task_id: 's-1', title: '任务 run-1', attention: 'none' }, watermark: 5, incomplete: false, confirmed_watermark: 5, events: [execEvent(5)] } } }),
    'GET /api/v1/workbench/executions/run-1/events': call => {
      eventStreams += 1;
      assert.equal(call.options.header['Last-Event-ID'], '5', 'resume must continue from the snapshot watermark');
      stub.emitHeaders(call, { 'Content-Type': 'text/event-stream' });
      stub.emitChunk(call, new TextEncoder().encode(`data: ${JSON.stringify(execEvent(6))}\n\n`).buffer);
      // 真实 SSE 是长连接：推送事件后保持流打开（不 succeed）。
      // 若在非终态立即结束流，已合并 task-detail 的 auto-resync（interrupt→hydrate 重置
      // cursor 至服务端 watermark→重开流）会形成永动重连直至内存耗尽——假后端不得模拟瞬时流。
    },
  });
  const handle = office.requireTaskOffice().open({ taskId: 's-1', runId: 'run-1' });
  const updates = [];
  const off = handle.updates(view => updates.push(view));
  const initial = await handle.hydrate();
  assert.equal(initial.taskId, 's-1');
  assert.equal(initial.cursor, 5);
  assert.ok(await until(() => updates.some(view => view.cursor === 6)), 'streamed event appends past the watermark');
  const latest = updates.at(-1);
  assert.equal(latest.timeline.at(-1).seq, 6);
  off();
  handle.close('test-done');
  await settle();
  assert.equal(eventStreams, 1, 'no reconnect after close');
});

test('scenario: inbox() reads the cross-run pending list and decide() distinguishes recorded from superseded', async () => {
  const decisions = [];
  await freshLogin({
    'GET /api/v1/workbench/interactions': call => {
      const limit = new URL(call.options.url).searchParams.get('limit');
      assert.ok(Number(limit) >= 1, 'inbox carries an explicit limit');
      stub.succeed(call, { data: { success: true, data: [
        // pending wire：decision_id/action 空串为必填（contracts parseInteraction:61-66）。
        { id: 'i-1', run_id: 'run-1', kind: 'tool_approval', decision_id: '', action: '', args_hash: 'h1', expected_revision: 2, created_at: '2026-09-24T00:00:00Z' },
      ] } });
    },
    'POST /api/v1/workbench/executions/interactions/i-1/decisions': call => {
      decisions.push(call.options.data);
      if (decisions.length === 1) stub.succeed(call, { data: { success: true, data: { id: 'i-1', run_id: 'run-1', kind: 'tool_approval', decision_id: call.options.data.decision_id, action: 'reject', args_hash: 'h1', expected_revision: 2 } } });
      else stub.succeed(call, { statusCode: 409, data: { success: false, code: 'interaction_superseded' } });
    },
  });
  const o = office.requireTaskOffice();
  const inbox = await o.inbox();
  assert.equal(inbox.items.length, 1);
  assert.equal(inbox.items[0].runId, 'run-1');
  const item = inbox.items[0];
  const recorded = await o.decide({ item, action: 'reject' });
  assert.equal(recorded.status, 'recorded');
  assert.equal(recorded.record.action, 'reject');
  assert.ok(typeof decisions[0].decision_id === 'string' && decisions[0].decision_id !== '', 'decision_id is a non-empty durable identity minted by the module');
  const superseded = await o.decide({ item, action: 'reject' });
  assert.equal(superseded.status, 'superseded');
});

test('scenario: shelf browse projects the three classes with verdicts; a 403 marks the class forbidden and revokes', async () => {
  await freshLogin({
    'GET /api/v1/agents': call => stub.succeed(call, { data: { success: true, data: [{ id: 'agent-1', name: 'Helper', description: '帮手', is_builtin: true }], disabled_own_agent_ids: [] } }),
    'GET /api/v1/knowledge-bases': call => stub.succeed(call, { data: { success: true, data: [{ id: 7, name: '团队知识', knowledge_count: 3, updated_at: '2026-09-24T00:00:00Z' }] } }),
    'GET /api/v1/apps/connections': call => stub.succeed(call, { data: { success: true, data: [{ id: 'conn-1', kind: 'github', state: 'active' }] } }),
  });
  const shelf = office.activeResourceShelf();
  const page = await shelf.browse();
  assert.equal(page.agents.length, 1);
  assert.equal(page.agents[0].name, 'Helper');
  assert.equal(page.knowledge.length, 1);
  assert.equal(page.connections.length, 1);
  assert.equal(page.classVerdicts.agent.state, 'supported');
  const verdict = shelf.selection({ agentId: 'agent-1' });
  assert.deepEqual(verdict, { allowed: true, selection: { kind: 'agent', agentId: 'agent-1' } });
  // 403 → 类级 forbidden + authorization-revoked 事件（#33 语义）。
  // 只重装路由 handler（backend 覆盖式），不 reset——shelf 用内存 activeCredential，但保持 storage 完好是通用纪律。
  const events = [];
  shelf.subscribe(event => events.push(event));
  backend(authRoutes({
    'GET /api/v1/agents': call => stub.succeed(call, { statusCode: 403, data: { success: false } }),
  }));
  const forbidden = await shelf.browse();
  assert.equal(forbidden.classVerdicts.agent.state, 'forbidden');
  assert.equal(forbidden.agents.length, 0, 'failed class clears the projection, never backfills stale rows');
  assert.ok(events.some(event => event.type === 'authorization-revoked' && event.resourceClass === 'agent'));
});

test('scenario: material index + preview + terminal flow through TaskMaterial', async () => {
  // 行形状与 wire 键对齐已合并契约（api-client materials.test.ts:8-9）：list 键为 items，
  // 行字段 id/name/mime/version/source_run；signed-url 响应必须回带 artifact 行供 mobile-core 回验。
  const artifactRow = { index: 0, id: 'run-1:0', name: 'report.txt', mime: 'text/plain', version: '0123456789abcdef', size: 5, source_run: 'run-1' };
  await freshLogin({
    'GET /api/v1/workbench/executions/run-1/artifacts': call => stub.succeed(call, { data: { success: true, data: { items: [artifactRow], terminal: { available: true } } } }),
    'POST /api/v1/workbench/executions/run-1/artifacts/0/signed-url': call => stub.succeed(call, { data: { success: true, data: { url: 'https://api.example.test/download/report.txt', expires_at: '2026-09-24T01:00:00Z', artifact: artifactRow } } }),
    'GET /download/report.txt': call => stub.succeed(call, { statusCode: 200, header: { 'content-type': 'text/plain' }, data: new TextEncoder().encode('hello').buffer }),
    'GET /api/v1/workbench/executions/run-1/terminal-log': call => stub.succeed(call, { data: { success: true, data: { lines: [{ seq: 1, occurred_at: '2026-09-24T00:00:00Z', stream: 'stdout', text: 'build ok' }], next_cursor: 1 } } }),
  });
  const material = office.openActiveMaterial();
  const index = await material.index({ runId: 'run-1' });
  assert.equal(index.materials.length, 1);
  assert.equal(index.terminal.available, true);
  const entry = index.materials[0];
  assert.equal(entry.kind, 'artifact');
  const view = await material.open({ kind: entry.kind, runId: 'run-1', materialId: entry.materialId });
  assert.equal(view.kind, 'artifact');
  assert.equal(view.preview.state, 'supported', 'small text/plain preview is supported');
  assert.equal(view.text, 'hello');
  const terminal = await material.open({ kind: 'terminal', runId: 'run-1' });
  assert.equal(terminal.kind, 'terminal');
  assert.equal(terminal.lines[0].text, 'build ok');
  material.close('scenario-done');
});

test('scenario: tenant switch revokes the old office lease — late reads fail closed (TASK_OFFICE_SCOPE_CHANGED)', async () => {
  await freshLogin({
    ...overviewRoutes(),
    'POST /api/v1/auth/switch-tenant': call => { setActiveTenant(2); stub.succeed(call, { data: { success: true, data: { token: 't3', refresh_token: 'r3', tenant: { id: 2, name: 'Space 2' }, memberships: [] } } }); },
  });
  const staleOffice = office.requireTaskOffice();
  await runtimeModule.auth.switchTenant(2);
  await assert.rejects(staleOffice.home(), error => error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  const nextOffice = office.requireTaskOffice();
  const home = await nextOffice.home();
  assert.equal(home.running[0].runId, 'run-1', 'a fresh office for the new scope reads normally');
});

// 最终审查修复 F3 回归：materials 缓存只以 origin 为键（offices 为 origin::tenant），
// 切租户复用同一 TaskMaterial 实例——安全边界在 handle 层逐操作 leaseActive fail closed。
// 这里钉住：切租户后「旧 handle」的一切读必须拒绝；「新 open」携带新 lease 正常读取。
test('scenario: after a tenant switch the cached TaskMaterial instance serves a fresh handle, while the stale handle fails closed (MATERIAL_SCOPE_CHANGED)', async () => {
  await freshLogin({
    'POST /api/v1/auth/switch-tenant': call => { setActiveTenant(2); stub.succeed(call, { data: { success: true, data: { token: 't3', refresh_token: 'r3', tenant: { id: 2, name: 'Space 2' }, memberships: [] } } }); },
    'GET /api/v1/workbench/executions/run-1/artifacts': call => stub.succeed(call, { data: { success: true, data: { items: [], terminal: { available: false } } } }),
  });
  const staleHandle = office.openActiveMaterial();
  assert.ok(staleHandle, 'material opens while the lease is active');
  await runtimeModule.auth.switchTenant(2); // 旧 lease 被吊销
  await assert.rejects(staleHandle.index({ runId: 'run-1' }), error => error.code === 'MATERIAL_SCOPE_CHANGED', 'every operation on the stale handle fails closed per-operation');
  await assert.rejects(staleHandle.open({ kind: 'terminal', runId: 'run-1' }), error => error.code === 'MATERIAL_SCOPE_CHANGED', 'terminal reads are guarded too');
  // 缓存实例按 origin 复用：新 open 以新 lease 通过，逐操作 guard 一致放行。
  const freshHandle = office.openActiveMaterial();
  assert.notEqual(freshHandle, staleHandle, 'a fresh handle is minted for the new lease');
  const index = await freshHandle.index({ runId: 'run-1' });
  assert.equal(index.materials.length, 0);
  freshHandle.close('scenario-done');
});

test('scenario: resolveTaskForRun derives taskId from the authoritative run row (ADR-0004 task-is-session)', async () => {
  await freshLogin({
    'GET /api/v1/workbench/executions/run-1': call => stub.succeed(call, { data: { success: true, data: execDto(5) } }),
  });
  const resolved = await office.resolveTaskForRun('run-1');
  assert.deepEqual(resolved, { taskId: 's-1', runId: 'run-1' });
});

test('replace-dont-layer: execution/home pages reference the deep modules, never the deleted workbench controller', () => {
  const executionPages = readFileSync(new URL('../src/features/execution/pages.tsx', import.meta.url), 'utf8');
  const homePages = readFileSync(new URL('../src/features/home/pages.tsx', import.meta.url), 'utf8');
  for (const source of [executionPages, homePages]) {
    assert.equal(source.includes('services/workbench'), false);
    assert.equal(source.includes('core/execution'), false);
  }
  assert.ok(executionPages.includes('requireTaskOffice'));
  assert.ok(executionPages.includes('openActiveMaterial'));
  assert.ok(homePages.includes('requireTaskOffice') || homePages.includes('activeResourceShelf'));
});
