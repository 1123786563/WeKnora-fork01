import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import type { ModelConfiguration, WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
// T12b models 迁移：卡面/编辑器引入 tdesign Dropdown/Popconfirm/Tooltip/Tabs
// （Popup 系），jsdom globals 扩展与 ResourceSettingsPanel.test 同款
// （settings-error-ux.test T12a d1fba03aa 先例）。
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { ModelSettingsPanel } = await import('./ModelSettingsPanel.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

type ClientOverrides = {
  providers?: (modelType: string) => Array<{ value: string; label: string; description: string; defaultUrls: Record<string, string>; modelTypes: string[] }>;
  ollamaAvailable?: boolean;
  ollamaModels?: Array<{ name: string; size?: number }>;
  wkcStatus?: Record<string, unknown>;
};
function makeClient(overrides: ClientOverrides = {}) {
  const calls = {
    create: [] as Array<Record<string, unknown>>,
    update: [] as Array<{ id: string; body: Record<string, unknown> }>,
    credentialsPut: [] as Array<{ id: string; body: Record<string, unknown> }>,
    connectionInputs: [] as Array<{ kind: string; input: Record<string, unknown> }>,
  };
  const client = {
    configuration: {
      models: {
        list: async () => [] as ModelConfiguration[],
        create: async (body: Record<string, unknown>) => {
          calls.create.push(body);
          return { id: 'model-new', name: String(body.name) } as never;
        },
        update: async (id: string, body: Record<string, unknown>) => {
          calls.update.push({ id, body });
          return { id, name: String(body.name) } as never;
        },
        remove: async () => ({}) as never,
        providers: { list: async (modelType?: string) => (overrides.providers?.(modelType ?? '') ?? []) },
        connection: {
          remote: async (input: Record<string, unknown>) => { calls.connectionInputs.push({ kind: 'remote', input }); return { available: true, message: 'ok' }; },
          embedding: async (input: Record<string, unknown>) => { calls.connectionInputs.push({ kind: 'embedding', input }); return { available: true, message: 'ok' }; },
          rerank: async (input: Record<string, unknown>) => { calls.connectionInputs.push({ kind: 'rerank', input }); return { available: true, message: 'ok' }; },
          asr: async (input: Record<string, unknown>) => { calls.connectionInputs.push({ kind: 'asr', input }); return { available: true, message: 'ok' }; },
        },
        credentials: {
          put: async (id: string, body: Record<string, unknown>) => { calls.credentialsPut.push({ id, body }); return {}; },
          remove: async () => { },
        },
        debug: async () => { throw new Error('not used'); },
      },
    },
    settings: {
      ollama: {
        status: async () => ({ available: overrides.ollamaAvailable ?? true }),
        models: async () => overrides.ollamaModels ?? [],
        download: async () => ({ task_id: 'task-1' }),
        progress: async () => ({ progress: 5, status: 'downloading' }),
        checkModels: async () => ({}),
      },
      weknoraCloud: {
        status: async () => overrides.wkcStatus ?? {},
        saveCredentials: async () => ({}),
      },
    },
  } as unknown as WeKnoraClient;
  return { client, calls };
}

async function mount(client: WeKnoraClient, role: 'viewer' | 'admin' | 'owner' | 'system-admin' = 'admin', initialModels: readonly ModelConfiguration[] = []) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<ModelSettingsPanel client={client} role={role} initialModels={initialModels} />);
  });
  return container;
}
function inputByPlaceholder(root: HTMLElement, placeholder: string) {
  return root.querySelector<HTMLInputElement>(`input[placeholder="${placeholder}"]`);
}
function modelOptionTrigger(root: HTMLElement, index = 0) {
  const trigger = root.querySelectorAll<HTMLButtonElement>('.wk-model-option-select__trigger')[index];
  assert.ok(trigger, `model option trigger ${index} renders`);
  return trigger;
}
async function setInput(input: HTMLInputElement, value: string) {
  await act(async () => {
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
    setValue?.call(input, value);
    input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    input.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
}
async function setSelect(select: HTMLSelectElement | HTMLButtonElement, value: string) {
  await act(async () => {
    if (select instanceof dom.window.HTMLButtonElement) {
      await click(select);
      const option = select.parentElement?.querySelector<HTMLButtonElement>(`[role="option"][data-value="${value}"]`);
      assert.ok(option, `model option ${value} renders`);
      await click(option);
    } else {
      const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLSelectElement.prototype, 'value')?.set;
      setValue?.call(select, value);
      select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
    }
  });
}
async function pressKey(target: EventTarget, key: string) {
  await act(async () => {
    target.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
  });
}
async function blur(target: Element) {
  await act(async () => {
    target.dispatchEvent(new dom.window.FocusEvent('focusout', { bubbles: true }));
    target.dispatchEvent(new dom.window.FocusEvent('blur'));
  });
}
async function click(button: Element) {
  await act(async () => button.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })));
}
async function submitForm(form: HTMLFormElement) {
  await act(async () => form.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true })));
}
// 添加磁贴的语义查询： localized add-model action（原 button.wk-model-card--add 钩子类已迁 Tailwind）
function findAddTile(container: HTMLElement) {
  return Array.from(container.querySelectorAll('button')).find((button) => (button.textContent ?? '').includes('添加模型')) ?? null;
}
async function openAddEditor(container: HTMLElement) {
  const add = findAddTile(container);
  assert.ok(add, 'the localized add-model action should render for admins');
  await click(add);
  assert.ok(container.querySelector('.wk-model-editor'), 'the editor should open');
}

test('model settings keeps the Vue read-only empty state for a viewer', () => {
  const html = renderToStaticMarkup(
    <ModelSettingsPanel client={{} as never} role="viewer" initialModels={[]} />,
  );
  assert.match(html, /模型配置/);
  assert.match(html, /管理不同类型的 AI 模型，支持 Ollama 本地模型和远程 API/);
  assert.match(html, /内置模型/);
  assert.match(html, /查看内置模型管理指南/);
  assert.match(html, /暂无对话模型/);
  assert.match(html, /全部\(0\)/);
  assert.doesNotMatch(html, /添加模型/);
  assert.doesNotMatch(html, /模型测试/);
});

test('model cards render Vue vendor labels, dimensions, context windows and builtin state', () => {
  const html = renderToStaticMarkup(
    <ModelSettingsPanel
      client={{} as never}
      role="admin"
      initialModels={[
        { id: 'm1', name: 'llama2', type: 'KnowledgeQA', source: 'local', parameters: {} },
        { id: 'm2', name: 'bge-m3', type: 'Embedding', source: 'remote', parameters: { provider: 'generic', dimension: 1024 } },
        { id: 'm3', name: 'gpt-4o', type: 'KnowledgeQA', source: 'remote', parameters: { provider: 'openai', context_window: 128000, supports_vision: true } },
        { id: 'm4', name: 'builtin-vlm', type: 'VLLM', source: 'remote', is_builtin: true, parameters: { provider: 'weknoracloud' } },
      ] as never}
    />,
  );
  // vendorLabel: local → Ollama, generic → 自定义, provider label → OpenAI (ModelSettings.vue vendorLabel).
  assert.match(html, /Ollama/);
  assert.match(html, /自定义/);
  assert.match(html, /OpenAI/);
  // Embedding dimension chip uses model.editor.dimensionLabel (ModelSettings.vue lines 106-109).
  assert.match(html, /向量维度 1024/);
  // Context window formatted through formatContextWindow (ModelSettings.vue formatContextWindow).
  assert.match(html, /128K/);
  assert.match(html, /title="未设置，使用默认 200K"/);
  // Vision chip + builtin lock carry their Vue titles (ModelSettings.vue lines 69-71, 118-124).
  assert.match(html, /title="支持视觉\/多模态"/);
  assert.match(html, /title="内置"/);
  // Builtin lock renders the t-icon lock-on sprite glyph (ModelSettings.vue L71,
  // t-icon name=lock-on → svg.t-icon.t-icon-lock-on).
  assert.match(html, /class="t-icon t-icon-lock-on"/);
  // Tabs show localized labels with counts (ModelSettings.vue lines 39-45,
  // t-tabs label-only panels).
  assert.match(html, /全部\(4\)/);
  assert.match(html, /对话\(2\)/);
  assert.match(html, /视觉\(1\)/);
  // Admin card actions: delete is an affix icon button; 编辑/复制 live in the
  // per-card ellipsis menu (ModelSettings.vue lines 73-102, t-dropdown).
  assert.match(html, /模型测试/);
  assert.ok(html.includes('model-card__delete'), 'delete stays an affix action');
});

test('system-admin sees builtin edit affordances only', () => {
  const html = renderToStaticMarkup(
    <ModelSettingsPanel
      client={{} as never}
      role="system-admin"
      initialModels={[
        { id: 'm4', name: 'builtin-vlm', type: 'VLLM', source: 'remote', is_builtin: true, parameters: {} },
      ] as never}
    />,
  );
  // System-admin builtin lock swaps to the t-icon edit-1 glyph
  // (ModelSettings.vue L71: isSystemAdmin ? 'edit-1' : 'lock-on').
  assert.match(html, /class="t-icon t-icon-edit-1"/);
  assert.ok(!html.includes('复制'), 'builtin cards expose no copy action');
  assert.ok(!html.includes('model-card__delete'), 'builtin cards expose no delete action');
});

test('add editor renders Vue sections, provider fallback and thinking defaults', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);

  const text = container.textContent ?? '';
  assert.match(text, /添加模型/);
  assert.match(text, /配置用于对话的大语言模型/);
  assert.match(text, /模型类型/);
  assert.match(text, /模型来源/);
  assert.match(text, /接入配置/);
  assert.match(text, /高级选项/);

  // Fallback provider catalogue localized per model.editor.providers.* (ModelEditorDialog.vue lines 505-646).
  const providerSelect = modelOptionTrigger(container);
  await click(providerSelect);
  const optionLabels = Array.from(container.querySelectorAll('[role="option"]')).map((option) => option.textContent ?? '');
  assert.ok(optionLabels.some((label) => label.includes('OpenAI')));
  assert.ok(optionLabels.some((label) => label.includes('阿里云 DashScope')));
  assert.ok(optionLabels.some((label) => label.includes('自定义 (OpenAI兼容接口)')));

  // Placeholders per type/source (ModelEditorDialog.vue getModelNamePlaceholder / getBaseUrlPlaceholder).
  assert.ok(inputByPlaceholder(container, '例如：gpt-4, claude-3-opus'));
  assert.ok(inputByPlaceholder(container, '例如：https://api.openai.com/v1'));
  assert.ok(inputByPlaceholder(container, '例如：客服问答模型'));

  // Thinking control defaults to the generic provider default (resetForm line 1143).
  const thinkingSelect = modelOptionTrigger(container, 1);
  assert.ok(thinkingSelect, 'chat + remote renders the thinking control select');
  assert.equal(thinkingSelect.value, 'chat_template_kwargs');
  // R484 G4 D4: Vue always shows the full form-desc below the select
  // (ModelEditorDialog.vue line 385), not the selected option's short hint.
  assert.match(text, /决定智能体「思考模式」开\/关时如何写入 API。已尝试按厂商\/模型预选，若与实际情况不符请按 API 文档手动修改；选「不写入」时，智能体「思考模式」开关不生效。/);

  // Context window + concurrency placeholders/desc (ModelEditorDialog.vue lines 347-398).
  assert.ok(inputByPlaceholder(container, '默认 200000'));
  assert.ok(inputByPlaceholder(container, '0 表示使用全局默认'));
  assert.match(text, /支持视觉\/多模态/);

  // Custom headers section (ModelEditorDialog.vue lines 285-306).
  assert.match(text, /自定义请求头（可选）/);
  assert.match(text, /添加请求头/);
});

// R484 G4 D4 (R482 report-B3.md D4): three add-editor micro-diffs vs the Vue
// ModelEditorDialog — (1) the provider select trigger shows only the option
// label when closed (t-select semantics; the description stays in the
// dropdown), (2) the thinking helper text is the full thinkingControlDesc
// form-desc (line 385), (3) the footer order is 测试连接 / 取消 / 保存
// (SettingDrawer footer-left + footer-right lines 50-66).
test('add editor matches the Vue provider trigger, thinking desc and footer button order', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);

  // (1) Closed trigger shows the localized label only — no description text.
  const providerTrigger = modelOptionTrigger(container);
  const triggerText = providerTrigger.textContent ?? '';
  assert.ok(triggerText.includes('自定义 (OpenAI兼容接口)'), 'the provider label renders in the closed trigger');
  assert.equal(triggerText.includes('Generic API endpoint (OpenAI-compatible)'), false, 'the provider description must stay in the dropdown only');

  // (2) The thinking helper is the full thinkingControlDesc, not the hint.
  const editorText = container.textContent ?? '';
  assert.match(editorText, /决定智能体「思考模式」开\/关时如何写入 API/);

  // (3) Footer order: 测试连接 → 取消 → 保存 like the Vue drawer footer.
  const footer = container.querySelector('.wk-model-editor .wk-list-actions:last-of-type') ?? container;
  const footerButtons = Array.from(footer.querySelectorAll('button')).map((button) => button.textContent ?? '');
  const testIndex = footerButtons.findIndex((label) => label.includes('测试连接'));
  const cancelIndex = footerButtons.findIndex((label) => label === '取消');
  const saveIndex = footerButtons.findIndex((label) => label === '保存');
  assert.ok(testIndex >= 0, 'the test-connection action renders');
  assert.ok(cancelIndex >= 0, 'the cancel action renders');
  assert.ok(saveIndex >= 0, 'the save action renders');
  assert.ok(cancelIndex < saveIndex, 'Vue footer order: 取消 before 保存');
  assert.ok(testIndex < cancelIndex, 'Vue footer order: 测试连接 before 取消');
});

test('model provider and thinking selectors keep Vue two-line options and keyboard behavior', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);

  const provider = modelOptionTrigger(container);
  await click(provider);
  assert.equal(container.querySelectorAll('select').length, 0, 'editor does not fall back to native select chrome');
  assert.ok((container.querySelector('[role="option"]')?.textContent ?? '').includes('OpenAI'));
  assert.ok(Array.from(container.querySelectorAll('[role="option"]')).some((option) => (option.textContent ?? '').includes('兼容')));
  await pressKey(provider, 'ArrowDown');
  await pressKey(provider, 'Enter');
  assert.equal(provider.value, 'openai');
  assert.equal(container.querySelector('[role="listbox"]'), null, 'selection closes the popup');

  const thinking = modelOptionTrigger(container, 1);
  await click(thinking);
  assert.ok(Array.from(container.querySelectorAll('[role="option"]')).some((option) => (option.textContent ?? '').includes('思考')));
  await pressKey(thinking, 'Escape');
  assert.equal(container.querySelector('[role="listbox"]'), null, 'escape closes the popup');
});

test('switching provider autofills the default URL and re-syncs thinking control', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);

  const providerSelect = modelOptionTrigger(container);
  await setSelect(providerSelect, 'openai');
  const baseUrl = container.querySelector<HTMLInputElement>('input[type="url"]');
  assert.ok(baseUrl);
  assert.equal(baseUrl.value, 'https://api.openai.com/v1');

  const thinkingSelect = modelOptionTrigger(container, 1);
  assert.equal(thinkingSelect.value, 'none', 'openai defaults to 不写入思考参数 (defaultThinkingControl)');

  await setSelect(providerSelect, 'aliyun');
  assert.equal(baseUrl.value, 'https://dashscope.aliyuncs.com/compatible-mode/v1');
});

test('embedding editor gates the dimension input behind the override toggle', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);

  const embeddingRadio = Array.from(container.querySelectorAll('[role="radio"]')).find((button) => button.textContent === 'Embedding');
  assert.ok(embeddingRadio);
  await click(embeddingRadio);

  const text = container.textContent ?? '';
  assert.match(text, /向量维度/);
  assert.match(text, /自定义输出维度/);
  const dimension = inputByPlaceholder(container, '例如：1536');
  assert.ok(dimension);
  assert.ok(dimension.closest('.flex.h-8'), 'advanced numeric fields use the shadcn number-input wrapper');
  assert.equal(dimension.disabled, true, 'dimension stays disabled until the override toggle is on (ModelEditorDialog.vue line 325)');

  const overrideToggle = Array.from(container.querySelectorAll('[role="switch"]'))
    .find((input) => input.getAttribute('aria-label')?.includes('自定义输出维度'));
  assert.ok(overrideToggle);
  assert.equal(overrideToggle.getAttribute('aria-checked'), 'false', 'shadcn switch keeps the Vue off state');
  await click(overrideToggle);
  assert.equal(dimension.disabled, false);
});

test('rerank locks the source to remote and swaps in signed-rerank credential fields', async () => {
  const { client } = makeClient({
    providers: (modelType) => modelType === 'rerank'
      ? [{ value: 'lkeap', label: 'LKEAP', description: 'from api', defaultUrls: {}, modelTypes: ['rerank'] }]
      : [],
  });
  const container = await mount(client, 'admin');
  await openAddEditor(container);

  const rerankRadio = Array.from(container.querySelectorAll('[role="radio"]')).find((button) => button.textContent === 'ReRank');
  assert.ok(rerankRadio);
  await click(rerankRadio);

  const text = container.textContent ?? '';
  assert.match(text, /Ollama 不支持 ReRank 模型，请使用远程接口配置/);
  const localRadio = Array.from(container.querySelectorAll('[role="radio"]')).find((button) => button.textContent === 'Ollama');
  assert.ok(localRadio);
  assert.equal((localRadio as HTMLButtonElement).disabled, true);

  const providerSelect = modelOptionTrigger(container);
  await setSelect(providerSelect, 'lkeap');

  // Signed rerank prefill + credential labels (ModelEditorDialog.vue lines 1168-1173, 757-781).
  assert.ok(inputByPlaceholder(container, '腾讯云 API 密钥 SecretId'));
  assert.ok(inputByPlaceholder(container, '腾讯云 API 密钥 SecretKey'));
  assert.match(container.textContent ?? '', /Rerank 使用腾讯云 API 签名（非 OpenAI API Key）。请在云 API 密钥控制台创建 SecretId\/SecretKey。/);
  // LKEAP region field defaults to ap-guangzhou (ModelEditorDialog.vue lines 278-282).
  const region = inputByPlaceholder(container, 'ap-guangzhou');
  assert.ok(region);
  assert.equal(region.value, 'ap-guangzhou');
  assert.match(container.textContent ?? '', /RunRerank 支持 ap-beijing、ap-guangzhou 等，默认 ap-guangzhou/);
  // Recommended rerank model prefilled on provider pick.
  const name = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus');
  assert.ok(name);
  assert.equal(name.value, 'lke-reranker-base');
});

test('weknoracloud provider gates editing on credential state', async () => {
  const wkcProvider = { value: 'weknoracloud', label: 'WeKnoraCloud', description: '', defaultUrls: {}, modelTypes: ['chat'] };
  const unconfigured = makeClient({ providers: () => [wkcProvider], wkcStatus: { needs_reinit: false, has_models: false } });
  const container = await mount(unconfigured.client, 'admin');
  await openAddEditor(container);
  const providerSelect = modelOptionTrigger(container);
  await setSelect(providerSelect, 'weknoracloud');

  const text = container.textContent ?? '';
  assert.match(text, /尚未配置 WeKnoraCloud 凭证，请先填写 APPID 和 APPSECRET。/);
  assert.match(text, /前往设置中配置/);
  // Model name and confirm stay disabled until credentials exist (ModelEditorDialog.vue lines 4, 222).
  const name = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus');
  assert.ok(name);
  assert.equal(name.disabled, true);
  const submit = Array.from(container.querySelectorAll('button')).find((button) => button.type === 'submit');
  assert.ok(submit);
  assert.equal(submit.disabled, true);
});

test('weknoracloud configured state unlocks the editor', async () => {
  const wkcProvider = { value: 'weknoracloud', label: 'WeKnoraCloud', description: '', defaultUrls: { chat: 'https://wkc.example/v1' }, modelTypes: ['chat'] };
  const configured = makeClient({ providers: () => [wkcProvider], wkcStatus: { needs_reinit: false, has_models: true } });
  const container = await mount(configured.client, 'admin');
  await openAddEditor(container);
  const providerSelect = modelOptionTrigger(container);
  await setSelect(providerSelect, 'weknoracloud');
  await act(async () => {});

  assert.match(container.textContent ?? '', /WeKnoraCloud 凭证已配置。支持的模型可参考/);
  const name = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus');
  assert.ok(name);
  assert.equal(name.disabled, false);
  const submit = Array.from(container.querySelectorAll('button')).find((button) => button.type === 'submit');
  assert.ok(submit);
  assert.equal(submit.disabled, false);
});

test('unavailable Ollama disables the local source option', async () => {
  const { client } = makeClient({ ollamaAvailable: false });
  const container = await mount(client, 'admin');
  await openAddEditor(container);
  await act(async () => {});
  const localRadio = Array.from(container.querySelectorAll('[role="radio"]')).find((button) => button.textContent === 'Ollama');
  assert.ok(localRadio);
  assert.equal((localRadio as HTMLButtonElement).disabled, true);
});

test('creating a model posts the Vue payload and shows the localized toast', async () => {
  const { client, calls } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);

  const providerSelect = modelOptionTrigger(container);
  await setSelect(providerSelect, 'openai');
  const name = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus');
  assert.ok(name);
  await setInput(name, 'gpt-4o-mini');
  await submitForm(container.querySelector('form')!);

  assert.equal(calls.create.length, 1);
  assert.deepEqual(calls.create[0], {
    name: 'gpt-4o-mini',
    display_name: '',
    description: '',
    type: 'KnowledgeQA',
    source: 'remote',
    parameters: {
      base_url: 'https://api.openai.com/v1',
      provider: 'openai',
      extra_config: { thinking_control: 'none' },
      supports_vision: false,
    },
  });
  assert.match(container.textContent ?? '', /模型已添加/);
  assert.equal(container.querySelector('.wk-model-editor'), null, 'editor closes after save');
});

test('submit gating renders the exact Vue validation copy next to the actions', async () => {
  const { client, calls } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);
  await submitForm(container.querySelector('form')!);
  assert.match(container.textContent ?? '', /模型名称不能为空/);
  assert.equal(calls.create.length, 0, 'invalid drafts never reach the API');

  const embeddingRadio = Array.from(container.querySelectorAll('[role="radio"]')).find((button) => button.textContent === 'Embedding');
  assert.ok(embeddingRadio);
  await click(embeddingRadio);
  const providerSelect = modelOptionTrigger(container);
  await setSelect(providerSelect, 'openai');
  const name = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus');
  assert.ok(name);
  await setInput(name, 'text-embedding-3-small');
  await submitForm(container.querySelector('form')!);
  assert.match(container.textContent ?? '', /Embedding 模型必须填写有效的向量维度（128-4096）/);
  assert.equal(calls.create.length, 0);
});

test('duplicate submit is blocked while a save is in flight', async () => {
  let release!: () => void;
  const gate = new Promise<void>((resolve) => { release = resolve; });
  const { client, calls } = makeClient();
  (client.configuration.models as unknown as { create: unknown }).create = async (body: Record<string, unknown>) => {
    calls.create.push(body);
    await gate;
    return { id: 'model-new', name: String(body.name) } as never;
  };
  const container = await mount(client, 'admin');
  await openAddEditor(container);
  const providerSelect = modelOptionTrigger(container);
  await setSelect(providerSelect, 'openai');
  const name = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus');
  assert.ok(name);
  await setInput(name, 'gpt-4o-mini');
  await submitForm(container.querySelector('form')!);
  await submitForm(container.querySelector('form')!);
  assert.equal(calls.create.length, 1, 'busy flag blocks a second submit (ModelEditorDialog.vue saving guard)');
  await act(async () => release());
  assert.equal(calls.create.length, 1);
});

test('edit prefill restores stored fields and routes credentials through the subresource', async () => {
  const { client, calls } = makeClient({
    providers: (modelType) => modelType === 'rerank'
      ? [{ value: 'lkeap', label: 'LKEAP', description: '', defaultUrls: {}, modelTypes: ['rerank'] }]
      : [],
  });
  const record = {
    id: 'rerank-1', name: 'lke-reranker-base', display_name: 'LKE 重排', type: 'Rerank', source: 'remote',
    parameters: { provider: 'lkeap', base_url: 'https://lkeap.example', extra_config: { region: 'ap-shanghai' } },
    credentials: { api_key: { configured: true }, app_secret: { configured: false } },
  } as never;
  const container = await mount(client, 'admin', [record]);
  const more = container.querySelector('.model-card .model-card__more');
  assert.ok(more, 'the per-card ellipsis menu trigger renders');
  await click(more);
  // t-dropdown 弹层 portal 到 body（台账 #8：不透传 attrs，走 li 文本断言）。
  const edit = Array.from(document.body.querySelectorAll('.t-dropdown__item')).find((item) => (item.textContent ?? '').includes('编辑'));
  assert.ok(edit);
  await click(edit);

  assert.match(container.textContent ?? '', /编辑模型/);
  assert.match(container.textContent ?? '', /配置用于结果重排序的模型/);
  const region = inputByPlaceholder(container, 'ap-guangzhou');
  assert.ok(region);
  assert.equal(region.value, 'ap-shanghai', 'stored extra_config.region prefills the region field');
  // Edit mode swaps the create-mode API key input for credential subresource rows.
  assert.ok(inputByPlaceholder(container, '腾讯云 API 密钥 SecretId'));
  assert.equal(inputByPlaceholder(container, '输入 API Key'), null);
  assert.ok(container.querySelector('span[title="configured"]'), 'configured api_key shows the indicator');

  const secret = inputByPlaceholder(container, '腾讯云 API 密钥 SecretKey');
  assert.ok(secret);
  await setInput(secret, 'sk-new');
  const credentialBlock = secret.closest('.form-item') ?? container;
  const saveButtons = Array.from(credentialBlock.querySelectorAll('button')).filter((button) => button.textContent === '保存');
  const save = saveButtons[saveButtons.length - 1];
  assert.ok(save, 'the app_secret row has its own save action');
  await click(save);
  assert.deepEqual(calls.credentialsPut, [{ id: 'rerank-1', body: { appSecret: 'sk-new' } }]);

  // Saving the model itself never embeds credentials (ModelSettings.vue lines 596-604).
  const submit = Array.from(container.querySelectorAll('button')).find((button) => button.type === 'submit');
  assert.ok(submit);
  await click(submit);
  assert.equal(calls.update.length, 1);
  assert.equal(calls.update[0].id, 'rerank-1');
  const body = calls.update[0].body;
  assert.equal(body.name, 'lke-reranker-base');
  assert.equal(body.display_name, 'LKE 重排');
  const parameters = body.parameters as Record<string, unknown>;
  assert.equal(parameters.provider, 'lkeap');
  assert.deepEqual(parameters.extra_config, { region: 'ap-shanghai' });
  assert.ok(!('api_key' in parameters));
  assert.ok(!('app_secret' in parameters));
});

test('connection test uses the per-type route and edit-mode modelId passthrough', async () => {
  const { client, calls } = makeClient();
  const record = {
    id: 'embed-1', name: 'bge-m3', type: 'Embedding', source: 'remote',
    parameters: { provider: 'openai', base_url: 'https://api.openai.com/v1', embedding_parameters: { dimension: 1024 } },
    credentials: { api_key: { configured: true } },
  } as never;
  const container = await mount(client, 'admin', [record]);
  const more = container.querySelector('.model-card .model-card__more');
  assert.ok(more);
  await click(more);
  const edit = Array.from(document.body.querySelectorAll('.t-dropdown__item')).find((item) => (item.textContent ?? '').includes('编辑'));
  assert.ok(edit);
  await click(edit);
  const test = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '测试连接');
  assert.ok(test);
  await click(test);
  await act(async () => {});
  assert.equal(calls.connectionInputs.length, 1);
  assert.equal(calls.connectionInputs[0].kind, 'embedding');
  assert.deepEqual(
    { ...calls.connectionInputs[0].input, customHeaders: undefined, apiKey: undefined },
    {
      source: 'remote', modelName: 'bge-m3', baseUrl: 'https://api.openai.com/v1', dimension: 1024,
      supportsDimensionOverride: false, provider: 'openai', modelId: 'embed-1',
      customHeaders: undefined, apiKey: undefined,
    },
  );
  assert.match(container.textContent ?? '', /连接成功/);
});

// B1: the Vue section header is h2 模型配置 + subtitle with a single green
// ▶ 模型测试 text trigger (ModelSettings.vue lines 3-21). No refresh button,
// no header-level add button — the add action is the dashed grid tile.
test('panel header keeps only the Vue title, subtitle and debug trigger', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => {
      root.render(<ModelSettingsPanel client={{} as never} role="admin" initialModels={[]} />);
    });
    const heading = container.querySelector('.model-settings > .section-header');
    assert.ok(heading, 'the panel renders its own Vue section-header');
    assert.equal(heading.querySelector('h2')?.textContent, '模型配置');
    assert.ok((heading.querySelector('.section-description')?.textContent ?? '').includes('管理不同类型的 AI 模型，支持 Ollama 本地模型和远程 API'));
    const headingButtons = Array.from(heading.querySelectorAll('button')).map((button) => (button.textContent ?? '').trim());
    assert.deepEqual(headingButtons, ['模型测试']);
    assert.ok(heading.querySelector('button.model-test-trigger'), 'debug trigger keeps the Vue trigger class');
    assert.ok((heading.querySelector('button.model-test-trigger') as HTMLElement | null)?.querySelector('svg.t-icon-play-circle'), 'the trigger carries the Vue play icon');
    // The dashed add tile replaces the header add button (ModelSettings.vue lines 128-139).
    const addTile = findAddTile(container);
    assert.ok(addTile, 'the dashed add tile renders for admins');
    assert.ok((addTile.textContent ?? '').includes('添加模型'));
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
});

test('model editor uses a Vue-style inner drawer overlay instead of an inline card', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);
  assert.ok(container.querySelector('.wk-model-editor-overlay'), 'Vue SettingDrawer overlay is mounted');
  assert.ok(container.querySelector('.wk-model-editor-drawer'), 'Vue editor drawer surface is mounted');
});

// B2: Vue model-card markup — type badge, title, vendor·context subtitle,
// hover/affix actions with an ellipsis menu (ModelSettings.vue lines 53-127).
test('model cards use the Vue card markup with a per-card action menu', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin', [
    { id: 'm3', name: 'gpt-4o', type: 'KnowledgeQA', source: 'remote', parameters: { provider: 'openai', context_window: 128000 } },
    { id: 'm4', name: 'builtin-vlm', type: 'VLLM', source: 'remote', is_builtin: true, parameters: { provider: 'weknoracloud' } },
  ] as never);
  try {
    const card = container.querySelector('.model-card');
    assert.ok(card, 'cards render the Vue model-card structure');
    assert.ok(card.querySelector('div[aria-label]'), 'type badge renders');
    assert.equal(card.querySelector('h3.model-card__title')?.textContent, 'gpt-4o');
    assert.ok((card.querySelector('.model-card__subtitle')?.textContent ?? '').includes('OpenAI·128K'));
    // Tenant model: the menu holds 编辑/复制; delete stays an affix action.
    await click(card.querySelector('.model-card__more')!);
    const menu = document.body.querySelector('.t-dropdown__menu');
    assert.ok(menu, 'the ellipsis menu opens (portal to body)');
    assert.ok((menu.textContent ?? '').includes('编辑'));
    assert.ok((menu.textContent ?? '').includes('复制'));
    assert.ok(card.querySelector('.model-card__delete'), 'delete stays an affix action');
    // Builtin model: lock icon, no menu, no delete (ModelSettings.vue lines 741-749).
    const builtin = Array.from(container.querySelectorAll('.model-card')).find((node) => (node.textContent ?? '').includes('builtin-vlm'));
    assert.ok(builtin);
    assert.ok(builtin.querySelector('span[title]'), 'builtin cards show the lock');
    assert.equal(builtin.querySelector('.model-card__more'), null);
    assert.equal(builtin.querySelector('.model-card__delete'), null);
  } finally {
    await act(async () => mountedRoot?.unmount());
    mountedRoot = undefined;
    container.remove();
    window.localStorage.clear();
  }
});

// B4a: combobox-style Ollama picker — editable input, suggestion dropdown with
// sizes, keyboard navigation, download option for unknown keywords
// (ModelEditorDialog.vue lines 109-133).
test('local source renders a keyboard-navigable Ollama combobox with a download option', async () => {
  const { client } = makeClient({ ollamaModels: [{ name: 'qwen2.5:0.5b', size: 512 * 1024 * 1024 }] });
  const container = await mount(client, 'admin');
  await openAddEditor(container);
  const localRadio = Array.from(container.querySelectorAll('[role="radio"]')).find((button) => button.textContent === 'Ollama');
  assert.ok(localRadio);
  await click(localRadio);
  await act(async () => {});

  const combobox = container.querySelector<HTMLInputElement>('input[role="combobox"]');
  assert.ok(combobox, 'the local picker is an editable combobox input');
  assert.equal(combobox.getAttribute('role'), 'combobox');
  assert.equal(combobox.getAttribute('aria-expanded'), 'false');
  await setInput(combobox, 'qwe');
  const listbox = container.querySelector('[role="listbox"]');
  assert.ok(listbox, 'typing opens the suggestion dropdown');
  assert.equal(combobox.getAttribute('aria-expanded'), 'true');
  const option = listbox.querySelector('[role="option"]');
  assert.ok(option);
  assert.ok((option.textContent ?? '').includes('qwen2.5:0.5b'));
  assert.ok((option.textContent ?? '').includes('512 MB'));

  // Keyboard nav: the first suggestion starts highlighted; ArrowDown/ArrowUp
  // move the highlight and Enter selects (ModelEditorDialog.vue filterable
  // select keyboard behavior).
  assert.ok(listbox.querySelector('[role="option"][aria-selected="true"]'), 'the first suggestion starts highlighted');
  await pressKey(combobox, 'ArrowDown');
  assert.ok(listbox.querySelector('[role="option"][aria-selected="true"]'), 'ArrowDown moves the highlight');
  await pressKey(combobox, 'ArrowUp');
  assert.ok((listbox.querySelector('[role="option"][aria-selected="true"]')?.textContent ?? '').includes('qwen2.5:0.5b'), 'ArrowUp returns to the model row');
  await pressKey(combobox, 'Enter');
  assert.equal(combobox.value, 'qwen2.5:0.5b', 'Enter selects the highlighted suggestion');
  assert.equal(container.querySelector('[role="listbox"]'), null, 'selection closes the dropdown');

  // Unknown keyword offers the Vue download option (ModelEditorDialog.vue line 124).
  await setInput(combobox, 'gemma3:1b');
  const downloadOption = Array.from(container.querySelectorAll('[role="option"]')).find((option) => (option.textContent ?? '').includes('下载')) ?? null;
  assert.ok(downloadOption);
  assert.ok((downloadOption.textContent ?? '').includes('下载: gemma3:1b'));
  await pressKey(combobox, 'Escape');
  assert.equal(container.querySelector('[role="listbox"]'), null, 'Escape closes the dropdown');
  const refresh = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '刷新列表');
  assert.ok(refresh, 'the Vue refresh action stays next to the picker');
});

// B4b: per-field blur validation with per-field messages
// (ModelEditorDialog.vue rules lines 907-946).
test('name and base URL validate on blur with per-field Vue copy', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);
  const providerSelect = modelOptionTrigger(container);
  await setSelect(providerSelect, 'openai');

  const name = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus')!;
  assert.ok(name);
  await blur(name);
  let fieldError = name.closest('.form-item')?.querySelector('.wk-field-error');
  assert.ok(fieldError, 'the name error renders next to the field');
  assert.equal(fieldError.textContent, '请输入模型名称');

  await setInput(name, '模型'.repeat(60));
  await blur(name);
  fieldError = name.closest('.form-item')?.querySelector('.wk-field-error');
  assert.equal(fieldError?.textContent, '模型名称不能超过100个字符');

  const baseUrl = container.querySelector<HTMLInputElement>('input[type="url"]')!;
  assert.ok(baseUrl);
  await setInput(baseUrl, '');
  await blur(baseUrl);
  fieldError = baseUrl.closest('label')?.querySelector('.wk-field-error');
  assert.equal(fieldError?.textContent, '请输入 Base URL');

  await setInput(baseUrl, 'not-a-url');
  await blur(baseUrl);
  fieldError = baseUrl.closest('label')?.querySelector('.wk-field-error');
  assert.equal(fieldError?.textContent, 'Base URL 格式不正确，请输入有效的 URL');

  await setInput(baseUrl, 'https://api.openai.com/v1');
  await blur(baseUrl);
  await setInput(name, 'gpt-4o-mini');
  await blur(name);
  assert.equal(container.querySelector('.wk-field-error'), null, 'fixing a field clears its error');
});

// B4c: ESC preserves the add draft for the next add open; explicit cancel
// discards it (ModelEditorDialog.vue visible watcher lines 1063-1104 and
// handleCancel lines 1715-1719).
test('ESC keeps the add draft, cancel discards it', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin');

  await openAddEditor(container);
  const name = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus')!;
  await setInput(name, 'my-drafted-model');
  await pressKey(document.body, 'Escape');
  assert.equal(container.querySelector('.wk-model-editor'), null, 'ESC closes the editor');

  const add = findAddTile(container);
  assert.ok(add);
  await click(add);
  const restored = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus')!;
  assert.equal(restored.value, 'my-drafted-model', 'the add draft is restored on reopen');

  const cancel = Array.from(container.querySelectorAll('.wk-model-editor button')).find((button) => button.textContent === '取消');
  assert.ok(cancel);
  await click(cancel);
  assert.equal(container.querySelector('.wk-model-editor'), null);
  await click(add);
  const fresh = inputByPlaceholder(container, '例如：gpt-4, claude-3-opus')!;
  assert.equal(fresh.value, '', 'cancel discards the draft (handleCancel resets the form)');
});

// B4d: the settings sub-section deep link preselects the type tab
// (ModelSettings.vue watches uiStore.settingsInitialSubSection, lines 329-337).
test('an initial sub-section preselects the matching type tab', () => {
  const html = renderToStaticMarkup(
    <ModelSettingsPanel client={{} as never} role="admin" initialModels={[
      { id: 'm1', name: 'bge-m3', type: 'Embedding', source: 'remote', parameters: {} },
      { id: 'm2', name: 'gpt-4o', type: 'KnowledgeQA', source: 'remote', parameters: { provider: 'openai' } },
    ] as never} initialSubSection="embedding" />,
  );
  const tabs = html.slice(html.indexOf('model-type-tabs'));
  const activeAt = tabs.indexOf('t-is-active');
  assert.ok(activeAt >= 0, 'a tab is active');
  const activeLabel = tabs.slice(activeAt, activeAt + 220);
  assert.ok(activeLabel.includes('Embedding(1)'), 'the embedding tab is the active one');
});
