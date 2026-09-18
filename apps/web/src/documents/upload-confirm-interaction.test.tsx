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
// file before handing the source to the transport (browser-only API).
const urlCtor = dom.window.URL as typeof URL & { createObjectURL?: unknown; revokeObjectURL?: unknown };
urlCtor.createObjectURL ??= (() => 'blob:mock-upload-source') as never;
urlCtor.revokeObjectURL ??= (() => undefined) as never;

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
  const zone = document.querySelector('.wk-dropzone');
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
