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
