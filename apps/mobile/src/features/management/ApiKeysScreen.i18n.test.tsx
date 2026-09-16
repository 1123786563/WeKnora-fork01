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

const keys = ['back', 'title', 'refresh', 'description', 'roleUnavailable', 'readOnly', 'noWorkspace', 'loadFailed', 'createFailed', 'revokeFailed', 'tokenTitle', 'tokenNote', 'nameLabel', 'namePlaceholder', 'create', 'creating', 'loading', 'empty', 'revokeTitle', 'revokeMessage', 'cancel', 'revoke', 'fullAccess', 'noCapabilities', 'created', 'selectCapability', 'capabilityStatus', 'nameRequired', 'capabilityRequired'] as const;

test('API key management copy resolves in every supported locale', () => {
  for (const locale of supportedLocales) for (const key of keys) {
    const value = formatMessage(locale, `mobileApiKeys.${key}`);
    assert.notEqual(value, `mobileApiKeys.${key}`, `${locale}:${key}`);
    assert.ok(value.trim());
  }
  assert.match(formatMessage('zh-CN', 'mobileApiKeys.revokeMessage', { name: 'demo' }), /demo/);
  assert.notEqual(formatMessage('ko-KR', 'mobileApiKeys.cancel'), formatMessage('ko-KR', 'mobileApiKeys.revoke'));
  assert.match(formatMessage('ko-KR', 'mobileApiKeys.revokeTitle'), /폐기/);
});

async function mount(locale: Locale, result: 'empty' | 'rows' | 'error') {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const runtime: any = {
    locale, tenantId: '7', workspaces: [{ id: 7, role: 'owner' }],
    client: { administration: { tenantApiKeys: { list: async () => {
      if (result === 'error') throw {};
      return result === 'empty' ? [] : [{ id: 1, name: 'demo', full_access: false, capabilities: ['retrieve'], created_at: 'today' }];
    } } } },
  };
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __apiKeysRuntime: runtime });
  const built = await build({ entryPoints: [resolve(root, 'apps/mobile/src/features/management/ApiKeysScreen.tsx')], bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'], plugins: [{ name: 'native-host-fixtures', setup(b) {
    b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
    b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router' ? `export const useRouter=()=>({back:()=>{}});` : path.endsWith('runtime.tsx') ? `export const useMobileRuntime=()=>globalThis.__apiKeysRuntime;` : `const E=({children,...p})=><div {...p}>{children}</div>; export const Alert={alert:()=>{}}; export const View=E; export const SafeAreaView=E; export const Text=E; export const ActivityIndicator=E; export const Pressable=({children,onPress,...p})=><button onClick={onPress} {...p}>{children}</button>; export const TextInput=({value,onChangeText,placeholder,...p})=><input value={value} placeholder={placeholder} onChange={e=>onChangeText(e.target.value)} {...p}/>; export const FlatList=({data,ListEmptyComponent,renderItem})=><div>{data.length ? data.map((item,index)=><div key={index}>{renderItem({item,index})}</div>) : ListEmptyComponent}</div>;` }));
  } }] });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', built.outputFiles[0].text)(require, module, module.exports);
  const host = dom.window.document.getElementById('root')!; const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(module.exports.ApiKeysScreen as React.ComponentType)));
  await act(async () => new Promise((done) => setTimeout(done, 0)));
  return { host, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

test('API key screen renders localized rows, empty state, and fallback error', async () => {
  const rows = await mount('zh-CN', 'rows');
  try { assert.match(rows.host.textContent ?? '', /API 密钥/); assert.match(rows.host.textContent ?? '', /完全访问|retrieve/); } finally { await rows.close(); }
  const empty = await mount('ja-JP', 'empty');
  try { assert.match(empty.host.textContent ?? '', /API キーが返されませんでした/); } finally { await empty.close(); }
  const error = await mount('ru-RU', 'error');
  try { assert.match(error.host.textContent ?? '', /Не удалось загрузить ключи API/); } finally { await error.close(); }
});
