import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import test, { afterEach } from 'node:test';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => {
  if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
  // @weknora/ui is workspace-linked and Node resolves its real path outside
  // apps/web, so direct node:test runs need the consuming app's React DOM.
  if (specifier === 'react-dom') return { shortCircuit: true, url: new URL('../../node_modules/react-dom/index.js', import.meta.url).href };
  return nextResolve(specifier, context);
};
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register('data:text/javascript,' + encodeURIComponent([
  'export async function resolve(specifier, context, nextResolve) {',
  "  if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };",
  "  if (specifier === 'react-dom') return { shortCircuit: true, url: new URL('../../node_modules/react-dom/index.js', import.meta.url).href };",
  '  return nextResolve(specifier, context);',
  '}',
].join('\n')), import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/knowledgeBase/kb-1/documents/doc-1' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');

const { KnowledgeDocumentDetailPage, knowledgeTraceNodeState, metadataRowsFromObject, metadataRowsToObject, validateMetadataRows } = await import('./KnowledgeDocumentDetailPage.tsx');

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

function detailClient(get: () => Promise<Record<string, unknown>>, role = 'contributor', knowledgeBaseOwnerId = 'user-1', chunkRows: Array<Record<string, unknown>> = [{ id: 'chunk-1', content: 'Current chunk', content_revision: 3, is_enabled: true }]) {
  return {
    knowledgeBases: {
      documents: {
        get,
        previewPath: (id: string) => `/api/v1/knowledge/${id}/preview`,
        downloadPath: (id: string) => `/api/v1/knowledge/${id}/download`,
        spans: async () => ({ trace: null }),
        preview: async () => ({ body: '', headers: {}, contentType: 'text/plain' }),
        download: async () => ({ body: '', headers: {}, contentType: 'text/plain' }),
        reparse: async () => undefined,
        cancelParse: async () => undefined,
        chunks: async () => ({ data: chunkRows, total: chunkRows.length, page: 1, page_size: 25 }),
        updateChunk: async () => ({ id: 'chunk-1', content: 'Updated chunk', content_revision: 4, is_enabled: true }),
        chunkRevisions: async () => [{ revision: 2, content: 'Previous chunk', is_enabled: true }],
        revertChunk: async () => ({ id: 'chunk-1', content: 'Previous chunk', content_revision: 5, is_enabled: true }),
        updateDetails: async (_id: string, input: unknown) => ({ id: 'doc-1', ...(input as object) }),
      },
      settings: { get: async () => ({ id: 'kb-1', user_id: knowledgeBaseOwnerId }) },
    },
    auth: { me: async () => ({ user: { id: 'user-1', role }, memberships: [{ role }] }) },
  } as never;
}

test('metadata rows preserve Vue scalar types and serialize a trimmed-key object', () => {
  const rows = metadataRowsFromObject({ title: 'Guide', count: 3, enabled: true, missing: null });
  assert.deepEqual(rows.map(({ key, value, type }) => ({ key, value, type })), [
    { key: 'title', value: 'Guide', type: 'text' },
    { key: 'count', value: '3', type: 'number' },
    { key: 'enabled', value: 'true', type: 'boolean' },
    { key: 'missing', value: '', type: 'null' },
  ]);
  assert.deepEqual(metadataRowsToObject([
    { id: 1, key: ' title ', value: 'Guide', type: 'text' },
    { id: 2, key: 'count', value: '3.5', type: 'number' },
    { id: 3, key: 'enabled', value: 'false', type: 'boolean' },
    { id: 4, key: 'missing', value: '', type: 'null' },
  ]), { title: 'Guide', count: 3.5, enabled: false, missing: null });
});

test('metadata validation rejects empty or duplicate keys and invalid numbers', () => {
  const emptyKey = validateMetadataRows([{ id: 1, key: ' ', value: 'x', type: 'text' }]);
  const duplicateKey = validateMetadataRows([
    { id: 1, key: 'owner', value: 'a', type: 'text' },
    { id: 2, key: ' owner ', value: 'b', type: 'text' },
  ]);
  const invalidNumber = validateMetadataRows([{ id: 1, key: 'count', value: 'NaN', type: 'number' }]);
  assert.equal((emptyKey as { message?: string }).message, '元数据字段名不能为空');
  assert.equal((duplicateKey as { message?: string }).message, '元数据字段 owner 重复');
  assert.equal((invalidNumber as { message?: string }).message, '元数据字段 count 必须是有效数字');
});

test('metadata editor submits structured values and keeps the draft after a failed save', async () => {
  let updateInput: unknown;
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.pdf', source: 'file', file_type: 'pdf', parse_status: 'completed',
    custom_metadata: { owner: 'docs' },
  }));
  (client as any).knowledgeBases.documents.updateDetails = async (_id: string, input: unknown) => {
    updateInput = input;
    throw new Error('save unavailable');
  };
  const container = await mountDetail(client);
  const metadataSection = container.ownerDocument.body.querySelector('[aria-label="自定义元数据"]');
  const edit = Array.from(metadataSection?.querySelectorAll('button') || []).find((button) => button.textContent === '编辑');
  assert.ok(edit);
  await act(async () => { edit!.click(); });
  const key = metadataSection?.querySelector('input[placeholder="字段名"]') as HTMLInputElement;
  const value = metadataSection?.querySelector('input[placeholder="字段值"]') as HTMLInputElement;
  assert.ok(key && value, 'structured key/value controls replace the JSON editor');
  await act(async () => {
    Object.getOwnPropertyDescriptor(value.ownerDocument.defaultView!.HTMLInputElement.prototype, 'value')?.set?.call(value, '42');
    value.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => {
    const type = metadataSection?.querySelector('select') as HTMLSelectElement;
    Object.getOwnPropertyDescriptor(type.ownerDocument.defaultView!.HTMLSelectElement.prototype, 'value')?.set?.call(type, 'number');
    type.dispatchEvent(new Event('change', { bubbles: true }));
  });
  await act(async () => {
    (Array.from(container.ownerDocument.body.querySelectorAll('[aria-label="自定义元数据"] button')).find((button) => button.textContent === '保存') as HTMLButtonElement | undefined)?.click();
  });
  await act(async () => {});
  assert.deepEqual(updateInput, { custom_metadata: { owner: 42 } });
  assert.ok(container.ownerDocument.body.textContent?.includes('save unavailable'));
  assert.ok(metadataSection?.querySelector('input[placeholder="字段值"]'), 'failed save retains row editing');
});

async function mountDetail(client: never, documentId = 'doc-1') {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeDocumentDetailPage client={client} documentId={documentId} onBack={() => {}} />);
  });
  await act(async () => {});
  await act(async () => {});
  return container;
}

test('document trace nodes expose the same status semantics as the Vue trace surface', () => {
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { status: 'failed' } }), 'failed');
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { status: 'running' } }), 'running');
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { status: 'completed' } }), 'done');
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: {} }), 'pending');
});

test('document detail uses the Vue document title-row anatomy', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentDetailPage, {
    client: {} as never,
    documentId: 'doc-1',
    onBack: () => {},
  }));

  assert.ok(html.includes('document-title-row'), 'detail header keeps the Vue title row');
  assert.ok(html.includes('document-breadcrumb'), 'detail header uses breadcrumb anatomy');
  assert.ok(html.includes('文档'), 'detail header keeps the localized document label');
  assert.ok(html.includes('返回'), 'detail header keeps the back action');
  assert.ok(!html.includes('wk-eyebrow'), 'React-only eyebrow is not rendered');
});

test('loaded file detail matches the Vue drawer title and metadata anatomy', async () => {
  const container = await mountDetail(detailClient(async () => ({
    id: 'doc-1',
    knowledge_base_id: 'kb-1',
    file_name: 'Deployment Guide.pdf',
    source: 'file',
    file_type: 'pdf',
    parse_status: 'pending',
    folder_path: 'Guides',
    created_at: '2026-09-15T13:36:00Z',
  })));

  const surface = container.ownerDocument.body;
  assert.equal(surface.querySelector('.breadcrumb-current')?.textContent, 'Deployment Guide', 'Vue removes a file extension from the detail title');
  assert.ok(surface.querySelector('.wk-document-detail-surface'), 'detail content uses the grouped drawer-like surface');
  assert.ok(surface.querySelector('.wk-document-metadata-section'), 'Vue basic-info section is retained');
  assert.ok(surface.textContent?.includes('基本信息'), 'metadata section has the Vue title');
});

test('detail load errors retain navigation and provide a retry without exposing the route document ID', async () => {
  let attempts = 0;
  const container = await mountDetail(detailClient(async () => {
    attempts += 1;
    throw new Error('network unavailable');
  }), 'private-doc-id');

  const surface = container.ownerDocument.body;
  assert.equal(surface.querySelector('[role="alert"]')?.textContent, 'network unavailable');
  assert.equal(surface.querySelector('.breadcrumb-current')?.textContent, '文档', 'a failed load does not present the opaque route ID as a title');
  const retry = Array.from(surface.querySelectorAll('button')).find((button) => button.textContent === '重试');
  assert.ok(retry, 'error state offers the Vue-style retry affordance');
  await act(async () => { retry!.click(); });
  await act(async () => {});
  assert.equal(attempts, 2, 'retry performs a new document request');
});

test('viewer detail keeps Vue download affordances hidden until permission grants access', async () => {
  const container = await mountDetail(detailClient(async () => ({
    id: 'doc-1',
    knowledge_base_id: 'kb-1',
    file_name: 'Viewer Guide.pdf',
    source: 'file',
    file_type: 'pdf',
    parse_status: 'pending',
  }), 'viewer', 'owner-1'));

  assert.equal(Array.from(container.querySelectorAll('button')).some((button) => button.textContent?.includes('下载')), false, 'viewer cannot see the original-file download action');
});

test('detail route renders the Vue right drawer and real chunk/history controls for editors', async () => {
  const container = await mountDetail(detailClient(async () => ({
    id: 'doc-1',
    knowledge_base_id: 'kb-1',
    file_name: 'Guide.pdf',
    source: 'file',
    file_type: 'pdf',
    parse_status: 'completed',
  })));

  const dialog = container.ownerDocument.body.querySelector('[role="dialog"]');
  assert.ok(dialog, 'detail is a right-side drawer surface');
  assert.equal(dialog?.getAttribute('data-side'), 'right');
  assert.ok(dialog?.querySelector('[aria-label="关闭"]'), 'drawer has an explicit close action');
  assert.ok(dialog?.querySelector('[aria-label="Resize drawer"]'), 'drawer exposes the Vue resize affordance');
  assert.ok(container.ownerDocument.body.querySelector('.wk-document-chunks'), 'detail loads real chunk content');
  assert.ok(container.ownerDocument.body.textContent?.includes('Current chunk'));
  const historyButton = Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent?.includes('编辑历史'));
  assert.ok(historyButton, 'editor can open chunk history');
  await act(async () => { historyButton!.click(); });
  assert.ok(container.ownerDocument.body.textContent?.includes('Previous chunk'), 'history is loaded from the chunk revision seam');
});

test('detail drawer exposes Vue header download and conditional document metadata', async () => {
  const downloads: string[] = [];
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.pdf', source: 'file', file_type: 'pdf', parse_status: 'completed',
    time: '2026-09-15T13:36:00Z', channel: 'feishu', tags: [{ id: 'tag-1', name: '重要' }],
  }));
  (client as any).knowledgeBases.documents.download = async () => {
    downloads.push('doc-1');
    return { body: 'bytes', headers: {}, contentType: 'application/pdf' };
  };
  const container = await mountDetail(client);
  const dialog = container.ownerDocument.body.querySelector('[role="dialog"]');
  assert.ok(dialog?.querySelector('button[aria-label="下载"]'), 'Vue header keeps a compact download action');
  assert.ok(dialog?.textContent?.includes('feishu'), 'channel metadata is shown when it is not the web source');
  assert.ok(dialog?.textContent?.includes('重要'), 'document tags remain visible in the detail drawer');
  await act(async () => { (dialog?.querySelector('button[aria-label="下载"]') as HTMLButtonElement).click(); });
  await act(async () => {});
  assert.deepEqual(downloads, ['doc-1']);
});

test('summary editor follows Vue permission and retains its draft after a failed save', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', source: 'file', file_type: 'md', parse_status: 'completed', description: 'Original summary',
  }));
  (client as any).knowledgeBases.documents.updateDetails = async () => { throw new Error('summary save unavailable'); };
  const container = await mountDetail(client);
  const summary = Array.from(container.ownerDocument.body.querySelectorAll('section')).find((section) => section.textContent?.includes('摘要'));
  const edit = Array.from(summary?.querySelectorAll('button') || []).find((button) => button.textContent === '编辑');
  assert.ok(edit);
  await act(async () => { edit!.click(); });
  const textarea = summary?.querySelector('textarea') as HTMLTextAreaElement;
  assert.ok(textarea);
  await act(async () => {
    Object.getOwnPropertyDescriptor(textarea.ownerDocument.defaultView!.HTMLTextAreaElement.prototype, 'value')?.set?.call(textarea, 'Updated summary');
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => { Array.from(summary?.querySelectorAll('button') || []).find((button) => button.textContent === '保存')?.click(); });
  await act(async () => {});
  assert.ok(summary?.querySelector('textarea'), 'failed save keeps the Vue inline editor open');
  assert.equal((summary?.querySelector('textarea') as HTMLTextAreaElement).value, 'Updated summary');
  assert.ok(container.ownerDocument.body.textContent?.includes('summary save unavailable'));
});

test('document content exposes Vue preview, merged and chunks tabs and merges chunks in index order', async () => {
  const container = await mountDetail(detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', source: 'file', file_type: 'md', parse_status: 'completed',
  }), 'contributor', 'user-1', [
    { id: 'chunk-2', chunk_index: 2, content: 'Second', content_revision: 1, is_enabled: true },
    { id: 'chunk-1', chunk_index: 1, content: 'First', content_revision: 1, is_enabled: true },
  ]));

  const buttons = () => Array.from(container.ownerDocument.body.querySelectorAll('button'));
  assert.ok(buttons().some((button) => button.textContent === '预览'));
  assert.ok(buttons().some((button) => button.textContent === '全文'));
  assert.ok(buttons().some((button) => button.textContent === '查看分块'));
  const merged = buttons().find((button) => button.textContent === '全文');
  await act(async () => { merged!.click(); });
  assert.ok(container.ownerDocument.body.textContent?.includes('First'));
  assert.ok(container.ownerDocument.body.textContent?.indexOf('First')! < container.ownerDocument.body.textContent?.indexOf('Second')!);
});

test('chunk enabled state can be toggled and failed indexing can be retried', async () => {
  const updates: unknown[] = [];
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.txt', source: 'file', file_type: 'txt', parse_status: 'completed',
  }), 'contributor', 'user-1', [{ id: 'chunk-1', content: 'Disabled chunk', content_revision: 3, is_enabled: false, index_status: 'failed' }]);
  const documents = (client as any).knowledgeBases.documents;
  documents.updateChunk = async (_id: string, _chunkId: string, input: { is_enabled?: boolean }) => {
    updates.push(input);
    return { id: 'chunk-1', content: 'Disabled chunk', content_revision: 4, is_enabled: input.is_enabled ?? false, index_status: input.is_enabled === undefined ? 'failed' : 'completed' };
  };
  const container = await mountDetail(client);
  const enable = Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '启用');
  assert.ok(enable);
  const retry = Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '重试');
  assert.ok(retry);
  await act(async () => { retry!.click(); });
  const refreshedEnable = Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '启用');
  assert.ok(refreshedEnable);
  await act(async () => { refreshedEnable!.click(); });
  assert.deepEqual(updates, [
    { expected_revision: 3 },
    { is_enabled: true, expected_revision: 4 },
  ]);
});
