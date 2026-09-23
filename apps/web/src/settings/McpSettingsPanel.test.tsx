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

// ---- jsdom interaction coverage: Vue McpServiceDialog drawer parity ----
const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  MouseEvent: dom.window.MouseEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// S6：tdesign 组件挂载依赖（Input 的 rAF、listener 的 document）——必须在动态
// import 面板前就位（tdesign 模块加载期探测 window，晚于 import 会锁死
// useEventCallback 的 noop 分支 / attachEvent 旧分支）。
Object.assign(globalThis, {
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
});
const { McpSettingsPanel, importMcpConfig, validateMcpDraft, buildMcpConnectionPayload, clampMcpDrawerWidth, normalizeMcpAdvancedNumber } = await import('./McpSettingsPanel.tsx');
const { formatMessage } = await import('@weknora/i18n');

const client = {} as never;

test('MCP drawer width follows Vue min/max and viewport clamp rules', () => {
  assert.equal(clampMcpDrawerWidth(680, 1200), 680);
  assert.equal(clampMcpDrawerWidth(400, 1200), 560);
  assert.equal(clampMcpDrawerWidth(1000, 1200), 920);
  assert.equal(clampMcpDrawerWidth(680, 500), 500);
});

test('MCP advanced number fields keep transient empty input and normalize on blur/save', () => {
  assert.equal(normalizeMcpAdvancedNumber('', 30, 1, 300), 30);
  assert.equal(normalizeMcpAdvancedNumber(-4, 30, 1, 300), 1);
  assert.equal(normalizeMcpAdvancedNumber(999, 3, 0, 10), 10);
  assert.equal(normalizeMcpAdvancedNumber(0, 1, 0, 60), 0);
});

test('MCP settings keeps the Vue empty state for a viewer', () => {
  const html = renderToStaticMarkup(React.createElement(McpSettingsPanel, {
    client,
    initialServices: [],
    role: 'viewer',
  }));
  assert.match(html, /暂无 MCP 服务/);
  assert.doesNotMatch(html, /添加服务/);
});

test('MCP settings keeps the Vue admin-only management boundary for a viewer-level role', () => {
  const html = renderToStaticMarkup(React.createElement(McpSettingsPanel, {
    client,
    initialServices: [{ id: 'mcp-1', name: 'Docs', description: 'Search docs', enabled: true, transport_type: 'sse', is_builtin: false }],
    role: 'viewer',
  }));
  assert.doesNotMatch(html, /编辑/);
  assert.doesNotMatch(html, /删除/);
  assert.doesNotMatch(html, /添加服务/);
  assert.doesNotMatch(html, /role="switch"/);
  assert.match(html, /Docs/);
});

test('MCP settings gives owners the Vue management controls (hasRole ranks owner above admin)', () => {
  const html = renderToStaticMarkup(React.createElement(McpSettingsPanel, {
    client,
    initialServices: [{ id: 'mcp-1', name: 'Docs', description: 'Search docs', enabled: true, transport_type: 'sse', is_builtin: false }],
    role: 'owner',
  }));
  assert.match(html, /编辑/);
  assert.match(html, /删除/);
  assert.match(html, /添加服务/);
  assert.match(html, /role="switch"/);
  assert.match(html, /Docs/);
});

test('MCP settings renders the Vue dashed add-service tile in the owner empty state (R428)', () => {
  const html = renderToStaticMarkup(React.createElement(McpSettingsPanel, {
    client,
    initialServices: [],
    role: 'owner',
  }));
  assert.doesNotMatch(html, /暂无 MCP 服务/);
  assert.match(html, /添加服务/);
  assert.match(html, /wk-mcp-add-tile/, 'tile keeps the Vue dashed service-card--add border hook');
});

test('MCP settings renders service metadata and admin actions', () => {
  const html = renderToStaticMarkup(React.createElement(McpSettingsPanel, {
    client,
    initialServices: [{ id: 'mcp-1', name: 'Docs', description: 'Search docs', enabled: true, transport_type: 'sse', is_builtin: false }],
    role: 'admin',
  }));
  assert.match(html, /Docs/);
  assert.match(html, /Search docs/);
  assert.match(html, /编辑/);
  assert.match(html, /删除/);
  assert.match(html, /添加服务/);
  assert.match(html, /wk-mcp-page-header/);
  assert.match(html, /wk-mcp-page-header h2|<h2>/);
});

test('MCP settings uses shared Vue-derived Chinese copy for the default locale', () => {
  const html = renderToStaticMarkup(React.createElement(McpSettingsPanel, {
    client,
    initialServices: [{ id: 'mcp-1', name: 'Docs', description: '', enabled: true, transport_type: 'sse', is_builtin: false }],
    role: 'admin',
  }));
  assert.match(html, /MCP 服务管理/);
  assert.match(html, /管理外部 MCP/);
  assert.match(html, /添加服务/);
  assert.match(html, /已启用/);
  assert.equal(formatMessage('zh-CN', 'mcpServiceDialog.testConnection'), '测试连接');
  assert.equal(formatMessage('en-US', 'mcpServiceDialog.testConnection'), 'Test connection');
});

test('MCP JSON import maps transport, auth, and custom headers without saving', () => {
  const base = { name: '', description: '', usageInstructions: '', url: '', transportType: 'sse', enabled: true, authType: '', apiKeyHeader: '', apiKey: '', oauthScopes: '', headers: [], timeout: 30, retryCount: 3, retryDelay: 1, codeImport: '', codeImportError: '', authConfig: {} } as never;
  const draft = importMcpConfig(JSON.stringify({ mcpServers: { docs: { url: 'https://example.com/mcp', headers: { Authorization: 'Bearer secret', 'X-Trace': 'yes' } } } }), base);
  assert.equal(draft.name, 'docs');
  assert.equal(draft.transportType, 'http-streamable');
  assert.equal(draft.authType, 'api_key');
  assert.equal(draft.apiKeyHeader, 'Authorization');
  assert.deepEqual(draft.headers, [{ key: 'X-Trace', value: 'yes' }]);
});

test('MCP connection payload mirrors Vue buildPayload and omits usage/description (backend rejects empty usage on PUT)', () => {
  // Vue McpServiceDialog.vue:898-938 never sends description/usage_instructions in the
  // connection save; the backend PUT rejects usage_instructions outside 1..16000 chars
  // (live 400 verified 2026-09-13), which broke React edit/create for services without
  // saved instructions.
  const base = { name: ' Docs ', description: 'd', usageInstructions: '', url: 'https://example.com/mcp', transportType: 'sse', enabled: true, authType: 'api_key', apiKeyHeader: ' X-Auth ', apiKey: '', oauthScopes: '', headers: [{ key: 'A', value: '1' }, { key: ' ', value: 'x' }, { key: 'B', value: '' }], timeout: 30, retryCount: 3, retryDelay: 1, codeImport: '', codeImportError: '', authConfig: {} } as Parameters<typeof buildMcpConnectionPayload>[0];
  const payload = buildMcpConnectionPayload(base);
  assert.equal(payload.name, 'Docs');
  assert.equal(payload.enabled, true);
  assert.equal(payload.transport_type, 'sse');
  assert.equal(payload.url, 'https://example.com/mcp');
  assert.deepEqual(payload.headers, { A: '1' });
  assert.deepEqual(payload.advanced_config, { timeout: 30, retry_count: 3, retry_delay: 1 });
  assert.deepEqual(payload.auth_config, { auth_type: 'api_key', api_key_header: 'X-Auth' });
  assert.ok(!('usage_instructions' in payload), 'must not send usage_instructions in connection save');
  assert.ok(!('description' in payload), 'must not send description in connection save');
  // stdio/empty url is sent as undefined exactly like Vue (url || undefined)
  assert.equal(buildMcpConnectionPayload({ ...base, url: '' } as never).url, undefined);
});

test('MCP draft validation mirrors Vue submit rules before mutation', () => {
  const base = { name: 'Docs', description: '', usageInstructions: 'Use for docs', url: 'https://example.com/mcp', transportType: 'sse', enabled: true, authType: '', apiKeyHeader: '', apiKey: '', oauthScopes: '', headers: [], timeout: 30, retryCount: 3, retryDelay: 1, codeImport: '', codeImportError: '', authConfig: {} } as Parameters<typeof validateMcpDraft>[0];
  assert.equal(validateMcpDraft({ ...base, name: '' }, 0), 'nameRequired');
  assert.equal(validateMcpDraft({ ...base, url: 'not a url' }, 0), 'urlInvalid');
  assert.equal(validateMcpDraft({ ...base, usageInstructions: '' }, 1), 'usageRequired');
  assert.equal(validateMcpDraft({ ...base, transportType: 'stdio' }, 0), 'stdioUnsupported');
  assert.equal(validateMcpDraft(base, 0), null);
});

test('MCP create payload carries the add-mode secret inline exactly like Vue buildPayload(true)', () => {
  const base = { name: 'Docs', description: '', usageInstructions: '', url: 'https://example.com/mcp', transportType: 'sse', enabled: true, authType: 'api_key', apiKeyHeader: '', apiKey: ' secret ', oauthScopes: '', headers: [], timeout: 30, retryCount: 3, retryDelay: 1, codeImport: '', codeImportError: '', authConfig: {} } as Parameters<typeof buildMcpConnectionPayload>[0];
  const create = buildMcpConnectionPayload(base, true);
  assert.equal((create.auth_config as Record<string, unknown>).api_key, 'secret');
  // Edit mode routes secrets through the /credentials subresource, never the body.
  const edit = buildMcpConnectionPayload(base, false);
  assert.ok(!('api_key' in (edit.auth_config as Record<string, unknown>)));
});

const { createRoot } = await import('react-dom/client');
const { act } = await import('react');

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

type StubMetadata = {
  serviceId?: string;
  tools: Array<{ name: string; description?: string }>;
  serverName: string;
  serverVersion: string;
  instructions?: string;
  serverDescription?: string;
  syncedAt: string;
  stale: boolean;
};

function mcpStubClient(overrides: { metadata?: StubMetadata; metadataGet?: () => Promise<StubMetadata | null>; metadataRefresh?: () => Promise<StubMetadata>; toolApprovalsList?: () => Promise<unknown[]>; toolApprovalsUpdate?: () => Promise<unknown> } = {}) {
  const metadata: StubMetadata = overrides.metadata ?? { tools: [{ name: 'search', description: 'Search docs' }], serverName: 'Srv', serverVersion: '1.0', syncedAt: '2026-09-13T00:00:00Z', stale: false };
  return {
    configuration: {
      mcp: {
        list: async () => [],
        create: async () => ({ id: 'svc-1', name: 'Docs', transport_type: 'sse', enabled: true }),
        update: async () => ({ id: 'svc-1', name: 'Docs', transport_type: 'sse', enabled: true }),
        remove: async () => ({}),
        credentials: { put: async () => ({}) },
        metadata: {
          get: overrides.metadataGet ?? (async () => metadata),
          refresh: overrides.metadataRefresh ?? (async () => metadata),
        },
        toolApprovals: { list: overrides.toolApprovalsList ?? (async () => []), update: overrides.toolApprovalsUpdate ?? (async () => ({})) },
        usageInstructions: { generate: async () => 'generated usage instructions' },
        oauth: {
          status: async () => ({ authorized: false, state: 'reauth_required', refreshAvailable: false }),
          authorizeUrl: async () => ({ authorizationUrl: 'https://auth.test', authorizationAttempt: 1 }),
          revoke: async () => ({}),
        },
      },
    },
  } as never;
}

test('MCP metadata automatically refreshes when Vue cache lookup returns empty', async () => {
  let refreshCalls = 0;
  const root = await mountEditor(React.createElement(McpSettingsPanel, {
    client: mcpStubClient({
      metadataGet: async () => null,
      metadataRefresh: async () => { refreshCalls += 1; return { tools: [{ name: 'search' }], serverName: 'Srv', serverVersion: '1.0', syncedAt: '2026-09-14T00:00:00Z', stale: false }; },
    }),
    initialServices: [{ id: 'svc-1', name: 'Docs', url: 'https://example.com/mcp', enabled: true, transport_type: 'sse', is_builtin: false }],
    role: 'admin',
  }));
  try {
    await act(async () => { findButton('编辑')?.click(); });
    const nameInput = document.querySelector('input[placeholder="请输入服务名称"]') as HTMLInputElement | null;
    const urlInput = document.querySelector('input[placeholder="https://example.com/mcp"]') as HTMLInputElement | null;
    await act(async () => { setInputValue(nameInput!, 'Docs'); setInputValue(urlInput!, 'https://example.com/mcp'); });
    await act(async () => { submitForm(document.querySelector('.wks-mcp-drawer form') as HTMLFormElement); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    assert.equal(refreshCalls, 1, 'empty cache triggers one Vue-compatible refresh');
    assert.match(document.querySelector('section[aria-label="Tools 清单"]')?.textContent ?? '', /1 个工具/);
  } finally {
    await unmountEditor(root);
  }
});

test('MCP tools render from a cached Vue snapshot while policy loading keeps switches disabled', async () => {
  const policyGate = deferred<unknown[]>();
  const root = await mountEditor(React.createElement(McpSettingsPanel, {
    client: mcpStubClient({
      metadata: { tools: [{ name: 'search' }], serverName: 'Srv', serverVersion: '1.0', syncedAt: '2026-09-14T00:00:00Z', stale: false },
      toolApprovalsList: () => policyGate.promise,
    }),
    initialServices: [{ id: 'svc-1', name: 'Docs', transport_type: 'sse', enabled: true, is_builtin: false }],
    role: 'admin',
  }));
  try {
    await act(async () => { findButton('编辑')?.click(); });
    const nameInput = document.querySelector('input[placeholder="请输入服务名称"]') as HTMLInputElement;
    const urlInput = document.querySelector('input[placeholder="https://example.com/mcp"]') as HTMLInputElement;
    await act(async () => { setInputValue(nameInput, 'Docs'); setInputValue(urlInput, 'https://example.com/mcp'); });
    await act(async () => { submitForm(document.querySelector('.wks-mcp-drawer form') as HTMLFormElement); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    assert.match(document.body.textContent ?? '', /search/);
    const switches = Array.from(document.querySelectorAll('[role="switch"]')) as HTMLButtonElement[];
    assert.equal(switches.length, 2);
    assert.ok(switches.every((control) => control.disabled), 'policy switches stay disabled while Vue policy loading is pending');
    await act(async () => { policyGate.resolve([]); await policyGate.promise; });
  } finally {
    await unmountEditor(root);
  }
});

test('MCP metadata refresh keeps cached tool policies interactive like Vue', async () => {
  const refreshGate = deferred<StubMetadata>();
  const root = await mountEditor(React.createElement(McpSettingsPanel, {
    client: mcpStubClient({ metadataRefresh: () => refreshGate.promise }),
    initialServices: [{ id: 'svc-1', name: 'Docs', transport_type: 'sse', enabled: true, is_builtin: false }],
    role: 'admin',
  }));
  try {
    await act(async () => { findButton('编辑')?.click(); });
    const nameInput = document.querySelector('input[placeholder="请输入服务名称"]') as HTMLInputElement;
    const urlInput = document.querySelector('input[placeholder="https://example.com/mcp"]') as HTMLInputElement;
    await act(async () => { setInputValue(nameInput, 'Docs'); setInputValue(urlInput, 'https://example.com/mcp'); });
    await act(async () => { submitForm(document.querySelector('.wks-mcp-drawer form') as HTMLFormElement); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    const refreshButton = findButton('刷新');
    assert.ok(refreshButton, 'cached metadata exposes refresh');
    const switches = Array.from(document.querySelectorAll('[role="switch"]')) as HTMLButtonElement[];
    assert.equal(switches.length, 2);
    assert.ok(switches.every((control) => !control.disabled), 'cached policies are interactive before refresh');
    await act(async () => { refreshButton?.click(); });
    assert.ok(switches.every((control) => !control.disabled), 'Vue keeps cached policy controls enabled while metadata refreshes');
    await act(async () => { refreshGate.resolve({ tools: [{ name: 'search' }], serverName: 'Srv', serverVersion: '1.0', syncedAt: '2026-09-14T00:00:00Z', stale: false }); await refreshGate.promise; });
  } finally {
    await unmountEditor(root);
  }
});

function findButton(label: string): HTMLButtonElement | undefined {
  // S6：tdesign Button 的 disabled 态渲染 div.t-button（台账 #7），经类查询。
  return Array.from(document.querySelectorAll('button, .t-button')).find((button) => (button.textContent ?? '').includes(label)) as HTMLButtonElement | undefined;
}
function buttonDisabled(label: string): boolean | undefined {
  return findButton(label)?.classList.contains('t-is-disabled');
}

function setInputValue(input: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = input instanceof dom.window.HTMLTextAreaElement ? dom.window.HTMLTextAreaElement.prototype : dom.window.HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, 'value')?.set;
  setter?.call(input, value);
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
}

function submitForm(form: HTMLFormElement) {
  form.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true }));
}

async function mountEditor(element: React.ReactElement) {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(element); });
  return root;
}

async function unmountEditor(root: ReturnType<typeof createRoot>) {
  await act(async () => root.unmount());
  document.body.replaceChildren();
}

test('MCP editor step 0 matches the Vue drawer structure and offers no stdio transport', async () => {
  const root = await mountEditor(React.createElement(McpSettingsPanel, { client: mcpStubClient(), initialServices: [], role: 'admin' }));
  try {
    await act(async () => { findButton('添加服务')?.click(); });
    const dialog = document.querySelector('.wks-mcp-drawer');
    assert.ok(dialog, 'drawer renders');
    assert.ok(dialog?.parentElement?.classList.contains('wks-mcp-overlay'), 'drawer uses the Vue right-side overlay shell');
    assert.ok(document.querySelector('[role="separator"][aria-orientation="vertical"]'), 'resizable Vue drawer exposes a vertical separator handle');
    const text = dialog?.textContent ?? '';
    // Scope to fieldset legends: the steps nav also says 连接配置.
    const legends = Array.from(dialog?.querySelectorAll('legend') ?? []).map((el) => el.textContent ?? '');
    const sections = ['基本信息', '连接配置', '认证配置', '高级配置'].map((title) => legends.indexOf(title));
    assert.ok(sections.every((index) => index >= 0), 'all Vue sections render');
    assert.ok(sections[0] < sections[1] && sections[1] < sections[2] && sections[2] < sections[3], 'Vue section order preserved');
    assert.match(text, /关闭后该服务不会被调用/);
    // R484 G4 D5 (R482 report-B3.md D5): the drawer subtitle shows the
    // transport label + enabled chip like Vue McpServiceDialog.vue lines
    // 27-36 (transportLabel + subtitle-tag). The scanner's 「SSE已启用」 vs
    // 「已启用」 diff was an innerText tokenization artifact (Vue concatenates
    // the two inline spans; the live DOMs are equivalent) — this locks the
    // aligned state so the SSE prefix cannot silently disappear.
    const subtitle = dialog?.querySelector('.wk-settings-panel-heading p') ?? null;
    assert.ok(subtitle, 'the drawer subtitle renders');
    assert.match(subtitle.textContent ?? '', /SSE/, 'the transport label prefixes the chip');
    assert.match(subtitle.textContent ?? '', /已启用/, 'the enabled chip renders');
    const unitText = Array.from(dialog?.querySelectorAll('span.wk-mcp-unit') ?? []).map((node) => node.textContent).join('');
    assert.equal(unitText, '秒次秒', 'advanced inputs show Vue unit suffixes');
    const transportGroup = dialog?.querySelector('[role="radiogroup"][aria-label="传输类型"]');
    assert.ok(transportGroup, 'Vue segmented transport group renders');
    assert.deepEqual(Array.from(transportGroup?.querySelectorAll('[role="radio"]') ?? []).map((button) => button.textContent?.trim()), ['SSE', 'HTTP Streamable']);
    const footerButtons = Array.from(dialog?.querySelectorAll('.wk-mcp-footer button') ?? []).map((button) => button.textContent ?? '');
    assert.ok(footerButtons.some((label) => label.includes('取消')) && footerButtons.some((label) => label.includes('保存并下一步')), 'footer cancel + confirm render');
    assert.ok(footerButtons.findIndex((label) => label.includes('取消')) < footerButtons.findIndex((label) => label.includes('保存并下一步')), 'Vue footer order: cancel before confirm');
    assert.ok(!findButton('测试连接'), 'Vue baseline has no reachable test-connection UI');
    assert.ok(!dialog?.querySelector('section[aria-label="Tools 清单"]'), 'step 0 does not mount the tools panel');
  } finally {
    await unmountEditor(root);
  }
});

test('the owner empty-state add tile opens the Vue add drawer like the admin tile (R428)', async () => {
  const root = await mountEditor(React.createElement(McpSettingsPanel, { client: mcpStubClient(), initialServices: [], role: 'owner' }));
  try {
    assert.doesNotMatch(document.body.textContent ?? '', /暂无 MCP 服务/);
    const addTile = findButton('添加服务');
    assert.ok(addTile, 'owner empty state renders the Vue add-service tile');
    await act(async () => { addTile?.click(); });
    const dialog = document.querySelector('.wks-mcp-drawer');
    assert.ok(dialog, 'clicking the tile opens the Vue add drawer');
    assert.match(dialog?.getAttribute('aria-label') ?? '', /添加/);
  } finally {
    await unmountEditor(root);
  }
});

test('editing a stdio service coerces to SSE exactly like Vue McpServiceDialog.vue:855', async () => {
  const root = await mountEditor(React.createElement(McpSettingsPanel, {
    client: mcpStubClient(),
    initialServices: [{ id: 'stdio-1', name: 'Local', enabled: true, transport_type: 'stdio', is_builtin: false }],
    role: 'admin',
  }));
  try {
    await act(async () => { findButton('编辑')?.click(); });
    const dialog = document.querySelector('.wks-mcp-drawer');
    const transportGroup = dialog?.querySelector('[role="radiogroup"][aria-label="传输类型"]');
    assert.equal(transportGroup?.querySelector('[role="radio"][aria-checked="true"]')?.textContent?.trim(), 'SSE');
    assert.equal(dialog?.textContent?.includes('Stdio') ?? false, false);
  } finally {
    await unmountEditor(root);
  }
});

test('edit mode renders the Vue credential resource card for configured API keys', async () => {
  const root = await mountEditor(React.createElement(McpSettingsPanel, {
    client: mcpStubClient(),
    initialServices: [{ id: 'mcp-credential', name: 'Private Docs', url: 'https://example.com/mcp', enabled: true, transport_type: 'sse', is_builtin: false, auth_config: { auth_type: 'api_key' }, credentials: { api_key: { configured: true } } }],
    role: 'admin',
  }));
  try {
    await act(async () => { findButton('编辑')?.click(); });
    const card = document.querySelector('.wk-mcp-credential-card');
    assert.ok(card, 'configured edit credentials use a dedicated card');
    assert.match(card?.textContent ?? '', /成功/);
    assert.ok(card?.querySelector('input[type="password"]'));
    assert.ok(Array.from(card?.querySelectorAll('button') ?? []).some((button) => (button.textContent ?? '').includes('删除')));
  } finally {
    await unmountEditor(root);
  }
});

test('step 2 gates save on tool sync and shows the Vue usage counter, hint and generate button', async () => {
  const metadataGate = deferred<StubMetadata>();
  const root = await mountEditor(React.createElement(McpSettingsPanel, {
    client: mcpStubClient({ metadataGet: () => metadataGate.promise }),
    initialServices: [],
    role: 'admin',
  }));
  try {
    await act(async () => { findButton('添加服务')?.click(); });
    const nameInput = document.querySelector('input[placeholder="请输入服务名称"]') as HTMLInputElement | null;
    const urlInput = document.querySelector('input[placeholder="https://example.com/mcp"]') as HTMLInputElement | null;
    assert.ok(nameInput && urlInput, 'Vue placeholders render');
    await act(async () => { setInputValue(nameInput, 'Docs'); });
    await act(async () => { setInputValue(urlInput, 'https://example.com/mcp'); });
    const form = document.querySelector('.wks-mcp-drawer form') as HTMLFormElement | null;
    assert.ok(form, 'drawer form renders');
    await act(async () => { submitForm(form); });
    const dialog = document.querySelector('.wks-mcp-drawer');
    const text = dialog?.textContent ?? '';
    assert.match(text, /服务用途/);
    assert.match(text, /模型先读取服务用途/);
    assert.match(text, /0\/16000/);
    assert.match(text, /根据已同步且启用的 Tools 生成精简说明/);
    const saveButton = findButton('保存');
    assert.ok(buttonDisabled('保存'), 'save is gated until tools are synced (Vue confirm-disabled)');
    const generateButton = findButton('AI 生成');
    assert.ok(buttonDisabled('AI 生成'), 'AI generate is gated until tools are synced');
    assert.ok(dialog?.querySelector('section[aria-label="Tools 清单"]'), 'step 2 mounts the metadata panel');
    await act(async () => {
      metadataGate.resolve({ serviceId: 'svc-1', tools: [{ name: 'search', description: 'Search docs' }], serverName: 'Srv', serverVersion: '1.0', instructions: 'Use the documentation tools.', serverDescription: 'Documentation server', syncedAt: '2026-09-13T00:00:00Z', stale: false });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    assert.ok(!buttonDisabled('保存'), 'save unlocks once the tool catalog syncs');
    assert.ok(!buttonDisabled('AI 生成'), 'AI generate unlocks once the tool catalog syncs');
    const docsTrigger = findButton('服务端原始说明');
    assert.ok(docsTrigger, 'server documentation trigger mirrors the Vue metadata popup');
    await act(async () => { docsTrigger?.click(); });
    assert.match(document.body.textContent ?? '', /Use the documentation tools\./);
    const toolDetails = findButton('详情');
    assert.ok(toolDetails, 'tool details trigger renders');
    await act(async () => { toolDetails?.click(); });
    const toolPopup = document.body.querySelector('.wk-mcp-tool-detail-popup');
    assert.ok(toolPopup, 'tool details are attached to document.body like Vue t-popup');
    assert.equal(dialog?.querySelector('.wk-mcp-tool-detail-popup'), null, 'tool details are not trapped in the drawer flow');
    await act(async () => { document.body.dispatchEvent(new dom.window.Event('pointerdown', { bubbles: true })); });
    assert.equal(document.body.querySelector('.wk-mcp-tool-detail-popup'), null, 'outside click closes the tool popup');
  } finally {
    await unmountEditor(root);
  }
});

test('MCP policy updates lock only the tool being saved like the Vue directory', async () => {
  const updateGate = deferred<unknown>();
  const stub = mcpStubClient({
    metadata: {
    serviceId: 'svc-1',
    tools: [{ name: 'search' }, { name: 'write' }],
    serverName: 'Srv',
    serverVersion: '1.0',
    syncedAt: '2026-09-14T00:00:00Z',
    stale: false,
    },
    toolApprovalsUpdate: async () => updateGate.promise,
  });
  const root = await mountEditor(React.createElement(McpSettingsPanel, {
    client: stub,
    initialServices: [],
    role: 'admin',
  }));
  try {
    await act(async () => { findButton('添加服务')?.click(); });
    await act(async () => {
      setInputValue(document.querySelector('input[placeholder="请输入服务名称"]') as HTMLInputElement, 'Docs');
      setInputValue(document.querySelector('input[placeholder="https://example.com/mcp"]') as HTMLInputElement, 'https://example.com/mcp');
    });
    await act(async () => { submitForm(document.querySelector('.wks-mcp-drawer form') as HTMLFormElement); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    const switches = Array.from(document.querySelectorAll('[role="switch"]')) as HTMLButtonElement[];
    assert.equal(switches.length, 4, 'two policy switches per tool');
    await act(async () => { switches[0].click(); });
    assert.equal(switches[0].disabled, true, 'the saving tool is disabled');
    assert.equal(switches[2].disabled, false, 'another tool remains interactive');
    await act(async () => { updateGate.resolve(undefined); await updateGate.promise; });
  } finally {
    await unmountEditor(root);
  }
});

test('step 2 save persists usage_instructions and rejects empty instructions like Vue handleSubmit', async () => {
  const updates: Array<Record<string, unknown>> = [];
  const saved = { id: 'svc-1', name: 'Docs', transport_type: 'sse', enabled: true };
  const stub = {
    configuration: {
      mcp: {
        list: async () => [saved],
        create: async () => saved,
        update: async (_id: string, patch: Record<string, unknown>) => { updates.push(patch); return saved; },
        remove: async () => ({}),
        credentials: { put: async () => ({}) },
        metadata: {
          get: async () => ({ tools: [], serverName: 'S', serverVersion: '1', syncedAt: '2026-09-13T00:00:00Z', stale: false }),
          refresh: async () => ({ tools: [], serverName: 'S', serverVersion: '1', syncedAt: '2026-09-13T00:00:00Z', stale: false }),
        },
        toolApprovals: { list: async () => [], update: async () => ({}) },
        usageInstructions: { generate: async () => 'generated' },
        oauth: {
          status: async () => ({ authorized: false, state: 'reauth_required', refreshAvailable: false }),
          authorizeUrl: async () => ({ authorizationUrl: 'u', authorizationAttempt: 1 }),
          revoke: async () => ({}),
        },
      },
    },
  } as never;
  const root = await mountEditor(React.createElement(McpSettingsPanel, {
    client: stub,
    initialServices: [{ id: 'svc-1', name: 'Docs', transport_type: 'sse', enabled: true, is_builtin: false }],
    role: 'admin',
  }));
  try {
    await act(async () => { findButton('编辑')?.click(); });
    const nameInput = document.querySelector('input[placeholder="请输入服务名称"]') as HTMLInputElement | null;
    const urlInput = document.querySelector('input[placeholder="https://example.com/mcp"]') as HTMLInputElement | null;
    assert.ok(nameInput && urlInput, 'edit draft placeholders render');
    await act(async () => { setInputValue(nameInput, 'Docs'); });
    await act(async () => { setInputValue(urlInput, 'https://example.com/mcp'); });
    const form = document.querySelector('.wks-mcp-drawer form') as HTMLFormElement | null;
    assert.ok(form);
    await act(async () => { submitForm(form); });
    // Metadata resolves immediately; flush it so toolsSynced is settled.
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    // Step 2 with empty usage: submit must be blocked with the Vue warning copy.
    await act(async () => { submitForm(form); });
    assert.match(document.body.textContent ?? '', /使用说明不能为空/);
    const usageTextarea = document.querySelector('.wks-mcp-drawer textarea') as HTMLTextAreaElement | null;
    assert.ok(usageTextarea, 'usage textarea renders in step 2');
    await act(async () => { setInputValue(usageTextarea, 'Use for docs'); });
    assert.match(document.body.textContent ?? '', /12\/16000/);
    await act(async () => { submitForm(form); });
    const usageUpdate = updates.find((patch) => typeof patch.usage_instructions === 'string');
    assert.equal(usageUpdate?.usage_instructions, 'Use for docs');
  } finally {
    await unmountEditor(root);
  }
});
