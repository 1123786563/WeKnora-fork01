import assert from 'node:assert/strict';
import { build } from 'esbuild';
import { JSDOM } from 'jsdom';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import test from 'node:test';

const root = resolve(import.meta.dirname, '../../../../..');
const require = createRequire(resolve(root, 'apps/web/package.json'));
const React = require('react') as typeof import('react');
const { act } = require('react') as typeof import('react');
const { createRoot } = require('react-dom/client') as { createRoot: (host: Element) => { render(node: unknown): void; unmount(): void } };

interface Harness {
  host: HTMLElement;
  requests: Array<{ method: string; path: string; body?: unknown }>;
  pinCalls: string[];
  storage: Map<string, string>;
  close: () => Promise<void>;
}

async function mount(favoriteIds: string[]): Promise<Harness> {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const pinCalls: string[] = [];
  const storage = new Map<string, string>();
  const runtime: any = {
    userId: 'user-1', tenantId: '4', locale: 'en-US',
    workspaces: [{ id: 4, role: 'admin' }],
    logout: async () => {},
    client: {
      request: async (input: { method: string; path: string; body?: unknown }) => {
        requests.push(input);
        if (input.method === 'GET') return { success: true, data: favoriteIds.map((id) => ({ resource_type: 'kb', resource_id: id, created_at: '2026-01-01T00:00:00Z' })) };
        return { success: true };
      },
      knowledgeBases: {
        list: async () => [
          { id: 'kb-1', name: 'Team KB', description: '', type: 'document', knowledge_count: 3 },
          { id: 'kb-2', name: 'Other KB', description: '', type: 'document', knowledge_count: 1 },
        ],
        create: async () => ({}),
        togglePin: async (id: string) => { pinCalls.push(id); return { is_pinned: true }; },
      },
    },
  };
  Object.assign(globalThis, {
    window: dom.window,
    document: dom.window.document,
    IS_REACT_ACT_ENVIRONMENT: true,
    __kbPinsRuntime: runtime,
    __kbPinsStorage: {
      getItemAsync: async (key: string) => storage.get(key) ?? null,
      setItemAsync: async (key: string, value: string) => { storage.set(key, value); },
    },
  });
  const result = await build({
    stdin: { contents: "export { KnowledgeBaseListScreen } from './KnowledgeBaseListScreen.tsx';", resolveDir: resolve(root, 'apps/mobile/src/features/knowledge'), sourcefile: 'entry.tsx' },
    bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'],
    plugins: [{ name: 'native-host-fixtures', setup(b) {
      b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$|expo-secure-store$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
      b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents:
        path === 'expo-router' ? `import React from 'react'; export const useLocalSearchParams=()=>({}); export const useRouter=()=>({back:()=>{},push:()=>{},replace:()=>{}});`
        : path === 'expo-secure-store' ? `export const getItemAsync=(key)=>globalThis.__kbPinsStorage.getItemAsync(key); export const setItemAsync=(key,value)=>globalThis.__kbPinsStorage.setItemAsync(key,value); export const deleteItemAsync=async()=>{};`
        : path.endsWith('runtime.tsx') ? `export const useMobileRuntime=()=>globalThis.__kbPinsRuntime;`
        : `import React from 'react'; export const View=({children})=><div>{children}</div>; export const SafeAreaView=View; export const Text=({children})=><span>{children}</span>; export const ActivityIndicator=({accessibilityLabel})=><span aria-label={accessibilityLabel}>loading</span>; export const Pressable=({children,onPress,disabled,accessibilityLabel})=><button disabled={disabled} aria-label={accessibilityLabel} onClick={onPress}>{children}</button>; export const FlatList=({data,renderItem,ListEmptyComponent})=> data?.length ? <div>{data.map((item,index)=><div key={index}>{renderItem({item,index})}</div>)}</div> : <div>{ListEmptyComponent}</div>; export const TextInput=({value,onChangeText,placeholder})=><input value={value} placeholder={placeholder} onChange={e=>onChangeText(e.target.value)}/>; export const Modal=({visible,children})=><div>{visible ? children : null}</div>;` }));
    } }],
  });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', result.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.KnowledgeBaseListScreen as React.ComponentType;
  const host = dom.window.document.getElementById('root')!;
  const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(Component)));
  await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
  return { host, requests, pinCalls, storage, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

function starTexts(host: HTMLElement): string[] {
  return [...host.querySelectorAll('span')].map((node) => node.textContent ?? '').filter((text) => text === '★' || text === '☆');
}

test('favorites hydrate from the server favorites endpoint', async () => {
  const page = await mount(['kb-1']);
  try {
    assert.deepEqual(page.requests.filter((request) => request.method === 'GET').map((request) => request.path), ['/api/v1/user/favorites?type=kb']);
    assert.deepEqual(starTexts(page.host), ['★', '☆']);
  } finally { await page.close(); }
});

test('tapping the star persists through the favorites endpoint', async () => {
  const page = await mount(['kb-1']);
  try {
    const star = [...page.host.querySelectorAll('button')].find((button) => (button.textContent ?? '').includes('★'))!;
    await act(async () => star.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.deepEqual(page.requests.filter((request) => request.method !== 'GET'), [{ method: 'DELETE', path: '/api/v1/user/favorites/kb/kb-1' }]);
    assert.deepEqual(starTexts(page.host), ['☆', '☆']);
  } finally { await page.close(); }
});

test('opening a KB persists a recent under the user-and-tenant key', async () => {
  const page = await mount([]);
  try {
    const row = [...page.host.querySelectorAll('button')].find((button) => (button.textContent ?? '').includes('Team KB'))!;
    await act(async () => row.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 20)));
    const persisted = page.storage.get('WeKnora_user-1_t4_resource_recents');
    assert.ok(persisted);
    assert.deepEqual(JSON.parse(persisted!).map((entry: { type: string; id: string }) => ({ type: entry.type, id: entry.id })), [{ type: 'kb', id: 'kb-1' }]);
  } finally { await page.close(); }
});

test('pinning a KB calls the server endpoint and regroups the list', async () => {
  const page = await mount([]);
  try {
    assert.equal((page.host.textContent ?? '').includes('Pinned'), false);
    const pin = [...page.host.querySelectorAll('button')].find((button) => (button.getAttribute('aria-label') ?? '').startsWith('Pin to Top'))!;
    await act(async () => pin.click());
    await act(async () => new Promise((resolve) => setTimeout(resolve, 20)));
    assert.deepEqual(page.pinCalls, ['kb-1']);
    assert.equal((page.host.textContent ?? '').includes('Pinned'), true);
  } finally { await page.close(); }
});
