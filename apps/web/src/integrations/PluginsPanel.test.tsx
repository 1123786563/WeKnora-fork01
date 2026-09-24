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
