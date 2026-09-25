import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const { PluginsPanel } = await import('./PluginsPanel.tsx');

type SummaryFixture = {
  installationId: string;
  pluginId: string;
  name: string;
  version: string;
  state: 'active' | 'disabled';
  driftState: 'none' | 'detected';
  requiresPersonalAuth: boolean;
  toolCount: number;
};

function installedPlugins(): SummaryFixture[] {
  return [
    { installationId: 'inst-1', pluginId: 'com.example.jira-todo', name: 'Jira 本周待办', version: '1.2.0', state: 'active', driftState: 'none', requiresPersonalAuth: true, toolCount: 2 },
    { installationId: 'inst-2', pluginId: 'com.example.weather', name: '天气查询', version: '0.3.1', state: 'disabled', driftState: 'detected', requiresPersonalAuth: false, toolCount: 1 },
  ];
}

test('成员插件面板 SSR 直出本空间已安装插件与状态徽标', () => {
  const html = renderToStaticMarkup(React.createElement(PluginsPanel, {
    client: {} as never,
    initialInstallations: installedPlugins(),
  }));
  assert.match(html, /Jira 本周待办/);
  assert.match(html, /1\.2\.0/);
  assert.match(html, /天气查询/);
  assert.match(html, /0\.3\.1/);
  assert.match(html, /可用/, 'the active plugin renders as available');
  assert.match(html, /不可用/, 'the disabled plugin renders as unavailable');
  assert.match(html, /检测到漂移/, 'the drift badge renders');
  assert.match(html, /需个人授权/, 'the personal-auth badge renders');
});

test('成员面板不显示管理操作（不停用/不启用/不确认安装）', () => {
  const html = renderToStaticMarkup(React.createElement(PluginsPanel, {
    client: {} as never,
    initialInstallations: installedPlugins(),
  }));
  // 全文 regex（计划 Task 8 Step 1 伪代码口径）：治理动词一个都不能出现，
  // 因此成员面板的状态徽标用「可用/不可用」而非管理侧的「启用中/已停用」。
  assert.doesNotMatch(html, /停用/);
  assert.doesNotMatch(html, /启用/);
  assert.doesNotMatch(html, /确认安装/);
  assert.equal(Array.from(html.matchAll(/<button[^>]*>/g)).length, 0, 'the member panel renders no action buttons at all');
});

test('成员面板不出现卸载/删除插件的入口与文案（停用即治理终点）', () => {
  const html = renderToStaticMarkup(React.createElement(PluginsPanel, {
    client: {} as never,
    initialInstallations: installedPlugins(),
  }));
  assert.doesNotMatch(html, /卸载/);
  assert.doesNotMatch(html, /删除/);
});

test('成员面板空列表给出空态文案', () => {
  const html = renderToStaticMarkup(React.createElement(PluginsPanel, {
    client: {} as never,
    initialInstallations: [],
  }));
  assert.match(html, /暂无已安装插件/);
});

// ---- jsdom interaction: mount → GET /plugins/installations → list renders ----
const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { act } = await import('react');

type CapturedRequest = { method: string; path: string; body?: unknown };

test('成员面板挂载时经 client 拉取已安装插件列表', async () => {
  const captured: CapturedRequest[] = [];
  const client = {
    request: async (input: { method: string; path: string; body?: unknown }) => {
      captured.push({ method: input.method, path: input.path, body: input.body });
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
        return {
          success: true,
          data: [
            { installation_id: 'inst-1', plugin_id: 'com.example.jira-todo', name: 'Jira 本周待办', version: '1.2.0', state: 'active', drift_state: 'none', requires_personal_auth: true, tool_count: 2 },
          ],
        };
      }
      // T12：挂载后需个人授权的行会追拉 connections/me（该测试只关心列表请求）。
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/connections/me') {
        return connectionEnvelope('inst-1', 'unauthorized');
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
  };
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(React.createElement(PluginsPanel, { client: client as never })); });
  try {
    for (let i = 0; i < 4; i += 1) await act(async () => {});
    assert.equal(captured.filter((entry) => entry.method === 'GET' && entry.path === '/api/v1/plugins/installations').length, 1, 'exactly one list request fires on mount');
    const html = container.innerHTML;
    assert.match(html, /Jira 本周待办/);
    assert.match(html, /1\.2\.0/);
    assert.match(html, /可用/);
    assert.ok(!html.includes('停用'), 'no governance action leaks into the member panel');
  } finally {
    await act(async () => { root.unmount(); });
    document.body.replaceChildren();
  }
});

// ---- T12: 成员个人授权/撤销入口（connections/me 三态徽标 + 既有 OAuth 端点复用） ----

function connectionEnvelope(installationId: string, state: 'authorized' | 'expired' | 'unauthorized'): unknown {
  return {
    success: true,
    data: {
      installation_id: installationId,
      plugin_id: 'com.example.jira-todo',
      name: '插件',
      service_id: `svc-${installationId}`,
      requires_personal_auth: true,
      authorized: state === 'authorized',
      state,
      authorize_url_path: `/api/v1/mcp-services/svc-${installationId}/oauth/authorize-url`,
      revoke_path: `/api/v1/mcp-services/svc-${installationId}/oauth/token`,
      requires_auth_tools: ['search_my_week_issues'],
    },
  };
}

function listEnvelopeWithAuthRows(): { success: boolean; data: Array<Record<string, unknown>> } {
  return {
    success: true,
    data: [
      { installation_id: 'inst-1', plugin_id: 'com.example.jira-todo', name: 'Jira 本周待办', version: '1.2.0', state: 'active', drift_state: 'none', requires_personal_auth: true, tool_count: 2 },
      { installation_id: 'inst-2', plugin_id: 'com.example.weather', name: '天气查询', version: '0.3.1', state: 'active', drift_state: 'none', requires_personal_auth: false, tool_count: 1 },
      { installation_id: 'inst-3', plugin_id: 'com.example.gitlab', name: 'GitLab 议题', version: '2.0.0', state: 'active', drift_state: 'none', requires_personal_auth: true, tool_count: 1 },
    ],
  };
}

function noOauthClient() {
  return {
    authorizeUrl: async () => { throw new Error('authorizeUrl must not be called in this test'); },
    revoke: async () => { throw new Error('revoke must not be called in this test'); },
  };
}

function findButton(container: HTMLElement, label: string): HTMLButtonElement | undefined {
  return Array.from(container.querySelectorAll('button')).find((button) => button.textContent === label);
}

test('成员面板为需个人授权插件渲染三态徽标与入口；无个人授权插件不出现授权区', async () => {
  const connectionCalls: string[] = [];
  const client = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') return listEnvelopeWithAuthRows();
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/connections/me') {
        connectionCalls.push(input.path);
        return connectionEnvelope('inst-1', 'authorized');
      }
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-3/connections/me') {
        connectionCalls.push(input.path);
        return connectionEnvelope('inst-3', 'expired');
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
    configuration: { mcp: { oauth: noOauthClient() } },
  };
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(React.createElement(PluginsPanel, { client: client as never })); });
  try {
    for (let i = 0; i < 6; i += 1) await act(async () => {});
    const rows = Array.from(container.querySelectorAll('li'));
    assert.equal(rows.length, 3);
    // authorized：徽标「已授权」+ 撤销入口，无去授权。
    assert.match(rows[0]!.innerHTML, /已授权/);
    assert.match(rows[0]!.innerHTML, /撤销/);
    assert.ok(!findButton(rows[0] as HTMLElement, '去授权'), 'an authorized connection offers revoke, not authorize');
    // requires_personal_auth=false：整行无授权区（无徽标、无按钮、不发 connections/me）。
    assert.ok(!rows[1]!.innerHTML.includes('已授权') && !rows[1]!.innerHTML.includes('已过期') && !rows[1]!.innerHTML.includes('未授权'), 'a no-auth plugin renders no connection badge');
    assert.equal(rows[1]!.querySelectorAll('button').length, 0, 'a no-auth plugin renders no auth buttons');
    // expired：徽标「已过期」+ 去授权（引导重授权），无撤销。
    assert.match(rows[2]!.innerHTML, /已过期/);
    assert.match(rows[2]!.innerHTML, /去授权/);
    assert.ok(!findButton(rows[2] as HTMLElement, '撤销'), 'an expired connection guides re-authorization, not revoke');
    // connections/me 只为 requires_personal_auth 插件发起。
    assert.deepEqual([...connectionCalls].sort(), [
      '/api/v1/plugins/installations/inst-1/connections/me',
      '/api/v1/plugins/installations/inst-3/connections/me',
    ]);
  } finally {
    await act(async () => { root.unmount(); });
    document.body.replaceChildren();
  }
});

test('成员面板未授权态点击「去授权」：经既有 oauth 客户端取授权地址并 window.open 弹窗', async () => {
  const opened: string[] = [];
  const originalOpen = dom.window.open;
  dom.window.open = ((url: unknown) => {
    opened.push(String(url));
    return { closed: true } as never;
  }) as never;
  const authorizeCalls: string[] = [];
  const client = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
        return { success: true, data: [listEnvelopeWithAuthRows().data[0]] };
      }
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/connections/me') {
        return connectionEnvelope('inst-1', 'unauthorized');
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
    configuration: { mcp: { oauth: {
      authorizeUrl: async (serviceId: string) => {
        authorizeCalls.push(serviceId);
        return { authorizationUrl: 'https://idp.test/authorize', authorizationAttempt: 'att-1' };
      },
      revoke: async () => { throw new Error('revoke must not be called in this test'); },
    } } },
  };
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(React.createElement(PluginsPanel, { client: client as never })); });
  try {
    for (let i = 0; i < 6; i += 1) await act(async () => {});
    assert.match(container.innerHTML, /未授权/);
    const authorizeButton = findButton(container, '去授权');
    assert.ok(authorizeButton, 'an unauthorized connection renders the authorize entry');
    await act(async () => { authorizeButton!.click(); });
    for (let i = 0; i < 4; i += 1) await act(async () => {});
    // 去授权 = 复用物化 service_id 上的既有 authorize-url 端点 + 弹窗打开授权服务器页面。
    assert.deepEqual(authorizeCalls, ['svc-inst-1']);
    assert.deepEqual(opened, ['https://idp.test/authorize']);
  } finally {
    await act(async () => { root.unmount(); });
    document.body.replaceChildren();
    dom.window.open = originalOpen;
  }
});

test('成员面板已授权态点击「撤销」：DELETE token 后刷新 connections/me 回到未授权', async () => {
  const sequence: string[] = [];
  let connectionState: 'authorized' | 'unauthorized' = 'authorized';
  const client = {
    request: async (input: { method: string; path: string }) => {
      sequence.push(`${input.method} ${input.path}`);
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
        return { success: true, data: [listEnvelopeWithAuthRows().data[0]] };
      }
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/connections/me') {
        return connectionEnvelope('inst-1', connectionState);
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
    configuration: { mcp: { oauth: {
      authorizeUrl: async () => { throw new Error('authorizeUrl must not be called in this test'); },
      revoke: async (serviceId: string) => {
        sequence.push(`REVOKE ${serviceId}`);
        connectionState = 'unauthorized';
      },
    } } },
  };
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(React.createElement(PluginsPanel, { client: client as never })); });
  try {
    for (let i = 0; i < 6; i += 1) await act(async () => {});
    assert.match(container.innerHTML, /已授权/);
    const revokeButton = findButton(container, '撤销');
    assert.ok(revokeButton, 'an authorized connection renders the revoke entry');
    const listAndFirstConnection = sequence.length;
    await act(async () => { revokeButton!.click(); });
    for (let i = 0; i < 4; i += 1) await act(async () => {});
    // 调用序列：撤销（既有 DELETE token 客户端方法，物化 service_id）→ 刷新 connections/me。
    assert.deepEqual(sequence.slice(listAndFirstConnection), [
      'REVOKE svc-inst-1',
      'GET /api/v1/plugins/installations/inst-1/connections/me',
    ]);
    assert.match(container.innerHTML, /未授权/);
    assert.ok(findButton(container, '去授权'), 'after revoke the panel offers authorize again');
    assert.ok(!findButton(container, '撤销'), 'the revoke entry disappears once unauthorized');
  } finally {
    await act(async () => { root.unmount(); });
    document.body.replaceChildren();
  }
});

// ---- T12-OCR1 回归（F3 空服务窗口 / F4 请求放大与失败态 / F5 轮询顺序） ----

function connectionEnvelopeVariant(
  installationId: string,
  state: 'authorized' | 'expired' | 'unauthorized',
  overrides: Partial<Record<string, unknown>>,
): unknown {
  const envelope = connectionEnvelope(installationId, state) as { data: Record<string, unknown> };
  Object.assign(envelope.data, overrides);
  return envelope;
}

test('OCR1-F3：物化服务缺失（service_id 空）时行内按钮禁用且不发起授权请求', async () => {
  const authorizeCalls: string[] = [];
  const originalOpen = dom.window.open;
  dom.window.open = (() => { throw new Error('window.open must not be called in this test'); }) as never;
  const client = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
        return { success: true, data: [listEnvelopeWithAuthRows().data[0]] };
      }
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/connections/me') {
        // confirm 在 CreateMCPService 之前中断：200 + 空 service_id + 空端点路径。
        return connectionEnvelopeVariant('inst-1', 'unauthorized', { service_id: '', authorize_url_path: '', revoke_path: '' });
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
    configuration: { mcp: { oauth: {
      authorizeUrl: async () => { authorizeCalls.push('called'); throw new Error('must not be called'); },
      revoke: async () => { throw new Error('must not be called'); },
    } } },
  };
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(React.createElement(PluginsPanel, { client: client as never })); });
  try {
    for (let i = 0; i < 6; i += 1) await act(async () => {});
    // 徽标仍渲染三态（该行退化为状态展示，而非整行加载错误）。
    assert.match(container.innerHTML, /未授权/);
    const authorizeButton = findButton(container, '去授权');
    assert.ok(authorizeButton, 'the authorize entry renders');
    assert.ok(authorizeButton!.disabled, 'entries stay disabled without a materialized service_id');
    await act(async () => { authorizeButton!.click(); });
    assert.deepEqual(authorizeCalls, [], 'no authorize-url request fires for an empty service_id');
  } finally {
    await act(async () => { root.unmount(); });
    document.body.replaceChildren();
    dom.window.open = originalOpen;
  }
});

test('OCR1-F4：连接加载失败行渲染失败态与重试入口，且不被其他行成功放大重发', async () => {
  const connectionCalls: Record<string, number> = { 'inst-1': 0, 'inst-3': 0 };
  let failInst3 = true;
  const client = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') return listEnvelopeWithAuthRows();
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/connections/me') {
        connectionCalls['inst-1']! += 1;
        return connectionEnvelope('inst-1', 'authorized');
      }
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-3/connections/me') {
        connectionCalls['inst-3']! += 1;
        if (failInst3) throw new Error('connection temporarily unavailable');
        return connectionEnvelope('inst-3', 'expired');
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
    configuration: { mcp: { oauth: noOauthClient() } },
  };
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(React.createElement(PluginsPanel, { client: client as never })); });
  try {
    // 多轮空转：给旧实现（deps 含 connections）足够机会把 inst-1 的成功落键
    // 变成对失败行 inst-3 的放大重发。
    for (let i = 0; i < 10; i += 1) await act(async () => {});
    assert.equal(connectionCalls['inst-1'], 1, 'a loaded connection is fetched exactly once');
    assert.equal(connectionCalls['inst-3'], 1, 'a failed connection is NOT re-fetched when other rows succeed');
    // 失败行不再静默缺失：行内呈现失败态与重试入口。
    const rows = Array.from(container.querySelectorAll('li'));
    assert.match(rows[2]!.innerHTML, /授权状态加载失败/, 'the failed row surfaces a visible failure state');
    const retryButton = findButton(rows[2] as HTMLElement, '重试');
    assert.ok(retryButton, 'the failed row offers a retry entry');
    // 重试成功：墓碑清除、连接重发一次、徽标渲染。
    failInst3 = false;
    await act(async () => { retryButton!.click(); });
    for (let i = 0; i < 4; i += 1) await act(async () => {});
    assert.equal(connectionCalls['inst-3'], 2, 'retry fires exactly one more request');
    assert.match(rows[2]!.innerHTML, /已过期/, 'the retried row renders its connection badge');
  } finally {
    await act(async () => { root.unmount(); });
    document.body.replaceChildren();
  }
});

test('OCR1-F5：用户在弹窗完成授权后关闭弹窗——先刷新先判 authorized，徽标不滞留未授权', async () => {
  const opened: string[] = [];
  const originalOpen = dom.window.open;
  let completedInPopup = false;
  let connectionState: 'authorized' | 'unauthorized' = 'unauthorized';
  dom.window.open = ((url: unknown) => {
    opened.push(String(url));
    // 用户在弹窗里完成授权（回调已入库）后随手关掉弹窗——落在下一次轮询间隔内。
    completedInPopup = true;
    return { closed: true } as never;
  }) as never;
  const client = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
        return { success: true, data: [listEnvelopeWithAuthRows().data[0]] };
      }
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/connections/me') {
        if (completedInPopup) connectionState = 'authorized';
        return connectionEnvelope('inst-1', connectionState);
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
    configuration: { mcp: { oauth: {
      authorizeUrl: async () => ({ authorizationUrl: 'https://idp.test/authorize', authorizationAttempt: 'att-1' }),
      revoke: async () => { throw new Error('revoke must not be called in this test'); },
    } } },
  };
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(React.createElement(PluginsPanel, { client: client as never })); });
  try {
    for (let i = 0; i < 6; i += 1) await act(async () => {});
    const authorizeButton = findButton(container, '去授权');
    assert.ok(authorizeButton);
    await act(async () => { authorizeButton!.click(); });
    // 轮询首拍是真实 1500ms sleep——act 空转只 flush 微任务，需真实等待
    // 一拍以上（覆盖 sleep + 刷新 + 兜底）。
    await new Promise((resolve) => setTimeout(resolve, 2000));
    for (let i = 0; i < 4; i += 1) await act(async () => {});
    assert.deepEqual(opened, ['https://idp.test/authorize']);
    // 先例顺序（McpSettingsPanel：先刷新先判 authorized 再判 popup.closed）下，
    // 授权已完成的关窗不得吞掉最终刷新。
    assert.match(container.innerHTML, /已授权/, 'the badge reflects the completed authorization despite the popup being closed');
  } finally {
    await act(async () => { root.unmount(); });
    document.body.replaceChildren();
    dom.window.open = originalOpen;
  }
});

// 跨任务转交 T08-OCR2-F5：PluginsPanel（成员插件发现面板）必须有真实挂载点
// ——面板本体（T08/T12）此前已建成但无任何页面接线，plugins tab 点开只有
// 标题+描述、正文空白。断言 apps/web 的集成路由页把面板经 pluginsSlot 挂进
// 共享 IntegrationsPage（依赖方向 apps/web -> packages/views）。
test('IntegrationsRoutePage wires PluginsPanel into the shared page via pluginsSlot', () => {
  const source = readFileSync(new URL('./IntegrationsRoutePage.tsx', import.meta.url), 'utf8');
  assert.match(source, /import \{ PluginsPanel \} from '\.\/PluginsPanel\.tsx'/, 'the panel must be imported');
  assert.match(source, /pluginsSlot=\{<PluginsPanel client=\{client\}/, 'the panel must be passed through the view-layer slot');
});
