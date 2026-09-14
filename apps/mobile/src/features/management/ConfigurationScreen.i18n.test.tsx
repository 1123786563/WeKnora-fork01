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

const keys = ['back', 'title', 'refresh', 'description', 'loading', 'loadFailed', 'operationFailed', 'removeFailed', 'agents', 'models', 'mcp', 'skills', 'noConfigured', 'noReadable', 'readOnlyCatalog', 'configuredAgent', 'noUrl', 'disabled', 'enabled', 'unknownType', 'unknownSource', 'add', 'edit', 'remove', 'removeTitle', 'create', 'nameLabel', 'namePlaceholder', 'descriptionLabel', 'descriptionPlaceholder', 'avatarLabel', 'avatarPlaceholder', 'modelTypeLabel', 'modelTypePlaceholder', 'modelSourceLabel', 'modelSourcePlaceholder', 'apiKeyLabel', 'apiKeyPlaceholder', 'appSecretLabel', 'appSecretPlaceholder', 'mcpUrlLabel', 'mcpUrlPlaceholder', 'transportLabel', 'transportSse', 'transportHttp', 'transportStdio', 'enabledLabel', 'mcpApiKeyLabel', 'mcpTokenLabel', 'tokenPlaceholder', 'detailsLabel', 'detailsPlaceholder', 'saving', 'save', 'cancel'] as const;

test('configuration copy resolves in every supported locale', () => {
  for (const locale of supportedLocales) for (const key of keys) {
    const value = formatMessage(locale, `mobileConfiguration.${key}`);
    assert.notEqual(value, `mobileConfiguration.${key}`, `${locale}:${key}`);
    assert.ok(value.trim());
  }
});

async function mount(locale: Locale, mode: 'empty' | 'error') {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const list = async () => mode === 'error' ? Promise.reject(new Error('server policy')) : [];
  const runtime: any = { locale, client: { configuration: { agents: { list }, models: { list }, mcp: { list }, skills: { list } } } };
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __configurationRuntime: runtime });
  const built = await build({ entryPoints: [resolve(root, 'apps/mobile/src/features/management/ConfigurationScreen.tsx')], bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'], plugins: [{ name: 'native-host-fixtures', setup(b) {
    b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
    b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router' ? `export const useRouter=()=>({back:()=>{}});` : path.endsWith('runtime.tsx') ? `export const useMobileRuntime=()=>globalThis.__configurationRuntime;` : `const E=({children,...p})=><div {...p}>{children}</div>; export const Alert={alert:()=>{}}; export const View=E; export const SafeAreaView=E; export const Text=E; export const ActivityIndicator=E; export const Pressable=({children,onPress,...p})=><button onClick={onPress} {...p}>{children}</button>; export const TextInput=({value,onChangeText,placeholder,...p})=><input value={value} placeholder={placeholder} onChange={e=>onChangeText(e.target.value)} {...p}/>; export const ScrollView=E; export const Switch=({value,onValueChange,...p})=><input type="checkbox" checked={value} onChange={e=>onValueChange(e.target.checked)} {...p}/>;` }));
  } }] });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', built.outputFiles[0].text)(require, module, module.exports);
  const host = dom.window.document.getElementById('root')!; const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(module.exports.ConfigurationScreen as React.ComponentType)));
  await act(async () => new Promise((done) => setTimeout(done, 0)));
  return { host, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

test('configuration screen renders localized empty and fallback error states', async () => {
  const empty = await mount('ja-JP', 'empty');
  try { assert.match(empty.host.textContent ?? '', /設定/); assert.match(empty.host.textContent ?? '', /設定済みの項目はありません/); } finally { await empty.close(); }
  const error = await mount('ru-RU', 'error');
  try { assert.match(error.host.textContent ?? '', /4 Не удалось загрузить конфигурацию/); } finally { await error.close(); }
});
