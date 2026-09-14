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

const graph = {
  nodes: [{ slug: 'start', title: 'Start', page_type: 'summary', link_count: 2, familiar: true }],
  edges: [],
  meta: { mode: 'overview', total: 1, returned: 1, truncated: false },
};

async function mount(locale: string, result: typeof graph | 'error') {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const runtime: any = {
    locale,
    client: { wiki: { graph: async () => { if (result === 'error') throw {}; return result; } } },
  };
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __knowledgeGraphRuntime: runtime });
  const built = await build({
    entryPoints: [resolve(root, 'apps/mobile/src/features/knowledge/KnowledgeGraphScreen.tsx')],
    bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic',
    external: ['react', 'react/jsx-runtime'],
    plugins: [{ name: 'native-host-fixtures', setup(b) {
      b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
      b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router'
        ? `import React from 'react'; export const useLocalSearchParams=()=>({id:'kb-1'}); export const useRouter=()=>({back:()=>{}});`
        : path.endsWith('runtime.tsx')
          ? `export const useMobileRuntime=()=>globalThis.__knowledgeGraphRuntime;`
          : `import React from 'react'; export const View=({children})=><div>{children}</div>; export const SafeAreaView=View; export const ScrollView=({children})=><div>{children}</div>; export const Text=({children,accessibilityLabel})=><span aria-label={accessibilityLabel}>{children}</span>; export const TextInput=({value,placeholder})=><input value={value} placeholder={placeholder} readOnly/>; export const ActivityIndicator=({accessibilityLabel})=><span aria-label={accessibilityLabel}>loading</span>; export const Pressable=({children,onPress,disabled})=><button disabled={disabled} onClick={onPress}>{children}</button>;` }));
    } }],
  });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', built.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.KnowledgeGraphScreen as React.ComponentType;
  const host = dom.window.document.getElementById('root')!;
  const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(Component)));
  await act(async () => new Promise((done) => setTimeout(done, 0)));
  return { host, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

test('graph copy is localized for all supported locales', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const copy = [
      knowledgeListLabel(locale, 'knowledgeBase.graph.depth'),
      knowledgeListLabel(locale, 'knowledgeBase.graph.showing', { visible: 1, total: 2 }),
      knowledgeListLabel(locale, 'knowledgeBase.graph.overviewBounded'),
      knowledgeListLabel(locale, 'knowledgeBase.graph.links'),
      knowledgeListLabel(locale, 'knowledgeBase.graph.familiar'),
    ].join(' ');
    assert.equal(copy.includes('knowledgeBase.graph.'), false, locale);
    assert.equal(copy.includes('Neighbor depth'), false, locale);
    assert.equal(copy.includes('Showing'), locale === 'en-US', locale);
  }
});

test('graph renders a localized empty state', async () => {
  const page = await mount('zh-CN', { ...graph, nodes: [], meta: { ...graph.meta, total: 0, returned: 0 } });
  try { assert.match(page.host.textContent ?? '', /暂无图谱数据/); }
  finally { await page.close(); }
});

test('graph renders the localized fallback error', async () => {
  const page = await mount('ja-JP', 'error');
  try { assert.match(page.host.textContent ?? '', /ナレッジグラフを読み込めません/); }
  finally { await page.close(); }
});
