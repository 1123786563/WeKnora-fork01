import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Event: dom.window.Event,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { SettingsPage, sectionErrorMode } = await import('./SettingsPage.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.history.replaceState(null, '', '/platform/settings');
  dom.window.localStorage.clear();
});

const UPSTREAM_FAILURE = 'simulated upstream failure';

/**
 * R472 A2 — settings 分区错误态 UX 对齐（.omc/state/r470/report-A3.md）。
 * R481 A1 — 按 R480 浏览器锚定基线修订
 * （docs/migrations/react/evidence/vue-react-parity/2026-09-19-r480-settings-error-anchoring.md）。
 * Vue 模式锚定：
 * - models        Toast「加载模型列表失败」(本地化) + 界面保持（ModelSettings.vue:488-490）
 * - skills        Toast(后端原文优先,本地化兜底) + 中央空态 + 重试（SkillSettings.vue:1161-1164）
 * - mcp           Toast「加载 MCP 服务列表失败」(纯本地化) + 中央空态 + 重试（McpSettings.vue:144-147）
 * - members       浅红横幅(透传后端原文) + 重试（TenantMembers.vue:838-841 / 313-318，面板自渲染）
 * - parser/system/userprofile 壳层横幅(透传后端原文) + 重试替换内容区
 *                  （ParserEngineSettings.vue:14-21 / SystemInfo.vue:13-20 / UserProfile.vue:14-21）
 * - storage/vectorstore/websearch/weknoracloud/ollama/retrieval 完全静默降级：
 *                  空列表/默认表单，无横幅、无 toast、无重试（R480：Vue 500 下 errHit=false）
 * 所有分区失败态下面板标题继续渲染（R470 缺陷 4：React 连标题都不渲染）。
 */

function failingClient(section: string): { client: WeKnoraClient; calls: Record<string, number> } {
  const calls: Record<string, number> = {};
  const fail = (name: string) => {
    calls[name] = (calls[name] ?? 0) + 1;
    return Promise.reject(new Error(UPSTREAM_FAILURE));
  };
  const succeed = <T,>(name: string, value: T) => {
    calls[name] = (calls[name] ?? 0) + 1;
    return Promise.resolve(value);
  };
  const base = {
    auth: {
      registrationConfig: async () => ({ complexPasswordEnabled: false }),
      me: async () => ({ user: { id: 'u-1' } }),
    },
    settings: {
      preferences: { get: async () => ({}) },
      tenant: { get: async () => ({ id: 10000, name: 'Parity 空间' }) },
      profile: { get: () => (section === 'userprofile' ? fail('profile.get') : succeed('profile.get', { id: 'u-1', username: 'parity' })) },
      system: { info: () => (section === 'system' ? fail('system.info') : succeed('system.info', { version: 'unknown' })) },
      parser: {
        engines: () => (section === 'parser' ? fail('parser.engines') : succeed('parser.engines', [])),
        config: { get: () => (section === 'parser' ? fail('parser.config') : succeed('parser.config', {})) },
      },
      retrieval: { get: () => (section === 'retrieval' ? fail('retrieval.get') : succeed('retrieval.get', {})) },
      vectorStores: { list: () => (section === 'vectorstore' ? fail('vectorStores.list') : succeed('vectorStores.list', [])) },
      webSearch: { providers: { list: () => (section === 'websearch' ? fail('webSearch.list') : succeed('webSearch.list', [])) } },
      weknoraCloud: { status: () => (section === 'weknoracloud' ? fail('weknoraCloud.status') : succeed('weknoraCloud.status', {})) },
      ollama: {
        status: () => (section === 'ollama' ? fail('ollama.status') : succeed('ollama.status', {})),
        models: () => (section === 'ollama' ? fail('ollama.models') : succeed('ollama.models', [])),
      },
      storage: {
        backends: { list: () => (section === 'storage' ? fail('storage.list') : succeed('storage.list', [])) },
        legacy: { status: () => (section === 'storage' ? fail('storage.legacy') : succeed('storage.legacy', {})) },
      },
    },
    configuration: {
      models: { list: () => (section === 'models' ? fail('models.list') : succeed('models.list', [])) },
      skills: {
        list: () => (section === 'skills' ? fail('skills.list') : succeed('skills.list', [])),
        catalog: { list: () => (section === 'skills' ? fail('skills.catalog') : succeed('skills.catalog', [])) },
      },
      mcp: { list: () => (section === 'mcp' ? fail('mcp.list') : succeed('mcp.list', [])) },
    },
    sandboxConfigurations: {
      list: () => (section === 'skills' ? fail('skills.sandboxConfigs') : succeed('skills.sandboxConfigs', { items: [] })),
    },
    identity: {
      tenants: {
        members: { list: () => (section === 'members' ? fail('members.list') : succeed('members.list', { items: [], total: 0 })) },
        invitations: { listTenant: async () => ({ items: [], total: 0 }) },
      },
    },
  };
  return { client: base as unknown as WeKnoraClient, calls };
}

async function mountPage(client: WeKnoraClient, search: string) {
  dom.window.history.replaceState(null, '', '/platform/settings' + search);
  const mountHost = document.createElement('div');
  document.body.append(mountHost);
  mountedRoot = createRoot(mountHost);
  await act(async () => {
    mountedRoot?.render(<SettingsPage client={client} tenantId={10000} role="owner" />);
  });
  // Lazy panels need one macrotask for the dynamic import + state settle.
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 100)); });
  return document.body;
}

async function settle() {
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
}

function headingTexts(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll('h2')).map((node) => node.textContent ?? '');
}

function findToast(container: HTMLElement): string | null {
  return container.querySelector('[data-testid="settings-toast"]')?.textContent ?? null;
}

function findBanner(container: HTMLElement): string | null {
  return container.querySelector('[data-testid="settings-section-error-banner"]')?.textContent ?? null;
}

function findRetryButton(container: HTMLElement, expectLabel: string): HTMLButtonElement | null {
  const buttons = Array.from(container.querySelectorAll('button'));
  return buttons.find((button) => (button.textContent ?? '').trim() === expectLabel) ?? null;
}

test('models: localized load-failed toast + panel keeps rendering (Vue ModelSettings.vue toast mode)', async () => {
  const { client } = failingClient('models');
  const container = await mountPage(client, '?section=models');
  await settle();
  // Toast 本地化文案，不透传后端英文原文。
  assert.equal(findToast(container), '加载模型列表失败');
  // 面板骨架保持：模型配置标题 + 默认空态仍渲染，无裸错误文本。
  assert.ok(headingTexts(container).includes('模型配置'), 'the models panel h2 keeps rendering on load failure');
  assert.ok(!container.textContent?.includes(UPSTREAM_FAILURE), 'no raw backend error text replaces the panel');
});

test('skills: toast + central empty state + retry re-sends the same load (Vue SkillSettings.vue mode)', async () => {
  const { client, calls } = failingClient('skills');
  const container = await mountPage(client, '?section=skills');
  await settle();
  // Vue: MessagePlugin.error(e?.message || t('settings.skills.loadFailed')) — 后端原文优先。
  assert.equal(findToast(container), UPSTREAM_FAILURE);
  assert.ok(headingTexts(container).includes('技能管理'), 'the skills panel h2 keeps rendering on load failure');
  // 中央空态 + 重试按钮。
  const before = container.querySelector('[data-testid="skill-settings"] .wk-empty, [data-testid="skill-settings"] [data-testid="settings-load-empty"]');
  assert.ok(before, 'a central empty state replaces the skill list');
  const retry = findRetryButton(container, '重试');
  assert.ok(retry, 'the error empty state offers a retry button');
  const catalogCallsBefore = calls['skills.catalog'] ?? 0;
  await act(async () => { retry!.click(); });
  await settle();
  assert.ok((calls['skills.catalog'] ?? 0) > catalogCallsBefore, 'retry re-sends the same catalog request');
});

test('mcp: localized toast + central empty state + retry re-sends the same load (Vue McpSettings.vue mode)', async () => {
  const { client, calls } = failingClient('mcp');
  const container = await mountPage(client, '?section=mcp');
  await settle();
  // Vue: MessagePlugin.error(t('mcpSettings.toasts.loadFailed')) — 纯本地化。
  assert.equal(findToast(container), '加载 MCP 服务列表失败');
  assert.ok(headingTexts(container).includes('MCP 服务管理'), 'the mcp panel h2 keeps rendering on load failure');
  const retry = findRetryButton(container, '重试');
  assert.ok(retry, 'the error empty state offers a retry button');
  const listCallsBefore = calls['mcp.list'] ?? 0;
  await act(async () => { retry!.click(); });
  await settle();
  assert.ok((calls['mcp.list'] ?? 0) > listCallsBefore, 'retry re-sends the same mcp list request');
});

test('members: inline banner passes the backend message through and retries the same request (Vue TenantMembers.vue mode)', async () => {
  const { client, calls } = failingClient('members');
  const container = await mountPage(client, '?section=members');
  await settle();
  assert.ok(headingTexts(container).includes('成员管理'), 'the members panel h2 keeps rendering on load failure');
  const banner = findBanner(container) ?? container.querySelector('[data-testid="tenant-members-error"]')?.textContent ?? null;
  assert.ok(banner?.includes(UPSTREAM_FAILURE), 'an inline banner shows the backend error text');
  const retry = findRetryButton(container, '重试');
  assert.ok(retry, 'the banner offers a retry button');
  const listCallsBefore = calls['members.list'] ?? 0;
  await act(async () => { retry!.click(); });
  await settle();
  assert.ok((calls['members.list'] ?? 0) > listCallsBefore, 'retry re-sends the same members list request');
});

// R481 A1 — 模式映射红测试：R480 浏览器锚定把 storage 从 banner-retry 改判为
// 静默（Vue StorageBackendSettings.vue 500 下空列表+添加按钮、零错误 UI），
// parser/system/userprofile 升级为壳层横幅+重试（此前缺重试按钮的裸 inline）。
test('sectionErrorMode maps the R480 baseline: silent resource group + banner-retry parser/system/userprofile', () => {
  for (const key of ['storage', 'vectorstore', 'websearch', 'weknoracloud', 'ollama', 'retrieval']) {
    assert.equal(sectionErrorMode(key), 'silent', `${key} degrades silently like the Vue baseline`);
  }
  for (const key of ['members', 'parser', 'system', 'userprofile']) {
    assert.equal(sectionErrorMode(key), 'banner-retry', `${key} keeps the banner + retry mode`);
  }
  assert.equal(sectionErrorMode('models'), 'toast-keep');
  assert.equal(sectionErrorMode('skills'), 'toast-retry');
  assert.equal(sectionErrorMode('mcp'), 'toast-retry');
  assert.equal(sectionErrorMode('general'), 'inline');
});

test('storage: load failure degrades silently to the empty list + add button (Vue StorageBackendSettings.vue mode)', async () => {
  const { client } = failingClient('storage');
  const container = await mountPage(client, '?section=storage');
  await settle();
  assert.ok(headingTexts(container).includes('存储引擎'), 'the storage section h2 keeps rendering on load failure');
  assert.equal(findBanner(container), null, 'no error banner replaces the panel');
  assert.equal(findToast(container), null, 'no toast fires for the silent degradation');
  assert.ok(!container.textContent?.includes(UPSTREAM_FAILURE), 'no raw backend error text leaks');
  // R480 探针：Vue 页面文本止于「添加存储实例」（空列表 + 添加按钮）。
  assert.ok(container.textContent?.includes('尚未配置存储实例'), 'the empty-list state renders');
  assert.ok(container.textContent?.includes('添加存储实例'), 'the add-instance button renders');
});

for (const [section, heading, panelSelector] of [
  ['vectorstore', '向量数据库引擎', '.wk-settings-resource'],
  ['websearch', '网络搜索配置', '.wk-settings-resource'],
  ['weknoracloud', 'WeKnora Cloud', '.weknoracloud-settings'],
  ['ollama', 'Ollama 配置', '.ollama-settings'],
  ['retrieval', '搜索设置', 'form.wk-settings-editor'],
] as const) {
  test(`${section}: load failure degrades silently with the panel rendering on a null payload (R480 Vue baseline)`, async () => {
    const { client } = failingClient(section);
    const container = await mountPage(client, `?section=${section}`);
    await settle();
    assert.ok(headingTexts(container).some((text) => text.includes(heading)), `the ${section} h2 keeps rendering on load failure`);
    assert.equal(findBanner(container), null, 'no shell error banner renders');
    assert.equal(findToast(container), null, 'no toast fires for the silent degradation');
    assert.ok(!container.textContent?.includes(UPSTREAM_FAILURE), 'no raw backend error text leaks');
    assert.ok(container.querySelector(panelSelector), 'the panel itself renders its empty/default state');
  });
}

test('parser: load failure degrades silently with the self-loading panel rendering (R481 architecture)', async () => {
  const { client } = failingClient('parser');
  const container = await mountPage(client, '?section=parser');
  await settle();
  assert.ok(headingTexts(container).includes('解析引擎'), 'the parser section h2 keeps rendering on load failure');
  // R481 之后 parser 与 retrieval 同构：壳层不再做分区读取（case 'parser'
  // 返回 null），ConfigSettingsPanel 以自加载 + 行内错误态渲染；壳层横幅
  // 只保留给 system/userprofile/members。
  assert.equal(findBanner(container), null, 'no shell error banner renders for the self-loading parser panel');
  assert.equal(findToast(container), null, 'no toast fires for the silent degradation');
  // 面板自身把 load 失败透传为行内错误态（R021 口径）：后端原文出现在
  // 面板内容区，壳层横幅/吐司均不介入。
  assert.ok(container.textContent?.includes(UPSTREAM_FAILURE), 'the self-loading parser panel surfaces the backend error inline');
});

test('system: banner passes the backend message through and retries the same request (Vue SystemInfo.vue mode)', async () => {
  const { client, calls } = failingClient('system');
  const container = await mountPage(client, '?section=system');
  await settle();
  assert.ok(headingTexts(container).includes('系统信息'), 'the system section h2 keeps rendering on load failure');
  const banner = findBanner(container);
  assert.ok(banner?.includes(UPSTREAM_FAILURE), 'the shell banner shows the backend error text');
  assert.equal(container.querySelector('[data-testid="system-info-panel"]'), null, 'the banner replaces the panel content');
  const retry = findRetryButton(container, '重试');
  assert.ok(retry, 'the banner offers a retry button');
  const before = calls['system.info'] ?? 0;
  await act(async () => { retry!.click(); });
  await settle();
  assert.ok((calls['system.info'] ?? 0) > before, 'retry re-sends the same system info request');
});

test('userprofile: the panel error alert passes the backend message through and retries (Vue UserProfile.vue error-inline mode)', async () => {
  const { client, calls } = failingClient('userprofile');
  const container = await mountPage(client, '?section=userprofile');
  await settle();
  // T12a：userprofile 面板自持 Vue error 态（UserProfile.vue :15-21
  // error-inline t-alert theme=error + 重试），壳层横幅不再顶替。
  assert.ok(headingTexts(container).includes('用户信息'), 'the userprofile section h2 keeps rendering on load failure');
  assert.ok(container.querySelector('[data-testid="user-profile-section"] .error-inline'), 'the panel renders its own error-inline block');
  assert.ok(container.textContent?.includes(UPSTREAM_FAILURE), 'the panel alert shows the backend error text');
  assert.equal(findBanner(container), null, 'no shell error banner for the self-erroring userprofile panel');
  const retry = findRetryButton(container, '重试');
  assert.ok(retry, 'the panel alert offers a retry button');
  const before = calls['profile.get'] ?? 0;
  await act(async () => { retry!.click(); });
  await settle();
  assert.ok((calls['profile.get'] ?? 0) > before, 'retry re-sends the same profile request');
});

test('toast auto-dismisses like the Vue MessagePlugin default 3s window', async () => {
  const { pushSettingsToast, SETTINGS_TOAST_DURATION_MS } = await import('./settings-toast.tsx');
  const { SettingsToastHost } = await import('./settings-toast.tsx');
  assert.equal(SETTINGS_TOAST_DURATION_MS, 3000);
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host);
  await act(async () => { root.render(<SettingsToastHost />); });
  await act(async () => { pushSettingsToast('加载模型列表失败'); });
  assert.equal(host.querySelector('[data-testid="settings-toast"]')?.textContent, '加载模型列表失败');
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 3200)); });
  assert.equal(host.querySelector('[data-testid="settings-toast"]'), null, 'the toast auto-dismisses after 3s');
  await act(async () => { root.unmount(); });
});
