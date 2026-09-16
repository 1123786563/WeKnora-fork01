import assert from 'node:assert/strict';
import { build } from 'esbuild';
import { JSDOM } from 'jsdom';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import test from 'node:test';
import { knowledgeListLabel } from './list.ts';

const root = resolve(import.meta.dirname, '../../../../..');
const require = createRequire(resolve(root, 'apps/web/package.json'));
const React = require('react') as typeof import('react');
const { act } = React;
const { createRoot } = require('react-dom/client') as { createRoot: (host: Element) => { render(node: unknown): void; unmount(): void } };

type Scenario = 'documents' | 'filters' | 'upload';

async function mount(locale: 'zh-CN' | 'en-US', scenario: Scenario) {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const runtime: any = {
    locale,
    tenantId: 'tenant-1',
    workspaces: [{ id: 'tenant-1', role: 'owner' }],
    client: {
      knowledge: {
        settings: { get: async () => ({ id: 'kb-1', type: 'document', isMine: true, indexing_strategy: { wiki_enabled: false, graph_enabled: true } }) },
        documents: {
          list: async () => {
            if (scenario === 'documents') throw {};
            return { data: [], total: 0, page: 1, page_size: 20 };
          },
          folders: async () => {
            if (scenario === 'filters') throw {};
            return { folders: [] };
          },
          tags: async () => [],
          upload: async () => { if (scenario === 'upload') throw {}; },
        },
      },
    },
  };
  Object.assign(globalThis, {
    window: dom.window,
    document: dom.window.document,
    IS_REACT_ACT_ENVIRONMENT: true,
    __knowledgeDocumentsRuntime: runtime,
    __knowledgeDocumentsPick: scenario === 'upload' ? [{ name: 'guide.pdf', uri: 'file:///guide.pdf', type: 'application/pdf', size: 42 }] : [],
  });
  const built = await build({
    entryPoints: [resolve(root, 'apps/mobile/src/features/knowledge/KnowledgeDocumentsScreen.tsx')],
    bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'],
    plugins: [{ name: 'native-host-fixtures', setup(b) {
      b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$|platform\/files\.ts$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
      b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router'
        ? `import React from 'react'; export const useLocalSearchParams=()=>({id:'kb-1'}); export const useRouter=()=>({back:()=>{},push:()=>{}});`
        : path.endsWith('runtime.tsx')
          ? `export const useMobileRuntime=()=>globalThis.__knowledgeDocumentsRuntime;`
          : path.endsWith('platform/files.ts')
            ? `export const pickNativeFiles=async()=>globalThis.__knowledgeDocumentsPick;`
            : `import React from 'react'; const Box=({children,...props})=><div {...props}>{children}</div>; export const AppState={addEventListener:()=>({remove:()=>{}})}; export const View=Box; export const SafeAreaView=Box; export const Text=({children,...props})=><span {...props}>{children}</span>; export const TextInput=({value,placeholder,...props})=><input value={value} placeholder={placeholder} {...props}/>; export const ActivityIndicator=({accessibilityLabel})=><span aria-label={accessibilityLabel}>loading</span>; export const Pressable=({children,onPress,disabled,...props})=><button disabled={disabled} onClick={onPress} {...props}>{children}</button>; export const FlatList=({data=[],renderItem,ListEmptyComponent,ListFooterComponent})=><div>{data.length ? data.map((item,index)=> <div key={index}>{renderItem({item,index})}</div>) : ListEmptyComponent}{ListFooterComponent}</div>;` }));
    } }],
  });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', built.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.KnowledgeDocumentsScreen as React.ComponentType;
  const host = dom.window.document.getElementById('root')!;
  const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(Component)));
  await act(async () => new Promise((done) => setTimeout(done, 0)));
  return { host, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

test('document list fallback errors are localized in zh-CN and en-US', async () => {
  for (const [locale, expected] of [['zh-CN', '加载文档失败'], ['en-US', 'Unable to load documents']] as const) {
    const page = await mount(locale, 'documents');
    try { assert.match(page.host.textContent ?? '', new RegExp(expected)); }
    finally { await page.close(); }
  }
});

test('document filter fallback error is localized', async () => {
  const page = await mount('zh-CN', 'filters');
  try { assert.match(page.host.textContent ?? '', /加载文档筛选条件失败/); }
  finally { await page.close(); }
});

test('document count interpolates the loaded total', async () => {
  const page = await mount('zh-CN', 'filters');
  try { assert.match(page.host.textContent ?? '', /0 项/); }
  finally { await page.close(); }
});

test('document shell exposes Graph when graph is enabled without Wiki', async () => {
  const page = await mount('en-US', 'documents');
  try { assert.match(page.host.textContent ?? '', /Graph/); }
  finally { await page.close(); }
});

test('upload failure summary uses the existing five-locale message catalog', async () => {
  for (const [locale, expected] of [['zh-CN', '所有文件上传失败'], ['en-US', 'All files failed to upload']] as const) {
    const page = await mount(locale, 'upload');
    try {
      const button = page.host.querySelector('button:last-of-type') as HTMLButtonElement;
      await act(async () => button.click());
      await act(async () => new Promise((done) => setTimeout(done, 0)));
      assert.match(page.host.textContent ?? '', new RegExp(expected));
    } finally { await page.close(); }
  }
});

test('document fallback keys resolve in every supported locale', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    for (const key of ['knowledgeBase.documents.loadFailed', 'knowledgeBase.documents.filtersLoadFailed']) {
      assert.notEqual(knowledgeListLabel(locale, key), key, `${locale}:${key}`);
    }
  }
});
