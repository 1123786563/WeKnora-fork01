import '../test-tdom-harness.ts'; // jsdom 全局（tdesign Popup 运行时）
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge/1/documents' });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
// jsdom does not implement blob object URLs; the upload pipeline mints one per
// file before handing the source to the transport (browser-only API). The page
// module resolves the bare `URL` to the Node realm global, so stub there (the
// jsdom File must be accepted).
const globalUrl = globalThis.URL as typeof URL & Record<string, unknown>;
globalUrl.createObjectObjectURLBackup ??= globalUrl.createObjectURL;
globalUrl.createObjectURL = (() => 'blob:mock-upload-source') as typeof globalUrl.createObjectURL;
globalUrl.revokeObjectURL ??= (() => undefined) as never;

const { createRoot } = await import('react-dom/client');
const { KnowledgeDocumentsPage } = await import('./KnowledgeDocumentsPage.tsx');

interface UploadCall { knowledgeBaseId: string; input: { fileName?: string; file?: { name?: string }; process_config?: unknown } }

function pageClient(kb: Record<string, unknown> = {}, uploadCalls: UploadCall[] = []): WeKnoraClient {
  const knowledgeBase = {
    id: 'kb-1',
    name: 'parity-kb',
    description: '',
    type: 'knowledge',
    created_at: '2030-01-01T00:00:00Z',
    storage_backend_id: 'sb-1',
    ...kb,
  };
  const documents = {
    list: async () => ({ data: [], total: 0, page: 1, page_size: 20 }),
    tagsPage: async () => ({ data: [], total: 0, page: 1, page_size: 20 }),
    tags: async () => [],
    folders: async () => ({ folders: [], root_document_count: 0, total_document_count: 0 }),
    upload: async (knowledgeBaseId: string, input: UploadCall['input']) => {
      uploadCalls.push({ knowledgeBaseId, input });
      return { id: `doc-${uploadCalls.length}`, file_name: input.fileName, title: input.fileName, source: 'file', created_at: '2030-01-01T00:00:00Z' };
    },
    createFromUrl: async () => ({ id: 'doc-url' }),
    createManual: async () => ({ id: 'doc-manual' }),
    moveToFolder: async () => ({}),
  };
  return {
    knowledgeBases: {
      settings: {
        get: async () => knowledgeBase,
        parserEngines: async () => ({ data: [] }),
      },
      list: async () => [knowledgeBase],
      documents,
    },
    knowledge: { documents },
    auth: { me: async () => ({ user: { id: 'u1', role: 'admin' }, memberships: [{ tenant_id: 1, role: 'admin' }] }) },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => null } } },
    configuration: { models: { list: async () => [] } },
    settings: { system: { info: async () => ({}) } },
  } as unknown as WeKnoraClient;
}

async function renderPage(client: WeKnoraClient): Promise<Root> {
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host);
  await act(async () => {
    root.render(React.createElement(KnowledgeDocumentsPage, { client, knowledgeBaseId: 'kb-1' }));
  });
  // Let the metadata/document/tag/folder/model requests settle before staging.
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  return root;
}

function dropFiles(files: File[]): void {
  // tdesign 平移后拖放面 = .knowledge-main（Vue 全局拖放语义）。
  const zone = document.querySelector('.knowledge-main');
  assert.ok(zone, 'documents dropzone is mounted');
  const event = new dom.window.Event('drop', { bubbles: true, cancelable: true });
  Object.defineProperty(event, 'dataTransfer', { value: { files } });
  act(() => { zone.dispatchEvent(event); });
}

function confirmButton(): HTMLButtonElement {
  const dialog = document.querySelector('[role="dialog"]');
  assert.ok(dialog, 'upload confirm dialog is open');
  const buttons = [...dialog.querySelectorAll('button')];
  const confirm = buttons.find((button) => button.textContent?.trim() === '确认上传并解析');
  assert.ok(confirm, 'confirm button renders with the Vue copy');
  return confirm as HTMLButtonElement;
}

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

// R474 A4 field observation (P2): clicking 确认上传并解析 in the React upload
// confirm dialog emitted no POST and no console error. The jsdom contract
// below pins the full UI chain — staging a file, opening the dialog, clicking
// confirm — to the client upload call, so a silent early-out fails here.
test('clicking 确认上传并解析 posts the staged file through the upload client', async () => {
  const uploadCalls: UploadCall[] = [];
  const client = pageClient({}, uploadCalls);
  mountedRoot = await renderPage(client);

  const file = new dom.window.File(['# r475 fixture\ncontent'], 'r475-a1.md', { type: 'text/markdown' });
  dropFiles([file]);

  const confirm = confirmButton();
  assert.equal(confirm.disabled, false, 'confirm is enabled with a staged file');
  await act(async () => { confirm.click(); });
  // Sequential pipeline runs to completion inside act's async window.
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });

  assert.equal(uploadCalls.length, 1, 'one upload POST is issued for the staged file');
  assert.equal(uploadCalls[0]?.knowledgeBaseId, 'kb-1');
  assert.equal(uploadCalls[0]?.input.fileName, 'r475-a1.md');
});

// R474 A4 P2 root cause reproduction: the live fixture KB stores
// chunking_config { chunk_size: 0, chunk_overlap: 0 } ("not customized").
// Vue seeds 0 || 512 = 512 and uploads; React used to keep 0 and silently
// reject the confirm click (< 100 guard) with no POST. The seeded defaults
// must carry the confirm through exactly like Vue initFromKbInfo.
test('a KB with zeroed chunking_config still uploads on confirm (R474 A4 regression)', async () => {
  const uploadCalls: UploadCall[] = [];
  const client = pageClient({ chunking_config: { chunk_size: 0, chunk_overlap: 0, separators: null } }, uploadCalls);
  mountedRoot = await renderPage(client);

  const file = new dom.window.File(['# r475 zero chunk fixture'], 'r475-zero-chunk.md', { type: 'text/markdown' });
  dropFiles([file]);

  const confirm = confirmButton();
  assert.equal(confirm.disabled, false, 'confirm stays enabled for a zero-config KB');
  await act(async () => { confirm.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });

  assert.equal(uploadCalls.length, 1, 'confirm posts the upload instead of silently rejecting chunk_size 0');
  const processConfig = uploadCalls[0]?.input.process_config as { chunking_config?: { chunk_size?: number } } | undefined;
  assert.equal(processConfig?.chunking_config?.chunk_size, 512, 'the Vue default 512 is submitted');
});

// R476 A3 contract lock: the Vue upload confirm dialog submits its AI question
// generation section through the multipart process_config JSON field
// (frontend/src/api/knowledge-base/index.ts uploadKnowledgeFile stringifies
// data.process_config; handler knowledge.go reads PostForm("process_config")).
// Whether the backend then auto-generates questions is gated server-side on
// kb.NeedsEmbeddingModel() — but the React payload must keep carrying the
// question_generation_config the dialog promised, byte-compatible with Vue
// buildProcessOverrides, or auto-generation can never trigger.
test('confirm carries the dialog question_generation_config into the upload process_config (R476 A3)', async () => {
  const uploadCalls: UploadCall[] = [];
  const client = pageClient({}, uploadCalls);
  mountedRoot = await renderPage(client);

  const file = new dom.window.File(['# r476 a3 question probe'], 'r476-a3-questions.md', { type: 'text/markdown' });
  dropFiles([file]);

  const confirm = confirmButton();
  await act(async () => { confirm.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });

  assert.equal(uploadCalls.length, 1, 'the staged file is uploaded once');
  const processConfig = uploadCalls[0]?.input.process_config as {
    question_generation_config?: { enabled?: boolean; question_count?: number; custom_instructions?: string };
  } | undefined;
  // Dialog defaults (Vue createDefaultUIState): question generation enabled
  // with 3 questions. The upload payload must mirror that selection.
  assert.deepEqual(
    processConfig?.question_generation_config,
    { enabled: true, question_count: 3, custom_instructions: '' },
    'process_config.question_generation_config mirrors the Vue buildProcessOverrides payload',
  );
});

// R476 A3 contract lock (seeded variant): a KB that stored its own
// question_generation_config seeds the dialog from it (Vue initFromKbInfo),
// and the confirmed upload round-trips the stored count instead of the default.
test('a KB question_generation_config seeds the confirmed upload payload (R476 A3)', async () => {
  const uploadCalls: UploadCall[] = [];
  const client = pageClient({ question_generation_config: { enabled: true, question_count: 5 } }, uploadCalls);
  mountedRoot = await renderPage(client);

  const file = new dom.window.File(['# r476 a3 seeded probe'], 'r476-a3-seeded.md', { type: 'text/markdown' });
  dropFiles([file]);

  const confirm = confirmButton();
  await act(async () => { confirm.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });

  assert.equal(uploadCalls.length, 1, 'the staged file is uploaded once');
  const processConfig = uploadCalls[0]?.input.process_config as {
    question_generation_config?: { enabled?: boolean; question_count?: number; custom_instructions?: string };
  } | undefined;
  assert.equal(processConfig?.question_generation_config?.enabled, true, 'stored enabled flag round-trips');
  assert.equal(processConfig?.question_generation_config?.question_count, 5, 'stored question_count 5 round-trips');
});
