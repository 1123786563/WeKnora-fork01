import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// Interactive render harness (same pattern as imWizardRender.test.tsx).
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

function setNativeValue(element: HTMLSelectElement | HTMLInputElement | HTMLTextAreaElement, value: string) {
  // React patches the instance value setter with its tracker; go through the
  // prototype setter (as testing-library does) so onChange actually fires.
  const proto = element instanceof dom.window.HTMLTextAreaElement
    ? dom.window.HTMLTextAreaElement.prototype
    : element instanceof dom.window.HTMLSelectElement
      ? dom.window.HTMLSelectElement.prototype
      : dom.window.HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, 'value')?.set;
  if (setter) setter.call(element, value); else element.value = value;
  element.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

// Semantic lookup for the dashed add tile (button role + tile label text);
// replaces the former .wk-channel-card--add class hook.
function findAddTile(container: HTMLElement, label: string) {
  return Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find((tile) => tile.textContent?.includes(label));
}

interface EmbedMountInput {
  embedChannels?: IntegrationResource[];
  agents?: { id: string; name: string; config?: Record<string, unknown> }[];
  canEdit?: boolean;
  onCreateEmbed?: (input: { agentId: string; payload: Record<string, unknown> }) => Promise<IntegrationResource | void>;
  onUpdateEmbed?: (id: string, input: Record<string, unknown>) => Promise<void>;
  onRotateEmbed?: (id: string) => Promise<void>;
  onEmbedDetail?: (id: string) => Promise<IntegrationResource | null>;
  onOpenEmbed?: (channel: IntegrationResource) => void;
}

async function mountEmbedPage(input: EmbedMountInput = {}) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(IntegrationsPage, {
      initialTab: 'embed',
      activeTab: 'embed',
      embedChannels: input.embedChannels ?? [],
      imChannels: [],
      apiBaseUrl: 'https://weknora.test',
      locale: 'zh-CN',
      agents: input.agents ?? [{ id: 'agent-1', name: '知识助手' }],
      canEdit: input.canEdit ?? true,
      onOpenEmbed: input.onOpenEmbed,
      actions: {
        onCreateEmbed: input.onCreateEmbed ?? (async () => undefined),
        onUpdateEmbed: input.onUpdateEmbed ?? (async () => undefined),
        onRotateEmbed: input.onRotateEmbed,
        onEmbedDetail: input.onEmbedDetail,
      },
    }));
  });
  return container;
}

const nextButton = (container: HTMLElement) => Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '下一步')!;
const saveButton = (container: HTMLElement) => Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).find((button) => button.textContent === '保存')!;

// Vue parity: AgentEmbedChannelPanel.vue renders the channel list with a dashed
// 新建嵌入渠道 tile that opens the 6-step SettingDrawer wizard (渠道 → 安全 → 能力 →
// 外观 → 回调, 部署 only while editing).
test('embed add tile opens the Vue 5-step create wizard in zh-CN', async () => {
  const container = await mountEmbedPage();
  const addTile = findAddTile(container, '新建嵌入渠道');
  assert.ok(addTile, 'add tile rendered');
  await act(async () => { addTile!.click(); });

  const steps = Array.from(container.querySelectorAll('.wk-embed-step'));
  assert.deepEqual(steps.map((step) => step.textContent), ['1渠道信息', '2安全限流', '3对话能力', '4外观展示', '5事件回调']);
  assert.match(container.querySelector('.wk-im-legend')?.textContent ?? '', /渠道信息/);

  const actions = Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-form-actions .wk-button')).map((button) => button.textContent);
  assert.ok(!actions.includes('上一步'), 'no back button on the first step');
  assert.ok(actions.includes('下一步'));
  assert.ok(!actions.includes('保存'), 'create mode has no deploy step, so the footer stays 下一步');
});

test('step gates mirror Vue validateWizardStep: agent first, then origins', async () => {
  const container = await mountEmbedPage();
  await act(async () => { findAddTile(container, '新建嵌入渠道')!.click(); });

  await act(async () => { nextButton(container).click(); });
  assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /请先选择一个智能体/);
  assert.match(container.querySelector('.wk-im-legend')?.textContent ?? '', /渠道信息/, 'stays on step 1');

  const agentSelect = container.querySelector('form select') as HTMLSelectElement;
  await act(async () => { setNativeValue(agentSelect, 'agent-1'); });
  await act(async () => { nextButton(container).click(); });
  assert.match(container.querySelector('.wk-im-legend')?.textContent ?? '', /安全与限流/);

  // Vue requires at least one origin before leaving the security step.
  await act(async () => { nextButton(container).click(); });
  assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /请至少填写一个域名白名单/);
  assert.match(container.querySelector('.wk-im-legend')?.textContent ?? '', /安全与限流/, 'stays on step 2');

  const origins = container.querySelector('form textarea') as HTMLTextAreaElement;
  await act(async () => { setNativeValue(origins, 'https://shop.example.com'); });
  await act(async () => { nextButton(container).click(); });
  assert.match(container.querySelector('.wk-im-legend')?.textContent ?? '', /对话能力/);
});

test('embed create posts the Vue payload, lands on the deploy step and reveals the key', async () => {
  const payloads: Array<{ agentId: string; payload: Record<string, unknown> }> = [];
  const created: IntegrationResource = {
    id: 'ch-new', name: '知识助手 · 网页嵌入', agent_id: 'agent-1', enabled: true, publish_token: 'tok_new',
    welcome_message: '', allowed_origins: ['https://shop.example.com'], rate_limit_per_minute: 30,
    rate_limit_per_day: 10000, primary_color: '#07C05F', page_title: '', header_title_mode: 'channel',
    show_suggested_questions: true, widget_position: 'bottom-right', allow_web_search: false,
    allow_file_upload: false, default_locale: '', webhook_url: '',
  };
  const container = await mountEmbedPage({ onCreateEmbed: async (input) => { payloads.push(input); return created; } });
  await act(async () => { findAddTile(container, '新建嵌入渠道')!.click(); });

  const agentSelect = container.querySelector('form select') as HTMLSelectElement;
  await act(async () => { setNativeValue(agentSelect, 'agent-1'); });
  // Vue applyDefaultChannelNameIfNeeded: untouched create name follows the agent.
  const nameInput = container.querySelector('form input:not([type="color"])') as HTMLInputElement;
  assert.equal(nameInput.value, '知识助手 · 网页嵌入');
  await act(async () => { nextButton(container).click(); });

  const origins = container.querySelector('form textarea') as HTMLTextAreaElement;
  await act(async () => { setNativeValue(origins, 'https://shop.example.com'); });
  await act(async () => { nextButton(container).click(); });
  await act(async () => { nextButton(container).click(); });
  await act(async () => { nextButton(container).click(); });
  // Create mode shows the deploy-after-save hint instead of a deploy step.
  assert.match(container.querySelector('[role="note"]')?.textContent ?? '', /保存渠道后，可在此复制渠道密钥与嵌入代码/);
  await act(async () => { saveButton(container).click(); });

  assert.equal(payloads.length, 1);
  assert.equal(payloads[0].agentId, 'agent-1');
  assert.deepEqual(payloads[0].payload, {
    name: '知识助手 · 网页嵌入',
    welcome_message: '',
    allowed_origins: ['https://shop.example.com'],
    rate_limit_per_minute: 30,
    rate_limit_per_day: 10000,
    primary_color: '#07C05F',
    page_title: '',
    header_title_mode: 'channel',
    show_suggested_questions: true,
    widget_position: 'bottom-right',
    allow_web_search: false,
    allow_file_upload: false,
    default_locale: '',
    webhook_url: '',
    enabled: true,
    agent_id: 'agent-1',
  });

  // Vue jumps to the deploy step after create (AgentEmbedChannelPanel L966).
  const steps = Array.from(container.querySelectorAll('.wk-embed-step'));
  assert.equal(steps.length, 6, 'deploy step appended after create');
  assert.match(container.querySelector('.wk-embed-step.is-active')?.textContent ?? '', /网页嵌入/);
  assert.match(container.querySelector('[role="status"]')?.textContent ?? '', /嵌入渠道已创建/);
  const code = container.querySelector('.wk-embed-code-panel pre');
  assert.ok(code?.textContent?.includes('embed/ch-new#token=tok_new'), 'iframe snippet carries the publish token');
  const keyInput = container.querySelector('.wk-embed-key-input') as HTMLInputElement;
  assert.equal(keyInput.value, 'tok_new', 'Vue reveals the created key (L954-956)');
});

test('clicking an embed card opens the deploy drawer: key reveal, rotate, snippet tabs and update payload', async () => {
  const updates: Array<{ id: string; input: Record<string, unknown> }> = [];
  const rotations: string[] = [];
  const previews: string[] = [];
  const channel: IntegrationResource = {
    id: 'ch-1', name: '客服渠道', agent_id: 'agent-1', enabled: true,
    welcome_message: '您好', rate_limit_per_minute: 20, rate_limit_per_day: 5000,
    primary_color: '#123456', page_title: '支持', header_title_mode: 'channel',
    show_suggested_questions: true, widget_position: 'bottom-right', allow_web_search: false,
    allow_file_upload: false, default_locale: '', webhook_url: '',
    allowed_origins: ['https://a.example.test'],
  };
  const detail = { ...channel, publish_token: 'tok_edit', has_webhook_secret: true };
  (dom.window as unknown as { confirm: (message?: string) => boolean }).confirm = () => true;
  const container = await mountEmbedPage({
    embedChannels: [channel],
    onEmbedDetail: async () => detail,
    onUpdateEmbed: async (id, input) => { updates.push({ id, input }); },
    onRotateEmbed: async (id) => { rotations.push(id); },
    onOpenEmbed: (target) => { previews.push(String(target.id)); },
  });
  await act(async () => { (container.querySelector('article') as HTMLElement).click(); });

  // Vue openDrawer lands on the deploy step for editing.
  assert.equal(container.querySelectorAll('.wk-embed-step').length, 6);
  assert.match(container.querySelector('.wk-embed-step.is-active')?.textContent ?? '', /网页嵌入/);
  const code = container.querySelector('.wk-embed-code-panel pre');
  assert.ok(code?.textContent?.includes('embed/ch-1#token=tok_edit'), 'edit drawer shows the iframe snippet');

  const keyInput = container.querySelector('.wk-embed-key-input') as HTMLInputElement;
  assert.equal(keyInput.value, '••••••••', 'masked like Vue displayChannelKey');
  const reveal = Array.from(container.querySelectorAll<HTMLButtonElement>('button[title]')).find((button) => button.title === '显示密钥');
  assert.ok(reveal, 'reveal button present');
  await act(async () => { reveal!.click(); });
  assert.equal((container.querySelector('.wk-embed-key-input') as HTMLInputElement).value, 'tok_edit');

  // Secure tab: snippet without token + Node/Go server examples.
  const secureTab = Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-embed-snippet-tabs button')).find((button) => button.textContent === '安全模式');
  await act(async () => { secureTab!.click(); });
  const secureCode = container.querySelector('.wk-embed-code-panel pre')?.textContent ?? '';
  assert.ok(secureCode.includes('data-token-endpoint'), 'secure widget points at the token endpoint');
  assert.ok((container.querySelector('.wk-embed-server-panel pre')?.textContent ?? '').includes("app.get('/weknora/embed-token'"), 'Node example rendered');
  const goTab = Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-embed-server-tabs button')).find((button) => button.textContent === 'Go');
  await act(async () => { goTab!.click(); });
  assert.ok((container.querySelector('.wk-embed-server-panel pre')?.textContent ?? '').includes('WEKNORA_PUBLISH_TOKEN'));

  const iframeTab = Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-embed-snippet-tabs button')).find((button) => button.textContent === 'iframe');
  await act(async () => { iframeTab!.click(); });
  const previewButton = Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-embed-code-panel button')).find((button) => button.textContent === '预览');
  await act(async () => { previewButton!.click(); });
  // The views panel defers the iframe until the drawer lays out (Vue nextTick
  // parity, layoutReady gate); flush the deferred mount before asserting.
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 60)); });
  assert.equal(previews.length, 0, 'preview stays in the current Vue-shaped drawer');
  // The preview drawer is the only aside[role="dialog"] with an aria-label
  // (replaces the former .wk-embed-preview-drawer class hook).
  assert.ok(container.querySelector('aside[role="dialog"][aria-label]'), 'preview drawer rendered');
  assert.ok(container.querySelector('aside[role="dialog"][aria-label] iframe'), 'preview iframe mounted');

  const rotate = Array.from(container.querySelectorAll<HTMLButtonElement>('button[title]')).find((button) => button.title === '重置渠道密钥');
  await act(async () => { rotate!.click(); });
  assert.deepEqual(rotations, ['ch-1'], 'rotate goes through the route page port');

  // Walk back to step 1 through the clickable step strip, then save unchanged.
  await act(async () => { (container.querySelectorAll('.wk-embed-step')[0] as HTMLElement).click(); });
  assert.match(container.querySelector('.wk-im-legend')?.textContent ?? '', /渠道信息/);
  await act(async () => { nextButton(container).click(); });
  await act(async () => { nextButton(container).click(); });
  await act(async () => { nextButton(container).click(); });
  await act(async () => { nextButton(container).click(); });
  // Edit mode swaps the deploy-after-save hint for the webhook keep-secret hint.
  const secretInput = container.querySelector('form input[type="password"]') as HTMLInputElement;
  assert.equal(secretInput.placeholder, '留空表示不修改已保存的密钥');
  await act(async () => { nextButton(container).click(); });
  await act(async () => { saveButton(container).click(); });

  assert.equal(updates.length, 1);
  assert.equal(updates[0].id, 'ch-1');
  assert.deepEqual(updates[0].input, {
    name: '客服渠道',
    welcome_message: '您好',
    allowed_origins: ['https://a.example.test'],
    rate_limit_per_minute: 20,
    rate_limit_per_day: 5000,
    primary_color: '#123456',
    page_title: '支持',
    header_title_mode: 'channel',
    show_suggested_questions: true,
    widget_position: 'bottom-right',
    allow_web_search: false,
    allow_file_upload: false,
    default_locale: '',
    webhook_url: '',
    enabled: true,
    agent_id: 'agent-1',
  });
});

test('non-admin embed editing keeps Vue fields read-only and hides the mutation footer', async () => {
  const channel: IntegrationResource = {
    id: 'ch-readonly', name: '只读渠道', agent_id: 'agent-1', enabled: true,
    allowed_origins: ['https://readonly.example.test'], publish_token: 'tok_readonly',
  };
  const container = await mountEmbedPage({ embedChannels: [channel], canEdit: false });
  await act(async () => { (container.querySelector('article') as HTMLElement).click(); });

  const firstStep = container.querySelector('.wk-embed-step') as HTMLElement;
  await act(async () => { firstStep.click(); });
  const form = container.querySelector('form')!;
  const editableControls = Array.from(form.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('input, select, textarea'));
  assert.ok(editableControls.length > 0, 'read-only drawer still exposes the Vue configuration fields');
  assert.ok(editableControls.every((control) => control.disabled || Array.from(control.closest('form')?.querySelectorAll('fieldset') ?? []).some((field) => field.contains(control) && field.disabled)), 'Vue disables every embed field for non-admins');
  assert.equal(form.querySelector('.wk-form-actions'), null, 'Vue hides the SettingDrawer footer for non-admins');

  await act(async () => { (container.querySelector('.wk-embed-step') as HTMLElement).click(); });
  assert.equal(Array.from(container.querySelectorAll('button')).some((button) => button.title === '重置渠道密钥'), false, 'Vue hides reset-key mutation for non-admins');
});

test('the wizard walks all steps with localized copy and no raw key leaks', async () => {
  const container = await mountEmbedPage();
  await act(async () => { findAddTile(container, '新建嵌入渠道')!.click(); });

  const agentSelect = container.querySelector('form select') as HTMLSelectElement;
  await act(async () => { setNativeValue(agentSelect, 'agent-1'); });
  let text = container.textContent ?? '';
  assert.ok(!text.includes('embedPublish.'), 'no raw embedPublish keys on step 1');

  await act(async () => { nextButton(container).click(); });
  const origins = container.querySelector('form textarea') as HTMLTextAreaElement;
  await act(async () => { setNativeValue(origins, 'https://shop.example.com'); });
  text = container.textContent ?? '';
  assert.ok(text.includes('域名白名单'));
  assert.ok(text.includes('每分钟请求上限'));
  assert.ok(text.includes('每日请求总上限'));

  await act(async () => { nextButton(container).click(); });
  text = container.textContent ?? '';
  assert.ok(text.includes('欢迎语'));
  assert.ok(text.includes('推荐问题'));
  assert.ok(text.includes('显示联网搜索开关'));
  assert.ok(text.includes('显示文件上传'));
  const capabilityChecks = container.querySelectorAll('form input[type="checkbox"]');
  assert.equal(capabilityChecks.length, 3, 'three capability switches');

  await act(async () => { nextButton(container).click(); });
  text = container.textContent ?? '';
  assert.ok(text.includes('主题色'));
  assert.ok(text.includes('右下角'));
  assert.ok(text.includes('跟随浏览器 / 宿主'));
  const selects = container.querySelectorAll('form select');
  assert.equal(selects.length, 3, 'header mode / position / default locale selects');
  assert.equal((selects[0] as HTMLSelectElement).options.length, 2, 'header title modes');
  assert.equal((selects[1] as HTMLSelectElement).options.length, 4, 'widget positions');
  assert.equal((selects[2] as HTMLSelectElement).options.length, 6, 'default locales');
  assert.ok(container.querySelector('form input[type="color"]'), 'primary color picker');

  await act(async () => { nextButton(container).click(); });
  text = container.textContent ?? '';
  assert.ok(text.includes('Webhook 地址'));
  assert.ok(text.includes('Webhook 签名密钥'));
  assert.ok(!text.includes('embedPublish.'), 'no raw embedPublish keys anywhere');
  assert.ok(!text.includes('integrations.wizard.'), 'no raw wizard footer keys');
  assert.ok(!text.includes('undefined'), 'no undefined leaked into the DOM');
});

test('embed drawer closes through the Vue cancel surfaces', async () => {
  const container = await mountEmbedPage();
  await act(async () => { findAddTile(container, '新建嵌入渠道')!.click(); });
  assert.ok(container.querySelector('.wk-integration-drawer'), 'drawer mounted');

  await act(async () => {
    const close = container.querySelector<HTMLButtonElement>('.wk-integration-drawer-close');
    assert.ok(close);
    close!.click();
  });
  assert.equal(container.querySelector('.wk-integration-drawer'), null, 'header close unmounts drawer');

  await act(async () => { findAddTile(container, '新建嵌入渠道')!.click(); });
  await act(async () => { window.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); });
  assert.equal(container.querySelector('.wk-integration-drawer'), null, 'Escape unmounts drawer');
});
