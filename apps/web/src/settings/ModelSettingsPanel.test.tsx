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
async function setInput(input: HTMLInputElement, value: string) {
  await act(async () => {
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
    setValue?.call(input, value);
    input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    input.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
}
async function setSelect(select: HTMLSelectElement, value: string) {
  await act(async () => {
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLSelectElement.prototype, 'value')?.set;
    setValue?.call(select, value);
    select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
}
async function click(button: Element) {
  await act(async () => button.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })));
}
async function submitForm(form: HTMLFormElement) {
  await act(async () => form.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true })));
}
async function openAddEditor(container: HTMLElement) {
  const buttons = Array.from(container.querySelectorAll('button'));
  const add = buttons.find((button) => button.textContent === '添加模型');
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
  assert.match(html, /🔒/);
  // Tabs show localized labels with counts (ModelSettings.vue lines 39-45).
  assert.match(html, /全部\(4\)/);
  assert.match(html, /对话\(2\)/);
  assert.match(html, /视觉\(1\)/);
  // Admin card actions: edit/copy/delete for tenant models, none for builtin (getModelOptions).
  assert.match(html, /复制/);
  assert.match(html, /删除/);
  assert.match(html, /模型测试/);
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
  assert.match(html, /✎/);
  assert.doesNotMatch(html, /复制<\/button>/);
  assert.doesNotMatch(html, /删除<\/button>/);
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
  const providerSelect = container.querySelector<HTMLSelectElement>('.wk-model-editor select');
  assert.ok(providerSelect);
  const optionLabels = Array.from(providerSelect.options).map((option) => option.textContent ?? '');
  assert.ok(optionLabels.some((label) => label.includes('OpenAI')));
  assert.ok(optionLabels.some((label) => label.includes('阿里云 DashScope')));
  assert.ok(optionLabels.some((label) => label.includes('自定义 (OpenAI兼容接口)')));

  // Placeholders per type/source (ModelEditorDialog.vue getModelNamePlaceholder / getBaseUrlPlaceholder).
  assert.ok(inputByPlaceholder(container, '例如：gpt-4, claude-3-opus'));
  assert.ok(inputByPlaceholder(container, '例如：https://api.openai.com/v1'));
  assert.ok(inputByPlaceholder(container, '例如：客服问答模型'));

  // Thinking control defaults to the generic provider default (resetForm line 1143).
  const thinkingSelect = Array.from(container.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'chat_template_kwargs'));
  assert.ok(thinkingSelect, 'chat + remote renders the thinking control select');
  assert.equal(thinkingSelect.value, 'chat_template_kwargs');
  assert.match(text, /自定义 OpenAI 兼容、NVIDIA NIM、vLLM \/ 本地 Qwen 部署/);

  // Context window + concurrency placeholders/desc (ModelEditorDialog.vue lines 347-398).
  assert.ok(inputByPlaceholder(container, '默认 200000'));
  assert.ok(inputByPlaceholder(container, '0 表示使用全局默认'));
  assert.match(text, /支持视觉\/多模态/);

  // Custom headers section (ModelEditorDialog.vue lines 285-306).
  assert.match(text, /自定义请求头（可选）/);
  assert.match(text, /添加请求头/);
});

test('switching provider autofills the default URL and re-syncs thinking control', async () => {
  const { client } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);

  const providerSelect = container.querySelector<HTMLSelectElement>('.wk-model-editor select');
  assert.ok(providerSelect);
  await setSelect(providerSelect, 'openai');
  const baseUrl = container.querySelector<HTMLInputElement>('input[type="url"]');
  assert.ok(baseUrl);
  assert.equal(baseUrl.value, 'https://api.openai.com/v1');

  const thinkingSelect = Array.from(container.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'chat_template_kwargs'));
  assert.ok(thinkingSelect);
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
  assert.equal(dimension.disabled, true, 'dimension stays disabled until the override toggle is on (ModelEditorDialog.vue line 325)');

  const overrideToggle = Array.from(container.querySelectorAll('input[type="checkbox"]'))
    .find((input) => input.closest('label')?.textContent?.includes('自定义输出维度'));
  assert.ok(overrideToggle);
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

  const providerSelect = container.querySelector<HTMLSelectElement>('.wk-model-editor select');
  assert.ok(providerSelect);
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
  const providerSelect = container.querySelector<HTMLSelectElement>('.wk-model-editor select');
  assert.ok(providerSelect);
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
  const providerSelect = container.querySelector<HTMLSelectElement>('.wk-model-editor select');
  assert.ok(providerSelect);
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

test('local source lists Ollama inventory with sizes and download affordances', async () => {
  const { client } = makeClient({ ollamaModels: [{ name: 'qwen2.5:0.5b', size: 512 * 1024 * 1024 }] });
  const container = await mount(client, 'admin');
  await openAddEditor(container);
  const localRadio = Array.from(container.querySelectorAll('[role="radio"]')).find((button) => button.textContent === 'Ollama');
  assert.ok(localRadio);
  await click(localRadio);
  await act(async () => {});

  const datalist = container.querySelector('datalist');
  assert.ok(datalist);
  const option = datalist.querySelector('option');
  assert.ok(option);
  assert.equal(option.value, 'qwen2.5:0.5b');
  assert.equal(option.label, '512 MB');
  const nameInput = inputByPlaceholder(container, '搜索模型...');
  assert.ok(nameInput, 'the local picker keeps the Vue search placeholder');
  await setInput(nameInput, 'qwen2.5:0.5b');
  const text = container.textContent ?? '';
  assert.match(text, /刷新列表/);
  assert.match(text, /下载: qwen2.5:0.5b/);
});

test('creating a model posts the Vue payload and shows the localized toast', async () => {
  const { client, calls } = makeClient();
  const container = await mount(client, 'admin');
  await openAddEditor(container);

  const providerSelect = container.querySelector<HTMLSelectElement>('.wk-model-editor select');
  assert.ok(providerSelect);
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
  const providerSelect = container.querySelector<HTMLSelectElement>('.wk-model-editor select');
  assert.ok(providerSelect);
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
  const providerSelect = container.querySelector<HTMLSelectElement>('.wk-model-editor select');
  assert.ok(providerSelect);
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
  const edit = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '编辑');
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
  const edit = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '编辑');
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
