// R485 H3 (G1 展示形态剩余批): the React knowledge-settings drawer must
// render the Vue edit-mode visible form for the four deferred sections —
// KBVectorStoreSettings' read-only bound-store badge card (not a disabled
// select), KBStorageSettings' instanceDesc row + manage-instances entry +
// default/endpoint hints, KBShareSettings' shared-to list (count badge,
// search, add popup, permission select, unshare confirm — not the ShareDialog
// form), and KBAdvancedSettings' auto-tag + table-metadata instruction rows
// (with the ragEnabled gate hiding the question-generation block).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
dom.window.localStorage.setItem('locale', 'en-US');
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLButtonElement: dom.window.HTMLButtonElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  PointerEvent: dom.window.PointerEvent,
  NodeFilter: dom.window.NodeFilter,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { KnowledgeSettingsPage } = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

// Vue editor edit-mode row (loadKBData) for a document KB.
const knowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
  chunking_config: {
    chunk_size: 700,
    chunk_overlap: 90,
    separators: ['\n\n', '\n'],
    enable_parent_child: true,
    parent_chunk_size: 4096,
    child_chunk_size: 384,
    strategy: 'auto',
    token_limit: 0,
    languages: ['zh'],
    table_metadata_instructions: 'keep headers',
  },
  question_generation_config: { enabled: true, question_count: 5, custom_instructions: 'gen rules' },
  vlm_config: { enabled: false, model_id: '', description_language: '', custom_instructions: '' },
  asr_config: { enabled: false, model_id: '', language: '' },
  auto_tag_config: { enabled: true, model_id: 'llm-1', max_tags: 5, skip_if_tagged: false },
};

const liveModels = [
  { id: 'llm-1', name: 'gpt-x', display_name: 'GPT X', type: 'KnowledgeQA', source: 'remote' },
  { id: 'embed-1', name: 'bge-m3', display_name: 'BGE M3', type: 'Embedding', source: 'local' },
];

const storageBackends = [
  { id: 'st-1', name: 'Main storage', provider: 's3', status: 'active', config: { endpoint: 's3.example.com' } },
  { id: 'st-2', name: 'Backup storage', provider: 'oss', status: 'active', config: { bucket_name: 'docs-bucket' } },
];

interface UiCalls { requests: Array<{ method: string; path: string; body: Record<string, unknown> }> }

interface ClientOverrides {
  shares?: Array<Record<string, unknown>>;
  organizations?: Array<Record<string, unknown>>;
}

function clientFor(calls: UiCalls, overrides: ClientOverrides = {}): WeKnoraClient {
  const organizations = overrides.organizations ?? [
    { id: 'org-1', name: 'Design space', is_owner: true },
    { id: 'org-2', name: 'Research space', is_owner: false, my_role: 'editor' },
  ];
  const shares = overrides.shares ?? [
    { id: 'share-1', organization_id: 'org-1', organization_name: 'Design space', permission: 'viewer', shared_by_username: 'alice', created_at: '2026-09-01T10:00:00Z' },
  ];
  const request = async (input: { method: string; path: string; body: Record<string, unknown> }) => {
    calls.requests.push({ method: input.method, path: input.path, body: input.body });
    return { success: true };
  };
  return {
    request,
    configuration: {
      models: { list: async () => liveModels },
    },
    identity: {
      organizations: {
        list: async () => ({ items: organizations, total: organizations.length }),
        knowledgeBaseShares: {
          list: async () => ({ items: shares, total: shares.length }),
          create: async (_kbId: string, body: Record<string, unknown>) => { calls.requests.push({ method: 'POST', path: '/api/v1/knowledge-bases/kb-1/shares', body }); return { id: 'share-new' }; },
          remove: async (_kbId: string, shareId: string) => { calls.requests.push({ method: 'DELETE', path: `/api/v1/knowledge-bases/kb-1/shares/${shareId}`, body: {} }); return { success: true }; },
          updatePermission: async () => ({ success: true }),
        },
      },
    },
    knowledgeBases: {
      documents: {
        list: async () => ({ data: [], total: 0 }),
      },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: storageBackends, default_storage_backend_id: 'st-1' }),
        vectorStores: async () => ({ data: [] }),
        activity: async () => ({ data: [], next_cursor: undefined }),
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient, kb: KnowledgeSettingsInput = knowledgeBase): Promise<void> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase: kb, role: 'admin' }));
  });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

async function openSection(section: string): Promise<void> {
  const button = document.body.querySelector(`button[data-section="${section}"]`);
  assert.ok(button, `expected a ${section} section button`);
  await act(async () => { button!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

function bodyText(): string { return document.body.textContent ?? ''; }

function buttonByText(text: string): HTMLButtonElement {
  const button = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === text || candidate.getAttribute('aria-label') === text);
  assert.ok(button, `expected a button labelled "${text}"; got: ${JSON.stringify([...document.body.querySelectorAll('button')].map((candidate) => candidate.textContent ?? candidate.getAttribute('aria-label')))}`);
  return button as HTMLButtonElement;
}

function setNativeValue(element: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  const prototype = element instanceof dom.window.HTMLTextAreaElement ? dom.window.HTMLTextAreaElement.prototype : dom.window.HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(prototype, 'value')!.set!;
  setter.call(element, value);
  element.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  element.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

function setSelectValue(select: HTMLSelectElement, value: string): void {
  [...select.options].forEach((option) => { option.selected = option.value === value; });
  select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

function checkboxByLabel(label: string): HTMLInputElement {
  const box = [...document.body.querySelectorAll('input[type="checkbox"]')].find((candidate) => candidate.getAttribute('aria-label') === label);
  assert.ok(box, `expected a checkbox with aria-label "${label}"; got: ${JSON.stringify([...document.body.querySelectorAll('input[type="checkbox"]')].map((candidate) => candidate.getAttribute('aria-label')))}`);
  return box as HTMLInputElement;
}

function numberInputByLabel(label: string): HTMLInputElement {
  const input = [...document.body.querySelectorAll('input[type="number"]')].find((candidate) => candidate.getAttribute('aria-label') === label);
  assert.ok(input, `expected a number input with aria-label "${label}"`);
  return input as HTMLInputElement;
}

function textareaByLabel(label: string): HTMLTextAreaElement {
  const area = [...document.body.querySelectorAll('textarea')].find((candidate) => candidate.getAttribute('aria-label') === label);
  assert.ok(area, `expected a textarea with aria-label "${label}"`);
  return area as HTMLTextAreaElement;
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

// ---- vectorStore: Vue KBVectorStoreSettings edit mode renders a read-only
// bound-store badge (VectorStoreBadge), not a disabled select. ----

test('vectorStore tab renders the read-only bound-store badge card for a user-bound store', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), { ...knowledgeBase, vector_store_source: 'user', vector_store_name: 'Team store', vector_store_engine_type: 'pgvector', vector_store_status: 'available' });
  await openSection('vectorStore');
  const text = bodyText();
  assert.ok(text.includes('Bound vector store'), 'the boundLabel row renders');
  assert.ok(text.includes('Vector store binding cannot be changed after creation.'), 'the immutableEdit description renders');
  assert.ok(text.includes('Team store'), 'the badge shows the bound store name');
  assert.ok(text.includes('(pgvector)'), 'the badge shows the engine type in parentheses');
  assert.equal(document.body.querySelectorAll('select').length, 0, 'the old disabled select must be gone');
  assert.equal(text.includes('cannot be changed after creation. To migrate'), false, 'the create-mode immutableHint must not leak into edit mode');
});

test('vectorStore tab renders the system-default badge with the env engine type', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), { ...knowledgeBase, vector_store_engine_type: 'postgres' });
  await openSection('vectorStore');
  assert.ok(bodyText().includes('System default'), 'an unbound KB shows the system-default badge');
  assert.ok(bodyText().includes('(postgres)'), 'the env engine type renders in parentheses');
});

test('vectorStore tab renders the unavailable tag and warning hint for an unhealthy binding', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), { ...knowledgeBase, vector_store_source: 'user', vector_store_name: 'Old store', vector_store_engine_type: 'pgvector', vector_store_status: 'unavailable' });
  await openSection('vectorStore');
  const text = bodyText();
  assert.ok(text.includes('Unavailable'), 'the badge carries the unavailable tag');
  assert.ok(text.includes('The bound vector store is currently unavailable'), 'the unavailableHint warning renders');
});

// ---- storage: Vue KBStorageSettings instanceDesc row, default tag, endpoint
// hint and the manage-instances entry. ----

test('storage tab renders instanceDesc, provider/default option tags, the endpoint hint and the manage-instances link', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), { ...knowledgeBase, storage_backend_id: 'st-1' });
  await openSection('storage');
  const text = bodyText();
  assert.ok(text.includes('The same storage type can have multiple instances'), 'the instanceDesc row renders');
  assert.ok(text.includes('s3.example.com'), 'the selected backend endpoint hint renders (Vue selected.config.endpoint)');
  const link = document.body.querySelector('a[data-storage-manage-instances]');
  assert.ok(link, 'the manage-instances entry renders');
  assert.equal(link!.getAttribute('href'), '/settings?section=storage', 'the entry opens the global storage settings section');
  const select = document.body.querySelector('select[aria-label="Storage instance"]') as HTMLSelectElement | null;
  assert.ok(select, 'the storage instance select keeps rendering');
  const st1 = [...select!.options].find((option) => option.value === 'st-1');
  const st2 = [...select!.options].find((option) => option.value === 'st-2');
  assert.ok(st1, 'option st-1 exists');
  assert.ok(st2, 'option st-2 exists');
  assert.ok(st1!.textContent?.includes('S3'), 'option st-1 shows the uppercased provider tag');
  assert.ok(st1!.textContent?.includes('Default'), 'the default backend carries the default tag');
  assert.ok(st2!.textContent?.includes('OSS'), 'option st-2 shows the uppercased provider tag');
  assert.equal(st1!.textContent?.includes('Default'), true, 'only the default carries the tag');
  assert.equal(st2!.textContent?.includes('Default'), false, 'the non-default backend has no default tag');
});

test('storage tab falls back to the bucket name hint when the backend has no endpoint', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), { ...knowledgeBase, storage_backend_id: 'st-2' });
  await openSection('storage');
  assert.ok(bodyText().includes('docs-bucket'), 'Vue selected.config.bucket_name is the second hint fallback');
});

// ---- share: Vue KBShareSettings list form (count badge, search, add popup,
// permission select, unshare) replaces the ShareDialog form. ----

test('share tab renders the shared-to list with count badge, search, add entry and table columns', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('share');
  const text = bodyText();
  assert.ok(text.includes('Shared to'), 'the sharedTo header renders');
  assert.ok(text.includes('Design space'), 'the shared organization row renders');
  assert.ok(text.includes('From alice'), 'the sharedFrom meta renders the sharing user');
  assert.ok(text.includes('Shared Space') && text.includes('Permission') && text.includes('Shared') && text.includes('Actions'), 'the Vue table columns render');
  const badge = document.body.querySelector('[data-share-count]');
  assert.ok(badge, 'the share count badge renders');
  assert.equal(badge!.textContent, '1', 'the badge counts the loaded shares');
  const search = document.body.querySelector('input[placeholder="Search shared spaces…"]');
  assert.ok(search, 'the search box renders with the Vue placeholder');
  assert.ok(buttonByText('Share'), 'the add-share entry renders with the addShare label');
  assert.equal([...document.body.querySelectorAll('button')].some((candidate) => (candidate.textContent ?? '').trim() === 'Confirm'), false, 'the ShareDialog confirm form must be gone');
  const permissionSelect = document.body.querySelector('select[data-share-permission]') as HTMLSelectElement | null;
  assert.ok(permissionSelect, 'the per-row permission select renders for a manager');
  assert.equal(permissionSelect!.value, 'viewer', 'the select binds the share permission');
});

test('share tab renders the empty state when nothing is shared yet', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { shares: [] }));
  await openSection('share');
  assert.ok(bodyText().includes('Not shared to any shared space yet'), 'the Vue noShares empty copy renders');
  const badge = document.body.querySelector('[data-share-count]');
  assert.ok(badge, 'the empty state still renders the shared-to list header');
  assert.equal(badge!.textContent, '0', 'the badge counts zero shares');
  assert.ok(buttonByText('Share'), 'the add-share entry stays available in the empty state');
});

test('share tab add popup carries the Vue addShareDialogTitle form and creates a share', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { shares: [] }));
  await openSection('share');
  await act(async () => { buttonByText('Share').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const text = bodyText();
  assert.ok(text.includes('Share to shared space'), 'the addShareDialogTitle popup title renders');
  assert.ok(text.includes('Select Shared Space'), 'the selectOrg field renders');
  assert.ok(text.includes('one extra model call') === false, 'unrelated advanced copy does not leak');
  const orgSelect = document.body.querySelector('select[data-share-add-org]') as HTMLSelectElement | null;
  assert.ok(orgSelect, 'the org select renders inside the popup');
  assert.ok([...orgSelect!.options].some((option) => option.value === 'org-2'), 'only un-shared organizations are selectable');
  await act(async () => { setSelectValue(orgSelect!, 'org-2'); });
  const confirmButton = document.body.querySelector('button[data-share-add-confirm]') as HTMLButtonElement | null;
  assert.ok(confirmButton, 'the popup confirm button renders');
  await act(async () => { confirmButton!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const create = calls.requests.find((request) => request.method === 'POST' && request.path === '/api/v1/knowledge-bases/kb-1/shares');
  assert.ok(create, 'the popup confirm issues the share create call');
  assert.deepEqual(create!.body, { organization_id: 'org-2', permission: 'viewer' }, 'the create body carries the selected org and permission');
});

test('share tab unshare confirm removes the share after confirmation', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('share');
  const remove = buttonByText('Remove share');
  await act(async () => { remove.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); });
  const text = bodyText();
  assert.ok(text.includes('Are you sure you want to unshare from "Design space"?'), 'the Vue unshareConfirm copy renders before removal');
  const confirmButton = document.body.querySelector('button[data-share-unshare-confirm]') as HTMLButtonElement | null;
  assert.ok(confirmButton, 'the inline unshare confirm button renders');
  await act(async () => { confirmButton!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const removeCall = calls.requests.find((request) => request.method === 'DELETE' && request.path === '/api/v1/knowledge-bases/kb-1/shares/share-1');
  assert.ok(removeCall, 'confirming issues the unshare delete call');
});

// ---- advanced: Vue KBAdvancedSettings auto-tag + table-metadata rows. ----

test('advanced tab renders the auto-tag rows and the table-metadata instructions textarea', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('advanced');
  const text = bodyText();
  assert.ok(text.includes('AI Question Generation'), 'the question-generation block still renders while RAG is enabled');
  assert.ok(text.includes('Automatic Tagging'), 'the autoTag label renders');
  assert.ok(text.includes('one extra model call'), 'the autoTag description renders');
  assert.equal(checkboxByLabel('Automatic Tagging').checked, true, 'the autoTag toggle binds auto_tag_config.enabled');
  const modelSelect = document.body.querySelector('select[data-autotag-model]') as HTMLSelectElement | null;
  assert.ok(modelSelect, 'the classification-model select renders');
  assert.equal(modelSelect!.value, 'llm-1', 'the select binds auto_tag_config.model_id');
  assert.ok([...modelSelect!.options].some((option) => option.value === ''), 'the model select is clearable (empty option)');
  assert.equal(numberInputByLabel('Maximum tags per document').value, '5', 'the max-tags input binds auto_tag_config.max_tags');
  assert.equal(checkboxByLabel('Skip documents that already have tags').checked, false, 'the skip toggle binds auto_tag_config.skip_if_tagged');
  assert.ok(text.includes('Table Metadata Instructions'), 'the tableMetadataInstructions label renders');
  assert.equal(textareaByLabel('Table Metadata Instructions').value, 'keep headers', 'the textarea binds chunking.table_metadata_instructions');
});

test('advanced tab hides the question-generation block when RAG indexing is off but keeps auto-tag', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls), { ...knowledgeBase, indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true, graph_enabled: false } });
  await openSection('advanced');
  const text = bodyText();
  assert.equal(text.includes('AI Question Generation'), false, 'Vue v-if="ragEnabled !== false" hides the question-generation block');
  assert.equal(text.includes('Question Count'), false, 'the question-generation subsection rows hide too');
  assert.ok(text.includes('Automatic Tagging'), 'the auto-tag rows stay visible');
  assert.ok(text.includes('Table Metadata Instructions'), 'the table-metadata row stays visible');
});

test('advanced tab edits persist auto_tag_config through the base update and table instructions through the config PUT', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('advanced');
  await act(async () => {
    const skip = checkboxByLabel('Skip documents that already have tags');
    skip.click();
  });
  await act(async () => {
    const maxTags = numberInputByLabel('Maximum tags per document');
    setNativeValue(maxTags, '7');
  });
  await act(async () => {
    const area = textareaByLabel('Table Metadata Instructions');
    setNativeValue(area, 'sales orders, amounts in CNY');
  });
  await act(async () => { buttonByText('Save and Close').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); });
  const baseUpdate = calls.requests.find((request) => request.method === 'PUT' && request.path === '/api/v1/knowledge-bases/kb-1');
  assert.ok(baseUpdate, 'the base update PUT fires');
  const autoTagBody = (baseUpdate!.body.config as Record<string, Record<string, unknown>>).auto_tag_config;
  assert.deepEqual(autoTagBody, { enabled: true, model_id: 'llm-1', max_tags: 7, skip_if_tagged: true }, 'the autoTag draft reaches auto_tag_config');
  const configPut = calls.requests.find((request) => request.method === 'PUT' && request.path === '/api/v1/initialization/config/kb-1');
  assert.ok(configPut, 'the config PUT fires');
  const splitting = (configPut!.body.documentSplitting ?? {}) as Record<string, unknown>;
  assert.equal(splitting.tableMetadataInstructions, 'sales orders, amounts in CNY', 'the table-metadata draft reaches documentSplitting');
});
