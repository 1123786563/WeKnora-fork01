import assert from 'node:assert/strict';
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

const { PluginsSettingsPanel, formatPreviewExpiry } = await import('./PluginsSettingsPanel.tsx');
const { ApiError } = await import('@weknora/api-client');

function previewEnvelope(): unknown {
  return {
    success: true,
    data: {
      preview_id: 'p-1',
      plugin_id: 'com.example.jira-todo',
      version: '1.2.0',
      name: 'Jira 本周待办',
      description: '个人待办视角',
      transport_type: 'http-streamable',
      endpoint_url: 'https://plugins.example.com/jira-todo/v1.2.0/mcp',
      tools: [
        { name: 'search_my_week_issues', description: '搜索本周待办', read_only: true, requires_personal_auth: true, scopes: ['read:jira-work'] },
        { name: 'create_todo', description: '创建待办', read_only: false, requires_personal_auth: false, scopes: null },
      ],
      identity_fingerprint: 'a'.repeat(64),
      expires_at: '2026-09-23T12:00:00Z',
    },
  };
}

test('插件面板管理员初始态渲染粘贴表单且无预览卡', () => {
  const html = renderToStaticMarkup(React.createElement(PluginsSettingsPanel, { client: {} as never, role: 'admin' }));
  assert.match(html, /插件管理/);
  assert.match(html, /manifest\.json/);
  assert.ok(html.includes('type="url"') || html.includes('type="text"'), 'the manifest URL input renders');
  assert.doesNotMatch(html, /data-testid="plugin-preview-card"/);
  assert.doesNotMatch(html, /data-testid="plugin-preview-error"/);
});

test('插件面板对 viewer 隐藏提交表单（registry minRole=admin）', () => {
  const html = renderToStaticMarkup(React.createElement(PluginsSettingsPanel, { client: {} as never, role: 'viewer' }));
  assert.doesNotMatch(html, /<form/, 'no submit form renders for a viewer');
  assert.doesNotMatch(html, /核验预览/, 'the admin submit action is hidden');
  assert.match(html, /管理员/);
});

test('formatPreviewExpiry 本地化有效期并防御非法时间串', () => {
  const formatted = formatPreviewExpiry('2026-09-23T12:00:00Z');
  assert.ok(formatted.includes('2026'), 'a parseable expiry renders a localized date');
  assert.equal(formatPreviewExpiry('not-a-date'), 'not-a-date', 'an unparseable expiry falls back to the raw string');
});

// ---- jsdom interaction: paste → submit → preview card / error state ----
const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { act } = await import('react');

type CapturedRequest = { method: string; path: string; body: unknown };

function stubClient(handler: (input: CapturedRequest) => Promise<unknown>) {
  const captured: CapturedRequest[] = [];
  const client = {
    request: async (input: { method: string; path: string; body?: unknown }) => {
      const entry = { method: input.method, path: input.path, body: input.body };
      captured.push(entry);
      return handler(entry);
    },
  };
  return { client: client as never, captured };
}

async function mount(element: React.ReactElement) {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(element); });
  return root;
}

async function unmount(root: ReturnType<typeof createRoot>) {
  await act(async () => root.unmount());
  document.body.replaceChildren();
}

function setInputValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  setter?.call(input, value);
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
}

function submitForm(form: HTMLFormElement) {
  form.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true }));
}

test('插件面板管理员可提交清单地址并渲染预览结果', async () => {
  const { client, captured } = stubClient(async (input) => {
    if (input.method === 'GET') return { success: true, data: [] };
    return previewEnvelope();
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    const input = document.querySelector('input[type="url"]') as HTMLInputElement | null;
    assert.ok(input, 'manifest URL input renders');
    await act(async () => { setInputValue(input!, 'https://plugins.example.com/manifest.json'); });
    const form = document.querySelector('form');
    assert.ok(form, 'preview form renders');
    await act(async () => { submitForm(form); });
    const previewRequests = captured.filter((entry) => entry.path === '/api/v1/plugins/installations/preview');
    assert.equal(previewRequests.length, 1, 'one preview request fires');
    assert.equal(previewRequests[0]!.method, 'POST');
    assert.deepEqual(previewRequests[0]!.body, { manifest_url: 'https://plugins.example.com/manifest.json' });
    const card = document.querySelector('[data-testid="plugin-preview-card"]');
    assert.ok(card, 'the preview card renders');
    const text = card?.textContent ?? '';
    assert.match(text, /Jira 本周待办/);
    assert.match(text, /1\.2\.0/);
    assert.match(text, /com\.example\.jira-todo/);
    assert.match(text, /https:\/\/plugins\.example\.com\/jira-todo\/v1\.2\.0\/mcp/);
    assert.match(text, /search_my_week_issues/);
    assert.match(text, /create_todo/);
    assert.match(text, /只读/, 'the read-only badge renders');
    assert.match(text, /写入/, 'the write classification badge renders');
    assert.match(text, /需个人授权/, 'the personal-auth badge renders');
    assert.match(text, /read:jira-work/, 'declared scopes render');
    assert.match(text, /2026/, 'the preview expiry renders');
    assert.match(text, /schema 指纹已核验/, 'each tool row carries the verified-schema badge');
    // Spec US9 取舍：平台 digest 核验，界面不渲染 schema 原文（远端不可信数据不进管理界面）。
    assert.ok(!text.includes('"type":"object"') && !text.includes('additionalProperties'), 'no raw schema JSON leaks into the panel');
    assert.ok(!document.querySelector('[data-testid="plugin-preview-error"]'), 'no error banner after a successful preview');
  } finally {
    await unmount(root);
  }
});

test('插件面板渲染校验失败错误且不残留预览卡（后端 ApiError 原文透传）', async () => {
  // 真实链路中后端拒绝（SSRF/清单非法等）经 request 抛 ApiError，message 为
  // 后端可读文案——沿用 McpSettingsPanel 的原文透传先例。
  const { client } = stubClient(async () => { throw new ApiError({ code: 'HTTP_400', message: 'manifest URL rejected: 私网地址禁止访问' }); });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await act(async () => { setInputValue(document.querySelector('input[type="url"]') as HTMLInputElement, 'http://127.0.0.1/manifest.json'); });
    await act(async () => { submitForm(document.querySelector('form')!); });
    const errorBanner = document.querySelector('[data-testid="plugin-preview-error"]');
    assert.ok(errorBanner, 'the error banner renders');
    assert.match(errorBanner?.textContent ?? '', /manifest URL rejected/);
    assert.ok(!document.querySelector('[data-testid="plugin-preview-card"]'), 'no preview card leaks after a rejected preview');
  } finally {
    await unmount(root);
  }
});

test('插件面板把非法 envelope 解析失败呈现为中文错误态且不暴露内部路径', async () => {
  const { client } = stubClient(async () => ({ success: false }));
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await act(async () => { setInputValue(document.querySelector('input[type="url"]') as HTMLInputElement, 'https://plugins.example.com/manifest.json'); });
    await act(async () => { submitForm(document.querySelector('form')!); });
    const errorBanner = document.querySelector('[data-testid="plugin-preview-error"]');
    assert.ok(errorBanner, 'a non-success envelope surfaces as the error state');
    // OCR low：解析器自产的英文诊断串（含内部 API 路径）不进中文管理界面。
    assert.match(errorBanner?.textContent ?? '', /核验未通过/);
    assert.doesNotMatch(errorBanner?.textContent ?? '', /\/api\/v1\//);
    assert.ok(!document.querySelector('[data-testid="plugin-preview-card"]'));
  } finally {
    await unmount(root);
  }
});

test('插件面板对网络类普通错误给出统一中文文案（console.warn 留痕）', async () => {
  const warnings: unknown[][] = [];
  const originalWarn = console.warn;
  console.warn = (...args: unknown[]) => { warnings.push(args); };
  const { client } = stubClient(async () => { throw new Error('fetch failed'); });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await act(async () => { setInputValue(document.querySelector('input[type="url"]') as HTMLInputElement, 'https://plugins.example.com/manifest.json'); });
    await act(async () => { submitForm(document.querySelector('form')!); });
    const errorBanner = document.querySelector('[data-testid="plugin-preview-error"]');
    assert.ok(errorBanner);
    assert.match(errorBanner?.textContent ?? '', /核验未通过/);
    assert.ok(warnings.length >= 1, 'the raw cause is preserved in console.warn');
  } finally {
    console.warn = originalWarn;
    await unmount(root);
  }
});

test('插件面板拒绝空 URL 提交且不发预览请求', async () => {
  const { client, captured } = stubClient(async () => previewEnvelope());
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await act(async () => { submitForm(document.querySelector('form')!); });
    assert.equal(captured.filter((entry) => entry.path === '/api/v1/plugins/installations/preview').length, 0, 'no preview request fires for an empty URL');
    assert.match(document.querySelector('[data-testid="plugin-preview-error"]')?.textContent ?? '', /请输入/);
  } finally {
    await unmount(root);
  }
});

// ---- T08: 确认安装 / 已安装列表 / 停用启用 / 卸载范围明示 ----

const jiraSummary = {
  installation_id: 'inst-1',
  plugin_id: 'com.example.jira-todo',
  name: 'Jira 本周待办',
  version: '1.2.0',
  state: 'active',
  drift_state: 'none',
  requires_personal_auth: true,
  tool_count: 2,
};

function installationEnvelope(): unknown {
  return {
    success: true,
    data: {
      installation_id: 'inst-1',
      plugin_id: 'com.example.jira-todo',
      name: 'Jira 本周待办',
      description: '个人待办视角',
      version: '1.2.0',
      state: 'active',
      drift_state: 'none',
      transport_type: 'http-streamable',
      endpoint_url: 'https://plugins.example.com/jira-todo/v1.2.0/mcp',
      service_id: 'svc-1',
      tools: [
        { name: 'search_my_week_issues', description: '搜索本周待办', read_only: true, requires_personal_auth: true, scopes: ['read:jira-work'], enabled: true },
        { name: 'create_todo', description: '创建待办', read_only: false, requires_personal_auth: true, scopes: null, enabled: false },
      ],
    },
  };
}

async function flushEffects(times = 4) {
  for (let i = 0; i < times; i += 1) await act(async () => {});
}

function findButtonByText(root: ParentNode, text: string): HTMLButtonElement | null {
  const buttons = Array.from(root.querySelectorAll('button'));
  return buttons.find((button) => (button.textContent ?? '').trim() === text) ?? null;
}

test('预览卡出现确认安装按钮：点击消费 preview_id，列表出现该插件且版本正确', async () => {
  const installed: unknown[] = [];
  const { client, captured } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [...installed] };
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations') {
      installed.push({ ...jiraSummary });
      return installationEnvelope();
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/preview') {
      return previewEnvelope();
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    await act(async () => { setInputValue(document.querySelector('input[type="url"]') as HTMLInputElement, 'https://plugins.example.com/manifest.json'); });
    await act(async () => { submitForm(document.querySelector('form')!); });
    const card = document.querySelector('[data-testid="plugin-preview-card"]');
    assert.ok(card, 'the preview card renders');
    const confirmButton = findButtonByText(card as ParentNode, '确认安装');
    assert.ok(confirmButton, 'the confirm-install button renders on the preview card');
    await act(async () => { confirmButton!.click(); });
    await flushEffects();
    const confirmRequest = captured.find((entry) => entry.method === 'POST' && entry.path === '/api/v1/plugins/installations');
    assert.ok(confirmRequest, 'a confirm request fires');
    assert.deepEqual(confirmRequest!.body, { preview_id: 'p-1' }, 'the confirm body carries the preview card preview_id');
    const list = document.querySelector('[data-testid="plugin-installations"]');
    assert.ok(list, 'the installed list renders');
    const text = list?.textContent ?? '';
    assert.match(text, /Jira 本周待办/);
    assert.match(text, /1\.2\.0/);
    assert.ok(!document.querySelector('[data-testid="plugin-preview-card"]'), 'the consumed preview card is dismissed after a confirmed install');
  } finally {
    await unmount(root);
  }
});

test('管理员列表行有停用按钮：点击调用 disable 端点并更新状态徽标', async () => {
  const rows: unknown[] = [{ ...jiraSummary }];
  const { client, captured } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [...rows] };
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/inst-1/disable') {
      rows[0] = { ...jiraSummary, state: 'disabled' };
      const disabled = installationEnvelope() as { data: Record<string, unknown> };
      disabled.data.state = 'disabled';
      return disabled;
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]');
    assert.ok(list, 'the installed list renders');
    assert.match(list?.textContent ?? '', /启用中/, 'an active installation carries the active badge');
    assert.match(list?.textContent ?? '', /无漂移/, 'the drift badge renders');
    const disableButton = findButtonByText(list as ParentNode, '停用');
    assert.ok(disableButton, 'the admin list row carries a disable button');
    await act(async () => { disableButton!.click(); });
    await flushEffects();
    assert.equal(captured.filter((entry) => entry.path === '/api/v1/plugins/installations/inst-1/disable').length, 1, 'the disable endpoint fires exactly once');
    const refreshed = document.querySelector('[data-testid="plugin-installations"]')?.textContent ?? '';
    assert.match(refreshed, /已停用/, 'the state badge flips to disabled');
    assert.ok(findButtonByText(document, '启用'), 'an enable button replaces the disable button');
  } finally {
    await unmount(root);
  }
});

test('viewer 列表只读：无停用/启用/确认安装操作', async () => {
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'viewer' }));
  try {
    await flushEffects();
    const html = document.body.innerHTML;
    assert.match(html, /Jira 本周待办/, 'the viewer still discovers the installed plugin');
    assert.match(html, /1\.2\.0/, 'the viewer sees the accepted version');
    assert.match(html, /启用中/, 'the viewer sees the state badge');
    for (const forbidden of ['停用', '启用', '确认安装']) {
      assert.ok(!findButtonByText(document, forbidden), `viewer must not see the ${forbidden} action`);
    }
  } finally {
    await unmount(root);
  }
});

test('卸载范围明示：面板不出现卸载/删除插件的入口与文案（停用即治理终点）', async () => {
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary, state: 'disabled' }] };
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'owner' }));
  try {
    await flushEffects();
    const html = document.body.innerHTML;
    assert.doesNotMatch(html, /卸载/, 'no uninstall entry or copy surfaces');
    assert.doesNotMatch(html, /删除插件/, 'no delete-plugin copy surfaces');
    assert.equal(Array.from(document.querySelectorAll('button')).filter((button) => (button.textContent ?? '').includes('删除')).length, 0, 'no delete buttons render');
  } finally {
    await unmount(root);
  }
});

// ---- T08-OCR1-F3/F7：listError 文案准确性 ----

test('列表加载失败（非 ApiError）呈现单一完整文案且不与空态并列', async () => {
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      throw new Error('network down');
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const section = document.querySelector('[data-testid="plugin-installations"]');
    assert.ok(section, 'the installations section renders');
    const text = section?.textContent ?? '';
    assert.match(text, /已安装插件加载失败/, 'the load failure surfaces');
    // F3：固定前缀拼接会造成同句重复——首末出现位置必须相同。
    assert.equal(text.indexOf('已安装插件加载失败'), text.lastIndexOf('已安装插件加载失败'), 'no duplicated prefix from the fixed JSX prefix');
    // F7：失败 ≠ 空——错误横幅与「暂无已安装插件」空态不得同时呈现。
    assert.ok(!text.includes('暂无已安装插件'), 'a failed load must not also render the empty state');
  } finally {
    await unmount(root);
  }
});

test('停用失败（非 ApiError）文案为状态变更失败而非误报加载失败', async () => {
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/inst-1/disable') {
      throw new Error('boom');
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const disableButton = findButtonByText(document.querySelector('[data-testid="plugin-installations"]') as ParentNode, '停用');
    assert.ok(disableButton, 'the disable button renders');
    await act(async () => { disableButton!.click(); });
    await flushEffects();
    const text = document.querySelector('[data-testid="plugin-installations"]')?.textContent ?? '';
    assert.match(text, /插件状态变更失败，请重试/, 'the state-change failure copy surfaces');
    assert.ok(!text.includes('已安装插件加载失败'), 'a toggle failure must not be misreported as a load failure');
    // 状态变更失败 ≠ 空：列表数据仍在，空态不应出现。
    assert.match(text, /Jira 本周待办/, 'the list itself stays rendered');
  } finally {
    await unmount(root);
  }
});

// ---- T15（Issue #114）：安装行「检查升级」→ 五维差异预览面板 ----

function upgradePreviewEnvelope(options?: { isDowngrade?: boolean; endpointChanged?: boolean }): unknown {
  const isDowngrade = options?.isDowngrade ?? true;
  const endpointChanged = options?.endpointChanged ?? true;
  return {
    success: true,
    data: {
      diff: {
        plugin_id: 'com.example.jira-todo',
        current_version: '1.2.0',
        candidate_version: isDowngrade ? '1.1.0' : '1.3.0',
        is_downgrade: isDowngrade,
        endpoint_changed: endpointChanged,
        current_endpoint: 'https://plugins.example.com/jira-todo/v1.2.0/mcp',
        candidate_endpoint: 'https://plugins.example.com/jira-todo/v1.3.0/mcp',
        added_tools: [
          { name: 'added_tool', description: '候选新增', input_schema_digest: 'd'.repeat(64), read_only: true, requires_personal_auth: false, scopes: ['read:jira-work'] },
        ],
        removed_tools: [
          { name: 'removed_tool', description: '候选移除', input_schema_digest: 'e'.repeat(64), read_only: false, requires_personal_auth: true, scopes: null },
        ],
        changed_tools: [
          {
            name: 'changed_tool',
            schema_changed: true,
            scope_changed: true,
            read_write_class_changed: true,
            personal_auth_changed: true,
            current: { name: 'changed_tool', description: '现版', input_schema_digest: 'a'.repeat(64), read_only: true, requires_personal_auth: false, scopes: ['read:jira-work'] },
            candidate: { name: 'changed_tool', description: '候选版', input_schema_digest: 'b'.repeat(64), read_only: false, requires_personal_auth: true, scopes: ['read:jira-work', 'write:jira-work'] },
          },
        ],
      },
      candidate_fingerprint: 'f'.repeat(64),
      candidate_tools_digest: '0'.repeat(64),
    },
  };
}

test('管理员安装行有检查升级按钮：点击请求 upgrade-preview 并渲染五维差异面板（含降级标注）', async () => {
  const { client, captured } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/inst-1/upgrade-preview') {
      return upgradePreviewEnvelope();
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]') as ParentNode;
    const checkButton = findButtonByText(list, '检查升级');
    assert.ok(checkButton, 'the admin installation row carries a check-upgrade button');
    await act(async () => { checkButton!.click(); });
    await flushEffects();
    const previewRequests = captured.filter((entry) => entry.method === 'POST' && entry.path === '/api/v1/plugins/installations/inst-1/upgrade-preview');
    assert.equal(previewRequests.length, 1, 'one upgrade-preview request fires');
    const panel = document.querySelector('[data-testid="plugin-upgrade-preview"]');
    assert.ok(panel, 'the upgrade diff panel renders');
    const text = panel?.textContent ?? '';
    // 维度 1：版本对 + 降级标注。
    assert.match(text, /1\.2\.0/, 'the current version renders');
    assert.match(text, /1\.1\.0/, 'the candidate version renders');
    assert.match(text, /降级/, 'the downgrade carries an explicit badge');
    // 维度 2：端点变化行。
    assert.match(text, /端点/, 'the endpoint dimension renders');
    assert.match(text, /https:\/\/plugins\.example\.com\/jira-todo\/v1\.2\.0\/mcp/, 'the current endpoint renders');
    assert.match(text, /https:\/\/plugins\.example\.com\/jira-todo\/v1\.3\.0\/mcp/, 'the candidate endpoint renders');
    // 维度 3/4：新增与移除工具表。
    assert.match(text, /added_tool/, 'an added tool row renders');
    assert.match(text, /removed_tool/, 'a removed tool row renders');
    // 维度 5：变更工具 + 四个变更理由徽标（schema/scope/读写分类/授权面）。
    assert.match(text, /changed_tool/, 'a changed tool row renders');
    assert.match(text, /schema 变更/, 'the schema-change reason badge renders');
    assert.match(text, /scope 变更/, 'the scope-change reason badge renders');
    assert.match(text, /读写分类变更/, 'the read/write-class reason badge renders');
    assert.match(text, /授权面变更/, 'the personal-auth reason badge renders');
    // 候选身份（指纹/摘要）帮助管理员核对将要接受的目录。
    assert.match(text, /f{64}/, 'the candidate identity fingerprint renders');
    // 预览不切换版本：安装列表的已接受版本保持 1.2.0。
    const listText = document.querySelector('[data-testid="plugin-installations"]')?.textContent ?? '';
    assert.match(listText, /1\.2\.0/, 'the accepted version in the installed list is untouched by a read-only preview');
  } finally {
    await unmount(root);
  }
});

test('升级方向的预览不渲染降级徽标', async () => {
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/inst-1/upgrade-preview') {
      return upgradePreviewEnvelope({ isDowngrade: false });
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const checkButton = findButtonByText(document.querySelector('[data-testid="plugin-installations"]') as ParentNode, '检查升级');
    assert.ok(checkButton);
    await act(async () => { checkButton!.click(); });
    await flushEffects();
    const panel = document.querySelector('[data-testid="plugin-upgrade-preview"]');
    assert.ok(panel, 'the panel still renders for an upgrade-direction diff');
    const text = panel?.textContent ?? '';
    assert.match(text, /1\.3\.0/, 'the candidate version renders');
    assert.ok(!text.includes('降级'), 'no downgrade badge renders when the candidate is newer');
  } finally {
    await unmount(root);
  }
});

test('候选不可达时显示错误条（后端 ApiError 原文透传）且安装列表不变形', async () => {
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/inst-1/upgrade-preview') {
      throw new ApiError({ code: 'HTTP_503', message: 'candidate manifest unreachable: 候选清单抓取失败' });
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]') as ParentNode;
    const checkButton = findButtonByText(list, '检查升级');
    assert.ok(checkButton);
    await act(async () => { checkButton!.click(); });
    await flushEffects();
    const errorBanner = document.querySelector('[data-testid="plugin-upgrade-error"]');
    assert.ok(errorBanner, 'the unreachable-candidate error banner renders');
    assert.match(errorBanner?.textContent ?? '', /候选清单抓取失败/, 'the backend message passes through verbatim');
    // T08-OCR1-F3 同类防护：前缀只由渲染层拼一次，文案不得自我重复。
    assert.equal(
      (errorBanner?.textContent ?? '').indexOf('升级预览失败'),
      (errorBanner?.textContent ?? '').lastIndexOf('升级预览失败'),
      'no duplicated 升级预览失败 prefix',
    );
    assert.ok(!document.querySelector('[data-testid="plugin-upgrade-preview"]'), 'no diff panel leaks after a failed preview');
    // 安装列表不变形：行内容、版本徽标与既有治理按钮全部保持。
    const listText = list.textContent ?? '';
    assert.match(listText, /Jira 本周待办/, 'the installation row stays rendered');
    assert.match(listText, /1\.2\.0/, 'the accepted version badge stays');
    assert.match(listText, /启用中/, 'the state badge stays');
    assert.ok(findButtonByText(list, '停用'), 'the disable governance button stays');
    assert.ok(!listText.includes('暂无已安装插件'), 'the empty state must not replace the intact list');
  } finally {
    await unmount(root);
  }
});

test('非 ApiError 的升级预览失败呈现统一中文文案（console.warn 留痕）', async () => {
  const warnings: unknown[][] = [];
  const originalWarn = console.warn;
  console.warn = (...args: unknown[]) => { warnings.push(args); };
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/inst-1/upgrade-preview') {
      throw new Error('fetch failed');
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const checkButton = findButtonByText(document.querySelector('[data-testid="plugin-installations"]') as ParentNode, '检查升级');
    assert.ok(checkButton);
    await act(async () => { checkButton!.click(); });
    await flushEffects();
    const errorBanner = document.querySelector('[data-testid="plugin-upgrade-error"]');
    assert.ok(errorBanner, 'the error banner renders');
    assert.match(errorBanner?.textContent ?? '', /升级预览失败/, 'a non-ApiError failure gets the unified Chinese copy');
    assert.equal(
      (errorBanner?.textContent ?? '').indexOf('升级预览失败'),
      (errorBanner?.textContent ?? '').lastIndexOf('升级预览失败'),
      'no duplicated prefix from the storage/render split',
    );
    assert.ok(warnings.length >= 1, 'the raw cause is preserved in console.warn');
  } finally {
    console.warn = originalWarn;
    await unmount(root);
  }
});

test('viewer 安装行无检查升级按钮（registry minRole=admin 治理动作）', async () => {
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'viewer' }));
  try {
    await flushEffects();
    assert.ok(!findButtonByText(document, '检查升级'), 'viewer must not see the check-upgrade action');
    assert.ok(!document.querySelector('[data-testid="plugin-upgrade-preview"]'), 'no diff panel renders for a viewer');
  } finally {
    await unmount(root);
  }
});

// ---- T15-OCR1-F3：其他行预览进行中的按钮禁用反馈（不静默吞点击） ----

test('其他行升级预览进行中时检查升级按钮呈现禁用态且完成后恢复', async () => {
  let releaseFirst!: (value: unknown) => void;
  const gatedPreview = new Promise<unknown>((resolve) => { releaseFirst = resolve; });
  const { client, captured } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return {
        success: true,
        data: [
          { ...jiraSummary },
          { ...jiraSummary, installation_id: 'inst-2', plugin_id: 'com.example.weather', name: '天气查询' },
        ],
      };
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/inst-1/upgrade-preview') {
      return gatedPreview; // 行 A 的预览挂起：重抓清单+核验的长网络往返
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const rows = Array.from(document.querySelectorAll('[data-testid="plugin-installations"] li'));
    assert.equal(rows.length, 2, 'two installation rows render');
    const firstButton = findButtonByText(rows[0] as ParentNode, '检查升级');
    const secondButton = findButtonByText(rows[1] as ParentNode, '检查升级');
    assert.ok(firstButton && secondButton, 'both rows carry a check-upgrade button');
    assert.equal(firstButton!.disabled, false, 'idle rows are clickable');
    await act(async () => { firstButton!.click(); });
    await flushEffects(2);
    // 挂起期间：请求行经 loading 禁用；另一行必须有可见禁用态——全局互斥不能
    // 只靠静默 return 吞点击（用户会误以为按钮失效）。
    assert.equal(firstButton!.disabled, true, 'the in-flight row disables itself via loading');
    assert.equal(secondButton!.disabled, true, 'another row disables visibly while a preview is in flight');
    // 禁用态下点击另一行不发请求（全局互斥语义不变）。
    await act(async () => { secondButton!.click(); });
    await flushEffects(2);
    assert.equal(captured.filter((entry) => entry.path === '/api/v1/plugins/installations/inst-2/upgrade-preview').length, 0, 'a disabled row fires no request');
    releaseFirst(upgradePreviewEnvelope());
    await flushEffects();
    assert.equal(secondButton!.disabled, false, 'buttons re-enable once the preview settles');
    assert.ok(document.querySelector('[data-testid="plugin-upgrade-preview"]'), 'the gated preview resolves and renders');
  } finally {
    await unmount(root);
  }
});

// ---- T15-OCR1-F4(low)：升级错误条生命周期与列表操作对齐 ----

test('停用成功后升级预览错误条同步清除（横幅清理行为一致）', async () => {
  const rows: unknown[] = [{ ...jiraSummary }];
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [...rows] };
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/inst-1/upgrade-preview') {
      throw new ApiError({ code: 'HTTP_503', message: 'candidate manifest unreachable: 候选清单抓取失败' });
    }
    if (input.method === 'POST' && input.path === '/api/v1/plugins/installations/inst-1/disable') {
      rows[0] = { ...jiraSummary, state: 'disabled' };
      const disabled = installationEnvelope() as { data: Record<string, unknown> };
      disabled.data.state = 'disabled';
      return disabled;
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]') as ParentNode;
    await act(async () => { findButtonByText(list, '检查升级')!.click(); });
    await flushEffects();
    assert.ok(document.querySelector('[data-testid="plugin-upgrade-error"]'), 'the upgrade error banner renders first');
    await act(async () => { findButtonByText(list, '停用')!.click(); });
    await flushEffects();
    assert.ok(!document.querySelector('[data-testid="plugin-upgrade-error"]'), 'a successful state change clears the stale upgrade error banner');
    assert.match(document.querySelector('[data-testid="plugin-installations"]')?.textContent ?? '', /已停用/, 'the state change itself still lands');
  } finally {
    await unmount(root);
  }
});

// ---- T19: 工具策略治理行（enabled 开关 + 成员审批开关同行）----

function toolPolicyRows(): Array<Record<string, unknown>> {
  return [
    {
      name: 'search_my_week_issues',
      description: '搜索本周待办',
      read_only: true,
      requires_personal_auth: true,
      scopes: ['read:jira-work'],
      enabled: true,
      require_approval: false,
      disabled_reason: '',
    },
    {
      name: 'create_todo',
      description: '创建待办',
      read_only: false,
      requires_personal_auth: true,
      scopes: [],
      enabled: false,
      require_approval: false,
      disabled_reason: 'write tool disabled by default; enable explicit',
    },
  ];
}

function switchByAriaLabel(root: ParentNode, label: string): HTMLButtonElement | null {
  return root.querySelector(`button[role="switch"][aria-label="${label}"]`);
}

test('管理员展开工具治理：GET tools 渲染策略行与双开关初始态（含关闭原因）', async () => {
  const { client, captured } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/tools') {
      return { success: true, data: toolPolicyRows() };
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]') as ParentNode;
    await act(async () => { findButtonByText(list, '工具治理')!.click(); });
    await flushEffects();
    const governance = document.querySelector('[data-testid="plugin-tool-policy"]');
    assert.ok(governance, 'the tool governance surface renders after expand');
    assert.equal(
      captured.filter((entry) => entry.method === 'GET' && entry.path === '/api/v1/plugins/installations/inst-1/tools').length,
      1, 'exactly one governance list request fires');
    const text = governance?.textContent ?? '';
    assert.match(text, /search_my_week_issues/);
    assert.match(text, /create_todo/);
    assert.match(text, /write tool disabled by default; enable explicit/, 'the disabled write tool carries its reason');
    const readEnabled = switchByAriaLabel(governance as ParentNode, 'search_my_week_issues 启用');
    assert.ok(readEnabled, 'each tool row carries an enable switch');
    assert.equal(readEnabled!.getAttribute('aria-checked'), 'true');
    const writeEnabled = switchByAriaLabel(governance as ParentNode, 'create_todo 启用');
    assert.equal(writeEnabled!.getAttribute('aria-checked'), 'false', 'the write tool installs disabled');
    const readApproval = switchByAriaLabel(governance as ParentNode, 'search_my_week_issues 成员审批');
    const writeApproval = switchByAriaLabel(governance as ParentNode, 'create_todo 成员审批');
    assert.ok(readApproval && writeApproval, 'each tool row carries the member-approval switch on the SAME row');
    assert.equal(writeApproval!.getAttribute('aria-checked'), 'false');
  } finally {
    await unmount(root);
  }
});

test('点击写工具启用开关：PUT policy 只带 enabled 且治理行刷新', async () => {
  const rows = toolPolicyRows();
  const { client, captured } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/tools') {
      return { success: true, data: rows.map((row) => ({ ...row })) };
    }
    if (input.method === 'PUT' && input.path === '/api/v1/plugins/installations/inst-1/tools/create_todo/policy') {
      const patch = input.body as { enabled?: boolean };
      const target = rows.find((row) => row.name === 'create_todo') as Record<string, unknown>;
      if (patch.enabled !== undefined) target.enabled = patch.enabled;
      return { success: true, data: rows.map((row) => ({ ...row })) };
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]') as ParentNode;
    await act(async () => { findButtonByText(list, '工具治理')!.click(); });
    await flushEffects();
    const governance = document.querySelector('[data-testid="plugin-tool-policy"]') as ParentNode;
    await act(async () => { switchByAriaLabel(governance, 'create_todo 启用')!.click(); });
    await flushEffects();
    const put = captured.find((entry) => entry.method === 'PUT' && entry.path === '/api/v1/plugins/installations/inst-1/tools/create_todo/policy');
    assert.ok(put, 'the enable toggle fires the policy PUT');
    assert.deepEqual(put!.body, { enabled: true }, 'the patch carries ONLY the toggled field — require_approval keeps its value');
    assert.equal(
      switchByAriaLabel(document.querySelector('[data-testid="plugin-tool-policy"]') as ParentNode, 'create_todo 启用')!.getAttribute('aria-checked'),
      'true', 'the refreshed governance list flips the switch');
  } finally {
    await unmount(root);
  }
});

test('点击成员审批开关：PUT policy 只带 require_approval 且开关翻转', async () => {
  const rows = toolPolicyRows();
  const { client, captured } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/tools') {
      return { success: true, data: rows.map((row) => ({ ...row })) };
    }
    if (input.method === 'PUT' && input.path === '/api/v1/plugins/installations/inst-1/tools/create_todo/policy') {
      const patch = input.body as { require_approval?: boolean };
      const target = rows.find((row) => row.name === 'create_todo') as Record<string, unknown>;
      if (patch.require_approval !== undefined) target.require_approval = patch.require_approval;
      return { success: true, data: rows.map((row) => ({ ...row })) };
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]') as ParentNode;
    await act(async () => { findButtonByText(list, '工具治理')!.click(); });
    await flushEffects();
    const governance = document.querySelector('[data-testid="plugin-tool-policy"]') as ParentNode;
    await act(async () => { switchByAriaLabel(governance, 'create_todo 成员审批')!.click(); });
    await flushEffects();
    const put = captured.find((entry) => entry.method === 'PUT' && entry.path === '/api/v1/plugins/installations/inst-1/tools/create_todo/policy');
    assert.ok(put, 'the approval toggle fires the policy PUT');
    assert.deepEqual(put!.body, { require_approval: true }, 'the patch carries ONLY require_approval');
    assert.equal(
      switchByAriaLabel(document.querySelector('[data-testid="plugin-tool-policy"]') as ParentNode, 'create_todo 成员审批')!.getAttribute('aria-checked'),
      'true', 'the refreshed list flips the approval switch');
  } finally {
    await unmount(root);
  }
});

test('viewer 安装行无工具治理入口', async () => {
  const { client, captured } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }] };
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'viewer' }));
  try {
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]') as ParentNode;
    assert.ok(!findButtonByText(list, '工具治理'), 'no governance entry for a viewer');
    assert.equal(captured.filter((entry) => entry.path.endsWith('/tools')).length, 0, 'no governance request fires for a viewer');
  } finally {
    await unmount(root);
  }
});

// ---- T19-OCR1-F3/F4：治理面状态污染与生命周期 ----

const otherSummary = { ...jiraSummary, installation_id: 'inst-2', plugin_id: 'com.example.other', name: '其他插件', tool_count: 1 };

test('T19-OCR1-F3 PUT 进行中其他安装的工具治理按钮禁用（互斥冻结，迟到失败不得污染新面板）', async () => {
  let releasePut: ((value: unknown) => void) | undefined;
  const { client } = stubClient(async (input) => {
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
      return { success: true, data: [{ ...jiraSummary }, { ...otherSummary }] };
    }
    if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/tools') {
      return { success: true, data: toolPolicyRows() };
    }
    if (input.method === 'PUT' && input.path === '/api/v1/plugins/installations/inst-1/tools/create_todo/policy') {
      // 挂起 PUT：模拟在途请求，由测试手动放行。
      await new Promise<unknown>((resolve) => { releasePut = resolve; });
      throw new ApiError({ code: 'HTTP_503', message: 'late policy failure' });
    }
    throw new Error(`unexpected request ${input.method} ${input.path}`);
  });
  const root = await mount(React.createElement(PluginsSettingsPanel, { client, role: 'admin' }));
  try {
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]') as ParentNode;
    await act(async () => { findButtonByText(list, '工具治理')!.click(); });
    await flushEffects();
    const governance = document.querySelector('[data-testid="plugin-tool-policy"]') as ParentNode;
    await act(async () => { switchByAriaLabel(governance, 'create_todo 成员审批')!.click(); });
    await flushEffects();
    // PUT 在途：另一安装的工具治理按钮必须禁用（互斥冻结）。
    const otherGovernance = Array.from(list.querySelectorAll('button'))
      .find((button) => (button.textContent ?? '').trim() === '工具治理'
        && button.closest('li')?.textContent?.includes('其他插件')) as HTMLButtonElement | undefined;
    assert.ok(otherGovernance, 'the other installation carries its governance button');
    assert.equal(otherGovernance!.disabled, true, 'the other installation\'s governance button is frozen while a policy PUT is in flight');
    // 放行 PUT → 失败回到本安装面板（错误归因到发起安装，不污染他人）。
    releasePut!(undefined);
    await flushEffects();
    assert.match(
      document.querySelector('[data-testid="plugin-tool-policy"]')?.textContent ?? '',
      /late policy failure/, 'the late failure surfaces on the OWNING installation\'s panel');
  } finally {
    await unmount(root);
  }
});

test('T19-OCR1-F4 client 变化重载安装列表时治理面同步失效（不残留旧 client 数据、迟到失败零污染）', async () => {
  let releasePut: ((value: unknown) => void) | undefined;
  const client1 = {
    request: async (input: { method: string; path: string; body?: unknown }) => {
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
        return { success: true, data: [{ ...jiraSummary }] };
      }
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations/inst-1/tools') {
        return { success: true, data: toolPolicyRows() };
      }
      if (input.method === 'PUT' && input.path === '/api/v1/plugins/installations/inst-1/tools/create_todo/policy') {
        await new Promise<unknown>((resolve) => { releasePut = resolve; });
        throw new ApiError({ code: 'HTTP_503', message: 'late policy failure' });
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
  };
  const client2 = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/plugins/installations') {
        // 同一安装仍在列表（client 切换 ≠ 安装消失）：若治理面状态不失效，
        // 旧行会继续渲染、开关会用新 client 对旧 installationId 发 PUT。
        return { success: true, data: [{ ...jiraSummary }] };
      }
      throw new Error(`unexpected request ${input.method} ${input.path}`);
    },
  };
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  try {
    await act(async () => { root.render(React.createElement(PluginsSettingsPanel, { client: client1 as never, role: 'admin' })); });
    await flushEffects();
    const list = document.querySelector('[data-testid="plugin-installations"]') as ParentNode;
    await act(async () => { findButtonByText(list, '工具治理')!.click(); });
    await flushEffects();
    assert.ok(document.querySelector('[data-testid="plugin-tool-policy"]'), 'the governance panel opens on the first client');
    await act(async () => { switchByAriaLabel(document.querySelector('[data-testid="plugin-tool-policy"]') as ParentNode, 'create_todo 成员审批')!.click(); });
    await flushEffects();
    // 切换 client（跨账号/工作区）：effect 重载列表，治理面必须整体失效收起。
    await act(async () => { root.render(React.createElement(PluginsSettingsPanel, { client: client2 as never, role: 'admin' })); });
    await flushEffects();
    assert.ok(!document.querySelector('[data-testid="plugin-tool-policy"]'),
      'the governance panel collapses when the client change reloads the installation list — stale rows never survive a client switch');
    // 迟到的 PUT 失败回到已失效的 null 状态：不渲染任何错误条（不复活面板）。
    releasePut!(undefined);
    await flushEffects();
    assert.ok(!document.querySelector('[data-testid="plugin-tool-policy"]'), 'a late PUT failure must not resurrect the invalidated panel');
    assert.match(document.querySelector('[data-testid="plugin-installations"]')?.textContent ?? '', /Jira 本周待办/, 'the reloaded list still renders the installation row');
  } finally {
    await act(async () => { root.unmount(); });
    document.body.replaceChildren();
  }
});
