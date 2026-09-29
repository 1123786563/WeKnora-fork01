import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
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
const { loadAssemblyHarness } = await import('./helpers/assembly-harness.mjs');

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
    'GET /api/v1/system/capabilities': call => stub.succeed(call, { data: { code: 0, msg: 'success', data: { protocol_minimum: 1, protocol_maximum: 5 } } }),
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
  const logout = runtime.auth.logout(); // 注销/切空间使旧 scope 失效
  const call = stub.lastCall('request');
  stub.succeed(call, { data: { success: true, data: [{ id: 'x', kind: 'platform', state: 'active' }] } });
  await logout;
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

test('assembly: Task list behavior routes through the dedicated TaskOffice service', async () => {
  const { stub: officeStub, office, harness } = await loadAssemblyHarness({ withOffice: true });
  await harness.freshLogin({
    'GET /api/v1/workbench/executions': call => officeStub.succeed(call, { data: { success: true, data: { items: [
      { run_id: 'run-1', session_id: 'session-1', title: '整理资料', status: 'running', run_status: 'running', attention: 'none', created_at: '2026-09-29T00:00:00Z', updated_at: '2026-09-29T00:00:00Z' },
    ] } } }),
  });
  const page = await office.requireTaskOffice().tasks({});
  assert.equal(page.items.length, 1);
  assert.equal(page.items[0].runId, 'run-1');
  assert.ok(officeStub.paths().includes('GET /api/v1/workbench/executions'));
  assert.equal(workbench.startTask, undefined, 'Task submission remains owned by TaskOffice');
  assert.equal(typeof runtime.auth.switchTenant, 'function', 'session switching remains owned by the auth session');
});

test('assembly: TaskOffice and session client paths have matching Go route registrations', () => {
  const workbenchRoutes = readFileSync(new URL('../../../internal/router/routes_workbench.go', import.meta.url), 'utf8');
  const chatRoutes = readFileSync(new URL('../../../internal/router/routes_chat.go', import.meta.url), 'utf8');
  const clientExecutions = readFileSync(new URL('../../../packages/api-client/src/mobile/executions.ts', import.meta.url), 'utf8');
  const clientSessions = readFileSync(new URL('../../../packages/api-client/src/chat/sessions.ts', import.meta.url), 'utf8');

  const functionBlock = (source, name) => {
    const start = source.indexOf(`func ${name}(`);
    assert.notEqual(start, -1, `missing route function ${name}`);
    const next = source.indexOf('\nfunc ', start + 1);
    return source.slice(start, next < 0 ? source.length : next);
  };
  const registrationTuples = (source, functionName) => {
    const body = functionBlock(source, functionName);
    const groups = [...body.matchAll(/(\w+)\s*:=\s*[^\n]*?r\.Group\("([^"]*)"[^\n]*\)/g)];
    return groups.flatMap((group, index) => {
      const end = groups[index + 1]?.index ?? body.length;
      const scope = body.slice(group.index, end);
      const aliases = [...scope.matchAll(new RegExp(`(\\w+)\\s*:=\\s*\\w+\\.apiKeyGroup\\(${group[1]}\\s*,`, 'g'))].map(match => match[1]);
      const variables = [group[1], ...aliases];
      return variables.flatMap(variable => [...scope.matchAll(new RegExp(`\\b${variable}\\.(GET|POST|PUT|PATCH|DELETE)\\("([^"]*)"`, 'g'))])
        .map((match) => [`/api/v1${group[2]}`, match[1], match[2]]);
    });
  };
  const registered = [
    ...['RegisterWorkbenchRoutes', 'RegisterWorkbenchStartRoutes', 'RegisterWorkbenchOverviewRoutes', 'RegisterWorkbenchCommandRoutes', 'RegisterWorkbenchTaskStateRoutes'].flatMap(name => registrationTuples(workbenchRoutes, name)),
    ...['RegisterSessionRoutes'].flatMap(name => registrationTuples(chatRoutes, name)),
  ];
  const hasRoute = (prefix, method, suffix) => assert.ok(
    registered.some(([group, verb, subpath]) => group === prefix && verb === method && subpath === suffix),
    `missing Go route tuple ${method} ${prefix}${suffix}`,
  );

  // Group bindings and complete subpaths are compared as exact tuples so a route
  // in a neighboring registration group cannot satisfy the contract.
  hasRoute('/api/v1/workbench/executions', 'GET', '');
  hasRoute('/api/v1/workbench/executions', 'GET', '/:run_id');
  hasRoute('/api/v1/workbench/executions', 'GET', '/:run_id/snapshot');
  hasRoute('/api/v1/workbench/executions', 'GET', '/:run_id/events');
  hasRoute('/api/v1/workbench/executions', 'POST', '');
  hasRoute('/api/v1/workbench/executions/requests', 'GET', '/:request_id');
  hasRoute('/api/v1/sessions', 'POST', '');
  hasRoute('/api/v1/workbench/executions', 'POST', '/:run_id/commands');
  hasRoute('/api/v1/workbench/executions', 'POST', '/interactions/:id/decisions');
  hasRoute('/api/v1/workbench/interactions', 'GET', '');
  hasRoute('/api/v1/workbench/tasks', 'POST', '/:task_id/archive');
  hasRoute('/api/v1/workbench/tasks', 'DELETE', '/:task_id/archive');

  const clientOverview = readFileSync(new URL('../../../packages/api-client/src/mobile/overview.ts', import.meta.url), 'utf8');
  const clientTaskOffice = readFileSync(new URL('../../../packages/api-client/src/mobile/task-office.ts', import.meta.url), 'utf8');
  const clientInteractions = readFileSync(new URL('../../../packages/api-client/src/mobile/interactions.ts', import.meta.url), 'utf8');
  const pair = (source, method, path) => assert.match(source, new RegExp(`method:\\s*'${method}',[\\s\\S]{0,220}?path:\\s*${path}`));
  hasRoute('/api/v1/workbench', 'GET', '/overview');
  pair(clientOverview, 'GET', "'\\/api\\/v1\\/workbench\\/overview'");
  pair(clientExecutions, 'GET', '`/api/v1/workbench/executions\\$\\{query === \'\' \\? \'\' : `\\?\\$\\{query\\}`\\}`');
  pair(clientExecutions, 'GET', '`/api/v1/workbench/executions/\\$\\{pathId\\(requestedRunID, \'runID\'\\)\\}`');
  pair(clientExecutions, 'GET', '`/api/v1/workbench/executions/\\$\\{pathId\\(requestedRunID, \'runID\'\\)\\}/snapshot`');
  pair(clientExecutions, 'GET', '`/api/v1/workbench/executions/\\$\\{id\\}/events\\?version=2`');
  pair(clientExecutions, 'POST', "'\\/api\\/v1\\/workbench\\/executions'");
  pair(clientExecutions, 'GET', '`/api/v1/workbench/executions/requests/\\$\\{id\\}`');
  pair(clientSessions, 'POST', "'\\/api\\/v1\\/sessions'");
  pair(clientExecutions, 'POST', '`/api/v1/workbench/executions/\\$\\{pathId\\(requestedRunID, \'runID\'\\)\\}/commands`');
  pair(clientInteractions, 'POST', '`/api/v1/workbench/executions/interactions/\\$\\{pathId\\(input.id, \'id\'\\)\\}/decisions`');
  pair(clientInteractions, 'GET', '`/api/v1/workbench/interactions\\?limit=\\$\\{value\\}`');
  pair(clientTaskOffice, 'POST', '`/api/v1/workbench/tasks/\\$\\{encodeURIComponent\\(taskId\\)\\}/archive`');
  pair(clientTaskOffice, 'DELETE', '`/api/v1/workbench/tasks/\\$\\{encodeURIComponent\\(taskId\\)\\}/archive`');
});

test('assembly: Task artifacts are listed by owned run and receive a fresh signed grant on each download action', async () => {
  let grants = 0;
  await freshLogin({
    'GET /api/v1/workbench/executions/run-1/artifacts': call => stub.succeed(call, { data: { success: true, data: { items: [
      { index: 0, id: 'msg-1:0', name: 'report.pdf', mime: 'application/pdf', version: '1', size: 12, source_run: 'run-1' },
      { index: 1, id: 'msg-2:0', name: 'resume.docx', mime: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', version: '1', size: 24, source_run: 'run-1' },
    ] } } }),
    'POST /api/v1/workbench/executions/run-1/artifacts/0/signed-url': call => {
      grants++;
      assert.equal(call.options.data, undefined, 'grant issuance has no client supplied scope or artifact identity');
      stub.succeed(call, { data: { success: true, data: { url: `${ORIGIN}/api/v1/workbench/artifacts/download?signature=grant-${grants}`, expires_at: '2026-09-24T00:15:00Z' } } });
    },
  });
  const items = await workbench.listTaskArtifacts('run-1');
  assert.deepEqual(items.map(item => item.name), ['report.pdf', 'resume.docx']);
  const first = await workbench.taskArtifactDownloadPath('run-1', items[0]);
  const second = await workbench.taskArtifactDownloadPath('run-1', items[0]);
  assert.equal(first, '/api/v1/workbench/artifacts/download?signature=grant-1');
  assert.equal(second, '/api/v1/workbench/artifacts/download?signature=grant-2');
  assert.equal(grants, 2, 'a short-lived URL is minted per user action and never cached');
  stub.use(call => {
    stub.state.fileContents.set('/tmp/report.pdf', 'PDF-CONTENT');
    call.options.success({ statusCode: 200, tempFilePath: '/tmp/report.pdf' });
  });
  const files = await import('../src/platform/files.ts');
  await files.openProtectedDocument(second, items[0].name, true);
  const download = stub.lastCall('downloadFile');
  assert.equal(download.options.url, `${ORIGIN}${second}`);
  assert.equal(authorization(download), 'Bearer t1');
  assert.equal(stub.state.copies.length, 1, '下载后必须先复制成 USER_DATA_PATH 私有副本');
  assert.equal(stub.state.copies[0].srcPath, '/tmp/report.pdf');
  assert.ok(stub.state.copies[0].destPath.startsWith('wxfile://usr/'));
  assert.deepEqual(stub.state.openedDocuments, [{ filePath: stub.state.copies[0].destPath, showMenu: true }]);
  assert.deepEqual(stub.state.removedFiles, [stub.state.copies[0].destPath], 'the private copy is deleted after viewing');
});

test('assembly: Task artifact listing and grant failures preserve permission and expiry statuses', async () => {
  await freshLogin({
    'GET /api/v1/workbench/executions/run-1/artifacts': call => stub.succeed(call, { statusCode: 403, data: { success: false } }),
  });
  await assert.rejects(workbench.listTaskArtifacts('run-1'), error => error.status === 403);
  stub.use(call => stub.succeed(call, { statusCode: 410, data: { success: false } }));
  await assert.rejects(workbench.taskArtifactDownloadPath('run-1', { index: 0, name: 'report.pdf', mime: 'application/pdf', size: 12 }), error => error.status === 410);
});

test('assembly: protected DOCX opens with the user menu and removes its private copy', async () => {
  await freshLogin();
  stub.use(call => {
    stub.state.fileContents.set('/tmp/resume.docx', 'DOCX-CONTENT');
    stub.succeed(call, { tempFilePath: '/tmp/resume.docx' });
  });
  const files = await import('../src/platform/files.ts');
  await files.openProtectedDocument('/api/v1/workbench/artifacts/download?signature=docx', 'resume.docx', true);
  assert.deepEqual(stub.state.openedDocuments, [{ filePath: stub.state.copies[0].destPath, showMenu: true }]);
  assert.deepEqual(stub.state.removedFiles, [stub.state.copies[0].destPath]);
});

test('assembly: scope switch during refresh rejects before downloading protected content', async () => {
  await freshLogin();
  let releaseMe;
  stub.use(call => {
    if (call.kind === 'request' && new URL(call.options.url).pathname === '/api/v1/auth/me') {
      releaseMe = () => stub.succeed(call, { data: me() });
      return;
    }
    call.options.fail({ errMsg: `unexpected ${call.kind}` });
  });
  const files = await import('../src/platform/files.ts');
  const pending = files.openProtectedDocument('/api/v1/workbench/artifacts/download?signature=scope', 'report.pdf');
  const releaseDeadline = Date.now() + 1500;
  while (!releaseMe && Date.now() < releaseDeadline) await new Promise(resolve => setTimeout(resolve, 10));
  assert.ok(releaseMe, 'timed out waiting for the /auth/me refresh request');
  runtime.auth.scope.switchTo({ origin: ORIGIN, userId: 'u1', tenantId: '2' });
  releaseMe();
  await assert.rejects(pending, /SCOPE_CHANGED/);
  assert.equal(stub.state.calls.filter(call => call.kind === 'downloadFile').length, 0, 'stale refresh cannot authorize a file request');
  assert.deepEqual(stub.state.copies, [], 'no private copy is created for a stale scope');
});

test('assembly: signed download 401 is a fresh-grant expiry and a second tap succeeds', async () => {
  let grants = 0, downloads = 0;
  const expiredBody = JSON.stringify({ success: false, code: 'artifact_grant_expired' });
  await freshLogin();
  stub.use(call => {
    if (call.kind === 'request') {
      const path = new URL(call.options.url).pathname;
      if (path === '/api/v1/auth/me') { stub.succeed(call, { data: me() }); return; }
      if (path.endsWith('/signed-url')) {
        grants++;
        stub.succeed(call, { data: { success: true, data: { url: `${ORIGIN}/api/v1/workbench/artifacts/download?signature=${grants}`, expires_at: '2026-09-24T00:15:00Z' } } });
        return;
      }
      call.options.fail({ errMsg: `unexpected request ${path}` });
      return;
    }
    if (call.kind !== 'downloadFile') { call.options.fail({ errMsg: `unexpected ${call.kind}` }); return; }
    downloads++;
    if (downloads === 1) {
      stub.state.fileContents.set('/tmp/expired.json', expiredBody);
      stub.succeed(call, { statusCode: 401, tempFilePath: '/tmp/expired.json' });
    } else {
      stub.state.fileContents.set('/tmp/report.pdf', 'PDF-CONTENT');
      stub.succeed(call, { tempFilePath: '/tmp/report.pdf' });
    }
  });
  const files = await import('../src/platform/files.ts');
  const { errorMessage } = await import('../src/core/errors.ts');
  const file = { index: 0, name: 'report.pdf', mime: 'application/pdf', size: 12 };
  const firstGrant = await workbench.taskArtifactDownloadPath('run-1', file);
  await assert.rejects(files.openProtectedDocument(firstGrant, file.name), error => {
    assert.equal(error.status, 401);
    assert.equal(error.code, 'ARTIFACT_GRANT_EXPIRED');
    assert.match(errorMessage(error), /重新获取|再次点击/);
    assert.doesNotMatch(errorMessage(error), /登录/);
    return true;
  });
  const secondGrant = await workbench.taskArtifactDownloadPath('run-1', file);
  await files.openProtectedDocument(secondGrant, file.name);
  assert.equal(grants, 2, 'retry obtains a new grant instead of reusing the expired URL');
  assert.equal(downloads, 2);
  assert.equal(stub.state.copies.length, 1, '只有成功的下载才创建私有副本');
  assert.deepEqual(stub.state.removedFiles, [stub.state.copies[0].destPath]);
  assert.deepEqual(stub.state.fileReads, [{ filePath: '/tmp/expired.json', encoding: 'utf8', position: 0, length: Buffer.byteLength(expiredBody) }]);
  assert.equal(stub.state.openedDocuments.length, 1);
});

test('assembly: revoked download is denied without creating any app-managed file', async () => {
  await freshLogin();
  stub.use(call => stub.succeed(call, { statusCode: 403, tempFilePath: '/tmp/denied.json' }));
  const files = await import('../src/platform/files.ts');
  await assert.rejects(files.openProtectedDocument('/api/v1/workbench/artifacts/download?signature=revoked', 'report.pdf'), error => error.status === 403);
  assert.equal(stub.state.openedDocuments.length, 0);
  assert.deepEqual(stub.state.copies, []);
  assert.deepEqual(stub.state.removedFiles, [], '运行时临时错误体不由应用管理（DevTools 对 tmp 路径 deny unlink）');
});

test('assembly: invalid signed-download 401 is an authorization failure rather than an expiry retry', async () => {
  await freshLogin();
  const invalidBody = JSON.stringify({ success: false, code: 'artifact_grant_invalid' });
  stub.use(call => {
    if (call.kind === 'request' && new URL(call.options.url).pathname === '/api/v1/auth/me') {
      stub.succeed(call, { data: me() });
      return;
    }
    if (call.kind !== 'downloadFile') { call.options.fail({ errMsg: `unexpected ${call.kind}` }); return; }
    stub.state.fileContents.set('/tmp/invalid-grant.json', invalidBody);
    stub.succeed(call, { statusCode: 401, tempFilePath: '/tmp/invalid-grant.json' });
  });
  const files = await import('../src/platform/files.ts');
  const { errorMessage } = await import('../src/core/errors.ts');
  await assert.rejects(files.openProtectedDocument('/api/v1/workbench/artifacts/download?signature=tampered', 'report.pdf'), error => {
    assert.equal(error.status, 401);
    assert.equal(error.code, 'ARTIFACT_GRANT_INVALID');
    assert.match(errorMessage(error), /无效|拒绝|重新读取/);
    assert.doesNotMatch(errorMessage(error), /再次点击|重新获取/);
    return true;
  });
  assert.equal(stub.state.openedDocuments.length, 0);
  assert.deepEqual(stub.state.copies, []);
  assert.deepEqual(stub.state.removedFiles, []);
  assert.deepEqual(stub.state.fileReads, [{ filePath: '/tmp/invalid-grant.json', encoding: 'utf8', position: 0, length: Buffer.byteLength(invalidBody) }]);
});

test('assembly: unknown signed-download 401 body defaults to denial without app-managed files', async () => {
  await freshLogin();
  const unknownBody = JSON.stringify({ success: false, code: 'different_auth_failure', detail: 'x'.repeat(5000) });
  stub.use(call => {
    if (call.kind === 'request' && new URL(call.options.url).pathname === '/api/v1/auth/me') {
      stub.succeed(call, { data: me() });
      return;
    }
    if (call.kind !== 'downloadFile') { call.options.fail({ errMsg: `unexpected ${call.kind}` }); return; }
    stub.state.fileContents.set('/tmp/unknown-grant.json', unknownBody);
    stub.succeed(call, { statusCode: 401, tempFilePath: '/tmp/unknown-grant.json' });
  });
  const files = await import('../src/platform/files.ts');
  const { errorMessage } = await import('../src/core/errors.ts');
  await assert.rejects(files.openProtectedDocument('/api/v1/workbench/artifacts/download?signature=unknown', 'report.pdf'), error => {
    assert.equal(error.code, 'ARTIFACT_GRANT_INVALID');
    assert.doesNotMatch(errorMessage(error), /再次点击|重新获取/);
    return true;
  });
  assert.deepEqual(stub.state.removedFiles, []);
  assert.deepEqual(stub.state.fileReads, [{ filePath: '/tmp/unknown-grant.json', encoding: 'utf8', position: 0, length: 4096 }]);
});

test('assembly: a scope switch during transfer prevents opening and any private copy', async () => {
  await freshLogin();
  stub.use(call => {
    if (call.kind === 'request') {
      const path = new URL(call.options.url).pathname;
      if (path === '/api/v1/auth/me') { stub.succeed(call, { data: me() }); return; }
      if (path === '/api/v1/auth/logout') { stub.succeed(call, { data: { success: true } }); return; }
      call.options.fail({ errMsg: `unexpected request ${path}` });
      return;
    }
  });
  const files = await import('../src/platform/files.ts');
  const pending = files.openProtectedDocument('/api/v1/workbench/artifacts/download?signature=old', 'report.pdf');
  const downloadDeadline = Date.now() + 1500;
  while (!stub.state.calls.some(call => call.kind === 'downloadFile') && Date.now() < downloadDeadline) {
    await new Promise(resolve => setTimeout(resolve, 10));
  }
  assert.ok(stub.state.calls.some(call => call.kind === 'downloadFile'), 'timed out waiting for the native download call');
  const download = stub.lastCall('downloadFile');
  const logout = runtime.auth.logout();
  stub.succeed(download, { tempFilePath: '/tmp/old-scope.pdf' });
  await logout;
  await assert.rejects(pending, /SCOPE_CHANGED/);
  assert.equal(stub.state.openedDocuments.length, 0);
  assert.deepEqual(stub.state.copies, [], 'scope 失效后不得创建任何私有副本');
  assert.deepEqual(stub.state.removedFiles, []);
});

test('assembly: scope change while obtaining a grant prevents an old-scope download', async () => {
  await freshLogin({ 'POST /api/v1/workbench/executions/run-1/artifacts/0/signed-url': () => {} });
  const pending = workbench.taskArtifactDownloadPath('run-1', { index: 0, name: 'report.pdf', mime: 'application/pdf', size: 12 });
  await new Promise(resolve => setImmediate(resolve));
  const grant = stub.lastCall('request');
  const logout = runtime.auth.logout();
  stub.succeed(grant, { data: { success: true, data: { url: `${ORIGIN}/api/v1/workbench/artifacts/download?signature=old`, expires_at: '2026-09-24T00:15:00Z' } } });
  await logout;
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
  assert.equal(stub.state.calls.some(call => call.kind === 'downloadFile'), false);
});

test('assembly: oversized files are rejected before any private copy exists', async () => {
  await freshLogin();
  stub.use(call => stub.succeed(call, { tempFilePath: '/tmp/candidate.pdf' }));
  const files = await import('../src/platform/files.ts');
  stub.state.fileInfoSize = 20 * 1024 * 1024 + 1;
  await assert.rejects(files.openProtectedDocument('/api/v1/workbench/artifacts/download?signature=large', 'large.pdf'), /上限/);
  assert.deepEqual(stub.state.copies, []);
  assert.deepEqual(stub.state.removedFiles, []);
  assert.equal(stub.state.openedDocuments.length, 0);
});

test('assembly: native viewer failure removes the private copy', async () => {
  await freshLogin();
  stub.use(call => {
    stub.state.fileContents.set('/tmp/viewer-failure.pdf', 'PDF-CONTENT');
    stub.succeed(call, { tempFilePath: '/tmp/viewer-failure.pdf' });
  });
  stub.state.openDocumentError = new Error('native viewer failed');
  const files = await import('../src/platform/files.ts');
  await assert.rejects(files.openProtectedDocument('/api/v1/workbench/artifacts/download?signature=native-failure', 'report.pdf'), /native viewer failed/);
  assert.deepEqual(stub.state.removedFiles, [stub.state.copies[0].destPath]);
});

test('assembly: file-info failure happens before any private copy exists', async () => {
  await freshLogin();
  stub.use(call => stub.succeed(call, { tempFilePath: '/tmp/file-info-failure.pdf' }));
  stub.state.fileInfoError = new Error('file info failed');
  const files = await import('../src/platform/files.ts');
  await assert.rejects(files.openProtectedDocument('/api/v1/workbench/artifacts/download?signature=file-info', 'report.pdf'), /file info failed/);
  assert.deepEqual(stub.state.copies, []);
  assert.deepEqual(stub.state.removedFiles, []);
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
