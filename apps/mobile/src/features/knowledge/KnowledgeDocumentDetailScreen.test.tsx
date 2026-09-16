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

async function mount(locale: 'zh-CN' | 'en-US') {
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://weknora.test' });
  const runtime: any = {
    locale,
    baseURL: 'https://weknora.test',
    credential: { kind: 'anonymous' },
    client: { knowledge: { documents: { get: async () => ({ id: 'doc-1', title: '指南', file_name: 'guide.md', file_type: 'text/markdown', file_size: 42, parse_status: 'completed' }), downloadPath: () => '/download', previewPath: () => '/preview' } } },
  };
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true, __documentDetailRuntime: runtime });
  const built = await build({
    entryPoints: [resolve(root, 'apps/mobile/src/features/knowledge/KnowledgeDocumentDetailScreen.tsx')],
    bundle: true, write: false, platform: 'node', format: 'cjs', jsx: 'automatic', external: ['react', 'react/jsx-runtime'],
    plugins: [{ name: 'native-host-fixtures', setup(b) {
      b.onResolve({ filter: /react-native|expo-router|runtime\.tsx$|platform\/files\.ts$|artifact-preview\.tsx$/ }, (args) => ({ path: args.path, namespace: 'mock' }));
      b.onLoad({ filter: /.*/, namespace: 'mock' }, ({ path }) => ({ loader: 'jsx', contents: path === 'expo-router'
        ? `import React from 'react'; export const useLocalSearchParams=()=>({id:'doc-1'}); export const useRouter=()=>({back:()=>{}});`
        : path.endsWith('runtime.tsx')
          ? `export const useMobileRuntime=()=>globalThis.__documentDetailRuntime;`
        : path === 'react-native'
          ? `import React from 'react'; const Box=({children})=><div>{children}</div>; export const AppState={addEventListener:()=>({remove:()=>{}})}; export const View=Box; export const SafeAreaView=Box; export const ScrollView=Box; export const Text=({children})=><span>{children}</span>; export const ActivityIndicator=({accessibilityLabel})=><span aria-label={accessibilityLabel}>loading</span>; export const Pressable=({children,onPress,disabled})=><button disabled={disabled} onClick={onPress}>{children}</button>;`
        : path.endsWith('platform/files.ts')
            ? `export const downloadKnowledgeFile=async()=>{if(globalThis.__documentDetailPreviewDownload) await globalThis.__documentDetailPreviewDownload; return '/tmp/guide.md';}; export const readNativeTextFile=async()=>'# Guide'; export const shareNativeFile=async()=>{};`
        : `import React from 'react'; export const NativeArtifactPreview=({labels,error})=><div>{labels?.back} {labels?.share} {labels?.loading} {labels?.downloadOnly} {error}</div>;` }));
    } }],
  });
  const module = { exports: {} as Record<string, unknown> };
  new Function('require', 'module', 'exports', built.outputFiles[0].text)(require, module, module.exports);
  const Component = module.exports.KnowledgeDocumentDetailScreen as React.ComponentType;
  const host = dom.window.document.getElementById('root')!;
  const renderer = createRoot(host);
  await act(async () => renderer.render(React.createElement(Component)));
  await act(async () => new Promise((done) => setTimeout(done, 0)));
  return { host, close: async () => { await act(async () => renderer.unmount()); dom.window.close(); } };
}

test('document detail copy resolves for zh-CN and en-US', () => {
  for (const locale of ['zh-CN', 'en-US'] as const) {
    const copy = [
      'knowledgeBase.detail.backShort', 'knowledgeBase.detail.title', 'knowledgeBase.detail.loading',
      'knowledgeBase.detail.preview', 'knowledgeBase.detail.preparing', 'knowledgeBase.detail.downloadShare', 'knowledgeBase.detail.downloadOnly',
      'knowledgeBase.detail.previewUnavailable', 'knowledgeBase.detail.previewFailed', 'knowledgeBase.detail.shareFailed',
    ].map((key) => knowledgeListLabel(locale, key, { status: knowledgeListLabel(locale, 'knowledgeBase.timeline.running') })).join(' ');
    assert.equal(copy.includes('knowledgeBase.detail.'), false, locale);
    assert.equal(copy.includes('Back'), locale === 'en-US', locale);
    assert.equal(copy.includes('返回'), locale === 'zh-CN', locale);
  }
});

test('download-only copy is available in all supported locales', () => {
  const expected = {
    'zh-CN': '下载后查看',
    'en-US': 'Download to view',
    'ja-JP': 'ダウンロードして表示',
    'ko-KR': '다운로드하여 보기',
    'ru-RU': 'Скачайте для просмотра',
  } as const;
  for (const locale of Object.keys(expected) as (keyof typeof expected)[]) {
    assert.equal(knowledgeListLabel(locale, 'knowledgeBase.detail.downloadOnly'), expected[locale], locale);
  }
});

test('document detail renders localized zh-CN labels', async () => {
  const page = await mount('zh-CN');
  try {
    assert.match(page.host.textContent ?? '', /文件详情/);
    assert.match(page.host.textContent ?? '', /预览/);
    assert.match(page.host.textContent ?? '', /下载并分享/);
    assert.match(page.host.textContent ?? '', /文档 ID/);
  } finally { await page.close(); }
});

test('document detail renders localized en-US labels', async () => {
  const page = await mount('en-US');
  try {
    assert.match(page.host.textContent ?? '', /File details/);
    assert.match(page.host.textContent ?? '', /Preview/);
    assert.match(page.host.textContent ?? '', /Download and share/);
    assert.match(page.host.textContent ?? '', /Document ID/);
  } finally { await page.close(); }
});

test('document detail disables download actions while native preview is loading', async () => {
  let resolvePreview!: () => void;
  const previewDownload = new Promise<void>((resolve) => { resolvePreview = resolve; });
  (globalThis as { __documentDetailPreviewDownload?: Promise<void> }).__documentDetailPreviewDownload = previewDownload;
  const page = await mount('en-US');
  try {
    const buttons = [...page.host.querySelectorAll('button')];
    const preview = buttons.find((button) => button.textContent === 'Preview')!;
    const download = buttons.find((button) => button.textContent === 'Download and share')!;
    await act(async () => { preview.click(); await Promise.resolve(); });
    assert.equal(preview.disabled, true);
    assert.equal(download.disabled, true);
  } finally {
    resolvePreview();
    delete (globalThis as { __documentDetailPreviewDownload?: Promise<void> }).__documentDetailPreviewDownload;
    await page.close();
  }
});
