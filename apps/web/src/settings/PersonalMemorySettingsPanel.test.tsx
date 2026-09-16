import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings?section=mymemory' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { PersonalMemorySettingsPanel, documentOpenUrl } = await import('./PersonalMemorySettingsPanel.tsx');
const { formatMessage } = await import('@weknora/i18n');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.clear();
});

interface MemoryCall { method: string; path: string; body?: unknown }

function memoryItem(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return { id: 'memory-1', kind: 'fact', content: 'User deploys with Docker', topic: 'deploy', importance: 3, origin: 'explicit', status: 'active', source_session_id: 's1', source_message_id: 'm1', valid_from: '2026-09-14T10:00:00Z', invalid_at: null, superseded_by: '', last_used_at: null, use_count: 0, created_at: '2026-09-14T10:00:00Z', updated_at: '2026-09-14T10:00:00Z', ...overrides };
}

function memoryDoc(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return { id: 'doc-1', knowledge_id: 'kb-doc-1', knowledge_base_id: 'kb-1', title: 'Runbook', hits: 3, last_used_at: '2026-09-14T10:00:00Z', ...overrides };
}

function memoryTopic(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return { id: 'topic-1', topic: 'kubernetes', aliases: ['k8s'], hits: 2, threshold: 3, last_seen_at: '2026-09-14T10:00:00Z', ...overrides };
}

// Vue MemorySettings.vue data contract: settings envelope is the merged
// MemorySettings struct (internal/types/memory.go), lists answer {data,total}.
function makeClient(overrides: {
  settings?: Record<string, unknown>;
  activeItems?: Record<string, unknown>[];
  activeTotal?: number;
  pendingItems?: Record<string, unknown>[];
  topics?: Record<string, unknown>[];
  documents?: Record<string, unknown>[];
  activePageGate?: Promise<{ rows: Record<string, unknown>[]; total: number }>;
  updateEnabled?: (enabled: boolean) => Promise<Record<string, unknown>>;
  onCall?: (call: MemoryCall) => void;
} = {}) {
  const calls: MemoryCall[] = [];
  const record = (method: string, path: string, body?: unknown) => {
    const call = { method, path, body };
    calls.push(call);
    overrides.onCall?.(call);
  };
  const page = (rows: Record<string, unknown>[] | undefined, total: number) => ({ rows: rows ?? [], total });
  const client = {
    settings: {
      memory: {
        personal: {
          settings: async () => { record('GET', '/memory/settings'); return overrides.settings ?? { user_enabled: true, workspace_enabled: true, effective: true, write_mode: 'auto', item_count: 1, max_items: 200 }; },
          updateEnabled: async (enabled: boolean) => { record('PUT', '/memory/settings', { enabled }); return (overrides.updateEnabled ?? (async (value: boolean) => ({ user_enabled: value, workspace_enabled: true, effective: value })))(enabled); },
          clear: async () => { record('DELETE', '/memory/items'); return { removed: 2 }; },
          export: async () => { record('GET', '/memory/export'); return [memoryItem()]; },
          consolidate: async () => { record('POST', '/memory/consolidate', {}); return { merged: 1, demoted: 0, expired: 0, reviewed: 4, candidates: 2 }; },
          items: {
            list: async (params: { status?: string; limit?: number; offset?: number }) => {
              record('GET', '/memory/items?' + (params.status ? 'status=' + params.status + '&' : '') + 'limit=' + (params.limit ?? 0) + '&offset=' + (params.offset ?? 0));
              if (params.status === 'active' && params.limit === 20 && overrides.activePageGate) return overrides.activePageGate;
              if (params.status === 'pending') return page(overrides.pendingItems, overrides.pendingItems?.length ?? 0);
              if (params.status === 'active' || params.status === undefined) return page(overrides.activeItems, overrides.activeTotal ?? overrides.activeItems?.length ?? 0);
              return page(undefined, 0);
            },
            create: async (input: unknown) => { record('POST', '/memory/items', input); return memoryItem({ status: 'active' }); },
            update: async (_id: string, input: unknown) => { record('PUT', '/memory/items/memory-1', input); return memoryItem(); },
            remove: async () => { record('DELETE', '/memory/items/memory-1'); return { success: true }; },
            confirm: async () => { record('POST', '/memory/items/memory-1/confirm', {}); return memoryItem({ status: 'active' }); },
            reject: async () => { record('POST', '/memory/items/memory-1/reject', {}); return { success: true }; },
          },
          topics: {
            list: async (params: { limit?: number }) => { record('GET', '/memory/topics?limit=' + (params.limit ?? 0) + '&offset=0'); return page(overrides.topics, overrides.topics?.length ?? 0); },
            promote: async () => { record('POST', '/memory/topics/topic-1/promote', {}); return memoryItem({ kind: 'interest' }); },
            remove: async () => { record('DELETE', '/memory/topics/topic-1'); return { success: true }; },
          },
          documents: {
            list: async (params: { limit?: number }) => { record('GET', '/memory/documents?limit=' + (params.limit ?? 0) + '&offset=0'); return page(overrides.documents, overrides.documents?.length ?? 0); },
            remove: async () => { record('DELETE', '/memory/documents/doc-1'); return { success: true }; },
          },
        },
      },
    },
  };
  return { client: client as never, calls };
}

async function mountPanel(client: never) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<PersonalMemorySettingsPanel client={client} initialSettings={null} />);
  });
  // Flush the async mount chain (settings + counts + list).
  await act(async () => {});
  return container;
}

test('shows the Vue loading indicator while the selected memory page is pending', async () => {
  let release!: () => void;
  const activePageGate = new Promise<{ rows: Record<string, unknown>[]; total: number }>((resolve) => {
    release = () => resolve({ rows: [], total: 0 });
  });
  const { client } = makeClient({ activePageGate });
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);

  await act(async () => {
    mountedRoot?.render(<PersonalMemorySettingsPanel client={client} initialSettings={null} />);
  });
  await new Promise((resolve) => setTimeout(resolve, 0));

  assert.equal(container.querySelector('[role="status"]')?.getAttribute('aria-label'), formatMessage('zh-CN', 'common.loading'), 'Vue t-loading equivalent is visible during list load');
  release();
  await act(async () => {});
});

// Vue parity anchor: MemorySettings.vue onMounted → loadSettings + reload, and
// loadCounts requests limit:1 per status so every tab carries its own count.
test('mount loads settings, per-status counts and the active page with pagination params', async () => {
  const { client, calls } = makeClient({ activeItems: [memoryItem()], pendingItems: [memoryItem({ id: 'p1', status: 'pending' })] });
  const container = await mountPanel(client);

  const tabs = Array.from(container.querySelectorAll('[role="tab"]'));
  assert.equal(tabs.length, 6);
  assert.equal(tabs[0]!.getAttribute('aria-selected'), 'true');
  assert.equal(tabs[1]!.textContent!.includes(formatMessage('zh-CN', 'memorySettings.statusPending') + '(1)'), true, 'pending tab shows its count');

  // Vue runs loadList and loadCounts concurrently (Promise.all), so only the
  // call set is contractual, not the order.
  const paths = calls.filter((call) => call.path.startsWith('/memory/items')).map((call) => call.path).sort();
  assert.deepEqual(paths, [
    '/memory/items?status=active&limit=1&offset=0',
    '/memory/items?status=active&limit=20&offset=0',
    '/memory/items?status=archived&limit=1&offset=0',
    '/memory/items?status=pending&limit=1&offset=0',
    '/memory/items?status=superseded&limit=1&offset=0',
  ]);
  assert.equal(container.querySelector('li p')!.textContent, 'User deploys with Docker');
  // Vue toolbar count = totalAll (sum of the four status counts).
  assert.equal(container.querySelector('h3 + span')!.textContent, formatMessage('zh-CN', 'memorySettings.listCount', { count: 2 }));
  const switchButton = container.querySelector('[role="switch"]') as HTMLButtonElement;
  assert.equal(switchButton.getAttribute('aria-checked'), 'true');
  assert.equal(switchButton.disabled, false);
});
test('tab switch refetches with the next status and pending rows confirm/reject like Vue', async () => {
  const { client, calls } = makeClient({ pendingItems: [memoryItem({ id: 'p1', status: 'pending', content: 'Guess: likes Rust' })] });
  const container = await mountPanel(client);

  await act(async () => {
    (container.querySelectorAll('[role="tab"]')[1] as HTMLButtonElement).click();
  });
  await act(async () => {});

  assert.ok(calls.some((call) => call.path === '/memory/items?status=pending&limit=20&offset=0'), 'pending page fetched with offset 0');
  const pendingContent = Array.from(container.querySelectorAll('li p')).find((node) => node.textContent === 'Guess: likes Rust');
  assert.ok(pendingContent, 'pending row rendered');

  const confirmButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.confirmGuess')));
  assert.ok(confirmButton);
  await act(async () => { confirmButton!.click(); });
  await act(async () => {});

  assert.ok(calls.some((call) => call.method === 'POST' && call.path === '/memory/items/memory-1/confirm'), 'confirm endpoint hit');
  assert.equal(container.querySelector('[role="status"]')?.textContent, formatMessage('zh-CN', 'memorySettings.confirmSuccess'));

  const rejectButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.rejectGuess')));
  assert.ok(rejectButton);
  await act(async () => { rejectButton!.click(); });
  await act(async () => {});
  assert.ok(calls.some((call) => call.method === 'POST' && call.path === '/memory/items/memory-1/reject'), 'reject endpoint hit');
  // Vue: reject is not canWrite-gated (declining an inference must stay possible).
  assert.equal(rejectButton!.disabled, false);
});

test('tracking tab renders progress and promote jumps back to active', async () => {
  const { client, calls } = makeClient({ topics: [memoryTopic()] });
  const container = await mountPanel(client);

  await act(async () => {
    (container.querySelectorAll('[role="tab"]')[2] as HTMLButtonElement).click();
  });
  await act(async () => {});

  const progress = container.querySelector('[role="progressbar"]');
  assert.ok(progress, 'topic progress bar rendered');
  assert.equal(progress!.getAttribute('aria-valuenow'), '67');
  assert.equal(container.querySelector('[role="progressbar"] + span')!.textContent, formatMessage('zh-CN', 'memorySettings.trackingProgress', { hits: 2, threshold: 3 }));

  const promote = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.promoteTopic')));
  assert.ok(promote);
  await act(async () => { promote!.click(); });
  await act(async () => {});
  assert.ok(calls.some((call) => call.method === 'POST' && call.path === '/memory/topics/topic-1/promote'), 'promote endpoint hit');
  const activeTab = container.querySelectorAll('[role="tab"]')[0] as HTMLButtonElement;
  assert.equal(activeTab.getAttribute('aria-selected'), 'true', 'Vue promoteTopic switches tab to active');
  assert.ok(calls.some((call) => call.path === '/memory/items?status=active&limit=20&offset=0'), 'active list refetched');
});

test('documents tab builds the Vue knowledgeBase deep link and stop-tracking confirms', async () => {
  const { client, calls } = makeClient({ documents: [memoryDoc()] });
  const container = await mountPanel(client);

  await act(async () => {
    (container.querySelectorAll('[role="tab"]')[3] as HTMLButtonElement).click();
  });
  await act(async () => {});

  // Vue handleOpenDocument navigation target, asserted on the pure helper
  // (JSDOM cannot intercept location.assign without leaking across tests).
  const doc = memoryDoc();
  assert.equal(documentOpenUrl(doc), '/knowledgeBase/kb-1?knowledge_id=kb-doc-1');
  assert.equal(documentOpenUrl({ knowledge_base_id: 'kb 1', knowledge_id: 'd/1' }), '/knowledgeBase/kb%201?knowledge_id=d%2F1');
  assert.equal(documentOpenUrl({ knowledge_base_id: '' }), null, 'no kb id → helper refuses');
  const openButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.openDocument'))) as HTMLButtonElement;
  assert.ok(openButton);
  assert.equal(openButton.disabled, false, 'open enabled with knowledge_base_id present');

  const stopButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.stopTrackingDocument')));
  assert.ok(stopButton);
  await act(async () => { stopButton!.click(); });
  await act(async () => {});
  const confirm = Array.from(container.querySelectorAll('[role="alertdialog"] button')).find((button) => button.textContent === formatMessage('zh-CN', 'memorySettings.stopTrackingDocument')) as HTMLButtonElement;
  assert.ok(confirm, 'popconfirm shows the danger confirm');
  await act(async () => { confirm!.click(); });
  await act(async () => {});
  assert.ok(calls.some((call) => call.method === 'DELETE' && call.path === '/memory/documents/doc-1'), 'stop-tracking endpoint hit');
  assert.equal(container.querySelector('[role="status"]')?.textContent, formatMessage('zh-CN', 'memorySettings.stopTrackingDocumentSuccess'));
});

test('toolbar clear is guarded by total emptiness and answers with the removed count', async () => {
  const { client } = makeClient({ activeItems: [memoryItem()] });
  const container = await mountPanel(client);
  const clearButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.clear'))) as HTMLButtonElement;
  assert.equal(clearButton.disabled, false, 'enabled while items exist');

  await act(async () => { clearButton.click(); });
  await act(async () => {});
  const confirm = Array.from(container.querySelectorAll('[role="alertdialog"] button')).find((button) => button.textContent === formatMessage('zh-CN', 'memorySettings.clear')) as HTMLButtonElement;
  assert.ok(confirm, 'danger popconfirm opens with clearConfirm copy');
  await act(async () => { confirm!.click(); });
  await act(async () => {});
  assert.equal(container.querySelector('[role="status"]')?.textContent, formatMessage('zh-CN', 'memorySettings.toasts.cleared', { count: 2 }));
});

test('clear is disabled when every store is empty (Vue disabled condition)', async () => {
  const { client } = makeClient({});
  const container = await mountPanel(client);
  const clearButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.clear'))) as HTMLButtonElement;
  assert.equal(clearButton.disabled, true);
  const consolidate = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.consolidate'))) as HTMLButtonElement;
  assert.equal(consolidate.disabled, true, 'consolidate disabled when totalAll is 0');
});

test('pagination pages with offset like Vue t-pagination', async () => {
  const rows = Array.from({ length: 20 }, (_, index) => memoryItem({ id: 'memory-' + index }));
  const { client, calls } = makeClient({ activeItems: rows, activeTotal: 45 });
  const container = await mountPanel(client);

  const nav = container.querySelector('[role="navigation"]');
  assert.ok(nav, 'pagination rendered above 20 rows');
  const pageTwo = Array.from(nav!.querySelectorAll('button')).find((button) => button.textContent === '2');
  assert.ok(pageTwo);
  await act(async () => { pageTwo!.click(); });
  await act(async () => {});
  assert.ok(calls.some((call) => call.path === '/memory/items?status=active&limit=20&offset=20'), 'page 2 fetches offset 20');
});

test('export downloads the JSON blob like Vue handleExport', async () => {
  const { client } = makeClient({});
  const container = await mountPanel(client);
  const clicks: string[] = [];
  const originalCreate = globalThis.URL.createObjectURL;
  const originalRevoke = globalThis.URL.revokeObjectURL;
  const originalClick = dom.window.HTMLAnchorElement.prototype.click;
  (globalThis.URL as unknown as { createObjectURL: () => string }).createObjectURL = () => 'blob:mock';
  (globalThis.URL as unknown as { revokeObjectURL: (url: string) => void }).revokeObjectURL = () => undefined;
  dom.window.HTMLAnchorElement.prototype.click = function click() { clicks.push((this as HTMLAnchorElement).download); };
  try {
    const exportButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.export'))) as HTMLButtonElement;
    await act(async () => { exportButton!.click(); });
    await act(async () => {});
    assert.deepEqual(clicks, ['weknora-memories.json']);
  } finally {
    (globalThis.URL as unknown as { createObjectURL: (obj: Blob | MediaSource) => string }).createObjectURL = originalCreate;
    (globalThis.URL as unknown as { revokeObjectURL: (url: string) => void }).revokeObjectURL = originalRevoke;
    dom.window.HTMLAnchorElement.prototype.click = originalClick;
  }
});

test('workspace-disabled notice gates the switch but keeps the list readable', async () => {
  const { client } = makeClient({
    settings: { user_enabled: true, workspace_enabled: false, effective: false, write_mode: 'auto', item_count: 1, max_items: 200 },
    activeItems: [memoryItem()],
  });
  const container = await mountPanel(client);

  const notice = container.querySelector('[role="status"]');
  assert.ok(notice, 'workspace-disabled notice rendered');
  assert.equal(notice!.textContent!.includes(formatMessage('zh-CN', 'memorySettings.workspaceDisabled')), true);
  const switchButton = container.querySelector('[role="switch"]') as HTMLButtonElement;
  assert.equal(switchButton.disabled, true, 'switch disabled while workspace is off');
  assert.ok(container.querySelector('li p'), 'list stays readable (Vue keeps the list visible)');
  const add = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.add'))) as HTMLButtonElement;
  assert.equal(add.disabled, true, 'write actions gated by effective=false');
});

test('failed enable change reverts the switch and surfaces saveFailed (Vue handleEnabledChange)', async () => {
  const { client } = makeClient({
    updateEnabled: async () => { throw new Error('network down'); },
  });
  const container = await mountPanel(client);
  const switchButton = container.querySelector('[role="switch"]') as HTMLButtonElement;

  await act(async () => { switchButton.click(); });
  await act(async () => {});

  const after = container.querySelector('[role="switch"]') as HTMLButtonElement;
  assert.equal(after.getAttribute('aria-checked'), 'true', 'optimistic change reverted on failure');
  assert.equal(container.querySelector('[role="alert"]')?.textContent, formatMessage('zh-CN', 'memorySettings.toasts.saveFailed', { message: 'network down' }));
});
test('consolidate reports the skip reason instead of a bare success', async () => {
  const base = makeClient({ activeItems: [memoryItem()] });
  const personal = (base.client as unknown as { settings: { memory: { personal: { consolidate: () => Promise<Record<string, unknown>> } } } }).settings.memory.personal;
  personal.consolidate = async () => ({ merged: 0, demoted: 0, expired: 0, reviewed: 1, candidates: 0, skipped: 'too_few_items' });
  const container = await mountPanel(base.client);
  const consolidateButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent!.includes(formatMessage('zh-CN', 'memorySettings.consolidate'))) as HTMLButtonElement;
  assert.equal(consolidateButton.disabled, false, 'consolidate enabled while items exist');
  await act(async () => { consolidateButton.click(); });
  await act(async () => {});
  const confirm = Array.from(container.querySelectorAll('[role="alertdialog"] button')).find((button) => button.textContent === formatMessage('zh-CN', 'memorySettings.consolidate')) as HTMLButtonElement;
  assert.ok(confirm, 'consolidate popconfirm opens');
  await act(async () => { confirm!.click(); });
  await act(async () => {});
  assert.equal(container.querySelector('[role="status"]')?.textContent, formatMessage('zh-CN', 'memorySettings.consolidateTooFewItems'));
});
