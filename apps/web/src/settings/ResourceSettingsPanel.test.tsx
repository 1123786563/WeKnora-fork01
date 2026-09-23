import assert from 'node:assert/strict';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import * as nodeModule from 'node:module';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => (specifier.endsWith('.css') || specifier.endsWith('.svg')) ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
// T12b：websearch 卡引入 tdesign Dropdown（Popup），jsdom globals 扩展与
// settings-error-ux.test 同款（T12a d1fba03aa 先例）。
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

// react-dom/client must load AFTER the jsdom globals exist — its module init
// feature-detects the DOM and silently skips event delegation otherwise
// (same ordering as SandboxSettingsPanel.test.tsx).
const { createRoot } = await import('react-dom/client');
const { ResourceSettingsPanel } = await import('./ResourceSettingsPanel.tsx');

const client = { settings: { webSearch: { providers: {
  list: async () => [], create: async () => ({}), update: async () => ({}), remove: async () => ({}), testById: async () => ({ success: true }),
} } } } as never;

test('WebSearch provider cards preserve Vue metadata anatomy', () => {
  const html = renderToStaticMarkup(React.createElement(ResourceSettingsPanel, {
    client,
    section: 'websearch',
    initialValue: [{
      id: 'provider-1',
      name: 'Docs Search',
      provider: 'tavily',
      description: 'Search public documentation',
      parameters: { proxy_url: 'https://proxy.example.test' },
    }],
  }));

  // Vue StorageBackendSettings backend-card anatomy shared by all three
  // resource surfaces: badge initial, name, provider·meta subtitle, and the
  // dashed add card.
  assert.match(html, /backend-card/);
  assert.match(html, /Docs Search/);
  assert.match(html, /tavily/);
  assert.match(html, /Search public documentation/);
  assert.match(html, /role="button"/);
  assert.match(html, /backend-card--add/);
  assert.match(html, /添加搜索引擎/);
});

test('WebSearch provider cards open the edit drawer instead of inline actions', () => {
  const html = renderToStaticMarkup(React.createElement(ResourceSettingsPanel, {
    client,
    section: 'websearch',
    initialValue: [{ id: 'provider-1', name: 'Docs Search', provider: 'tavily' }],
  }));

  // Vue keeps card actions inside the drawer (create/edit titles) — the
  // card itself is a button; no always-visible 编辑/删除 labels.
  assert.doesNotMatch(html, /aria-label="编辑 Docs Search"/);
  assert.doesNotMatch(html, /aria-label="删除 Docs Search"/);
  assert.match(html, /添加搜索引擎/);
});

// ---------------------------------------------------------------------------
// R485 H2 — structured add/edit drawers (R482 B3-D1). The bare
// name/type/JSON form is replaced by the per-engine field forms from
// VectorStoreSettings.vue / StorageBackendSettings.vue / WebSearchSettings.vue.

type TestResult = { success: boolean; error?: string; message?: string };
type CallLog = Array<{ method: string; path: string; body?: unknown }>;

function makeSectionClient(section: 'storage' | 'vectorstore' | 'websearch', routes: { list?: unknown[]; types?: unknown[]; envelope?: { rows: unknown[]; defaultId?: string } } = {}) {
  const calls: CallLog = [];
  const created: unknown[] = [];
  const updated: Array<{ id: string; input: unknown }> = [];
  const tested: unknown[] = [];
  const sectionApi = {
    list: async () => { calls.push({ method: 'GET', path: 'list' }); return routes.list ?? []; },
    ...(routes.envelope ? { listWithEnvelope: async () => { calls.push({ method: 'GET', path: 'list' }); return routes.envelope; } } : {}),
    types: async () => { calls.push({ method: 'GET', path: 'types' }); return routes.types ?? []; },
    create: async (input: unknown) => { calls.push({ method: 'POST', path: 'create', body: input }); created.push(input); return { id: 'new-id', ...(input as object) }; },
    update: async (id: string, input: unknown) => { calls.push({ method: 'PUT', path: `update/${id}`, body: input }); updated.push({ id, input }); return { id, ...(input as object) }; },
    remove: async (id: string) => { calls.push({ method: 'DELETE', path: `remove/${id}` }); return { success: true }; },
    test: async (input: unknown) => { calls.push({ method: 'POST', path: 'test', body: input }); tested.push(input); return { success: true, message: 'ok' }; },
    testById: async (id: string) => { calls.push({ method: 'POST', path: `testById/${id}` }); tested.push({ id }); return { success: true }; },
    // Credential subresource (Vue WebSearchSettings credentialApi).
    putCredentials: async (id: string, input: unknown) => { calls.push({ method: 'PUT', path: `credentials/${id}`, body: input }); return { fields: { api_key: { configured: true } } }; },
    deleteCredential: async (id: string, field: string) => { calls.push({ method: 'DELETE', path: `credentials/${id}/${field}` }); return { success: true }; },
  };
  const settings = section === 'storage'
    ? { storage: { backends: sectionApi } }
    : section === 'vectorstore'
      ? { vectorStores: sectionApi }
      : { webSearch: { providers: sectionApi } };
  return { client: { settings } as never, calls, created, updated, tested };
}

const vectorTypes = [{
  type: 'elasticsearch', display_name: 'Elasticsearch',
  connection_fields: [
    { name: 'addr', type: 'string', required: true },
    { name: 'username', type: 'string', required: false },
    { name: 'password', type: 'string', required: false, sensitive: true },
  ],
  index_fields: [{ name: 'number_of_shards', type: 'number', required: false, min: 1, max: 64 }],
}, {
  type: 'sqlite', display_name: 'SQLite (built-in)',
  connection_fields: [], index_fields: [],
}];

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => { mountedRoot?.unmount(); });
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

async function mountPanel(client: never, section: 'storage' | 'vectorstore' | 'websearch', initialValue: unknown = []) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(ResourceSettingsPanel, { client, section, initialValue }));
  });
  // Types load asynchronously (like every other drawer data fetch) — flush
  // the micro/tick queue so both the row list and the type metadata land.
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  return container;
}

function findButton(root: ParentNode, text: string): HTMLElement {
  // Resource cards are <article role="button">, drawer actions are <button>.
  const button = Array.from(root.querySelectorAll('button, [role="button"]'))
    .find((entry) => entry.textContent?.includes(text));
  assert.ok(button, `expected a button containing "${text}"`);
  return button as HTMLElement;
}

function findLabelledControl(root: ParentNode, label: string): HTMLInputElement | HTMLSelectElement {
  const labelEl = Array.from(root.querySelectorAll('label')).find((entry) => entry.textContent?.replace('*', '').trim() === label || entry.textContent?.includes(label));
  assert.ok(labelEl, `expected a field labelled "${label}"`);
  const control = labelEl.querySelector('input, select');
  assert.ok(control, `expected an input under label "${label}"`);
  return control as HTMLInputElement | HTMLSelectElement;
}

/* S6：tdesign Select 无原生 select（弹层 portal 到 body，台账 #8），经触发器
   点击 + li.t-select-option 文本点击驱动。 */
async function chooseSelectOption(root: ParentNode, label: string, optionText: string) {
  const labelEl = Array.from(root.querySelectorAll('label')).find((entry) => entry.textContent?.replace('*', '').trim() === label || entry.textContent?.includes(label));
  assert.ok(labelEl, `expected a field labelled "${label}"`);
  const trigger = labelEl.querySelector('.t-select__wrap');
  assert.ok(trigger, `expected a tdesign select under label "${label}"`);
  await act(async () => trigger.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })));
  const option = Array.from(document.body.querySelectorAll('.t-select-option'))
    .find((node) => (node.textContent ?? '').trim() === optionText);
  assert.ok(option, `expected the ${optionText} option in the select popup`);
  await act(async () => option.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })));
}

function setInputValue(input: HTMLInputElement, value: string) {
  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  setValue?.call(input, value);
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
}

function setSelectValue(select: HTMLSelectElement, value: string) {
  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLSelectElement.prototype, 'value')?.set;
  setValue?.call(select, value);
  select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

function submitForm(root: ParentNode) {
  const form = root.querySelector('form');
  assert.ok(form, 'expected the drawer form');
  return act(async () => {
    form.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true }));
  });
}

test('vectorstore add drawer renders the Vue structured form, not a JSON textarea', async () => {
  const { client } = makeSectionClient('vectorstore', { types: vectorTypes });
  const container = await mountPanel(client as never, 'vectorstore');

  await act(async () => { findButton(container, '添加数据库').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;
  assert.ok(drawer, 'the add drawer should open');

  // Section titles + structured fields, mirroring the Vue create drawer.
  assert.match(drawer.textContent!, /基本信息/);
  assert.match(drawer.textContent!, /连接信息/);
  assert.match(drawer.textContent!, /引擎类型/);
  // S6：tdesign Select 关闭态只显示已选 label（原生 select 会把全部 option 渲染
  // 进 DOM），选项断言改开弹层。
  await act(async () => {
    const label = Array.from(drawer.querySelectorAll('label')).find((entry) => (entry.textContent ?? '').includes('引擎类型'))!;
    label.querySelector('.t-select__wrap')?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  assert.ok(Array.from(document.body.querySelectorAll('.t-select-option')).some((node) => (node.textContent ?? '').includes('Elasticsearch')));
  assert.match(drawer.textContent!, /addr/);
  assert.match(drawer.textContent!, /password/);
  // Advanced index config section exists (collapsed by default like Vue).
  assert.match(drawer.textContent!, /高级设置/);
  // Test connection lives in the drawer footer with the save/cancel pair.
  assert.match(drawer.textContent!, /测试连接/);
  // R490 C3 (M3-N1) — footer order mirrors Vue SettingDrawer (L45-61):
  // footer-left 测试连接, right pair 取消 → 保存; the create submit reads
  // common.save (Vue never overrides confirmText in the engine drawers).
  // The drawer TITLE still reuses the panel add label (Vue addStore).
  // S6：disabled 的 tdesign Button 渲染 div.t-button（台账 #7），经类查询。
  const footerButtons = Array.from(drawer.querySelectorAll('.t-button'))
    .filter((button) => button.closest('form'))
    .map((button) => (button.textContent ?? '').trim())
    .filter((text) => ['测试连接', '取消', '保存'].includes(text));
  assert.deepEqual(footerButtons, ['测试连接', '取消', '保存'], 'footer reads 测试连接/取消/保存 in the Vue SettingDrawer order');
  assert.equal(drawer.querySelector('form button[type="submit"]')?.textContent, '保存', 'create submit is 保存, not the panel add label');
  // The legacy bare form is gone.
  assert.doesNotMatch(drawer.textContent!, /安全配置 JSON/);
  assert.equal(drawer.querySelector('textarea'), null);
  // Password-type connection field renders as a password input.
  const passwordInput = Array.from(drawer.querySelectorAll('input[type="password"]'));
  assert.ok(passwordInput.length >= 1, 'sensitive connection fields render as password inputs');
});

test('vectorstore add drawer submits the Vue payload shape and tests the live connection', async () => {
  const { client, calls, created, tested } = makeSectionClient('vectorstore', { types: vectorTypes });
  const container = await mountPanel(client as never, 'vectorstore');

  await act(async () => { findButton(container, '添加数据库').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;

  const name = findLabelledControl(drawer, '名称') as HTMLInputElement;
  setInputValue(name, 'Primary ES');
  const addr = findLabelledControl(drawer, 'addr') as HTMLInputElement;
  setInputValue(addr, 'https://es.example.com:9200');

  // Test connection fires POST /vector-stores/test with the typed form data.
  await act(async () => { findButton(drawer, '测试连接').click(); });
  assert.deepEqual(tested, [{ engine_type: 'elasticsearch', connection_config: { addr: 'https://es.example.com:9200' } }]);

  await submitForm(drawer);
  assert.deepEqual(created, [{
    name: 'Primary ES',
    engine_type: 'elasticsearch',
    connection_config: { addr: 'https://es.example.com:9200' },
    index_config: {},
  }]);
  assert.equal(calls.some((call) => call.method === 'POST' && call.path === 'create'), true);
});

test('vectorstore edit drawer is name-only with the immutable notice (Vue edit mode)', async () => {
  const { client, updated } = makeSectionClient('vectorstore', {
    types: vectorTypes,
    list: [{ id: 'store-1', name: 'Primary ES', engine_type: 'elasticsearch', source: 'user', connection_config: { addr: 'https://es:9200', username: 'elastic' }, index_config: { number_of_shards: 2 } }],
  });
  const container = await mountPanel(client as never, 'vectorstore');

  await act(async () => { findButton(container, 'Primary ES').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;
  assert.match(drawer.textContent!, /创建后无法更改引擎类型、连接和索引设置。/);
  // Read-only rows show the stored connection config.
  assert.match(drawer.textContent!, /https:\/\/es:9200/);

  const name = findLabelledControl(drawer, '名称') as HTMLInputElement;
  setInputValue(name, 'Renamed ES');
  await submitForm(drawer);
  assert.deepEqual(updated, [{ id: 'store-1', input: { name: 'Renamed ES' } }]);
});

test('storage add drawer renders the Vue sections and submits {name, provider, config}', async () => {
  const { client, created } = makeSectionClient('storage', { types: ['local', 'minio', 's3', 'cos'], envelope: { rows: [], defaultId: '' } });
  const container = await mountPanel(client as never, 'storage');

  await act(async () => { findButton(container, '添加存储实例').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;

  assert.match(drawer.textContent!, /基本信息/);
  assert.match(drawer.textContent!, /存储类型/);
  assert.match(drawer.textContent!, /连接配置/);
  assert.match(drawer.textContent!, /高级选项/);
  assert.match(drawer.textContent!, /路径前缀/);
  assert.match(drawer.textContent!, /测试连接/);
  assert.doesNotMatch(drawer.textContent!, /安全配置 JSON/);

  const name = findLabelledControl(drawer, '名称') as HTMLInputElement;
  setInputValue(name, 'Prod S3');
  const provider = findLabelledControl(drawer, '存储类型') as HTMLSelectElement;
  await chooseSelectOption(drawer, '存储类型', 'S3');
  // S3 connection fields: Endpoint / Region / keys / Bucket.
  const endpoint = findLabelledControl(drawer, 'Endpoint') as HTMLInputElement;
  setInputValue(endpoint, 'https://s3.amazonaws.com');
  const region = findLabelledControl(drawer, 'Region') as HTMLInputElement;
  setInputValue(region, 'us-east-1');
  const accessKey = findLabelledControl(drawer, 'Access Key / Secret ID') as HTMLInputElement;
  setInputValue(accessKey, 'AKIA-test');
  const secret = findLabelledControl(drawer, 'Secret Key') as HTMLInputElement;
  setInputValue(secret, 'secret-key');
  const bucket = findLabelledControl(drawer, 'Bucket') as HTMLInputElement;
  setInputValue(bucket, 'weknora-files');

  await submitForm(drawer);
  assert.equal(created.length, 1);
  assert.deepEqual(created[0], {
    name: 'Prod S3',
    provider: 's3',
    config: {
      mode: 'remote',
      endpoint: 'https://s3.amazonaws.com',
      region: 'us-east-1',
      access_key_id: 'AKIA-test',
      secret_access_key: 'secret-key',
      bucket_name: 'weknora-files',
      path_prefix: '',
      use_ssl: true,
    },
  });
});

test('storage add drawer switches MinIO deploy mode like the Vue segmented pills', async () => {
  const { client, created } = makeSectionClient('storage', { types: ['local', 'minio'], envelope: { rows: [], defaultId: '' } });
  const container = await mountPanel(client as never, 'storage');

  await act(async () => { findButton(container, '添加存储实例').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;
  // The deploy-mode pills only render for MinIO — switch the provider first.
  await chooseSelectOption(drawer, '存储类型', 'MINIO');
  assert.match(drawer.textContent!, /部署模式/);
  assert.match(drawer.textContent!, /远程实例/);
  assert.match(drawer.textContent!, /环境变量/);

  // env mode hides endpoint + credentials for MinIO (needsCredentials=false).
  await act(async () => { findButton(drawer, '环境变量').click(); });
  assert.equal(Array.from(drawer.querySelectorAll('label')).some((entry) => entry.textContent?.includes('Endpoint')), false);

  const name = findLabelledControl(drawer, '名称') as HTMLInputElement;
  setInputValue(name, 'Compose MinIO');
  await submitForm(drawer);
  assert.deepEqual((created[0] as { config: Record<string, unknown> }).config, {
    mode: 'docker', endpoint: '', region: '', access_key_id: '', secret_access_key: '',
    bucket_name: '', path_prefix: '', use_ssl: true,
  });
});

test('storage edit drawer keeps connection fields read-only and tests by id', async () => {
  const { client, updated, tested } = makeSectionClient('storage', {
    types: ['local', 's3'],
    envelope: {
      rows: [{ id: 'backend-1', name: 'Prod S3', provider: 's3', source: 'user', config: { endpoint: 'https://s3.amazonaws.com', region: 'us-east-1', bucket_name: 'weknora' } }],
      defaultId: '',
    },
  });
  const container = await mountPanel(client as never, 'storage');

  await act(async () => { findButton(container, 'Prod S3').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;
  const endpoint = findLabelledControl(drawer, 'Endpoint') as HTMLInputElement;
  assert.equal(endpoint.disabled, true);

  // Vue edit mode tests via POST /storage-backends/{id}/test.
  await act(async () => { findButton(drawer, '测试连接').click(); });
  assert.deepEqual(tested, [{ id: 'backend-1' }]);

  const name = findLabelledControl(drawer, '名称') as HTMLInputElement;
  setInputValue(name, 'Prod S3 renamed');
  await submitForm(drawer);
  assert.equal(updated.length, 1);
  const input = updated[0]!.input as { name: string; provider: string; config: Record<string, unknown> };
  assert.equal(input.name, 'Prod S3 renamed');
  assert.equal(input.provider, 's3');
  assert.equal(input.config.endpoint, 'https://s3.amazonaws.com');
});

const webSearchTypes = [
  { id: 'brave', name: 'Brave Search', requires_api_key: true, requires_engine_id: false, requires_base_url: false, supports_proxy: true, docs_url: 'https://brave.com/search/api/' },
  { id: 'searxng', name: 'SearXNG', requires_api_key: false, requires_engine_id: false, requires_base_url: true, supports_proxy: true },
];

test('websearch add drawer renders the Vue sections with proxy + default options', async () => {
  const { client } = makeSectionClient('websearch', { types: webSearchTypes });
  const container = await mountPanel(client as never, 'websearch');

  await act(async () => { findButton(container, '添加搜索引擎').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;

  assert.match(drawer.textContent!, /基本信息/);
  assert.match(drawer.textContent!, /引擎类型/);
  assert.match(drawer.textContent!, /Brave Search/);
  assert.match(drawer.textContent!, /备注/);
  assert.match(drawer.textContent!, /连接配置/);
  assert.match(drawer.textContent!, /API 密钥/);
  assert.match(drawer.textContent!, /选项/);
  assert.match(drawer.textContent!, /HTTP 代理/);
  assert.match(drawer.textContent!, /设为默认/);
  assert.match(drawer.textContent!, /测试连接/);
  assert.doesNotMatch(drawer.textContent!, /安全配置 JSON/);
});

test('websearch add drawer submits the Vue payload and carries the api_key on create', async () => {
  const { client, created, tested } = makeSectionClient('websearch', { types: webSearchTypes });
  const container = await mountPanel(client as never, 'websearch');

  await act(async () => { findButton(container, '添加搜索引擎').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;

  const name = findLabelledControl(drawer, '名称') as HTMLInputElement;
  setInputValue(name, 'Prod Brave');
  const desc = findLabelledControl(drawer, '备注') as HTMLInputElement;
  setInputValue(desc, 'notes');
  const apiKey = findLabelledControl(drawer, 'API 密钥') as HTMLInputElement;
  setInputValue(apiKey, 'BSA-key-1');
  const proxy = findLabelledControl(drawer, 'HTTP 代理') as HTMLInputElement;
  setInputValue(proxy, 'http://127.0.0.1:7890');

  // Test connection posts the live form (provider + parameters).
  await act(async () => { findButton(drawer, '测试连接').click(); });
  assert.deepEqual(tested, [{
    provider: 'brave',
    parameters: { api_key: 'BSA-key-1', engine_id: '', base_url: '', proxy_url: 'http://127.0.0.1:7890', extra_config: {} },
  }]);

  await submitForm(drawer);
  // Vue openAddDialog defaults is_default to true while the tenant has no
  // providers yet (WebSearchSettings.vue L539).
  assert.deepEqual(created, [{
    name: 'Prod Brave',
    provider: 'brave',
    description: 'notes',
    parameters: { engine_id: '', base_url: '', proxy_url: 'http://127.0.0.1:7890', api_key: 'BSA-key-1' },
    is_default: true,
  }]);
});

test('websearch edit drawer locks the provider and never posts the api_key', async () => {
  const { client, updated } = makeSectionClient('websearch', {
    types: webSearchTypes,
    list: [{ id: 'provider-1', name: 'Prod Brave', provider: 'brave', description: 'notes', parameters: { proxy_url: 'http://127.0.0.1:7890' }, is_default: false }],
  });
  const container = await mountPanel(client as never, 'websearch');

  await act(async () => { findButton(container, 'Prod Brave').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;
  // S6：tdesign Select disabled 态走 t-is-disabled 类（台账 #7 同口径）。
  const providerLabel = Array.from(drawer.querySelectorAll('label')).find((entry) => (entry.textContent ?? '').includes('引擎类型'))!;
  assert.ok(providerLabel.querySelector('.t-is-disabled'), 'the provider select stays locked in edit mode');

  // Editing without retyping the key posts no api_key anywhere.
  await submitForm(drawer);
  assert.equal(updated.length, 1);
  const input = updated[0]!.input as { provider: string; parameters: Record<string, unknown> };
  assert.equal(input.provider, 'brave');
  assert.equal('api_key' in input.parameters, false);
  assert.equal(input.parameters.proxy_url, 'http://127.0.0.1:7890');
});

// ---------------------------------------------------------------------------
// R486 J2 — R485 H2 debt 1: websearch edit mode manages the api_key through
// the /credentials subresource with the Vue CredentialResource card
// (configured badge / inline confirm-remove / per-field replace), instead of
// a plain password input piggybacked on the main save.

const credentialedProvider = {
  id: 'provider-1', name: 'Prod Brave', provider: 'brave', description: 'notes',
  parameters: { proxy_url: 'http://127.0.0.1:7890' },
  credentials: { api_key: { configured: true } },
  is_default: false,
};

function findCredentialRow(drawer: Element, kind: string): HTMLElement {
  const row = drawer.querySelector(`[data-kind="${kind}"]`);
  assert.ok(row, `expected the credential card row [data-kind="${kind}"]`);
  return row as HTMLElement;
}

test('websearch edit drawer replaces the api_key through the /credentials subresource', async () => {
  const { client, calls, updated } = makeSectionClient('websearch', {
    types: webSearchTypes,
    list: [credentialedProvider],
  });
  const container = await mountPanel(client as never, 'websearch');

  await act(async () => { findButton(container, 'Prod Brave').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;

  // Edit mode shows the configured credential card, not a password input.
  const configured = findCredentialRow(drawer, 'configured');
  assert.match(configured.textContent!, /已配置/);
  assert.equal(drawer.querySelectorAll('input[type="password"]').length, 0);

  // 更换 expands an explicit input + 保存/取消 pair (Vue CredentialResource
  // editing state) — the commit is its own PUT, not the main form submit.
  await act(async () => { findButton(configured, '更换').click(); });
  const draft = drawer.querySelector('input[type="password"]') as HTMLInputElement;
  assert.ok(draft, 'editing state exposes a password input');
  assert.equal(draft.placeholder, '请输入');
  setInputValue(draft, 'BSA-rotated');
  await act(async () => { findButton(drawer, '保存').click(); });

  const credentialPut = calls.filter((call) => call.method === 'PUT' && call.path === 'credentials/provider-1');
  assert.deepEqual(credentialPut, [{ method: 'PUT', path: 'credentials/provider-1', body: { api_key: 'BSA-rotated' } }]);
  // The save's returned meta flips the card back to configured.
  findCredentialRow(drawer, 'configured');

  // The main save never carries the credential — one credentials PUT total.
  await submitForm(drawer);
  assert.equal(calls.filter((call) => call.path === 'credentials/provider-1').length, 1);
  assert.equal(updated.length, 1);
  assert.equal('api_key' in (updated[0]!.input as { parameters: Record<string, unknown> }).parameters, false);
});

test('websearch edit drawer removes the api_key with an inline confirm, then reconfigures', async () => {
  const { client, calls } = makeSectionClient('websearch', {
    types: webSearchTypes,
    list: [credentialedProvider],
  });
  const container = await mountPanel(client as never, 'websearch');

  await act(async () => { findButton(container, 'Prod Brave').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;

  // First 移除 click only flips the row into the danger confirm standoff —
  // no DELETE until the second deliberate click (Vue two-step remove).
  const configured = findCredentialRow(drawer, 'configured');
  await act(async () => { findButton(configured, '移除').click(); });
  const confirmRow = findCredentialRow(drawer, 'confirm-remove');
  assert.match(confirmRow.textContent!, /确认移除？此操作不可撤销/);
  assert.equal(calls.some((call) => call.method === 'DELETE'), false);

  await act(async () => { findButton(confirmRow, '确认移除').click(); });
  assert.deepEqual(calls.filter((call) => call.method === 'DELETE'), [{ method: 'DELETE', path: 'credentials/provider-1/api_key' }]);
  // The row flips to unconfigured and flashes the anchored removed toast —
  // Vue swaps the placeholder text for removedToast during the flash window
  // and hides the Configure affordance until the flash clears.
  const unconfigured = findCredentialRow(drawer, 'unconfigured');
  assert.match(unconfigured.textContent!, /凭据已移除/);
});

test('websearch edit drawer with an unconfigured credential opens editing via 配置', async () => {
  const { client } = makeSectionClient('websearch', {
    types: webSearchTypes,
    list: [{ ...credentialedProvider, credentials: { api_key: { configured: false } } }],
  });
  const container = await mountPanel(client as never, 'websearch');

  await act(async () => { findButton(container, 'Prod Brave').click(); });
  const drawer = document.body.querySelector('.t-drawer')!;
  const unconfigured = findCredentialRow(drawer, 'unconfigured');
  assert.match(unconfigured.textContent!, /未配置/);

  // 配置 re-enters the editing state from the unconfigured row.
  await act(async () => { findButton(unconfigured, '配置').click(); });
  assert.ok(drawer.querySelector('input[type="password"]'), '配置 re-opens the editing input');
});

// ---------------------------------------------------------------------------
// R486 J2 — R485 H2 debt 2: per-engine brand badge in the drawer header
// (Vue SettingDrawer #headerIcon) for all three resource sections.

test('resource drawers render the per-engine brand badge in the header', async () => {
  // Websearch: the monogram initial comes from the type display name (Vue
  // providerInitial looks providerTypes up by id) and tint from the
  // websearch brand table (zhipu → #2563EB, WebSearchSettings.vue L1200;
  // jsdom normalizes the hex to rgb() in serialized inline styles).
  const web = makeSectionClient('websearch', { types: [{ id: 'zhipu', name: '智谱搜索', requires_api_key: true }] });
  const webContainer = await mountPanel(web.client as never, 'websearch');
  await act(async () => { findButton(webContainer, '添加搜索引擎').click(); });
  let drawer = document.body.querySelector('.t-drawer')!;
  // S6：tdesign Drawer header 容器为 .t-drawer__header（原 Sheet 的 header/h2
  // 不复存在），徽章+标题由自定义 header prop 渲染。
  const webHeader = drawer.querySelector('.t-drawer__header')!;
  assert.match(webHeader.textContent!, /智/);
  assert.match(webHeader.innerHTML, /37, 99, 235/);
  const badgeIndex = webHeader.innerHTML.indexOf('智');
  const titleIndex = webHeader.textContent!.indexOf('添加');
  assert.ok(badgeIndex > -1 && titleIndex > -1, 'badge and title render in the header');

  // S6：tdesign Drawer 的 portal 子树由 React 托管，手动 remove 会破坏后续
  // unmount（"node to be removed is not a child"）——直接卸载根即可。
  await act(async () => { mountedRoot?.unmount(); });
  mountedRoot = undefined;

  // Vectorstore: initial comes from the raw engine_type (Vue engineInitial)
  // and tint from the vectorstore brand table (elasticsearch → #D97706 →
  // jsdom-normalized rgb(217, 119, 6)).
  const vector = makeSectionClient('vectorstore', { types: vectorTypes });
  const vectorContainer = await mountPanel(vector.client as never, 'vectorstore');
  await act(async () => { findButton(vectorContainer, '添加数据库').click(); });
  drawer = document.body.querySelector('.t-drawer')!;
  const vectorHeader = drawer.querySelector('.t-drawer__header')!;
  assert.match(vectorHeader.textContent!, /E/);
  assert.match(vectorHeader.innerHTML, /217, 119, 6/);
});
