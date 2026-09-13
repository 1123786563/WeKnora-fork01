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

type TenantRecord = Record<string, unknown>;

function makeClient(options: {
  tenant?: TenantRecord | Promise<never>;
  system?: TenantRecord;
  chathistory?: { config: TenantRecord; stats?: TenantRecord | null };
  models?: unknown[];
} = {}) {
  const tenantGet = options.tenant instanceof Promise
    ? () => options.tenant as Promise<never>
    : async () => options.tenant ?? { id: 10000, name: 'Parity 空间', description: '', status: 'active', created_at: '2026-01-01T00:00:00Z' };
  return {
    auth: {
      registrationConfig: async () => ({ complexPasswordEnabled: false }),
    },
    settings: {
      preferences: { get: async () => ({}) },
      tenant: {
        get: tenantGet,
        update: async (_id: number, body: Record<string, unknown>) => ({ id: 10000, ...body, status: 'active', created_at: '2026-01-01T00:00:00Z' }),
      },
      profile: { get: async () => ({ id: 'u-1', username: 'parity', email: 'parity-test@local.dev', created_at: '2026-01-01T00:00:00Z' }) },
      system: { info: async () => options.system ?? { version: 'unknown' } },
      chatHistory: { config: { get: async () => options.chathistory?.config ?? { enabled: false, embedding_model_id: '' }, update: async () => options.chathistory?.config ?? {} }, stats: async () => options.chathistory?.stats ?? null },
    },
    configuration: {
      models: { list: async () => options.models ?? [] },
    },
  } as unknown as WeKnoraClient;
}

async function mountPage(client: WeKnoraClient, search = '') {
  if (search) dom.window.history.replaceState(null, '', '/platform/settings' + search);
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<SettingsPage client={client} tenantId={10000} role="owner" />);
  });
  await act(async () => {});
  return container;
}

// A1/A4: the Vue drawer has no per-section header refresh and never dumps raw
// payloads (frontend/src/views/settings/Settings.vue section wrappers).
test('settings wrapper drops the refresh button and the raw payload dump', async () => {
  const container = await mountPage(makeClient());
  assert.equal(container.querySelector('.wks-reload'), null, 'no refresh button in the panel heading');
  assert.equal(container.querySelector('dl.wk-settings-values'), null, 'no raw settings value dump');
});

// A3: the general section mounts the Vue GeneralSettings parity panel.
test('general section mounts the preferences panel with the Vue copy', async () => {
  const container = await mountPage(makeClient(), '?section=general');
  assert.ok(container.querySelector('[data-testid="general-preferences-panel"]'), 'the general preferences panel is mounted');
  const headings = Array.from(container.querySelectorAll('h2')).map((node) => node.textContent ?? '');
  assert.ok(headings.includes('常规设置'), 'the Vue h2 renders');
  assert.ok(container.textContent?.includes('配置语言、外观等基础选项'), 'the Vue subtitle renders');
  assert.ok(container.textContent?.includes('界面字体'), 'the UI font field renders');
  assert.ok(container.textContent?.includes('代码字体'), 'the code font field renders');
  assert.ok(container.textContent?.includes('示例 Sample 字体 Font — Aa Gg Oo 0123'), 'the UI font preview renders');
  assert.ok(container.textContent?.includes('字体大小'), 'the font size field renders');
});

// A5: tenant + userprofile forms use the shared Vue i18n copy.
test('tenant section shows the Vue info rows instead of the English form', async () => {
  const container = await mountPage(makeClient(), '?section=tenant');
  const text = container.textContent ?? '';
  assert.ok(text.includes('空间信息'), 'the Vue tenant h2 renders');
  assert.ok(text.includes('空间名称'), 'the Vue name row label renders');
  assert.ok(text.includes('空间描述'), 'the Vue description row label renders');
  assert.ok(text.includes('空间 ID'), 'the Vue id row label renders');
  assert.ok(text.includes('10000'), 'the tenant id renders');
  assert.equal(text.includes('Save tenant information'), false, 'no English form copy');
  assert.equal(text.includes('Name'), false, 'no English field labels');
});

test('userprofile section shows the Vue rows and localized change-password copy', async () => {
  const container = await mountPage(makeClient(), '?section=userprofile');
  const text = container.textContent ?? '';
  assert.ok(text.includes('用户信息'), 'the Vue userprofile h2 renders');
  assert.ok(text.includes('用户名'), 'the username row renders');
  assert.ok(text.includes('邮箱'), 'the email row renders');
  assert.ok(text.includes('修改密码'), 'the change password row renders');
  assert.ok(text.includes('更新密码'), 'the Vue submit label renders');
  assert.equal(text.includes('Change password'), false, 'no English button copy');
  assert.equal(text.includes('Profile identity fields are server-owned'), false, 'no English help copy');
});

// A2: loading copy is localized shared copy without the API domain.
test('loading state uses localized shared copy without leaking the API domain', async () => {
  const container = await mountPage(makeClient({ tenant: new Promise(() => {}) }), '?section=tenant');
  const text = container.textContent ?? '';
  assert.ok(text.includes('加载中'), 'the localized loading copy renders');
  assert.equal(text.includes('Loading from'), false, 'no English loading copy');
  assert.equal(text.includes('configuration'), false, 'no API domain leak');
});

test('settings navigation clears focus after switching sections like the Vue drawer', async () => {
  const container = await mountPage(makeClient(), '?section=general');
  const navigationButton = Array.from(container.querySelectorAll<HTMLButtonElement>('.wks-nav-item'))
    .find((button) => button.getAttribute('aria-current') !== 'page');
  assert.ok(navigationButton, 'a second settings section is available');
  await act(async () => navigationButton?.click());
  assert.notEqual(document.activeElement, navigationButton, 'section navigation should not retain focus on the old drawer control');
});

test('settings close blurs the focused control before leaving like the Vue drawer', async () => {
  const container = await mountPage(makeClient(), '?section=general');
  const closeButton = container.querySelector<HTMLButtonElement>('[data-testid="settings-close"]');
  assert.ok(closeButton, 'the settings close button renders');
  closeButton.focus();
  assert.equal(document.activeElement, closeButton);
  await act(async () => closeButton.click());
  assert.notEqual(document.activeElement, closeButton, 'closing should blur the old drawer control');
});

// B4d: section=subsection deep link reaches the model panel type tabs.
test('a subsection query param preselects the model type tab', async () => {
  const container = await mountPage(makeClient({ models: [
    { id: 'm1', name: 'bge-m3', type: 'Embedding', source: 'remote', parameters: {} },
  ] }), '?section=models&subsection=embedding');
  const tabs = container.querySelector('.wk-model-tabs');
  assert.ok(tabs, 'the model type tabs render');
  const active = tabs.querySelector('.is-active');
  assert.ok(active);
  assert.ok((active.textContent ?? '').includes('Embedding(1)'), 'the embedding tab is preselected');
});

// Item D: the system section renders the Vue SystemInfo display list instead
// of the English read-note fallback (frontend/src/views/settings/SystemInfo.vue).
test('system section renders the Vue localized display rows', async () => {
  const container = await mountPage(makeClient({ system: {
    version: 'v1.0.0', edition: 'standard', commit_id: 'abc1234', build_time: '2026-09-01 12:00:00',
    go_version: 'go1.24.0', started_at: '2026-09-13T08:00:00Z', uptime_seconds: 3810,
    db_version: '20260901', keyword_index_engine: 'elasticsearch', vector_store_engine: 'elasticsearch', graph_database_engine: '',
  } }), '?section=system');
  const text = container.textContent ?? '';
  assert.ok(text.includes('系统信息'), 'the Vue system h2 renders');
  assert.ok(text.includes('查看系统版本信息和用户账户配置'), 'the Vue description renders');
  assert.ok(text.includes('应用版本'), 'the app version row renders');
  assert.ok(text.includes('UI 版本'), 'the UI version row renders');
  assert.ok(text.includes('运行时长'), 'the uptime row renders');
  assert.ok(/[0-9]+ (小时|分钟|天)/.test(text) || /[0-9]+ 秒/.test(text), 'the uptime is humanized');
  assert.ok(text.includes('数据库版本'), 'the db version row renders');
  assert.ok(text.includes('关键词索引引擎'), 'the keyword engine row renders');
  assert.ok(text.includes('向量库引擎') || text.includes('向量存储引擎'), 'the vector engine row renders');
  assert.ok(text.includes('图数据库引擎'), 'the graph engine row renders');
  assert.ok(text.includes('Standard'), 'the edition badge renders');
  assert.equal(text.includes('Read result received from the server'), false, 'no English read-note fallback');
});


// Item F: the chathistory section renders the Vue rows, the localized stats
// panel and no English notes (frontend/src/views/settings/ChathistorySettings.vue).
test('chathistory section renders Vue rows, stats panel and no English notes', async () => {
  const container = await mountPage(makeClient({ chathistory: { config: { enabled: false, embedding_model_id: '' }, stats: null } }), '?section=chathistory');
  const text = container.textContent ?? "";
  assert.ok(text.includes('消息管理'), 'the Vue chathistory h2 renders');
  assert.ok(text.includes('配置聊天历史知识库，将对话消息自动向量化索引，实现语义搜索'), 'the Vue subtitle renders');
  assert.ok(text.includes('启用消息索引'), 'the enable row renders');
  assert.ok(text.includes('开启后，新的对话消息将自动索引到知识库，支持向量搜索'), 'the enable hint renders under the label');
  assert.ok(text.includes('索引统计'), 'the stats section renders');
  assert.ok(text.includes('消息索引未配置'), 'the empty-stats title renders');
  assert.ok(text.includes('启用并选择 Embedding 模型后，对话消息将自动向量化索引'), 'the empty-stats hint renders');
  assert.equal(text.includes('The server owns the generated knowledge-base identity'), false, 'no English stats note');
});
