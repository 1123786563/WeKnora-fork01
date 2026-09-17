import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

// 平台边界替换：真实 @tarojs/taro 在 Node 下因 webpack DefinePlugin 常量无法求值，
// 用契约级替身承载 request/uploadFile/storage；其余全部为待提交真实源码。
const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';
const { stub } = await import('./helpers/taro-stub.mjs');
const runtime = await import('../src/services/runtime.ts');
const workbench = await import('../src/services/workbench.ts');

const ORIGIN = 'https://api.example.test';
const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
const execDto = (seq = 5) => ({ schema_version: 1, run_id: 'run-1', session_id: 's-1', revision: 3, driver: 'platform', run_status: 'running', execution_status: 'running', settlement_status: 'pending', seq, capabilities: {} });
const execEvent = seq => ({ schema_version: 1, run_id: 'run-1', attempt_id: 'a-1', seq, type: 'progress', occurred_at: '2026-09-18T00:00:00Z', payload: { summary: 'working' } });

/** 安装按 method+pathname 路由的假后端；route 返回 undefined 时挂起（等测试手动响应）。 */
function backend(routes) {
  stub.use(call => {
    const method = call.options.method, path = new URL(call.options.url).pathname;
    const exact = routes[`${method} ${path}`];
    let fn = exact;
    if (fn === undefined) {
      // 以 '/' 结尾的键按前缀匹配，承载 :run_id / :request_id 路径参数。
      const prefix = Object.keys(routes).filter(k => k.endsWith('/') && `${method} ${path}`.startsWith(k)).sort((a, b) => b.length - a.length)[0];
      if (prefix) fn = routes[prefix];
    }
    if (fn === undefined) { call.options.fail({ errMsg: `no backend route for ${method} ${path}` }); return; }
    fn(call);
  });
}
function freshLogin(extraRoutes = {}) {
  stub.reset();
  backend({
    'POST /api/v1/auth/login': call => stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }),
    'GET /api/v1/auth/me': call => stub.succeed(call, { data: me() }),
    ...extraRoutes,
  });
  return runtime.auth.login('u@example.test', 'pw');
}
const authorization = call => call.options.header.Authorization ?? call.options.header.authorization;

test('assembly: login through real client+transport+AuthCoordinator stores credentials and stamps scope', async () => {
  await freshLogin();
  assert.equal(runtime.auth.snapshot().phase, 'ready');
  assert.equal(runtime.auth.snapshot().tenantId, '1');
  const loginCall = stub.state.calls.find(c => new URL(c.options.url).pathname === '/api/v1/auth/login');
  assert.equal(authorization(loginCall), undefined, 'login must not carry a stale Authorization header');
  const stored = [...stub.state.storage.keys()].filter(k => k.startsWith('wk:auth:'));
  assert.equal(stored.length, 1, 'credential persisted exactly once');
});

test('assembly: scoped GET 401 refreshes single-flight and replays with the rotated token', async () => {
  let meCount = 0, refreshCount = 0, replayAuth = '';
  await freshLogin({
    'GET /api/v1/execution-targets': call => {
      if (authorization(call) === 'Bearer t1') { stub.succeed(call, { statusCode: 401, data: { success: false } }); return; }
      replayAuth = authorization(call);
      stub.succeed(call, { data: { success: true, data: [{ id: 'platform', kind: 'platform', state: 'active' }] } });
    },
    'POST /api/v1/auth/refresh': call => { refreshCount++; stub.succeed(call, { data: { success: true, access_token: 't2', refresh_token: 'r2' } }); },
  });
  const [a, b] = await Promise.all([
    runtime.client.request({ method: 'GET', path: '/api/v1/execution-targets' }),
    runtime.client.request({ method: 'GET', path: '/api/v1/execution-targets' }),
  ]);
  assert.equal(refreshCount, 1, 'concurrent 401s must share one refresh (single-flight through the real coordinator)');
  assert.equal(replayAuth, 'Bearer t2', 'replayed request must carry the rotated token');
  assert.equal(a.success, true); assert.equal(b.success, true);
});

test('assembly: write requests never auto-retry or refresh on 401', async () => {
  let posts = 0, refreshes = 0;
  await freshLogin({
    'POST /api/v1/workbench/executions': call => { posts++; stub.succeed(call, { statusCode: 401, data: { success: false } }); },
    'POST /api/v1/auth/refresh': call => { refreshes++; stub.succeed(call, { data: { success: true, access_token: 't2', refresh_token: 'r2' } }); },
  });
  await assert.rejects(runtime.client.request({ method: 'POST', path: '/api/v1/workbench/executions', body: {} }));
  assert.equal(posts, 1, 'the POST must not be replayed');
  assert.equal(refreshes, 0, 'a failed write must not trigger credential refresh/retry');
});

test('assembly: 403 is surfaced as permission denial, never treated as 401 refresh', async () => {
  let refreshes = 0;
  await freshLogin({
    'GET /api/v1/execution-targets': call => stub.succeed(call, { statusCode: 403, data: { success: false } }),
    'POST /api/v1/auth/refresh': call => { refreshes++; stub.succeed(call, {}); },
  });
  await assert.rejects(runtime.client.request({ method: 'GET', path: '/api/v1/execution-targets' }), error => error.status === 403);
  assert.equal(refreshes, 0, '403 must not enter the refresh path');
});

test('assembly: a response landing after a scope change is discarded (SCOPE_CHANGED)', async () => {
  await freshLogin({
    'GET /api/v1/execution-targets': () => {/* 挂起：等切换后再响应 */}
  });
  const pending = runtime.client.request({ method: 'GET', path: '/api/v1/execution-targets' });
  runtime.auth.clear(); // 注销/切空间使旧 scope 失效
  const call = stub.lastCall('request');
  stub.succeed(call, { data: { success: true, data: [{ id: 'x', kind: 'platform', state: 'active' }] } });
  // 中止先于响应到达：表现为 CANCELLED（已中止）或 SCOPE_CHANGED（丢弃），都证明旧数据未回写。
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
});

test('assembly: chatStream assembles SSE frames end-to-end through the native chunk boundary', async () => {
  await freshLogin({
    'POST /api/v1/knowledge-chat/s-1': call => {
      stub.emitHeaders(call, { 'Content-Type': 'text/event-stream; charset=utf-8' });
      // 中文+emoji 逐字节拆分跨 chunk，验证真实 Utf8Decoder 与 SSE 解析装配。
      const wire = new TextEncoder().encode('data: {"response_type":"answer","content":"你好😀"}\r\n\r\n');
      for (const byte of wire) stub.emitChunk(call, Uint8Array.of(byte).buffer);
      stub.succeed(call, { statusCode: 200, header: { 'Content-Type': 'text/event-stream' }, data: '' });
    },
  });
  const events = [];
  await runtime.chatStream({ sessionId: 's-1', body: { query: '问' } }, event => events.push(event));
  assert.equal(events.length, 1);
  assert.equal(events[0].response_type, 'answer');
  assert.equal(events[0].content, '你好😀');
});

test('assembly: watchExecution installs the snapshot then appends streamed events from the watermark', async () => {
  await freshLogin({
    'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: { execution: execDto(5), watermark: 5, events: [execEvent(5)] } } }),
    'GET /api/v1/workbench/executions/run-1/events': call => {
      assert.equal(call.options.header['Last-Event-ID'], '5', 'resume must continue from the snapshot watermark');
      stub.emitHeaders(call, { 'Content-Type': 'text/event-stream' });
      stub.emitChunk(call, new TextEncoder().encode(`data: ${JSON.stringify(execEvent(6))}\n\n`).buffer);
      stub.succeed(call, { statusCode: 200, header: { 'Content-Type': 'text/event-stream' }, data: '' });
    },
  });
  const updates = [];
  await workbench.watchExecution('run-1', new AbortController().signal, value => updates.push(value));
  assert.equal(updates.length, 2, 'snapshot projection then streamed append');
  assert.equal(updates[0].cursor, 5);
  assert.equal(updates[1].cursor, 6);
  assert.equal(updates[1].events.at(-1).seq, 6);
  assert.deepEqual(workbench.recentRuns().slice(0, 1), ['run-1'], 'watched run is remembered per scope');
});

test('assembly: startTask unknown after a lost response resubmits the SAME request id (D5)', async () => {
  let startCalls = [];
  const admit = call => {
    startCalls.push(call);
    const requestId = call.options.data.request_id;
    stub.succeed(call, { statusCode: 202, data: { success: true, data: { run_id: 'run-9', request_id: requestId, status: 'admitted' } } });
  };
  await freshLogin({
    'POST /api/v1/workbench/executions': call => {
      if (startCalls.length === 0) { startCalls.push(call); call.options.fail({ errMsg: 'request lost' }); return; }
      admit(call);
    },
    'GET /api/v1/workbench/executions/requests/': () => {},
  });
  const input = { session_id: 's-1', agent_id: 'agent-1', target_id: 'platform', workspace_ref: '', text: '做点事', budget_upper: 100 };
  await assert.rejects(workbench.startTask(input), /NETWORK_ERROR/);
  assert.equal(startCalls.length, 1);
  // 服务端明确查询结果：unknown（无持久记录，见 admission.go LookupRequest 的 ErrRecordNotFound 分支）
  stub.use(call => {
    const path = new URL(call.options.url).pathname;
    if (path.startsWith('/api/v1/workbench/executions/requests/')) {
      stub.succeed(call, { data: { success: true, data: { state: 'unknown' } } }); return;
    }
    if (path === '/api/v1/workbench/executions' && call.options.method === 'POST') { admit(call); return; }
    call.options.fail({ errMsg: `unexpected ${call.options.method} ${path}` });
  });
  const runId = await workbench.startTask(input);
  assert.equal(runId, 'run-9');
  assert.equal(startCalls.length, 2, 'unknown lookup must allow resubmission');
  assert.equal(startCalls[1].options.data.request_id, startCalls[0].options.data.request_id, 'the SAME intent keeps the SAME request_id');
  assert.equal(workbench.pendingIntent().current(), null, 'admitted intent is acknowledged');
});

test('assembly: duplicate taps while a submission is unresolved never create a second intent', async () => {
  let started = 0;
  await freshLogin({
    'GET /api/v1/workbench/executions/requests/': call => stub.succeed(call, { data: { success: true, data: { state: 'pending' } } }),
    'POST /api/v1/workbench/executions': call => { started++; call.options.fail({ errMsg: 'lost' }); },
  });
  const input = { session_id: 's-1', agent_id: 'agent-1', target_id: 'platform', workspace_ref: '', text: '任务', budget_upper: 100 };
  await assert.rejects(workbench.startTask(input), /NETWORK_ERROR/);
  const second = workbench.startTask(input);
  await assert.rejects(second, /前一次任务仍待确认|原请求/);
  assert.equal(started, 1, 'second tap must not issue another POST');
  const pending = workbench.pendingIntent().current();
  assert.ok(pending && pending.requestId, 'the original intent stays durable');
});

test('assembly: wire paths used by the miniprogram match the Go route table', async () => {
  stub.reset();
  backend({
    'POST /api/v1/auth/login': call => stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }),
    'GET /api/v1/auth/me': call => stub.succeed(call, { data: me() }),
    'GET /api/v1/execution-targets': call => stub.succeed(call, { data: { success: true, data: [] } }),
    'GET /api/v1/workbench/executions/run-1': call => stub.succeed(call, { data: { success: true, data: execDto(5) } }),
    'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: { execution: execDto(5), watermark: 5, events: [] } } }),
    'GET /api/v1/workbench/executions/run-1/events': () => {},
    'GET /api/v1/workbench/executions/requests/req-1': call => stub.succeed(call, { data: { success: true, data: { state: 'unknown' } } }),
    'GET /api/v1/workbench/executions/run-1/interactions': call => stub.succeed(call, { data: { success: true, data: [{ id: 'i-1', kind: 'tool_approval', args_hash: 'h', expected_revision: 2, decision_id: '', action: '' }] } }),
    'POST /api/v1/workbench/executions/interactions/i-1/decisions': call => stub.succeed(call, { data: { success: true, data: { id: 'i-1' } } }),
    'POST /api/v1/workbench/executions/run-1/commands': call => stub.succeed(call, { data: { success: true, data: { run_id: 'run-1', action: 'cancel' } } }),
    'POST /api/v1/auth/logout': call => stub.succeed(call, { data: { success: true } }),
  });
  await runtime.auth.login('u', 'pw');
  await workbench.targets();
  await runtime.executions.get('run-1');
  await runtime.executions.snapshot('run-1');
  runtime.executions.lookup('req-1');
  await workbench.interactions('run-1');
  await workbench.rejectInteraction({ id: 'i-1', kind: 'tool_approval', argsHash: 'h', revision: 2, decisionId: '', action: '' });
  await runtime.executions.command('run-1', { action: 'cancel', expected_revision: 3 });
  await runtime.logout();
  const seen = [...new Set(stub.paths())];
  // 与 internal/router/routes_workbench.go / routes_auth*.go 逐条对应；
  // GET /workbench/executions 列表在 Go 侧尚未注册（not-implemented），不在此出现。
  const expected = [
    'POST /api/v1/auth/login', 'GET /api/v1/auth/me', 'GET /api/v1/execution-targets',
    'GET /api/v1/workbench/executions/run-1', 'GET /api/v1/workbench/executions/run-1/snapshot',
    'GET /api/v1/workbench/executions/run-1/events', 'GET /api/v1/workbench/executions/requests/req-1',
    'GET /api/v1/workbench/executions/run-1/interactions', 'POST /api/v1/workbench/executions/interactions/i-1/decisions',
    'POST /api/v1/workbench/executions/run-1/commands', 'POST /api/v1/auth/logout',
  ];
  for (const path of seen) assert.ok(expected.includes(path), `unexpected wire path: ${path}`);
  for (const path of expected.slice(0, 5).concat(expected.slice(6))) assert.ok(seen.includes(path), `missing expected wire path: ${path}`);
});

test('assembly: logout clears credentials and private cache but keeps the flow silent on network errors', async () => {
  await freshLogin({
    'POST /api/v1/auth/logout': call => call.options.fail({ errMsg: 'offline' }),
  });
  stub.state.storage.set('wk:recent-runs:x', ['run-1']);
  await runtime.logout();
  assert.equal(runtime.auth.snapshot().phase, 'anonymous');
  assert.equal(runtime.auth.credential().kind, 'anonymous');
  assert.equal([...stub.state.storage.keys()].filter(k => k.startsWith('wk:')).length, 0, 'private cache removed even when remote revocation fails');
});
