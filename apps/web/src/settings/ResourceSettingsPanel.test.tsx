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
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
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
  const drawer = document.body.querySelector('aside[role="dialog"]')!;
  assert.ok(drawer, 'the add drawer should open');

  // Section titles + structured fields, mirroring the Vue create drawer.
  assert.match(drawer.textContent!, /基本信息/);
  assert.match(drawer.textContent!, /连接信息/);
  assert.match(drawer.textContent!, /引擎类型/);
  assert.match(drawer.textContent!, /Elasticsearch/);
  assert.match(drawer.textContent!, /addr/);
  assert.match(drawer.textContent!, /password/);
  // Advanced index config section exists (collapsed by default like Vue).
  assert.match(drawer.textContent!, /高级设置/);
  // Test connection lives in the drawer footer with the save/cancel pair.
  assert.match(drawer.textContent!, /测试连接/);
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
  const drawer = document.body.querySelector('aside[role="dialog"]')!;

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
  const drawer = document.body.querySelector('aside[role="dialog"]')!;
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
  const drawer = document.body.querySelector('aside[role="dialog"]')!;

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
  setSelectValue(provider, 's3');
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
  const drawer = document.body.querySelector('aside[role="dialog"]')!;
  // The deploy-mode pills only render for MinIO — switch the provider first.
  setSelectValue(findLabelledControl(drawer, '存储类型') as HTMLSelectElement, 'minio');
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
  const drawer = document.body.querySelector('aside[role="dialog"]')!;
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
  const drawer = document.body.querySelector('aside[role="dialog"]')!;

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
  const drawer = document.body.querySelector('aside[role="dialog"]')!;

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
  const drawer = document.body.querySelector('aside[role="dialog"]')!;
  const provider = findLabelledControl(drawer, '引擎类型') as HTMLSelectElement;
  assert.equal(provider.disabled, true);

  // Editing without retyping the key posts no api_key anywhere.
  await submitForm(drawer);
  assert.equal(updated.length, 1);
  const input = updated[0]!.input as { provider: string; parameters: Record<string, unknown> };
  assert.equal(input.provider, 'brave');
  assert.equal('api_key' in input.parameters, false);
  assert.equal(input.parameters.proxy_url, 'http://127.0.0.1:7890');
});
