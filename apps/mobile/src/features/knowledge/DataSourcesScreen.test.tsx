import assert from 'node:assert/strict';
import { build } from 'esbuild';
import { JSDOM } from 'jsdom';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import test from 'node:test';

const root = resolve(import.meta.dirname, '../../../../..');
// Use the workspace's paired React/ReactDOM versions for the DOM harness;
// resolving React from the mobile package alone can load a second minor.
const require = createRequire(resolve(root, 'apps/web/package.json'));
const React = require('react') as typeof import('react');
const { act } = require('react') as typeof import('react');
const { createRoot } = require('react-dom/client') as { createRoot: (host: Element) => { render(node: unknown): void; unmount(): void } };

async function mount(role: string, locale = 'en-US', overrides: { list?: () => Promise<unknown>; logs?: (...args: unknown[]) => Promise<unknown>; resume?: (...args: unknown[]) => Promise<unknown>; resourceAncestors?: (...args: unknown[]) => Promise<unknown> } = {}) {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const runtime: any = {
    tenantId: '1', locale,
    workspaces: [{ id: 1, role }],
    client: { dataSources: {
      list: async () => [
        { id: 'source-1', name: 'Docs', type: 'feishu_drive', status: 'active', sync_mode: 'incremental', config: { resource_ids: ['root-token'] }, latest_sync_log: { id: 'log-1', status: 'running', items_created: 2, items_failed: 0 } },
        { id: 'legacy-source', name: 'Legacy connector', type: 'legacy_connector', status: 'paused', sync_mode: 'full', config: {} },
      ],
      types: async () => [{ type: 'feishu_drive', name: 'Feishu Drive', description: 'Drive', priority: 1, auth_type: 'oauth', capabilities: ['resources'] }, { type: 'gitlab', name: 'GitLab', description: 'GitLab', priority: 2, auth_type: 'token', capabilities: ['resources'] }, { type: 'notion', name: 'Notion', description: 'Notion', priority: 3, auth_type: 'token', capabilities: ['resources'] }, { type: 'rss', name: 'RSS', description: 'RSS', priority: 4, auth_type: 'none', capabilities: [] }],
      validateCredentials: async () => ({ success: true }), validate: async () => ({ success: true }),
      create: async () => ({ id: 'temporary-source', knowledge_base_id: 'kb-1', name: 'New source', type: 'feishu_drive', status: 'paused', config: {} }), update: async () => ({}), putCredentials: async () => ({}),
      sync: async () => ({ success: true }), pause: async () => ({ success: true }), resume: async () => ({ success: true }),
      remove: async () => undefined, logs: async (_id: string, _limit = 20, _offset = 0) => [{ id: 'log-1', status: 'success', started_at: '2026-09-13T08:00:00Z', finished_at: '2026-09-13T08:00:02Z', items_created: 3, items_updated: 1, items_deleted: 0, items_skipped: 0, items_failed: 0 }], resources: async (_id: string, parentId?: string) => parentId ? [{ external_id: 'page-2', parent_id: parentId, name: 'Child page', type: 'page' }] : [{ external_id: 'page-1', name: 'Project docs', type: 'folder', has_children: true }], resourceAncestors: async () => [],
    } },
  };
  if (overrides.list) runtime.client.dataSources.list = overrides.list;
  if (overrides.logs) runtime.client.dataSources.logs = overrides.logs;
  if (overrides.resume) runtime.client.dataSources.resume = overrides.resume;
  if (overrides.resourceAncestors) runtime.client.dataSources.resourceAncestors = overrides.resourceAncestors;
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __dataSourcesRuntime: runtime });
  const result = await build({ entryPoints: [resolve(root, 'apps/mobile/src/features/knowledge/DataSourcesScreen.tsx')], bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'], plugins: [{ name: 'native-host-fixtures', setup(b) {
    b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
    b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router' ? `import React from 'react'; export const useLocalSearchParams=()=>({id:'kb-1'}); export const useRouter=()=>({back:()=>{}});` : path.endsWith('runtime.tsx') ? `export const useMobileRuntime=()=>globalThis.__dataSourcesRuntime;` : `import React from 'react'; export const View=({children})=><div>{children}</div>; export const SafeAreaView=View, ScrollView=View; export const Text=({children})=><span>{children}</span>; export const ActivityIndicator=({accessibilityLabel})=><span aria-label={accessibilityLabel}>loading</span>; export const Pressable=({children,onPress,disabled})=><button disabled={disabled} onClick={onPress}>{children}</button>; export const FlatList=({data,renderItem,ListEmptyComponent})=> data?.length ? <div>{data.map((item,index)=><div key={index}>{renderItem({item,index})}</div>)}</div> : <div>{ListEmptyComponent}</div>; export const Switch=()=> <input type="checkbox"/>; export const TextInput=({value,onChangeText,placeholder})=><input value={value} placeholder={placeholder} onChange={e=>onChangeText(e.target.value)} onInput={e=>onChangeText(e.target.value)}/>; export const Alert={alert:()=>{}};` }));
  }}] });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', result.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.DataSourcesScreen as React.ComponentType;
  const host = dom.window.document.getElementById('root')!;
  const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(Component)));
  await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
  return { host, runtime, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}



test('source inventory localizes the load failure fallback in every supported locale', async () => {
  const expected: Record<string, string> = {
    'zh-CN': '加载数据源失败',
    'en-US': 'Unable to load data sources',
    'ja-JP': 'データソースを読み込めません',
    'ko-KR': '데이터 소스를 불러올 수 없습니다',
    'ru-RU': 'Не удалось загрузить источники данных',
  };
  for (const [locale, message] of Object.entries(expected)) {
    const page = await mount('viewer', locale, { list: async () => { throw 'network'; } });
    try { assert.match(page.host.textContent ?? '', new RegExp(message.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), locale); }
    finally { await page.close(); }
  }
});

test('sync log and resume failures use localized fallbacks', async () => {
  const page = await mount('admin', 'zh-CN', {
    logs: async () => { throw 'network'; },
    resume: async () => { throw 'network'; },
  });
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === '日志')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /加载同步日志失败/);
    const resume = [...page.host.querySelectorAll('button')].find((item) => item.textContent === '恢复');
    assert.ok(resume);
    await act(async () => resume?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /恢复数据源失败/);
  } finally { await page.close(); }
});

test('viewer sees read-only data-source inventory without mutation controls', async () => {
  const page = await mount('viewer');
  try { const buttons = () => Array.from(page.host.querySelectorAll('button')).map((item) => item.textContent); assert.equal(buttons().includes('Add Data Source'), false); assert.equal(buttons().includes('Edit'), false); assert.equal(buttons().includes('Sync Now'), false); assert.equal(buttons().includes('Delete'), false); assert.equal(buttons().includes('Logs'), true); }
  finally { await page.close(); }
});

test('existing unknown connector sources remain visible in the inventory', async () => {
  const page = await mount('viewer');
  try { assert.match(page.host.textContent ?? '', /Legacy connector/); assert.match(page.host.textContent ?? '', /legacy_connector/); }
  finally { await page.close(); }
});

test('admin sees data-source mutation controls', async () => {
  const page = await mount('admin');
  try { assert.equal(page.host.textContent?.includes('Add'), true); assert.equal(page.host.textContent?.includes('Edit'), true); assert.equal(page.host.textContent?.includes('Sync'), true); assert.equal(page.host.textContent?.includes('Delete'), true); }
  finally { await page.close(); }
});

test('inventory renders the server latest sync result and running state', async () => {
  const page = await mount('admin');
  try {
    assert.match(page.host.textContent ?? '', /Syncing · \+2/);
  } finally { await page.close(); }
});

test('sync log drawer renders summary and expandable counters', async () => {
  const page = await mount('admin');
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Logs')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /Runs 1 · Success 1 · Failed 0 · Items 4/);
    const row = [...page.host.querySelectorAll('button')].find((item) => item.textContent?.includes('Success'));
    assert.ok(row);
    await act(async () => row?.click());
    assert.match(page.host.textContent ?? '', /Created 3 · Updated 1 · Deleted 0 · Skipped 0 · Failed 0/);
  } finally { await page.close(); }
});

test('sync log drawer paginates with a guarded server offset', async () => {
  const page = await mount('admin');
  try {
    const calls: unknown[][] = [];
    page.runtime.client.dataSources.logs = async (...args: unknown[]) => {
      calls.push(args);
      return Array.from({ length: calls.length === 1 ? 50 : 1 }, (_, index) => ({ id: `log-${calls.length}-${index}`, status: 'success' }));
    };
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Logs')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.deepEqual(calls[0], ['source-1', 50, 0]);
    const more = [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Load more');
    assert.ok(more);
    await act(async () => more?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.deepEqual(calls[1], ['source-1', 50, 50]);
    assert.equal(Array.from(page.host.querySelectorAll('button')).some((item) => item.textContent === 'Load more'), false);
  } finally { await page.close(); }
});

test('sync log drawer uses the active locale for empty state and controls', async () => {
  const page = await mount('admin', 'zh-CN');
  try {
    page.runtime.client.dataSources.logs = async () => [];
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === '日志')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /同步历史/);
    assert.match(page.host.textContent ?? '', /暂无同步记录/);
    assert.match(page.host.textContent ?? '', /关闭/);
  } finally { await page.close(); }
});

test('editor chrome and connector copy follow the active locale', async () => {
  const page = await mount('admin', 'zh-CN');
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === '添加数据源')?.click());
    assert.match(page.host.textContent ?? '', /数据源管理/);
    assert.match(page.host.textContent ?? '', /基本信息/);
    assert.ok(page.host.querySelector('input[placeholder="输入数据源名称"]'));
    assert.match(page.host.textContent ?? '', /飞书云盘/);
    assert.match(page.host.textContent ?? '', /同步飞书云盘文件夹中的文档、表格、文件/);
    assert.equal([...page.host.querySelectorAll('button')].some((item) => item.textContent === '测试连接'), true);
    assert.equal([...page.host.querySelectorAll('button')].some((item) => item.textContent === '取消'), true);
  } finally { await page.close(); }
});

test('admin can test a connector before saving and sees the server result', async () => {
  const page = await mount('admin');
  try {
    const edit = [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Edit');
    assert.ok(edit);
    await act(async () => edit?.click());
    const testButton = [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Test Connection');
    assert.ok(testButton, 'editor should expose a pre-save connection test');
    await act(async () => testButton?.click());
    assert.match(page.host.textContent ?? '', /Connection successful/);
  } finally { await page.close(); }
});

test('editing a data source loads and selects server resources for the next save', async () => {
  const page = await mount('admin');
  try {
    const edit = [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Edit');
    assert.ok(edit);
    await act(async () => edit?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /Project docs/);
    const resource = [...page.host.querySelectorAll('button')].find((item) => item.textContent?.includes('Project docs'));
    assert.ok(resource);
    await act(async () => resource?.click());
    assert.match(page.host.textContent ?? '', /✓ Project docs/);
  } finally { await page.close(); }
});

test('expanding a resource requests and renders its child resources', async () => {
  const page = await mount('admin');
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Edit')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    const expand = [...page.host.querySelectorAll('button')].find((item) => item.textContent === '›');
    assert.ok(expand);
    await act(async () => expand?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /Child page/);
  } finally { await page.close(); }
});

test('editing a source reveals saved deep resource selections', async () => {
  const calls: unknown[][] = [];
  const page = await mount('admin', 'en-US', {
    list: async () => [{ id: 'source-1', name: 'Docs', type: 'feishu_drive', status: 'active', config: { resource_ids: ['page-2'] } }],
    resourceAncestors: async (...args: unknown[]) => { calls.push(args); return ['page-1']; },
  });
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Edit')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.deepEqual(calls, [['source-1', ['page-2']]]);
    assert.match(page.host.textContent ?? '', /Child page/);
  } finally { await page.close(); }
});

test('drive editor exposes the root token and loads resources through the existing source', async () => {
  const page = await mount('admin');
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Edit')?.click());
    const rootInput = page.host.querySelector('input[placeholder="Enter a folder_token or a Feishu Drive folder URL"]');
    assert.ok(rootInput);
    const load = [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Load');
    assert.ok(load);
    await act(async () => load?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /Project docs/);
  } finally { await page.close(); }
});

test('new Drive source creates a paused temporary row before loading resources', async () => {
  const page = await mount('admin');
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Add Data Source')?.click());
    const nameInput = page.host.querySelector('input[placeholder="Enter data source name"]') as HTMLInputElement | null;
    assert.ok(nameInput);
    await act(async () => { nameInput!.value = 'New source'; nameInput!.dispatchEvent(new page.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    const rootInput = page.host.querySelector('input[placeholder="Enter a folder_token or a Feishu Drive folder URL"]') as HTMLInputElement | null;
    assert.ok(rootInput);
    await act(async () => { rootInput!.value = 'https://example.feishu.cn/drive/folder/new-root'; rootInput!.dispatchEvent(new page.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Load')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /Project docs/);
  } finally { await page.close(); }
});

test('canceling a Drive resource draft removes the temporary source', async () => {
  const page = await mount('admin');
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Add Data Source')?.click());
    const nameInput = page.host.querySelector('input[placeholder="Enter data source name"]') as HTMLInputElement;
    await act(async () => { nameInput.value = 'Draft source'; nameInput.dispatchEvent(new page.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    const rootInput = page.host.querySelector('input[placeholder="Enter a folder_token or a Feishu Drive folder URL"]') as HTMLInputElement;
    await act(async () => { rootInput.value = 'draft-root'; rootInput.dispatchEvent(new page.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Load')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    const removed: string[] = [];
    page.runtime.client.dataSources.remove = async (id: string) => { removed.push(id); };
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Cancel')?.click());
    assert.deepEqual(removed, ['temporary-source']);
  } finally { await page.close(); }
});

test('GitLab editor exposes project-specific fields and requires a project id', async () => {
  const page = await mount('admin');
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Add Data Source')?.click());
    const gitlab = [...page.host.querySelectorAll('button')].find((item) => item.textContent?.includes('GitLab'));
    assert.ok(gitlab);
    await act(async () => gitlab?.click());
    assert.ok(page.host.querySelector('input[placeholder="For example: 12345 or group/project"]'));
    assert.ok(page.host.querySelector('input[placeholder="Leave empty to use the default branch"]'));
    assert.ok([...page.host.querySelectorAll('button')].some((item) => item.textContent === 'Add project'));
  } finally { await page.close(); }
});

test('RSS editor exposes feed URLs and custom request headers', async () => {
  const page = await mount('admin');
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Add Data Source')?.click());
    const rss = [...page.host.querySelectorAll('button')].find((item) => item.textContent?.includes('RSS'));
    assert.ok(rss);
    await act(async () => rss?.click());
    assert.ok(page.host.querySelector('textarea[placeholder="https://example.com/feed.xml"]') || page.host.querySelector('input[placeholder="https://example.com/feed.xml"]'));
    assert.ok(page.host.querySelector('textarea[placeholder="Authorization: Bearer …"]') || page.host.querySelector('input[placeholder="Authorization: Bearer …"]'));
  } finally { await page.close(); }
});

test('Notion editor exposes its integration token instead of generic credentials', async () => {
  const page = await mount('admin');
  try {
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Add Data Source')?.click());
    const notion = [...page.host.querySelectorAll('button')].find((item) => item.textContent?.includes('Notion'));
    assert.ok(notion);
    await act(async () => notion?.click());
    assert.ok(page.host.querySelector('input[placeholder="ntn_xxxx"]'));
    assert.equal(page.host.querySelector('input[placeholder="Credentials: token = secret"]'), null);
  } finally { await page.close(); }
});

test('editing credentials uses the dedicated endpoint and keeps them out of the main update', async () => {
  const page = await mount('admin');
  try {
    const putCalls: unknown[] = [];
    const updateCalls: unknown[] = [];
    const events: string[] = [];
    page.runtime.client.dataSources.putCredentials = async (...args: unknown[]) => { events.push('credentials'); putCalls.push(args); return {}; };
    page.runtime.client.dataSources.update = async (...args: unknown[]) => { events.push('update'); updateCalls.push(args); return {}; };
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Edit')?.click());
    const appId = page.host.querySelector('input[placeholder="cli_xxxx"]') as HTMLInputElement | null;
    const appSecret = page.host.querySelector('input[placeholder="App Secret"]') as HTMLInputElement | null;
    assert.ok(appId); assert.ok(appSecret);
    await act(async () => { appId!.value = 'cli_test'; appId!.dispatchEvent(new page.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); appSecret!.value = 'secret'; appSecret!.dispatchEvent(new page.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Save')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.equal(putCalls.length, 1);
    assert.deepEqual(putCalls[0], ['source-1', { app_id: 'cli_test', app_secret: 'secret' }]);
    assert.deepEqual(events, ['credentials', 'update']);
    assert.equal(updateCalls.length, 1);
    assert.equal((updateCalls[0] as unknown[])[1] && ((updateCalls[0] as unknown[])[1] as { config?: { credentials?: unknown } }).config?.credentials, undefined);
  } finally { await page.close(); }
});

test('editing without new credentials validates the persisted source by id', async () => {
  const page = await mount('admin');
  try {
    let validateId = '';
    let rawValidationCalled = false;
    page.runtime.client.dataSources.validate = async (id: string) => { validateId = id; return { success: true }; };
    page.runtime.client.dataSources.validateCredentials = async () => { rawValidationCalled = true; return { success: true }; };
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Edit')?.click());
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Test Connection')?.click());
    assert.equal(validateId, 'source-1');
    assert.equal(rawValidationCalled, false);
  } finally { await page.close(); }
});

test('RSS connection test sends feed URLs to the raw credential validator', async () => {
  const page = await mount('admin');
  try {
    let payload: Record<string, unknown> | undefined;
    page.runtime.client.dataSources.validateCredentials = async (_type: string, credentials: Record<string, unknown>) => { payload = credentials; return { success: true }; };
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Add Data Source')?.click());
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent?.includes('RSS'))?.click());
    const name = page.host.querySelector('input[placeholder="Enter data source name"]') as HTMLInputElement | null;
    assert.ok(name);
    await act(async () => { Object.getOwnPropertyDescriptor(page.host.ownerDocument.defaultView!.HTMLInputElement.prototype, 'value')!.set!.call(name, 'RSS source'); name!.dispatchEvent(new page.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    const feed = page.host.querySelector('input[placeholder="https://example.com/feed.xml"]') as HTMLInputElement | null;
    assert.ok(feed);
    await act(async () => { Object.getOwnPropertyDescriptor(page.host.ownerDocument.defaultView!.HTMLInputElement.prototype, 'value')!.set!.call(feed, 'https://example.test/feed.xml'); feed!.dispatchEvent(new page.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Test Connection')?.click());
    assert.equal(payload?.feed_urls, 'https://example.test/feed.xml');
  } finally { await page.close(); }
});

test('editing feed URLs updates settings without writing an empty credential map', async () => {
  const page = await mount('admin');
  try {
    let updateCalls = 0;
    page.runtime.client.dataSources.putCredentials = async () => { throw new Error('empty credentials must not be written'); };
    page.runtime.client.dataSources.validateCredentials = async () => ({ success: true });
    page.runtime.client.dataSources.update = async () => { updateCalls += 1; return {}; };
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Edit')?.click());
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent?.includes('RSS'))?.click());
    const feed = page.host.querySelector('input[placeholder="https://example.com/feed.xml"]') as HTMLInputElement | null;
    assert.ok(feed);
    await act(async () => { Object.getOwnPropertyDescriptor(page.host.ownerDocument.defaultView!.HTMLInputElement.prototype, 'value')!.set!.call(feed, 'https://example.test/changed.xml'); feed!.dispatchEvent(new page.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    await act(async () => [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Save')?.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.equal(updateCalls, 1);
  } finally { await page.close(); }
});
