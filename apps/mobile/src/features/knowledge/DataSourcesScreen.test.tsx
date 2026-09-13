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

async function mount(role: string) {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const runtime = {
    tenantId: '1',
    workspaces: [{ id: 1, role }],
    client: { dataSources: {
      list: async () => [{ id: 'source-1', name: 'Docs', type: 'notion', status: 'active', sync_mode: 'incremental', config: {} }],
      types: async () => [],
      validateCredentials: async () => ({ success: true }),
      create: async () => ({}), update: async () => ({}), putCredentials: async () => ({}),
      sync: async () => ({ success: true }), pause: async () => ({ success: true }), resume: async () => ({ success: true }),
      remove: async () => undefined, logs: async () => [], resources: async () => [{ external_id: 'page-1', name: 'Project docs', type: 'page' }],
    } },
  };
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __dataSourcesRuntime: runtime });
  const result = await build({ entryPoints: [resolve(root, 'apps/mobile/src/features/knowledge/DataSourcesScreen.tsx')], bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'], plugins: [{ name: 'native-host-fixtures', setup(b) {
    b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
    b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router' ? `import React from 'react'; export const useLocalSearchParams=()=>({id:'kb-1'}); export const useRouter=()=>({back:()=>{}});` : path.endsWith('runtime.tsx') ? `export const useMobileRuntime=()=>globalThis.__dataSourcesRuntime;` : `import React from 'react'; export const View=({children})=><div>{children}</div>; export const SafeAreaView=View, ScrollView=View; export const Text=({children})=><span>{children}</span>; export const ActivityIndicator=({accessibilityLabel})=><span aria-label={accessibilityLabel}>loading</span>; export const Pressable=({children,onPress,disabled})=><button disabled={disabled} onClick={onPress}>{children}</button>; export const FlatList=({data,renderItem,ListEmptyComponent})=> data?.length ? <div>{data.map((item,index)=><div key={index}>{renderItem({item,index})}</div>)}</div> : <div>{ListEmptyComponent}</div>; export const Switch=()=> <input type="checkbox"/>; export const TextInput=({value,onChangeText,placeholder})=><input value={value} placeholder={placeholder} onChange={e=>onChangeText(e.target.value)}/>; export const Alert={alert:()=>{}};` }));
  }}] });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', result.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.DataSourcesScreen as React.ComponentType;
  const host = dom.window.document.getElementById('root')!;
  const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(Component)));
  await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
  return { host, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

test('viewer sees read-only data-source inventory without mutation controls', async () => {
  const page = await mount('viewer');
  try { assert.equal(page.host.textContent?.includes('Add'), false); assert.equal(page.host.textContent?.includes('Edit'), false); assert.equal(page.host.textContent?.includes('Sync'), false); assert.equal(page.host.textContent?.includes('Delete'), false); assert.equal(page.host.textContent?.includes('Logs'), true); }
  finally { await page.close(); }
});

test('admin sees data-source mutation controls', async () => {
  const page = await mount('admin');
  try { assert.equal(page.host.textContent?.includes('Add'), true); assert.equal(page.host.textContent?.includes('Edit'), true); assert.equal(page.host.textContent?.includes('Sync'), true); assert.equal(page.host.textContent?.includes('Delete'), true); }
  finally { await page.close(); }
});

test('admin can test a connector before saving and sees the server result', async () => {
  const page = await mount('admin');
  try {
    const edit = [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Edit');
    assert.ok(edit);
    await act(async () => edit?.click());
    const testButton = [...page.host.querySelectorAll('button')].find((item) => item.textContent === 'Test connection');
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
