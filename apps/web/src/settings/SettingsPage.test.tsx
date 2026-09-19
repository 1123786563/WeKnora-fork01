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
const { SettingsPage, settingsNavGroups } = await import('./SettingsPage.tsx');
const { auditDateParts, auditOutcomeTone, auditTargetSummary } = await import('./SystemAuditLogPanel.tsx');

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
  runtime?: TenantRecord;
  runtimeTasks?: TenantRecord[];
  systemSettings?: TenantRecord[];
  apiKeys?: TenantRecord[];
  audit?: TenantRecord[];
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
    administration: {
      runtime: {
        queues: async () => options.runtime ?? { available: true, upstream_concurrency: 4, parse_concurrency: 2, wiki_concurrency: 1, pools: [], queues: [], model_limiter_available: false, models: [], timestamp: 0 },
        tasks: { list: async () => ({ available: true, tasks: options.runtimeTasks ?? [], pageSize: 20, hasMore: false }) },
      },
      settings: {
        list: async () => options.systemSettings ?? [],
        update: async (_key: string, value: unknown) => ({ key: _key, value }),
        reset: async () => undefined,
      },
      apiKeys: {
        list: async () => options.apiKeys ?? [],
        create: async (input: Record<string, unknown>) => ({ id: 1, name: input.name, api_key: 'wk-test', token: 'wk-secret', capabilities: input.capabilities, full_access: false, knowledge_base_ids: null, created_at: '2026-01-01T00:00:00Z' }),
        revoke: async () => undefined,
      },
      auditLog: { list: async () => ({ items: options.audit ?? [], nextCursor: 0 }) },
    },
  } as unknown as WeKnoraClient;
}

async function mountPage(
  client: WeKnoraClient,
  search = '',
  role: 'owner' | 'system-admin' = 'owner',
  capabilities: Record<string, { supported: boolean }> = {},
) {
  if (search) dom.window.history.replaceState(null, '', '/platform/settings' + search);
  const mountHost = document.createElement('div');
  document.body.append(mountHost);
  mountedRoot = createRoot(mountHost);
  await act(async () => {
    mountedRoot?.render(<SettingsPage client={client} tenantId={10000} role={role} capabilities={capabilities} />);
  });
  // The page and section panels are lazy-loaded; allow the first import and
  // its ensuing state update to settle when this file runs in isolation.
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 100)); });
  return document.body;
}

// A1/A4: the Vue drawer has no per-section header refresh and never dumps raw
// payloads (frontend/src/views/settings/Settings.vue section wrappers).
test('settings wrapper drops the refresh button and the raw payload dump', async () => {
  const container = await mountPage(makeClient());
  assert.equal(container.querySelector('.wks-reload'), null, 'no refresh button in the panel heading');
  assert.equal(container.querySelector('dl'), null, 'no raw settings value dump');
});

test('settings drawer is portalled to body like the Vue Teleport shell', async () => {
  await mountPage(makeClient(), '?section=general');
  const drawer = document.body.querySelector('.wk-settings-drawer-root');
  assert.ok(drawer, 'the settings drawer renders');
  assert.equal(drawer.parentElement, document.body, 'the drawer is a direct body child like Vue Teleport');
});

test('settings shell controls expose the shared visible-focus contract', async () => {
  const container = await mountPage(makeClient(), '?section=general');
  const closeButton = container.querySelector('[data-testid="settings-close"]');
  const navigationButton = container.querySelector('.wks-nav-item');
  assert.ok(closeButton?.classList.contains('focus-visible:outline-2'), 'close control uses the shared focus token');
  assert.ok(closeButton?.classList.contains('focus-visible:outline-accent/35'), 'close control uses the shared focus color');
  assert.ok(navigationButton?.classList.contains('focus-visible:outline-2'), 'navigation controls use the shared focus token');
  assert.ok(navigationButton?.classList.contains('focus-visible:outline-accent/35'), 'navigation controls use the shared focus color');
});

test('settings navigation hides the Vue-unlisted retrieval deep-link section', () => {
  const groups = settingsNavGroups('zh-CN', ['general', 'retrieval', 'system']);
  const keys = groups.flatMap((group) => group.items.map((item) => item.key));
  assert.deepEqual(keys, ['general', 'system']);
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
  const tenantInfo = container.querySelector('[data-testid="tenant-info-section"]');
  const deleteZone = container.querySelector('[data-testid="tenant-delete-zone"]');
  assert.ok(tenantInfo && deleteZone, 'tenant info and owner danger zone render');
  assert.equal(Boolean(tenantInfo.compareDocumentPosition(deleteZone) & 4), true, 'danger zone follows tenant information like Vue');
});

test('tenant deletion accepts a space-padded confirmation name like the Vue dialog', async () => {
  const container = await mountPage(makeClient({ tenant: { id: 10000, name: 'Parity 空间', description: '' } }), '?section=tenant');
  const openDelete = Array.from(container.querySelectorAll<HTMLButtonElement>('button'))
    .find((button) => button.textContent === '删除空间');
  assert.ok(openDelete, 'the owner danger-zone action renders');
  await act(async () => openDelete.click());
  await act(async () => {});
  const input = container.querySelector<HTMLInputElement>('[role="dialog"] input');
  assert.ok(input, 'the confirmation dialog input renders');
  await act(async () => {
    Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set?.call(input, '  Parity 空间  ');
    input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  assert.equal(container.querySelector<HTMLButtonElement>('[data-testid="tenant-delete-button"]')?.disabled, false,
    'Vue enables deletion after trim() matches the tenant name');
});

test('an unsupported settings section falls back to general and is absent from the Vue-equivalent navigation', async () => {
  const container = await mountPage(makeClient(), '?section=sandbox', 'owner', { 'settings.sandbox': { supported: false } });
  await act(async () => {});
  assert.ok(container.querySelector('[data-testid="general-preferences-panel"]'), 'unsupported deep links fall back to general');
  assert.equal(Array.from(container.querySelectorAll('.wks-nav-label')).some((node) => node.textContent === '沙箱'), false,
    'unsupported settings sections are not navigable');
  assert.equal(dom.window.location.search, '?section=general', 'the normalized section is reflected in the URL');
});

test('userprofile section shows the Vue rows and localized change-password copy', async () => {
  const container = await mountPage(makeClient(), '?section=userprofile');
  const text = container.textContent ?? '';
  assert.ok(text.includes('用户信息'), 'the Vue userprofile h2 renders');
  assert.ok(text.includes('用户名'), 'the username row renders');
  assert.ok(text.includes('邮箱'), 'the email row renders');
  assert.ok(text.includes('修改密码'), 'the change password row renders');
  // Vue UserProfile: the change-password form is a popup opened by the masked
  // row's edit button — collapsed until clicked.
  assert.equal(text.includes('更新密码'), false, 'the password form stays collapsed until the edit button opens it');
  const editButton = Array.from(container.querySelectorAll<HTMLButtonElement>('.edit-btn'))
    .find((button) => button.getAttribute('aria-label') === '修改密码');
  assert.ok(editButton, 'the masked row exposes an edit pencil');
  await act(async () => editButton?.click());
  const openedText = container.textContent ?? '';
  assert.ok(openedText.includes('更新密码'), 'the popup reveals the Vue submit label');
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

test('ignores a stale section error after navigating to another section', async () => {
  let rejectTenant!: (reason: Error) => void;
  const staleTenant = new Promise<never>((_, reject) => { rejectTenant = reject; });
  const container = await mountPage(makeClient({ tenant: staleTenant }), '?section=tenant');
  const generalButton = Array.from(container.querySelectorAll<HTMLButtonElement>('.wks-nav-item'))
    .find((button) => button.textContent?.includes('常规设置'));
  assert.ok(generalButton, 'the general settings navigation item renders');

  await act(async () => generalButton?.click());
  await act(async () => { rejectTenant(new Error('stale tenant failure')); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 150)); });

  assert.equal(container.textContent?.includes('stale tenant failure'), false, 'a stale request cannot replace the current section state');
  assert.ok(container.querySelector('[data-testid="general-preferences-panel"]'), 'the current section remains mounted');
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
  const originalBack = dom.window.history.back;
  let backCalls = 0;
  dom.window.history.back = () => { backCalls += 1; };
  closeButton.focus();
  assert.equal(document.activeElement, closeButton);
  await act(async () => closeButton.click());
  assert.notEqual(document.activeElement, closeButton, 'closing should blur the old drawer control');
  assert.equal(backCalls, 1, 'ordinary sections should return to the route that opened the drawer');
  dom.window.history.back = originalBack;
});

test('settings drawer owns focus and restores it after the portal closes', async () => {
  const opener = document.createElement('button');
  opener.type = 'button';
  document.body.append(opener);
  opener.focus();

  await mountPage(makeClient(), '?section=general');
  const drawer = document.querySelector<HTMLElement>('.wks-modal');
  const closeButton = document.querySelector<HTMLButtonElement>('[data-testid="settings-close"]');
  assert.ok(drawer && closeButton, 'the dialog and close control render');
  assert.equal(document.activeElement, closeButton, 'opening focuses the first dialog control');

  await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  assert.equal(document.activeElement, opener, 'unmount restores focus to the opener');
  opener.remove();
});

test('runtime queues section renders Vue overview, empty table and limiter states', async () => {
  const container = await mountPage(makeClient({ runtimeTasks: [{ id: 'task-1', queue: 'document', type: 'document:process', state: 'active', last_error: '' }], runtime: {
    available: true, upstream_concurrency: 4, parse_concurrency: 2, wiki_concurrency: 1,
    pools: [{ name: 'upstream', active: 2, instances: 1, cluster_capacity: 4, concurrency: 4, queue_count: 1 }],
    queues: [{ name: 'document', active: 2, pending: 3, retry: 1, archived: 0, completed: 9, scheduled: 0, latency_ms: 120, paused: false }],
    model_limiter_available: true, models: [{ model_id: 'm1', name: 'gpt-test', active: 1, waiting: 2, limit: 4 }], timestamp: 0,
  } }), '?section=runtime-queues', 'system-admin');
  const text = container.textContent ?? '';
  assert.ok(text.includes('运行时队列'), 'the Vue runtime heading renders');
  assert.ok(text.includes('活跃任务'), 'the summary metric renders');
  assert.ok(text.includes('document'), 'the queue row renders');
  assert.ok(text.includes('gpt-test'), 'the model limiter row renders');
  assert.equal(text.includes('尚未移植'), false, 'the generic placeholder is gone');
  const activeButton = container.querySelector<HTMLButtonElement>('.wk-rq-count-button');
  assert.ok(activeButton, 'non-empty task counts are interactive');
  await act(async () => activeButton?.click());
  await act(async () => {});
  assert.ok(container.querySelector('[role="dialog"]'), 'clicking a task count opens the Vue task drawer');
  assert.ok(container.textContent?.includes('document:process'), 'the task drawer renders the loaded task');
});

test('system-global section renders grouped editable settings instead of a generic placeholder', async () => {
  const container = await mountPage(makeClient({ systemSettings: [
    { id: 1, key: 'auth.registration_mode', value: 'open', value_type: 'string', enum: ['open', 'invite_only'], description: '注册方式', is_secret: false, requires_restart: false },
    { id: 2, key: 'sandbox.docker_enabled', value: true, value_type: 'bool', description: 'Docker 沙箱', is_secret: false, requires_restart: true },
  ] }), '?section=system-global', 'system-admin');
  const text = container.textContent ?? '';
  assert.ok(text.includes('系统全局设置'), 'the Vue system settings heading renders');
  assert.ok(text.includes('访问控制'), 'the access tab renders');
  assert.ok(text.includes('注册模式'), 'the localized setting row renders');
  const registrationSelect = container.querySelector<HTMLSelectElement>('select');
  assert.ok(registrationSelect, 'enum settings use a select control');
  registrationSelect.value = 'invite_only';
  await act(async () => registrationSelect.dispatchEvent(new dom.window.Event('change', { bubbles: true })));
  assert.ok(container.querySelector('[role="alertdialog"]'), 'high-risk enum changes require Vue-style confirmation');
  const cancelConfirm = Array.from(container.querySelectorAll<HTMLButtonElement>('[role="alertdialog"] button')).find((button) => button.textContent === '取消');
  assert.ok(cancelConfirm);
  await act(async () => cancelConfirm?.click());
  assert.equal(container.querySelector('[role="alertdialog"]'), null, 'cancelling rolls back the pending high-risk edit');
  const securityTab = Array.from(container.querySelectorAll<HTMLButtonElement>('[role="tab"]')).find((button) => button.textContent === '安全');
  assert.ok(securityTab, 'the security tab renders');
  await act(async () => securityTab?.click());
  assert.ok(container.querySelector('[role="switch"]'), 'boolean settings use the shared switch control');
  assert.equal(text.includes('尚未移植'), false, 'the generic placeholder is gone');
});

test('platform API keys section renders the Vue table and one-time token surface', async () => {
  const container = await mountPage(makeClient({ apiKeys: [{ id: 7, name: 'ops', api_key: 'wk-****', capabilities: ['system_runtime_read'], full_access: false, knowledge_base_ids: null, created_at: '2026-01-01T00:00:00Z' }] }), '?section=platform-api-keys', 'system-admin');
  assert.ok(container.textContent?.includes('平台 API Key'), 'the platform key heading renders');
  assert.ok(container.textContent?.includes('ops'), 'the existing key row renders');
  const name = container.querySelector<HTMLInputElement>('[aria-label="密钥名称"]');
  assert.ok(name);
  await act(async () => { Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set?.call(name, 'new-key'); name.dispatchEvent(new dom.window.Event('input', { bubbles: true })); });
  const capability = container.querySelector<HTMLInputElement>('.wk-api-key-capabilities input');
  assert.ok(capability);
  await act(async () => capability?.click());
  const createButton = Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find((button) => button.textContent === '创建');
  assert.ok(createButton);
  await act(async () => createButton?.click());
  await act(async () => {});
  assert.ok(container.querySelector('[role="alert"]'), 'creation renders the one-time token surface');
});

// R484 G4 D7 (R482 report-B3.md D7): Vue Settings.vue renders ONLY the
// role-denied block when canSeeSection fails (lines 85-94) — the section
// component (and with it the section h2/description) never mounts. The React
// shell must not render its wrapper heading on top of the denied panel.
test('role-denied system sections render only the Vue denied block without the section heading', async () => {
  const container = await mountPage(makeClient(), '?section=system-global', 'owner');
  const text = container.textContent ?? '';
  assert.ok(container.querySelector('[data-testid="role-denied-panel"]'), 'the role-denied panel renders');
  assert.ok(text.includes('权限不足'), 'the Vue denied title renders');
  assert.ok(text.includes('你当前的角色无权访问此设置项'), 'the Vue denied description renders');
  assert.equal(text.includes('系统全局设置'), false, 'the section heading must not render when role-denied');
  assert.equal(text.includes('管理平台级配置'), false, 'the section description must not render when role-denied');

  const queues = await mountPage(makeClient(), '?section=runtime-queues', 'owner');
  const queuesText = queues.textContent ?? '';
  assert.ok(queues.querySelector('[data-testid="role-denied-panel"]'), 'runtime-queues shows the denied panel');
  assert.equal(queuesText.includes('运行时队列'), false, 'runtime-queues heading must not render when role-denied');

  const keys = await mountPage(makeClient(), '?section=platform-api-keys', 'owner');
  const keysText = keys.textContent ?? '';
  assert.ok(keys.querySelector('[data-testid="role-denied-panel"]'), 'platform-api-keys shows the denied panel');
  assert.equal(keysText.includes('平台 API Key'), false, 'platform-api-keys heading must not render when role-denied');
});

test('system audit section renders Vue rows and opens a keyboard-accessible detail drawer', async () => {
  const container = await mountPage(makeClient({ audit: [{ id: 9, created_at: '2026-09-14T10:00:00Z', actor_user_id: 'u-1', actor_role: 'system_admin', action: 'system.setting_changed', target_id: 'auth.registration_mode', outcome: 'success', request_path: '/api/v1/system/admin/settings/auth.registration_mode' }] }), '?section=system-audit-log', 'system-admin');
  assert.ok(container.textContent?.includes('审计日志'), 'the Vue audit heading renders');
  const row = container.querySelector<HTMLTableRowElement>('.wk-audit-table tbody tr');
  assert.ok(row, 'the audit row renders');
  await act(async () => row?.focus());
  await act(async () => row?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
  const detail = document.querySelector('.wk-audit-detail');
  assert.ok(detail, 'Enter opens the audit detail drawer');
  assert.equal(detail.parentElement, document.body, 'the detail drawer is portalled like Vue SettingDrawer');
  assert.ok(container.textContent?.includes('system.setting_changed'), 'the audit detail renders the full record');
  await act(async () => document.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })));
  assert.equal(document.querySelector('.wk-audit-detail'), null, 'Escape closes the audit detail drawer');
});

test('system audit helpers mirror Vue date, outcome and target summaries', () => {
  assert.deepEqual(auditDateParts('2026-09-14T08:09:10.000Z', 'zh-CN'), { date: '2026/09/14', time: '16:09:10' });
  assert.equal(auditOutcomeTone('denied'), 'danger');
  assert.equal(auditOutcomeTone('success'), 'success');
  assert.equal(auditOutcomeTone('accepted'), 'default');
  assert.deepEqual(auditTargetSummary({ action: 'system.setting_changed', target_type: 'setting', details: { key: 'auth.registration_mode', before: 'open', after: 'invite' } }), { key: 'auth.registration_mode', diff: 'open → invite' });
  assert.deepEqual(auditTargetSummary({ action: 'system.queue_task_retried', target_id: 'task-1', details: { queue: 'default', task_id: 'task-1' } }), { key: 'default:task-1', diff: '' });
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

test('the Vue knowledgeqa settings entry preselects the chat model tab', async () => {
  const container = await mountPage(makeClient({ models: [
    { id: 'm1', name: 'gpt-test', type: 'KnowledgeQA', source: 'remote', parameters: {} },
  ] }), '?section=models&subsection=knowledgeqa');
  const tabs = container.querySelector('.wk-model-tabs');
  assert.ok(tabs, 'the model type tabs render');
  const active = tabs.querySelector('.is-active');
  assert.ok(active);
  assert.ok((active.textContent ?? '').includes('对话(1)'), 'the chat tab is preselected');
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
