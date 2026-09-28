import test from 'node:test';
import assert from 'node:assert/strict';
import { createElement } from 'react';
import { registerHooks, createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// ---- 平台边界替换：真实页面在纯 Node 中挂载为对象树 ----
// '@tarojs/taro' / '@tarojs/components' / 静态资源重定向到契约替身；
// .tsx 由 esbuild 现场编译（Node strip-types 不认 JSX），其余源码原样装配。
const stubURL = new URL('./helpers/taro-stub.mjs', import.meta.url).href;
const componentsURL = new URL('./helpers/components-host.mjs', import.meta.url).href;
const req = createRequire(import.meta.url);
const esbuild = (() => {
  const anchor = req.resolve('@tarojs/webpack5-runner/package.json').replace(/\/package\.json$/, '');
  return req(req.resolve('esbuild', { paths: [anchor] }));
})();

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
    if (specifier === '@tarojs/components') return { url: componentsURL, shortCircuit: true };
    if (/\.(png|scss|css|woff2?)$/.test(specifier)) {
      return { url: `data:text/javascript;charset=utf-8,${encodeURIComponent(`export default ${JSON.stringify(specifier)}`)}`, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.endsWith('.tsx')) {
      const source = readFileSync(fileURLToPath(url), 'utf8');
      const compiled = esbuild.transformSync(source, { loader: 'tsx', jsx: 'automatic', format: 'esm' });
      return { source: compiled.code, format: 'module', shortCircuit: true };
    }
    return nextLoad(url, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';

const { stub } = await import('./helpers/taro-stub.mjs');
const { render } = await import('./helpers/host-render.mjs');
const runtime = await import('../src/services/runtime.ts');
const homePages = await import('../src/features/home/pages.tsx');
const authPages = await import('../src/features/auth/pages.tsx');
const executionPages = await import('../src/features/execution/pages.tsx');
const knowledgePages = await import('../src/features/knowledge/pages.tsx');
const accountPages = await import('../src/features/account/pages.tsx');
const chatPage = await import('../src/features/chat/page.tsx');

const meFixture = (memberships = []) => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships } });
const execDto = (status = 'running', seq = 5) => ({ schema_version: 1, run_id: 'run-1', session_id: 's-1', revision: 3, driver: 'platform', run_status: status, execution_status: 'running', settlement_status: 'pending', seq, capabilities: { steer: { state: 'unavailable', reason: '服务端未开放追加指令' }, cancel: { state: 'supported', reason: '' } } });
const execEvent = seq => ({ schema_version: 1, run_id: 'run-1', attempt_id: 'a-1', seq, type: 'progress', occurred_at: '2026-09-18T00:00:00Z', payload: { summary: '整理资料' } });

/** 按 method+pathname 路由的假后端（与 assembly.test.mjs 同款口径）。 */
function backend(routes) {
  stub.use(call => {
    if (call.kind !== 'request' && call.kind !== 'uploadFile') return;
    const method = call.options.method ?? 'POST', path = new URL(call.options.url).pathname;
    let fn = routes[`${method} ${path}`];
    if (fn === undefined) {
      const prefix = Object.keys(routes).filter(k => k.endsWith('/') && `${method} ${path}`.startsWith(k)).sort((a, b) => b.length - a.length)[0];
      if (prefix) fn = routes[prefix];
    }
    if (fn === undefined) { call.options.fail({ errMsg: `no backend route for ${method} ${path}` }); return; }
    fn(call);
  });
}
function loginBackend(extra = {}, memberships = []) {
  backend({
    'POST /api/v1/auth/login': call => stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }),
    'GET /api/v1/auth/me': call => stub.succeed(call, { data: meFixture(memberships) }),
    ...extra,
  });
}
async function freshLogin(extra = {}, memberships = []) {
  stub.reset();
  loginBackend(extra, memberships);
  await runtime.auth.login('u@example.test', 'pw');
  return async page => {
    const view = await render(page);
    await view.settle();
    return view;
  };
}
const navigated = kind => stub.navigations().filter(entry => entry.kind === kind).map(entry => entry.url);
const postedBody = call => call.options.data;

test('render: anonymous users see the login gate, not the workbench (HomePage/Screen)', async () => {
  stub.reset(); runtime.auth.clear();
  loginBackend({});
  const view = await render(createElement(homePages.HomePage));
  assert.equal(view.hasText('让工作，在微信里继续'), true, 'must render the login empty-state');
  assert.equal(view.hasText('今天想完成什么'), false, 'the intent card must stay hidden before login');
  const loginButton = view.clickable('登录');
  loginButton.props.onClick();
  await view.settle();
  assert.deepEqual(navigated('navigateTo'), ['/subpackages/auth/login/index']);
  view.unmount();
});

test('render: LoginPage gates on consent, logs in through the real client, and surfaces a wrong password', async () => {
  stub.reset(); runtime.auth.clear();
  let attempts = 0;
  backend({
    'POST /api/v1/auth/login': call => {
      attempts++;
      if (attempts === 1) { stub.succeed(call, { statusCode: 401, data: { success: false, message: '账号或密码不正确' } }); return; }
      stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } });
    },
    'GET /api/v1/auth/me': call => stub.succeed(call, { data: meFixture() }),
  });
  const view = await render(createElement(authPages.LoginPage));
  const loginAction = view.byClassName('wk-button').find(button => buttonText(view, button) === '登录并继续');
  assert.ok(loginAction, 'submit action rendered');
  assert.equal(loginAction.props.disabled, true, 'submit disabled before consent');

  await view.type('邮箱', 'lin@example.test');
  await view.type('密码', 'pw');
  await view.pressCheckboxGroup(['consent']);
  const ready = view.byClassName('wk-button').find(button => buttonText(view, button) === '登录并继续');
  assert.equal(ready.props.disabled, false, 'submit enabled after consent + credentials');

  ready.props.onClick();
  await view.settle(); await view.settle();
  assert.ok(view.byClassName('wk-notice--danger').length >= 1, 'wrong password surfaces a danger notice');
  assert.equal(runtime.auth.snapshot().phase, 'anonymous', 'failed login keeps the session anonymous');

  ready.props.onClick();
  await view.settle(); await view.settle();
  assert.equal(runtime.auth.snapshot().phase, 'ready');
  assert.deepEqual(navigated('navigateTo'), ['/subpackages/auth/workspace/index'], 'success navigates to workspace selection');
  view.unmount();
});

function buttonText(view, button) {
  return view.textOf(button).trim();
}

test('render: WorkspacePage lists memberships and requires confirm before switching tenant', async () => {
  const mount = await freshLogin({}, [
    { tenant_id: 1, tenant_name: 'Space', role: 'Owner' },
    { tenant_id: 2, tenant_name: 'Second', role: 'Member' },
  ]);
  let switched = 0;
  stub.use(call => {
    const path = new URL(call.options.url).pathname;
    if (path === '/api/v1/auth/switch-tenant') { switched++; stub.succeed(call, { data: { success: true, data: { token: 't2', refresh_token: 'r2', user: { id: 'u1', username: 'Lin' }, tenant: { id: 2, name: 'Second' }, memberships: [] } } }); return; }
    if (path === '/api/v1/auth/me') { stub.succeed(call, { data: meFixture() }); return; }
    call.options.fail({ errMsg: `unexpected ${path}` });
  });
  const view = await mount(createElement(authPages.WorkspacePage));
  assert.ok(view.hasText('Second'), 'second membership is listed');
  view.click('Second');
  await view.settle();
  stub.state.modal.confirm = false; // 默认替身会确认弹窗；先显式走“取消”分支。
  view.click('进入工作空间');
  await view.settle(); await view.settle();
  assert.equal(switched, 0, 'declined modal must not call switch-tenant');
  assert.equal(navigated('switchTab').length, 0);

  stub.state.modal.confirm = true;
  view.click('进入工作空间');
  await view.settle(); await view.settle();
  assert.equal(switched, 1, 'confirmed modal switches tenant exactly once');
  assert.deepEqual(navigated('switchTab'), ['/pages/home/index']);
  view.unmount();
});

test('render: HomePage shows real backend data and carries the typed draft into AgentPage', async () => {
  const mount = await freshLogin({
    'GET /api/v1/agents': call => stub.succeed(call, { data: { success: true, data: [
      { id: 'agent-1', name: '周报助手', description: '整理一周工作' },
      { id: 'agent-2', name: '资料研究员', description: null },
    ] } }),
    'GET /api/v1/sessions': call => stub.succeed(call, { data: { success: true, data: [{ id: 's-1', title: '上周的整理', is_pinned: false }], total: 1, page: 1, page_size: 4 } }),
  });
  const view = await mount(createElement(homePages.HomePage));
  assert.ok(view.hasText('周报助手'), 'agent card from the real agents API');
  assert.ok(view.hasText('上周的整理'), 'recent session row from the real sessions API');
  assert.ok(view.hasText('Space ⌄'), 'workspace button shows the tenant name');

  // HomePage 的意图输入是独立 Textarea（placeholder 而非 Field label），直接驱动 onInput。
  const intent = view.byClassName('wk-intent')[0];
  intent.props.onInput({ detail: { value: '整理本周工作，生成一份周报' } });
  await view.settle();
  view.click('↑');
  await view.settle();
  assert.deepEqual(navigated('navigateTo'), ['/subpackages/agent/list/index'], 'send-square opens the agent picker');
  view.unmount();

  stub.state.routerParams = { id: 'agent-1' };
  loginBackend({
    'GET /api/v1/agents': call => stub.succeed(call, { data: { success: true, data: [{ id: 'agent-1', name: '周报助手', description: '整理一周工作' }] } }),
  });
  const agent = await render(createElement(homePages.AgentPage));
  await agent.settle();
  const carried = agent.byClassName('wk-textarea').at(-1);
  assert.equal(carried.props.value, '整理本周工作，生成一份周报', 'the draft typed on HomePage survives the page transition');
  agent.unmount();
  stub.state.routerParams = {};
});

test('render: AgentsPage search filters the list and marks disabled agents', async () => {
  const mount = await freshLogin({
    'GET /api/v1/agents': call => stub.succeed(call, { data: { success: true, data: [
      { id: 'agent-1', name: '周报助手', description: '整理一周工作' },
      { id: 'agent-2', name: '资料研究员', description: '查资料' },
    ], disabled_own_agent_ids: ['agent-2'] } }),
  });
  const view = await mount(createElement(homePages.AgentsPage));
  assert.ok(view.hasText('周报助手') && view.hasText('资料研究员'));
  assert.ok(view.hasText('当前不可用'), 'disabled agent carries the warning badge');
  await view.type('搜索 Agent', '周报');
  assert.equal(view.hasText('资料研究员'), false, 'non-matching agent is filtered out of the render');
  assert.ok(view.hasText('周报助手'));
  view.click('周报助手');
  await view.settle();
  assert.deepEqual(navigated('navigateTo'), ['/subpackages/agent/detail/index?id=agent-1']);
  view.unmount();
});

test('render: AgentPage validates budget client-side and submits a request_id-branded intent', async () => {
  const starts = [];
  const mount = await freshLogin({
    'GET /api/v1/agents': call => stub.succeed(call, { data: { success: true, data: [{ id: 'agent-1', name: '周报助手', description: '' }] } }),
    'POST /api/v1/sessions': call => stub.succeed(call, { data: { success: true, data: { id: 's-1', title: 'x', is_pinned: false } } }),
    'POST /api/v1/workbench/executions': call => {
      starts.push(call);
      stub.succeed(call, { statusCode: 202, data: { success: true, data: { run_id: 'run-9', request_id: postedBody(call).request_id, status: 'admitted' } } });
    },
  });
  stub.state.routerParams = { id: 'agent-1' }; // 在 freshLogin 的 reset 之后设置，render 之前生效。
  const view = await mount(createElement(homePages.AgentPage));
  await view.type('希望完成什么？', '帮我整理周报');
  await view.type('本次任务预算上限（Credits）', '12.5');
  view.click('发起任务');
  await view.settle(); await view.settle();
  assert.equal(starts.length, 0, 'non-integer budget must block the wire');
  // errorMessage 会把非 ApiError 归一为通用文案；渲染级只断言 danger 提示确实出现。
  assert.ok(view.byClassName('wk-notice--danger').length >= 1, 'invalid budget surfaces a danger notice');

  await view.type('本次任务预算上限（Credits）', '300');
  view.click('发起任务');
  await view.settle(); await view.settle(); await view.settle();
  assert.equal(starts.length, 1);
  const body = postedBody(starts[0]);
  assert.equal(body.agent_id, 'agent-1');
  assert.equal(body.budget_upper, 300);
  assert.match(body.request_id, /^mini-/, 'intent carries a durable mini- branded request id');
  assert.deepEqual(navigated('navigateTo'), ['/subpackages/execution/detail/index?id=run-9']);
  view.unmount();
  stub.state.routerParams = {};
});

test('render: ChatPage creates a session on first send and renders the streamed answer, hide marks interrupted', async () => {
  const mount = await freshLogin({
    'POST /api/v1/sessions': call => stub.succeed(call, { data: { success: true, data: { id: 's-9', title: '新对话', is_pinned: false } } }),
    'POST /api/v1/knowledge-chat/s-9': call => {
      stub.emitHeaders(call, { 'Content-Type': 'text/event-stream' });
      stub.emitChunk(call, new TextEncoder().encode('data: {"response_type":"answer","content":"第一段"}\n\n').buffer);
      stub.emitChunk(call, new TextEncoder().encode('data: {"response_type":"answer","content":"第二段"}\n\n').buffer);
      stub.succeed(call, { statusCode: 200, header: { 'Content-Type': 'text/event-stream' }, data: '' });
    },
  });
  const view = await mount(createElement(chatPage.default));
  assert.ok(view.hasText('从一个好问题开始'), 'fresh chat shows the empty state');
  await view.type('你的问题', '本周做了什么？');
  view.click('发送');
  await view.settle(); await view.settle(); await view.settle();
  assert.ok(view.hasText('本周做了什么？'), 'the question bubble renders');
  assert.ok(view.hasText('第一段第二段'), 'the SSE answer renders progressively through the real reducer');
  const create = stub.state.calls.find(call => new URL(call.options.url).pathname === '/api/v1/sessions');
  assert.equal(create.options.data.title, '本周做了什么？'.slice(0, 12), 'session title derives from the question');

  stub.pageHidden();
  await view.settle();
  assert.ok(view.hasText('连接已中断'), 'hide marks the stream interrupted');
  view.unmount();
});

test('render: TasksPage filters hit the wire and cards navigate to execution', async () => {
  const statuses = { '': 'succeeded', running: 'running', waiting_user: 'waiting_user', succeeded: 'succeeded' };
  const mount = await freshLogin({
    'GET /api/v1/workbench/executions': call => {
      const url = new URL(call.options.url);
      const status = url.searchParams.get('status') ?? '';
      const current = statuses[status] ?? status;
      stub.succeed(call, { data: { success: true, data: { items: [{ run_id: `run-${current || 'all'}`, title: `任务 ${current || '全部'}`, run_status: current || 'succeeded', updated_at: '2026-09-20T10:00:00Z' }], next_cursor: '' } } });
    },
  });
  const view = await mount(createElement(executionPages.TasksPage));
  assert.ok(view.hasText('已完成'), 'badge label localized');
  view.click('运行中');
  await view.settle(); await view.settle();
  assert.ok(view.hasText('任务 running') && view.hasText('运行中'));
  const filterCall = [...stub.state.calls].reverse().find(call => new URL(call.options.url).searchParams.get('status') === 'running');
  assert.ok(filterCall, 'the running filter reaches the wire as status=running');
  view.click('任务 running');
  await view.settle();
  assert.deepEqual(navigated('navigateTo'), ['/subpackages/execution/detail/index?id=run-running']);
  view.unmount();
});

test('render: ExecutionPage renders the snapshot timeline and confirm-gates cancellation', async () => {
  let snapshots = 0, cancels = 0;
  const mount = await freshLogin({
    'GET /api/v1/workbench/executions/run-1/snapshot': call => {
      snapshots++;
      stub.succeed(call, { data: { success: true, data: { execution: execDto('running'), watermark: 5, incomplete: false, confirmed_watermark: 5, events: [execEvent(5)] } } });
    },
    'GET /api/v1/workbench/executions/run-1/events': call => {
      stub.emitHeaders(call, { 'Content-Type': 'text/event-stream' });
      stub.emitChunk(call, new TextEncoder().encode(`data: ${JSON.stringify(execEvent(6))}\n\n`).buffer);
      stub.succeed(call, { statusCode: 200, header: { 'Content-Type': 'text/event-stream' }, data: '' });
    },
    'POST /api/v1/workbench/executions/run-1/commands': call => {
      cancels++;
      stub.succeed(call, { data: { success: true, data: { run_id: 'run-1', action: 'cancel' } } });
    },
  });
  stub.state.routerParams = { id: 'run-1' };
  const view = await mount(createElement(executionPages.ExecutionPage));
  await view.settle(); await view.settle(); await view.settle();
  assert.ok(view.hasText('执行状态已更新'), 'timeline renders the streamed event');
  assert.ok(view.hasText('已接收事件：6'), 'the projection cursor advances from the SSE append');
  assert.ok(view.hasText('已连接'));
  const steer = view.byClassName('wk-button').find(button => view.hasText('提交追加指令', button));
  assert.equal(steer.props.disabled, true, 'steer stays disabled while the server blocks the capability');

  stub.state.modal.confirm = false;
  view.click('取消任务');
  await view.settle(); await view.settle();
  assert.equal(cancels, 0, 'declined confirm must not POST cancel');

  stub.state.modal.confirm = true;
  view.click('取消任务');
  await view.settle(); await view.settle(); await view.settle();
  assert.equal(cancels, 1, 'confirmed cancel reaches the wire exactly once');
  assert.equal(stub.state.calls.filter(call => new URL(call.options.url).pathname.endsWith('/commands')).at(-1).options.data.action, 'cancel');
  assert.ok(snapshots >= 2, 'cancel triggers a snapshot resync');
  view.unmount();
  stub.state.routerParams = {};
});

test('render: ApprovalPage renders interactions and only ever offers safe rejection', async () => {
  let decisions = 0;
  const mount = await freshLogin({
    'GET /api/v1/workbench/executions/run-1/interactions': call => {
      stub.succeed(call, { data: { success: true, data: [{ id: 'i-1', kind: 'tool_approval', args_hash: 'h', expected_revision: 2, decision_id: '', action: '' }] } });
    },
    'POST /api/v1/workbench/executions/interactions/i-1/decisions': call => {
      decisions++;
      stub.succeed(call, { data: { success: true, data: { id: 'i-1' } } });
    },
  });
  stub.state.routerParams = { id: 'run-1' };
  const view = await mount(createElement(executionPages.ApprovalPage));
  assert.ok(view.hasText('工具操作确认') && view.hasText('待处理'));
  const approve = view.byClassName('wk-button').find(button => view.hasText('缺少动作详情，暂不可批准', button));
  assert.equal(approve.props.disabled, true, 'approval stays disabled without action details');

  stub.state.modal.confirm = true;
  view.click('拒绝本次操作');
  await view.settle(); await view.settle(); await view.settle();
  assert.equal(decisions, 1);
  const decisionBody = stub.state.calls.filter(call => new URL(call.options.url).pathname.endsWith('/decisions')).at(-1).options.data;
  assert.equal(decisionBody.action, 'reject');
  view.unmount();
  stub.state.routerParams = {};
});

test('render: KnowledgePage filters and KnowledgeDetailPage paginates with real wire queries', async () => {
  const mount = await freshLogin({
    'GET /api/v1/knowledge-bases': call => stub.succeed(call, { data: { success: true, data: [
      { id: 'kb-1', name: '产品文档', description: '规格与发布' },
      { id: 'kb-2', name: '会议纪要', description: '' },
    ] } }),
  });
  const view = await mount(createElement(knowledgePages.KnowledgePage));
  await view.type('搜索知识库', '产品');
  assert.equal(view.hasText('会议纪要'), false, 'search filters knowledge bases');
  view.click('产品文档');
  await view.settle();
  assert.deepEqual(navigated('navigateTo'), ['/subpackages/knowledge/detail/index?id=kb-1']);
  view.unmount();

  stub.state.routerParams = { id: 'kb-1' };
  const docPages = { 1: Array.from({ length: 20 }, (_, i) => ({ id: `d-1-${i}`, file_name: `文档-${i}.pdf`, parse_status: 'completed' })), 2: [{ id: 'd-2-0', file_name: '文档-20.pdf', parse_status: 'pending' }] };
  loginBackend({
    'GET /api/v1/knowledge-bases/kb-1/knowledge': call => {
      const url = new URL(call.options.url);
      const page = url.searchParams.get('page') ?? '1';
      const docs = docPages[page] ?? [];
      stub.succeed(call, { data: { success: true, data: docs, total: 21, page: Number(page), page_size: 20 } });
    },
  });
  const detail = await render(createElement(knowledgePages.KnowledgeDetailPage));
  await detail.settle(); await detail.settle();
  assert.ok(detail.hasText('文档-19.pdf'), 'first page documents render');
  const prev = detail.byClassName('wk-button').find(button => detail.hasText('上一页', button));
  assert.equal(prev.props.disabled, true, 'page 1 disables 上一页');
  detail.click('下一页');
  await detail.settle(); await detail.settle();
  assert.ok(detail.hasText('文档-20.pdf'), 'second page renders after pagination');
  assert.ok([...stub.state.calls].some(call => new URL(call.options.url).searchParams.get('page') === '2'), 'page=2 reaches the wire');
  detail.unmount();
  stub.state.routerParams = {};
});

test('render: UploadPage guards url/file inputs and drives the multipart upload wire', async () => {
  const uploads = [];
  const mount = await freshLogin({});
  stub.use(call => {
    const path = new URL(call.options.url).pathname;
    if (path === '/api/v1/knowledge-bases/kb-1/knowledge/file') { uploads.push(call); stub.succeed(call, { statusCode: 200, data: { success: true, data: { id: 'd-new' } } }); return; }
    if (path === '/api/v1/auth/me') { stub.succeed(call, { data: meFixture() }); return; }
    call.options.fail({ errMsg: `unexpected ${path}` });
  });
  stub.state.routerParams = { kb: 'kb-1' };
  const view = await mount(createElement(knowledgePages.UploadPage));
  view.click('导入链接');
  await view.settle();
  const submit = () => view.byClassName('wk-button').find(button => view.hasText('确认添加', button));
  assert.equal(submit().props.disabled, true, 'non-http url keeps submit disabled');
  await view.type('网页链接', 'https://example.test/post');
  assert.equal(submit().props.disabled, false);

  view.click('上传文件');
  await view.settle();
  stub.state.chosenFile = { path: 'wxfile://big.pdf', name: 'big.pdf', size: 21 * 1024 * 1024 };
  view.click('选择一份资料');
  await view.settle(); await view.settle();
  assert.ok(view.hasText('20 MiB'), 'oversize file is rejected before any upload');

  stub.state.chosenFile = { path: 'wxfile://report.pdf', name: 'report.pdf', size: 2048 };
  view.click('选择一份资料');
  await view.settle(); await view.settle();
  view.click('确认添加');
  await view.settle(); await view.settle(); await view.settle();
  assert.equal(uploads.length, 1, 'exactly one multipart upload');
  assert.equal(new URL(uploads[0].options.url).pathname, '/api/v1/knowledge-bases/kb-1/knowledge/file');
  assert.equal(uploads[0].options.filePath, 'wxfile://report.pdf');
  assert.ok(view.hasText('已提交到后台'));
  view.click('查看处理状态');
  await view.settle();
  assert.deepEqual(navigated('navigateTo').at(-1), '/subpackages/knowledge/document/index?id=d-new&kb=kb-1');
  view.unmount();
  stub.state.routerParams = {};
});

test('render: MePage/UsagePage render ledger numbers and stale warnings from the summary API', async () => {
  const mount = await freshLogin({
    'GET /api/v1/commercial/summary': call => stub.succeed(call, { data: { success: true, data: {
      tenant_id: 1, base_tier: false, can_manage_billing: false,
      subscription: { id: 'sub-1', plan_key: 'pro', plan_version: 1, paid_until: null, version: 1 },
      available: '12000', held: '100', refund_locked: '0', stale: false, as_of: '2026-09-28T08:00:00Z', plan_name: '专业版',
    } } }),
  });
  const view = await mount(createElement(accountPages.MePage));
  assert.ok(view.hasText('Lin'), 'profile shows the account name');
  assert.ok(view.hasText('12,000'), 'credits render with grouping, never as a float');
  view.unmount();

  loginBackend({
    'GET /api/v1/commercial/summary': call => stub.succeed(call, { data: { success: true, data: {
      tenant_id: 1, base_tier: false, can_manage_billing: false,
      subscription: { id: 'sub-1', plan_key: 'pro', plan_version: 1, paid_until: null, version: 1 },
      available: '0', held: '0', refund_locked: '0', stale: true, as_of: '2026-09-27T08:00:00Z', plan_name: '专业版',
    } } }),
  });
  const usage = await render(createElement(accountPages.UsagePage));
  await usage.settle(); await usage.settle();
  assert.ok(usage.hasText('专业版'));
  assert.ok(usage.hasText('统计结果已过期'), 'stale summary carries the admission warning');
  usage.unmount();
});

test('render: OrderPage resolves an order by id and InvitationsPage accepts by token', async () => {
  const mount = await freshLogin({});
  stub.use(call => {
    const path = new URL(call.options.url).pathname;
    if (path === '/api/v1/commercial/orders/o-1') { stub.succeed(call, { data: { success: true, data: { id: 'o-1', amount_fen: '9900', currency: 'CNY', payment: 'paid', fulfillment: 'processing' } } }); return; }
    if (path === '/api/v1/me/invitations/accept-by-token') { stub.succeed(call, { data: { success: true, data: { membership: { tenant_id: 2 }, tenant_name: '新空间' } } }); return; }
    if (path === '/api/v1/auth/me') { stub.succeed(call, { data: meFixture() }); return; }
    call.options.fail({ errMsg: `unexpected ${path}` });
  });
  const view = await mount(createElement(accountPages.OrderPage));
  await view.type('已有订单号', 'o-1');
  view.click('查询原订单');
  await view.settle(); await view.settle();
  assert.ok(view.hasText('¥99.00'), 'money renders as fen→yuan, dot-separated');
  assert.ok(view.hasText('已付款') && view.hasText('处理中'), 'payment and fulfillment tracked separately');
  view.unmount();

  const invitations = await render(createElement(accountPages.InvitationsPage));
  await invitations.type('邀请凭证', 'tok-1');
  stub.state.modal.confirm = true;
  invitations.click('验证并接受邀请');
  await invitations.settle(); await invitations.settle(); await invitations.settle();
  assert.ok(invitations.hasText('新空间'), 'accepted invitation names the joined space');
  const accepted = stub.state.calls.find(call => new URL(call.options.url).pathname === '/api/v1/me/invitations/accept-by-token');
  assert.equal(accepted.options.data.token, 'tok-1');
  invitations.unmount();
});
