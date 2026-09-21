import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import * as React from 'react';
import * as XLSX from 'xlsx';
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

const { KnowledgeDocumentDetailPage, isLegacyGeneratedQuestion, knowledgeTraceNodeState, metadataRowsFromObject, metadataRowsToObject, validateMetadataRows } = await import('./KnowledgeDocumentDetailPage.tsx');

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

function detailClient(get: (id: string) => Promise<Record<string, unknown>>, role = 'contributor', knowledgeBaseOwnerId = 'user-1', chunkRows: Array<Record<string, unknown>> = [{ id: 'chunk-1', content: 'Current chunk', content_revision: 3, is_enabled: true }]) {
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
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.pdf', type: 'file', file_type: 'pdf', parse_status: 'completed',
    custom_metadata: { owner: 'docs' },
  }));
  (client as any).knowledgeBases.documents.updateDetails = async (_id: string, input: unknown) => {
    updateInput = input;
    throw new Error('save unavailable');
  };
  const container = await mountDetail(client);
  const metadataSection = container.ownerDocument.body.querySelector('[aria-label="自定义元数据"]');
  const edit = Array.from(metadataSection?.querySelectorAll('button') || []).find((button) => button.getAttribute('aria-label') === '编辑');
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

test('failed chunk retry follows the Vue retryIndex contract: copy, payload, and success feedback', async () => {
  const updateCalls: Array<Record<string, unknown>> = [];
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'contributor', 'user-1', [
    { id: 'chunk-1', content: 'Broken chunk', content_revision: 3, is_enabled: true, index_status: 'failed' },
  ]);
  (client as any).knowledgeBases.documents.updateChunk = async (_documentId: string, _chunkId: string, input: Record<string, unknown>) => {
    updateCalls.push(input);
    return { id: 'chunk-1', content: 'Broken chunk', content_revision: 4, is_enabled: true, index_status: 'completed' };
  };
  const container = await mountDetail(client);
  await act(async () => { Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '查看分块')!.click(); });

  const body = () => container.ownerDocument.body;
  const retryButton = () => Array.from(body().querySelectorAll('button')).find((button) => button.textContent === '重试索引');
  const retry = retryButton();
  assert.ok(retry, 'Vue labels the failed-index action with knowledgeBase.retryIndex (重试索引), not common.retry');
  assert.equal(retry?.getAttribute('title') || retry?.getAttribute('aria-label'), '重试索引');

  await act(async () => { retry!.click(); });
  await act(async () => {});
  assert.deepEqual(updateCalls, [{ expected_revision: 3 }], 'Vue retryChunkIndex sends only expected_revision, no content or is_enabled');
  assert.ok(body().textContent?.includes('索引已同步'), 'Vue shows the indexRetrySuccess message after a successful retry');
  assert.ok(!retryButton(), 'a recovered chunk no longer offers the retry action');
});

test('a retry that stays failed surfaces the Vue indexFailed error and keeps the retry control', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'contributor', 'user-1', [
    { id: 'chunk-1', content: 'Broken chunk', content_revision: 3, is_enabled: true, index_status: 'failed' },
  ]);
  (client as any).knowledgeBases.documents.updateChunk = async () => ({ id: 'chunk-1', content: 'Broken chunk', content_revision: 4, is_enabled: true, index_status: 'failed' });
  const container = await mountDetail(client);
  await act(async () => { Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '查看分块')!.click(); });

  const retry = () => Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '重试索引');
  assert.ok(retry());
  await act(async () => { retry()!.click(); });
  await act(async () => {});
  assert.ok(container.ownerDocument.body.textContent?.includes('索引同步失败'), 'Vue shows indexFailed when the returned chunk is still failed');
  assert.ok(retry(), 'the retry affordance remains for a still-failed chunk');
});

test('retry isolates its loading state from the enable/disable control and viewers see no retry action', async () => {
  let release: ((value: Record<string, unknown>) => void) | undefined;
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'contributor', 'user-1', [
    { id: 'chunk-1', content: 'Broken chunk', content_revision: 3, is_enabled: true, index_status: 'failed' },
  ]);
  (client as any).knowledgeBases.documents.updateChunk = async () => new Promise((resolve) => { release = resolve; });
  const container = await mountDetail(client);
  await act(async () => { Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '查看分块')!.click(); });

  const retry = () => Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '重试索引') as HTMLButtonElement;
  const toggle = () => Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '停用') as HTMLButtonElement;
  await act(async () => { retry().click(); });
  assert.ok(retry().disabled, 'an in-flight retry cannot be double-submitted');
  assert.ok(!toggle().disabled, 'Vue retry loading never disables the chunk enable/disable switch');
  await act(async () => { release!({ id: 'chunk-1', content: 'Broken chunk', content_revision: 4, is_enabled: true, index_status: 'completed' }); });
  await act(async () => {});
  assert.ok(!retry(), 'retry clears after completion');

  const viewer = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'viewer', 'owner-1', [
    { id: 'chunk-1', content: 'Broken chunk', content_revision: 3, is_enabled: true, index_status: 'failed' },
  ]);
  mountedRoot?.unmount();
  document.body.replaceChildren();
  const viewerContainer = await mountDetail(viewer);
  assert.equal(Array.from(viewerContainer.ownerDocument.body.querySelectorAll('button')).some((button) => button.textContent === '重试索引'), false, 'viewers never see the chunk retry control');
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

// R472-A1 (R470 遗留): a disabled multimodal stage closes as status
// 'skipped' with started_at/finished_at timestamps (Vue timeline shows
// knowledgeStages.status.skipped 已跳过); the trace rows must not fall
// back to running/pending, and finished_at must read like Vue nodeEnd.
test('trace node state recognizes skipped spans and finished_at/started_at aliases', () => {
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { status: 'skipped', started_at: '2026-09-18T10:00:05Z', finished_at: '2026-09-18T10:00:05Z' } }), 'skipped');
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { finished_at: '2026-09-18T10:00:02Z' } }), 'done');
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { started_at: '2026-09-18T10:00:02Z' } }), 'running');
  // R474/A3: an explicit status 'pending' span that already serialized
  // started_at stays pending — Vue reads node.status verbatim (the drawer
  // row then renders '—' with no status text); the started_at fallback
  // must not swallow it into running/进行中.
  assert.equal(knowledgeTraceNodeState({ key: 'root', depth: 0, hasChildren: false, node: { status: 'pending', started_at: '2026-09-18T10:00:02Z', duration_ms: 500 } }), 'pending');
});

// R469/A3 N009 capture: the /spans endpoint replies with the backend envelope
// { success: true, data: { parse_status, current_stage, trace, ... } }
// (internal/handler/knowledge.go GetKnowledgeSpans); the raw api-client body
// is the envelope itself, so the documents consumers must unwrap data like
// Vue's res.data before reading the trace tree.
const R469_COMPLETED_SPANS = {
  success: true,
  data: {
    knowledge_id: 'a20e53a5',
    attempt: 1,
    latest_attempt: 1,
    current_attempt: 1,
    parse_status: 'completed',
    current_stage: '',
    trace: {
      span_id: 'root-1',
      kind: 'root',
      name: 'knowledge_processing',
      status: 'done',
      started_at: '2026-09-17T10:00:00Z',
      finished_at: '2026-09-17T10:02:10.400Z',
      duration_ms: 130400,
      children: [
        { span_id: 's1', kind: 'stage', name: 'docreader', status: 'done', duration_ms: 7 },
        { span_id: 's2', kind: 'stage', name: 'chunking', status: 'done', duration_ms: 13 },
        { span_id: 's3', kind: 'stage', name: 'embedding', status: 'done', duration_ms: 5 },
        { span_id: 's4', kind: 'stage', name: 'multimodal', status: 'skipped' },
        {
          span_id: 's5', kind: 'stage', name: 'postprocess', status: 'done', duration_ms: 9,
          children: [
            { span_id: 's5-1', kind: 'span', name: 'postprocess.summary', status: 'failed', duration_ms: 30017, error_code: 'LLM_TIMEOUT', error_message: 'summary timed out' },
            { span_id: 's5-2', kind: 'span', name: 'postprocess.summary', status: 'failed', duration_ms: 30025, error_code: 'LLM_TIMEOUT', error_message: 'summary timed out' },
            { span_id: 's5-3', kind: 'span', name: 'postprocess.summary', status: 'failed', duration_ms: 30031, error_code: 'LLM_TIMEOUT', error_message: 'summary timed out' },
            { span_id: 's5-4', kind: 'span', name: 'postprocess.summary', status: 'failed', duration_ms: 30040, error_code: 'LLM_TIMEOUT', error_message: 'summary timed out' },
          ],
        },
      ],
    },
    last_error: null,
  },
};

test('trace drawer consumes the {success,data} spans envelope and renders the completed trace (N009)', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'weknora-upload-test.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }));
  (client as { knowledgeBases: { documents: { spans: (id: string) => Promise<unknown> } } }).knowledgeBases.documents.spans = async () => R469_COMPLETED_SPANS;
  const container = await mountDetail(client);
  await act(async () => {
    (container.ownerDocument.body.querySelector('button[aria-label="解析进度"]') as HTMLButtonElement).click();
  });
  await act(async () => {});
  const text = container.ownerDocument.body.textContent || '';
  // Pre-fix symptom: every pill rendered 等待中 because the trace tree was
  // read off the unwrapped envelope (spans.trace === undefined).
  assert.ok(text.includes('文档解析已完成'), 'docreader pill shows 已完成 like the Vue trace drawer');
  assert.ok(text.includes('分块已完成'));
  assert.ok(text.includes('向量化已完成'));
  assert.ok(text.includes('后处理失败'), 'the 4 failed postprocess.summary retries roll up onto the postprocess pill');
  // Waterfall rows: root total plus the failed summary sub-spans with durations.
  assert.ok(text.includes('postprocess.summary'), 'failed summary sub-spans render in the waterfall');
  assert.ok(text.includes('30017ms'));
  assert.ok(text.includes('130400ms'), 'the root total duration (≈2m10.4s) renders like the Vue drawer');
});

test('inline processing timeline consumes envelope-wrapped spans while parsing (N009)', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'weknora-upload-test.md', type: 'file', file_type: 'md', parse_status: 'processing',
  }));
  (client as { knowledgeBases: { documents: { spans: (id: string) => Promise<unknown> } } }).knowledgeBases.documents.spans = async () => ({
    success: true,
    data: {
      parse_status: 'processing',
      current_stage: 'chunking',
      trace: {
        kind: 'root', name: 'knowledge_processing', status: 'running',
        children: [
          { kind: 'stage', name: 'docreader', status: 'done', duration_ms: 7 },
          { kind: 'stage', name: 'chunking', status: 'running' },
        ],
      },
    },
  });
  const container = await mountDetail(client);
  await act(async () => {});
  await act(async () => {});
  const text = container.ownerDocument.body.textContent || '';
  assert.ok(text.includes('文档解析 — 已完成'), 'the inline card shows the real docreader state while parsing');
  assert.ok(text.includes('分块 — 进行中'));
});

// R473 capture / R474-A3: the very same pending span — an explicit
// status 'pending' stage that already serialized started_at — showed
// 进行中 in React while the Vue trace drawer (knowledge-processing-timeline.vue)
// reads node.status verbatim: the waterfall row carries no status text,
// formatSpanDuration renders '—' for pending, and the drawer head shows
// the LIVE badge plus the 当前阶段 n/5 counter.
test('trace drawer matches the Vue pending contract: no per-row status text, — duration, LIVE badge, stage counter', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'weknora-upload-test.md', type: 'file', file_type: 'md', parse_status: 'processing',
  }));
  (client as { knowledgeBases: { documents: { spans: (id: string) => Promise<unknown> } } }).knowledgeBases.documents.spans = async () => ({
    success: true,
    data: {
      parse_status: 'processing',
      current_stage: 'chunking',
      trace: {
        kind: 'root', name: 'knowledge_processing', status: 'running',
        children: [
          { kind: 'stage', name: 'docreader', status: 'done', duration_ms: 7, started_at: '2026-09-18T10:00:00Z', finished_at: '2026-09-18T10:00:00Z' },
          { kind: 'stage', name: 'chunking', status: 'pending', started_at: '2026-09-18T10:00:00Z', duration_ms: 500 },
        ],
      },
    },
  });
  const container = await mountDetail(client);
  await act(async () => {
    (container.ownerDocument.body.querySelector('button[aria-label="解析进度"]') as HTMLButtonElement).click();
  });
  await act(async () => {});
  // The inline processing card also carries .wk-processing-timeline, so the
  // drawer assertions read the body: the LIVE badge, the stage counter, and
  // the waterfall rows only exist inside the opened trace drawer.
  const drawer = container.ownerDocument.body;
  assert.ok(drawer.querySelector('.wk-trace-live'), 'the drawer shows the Vue LIVE badge while the trace is live');
  const drawerText = drawer.textContent || '';
  assert.ok(drawerText.includes('LIVE'), 'the drawer shows the Vue LIVE badge while the trace is live');
  assert.ok(drawerText.includes('当前阶段'), 'the drawer shows the Vue stagesProgress counter');
  assert.ok(drawerText.includes('2/5'), 'the counter counts done stages plus the pending one like Vue currentStageIndex');
  // Waterfall rows: the pending chunking row carries no status text and a
  // '—' duration even though duration_ms is present.
  const waterfallRows = Array.from(drawer.querySelectorAll('.overflow-x-auto li')) as HTMLLIElement[];
  const chunkingRow = waterfallRows.find((row) => (row.textContent || '').includes('chunking'));
  assert.ok(chunkingRow, 'the chunking span renders a waterfall row');
  assert.equal(chunkingRow.getAttribute('data-state'), 'pending');
  assert.ok(!(chunkingRow.textContent || '').includes('进行中'), 'a pending row no longer reads 进行中');
  assert.ok(!(chunkingRow.textContent || '').includes('等待中'), 'Vue waterfall rows carry no per-row status text');
  assert.ok((chunkingRow.textContent || '').includes('—'), 'the pending row renders the Vue — duration placeholder');
});

test('document detail uses the Vue document title-row anatomy', () => {
  const html = renderToStaticMarkup(React.createElement(KnowledgeDocumentDetailPage, {
    client: {} as never,
    documentId: 'doc-1',
    onBack: () => {},
  }));

  assert.ok(html.includes('doc-drawer-header-title'), 'detail header keeps the Vue doc-drawer-header-title anatomy');
  assert.ok(html.includes('doc-drawer-header-icon'), 'detail header keeps the Vue icon slot');
  assert.ok(html.includes('文档'), 'detail header keeps the localized document label');
  assert.ok(!html.includes('wk-eyebrow'), 'React-only eyebrow is not rendered');
});

test('loaded file detail matches the Vue drawer title and metadata anatomy', async () => {
  const container = await mountDetail(detailClient(async () => ({
    id: 'doc-1',
    knowledge_base_id: 'kb-1',
    file_name: 'Deployment Guide.pdf',
    type: 'file',
    file_type: 'pdf',
    parse_status: 'pending',
    folder_path: 'Guides',
    created_at: '2026-09-15T13:36:00Z',
  })));

  const surface = container.ownerDocument.body;
  assert.equal(surface.querySelector('.doc-drawer-header-title')?.textContent, 'Deployment Guide', 'Vue removes a file extension from the detail title');
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
  assert.equal(surface.querySelector('.doc-drawer-header-title')?.textContent, '文档', 'a failed load does not present the opaque route ID as a title');
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
    type: 'file',
    file_type: 'pdf',
    parse_status: 'pending',
  }), 'viewer', 'owner-1'));

  assert.equal(Array.from(container.querySelectorAll('button')).some((button) => button.textContent?.includes('下载')), false, 'viewer cannot see the original-file download action');
});

test('independent document detail grants tenant admin and contributor mutation affordances', async () => {
  for (const role of ['admin', 'contributor']) {
    const container = await mountDetail(detailClient(async () => ({
      id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Tenant Guide.pdf', type: 'file', file_type: 'pdf', parse_status: 'completed',
    }), role, 'another-user'));
    await act(async () => {});
    await act(async () => {});
    await act(async () => { await Promise.resolve(); });
    assert.ok(Array.from(container.ownerDocument.body.querySelectorAll('button')).some((button) => /下载|download/i.test(button.getAttribute('aria-label') ?? button.textContent ?? '')), `${role} can download from the independent detail route`);
    assert.ok(Array.from(container.ownerDocument.body.querySelectorAll('button')).some((button) => button.textContent === '编辑'), `${role} can edit document details`);
    mountedRoot?.unmount();
    mountedRoot = undefined;
    document.body.replaceChildren();
  }
});

test('detail route renders the Vue right drawer and real chunk/history controls for editors', async () => {
  const container = await mountDetail(detailClient(async () => ({
    id: 'doc-1',
    knowledge_base_id: 'kb-1',
    file_name: 'Guide.pdf',
    type: 'file',
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
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.pdf', type: 'file', file_type: 'pdf', parse_status: 'completed',
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
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed', description: 'Original summary',
  }));
  (client as any).knowledgeBases.documents.updateDetails = async () => { throw new Error('summary save unavailable'); };
  const container = await mountDetail(client);
  const summary = container.ownerDocument.body.querySelector('section[aria-label="摘要"]');
  const edit = Array.from(summary?.querySelectorAll('button') || []).find((button) => button.getAttribute('aria-label') === '编辑');
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

test('merged view keeps the Vue chunk pagination so multi-page documents merge past page one', async () => {
  const pages: Array<number> = [];
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }));
  const rowsByPage: Record<number, Array<Record<string, unknown>>> = {
    1: [{ id: 'c1', chunk_index: 1, content: 'Page one chunk', content_revision: 1, is_enabled: true }],
    2: [{ id: 'c2', chunk_index: 26, content: 'Page two chunk', content_revision: 1, is_enabled: true }],
  };
  (client as any).knowledgeBases.documents.chunks = async (_id: string, page = 1) => {
    pages.push(page);
    return { data: rowsByPage[page] ?? [], total: 30, page, page_size: 25 };
  };
  const container = await mountDetail(client);
  const body = () => container.ownerDocument.body;
  await act(async () => { Array.from(body().querySelectorAll('button')).find((button) => button.textContent === '全文')!.click(); });
  assert.ok(body().textContent?.includes('Page one chunk'));
  const nav = () => body().querySelector('nav[aria-label="查看分块"]');
  assert.ok(nav(), 'Vue renders the chunk pagination for the merged view too (viewMode merged || chunks)');
  await act(async () => { Array.from(nav()!.querySelectorAll('button')).at(-1)!.click(); });
  await act(async () => {});
  assert.ok(body().textContent?.includes('Page two chunk'), 'the merged view advances to the next chunk page');
  assert.deepEqual(pages, [1, 2]);
});

test('chunk page transitions follow the Vue form: pagination and header stay, content swaps to a small local loading row', async () => {
  let releasePageTwo!: () => void;
  const pageTwo = new Promise<Record<string, unknown>>((resolve) => { releasePageTwo = () => resolve({ data: [{ id: 'c2', chunk_index: 26, content: 'Page two chunk', content_revision: 1, is_enabled: true }], total: 30, page: 2, page_size: 25 }); });
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }));
  (client as any).knowledgeBases.documents.chunks = async (_id: string, page = 1) => {
    if (page === 1) return { data: [{ id: 'c1', chunk_index: 1, content: 'Page one chunk', content_revision: 1, is_enabled: true }], total: 30, page: 1, page_size: 25 };
    return pageTwo;
  };
  const container = await mountDetail(client);
  const body = () => container.ownerDocument.body;
  await act(async () => { Array.from(body().querySelectorAll('button')).find((button) => button.textContent === '全文')!.click(); });
  assert.ok(body().textContent?.includes('Page one chunk'));
  const nav = () => body().querySelector('nav[aria-label="查看分块"]');
  assert.ok(nav());
  await act(async () => { Array.from(nav()!.querySelectorAll('button')).at(-1)!.click(); });
  assert.ok(body().querySelector('.wk-chunk-page-loading'), 'a small local loading row replaces the whole-block 加载中 during page transitions');
  assert.ok(nav(), 'Vue keeps chunk-pagination mounted during the transition');
  assert.ok(body().textContent?.includes('查看分块'), 'the section header stays mounted during the transition');
  assert.equal(body().textContent?.includes('Page one chunk'), false, 'Vue v-else hides the stale page while the next page loads');
  assert.equal(nav()!.textContent?.includes('2'), true, 'the pagination shows the requested page like the Vue v-model chunkPage');
  releasePageTwo();
  await act(async () => {});
  assert.ok(body().textContent?.includes('Page two chunk'));
  assert.equal(body().querySelector('.wk-chunk-page-loading'), null);
  assert.ok(nav());
  await act(async () => { Array.from(body().querySelectorAll('button')).find((button) => button.textContent === '查看分块')!.click(); });
  assert.ok(body().textContent?.includes('片段 26'), 'segment numbering follows the loaded page like Vue loadedChunkPage');
});

test('a failed page transition stays on the loaded page with the old content like the Vue chunkLoadError branch', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }));
  (client as any).knowledgeBases.documents.chunks = async (_id: string, page = 1) => {
    if (page === 1) return { data: [{ id: 'c1', chunk_index: 1, content: 'Page one chunk', content_revision: 1, is_enabled: true }], total: 30, page: 1, page_size: 25 };
    throw new Error('page two unavailable');
  };
  const container = await mountDetail(client);
  const body = () => container.ownerDocument.body;
  await act(async () => { Array.from(body().querySelectorAll('button')).find((button) => button.textContent === '全文')!.click(); });
  const nav = () => body().querySelector('nav[aria-label="查看分块"]');
  await act(async () => { Array.from(nav()!.querySelectorAll('button')).at(-1)!.click(); });
  await act(async () => {});
  assert.ok(body().textContent?.includes('Page one chunk'), 'the loaded page is restored after a failed page fetch');
  assert.ok(body().textContent?.includes('page two unavailable'), 'the transition error is surfaced');
  assert.ok(nav(), 'pagination stays available for a retry');
});

test('audio documents load the embedded player without a completed parse_status like Vue', async () => {
  (URL as unknown as { createObjectURL: () => string }).createObjectURL = () => 'blob:mock-audio';
  (URL as unknown as { revokeObjectURL: () => void }).revokeObjectURL = () => {};
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Interview.mp3', type: 'file', file_type: 'mp3', parse_status: 'pending',
  }));
  (client as any).knowledgeBases.documents.preview = async () => ({ body: 'audio-bytes', headers: {}, contentType: 'audio/mpeg' });
  const container = await mountDetail(client);
  const audio = container.ownerDocument.body.querySelector('audio');
  assert.ok(audio, 'Vue embeds the audio player regardless of parse status');
  assert.equal(audio?.getAttribute('src'), 'blob:mock-audio');
});

test('preview tab content loads without gating on parse_status like the Vue canPreview gate', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'notes.txt', type: 'file', file_type: 'txt', parse_status: 'pending',
  }));
  (client as any).knowledgeBases.documents.preview = async () => ({ body: 'pending-parse body text', headers: {}, contentType: 'text/plain' });
  const container = await mountDetail(client);
  const body = container.ownerDocument.body;
  assert.ok(body.textContent?.includes('pending-parse body text'), 'Vue loads preview content without consulting parse_status');
  assert.equal(body.textContent?.includes('处理完成后才能预览'), false, 'the parse_status unavailable notice is gone with the Vue-aligned gate');
});

test('audio documents follow the Vue default view: merged with an embedded player, no preview tab', async () => {
  (URL as unknown as { createObjectURL: () => string }).createObjectURL = () => 'blob:mock-audio';
  (URL as unknown as { revokeObjectURL: () => void }).revokeObjectURL = () => {};
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Interview.mp3', type: 'file', file_type: 'mp3', parse_status: 'completed',
  }));
  (client as any).knowledgeBases.documents.preview = async () => ({ body: 'audio-bytes', headers: {}, contentType: 'audio/mpeg' });
  const container = await mountDetail(client);
  const body = container.ownerDocument.body;
  assert.equal(Array.from(body.querySelectorAll('button')).some((button) => button.textContent === '预览'), false, 'Vue canPreview() excludes audio so the preview tab is hidden');
  assert.ok(body.querySelector('.wk-document-merged'), 'audio documents open on the merged (全文) view like Vue');
  assert.equal((body.querySelector('audio') as HTMLAudioElement | null)?.getAttribute('src'), 'blob:mock-audio', 'the audio player stays embedded like the Vue audio-player-section');
});

test('document content exposes Vue preview, merged and chunks tabs and merges chunks in index order', async () => {
  const container = await mountDetail(detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'contributor', 'user-1', [
    { id: 'chunk-2', chunk_index: 2, content: '# Second', content_revision: 1, is_enabled: true },
    { id: 'chunk-1', chunk_index: 1, content: '# First\n\n- one\n- two', content_revision: 1, is_enabled: true },
  ]));

  const buttons = () => Array.from(container.ownerDocument.body.querySelectorAll('button'));
  assert.ok(buttons().some((button) => button.textContent === '预览'));
  assert.ok(buttons().some((button) => button.textContent === '全文'));
  assert.ok(buttons().some((button) => button.textContent === '查看分块'));
  const merged = buttons().find((button) => button.textContent === '全文');
  await act(async () => { merged!.click(); });
  assert.ok(container.ownerDocument.body.textContent?.includes('First'));
  assert.ok(container.ownerDocument.body.textContent?.indexOf('First')! < container.ownerDocument.body.textContent?.indexOf('Second')!);
  const mergedSurface = container.ownerDocument.body.querySelector('.wk-document-merged');
  assert.ok(mergedSurface?.querySelector('h1'), 'Vue renders merged Markdown headings instead of exposing raw markers');
  assert.ok(mergedSurface?.querySelector('ul'), 'Vue renders merged Markdown lists instead of plain text');
  assert.match(mergedSurface?.className ?? '', /bg-surface-muted/, 'merged code blocks use the project surface token');
});

test('completed Excel detail opens the Vue-style inline worksheet preview', async () => {
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([
    ['Name', 'Count'],
    ['Alpha', 2],
  ]), 'Summary');
  XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([
    ['Status'],
    ['Ready'],
  ]), 'Details');
  const bytes = XLSX.write(workbook, { bookType: 'xlsx', type: 'array' });
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Report.xlsx', type: 'file', file_type: 'xlsx', parse_status: 'completed',
  }));
  (client as any).knowledgeBases.documents.preview = async () => ({ body: bytes, headers: {}, contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });

  const container = await mountDetail(client);
  const body = container.ownerDocument.body;
  assert.ok(Array.from(body.querySelectorAll('button')).some((button) => button.textContent === '预览'));
  assert.ok(body.querySelector('.wk-preview-spreadsheet table'));
  assert.ok(body.textContent?.includes('Summary'));
  assert.ok(body.textContent?.includes('Alpha'));
  assert.ok(body.textContent?.includes('Details'));
  assert.equal(body.querySelectorAll('.wk-preview-spreadsheet thead th').length, 3);
});

test('completed Mermaid detail loads the source into the Vue-style inline preview', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'architecture.mmd', type: 'file', file_type: 'mmd', parse_status: 'completed',
  }));
  (client as any).knowledgeBases.documents.preview = async () => ({ body: 'graph TD\n  A[Start] --> B[Finish]', headers: {}, contentType: 'text/plain' });

  const container = await mountDetail(client);
  const body = container.ownerDocument.body;
  assert.ok(Array.from(body.querySelectorAll('button')).some((button) => button.textContent === '预览'));
  assert.equal(body.querySelector('.wk-preview-mermaid code')?.textContent, 'graph TD\n  A[Start] --> B[Finish]');
});

test('document preview ignores delayed text from a document that was replaced', async () => {
  const previewCalls: string[] = [];
  let releaseFirstPreview!: () => void;
  const firstPreviewText = new Promise<void>((resolve) => { releaseFirstPreview = resolve; });
  let textReadStarted!: () => void;
  const firstTextReadStarted = new Promise<void>((resolve) => { textReadStarted = resolve; });
  const firstBody = new Blob(['stale document text'], { type: 'text/plain' });
  Object.defineProperty(firstBody, 'text', { value: async () => { textReadStarted(); await firstPreviewText; return 'stale document text'; } });
  const client = {
    knowledgeBases: {
      documents: {
        get: async (id: string) => ({
          id, knowledge_base_id: 'kb-1', file_name: `${id}.txt`, type: 'file', file_type: 'txt', parse_status: 'completed',
        }),
        previewPath: (id: string) => `/api/v1/knowledge/${id}/preview`,
        downloadPath: (id: string) => `/api/v1/knowledge/${id}/download`,
        spans: async () => ({ trace: null }),
        preview: async (id: string) => { previewCalls.push(id); return { body: id === 'doc-1' ? firstBody : 'current document text', headers: {}, contentType: 'text/plain' }; },
        download: async () => ({ body: '', headers: {}, contentType: 'text/plain' }),
        chunks: async () => ({ data: [], total: 0, page: 1, page_size: 25 }),
      },
      settings: { get: async () => ({ id: 'kb-1', user_id: 'user-1' }) },
    },
    auth: { me: async () => ({ user: { id: 'user-1', role: 'contributor' }, memberships: [{ role: 'contributor' }] }) },
  } as never;
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => { mountedRoot?.render(<KnowledgeDocumentDetailPage client={client} documentId="doc-1" onBack={() => {}} />); });
  await act(async () => { await firstTextReadStarted; });
  await act(async () => { mountedRoot?.render(<KnowledgeDocumentDetailPage client={client} documentId="doc-2" onBack={() => {}} />); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
  releaseFirstPreview();
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
  const surfaceText = container.ownerDocument.body.textContent || '';
  assert.ok(surfaceText.includes('current document text'), JSON.stringify({ previewCalls, text: surfaceText }));
  assert.equal(surfaceText.includes('stale document text'), false);
});

test('chunk enabled state can be toggled and failed indexing can be retried', async () => {
  const updates: unknown[] = [];
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.txt', type: 'file', file_type: 'txt', parse_status: 'completed',
  }), 'contributor', 'user-1', [{ id: 'chunk-1', content: 'Disabled chunk', content_revision: 3, is_enabled: false, index_status: 'failed' }]);
  const documents = (client as any).knowledgeBases.documents;
  documents.updateChunk = async (_id: string, _chunkId: string, input: { is_enabled?: boolean }) => {
    updates.push(input);
    return { id: 'chunk-1', content: 'Disabled chunk', content_revision: 4, is_enabled: input.is_enabled ?? false, index_status: input.is_enabled === undefined ? 'failed' : 'completed' };
  };
  const container = await mountDetail(client);
  const enable = Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '启用');
  assert.ok(enable);
  const retry = Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '重试索引');
  assert.ok(retry, 'Vue labels the failed-index action with knowledgeBase.retryIndex');
  await act(async () => { retry!.click(); });
  const refreshedEnable = Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '启用');
  assert.ok(refreshedEnable);
  await act(async () => { refreshedEnable!.click(); });
  assert.deepEqual(updates, [
    { expected_revision: 3 },
    { is_enabled: true, expected_revision: 4 },
  ]);
});

// ─── R466/A1 — merged/chunks bodies render markdown so inline mermaid hydrates ─
// Vue doc-content renders 全文 (merged) and every chunk body through
// processMarkdown (```mermaid fences become diagram blocks) and then runs the
// post-render pipeline that hydrates them. The React bodies must therefore
// render through the markdown pipeline (renderChatMarkdown) instead of plain
// text so the shared mermaid engine has pre[data-markdown-diagram="mermaid"]
// nodes to hydrate.
test('merged and chunks views render markdown bodies with hydratable mermaid blocks (Vue processMarkdown)', async () => {
  const chunkRows = [
    { id: 'chunk-1', content: 'Intro text\n\n```mermaid\ngraph TD; A-->B\n```\n', content_revision: 3, is_enabled: true },
  ];
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', title: 'Manual notes', type: 'manual', parse_status: 'completed',
  }), 'contributor', 'user-1', chunkRows);
  const container = await mountDetail(client);

  const merged = container.ownerDocument.body.querySelector<HTMLElement>('.wk-document-merged');
  assert.ok(merged, 'manual documents open on the merged view');
  assert.ok(merged.querySelector('pre[data-markdown-diagram="mermaid"]'), 'the merged body renders the mermaid diagram block for post-render hydration');
  assert.ok(merged.textContent?.includes('Intro text'), 'surrounding markdown text renders alongside the diagram');

  const chunksTab = Array.from(container.ownerDocument.body.querySelectorAll<HTMLButtonElement>('.view-mode-btn'))
    .find((button) => button.textContent === '查看分块');
  assert.ok(chunksTab, 'the chunks tab is reachable');
  await act(async () => { chunksTab!.click(); });
  const chunkBody = container.ownerDocument.body.querySelector<HTMLElement>('.wk-document-chunks article .wk-document-chunk-content');
  assert.ok(chunkBody, 'each chunk body renders through the markdown pipeline');
  assert.ok(chunkBody.querySelector('pre[data-markdown-diagram="mermaid"]'), 'chunk bodies keep inline mermaid blocks hydratable like the Vue md-content v-html');
});

// ─── R469/A1 — parent chunk context (Vue doc-content git-branch popup, L1886-1908) ──
// Vue contract: a chunk with parent_chunk_id shows a git-branch icon entry
// (title=viewParentContext, not gated by edit permission); opening it closes
// the question/history expansions, lazy-loads GET /chunks/by-id/{parent} once
// per parent id (cache), renders the parent markdown, closes with the
// parentContextLoadFailed notice on failure, and a document switch closes it
// while the cache survives.

function parentEntry(scope: ParentNode): HTMLButtonElement | undefined {
  return Array.from(scope.querySelectorAll('button'))
    .find((button) => button.getAttribute('title') === '查看父块上下文' || button.getAttribute('aria-label') === '查看父块上下文') as HTMLButtonElement | undefined;
}

async function openChunksView(container: HTMLElement) {
  await act(async () => {
    Array.from(container.ownerDocument.body.querySelectorAll('button')).find((button) => button.textContent === '查看分块')!.click();
  });
}

test('the parent-context entry appears only for chunks with parent_chunk_id and stays visible to viewers', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'viewer', 'owner-1', [
    { id: 'chunk-1', content: 'Child A', content_revision: 1, is_enabled: true, parent_chunk_id: 'parent-1' },
    { id: 'chunk-2', content: 'Child B', content_revision: 1, is_enabled: true },
  ]);
  const container = await mountDetail(client);
  await openChunksView(container);
  const body = () => container.ownerDocument.body;

  assert.ok(parentEntry(body()!), 'a chunk with parent_chunk_id keeps the Vue git-branch entry');
  assert.equal(body().querySelectorAll('[title="查看父块上下文"]').length, 1, 'chunks without parent_chunk_id render no entry');
  assert.equal(Array.from(body().querySelectorAll('button')).some((button) => button.textContent === '编辑历史'), false, 'the entry is not gated by edit permission like the Vue icon button');
});

test('parent context lazy-loads once per parent id, renders the parent markdown, and reuses the cache across chunks', async () => {
  const getCalls: string[] = [];
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'contributor', 'user-1', [
    { id: 'chunk-1', content: 'Child A', content_revision: 1, is_enabled: true, parent_chunk_id: 'parent-1' },
    { id: 'chunk-2', content: 'Child B', content_revision: 1, is_enabled: true, parent_chunk_id: 'parent-1' },
  ]);
  let releaseParent!: (value: { id: string; content: string }) => void;
  const parentResponse = new Promise<{ id: string; content: string }>((resolve) => { releaseParent = resolve; });
  (client as any).knowledgeBases.documents.getChunkById = async (id: string) => { getCalls.push(id); return parentResponse; };
  const container = await mountDetail(client);
  await openChunksView(container);
  const body = () => container.ownerDocument.body;

  await act(async () => { parentEntry(body()!)!.click(); });
  const panel = () => body().querySelector<HTMLElement>('.wk-chunk-parent-context');
  assert.ok(panel(), 'opening the entry expands the parent-context panel');
  assert.ok(panel()!.textContent?.includes('加载中'), 'the Vue popup keeps a common.loading state until the parent chunk arrives');

  await act(async () => { releaseParent({ id: 'parent-1', content: '# Parent heading' }); });
  await act(async () => {});
  assert.ok(panel()!.querySelector('h1'), 'the parent body renders through the markdown pipeline (Vue processMarkdown)');
  assert.equal(panel()!.textContent?.includes('加载中'), false);

  // Cache hit: a sibling chunk sharing the same parent renders without a second fetch.
  await act(async () => { parentEntry(body().querySelector('[data-chunk-id="chunk-2"]')!)!.click(); });
  const siblingPanel = body().querySelector<HTMLElement>('[data-chunk-id="chunk-2"] .wk-chunk-parent-context');
  assert.ok(siblingPanel?.querySelector('h1'), 'the shared parent content renders from the cache');
  assert.ok(!body().querySelector('[data-chunk-id="chunk-1"] .wk-chunk-parent-context'), 'only the requesting chunk keeps its panel open');
  assert.deepEqual(getCalls, ['parent-1'], 'the parent chunk is fetched exactly once per parent id');

  // Toggle close like the Vue popup trigger.
  await act(async () => { parentEntry(body().querySelector('[data-chunk-id="chunk-2"]')!)!.click(); });
  assert.equal(body().querySelector('.wk-chunk-parent-context'), null, 'clicking the entry again closes the panel');
  await act(async () => { parentEntry(body()!)!.click(); });
  assert.ok(body().querySelector('.wk-chunk-parent-context')?.querySelector('h1'), 'reopening serves the cached parent without a refetch');
  assert.deepEqual(getCalls, ['parent-1']);
});

test('a failed parent load closes the panel with the Vue parentContextLoadFailed notice', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'contributor', 'user-1', [
    { id: 'chunk-1', content: 'Child A', content_revision: 1, is_enabled: true, parent_chunk_id: 'parent-1' },
  ]);
  (client as any).knowledgeBases.documents.getChunkById = async () => { throw new Error('parent unavailable'); };
  const container = await mountDetail(client);
  await openChunksView(container);
  await act(async () => { parentEntry(container.ownerDocument.body)!.click(); });
  await act(async () => {});

  const body = () => container.ownerDocument.body;
  assert.equal(body().querySelector('.wk-chunk-parent-context'), null, 'Vue closes the popup after a failed parent fetch');
  assert.ok(body().textContent?.includes('加载父上下文失败'), 'the parentContextLoadFailed copy is surfaced');
});

test('parent context is mutually exclusive with the edit and history expansions', async () => {
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'contributor', 'user-1', [
    { id: 'chunk-1', content: 'Child A', content_revision: 1, is_enabled: true, parent_chunk_id: 'parent-1' },
  ]);
  (client as any).knowledgeBases.documents.getChunkById = async (id: string) => ({ id, content: 'Parent body' });
  const container = await mountDetail(client);
  await openChunksView(container);
  const body = () => container.ownerDocument.body;
  const chunkRow = () => body().querySelector('[data-chunk-id="chunk-1"]')!;
  const chunkButton = (label: string) => Array.from(chunkRow().querySelectorAll('button')).find((button) => button.textContent?.includes(label));

  await act(async () => { parentEntry(body()!)!.click(); });
  assert.ok(body().querySelector('.wk-chunk-parent-context'));
  await act(async () => { chunkButton('编辑')!.click(); });
  assert.ok(chunkRow().querySelector('textarea'), 'edit mode opens');
  assert.equal(body().querySelector('.wk-chunk-parent-context'), null, 'opening the editor closes the parent panel like the Vue popup mutex');

  await act(async () => { parentEntry(body()!)!.click(); });
  assert.equal(chunkRow().querySelector('textarea'), null, 'opening the parent panel closes the editor');
  await act(async () => { chunkButton('编辑历史')!.click(); });
  await act(async () => {});
  assert.equal(body().querySelector('.wk-chunk-parent-context'), null, 'opening the chunk history closes the parent panel');
  assert.ok(chunkRow().textContent?.includes('Previous chunk'), 'the history expansion still loads');
});

test('switching documents closes the parent panel but keeps the parent cache', async () => {
  const getCalls: string[] = [];
  const client = detailClient(async (id: string) => ({
    id, knowledge_base_id: 'kb-1', file_name: `${id}.md`, type: 'file', file_type: 'md', parse_status: 'completed',
  }), 'contributor', 'user-1', [
    { id: 'chunk-1', content: 'Child A', content_revision: 1, is_enabled: true, parent_chunk_id: 'parent-1' },
  ]);
  (client as any).knowledgeBases.documents.chunks = async (id: string) => ({
    data: [{ id: `${id}-c1`, content: 'Child row', content_revision: 1, is_enabled: true, parent_chunk_id: 'parent-1' }],
    total: 1, page: 1, page_size: 25,
  });
  (client as any).knowledgeBases.documents.getChunkById = async (id: string) => { getCalls.push(id); return { id, content: '# Cached parent' }; };
  const container = await mountDetail(client);
  await openChunksView(container);
  const body = () => container.ownerDocument.body;
  await act(async () => { parentEntry(body()!)!.click(); });
  await act(async () => {});
  assert.ok(body().querySelector('.wk-chunk-parent-context')?.querySelector('h1'));

  await act(async () => {
    mountedRoot?.render(<KnowledgeDocumentDetailPage client={client} documentId="doc-2" onBack={() => {}} />);
  });
  await act(async () => {});
  await act(async () => {});
  assert.equal(body().querySelector('.wk-chunk-parent-context'), null, 'a document switch closes the panel like the Vue watch(details.id)');

  await openChunksView(container);
  await act(async () => { parentEntry(body()!)!.click(); });
  await act(async () => {});
  assert.ok(body().querySelector('.wk-chunk-parent-context')?.querySelector('h1'), 'the cached parent content renders after the switch');
  assert.deepEqual(getCalls, ['parent-1'], 'the parent cache survives the document switch');
});

// ─── R471/A3 — generated questions (Vue doc-content questions popup, L1911-2010) ──
// Vue contract: a help-circle entry (title=generatedQuestions) renders when
// questions.length > 0 || canEditContent; the panel shows the count, the
// stale hint (a question whose content_revision differs from the chunk
// revision), an add composer + regenerate action for editors, inline row
// edit/save, popconfirm delete gated to !legacy- ids, and the regenerate
// replacement writes generated_questions_revision. Vue canEditContent =
// canEditKB === true || admin (React: canMutateDocument gate).

const QUESTION_ROWS = [
  { id: 'chunk-1', content: 'Chunk body', content_revision: 3, is_enabled: true, metadata: { generated_questions: [
    { id: 'q1', question: 'Current question?', content_revision: 3 },
    { id: 'q2', question: 'Stale question?', content_revision: 1 },
  ] } },
  { id: 'chunk-2', content: 'Empty chunk', content_revision: 1, is_enabled: true },
];

function questionClient(chunkRows: Array<Record<string, unknown>>, role = 'contributor') {
  // The viewer fixture must not own the KB (computeKBPermissions would then
  // grant canContribute like the parent-context viewer test's 'owner-1').
  const knowledgeBaseOwnerId = role === 'viewer' ? 'owner-1' : 'user-1';
  const client = detailClient(async () => ({
    id: 'doc-1', knowledge_base_id: 'kb-1', file_name: 'Guide.md', type: 'file', file_type: 'md', parse_status: 'completed',
  }), role, knowledgeBaseOwnerId, chunkRows);
  const documents = (client as any).knowledgeBases.documents;
  documents.getChunkById = async (id: string) => ({ id, content: 'Parent body' });
  documents.upsertGeneratedQuestion = async () => ({ id: 'q-saved', question: 'Saved', content_revision: 3 });
  documents.deleteGeneratedQuestion = async () => undefined;
  documents.regenerateGeneratedQuestions = async () => [{ id: 'r1', question: 'Regenerated?', content_revision: 3 }];
  return client;
}

function questionEntry(scope: ParentNode): HTMLButtonElement | undefined {
  return Array.from(scope.querySelectorAll('button'))
    .find((button) => button.getAttribute('title') === '辅助召回问题' || button.getAttribute('aria-label') === '辅助召回问题') as HTMLButtonElement | undefined;
}

test('question entry follows the Vue gate and the panel renders count, stale hint, and rows for viewers', async () => {
  const container = await mountDetail(questionClient(QUESTION_ROWS, 'viewer'));
  await openChunksView(container);
  const body = () => container.ownerDocument.body;

  assert.ok(questionEntry(body().querySelector('[data-chunk-id="chunk-1"]')!), 'chunks with questions expose the entry even to viewers');
  assert.equal(questionEntry(body().querySelector('[data-chunk-id="chunk-2"]')!), undefined, 'a questionless chunk hides the entry when the viewer cannot edit');
  assert.equal(Array.from(body().querySelectorAll('button')).some((button) => button.getAttribute('title') === '添加辅助召回问题'), false, 'viewers get no add action like Vue canEditContent');

  await act(async () => { questionEntry(body()!)!.click(); });
  const panel = () => body().querySelector<HTMLElement>('.wk-chunk-questions');
  assert.ok(panel(), 'clicking the entry expands the questions panel');
  const text = panel()!.textContent || '';
  assert.ok(text.includes('Current question?') && text.includes('Stale question?'), 'the metadata questions render');
  assert.ok(text.includes('问题基于编辑前内容生成'), 'a question older than the chunk revision surfaces the Vue stale hint');
  assert.equal(Array.from(panel()!.querySelectorAll('button')).length, 0, 'viewers see no edit/delete/regenerate affordances');

  await act(async () => { questionEntry(body()!)!.click(); });
  assert.equal(body().querySelector('.wk-chunk-questions'), null, 'clicking the entry again closes the panel like the Vue popup trigger');
});

test('legacy string questions render but hide the Vue edit/delete affordances', async () => {
  const legacyRows = [
    { id: 'chunk-1', content: 'Chunk body', content_revision: 3, is_enabled: true, metadata: { generated_questions: ['Legacy string question?'] } },
  ];
  const container = await mountDetail(questionClient(legacyRows));
  await openChunksView(container);
  const body = () => container.ownerDocument.body;
  await act(async () => { questionEntry(body()!)!.click(); });
  const panel = () => body().querySelector<HTMLElement>('.wk-chunk-questions');
  assert.ok(panel()!.textContent?.includes('Legacy string question?'), 'legacy string-array metadata maps onto displayable questions');
  assert.equal(Array.from(panel()!.querySelectorAll('button')).some((button) => button.textContent === '删除'), false, 'legacy-* questions cannot be deleted like Vue !startsWith(legacy-)');
});

// R474/A3: Vue handleDeleteQuestion (doc-content.vue L1443-1447) keeps a
// defensive guard behind the hidden buttons — a legacy- id can never reach
// DELETE /chunks/by-id/:id/questions; it toasts
// knowledgeBase.legacyQuestionCannotDelete instead. The React delete path
// carries the same guard so programmatic callers cannot bypass the row buttons.
test('legacy question ids stay undeletable behind the row-button gate', () => {
  assert.equal(isLegacyGeneratedQuestion({ id: 'legacy-0', question: 'Legacy question?' }), true);
  assert.equal(isLegacyGeneratedQuestion({ id: 'q1', question: 'Saved question?' }), false);
});

test('an editor adds a question through the Vue composer contract', async () => {
  const upserts: Array<unknown[]> = [];
  const client = questionClient(QUESTION_ROWS);
  (client as any).knowledgeBases.documents.upsertGeneratedQuestion = async (chunkId: string, question: string, questionId?: string) => {
    upserts.push([chunkId, question, questionId]);
    return { id: 'q-new', question, content_revision: 3 };
  };
  const container = await mountDetail(client);
  await openChunksView(container);
  const body = () => container.ownerDocument.body;
  await act(async () => { questionEntry(body()!)!.click(); });

  await act(async () => { (Array.from(body().querySelectorAll('button')).find((button) => button.getAttribute('title') === '添加辅助召回问题') as HTMLButtonElement).click(); });
  const input = body().querySelector<HTMLInputElement>('input[placeholder="添加辅助召回问题"]');
  assert.ok(input, 'the composer opens with the addGeneratedQuestion placeholder');
  await act(async () => {
    Object.getOwnPropertyDescriptor(input!.ownerDocument.defaultView!.HTMLInputElement.prototype, 'value')?.set?.call(input!, 'Brand new question?');
    input!.dispatchEvent(new Event('input', { bubbles: true }));
  });
  const submit = Array.from(body().querySelectorAll('button')).find((button) => button.textContent === '添加') as HTMLButtonElement;
  assert.ok(submit!.disabled === false, 'a non-empty draft enables the add action');
  await act(async () => { submit!.click(); });
  await act(async () => {});

  assert.deepEqual(upserts, [['chunk-1', 'Brand new question?', undefined]], 'add sends the question without a question id');
  assert.ok(body().querySelector('.wk-chunk-questions')?.textContent?.includes('Brand new question?'), 'the saved question joins the local list from the response data');
  assert.equal(body().querySelector('input[placeholder="添加辅助召回问题"]'), null, 'the composer closes after a successful add');
  // R472-A1: the toast reads the ported Vue key common.saveSuccess (保存成功),
  // not the R471 fallback common.success.
  assert.ok(body().textContent?.includes('保存成功'), 'the Vue save toast maps onto a success notice');
});

test('an editor saves an inline question edit through the upsert endpoint', async () => {
  const upserts: Array<unknown[]> = [];
  const client = questionClient(QUESTION_ROWS);
  (client as any).knowledgeBases.documents.upsertGeneratedQuestion = async (chunkId: string, question: string, questionId?: string) => {
    upserts.push([chunkId, question, questionId]);
    return { id: questionId!, question, content_revision: 3 };
  };
  const container = await mountDetail(client);
  await openChunksView(container);
  const body = () => container.ownerDocument.body;
  await act(async () => { questionEntry(body()!)!.click(); });

  await act(async () => { (Array.from(body().querySelector('.wk-chunk-questions')!.querySelectorAll('button')).find((button) => button.textContent === '编辑') as HTMLButtonElement).click(); });
  const editor = body().querySelector<HTMLInputElement>('.wk-chunk-questions input');
  assert.ok(editor, 'the inline editor opens for the question row');
  await act(async () => {
    Object.getOwnPropertyDescriptor(editor!.ownerDocument.defaultView!.HTMLInputElement.prototype, 'value')?.set?.call(editor!, 'Edited question?');
    editor!.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => { (Array.from(body().querySelector('.wk-chunk-questions')!.querySelectorAll('button')).find((button) => button.textContent === '保存') as HTMLButtonElement).click(); });
  await act(async () => {});

  assert.deepEqual(upserts, [['chunk-1', 'Edited question?', 'q1']], 'edit sends the existing question id for the upsert');
  const text = body().querySelector('.wk-chunk-questions')!.textContent || '';
  assert.ok(text.includes('Edited question?') && !text.includes('Current question?'), 'the local row is replaced with the saved question');
  assert.equal(body().querySelector('.wk-chunk-questions input'), null, 'the inline editor closes after save');
});

test('delete confirms inline and removes the row through the questions endpoint', async () => {
  const deletes: Array<unknown[]> = [];
  const client = questionClient(QUESTION_ROWS);
  (client as any).knowledgeBases.documents.deleteGeneratedQuestion = async (chunkId: string, questionId: string) => {
    deletes.push([chunkId, questionId]);
  };
  const container = await mountDetail(client);
  await openChunksView(container);
  const body = () => container.ownerDocument.body;
  await act(async () => { questionEntry(body()!)!.click(); });

  await act(async () => { (Array.from(body().querySelector('.wk-chunk-questions')!.querySelectorAll('button')).find((button) => button.textContent === '删除') as HTMLButtonElement).click(); });
  const panel = () => body().querySelector('.wk-chunk-questions')!;
  assert.ok(panel().textContent?.includes('确定要删除这个问题吗'), 'the Vue popconfirm copy surfaces before the delete');
  await act(async () => { (Array.from(panel().querySelectorAll('button')).find((button) => button.textContent === '确认删除') as HTMLButtonElement).click(); });
  await act(async () => {});

  assert.deepEqual(deletes, [['chunk-1', 'q1']], 'delete targets the question id on the by-id questions endpoint');
  // R472-A1: the toast reads the ported Vue key common.deleteSuccess (删除成功),
  // not the R471 fallback common.success.
  assert.ok(body().textContent?.includes('删除成功'), 'the Vue delete toast maps onto a success notice');
  const text = panel().textContent || '';
  assert.ok(!text.includes('Current question?') && text.includes('Stale question?'), 'only the deleted row leaves the local list');
});

test('regenerate replaces the questions and clears the stale hint like Vue', async () => {
  const regenerations: string[] = [];
  const client = questionClient(QUESTION_ROWS);
  (client as any).knowledgeBases.documents.regenerateGeneratedQuestions = async (chunkId: string) => {
    regenerations.push(chunkId);
    return [{ id: 'r1', question: 'Regenerated?', content_revision: 3 }];
  };
  const container = await mountDetail(client);
  await openChunksView(container);
  const body = () => container.ownerDocument.body;
  await act(async () => { questionEntry(body()!)!.click(); });
  assert.ok(body().querySelector('.wk-chunk-questions')?.textContent?.includes('问题基于编辑前内容生成'));

  await act(async () => { (Array.from(body().querySelectorAll('button')).find((button) => button.getAttribute('title') === '重新生成问题') as HTMLButtonElement).click(); });
  await act(async () => {});

  assert.deepEqual(regenerations, ['chunk-1']);
  const text = body().querySelector('.wk-chunk-questions')!.textContent || '';
  assert.ok(text.includes('Regenerated?') && !text.includes('Current question?'), 'regenerate swaps the whole list from the response data');
  assert.ok(!text.includes('问题基于编辑前内容生成'), 'generated_questions_revision resets so the stale hint clears');
  assert.ok(body().textContent?.includes('辅助召回问题已更新'), 'the questionsRegenerated feedback is surfaced');
});

test('the questions panel is mutually exclusive with edit, history, and parent context', async () => {
  const rows = [
    { id: 'chunk-1', content: 'Chunk body', content_revision: 3, is_enabled: true, parent_chunk_id: 'parent-1', metadata: { generated_questions: [{ id: 'q1', question: 'Current question?', content_revision: 3 }] } },
  ];
  const client = questionClient(rows);
  const container = await mountDetail(client);
  await openChunksView(container);
  const body = () => container.ownerDocument.body;
  const chunkRow = () => body().querySelector('[data-chunk-id="chunk-1"]')!;
  const chunkButton = (label: string) => Array.from(chunkRow().querySelectorAll('button')).find((button) => button.textContent?.includes(label));
  const panel = () => body().querySelector('.wk-chunk-questions');

  await act(async () => { questionEntry(body()!)!.click(); });
  assert.ok(panel());
  await act(async () => { chunkButton('编辑')!.click(); });
  assert.ok(chunkRow().querySelector('textarea'), 'edit mode opens');
  assert.equal(panel(), null, 'opening the chunk editor closes the questions panel');

  await act(async () => { questionEntry(body()!)!.click(); });
  await act(async () => { chunkButton('编辑历史')!.click(); });
  await act(async () => {});
  assert.equal(panel(), null, 'opening the chunk history closes the questions panel like the Vue popup mutex');

  await act(async () => { questionEntry(body()!)!.click(); });
  await act(async () => { parentEntry(body()!)!.click(); });
  assert.equal(panel(), null, 'opening the parent-context panel closes the questions panel');
  assert.ok(body().querySelector('.wk-chunk-parent-context'), 'the parent panel opens in its place');
});
