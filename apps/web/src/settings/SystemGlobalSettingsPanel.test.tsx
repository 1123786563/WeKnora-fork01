import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import type { Root } from 'react-dom/client';
import type { SystemSetting, WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => void }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { act } = await import('react');
const { SystemGlobalSettingsPanel } = await import('./SystemGlobalSettingsPanel.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.clear();
});

type SettingFixture = Record<string, unknown>;

// Full 17-key registry fixture (the 19-row smoke list minus two virtual
// rows the backend merges) — counts per Vue SystemSettings.vue sections.
function fullSettings(overrides: Record<string, Partial<SettingFixture>> = {}): SystemSetting[] {
  const base: Array<[string, unknown, SystemSetting['value_type'], Partial<SettingFixture>]> = [
    ['auth.registration_mode', 'invite_only', 'string', { enum: ['self_serve', 'invite_only'], last_modified_by: 'u-abc12345-6', last_modified_by_name: 'parity-admin', updated_at: '2026-09-01T08:00:00Z' }],
    ['auth.complex_password_enabled', true, 'bool', {}],
    ['auth.default_tenant_mode', 'create_personal', 'string', { enum: ['create_personal', 'tenantless'] }],
    ['tenant.self_service_creation_enabled', true, 'bool', {}],
    ['tenant.max_owned_per_user', 10, 'int', {}],
    ['tenant.default_storage_quota_gb', 10, 'int', { last_modified_by: 'u-abc12345-6', updated_at: '2026-09-01T08:00:00Z' }],
    ['tenant.auto_create_api_key', false, 'bool', {}],
    ['tenant.auto_accept_invitation', false, 'bool', {}],
    ['asynq.core_concurrency', 4, 'int', {}],
    ['asynq.enrichment_concurrency', 4, 'int', {}],
    ['asynq.postprocess_concurrency', 2, 'int', {}],
    ['asynq.maintenance_concurrency', 2, 'int', {}],
    ['asynq.shared_concurrency', 2, 'int', {}],
    ['asynq.wiki_concurrency', 1, 'int', {}],
    ['model.max_concurrency', 0, 'int', {}],
    ['ssrf.whitelist', ['10.0.0.0/8'], 'string_list', {}],
    ['sandbox.docker_enabled', false, 'bool', { requires_restart: false }],
  ];
  return base.map(([key, value, value_type, extra], index) => ({
    id: index + 1, key, value, value_type, category: 'system', description: `${key} backend description`,
    is_secret: false, requires_restart: false, last_modified_by: '', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
    ...extra, ...(overrides[key] ?? {}),
  })) as SystemSetting[];
}

function makeClient(options: {
  settings?: SystemSetting[];
  admins?: Array<Record<string, unknown>>;
  profile?: Record<string, unknown>;
  calls?: { promote: Array<unknown>; revoke: Array<unknown>; resetPassword: Array<unknown>; createUser: Array<unknown>; update: Array<[string, unknown]>; reset: Array<string>; bulk: number };
} = {}) {
  const calls = options.calls ?? { promote: [], revoke: [], resetPassword: [], createUser: [], update: [], reset: [], bulk: 0 };
  return {
    auth: { registrationConfig: async () => ({ complexPasswordEnabled: false }) },
    settings: { profile: { get: async () => options.profile ?? { id: 'u-self' } } },
    administration: {
      admins: {
        list: async () => ({ items: options.admins ?? [], total: (options.admins ?? []).length }),
        promote: async (input: unknown) => { calls.promote.push(input); return { id: 'u-2', username: 'x', email: '', is_active: true, is_system_admin: true, created_at: '', updated_at: '' }; },
        revoke: async (userId: string) => { calls.revoke.push(userId); return { id: userId, username: 'x', email: '', is_active: true, is_system_admin: false, created_at: '', updated_at: '' }; },
        resetPassword: async (input: unknown) => { calls.resetPassword.push(input); return { message: 'ok' }; },
        createUser: async (input: unknown) => { calls.createUser.push(input); return { user: { id: 'u-3', username: '', email: '', is_active: true, is_system_admin: false, created_at: '', updated_at: '' }, generatedPassword: 'Gen-Passw0rd' }; },
      },
      settings: {
        list: async () => options.settings ?? [],
        update: async (key: string, value: unknown) => { calls.update.push([key, value]); const found = (options.settings ?? []).find((item) => item.key === key); return { ...(found ?? { id: 0, key, value_type: 'string', category: '', description: '', is_secret: false, requires_restart: false, last_modified_by: 'u-editor', created_at: '', updated_at: '' }), key, value, last_modified_by: 'u-editor' }; },
        reset: async (key: string) => { calls.reset.push(key); },
        applyDefaultStorageQuota: async () => { calls.bulk += 1; return { affected: 3, quotaBytes: 32212254720, quotaGb: 30 }; },
      },
    },
  } as unknown as WeKnoraClient;
}

async function mountPanel(client: WeKnoraClient, settings: SystemSetting[]) {
  const host = document.createElement('div');
  document.body.append(host);
  mountedRoot = createRoot(host);
  await act(async () => { mountedRoot?.render(React.createElement(SystemGlobalSettingsPanel, { client, initialSettings: settings })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 50)); });
  return document.body;
}

function tabButton(container: ParentNode, label: string) {
  return Array.from(container.querySelectorAll<HTMLButtonElement>('[role="tab"]')).find((button) => button.textContent === label);
}
function setInputValue(input: HTMLInputElement, value: string) {
  Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set?.call(input, value);
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
}
async function clickButton(container: ParentNode, text: string) {
  const button = Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find((candidate) => candidate.textContent === text);
  assert.ok(button, `button "${text}" must render`);
  await act(async () => { button.click(); });
  return button;
}

test('renders the Vue section shape: counts, descriptions, labels, badges and modified-by meta', async () => {
  const container = await mountPanel(makeClient({ settings: fullSettings() }), fullSettings());
  const text = container.textContent ?? '';
  assert.ok(text.includes('系统设置'), 'the Vue heading renders');
  assert.ok(text.includes('平台级运行时配置，保存后立即对所有空间生效。仅系统管理员可见可改。'), 'the Vue description renders');
  for (const tab of ['账户与访问 7', '空间默认值 3', '运行与并发 7', '网络安全 2']) {
    assert.ok(tabButton(container, tab), `tab "${tab}" renders with the Vue count`);
  }
  assert.ok(text.includes('管理系统管理员、公开注册与用户创建空间的规则。'), 'the access section description renders');
  assert.ok(text.includes('自助注册模式'), 'the Vue keyLabel renders');
  assert.ok(text.includes('仅邀请（关闭公网注册）'), 'enum options render the Vue labels');
  assert.ok(text.includes('已覆盖'), 'the override badge renders for persisted rows');
  assert.ok(text.includes('上次修改：'), 'the modified-by meta line renders');
  assert.ok(text.includes('高风险'), 'the high-risk badge renders');
  assert.ok(text.includes('系统管理员'), 'the admins row renders');
  assert.ok(text.includes('重置用户密码'), 'the reset-password row renders');
  assert.ok(text.includes('创建用户'), 'the create-user row renders');

  await act(async () => tabButton(container, '运行与并发 7')?.click());
  const runtimeText = container.textContent ?? '';
  assert.ok(runtimeText.includes('配置后台任务池与模型服务的并发容量。'), 'the runtime section description renders');
  assert.ok(runtimeText.includes('Worker 配置需重启生效'), 'the runtime restart hint badge renders');
  assert.ok(runtimeText.includes('配置项与用途') && runtimeText.includes('当前值'), 'the runtime table header renders');
  assert.ok(runtimeText.includes('模型默认并发上限'), 'runtime keys use Vue keyLabels');

  await act(async () => tabButton(container, '网络安全 2')?.click());
  assert.ok((container.textContent ?? '').includes('SSRF 防护白名单'), 'the security section renders the ssrf label');
});

test('reset-to-default only renders for rows with a DB override; quota keeps the bulk-apply action', async () => {
  const container = await mountPanel(makeClient({ settings: fullSettings() }), fullSettings());
  const accessResets = Array.from(container.querySelectorAll('button')).filter((button) => button.textContent === '重置');
  assert.equal(accessResets.length, 1, 'only the overridden registration_mode row shows 重置 in the access group');

  await act(async () => tabButton(container, '空间默认值 3')?.click());
  const text = container.textContent ?? '';
  assert.ok(text.includes('新空间默认存储配额 (GB)'), 'the quota row uses the Vue keyLabel');
  const bulk = container.querySelector<HTMLButtonElement>('.wk-system-global-bulk');
  assert.ok(bulk, 'the quota row carries the bulk-apply action');
});

test('confirming the bulk apply calls applyDefaultStorageQuota and reports the Vue success copy', async () => {
  const calls = { promote: [], revoke: [], resetPassword: [], createUser: [], update: [], reset: [], bulk: 0 };
  const container = await mountPanel(makeClient({ settings: fullSettings(), calls }), fullSettings());
  await act(async () => tabButton(container, '空间默认值 3')?.click());
  await clickButton(container, '应用到所有现有空间');
  assert.ok((container.textContent ?? '').includes('将把所有现有空间的存储配额覆盖为'), 'the bulk confirm body renders with the saved value');
  await clickButton(container, '确认应用');
  await act(async () => {});
  assert.equal(calls.bulk, 1, 'applyDefaultStorageQuota was called once');
  assert.ok((container.textContent ?? '').includes('已将 3 个空间的存储配额更新为 30 GB'), 'the Vue bulk success copy renders');
});

test('promoting an admin goes through the Vue confirm copy and the promote API', async () => {
  const calls = { promote: [] as Array<unknown>, revoke: [] as Array<unknown>, resetPassword: [] as Array<unknown>, createUser: [] as Array<unknown>, update: [] as Array<[string, unknown]>, reset: [] as Array<string>, bulk: 0 };
  const admins = [{ id: 'u-peer', username: 'peer', email: 'peer-admin@local.dev', is_active: true, is_system_admin: true, created_at: '', updated_at: '' }];
  const container = await mountPanel(makeClient({ settings: fullSettings(), admins, calls, profile: { id: 'u-self' } }), fullSettings());
  assert.ok((container.textContent ?? '').includes('peer-admin@local.dev'), 'the peer admin tag renders');
  assert.equal((container.textContent ?? '').includes('self@local.dev'), false, 'the current user is not tagged');

  const field = container.querySelector<HTMLInputElement>('.wk-tag-input-field');
  assert.ok(field, 'the admin tag input renders');
  await act(async () => {
    setInputValue(field, 'new-admin@local.dev');
    field.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  });
  const confirmText = container.textContent ?? '';
  assert.ok(confirmText.includes('确认将 new-admin@local.dev 提升为系统管理员？'), 'the Vue promote confirm body renders');
  await clickButton(container, '确认提升');
  await act(async () => {});
  assert.deepEqual(calls.promote, [{ email: 'new-admin@local.dev' }], 'promote was called with the email payload');
  assert.ok((container.textContent ?? '').includes('已更新系统管理员'), 'the Vue save success copy renders');
});

test('removing an admin tag goes through the revoke confirm and API', async () => {
  const calls = { promote: [], revoke: [] as Array<unknown>, resetPassword: [], createUser: [], update: [] as Array<[string, unknown]>, reset: [], bulk: 0 };
  const admins = [{ id: 'u-peer', username: 'peer', email: 'peer-admin@local.dev', is_active: true, is_system_admin: true, created_at: '', updated_at: '' }];
  const container = await mountPanel(makeClient({ settings: fullSettings(), admins, calls, profile: { id: 'u-self' } }), fullSettings());
  const remove = container.querySelector<HTMLButtonElement>('.wk-tag-input-tag button');
  assert.ok(remove, 'the tag remove button renders');
  await act(async () => remove.click());
  assert.ok((container.textContent ?? '').includes('确认撤销 peer-admin@local.dev 的系统管理员权限？'), 'the Vue revoke confirm body renders');
  await clickButton(container, '确认撤销');
  await act(async () => {});
  assert.deepEqual(calls.revoke, ['u-peer'], 'revoke was called with the user id');
});

test('the reset-password dialog validates and calls resetPassword with the trimmed email', async () => {
  const calls = { promote: [], revoke: [], resetPassword: [] as Array<unknown>, createUser: [], update: [] as Array<[string, unknown]>, reset: [], bulk: 0 };
  const container = await mountPanel(makeClient({ settings: fullSettings(), calls }), fullSettings());
  await clickButton(container, '重置密码');
  // The Dialog portals to body; re-query after every interaction because a
  // re-render can replace the portal subtree (stale refs read empty text).
  const activeDialog = () => document.querySelector('[role="dialog"]');
  assert.ok(activeDialog(), 'the reset-password dialog opened');
  assert.ok((activeDialog()?.textContent ?? '').includes('重置其他用户的密码'), 'the Vue dialog title renders');
  assert.ok((activeDialog()?.textContent ?? '').includes('这是高风险操作。请核对用户邮箱'), 'the Vue warning renders');

  await clickButton(activeDialog()!, '确认重置');
  const errorText = activeDialog()?.textContent ?? '';
  assert.ok(errorText.includes('请输入邮箱') || errorText.includes('请输入有效的邮箱地址'), 'validation errors render');

  const inputs = () => Array.from((activeDialog()?.querySelectorAll<HTMLInputElement>('input')) ?? []);
  await act(async () => { setInputValue(inputs()[0]!, '  forgetful@local.dev '); });
  await act(async () => { setInputValue(inputs()[1]!, 'NewPassw0rd'); });
  await act(async () => { setInputValue(inputs()[2]!, 'NewPassw0rd'); });
  await clickButton(activeDialog()!, '确认重置');
  await act(async () => {});
  assert.deepEqual(calls.resetPassword, [{ email: 'forgetful@local.dev', new_password: 'NewPassw0rd' }], 'resetPassword received the trimmed payload');
  assert.ok((container.textContent ?? '').includes('密码已重置，该用户的现有会话已失效'), 'the Vue success copy renders');
});

test('creating a user with a generated password shows the one-time reveal view', async () => {
  const calls = { promote: [], revoke: [], resetPassword: [], createUser: [] as Array<unknown>, update: [] as Array<[string, unknown]>, reset: [], bulk: 0 };
  const container = await mountPanel(makeClient({ settings: fullSettings(), calls }), fullSettings());
  await clickButton(container, '创建用户');
  const activeDialog = () => document.querySelector('[role="dialog"]');
  assert.ok(activeDialog(), 'the create-user dialog opened');
  assert.ok((activeDialog()?.textContent ?? '').includes('创建新用户'), 'the Vue dialog title renders');
  const inputs = () => Array.from((activeDialog()?.querySelectorAll<HTMLInputElement>('input')) ?? []);
  await act(async () => { setInputValue(inputs()[0]!, 'parity-new'); });
  await act(async () => { setInputValue(inputs()[1]!, 'new-user@local.dev'); });
  await clickButton(activeDialog()!, '创建用户');
  await act(async () => {});
  assert.deepEqual(calls.createUser, [{ username: 'parity-new', email: 'new-user@local.dev' }], 'createUser omitted the password (auto-generate default)');
  const revealText = activeDialog()?.textContent ?? '';
  assert.ok(revealText.includes('已为该用户生成一个随机密码。此密码只会显示一次。'), 'the reveal body renders');
  assert.ok(revealText.includes('Gen-Passw0rd'), 'the one-time password is revealed');
  assert.ok(revealText.includes('我已保存密码'), 'the acknowledge button renders');
  await clickButton(activeDialog()!, '我已保存密码');
  assert.equal(document.querySelector('[role="dialog"]'), null, 'acknowledging closes the locked dialog');
});

test('high-risk registration mode requires confirmation before the PUT lands', async () => {
  const calls = { promote: [], revoke: [], resetPassword: [], createUser: [], update: [] as Array<[string, unknown]>, reset: [], bulk: 0 };
  const container = await mountPanel(makeClient({ settings: fullSettings(), calls }), fullSettings());
  const select = container.querySelector<HTMLSelectElement>('select');
  assert.ok(select, 'the enum control renders');
  await act(async () => {
    select.value = 'self_serve';
    select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
  const confirmText = container.textContent ?? '';
  assert.ok(confirmText.includes('即将把「自助注册模式」改为：self_serve'), 'the Vue high-risk confirm body renders with the new value');
  await clickButton(container, '确认保存');
  await act(async () => {});
  assert.deepEqual(calls.update, [['auth.registration_mode', 'self_serve']], 'the confirmed PUT landed with the selected value');
  assert.ok((container.textContent ?? '').includes('已保存'), 'the Vue save success copy renders');
});

test('resetting an overridden row confirms and deletes the override', async () => {
  const calls = { promote: [], revoke: [], resetPassword: [], createUser: [], update: [] as Array<[string, unknown]>, reset: [] as Array<string>, bulk: 0 };
  const container = await mountPanel(makeClient({ settings: fullSettings(), calls }), fullSettings());
  await clickButton(container, '重置');
  assert.ok((container.textContent ?? '').includes('确定要重置「自助注册模式」吗？'), 'the Vue reset confirm body renders');
  await clickButton(container, '确认重置');
  await act(async () => {});
  assert.deepEqual(calls.reset, ['auth.registration_mode'], 'the reset DELETE landed for the overridden key');
  assert.ok((container.textContent ?? '').includes('已重置为默认值'), 'the Vue reset success copy renders');
});

test('ssrf whitelist edits confirm per entry before the PUT', async () => {
  const calls = { promote: [], revoke: [], resetPassword: [], createUser: [], update: [] as Array<[string, unknown]>, reset: [], bulk: 0 };
  const container = await mountPanel(makeClient({ settings: fullSettings(), calls }), fullSettings());
  await act(async () => tabButton(container, '网络安全 2')?.click());
  const field = container.querySelector<HTMLInputElement>('.wk-tag-input-field');
  assert.ok(field, 'the ssrf tag input renders');
  await act(async () => {
    setInputValue(field, 'example.com');
    field.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  });
  const confirmText = container.textContent ?? '';
  assert.ok(confirmText.includes('确认把 example.com 加入 SSRF 白名单？'), 'the Vue per-entry add confirm renders');
  await clickButton(container, '确认添加');
  await act(async () => {});
  assert.deepEqual(calls.update, [['ssrf.whitelist', ['10.0.0.0/8', 'example.com']]], 'the merged whitelist landed in a single PUT');
});

test('other tab appears only when unknown keys exist', async () => {
  const settings = [...fullSettings(), { id: 99, key: 'future.unknown_key', value: 'x', value_type: 'string', category: 'system', description: '', is_secret: false, requires_restart: false, last_modified_by: '', created_at: '', updated_at: '' }] as SystemSetting[];
  const container = await mountPanel(makeClient({ settings }), settings);
  assert.ok(tabButton(container, '其他 1'), 'the conditional other tab renders with its count');
  const plain = await mountPanel(makeClient({ settings: fullSettings() }), fullSettings());
  assert.equal(tabButton(plain, '其他 1'), undefined, 'no other tab without unknown keys');
});
