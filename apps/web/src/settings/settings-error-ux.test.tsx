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
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { SettingsPage } = await import('./SettingsPage.tsx');

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
 * Vue 三种模式锚定：
 * - models        Toast「加载模型列表失败」(本地化) + 界面保持（ModelSettings.vue:488-490）
 * - skills        Toast(后端原文优先,本地化兜底) + 中央空态 + 重试（SkillSettings.vue:1161-1164）
 * - mcp           Toast「加载 MCP 服务列表失败」(纯本地化) + 中央空态 + 重试（McpSettings.vue:144-147）
 * - members       浅红横幅(透传后端原文) + 重试（TenantMembers.vue:838-841 / 313-318）
 * - storage       浅红横幅(透传后端原文) + 重试（StorageEngineSettings.vue:920-921 / 15-19）
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
      profile: { get: async () => ({ id: 'u-1', username: 'parity' }) },
      system: { info: async () => ({ version: 'unknown' }) },
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

test('storage: inline banner passes the backend message through and retries the same request (Vue StorageEngineSettings.vue mode)', async () => {
  const { client, calls } = failingClient('storage');
  const container = await mountPage(client, '?section=storage');
  await settle();
  assert.ok(headingTexts(container).includes('存储引擎'), 'the storage section h2 keeps rendering on load failure');
  const banner = findBanner(container);
  assert.ok(banner?.includes(UPSTREAM_FAILURE), 'an inline banner shows the backend error text');
  const retry = findRetryButton(container, '重试');
  assert.ok(retry, 'the banner offers a retry button');
  const listCallsBefore = calls['storage.list'] ?? 0;
  await act(async () => { retry!.click(); });
  await settle();
  assert.ok((calls['storage.list'] ?? 0) > listCallsBefore, 'retry re-sends the same storage backends request');
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
