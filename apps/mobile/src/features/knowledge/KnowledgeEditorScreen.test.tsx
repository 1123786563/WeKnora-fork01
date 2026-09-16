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

async function mount(locale: string, kind: 'wiki' | 'faq', slug?: string, result: 'ok' | 'error' = 'ok', workspaceRole = 'owner', sharedPermission?: string) {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const wiki = { slug: 'guide', title: 'Guide', summary: 'Summary', content: '# Guide', version: 3 };
  const faq = { id: 7, standard_question: 'How?', answers: ['Like this'], is_enabled: true, is_recommended: false };
  const runtime: any = {
    locale,
    tenantId: 'tenant-1',
    workspaces: [{ id: 'tenant-1', role: workspaceRole }],
    client: {
      wiki: { get: async () => { if (result === 'error') throw {}; return wiki; }, update: async () => wiki, create: async () => wiki },
      knowledge: { faq: { get: async () => { if (result === 'error') throw {}; return faq; }, update: async () => faq, create: async () => faq }, settings: { get: async () => ({ my_permission: sharedPermission, isMine: sharedPermission !== undefined ? false : true }) } },
    },
  };
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __knowledgeEditorRuntime: runtime });
  const built = await build({
    entryPoints: [resolve(root, 'apps/mobile/src/features/knowledge/KnowledgeEditorScreen.tsx')],
    bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'],
    plugins: [{ name: 'native-host-fixtures', setup(b) {
      b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
      b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router'
        ? `import React from 'react'; export const useLocalSearchParams=()=>({id:'kb-1',kind:'${kind}',slug:${slug ? `'${slug}'` : 'undefined'}}); export const useRouter=()=>({back:()=>{}});`
        : path.endsWith('runtime.tsx')
          ? `export const useMobileRuntime=()=>globalThis.__knowledgeEditorRuntime;`
          : `import React from 'react'; const Box=({children})=><div>{children}</div>; export const View=Box; export const SafeAreaView=Box; export const ScrollView=Box; export const Text=({children,accessibilityLabel})=><span aria-label={accessibilityLabel}>{children}</span>; export const ActivityIndicator=({accessibilityLabel})=><span aria-label={accessibilityLabel}>loading</span>; export const Pressable=({children,onPress,disabled})=><button disabled={disabled} onClick={onPress}>{children}</button>; export const TextInput=({value,onChangeText,accessibilityLabel})=><input aria-label={accessibilityLabel} value={value} onChange={e=>onChangeText(e.target.value)}/>; export const Switch=({accessibilityLabel})=><input type="checkbox" aria-label={accessibilityLabel}/>;` }));
    } }],
  });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', built.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.KnowledgeEditorScreen as React.ComponentType;
  const host = dom.window.document.getElementById('root')!;
  const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(Component)));
  await act(async () => new Promise((done) => setTimeout(done, 0)));
  return { host, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

test('editor-specific copy exists in all supported locales', () => {
  const keys = [
    'knowledgeEditor.mobile.editWikiTitle', 'knowledgeEditor.mobile.editPermission', 'knowledgeEditor.mobile.saved',
    'knowledgeEditor.mobile.loadWikiFailed', 'knowledgeEditor.mobile.loadFaqFailed', 'knowledgeEditor.mobile.saveWikiFailed',
    'knowledgeEditor.mobile.saveFaqFailed', 'knowledgeEditor.mobile.versionConflict',
  ];
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    for (const key of keys) assert.equal(knowledgeListLabel(locale, key, { version: 3 }).startsWith('knowledgeEditor.'), false, `${locale}:${key}`);
  }
});

test('wiki editor renders localized permission and field copy', async () => {
  const page = await mount('zh-CN', 'wiki', 'guide');
  try {
    const text = page.host.textContent ?? '';
    assert.match(text, /编辑 Wiki 页面/);
    assert.match(text, /页面标题/);
    assert.match(text, /版本 3/);
  } finally { await page.close(); }
});

test('faq editor renders localized labels and localized fallback error', async () => {
  const page = await mount('ja-JP', 'faq', '7', 'error');
  try {
    const text = page.host.textContent ?? '';
    assert.match(text, /FAQエントリを編集/);
    assert.match(text, /標準質問/);
    assert.match(text, /FAQを読み込めません/);
    assert.doesNotMatch(text, /Unable to load FAQ/);
  } finally { await page.close(); }
});

test('shared editor permission enables saving for a viewer workspace role', async () => {
  const page = await mount('en-US', 'faq', undefined, 'ok', 'viewer', 'editor');
  try {
    const save = [...page.host.querySelectorAll('button')].find((button) => button.textContent === 'Save');
    assert.ok(save, 'shared editor permission should expose Save');
  } finally { await page.close(); }
});
