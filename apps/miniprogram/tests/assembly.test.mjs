import test from 'node:test';
import assert from 'node:assert/strict';
// 平台边界替换与脚手架（hook/me/capabilities/backend/authRoutes/freshLogin/until）共享自
// helpers/assembly-harness.mjs（最终审查修复 F5）；本文件只保留会话域场景本身。
import { loadAssemblyHarness } from './helpers/assembly-harness.mjs';

const { stub, runtime: runtimeModule, harness } = await loadAssemblyHarness();
const { me, backend, authRoutes, freshLogin, authorization, authKeys, until, setActiveTenant } = harness;
const files = await import('../src/platform/files.ts');

test('assembly: login through the real MobileRuntime stores exactly one credential and stamps scope', async () => {
  await freshLogin();
  assert.equal(runtimeModule.auth.snapshot().phase, 'ready');
  assert.equal(runtimeModule.auth.snapshot().tenantId, '1');
  assert.equal(JSON.stringify(runtimeModule.auth.snapshot()).includes('t1'), false, 'tokens never escape into observable UI state');
  assert.equal(authKeys().length, 1, 'exactly one credential key (single writer: the runtime)');
  const stored = stub.state.storage.get(authKeys()[0]);
  assert.ok(stored && (stored.token === 't1' || stored.accessToken === 't1'), 'runtime persists the credential in the store');
  assert.ok(runtimeModule.runtime.scopeLease(), 'authorization mints a scope lease for deep modules');
  assert.ok(runtimeModule.runtime.resourceShelf(), 'resource shelf opens with the lease');
  // 身份富集是 authorized 后的一次异步 /auth/me：轮询等待而非 sleep。
  assert.ok(await until(() => runtimeModule.auth.snapshot().userName === 'Lin'), 'identity enrichment reads /auth/me through the authorized channel');
});

test('assembly: concurrent 401s share one refresh and replay with the rotated token (all methods)', async () => {
  let refreshCount = 0, replayAuth = '';
  await freshLogin({
    'GET /api/v1/execution-targets': call => {
      if (authorization(call) === 'Bearer t1') { stub.succeed(call, { statusCode: 401, data: { success: false } }); return; }
      replayAuth = authorization(call);
      stub.succeed(call, { data: { success: true, data: [{ id: 'platform', kind: 'platform', state: 'active' }] } });
    },
    'POST /api/v1/auth/refresh': call => { refreshCount++; stub.succeed(call, { data: { success: true, access_token: 't2', refresh_token: 'r2' } }); },
  });
  const [a, b] = await Promise.all([
    runtimeModule.client.request({ method: 'GET', path: '/api/v1/execution-targets' }),
    runtimeModule.client.request({ method: 'GET', path: '/api/v1/execution-targets' }),
  ]);
  assert.equal(refreshCount, 1, 'concurrent 401s must share one refresh (runtime single-flight)');
  assert.equal(replayAuth, 'Bearer t2');
  assert.equal(a.success, true); assert.equal(b.success, true);
  assert.equal(runtimeModule.auth.credential().accessToken, 't2', 'just-in-time readers see the rotation immediately');
});

test('assembly: a definitive 401 on a POST is refreshed and replayed; an ambiguous network failure never replays', async () => {
  // 401 = 服务端未认证即拒绝，请求未被执行，重放安全（Runtime 语义，与本计划差异记录一致）。
  let posts = 0, refreshes = 0, lastAuth = '';
  await freshLogin({
    'POST /api/v1/workbench/executions': call => {
      posts++;
      if (authorization(call) === 'Bearer t1') { stub.succeed(call, { statusCode: 401, data: { success: false } }); return; }
      lastAuth = authorization(call);
      stub.succeed(call, { statusCode: 202, data: { success: true, data: { run_id: 'run-1', request_id: 'req-1', status: 'admitted' } } });
    },
    'POST /api/v1/auth/refresh': call => { refreshes++; stub.succeed(call, { data: { success: true, access_token: 't2', refresh_token: 'r2' } }); },
  });
  const ack = await runtimeModule.client.request({ method: 'POST', path: '/api/v1/workbench/executions', body: {} });
  // client.request 按已合并语义返回 {success,data} 信封（同文件 test 2 断言 a.success、workbench.ts
  // 以 data(await client.request(...)) 消费、executions.ts:154 unwrap 同形状）——run_id 在 data 内。
  assert.equal(ack.data.run_id, 'run-1');
  assert.equal(posts, 2, 'the 401 POST is replayed exactly once after refresh');
  assert.equal(refreshes, 1);
  assert.equal(lastAuth, 'Bearer t2');
  // 歧义失败（网络中断）：绝不重放——重复提交的风险面在这里，不在 401。
  // 注意只重装路由 handler（backend 直接覆盖），不 reset——凭据仓里的 t2 必须仍在。
  let lost = 0;
  backend(authRoutes({
    'POST /api/v1/workbench/executions': call => { lost++; call.options.fail({ errMsg: 'request lost' }); },
  }));
  await assert.rejects(runtimeModule.client.request({ method: 'POST', path: '/api/v1/workbench/executions', body: {} }));
  assert.equal(lost, 1, 'NETWORK_ERROR on a POST must never be retried');
});

test('assembly: 403 is surfaced as permission denial, never treated as 401 refresh', async () => {
  let refreshes = 0;
  await freshLogin({
    'GET /api/v1/execution-targets': call => stub.succeed(call, { statusCode: 403, data: { success: false } }),
    'POST /api/v1/auth/refresh': call => { refreshes++; stub.succeed(call, {}); },
  });
  await assert.rejects(runtimeModule.client.request({ method: 'GET', path: '/api/v1/execution-targets' }), error => error.status === 403);
  assert.equal(refreshes, 0);
});

test('assembly: a response landing after sign-out is discarded (RUNTIME_SCOPE_CHANGED)', async () => {
  await freshLogin({
    'GET /api/v1/execution-targets': () => {/* 挂起：等登出后再响应 */},
  });
  const pending = runtimeModule.client.request({ method: 'GET', path: '/api/v1/execution-targets' });
  await runtimeModule.runtime.signOut();
  const call = stub.lastCall('request');
  stub.succeed(call, { data: { success: true, data: [{ id: 'x', kind: 'platform', state: 'active' }] } });
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
  assert.equal(runtimeModule.auth.snapshot().phase, 'anonymous');
});

test('assembly: chatStream assembles SSE frames end-to-end through the authorized Taro stream channel', async () => {
  await freshLogin({
    'POST /api/v1/knowledge-chat/s-1': call => {
      assert.equal(call.options.header.authorization, 'Bearer t1', 'the runtime injects the bearer into the stream channel');
      stub.emitHeaders(call, { 'Content-Type': 'text/event-stream; charset=utf-8' });
      // 中文+emoji 逐字节拆分跨 chunk，验证真实 Utf8Decoder 与 SSE 解析装配。
      const wire = new TextEncoder().encode('data: {"response_type":"answer","content":"你好😀"}\r\n\r\n');
      for (const byte of wire) stub.emitChunk(call, Uint8Array.of(byte).buffer);
      stub.succeed(call, { statusCode: 200, header: { 'Content-Type': 'text/event-stream' }, data: '' });
    },
  });
  const events = [];
  await runtimeModule.chatStream({ sessionId: 's-1', body: { query: '问' } }, event => events.push(event));
  assert.equal(events.length, 1);
  assert.equal(events[0].response_type, 'answer');
  assert.equal(events[0].content, '你好😀');
});

test('assembly: tenant switch rotates the scope and aborts in-flight subscriptions', async () => {
  await freshLogin({
    'POST /api/v1/auth/switch-tenant': call => {
      setActiveTenant(2); // 切换后 me() 返回新租户（activateTenant 以 me() 为身份权威）
      stub.succeed(call, { data: { success: true, data: { token: 't3', refresh_token: 'r3', tenant: { id: 2, name: 'Space 2' }, memberships: [] } } });
    },
  });
  const before = runtimeModule.auth.scope.capture();
  const controller = runtimeModule.auth.scope.controller();
  await runtimeModule.auth.switchTenant(2);
  assert.equal(controller.signal.aborted, true, 'old subscriptions are aborted on switch');
  assert.equal(runtimeModule.auth.scope.isCurrent(before), false);
  assert.equal(runtimeModule.auth.snapshot().tenantId, '2');
  assert.equal(runtimeModule.auth.snapshot().tenantName, 'Space 2', 'tenantName derives synchronously from the memberships-backed tenant list');
});

test('assembly: bootstrap restores a stored credential without a fresh login', async () => {
  stub.reset();
  stub.state.storage.set('wk:auth:https://api.example.test', { token: 't1', refreshToken: 'r1' });
  backend(authRoutes());
  await runtimeModule.auth.bootstrap();
  assert.equal(runtimeModule.auth.snapshot().phase, 'ready');
  assert.equal(runtimeModule.auth.snapshot().userId, 'u1');
});

test('assembly: bootstrap with a dead network keeps an honest error, not a silent anonymous', async () => {
  stub.reset();
  stub.state.storage.set('wk:auth:https://api.example.test', { token: 't1', refreshToken: 'r1' });
  // me() 以网络失败拒绝（不能挂起——boot 会永远等不到响应）；boot 吞错回 deployment-login，
  // 凭据未被清除 ⇒ 门面给出可重试 error 而非静默登出。
  backend({ 'GET /api/v1/auth/me': call => call.options.fail({ errMsg: 'network down' }) });
  await runtimeModule.auth.bootstrap();
  assert.equal(runtimeModule.auth.snapshot().phase, 'error', 'stored credential + unreachable server = retryable error, not a silent logout');
});

test('assembly: logout revokes remotely best-effort, clears credentials and the private cache', async () => {
  await freshLogin({
    'POST /api/v1/auth/logout': call => call.options.fail({ errMsg: 'offline' }),
  });
  stub.state.storage.set('wk:recent-runs:x', ['run-1']);
  await runtimeModule.logout();
  assert.equal(runtimeModule.auth.snapshot().phase, 'anonymous');
  assert.equal(runtimeModule.auth.credential().kind, 'anonymous');
  assert.equal([...stub.state.storage.keys()].filter(k => k.startsWith('wk:')).length, 0, 'private cache and credentials removed even when remote revocation fails');
});

// 最终审查修复 F2 回归：受保护下载通道的 token 必须经 currentBearerToken() 唯一入口铸造
//（先授权 GET 触发 Runtime refresh-once，再「用时现读」），而非 auth.credential() 现读——
// 后者在 token 过期未轮换时会拿着死 token 直连，一次性 401。
test('assembly: the protected download channel mints its token through currentBearerToken — a stale token is refreshed before download', async () => {
  await freshLogin(); // t1 落盘
  // 重装路由（不 reset——凭据仓里的 t1 必须保留）：模拟 t1 已在服务端过期。
  let refreshes = 0, downloadAuth = '';
  backend(authRoutes({
    'GET /api/v1/auth/me': call => {
      if (authorization(call) === 'Bearer t1') { stub.succeed(call, { statusCode: 401, data: { success: false } }); return; }
      stub.succeed(call, { data: me() });
    },
    'POST /api/v1/auth/refresh': call => { refreshes++; stub.succeed(call, { data: { success: true, access_token: 't2', refresh_token: 'r2' } }); },
    'GET /api/v1/files/report': call => {
      downloadAuth = authorization(call);
      if (downloadAuth === 'Bearer t1') { stub.succeed(call, { statusCode: 401, data: { success: false } }); return; }
      stub.succeed(call, { statusCode: 200, header: { 'content-type': 'application/pdf' }, data: new TextEncoder().encode('%PDF-1').buffer, tempFilePath: 'wxfile://tmp/report.pdf' });
    },
  }));
  await files.openProtectedDocument('/api/v1/files/report', 'report.pdf');
  assert.equal(refreshes, 1, 'the stale token is refreshed exactly once through the runtime single-flight');
  assert.equal(downloadAuth, 'Bearer t2', 'the download header carries the rotated token, never the stale one');
  assert.equal(stub.lastCall('downloadFile').options.header.Authorization, 'Bearer t2');
  // 路径不可信 / 格式不许可仍在前置校验拒绝（不发起任何网络调用）。
  await assert.rejects(files.openProtectedDocument('https://evil.example/file.pdf', 'a.pdf'), /不可信下载路径/);
  await assert.rejects(files.openProtectedDocument('/api/v1/files/report', 'a.exe'), /授权 Web 工作台/);
});
