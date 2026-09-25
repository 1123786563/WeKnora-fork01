import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// Interactive render harness (same pattern as apps/web GeneralPreferencesPanel.test.tsx).
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/integrations' });
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
type IntegrationResource = { id: string; name?: string; platform?: string; agent_id?: string; enabled?: boolean; [key: string]: unknown };

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

function setNativeValue(element: HTMLSelectElement | HTMLInputElement, value: string) {
  element.value = value;
  element.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

// Semantic lookup for the dashed add tile (button role + tile label text);
// replaces the former .wk-channel-card--add class hook.
function findAddTile(container: HTMLElement, label: string) {
  return Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find((tile) => tile.textContent?.includes(label));
}

async function mountPage(input: { imChannels?: IntegrationResource[]; agents?: { id: string; name: string }[]; onCreateIm?: (input: { agentId: string; payload: Record<string, unknown> }) => Promise<void>; onUpdateIm?: (id: string, input: Record<string, unknown>) => Promise<void> }) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(IntegrationsPage, {
      initialTab: 'im',
      activeTab: 'im',
      embedChannels: [],
      imChannels: input.imChannels ?? [],
      apiBaseUrl: 'https://weknora.test',
      locale: 'zh-CN',
      agents: input.agents ?? [],
      actions: {
        onCreateIm: input.onCreateIm ?? (async () => undefined),
        onUpdateIm: input.onUpdateIm ?? (async () => undefined),
      },
    }));
  });
  return container;
}

// Vue parity: IMChannelPanel.vue renders the channel list with a dashed
// 添加渠道 tile that opens the 4-step drawer (基本信息 → 连接设置 → 文件存储 → 平台凭证).
test('IM add tile opens the Vue 4-step wizard in zh-CN', async () => {
  const container = await mountPage({ agents: [{ id: 'agent-1', name: '知识助手' }] });
  const addTile = findAddTile(container, '添加渠道');
  assert.ok(addTile, 'add tile rendered');
  await act(async () => { addTile!.click(); });

  const steps = Array.from(container.querySelectorAll('.wk-im-step'));
  assert.deepEqual(steps.map((step) => step.textContent), ['1基本信息', '2连接设置', '3文件存储', '4平台凭证']);
  const active = container.querySelector('.wk-im-step.is-active');
  assert.match(active?.textContent ?? '', /基本信息/);

  // Footer buttons follow the Vue drawer: 下一步 on step 1, no 上一步 yet.
  const actions = container.querySelectorAll('.wk-form-actions .wk-button');
  assert.ok(!Array.from(actions).some((button) => button.textContent === '上一步'));
  assert.ok(Array.from(actions).some((button) => button.textContent === '下一步'));
});

test('step 0 blocks Next without a bound agent like Vue validateWizardStep', async () => {
  const container = await mountPage({ agents: [{ id: 'agent-1', name: '知识助手' }] });
  await act(async () => { findAddTile(container, '添加渠道')!.click(); });
  await act(async () => { Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '下一步')!.click(); });
  const alert = container.querySelector('[role="alert"]');
  assert.match(alert?.textContent ?? '', /请先选择一个智能体/);
  assert.ok(container.querySelector('.wk-im-step.is-active'), 'stays on step 1');
});

test('wecom websocket wizard reaches credentials and posts the Vue create payload', async () => {
  const payloads: Array<{ agentId: string; payload: Record<string, unknown> }> = [];
  const container = await mountPage({
    agents: [{ id: 'agent-1', name: '知识助手' }],
    onCreateIm: async (input) => { payloads.push(input); },
  });
  await act(async () => { findAddTile(container, '添加渠道')!.click(); });

  // Step order inside the wizard form: bound agent, then platform.
  const agentSelect = container.querySelector('form select') as HTMLSelectElement;
  await act(async () => { setNativeValue(agentSelect, 'agent-1'); });
  await act(async () => { Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '下一步')!.click(); });

  // Step 2 (connection): mode + output chips, and the wecom thread chip is off.
  assert.match(container.querySelector('.wk-im-step.is-active')?.textContent ?? '', /连接设置/);
  const radios = Array.from(container.querySelectorAll('[role="radiogroup"] [role="radio"]'));
  assert.equal(radios[0]?.getAttribute('aria-checked'), 'true');
  const next = () => Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '下一步')!;
  await act(async () => { next().click(); });
  assert.match(container.querySelector('.wk-im-step.is-active')?.textContent ?? '', /文件存储/);
  await act(async () => { next().click(); });

  // Step 3 (credentials): Vue wecom websocket table — Bot ID / Bot Secret / WS endpoint.
  assert.match(container.querySelector('.wk-im-step.is-active')?.textContent ?? '', /平台凭证/);
  const labels = Array.from(container.querySelectorAll('label')).map((label) => label.textContent ?? '');
  assert.ok(labels.some((label) => label.includes('Bot ID')));
  assert.ok(labels.some((label) => label.includes('Bot Secret')));
  assert.ok(labels.some((label) => label.includes('WebSocket Endpoint')));

  const saveButton = Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '保存')!;
  await act(async () => { saveButton.click(); });
  assert.equal(payloads.length, 1);
  assert.equal(payloads[0]!.agentId, 'agent-1');
  assert.deepEqual(payloads[0]!.payload, {
    platform: 'wecom',
    name: '企业微信',
    mode: 'websocket',
    output_mode: 'stream',
    session_mode: 'user',
    knowledge_base_id: '',
    credentials: {},
  });
});

test('wechat platform hides the access section and gates save on the QR binding', async () => {
  const container = await mountPage({ agents: [{ id: 'agent-1', name: '知识助手' }] });
  await act(async () => { findAddTile(container, '添加渠道')!.click(); });
  const agentSelect = container.querySelector('form select') as HTMLSelectElement;
  await act(async () => { setNativeValue(agentSelect, 'agent-1'); });
  const platformSelect = container.querySelectorAll('form select')[1] as HTMLSelectElement;
  await act(async () => { setNativeValue(platformSelect, 'wechat'); });

  await act(async () => { Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '下一步')!.click(); });
  assert.match(container.querySelector('.wk-im-step.is-active')?.textContent ?? '', /连接设置/);
  // Vue hides the 接入与输出 section for wechat; only 会话设置 remains.
  const legends = Array.from(container.querySelectorAll('legend')).map((legend) => legend.textContent ?? '');
  assert.ok(legends.some((legend) => legend.includes('会话设置')));
  assert.ok(!legends.some((legend) => legend.includes('接入与输出')));

  await act(async () => { Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '下一步')!.click(); });
  await act(async () => { Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '下一步')!.click(); });
  assert.match(container.querySelector('.wk-im-step.is-active')?.textContent ?? '', /平台凭证/);
  assert.match(container.textContent ?? '', /扫码绑定微信/);
  const saveButton = Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '保存')!;
  await act(async () => { saveButton.click(); });
  assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /扫码绑定微信/);
});

test('clicking an IM channel card reopens the wizard prefilled like Vue openDrawer', async () => {
  const updates: Array<{ id: string; input: Record<string, unknown> }> = [];
  const channel: IntegrationResource = {
    id: 'ch-1', name: '客服渠道', platform: 'slack', agent_id: 'agent-9', enabled: false,
    mode: 'webhook', output_mode: 'full', session_mode: 'thread', knowledge_base_id: 'kb-1',
    credentials: { bot_token: 'xoxb-', signing_secret: 's' },
  };
  const container = await mountPage({ imChannels: [channel], onUpdateIm: async (id, input) => { updates.push({ id, input }); } });
  await act(async () => { (container.querySelector('article') as HTMLElement).click(); });

  const drawerTitle = container.querySelector('.setting-drawer__title');
  assert.equal(drawerTitle?.textContent, '客服渠道');
  const platformSelect = Array.from(container.querySelectorAll('select')).find((select) => select.disabled) as HTMLSelectElement | undefined;
  assert.equal(platformSelect?.value, 'slack', 'platform select disabled and pinned while editing');
  assert.ok((container.querySelector('input[type="checkbox"]') as HTMLInputElement | null)?.checked === false, 'enabled switch reflects the disabled channel');

  const next = () => Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '下一步')!;
  await act(async () => { next().click(); });
  // Edit + webhook shows the Vue callback URL section.
  const legends = Array.from(container.querySelectorAll('legend')).map((legend) => legend.textContent ?? '');
  assert.ok(legends.some((legend) => legend.includes('回调地址')));
  const callbackInput = container.querySelector('input[readonly]') as HTMLInputElement;
  assert.equal(callbackInput?.value, 'https://weknora.test/api/v1/im/callback/ch-1');

  await act(async () => { next().click(); });
  await act(async () => { next().click(); });
  assert.match(container.querySelector('.wk-im-step.is-active')?.textContent ?? '', /平台凭证/);
  const labels = Array.from(container.querySelectorAll('label')).map((label) => label.textContent ?? '');
  assert.ok(labels.some((label) => label.includes('Signing Secret')), 'slack webhook credential table');

  await act(async () => { Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '保存')!.click(); });
  assert.equal(updates.length, 1);
  assert.equal(updates[0]!.id, 'ch-1');
  assert.deepEqual(updates[0]!.input, {
    name: '客服渠道',
    mode: 'webhook',
    output_mode: 'full',
    session_mode: 'thread',
    knowledge_base_id: 'kb-1',
    credentials: { bot_token: 'xoxb-', signing_secret: 's' },
    enabled: false,
    agent_id: 'agent-9',
  });
});
