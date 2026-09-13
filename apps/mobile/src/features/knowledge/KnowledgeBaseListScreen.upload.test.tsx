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

async function mount(locale = 'en-US') {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  let listCalls = 0;
  const runtime: any = {
    tenantId: '1', locale,
    workspaces: [{ id: 1, role: 'admin' }],
    logout: async () => {},
    client: {
      request: async () => ({ success: true, data: [] }),
      knowledgeBases: {
        list: async () => { listCalls += 1; return [{ id: 'kb-1', name: 'Team KB', description: '', type: 'document', knowledge_count: 3 }]; },
        create: async () => ({}),
      },
    },
  };
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __kbListRuntime: runtime });
  // One bundle for the screen AND the dispatcher so both share the single
  // module-level event bus (the mobile stand-in for Vue's window events).
  const result = await build({ stdin: { contents: "export { KnowledgeBaseListScreen } from './KnowledgeBaseListScreen.tsx'; export { dispatchUploadEvent } from './upload-progress.ts';", resolveDir: resolve(root, 'apps/mobile/src/features/knowledge'), sourcefile: 'entry.tsx' }, bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'], plugins: [{ name: 'native-host-fixtures', setup(b) {
    b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$|expo-secure-store$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
    b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-secure-store' ? "export const getItemAsync=async()=>null; export const setItemAsync=async()=>{}; export const deleteItemAsync=async()=>{};" : path === 'expo-router' ? `import React from 'react'; export const useLocalSearchParams=()=>({}); export const useRouter=()=>({back:()=>{},push:()=>{},replace:()=>{}});` : path.endsWith('runtime.tsx') ? `export const useMobileRuntime=()=>globalThis.__kbListRuntime;` : `import React from 'react'; export const View=({children})=><div>{children}</div>; export const SafeAreaView=View; export const Text=({children})=><span>{children}</span>; export const ActivityIndicator=({accessibilityLabel})=><span aria-label={accessibilityLabel}>loading</span>; export const Pressable=({children,onPress,disabled})=><button disabled={disabled} onClick={onPress}>{children}</button>; export const FlatList=({data,renderItem,ListEmptyComponent})=> data?.length ? <div>{data.map((item,index)=><div key={index}>{renderItem({item,index})}</div>)}</div> : <div>{ListEmptyComponent}</div>; export const TextInput=({value,onChangeText,placeholder})=><input value={value} placeholder={placeholder} onChange={e=>onChangeText(e.target.value)}/>; export const Modal=({visible,children})=><div>{visible ? children : null}</div>;` }));
  }}] });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', result.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.KnowledgeBaseListScreen as React.ComponentType;
  const dispatch = module.exports.dispatchUploadEvent as (event: unknown) => void;
  const host = dom.window.document.getElementById('root')!;
  const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(Component)));
  await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
  return { host, runtime, dispatch, listCalls: () => listCalls, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

test('upload events render the Vue-parity progress mask on the KB list', async () => {
  const page = await mount();
  try {
    await act(async () => { page.dispatch({ type: 'start', uploadId: 'u1', kbId: 'kb-1', fileName: 'doc.pdf' }); });
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /Uploading folder documents to "Team KB"/);
    assert.match(page.host.textContent ?? '', /0\/1 files finished/);

    await act(async () => { page.dispatch({ type: 'complete', uploadId: 'u1', kbId: 'kb-1', status: 'success', progress: 100 }); });
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /Upload finished for "Team KB"/);
    assert.match(page.host.textContent ?? '', /All 1 files uploaded/);
  } finally { await page.close(); }
});

test('failed uploads stay visible on the mask with the error tip', async () => {
  const page = await mount();
  try {
    await act(async () => { page.dispatch({ type: 'start', uploadId: 'u1', kbId: 'kb-1' }); page.dispatch({ type: 'complete', uploadId: 'u1', kbId: 'kb-1', status: 'error', error: 'duplicate_file' }); });
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.match(page.host.textContent ?? '', /Some files failed to upload/);
    assert.match(page.host.textContent ?? '', /Upload finished for "Team KB"/);
  } finally { await page.close(); }
});

test('a finished upload batch refreshes the list after the Vue debounce', async () => {
  const page = await mount();
  try {
    const before = page.listCalls();
    await act(async () => { page.dispatch({ type: 'uploaded', kbId: 'kb-1' }); });
    await act(async () => new Promise((resolve) => setTimeout(resolve, 0)));
    assert.equal(page.listCalls(), before); // debounced, not immediate
    await act(async () => new Promise((resolve) => setTimeout(resolve, 900)));
    assert.ok(page.listCalls() > before, 'list should refetch after the 800ms debounce');
  } finally { await page.close(); }
});
