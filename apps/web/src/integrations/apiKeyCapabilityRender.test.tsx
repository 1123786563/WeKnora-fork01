import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// Interactive render harness (same pattern as apps/web imWizardRender.test.tsx).
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
// Direct source import keeps this render test independent of the package barrel.
const { IntegrationsPage } = await import('../../../../packages/views/src/integrations/page.tsx');
type ApiKeyRow = { id: number | string; name: string; api_key: string; full_access: boolean; capabilities?: string[] };
type ApiKeyPayload = { name: string; full_access: boolean; knowledge_base_ids: string[]; capabilities: string[] };

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

async function mountPage(input: {
  tab?: 'api' | 'chrome' | 'claw';
  knowledgeBases?: { id: string; name: string }[];
  onCreateApiKey?: (payload: ApiKeyPayload) => Promise<ApiKeyRow>;
}) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(IntegrationsPage, {
      initialTab: input.tab ?? 'api',
      activeTab: input.tab ?? 'api',
      embedChannels: [],
      imChannels: [],
      apiBaseUrl: 'https://weknora.test',
      locale: 'zh-CN',
      knowledgeBases: input.knowledgeBases ?? [],
      actions: {
        onCreateApiKey: input.onCreateApiKey ?? (async () => ({ id: 1, name: 'k', api_key: 'wk-x', full_access: false })),
      },
    }));
  });
  return container;
}

function buttonByText(container: HTMLElement, text: string, selector = 'button'): HTMLButtonElement | undefined {
  return Array.from(container.querySelectorAll<HTMLButtonElement>(selector)).find((button) => button.textContent?.trim() === text);
}

// React 19 drives checkbox onChange from the click event (react-dom
// getTargetInstForClickEvent), and jsdom's activation behavior toggles
// `checked` on click — so toggling via click() is the faithful user path.
// Text inputs still need the native prototype value setter because React's
// value tracker swallows plain assignment.
const nativeInputValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;

function setChecked(input: HTMLInputElement, checked: boolean) {
  if (input.checked !== checked) input.click();
}

function setInputValue(input: HTMLInputElement, value: string) {
  nativeInputValue?.call(input, value);
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
}

async function openCreateDialog(container: HTMLElement) {
  await act(async () => { buttonByText(container, '创建 API Key', 'button[type="button"]')?.click(); });
}

// Vue baseline: ApiIntegrationSettings.vue create drawer (SettingDrawer
// L462-564) — 访问类型 radio (能力授权 / 空间完全访问), the four capability
// groups (知识库数据 / 智能体与集成 / 成员与空间 / 空间配置) with a 全选/清空
// toggle per group and a checkbox + hint per capability, plus the 知识库范围
// multi-select shown while a KB-scoped capability is selected below full access.

test('create dialog renders the full Vue capability matrix in zh-CN', async () => {
  const container = await mountPage({ knowledgeBases: [{ id: 'kb-1', name: '产品手册' }] });
  await openCreateDialog(container);

  const form = container.querySelector('form');
  assert.ok(form, 'create form rendered');
  // Vue drawer description (integrations.api.createApiKeyDialogDesc).
  assert.match(form?.textContent ?? '', /选择该 API Key 可用的能力和知识库范围/);

  // 访问类型 radio: 能力授权 (scoped) default-on, 空间完全访问 (full).
  const accessGroup = Array.from(form?.querySelectorAll('[role="radiogroup"]') ?? []).find((group) => group.textContent?.includes('空间完全访问'));
  assert.ok(accessGroup, 'access type radio rendered');
  const scopedChip = Array.from(accessGroup?.querySelectorAll('[role="radio"]') ?? []).find((chip) => chip.textContent === '能力授权');
  const fullChip = Array.from(accessGroup?.querySelectorAll('[role="radio"]') ?? []).find((chip) => chip.textContent === '空间完全访问');
  assert.equal(scopedChip?.getAttribute('aria-checked'), 'true');
  assert.equal(fullChip?.getAttribute('aria-checked'), 'false');
  assert.match(form?.textContent ?? '', /「知识库」按下方权限级别/);

  // The four group headers in Vue order, each with a 全选 toggle.
  const groupHeads = Array.from(form?.querySelectorAll('.api-key-capability-group__header span') ?? []).map((node) => node.textContent);
  assert.deepEqual(groupHeads, ['知识库数据', '智能体与集成', '成员与空间', '空间配置']);
  assert.equal(form?.querySelectorAll('.api-key-capability-group__header button').length, 4, 'one 全选/清空 button per group');

  // All 18 capability checkboxes with Vue default on retrieve/chat/read_agents.
  const checkboxes = Array.from(form?.querySelectorAll<HTMLInputElement>('.api-key-capability-item input[type="checkbox"]') ?? []);
  assert.equal(checkboxes.length, 18);
  const labels = Array.from(form?.querySelectorAll('.api-key-capability-item .wk-check-row') ?? []).map((node) => node.textContent);
  for (const expected of ['检索知识库', '对话能力', '写入知识库内容', '管理知识库', '消息历史', '读取智能体', '管理智能体', '管理 MCP 服务', '管理数据源', '管理成员', '管理空间', '管理模型', '管理检索基础设施', '管理存储后端', '管理联网搜索', '管理渠道', '运行评测', '管理空间设置']) {
    assert.ok(labels.some((label) => label === expected), 'capability label ' + expected);
  }
  // Hint copy is ported too (Vue capability.*Hint).
  assert.match(form?.textContent ?? '', /允许读取、查询和检索所选知识库范围内的数据/);
  assert.match(form?.textContent ?? '', /允许创建、修改、删除、复制智能体/);

  // Default selections: 检索知识库 / 对话能力 / 读取智能体.
  const checked = (label: string) => Array.from(form?.querySelectorAll<HTMLInputElement>('.api-key-capability-item') ?? [])
    .find((item) => item.querySelector('.wk-check-row')?.textContent === label)?.querySelector('input');
  assert.equal(checked('检索知识库')?.checked, true);
  assert.equal(checked('对话能力')?.checked, true);
  assert.equal(checked('读取智能体')?.checked, true);
  assert.equal(checked('写入知识库内容')?.checked, false);

  // 知识库范围 multi-select is visible (retrieve is KB-scoped by default).
  const kbSelect = form?.querySelector<HTMLSelectElement>('select[multiple]');
  assert.ok(kbSelect, 'knowledge scope multi-select rendered');
  assert.ok(Array.from(kbSelect?.options ?? []).some((option) => option.textContent === '产品手册'));
});

test('full access hides the matrix and switches the hint like Vue', async () => {
  const container = await mountPage({});
  await openCreateDialog(container);
  const form = container.querySelector('form')!;

  await act(async () => { (Array.from(form.querySelectorAll('[role="radio"]')).find((chip) => chip.textContent === '空间完全访问') as HTMLElement).click(); });
  assert.match(form.textContent ?? '', /允许调用模型、向量库、数据源、渠道等全部空间级接口/);
  assert.equal(form.querySelectorAll('.api-key-capability-item').length, 0, 'capability matrix hidden');
  assert.equal(form.querySelectorAll('select[multiple]').length, 0, 'knowledge scope hidden');
});

test('scoped submit posts the Vue payload shape; empty selection is blocked', async () => {
  const payloads: ApiKeyPayload[] = [];
  const container = await mountPage({ onCreateApiKey: async (payload) => { payloads.push(payload); return { id: 7, name: payload.name, api_key: "wk-7", full_access: payload.full_access }; } });
  await openCreateDialog(container);
  const form = container.querySelector('form')!;

  // Defaults: scoped with retrieve/chat/read_agents.
  await act(async () => { setInputValue(form.querySelector('input[type="text"]') as HTMLInputElement, '集成只读 Key'); });
  await act(async () => { (form.querySelector('button[type="submit"]') as HTMLButtonElement).click(); });
  assert.deepEqual(payloads[0], {
    name: '集成只读 Key',
    full_access: false,
    knowledge_base_ids: [],
    capabilities: ['retrieve', 'chat', 'read_agents'],
  });
  assert.equal(container.contains(form), false, 'form closes after a successful create like Vue');

  // Re-open resets to the Vue defaults; emptying the selection is rejected.
  await openCreateDialog(container);
  const reopened = container.querySelector('form')!;
  await act(async () => { setInputValue(reopened.querySelector('input[type="text"]') as HTMLInputElement, '空能力 Key'); });
  for (const label of ['检索知识库', '对话能力', '读取智能体']) {
    const item = Array.from(reopened.querySelectorAll<HTMLElement>('.api-key-capability-item')).find((node) => node.querySelector('.wk-check-row')?.textContent === label);
    await act(async () => { setChecked(item?.querySelector('input') as HTMLInputElement, false); });
  }
  await act(async () => { (reopened.querySelector('button[type="submit"]') as HTMLButtonElement).click(); });
  assert.equal(payloads.length, 1, 'no second payload');
  assert.match(reopened.querySelector('[role="alert"]')?.textContent ?? '', /至少需要选择一项能力/);
});

test('group 全选 toggles the whole group and the payload follows the Vue canonical order', async () => {
  const payloads: ApiKeyPayload[] = [];
  const container = await mountPage({ onCreateApiKey: async (payload) => { payloads.push(payload); return { id: 8, name: payload.name, api_key: "wk-8", full_access: payload.full_access }; } });
  await openCreateDialog(container);
  const form = container.querySelector('form')!;

  await act(async () => { setInputValue(form.querySelector('input[type="text"]') as HTMLInputElement, '成员 Key'); });
  // 成员与空间 group 全选 adds manage_members + manage_spaces.
  await act(async () => {
    const collaboration = Array.from(form.querySelectorAll('.api-key-capability-group')).find((group) => group.querySelector('.api-key-capability-group__header span')?.textContent === '成员与空间');
    buttonByText(collaboration as HTMLElement, '全选')?.click();
  });
  await act(async () => { (form.querySelector('button[type="submit"]') as HTMLButtonElement).click(); });
  assert.deepEqual(payloads[0]?.capabilities, ['retrieve', 'chat', 'read_agents', 'manage_members', 'manage_spaces']);
});

test('full-access submit empties capabilities and knowledge scope like Vue', async () => {
  const payloads: ApiKeyPayload[] = [];
  const container = await mountPage({ onCreateApiKey: async (payload) => { payloads.push(payload); return { id: 9, name: payload.name, api_key: "wk-9", full_access: payload.full_access }; } });
  await openCreateDialog(container);
  const form = container.querySelector('form')!;

  await act(async () => { setInputValue(form.querySelector('input[type="text"]') as HTMLInputElement, '完全访问 Key'); });
  await act(async () => { (Array.from(form.querySelectorAll('[role="radio"]')).find((chip) => chip.textContent === '空间完全访问') as HTMLElement).click(); });
  await act(async () => { (form.querySelector('button[type="submit"]') as HTMLButtonElement).click(); });
  assert.deepEqual(payloads[0], { name: '完全访问 Key', full_access: true, knowledge_base_ids: [], capabilities: [] });
});

// D8 (R482 B3): Vue ChromeExtensionLanding.vue L61-63 / ClawSkillLanding.vue
// L55-57 render a 打开 API 信息 outline button on the 'api' step that opens the
// API integration settings (router.push ?section=integration-api). The React
// equivalent switches the integrations surface to the API tab.

test('chrome and claw landings expose 打开 API 信息 and switch to the API tab', async () => {
  for (const tab of ['chrome', 'claw'] as const) {
    const container = await mountPage({ tab });
    const apiButton = buttonByText(container, '打开 API 信息');
    assert.ok(apiButton, tab + ' landing has the open API settings entry');
    await act(async () => { apiButton!.click(); });
    assert.match(container.textContent ?? '', /API Keys/, tab + ' switched to the API integration panel');
  }
});
