import assert from 'node:assert/strict';
import { build } from 'esbuild';
import { JSDOM } from 'jsdom';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import test from 'node:test';
import { formatMessage, supportedLocales, type Locale } from '@weknora/i18n';

const root = resolve(import.meta.dirname, '../../../../..');
const require = createRequire(resolve(root, 'apps/web/package.json'));
const React = require('react') as typeof import('react');
const { act } = React;
const { createRoot } = require('react-dom/client') as { createRoot: (host: Element) => { render(node: unknown): void; unmount(): void } };

const keys = [
  'mobileIdentity.back', 'mobileIdentity.title', 'mobileIdentity.description',
  'mobileIdentity.loading', 'mobileIdentity.loadFailed', 'mobileIdentity.empty',
  'mobileIdentity.supported', 'mobileIdentity.unavailable', 'mobileIdentity.unavailableWithReason',
] as const;

test('identity capability copy resolves in all supported locales', () => {
  for (const locale of supportedLocales) {
    for (const key of keys) assert.notEqual(formatMessage(locale, key), key, `${locale}:${key}`);
    assert.notEqual(formatMessage(locale, 'mobileIdentity.unavailableWithReason', { reason: 'server policy' }), 'mobileIdentity.unavailableWithReason');
  }
});

async function mount(locale: Locale, result: 'empty' | 'rows' | 'error') {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const runtime: any = {
    locale,
    client: { administration: { capabilities: async () => {
      if (result === 'error') throw {};
      return { capabilities: result === 'empty' ? {} : { members: { supported: true }, audit: { supported: false, reason: 'server policy' } } };
    } } },
  };
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __identityRuntime: runtime });
  const built = await build({
    entryPoints: [resolve(root, 'apps/mobile/src/features/management/IdentityScreen.tsx')],
    bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic',
    external: ['react', 'react/jsx-runtime'],
    plugins: [{ name: 'native-host-fixtures', setup(b) {
      b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
      b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router'
        ? `import React from 'react'; export const useRouter=()=>({back:()=>{}});`
        : path.endsWith('runtime.tsx')
          ? `export const useMobileRuntime=()=>globalThis.__identityRuntime;`
          : `import React from 'react'; const View=({children})=><div>{children}</div>; export {View}; export const SafeAreaView=View; export const Text=({children,accessibilityLabel})=><span aria-label={accessibilityLabel}>{children}</span>; export const ActivityIndicator=({accessibilityLabel})=><span aria-label={accessibilityLabel}>loading</span>; export const Pressable=({children,onPress})=><button onClick={onPress}>{children}</button>; export const FlatList=({data,ListEmptyComponent,renderItem})=><div>{data.length ? data.map((item,index)=><div key={index}>{renderItem({item,index})}</div>) : ListEmptyComponent}</div>;` }));
    } }],
  });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', built.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.IdentityScreen as React.ComponentType;
  const host = dom.window.document.getElementById('root')!;
  const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(Component)));
  await act(async () => new Promise((done) => setTimeout(done, 0)));
  return { host, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

test('identity screen renders localized empty and unavailable states', async () => {
  const page = await mount('zh-CN', 'rows');
  try {
    assert.match(page.host.textContent ?? '', /身份与审计/);
    assert.match(page.host.textContent ?? '', /支持/);
    assert.match(page.host.textContent ?? '', /不可用 · server policy/);
  } finally { await page.close(); }
  const empty = await mount('ja-JP', 'empty');
  try { assert.match(empty.host.textContent ?? '', /報告された ID 機能はありません/); }
  finally { await empty.close(); }
});

test('identity screen renders the localized load error fallback', async () => {
  const page = await mount('ru-RU', 'error');
  try { assert.match(page.host.textContent ?? '', /Не удалось загрузить возможности идентификации/); }
  finally { await page.close(); }
});
