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
const { KnowledgeSettingsPage, loadKnowledgeSettingsOptions } = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

const knowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  type: 'document',
  chunking_config: { parser_engine_rules: [{ file_types: ['pdf'], engine: 'mineru' }] },
  vector_store_id: 'vs-1',
  vector_store_name: 'Vectors',
  vector_store_engine_type: 'pgvector',
  vector_store_source: 'tenant',
  storage_backend_id: 'st-1',
  storage_provider_config: { provider: 's3' },
};

interface ClientCalls { dataSources: string[]; shareList: number; activityCalls: Array<{ knowledgeBaseId: string; query: Record<string, unknown> }>; }

function clientFor(calls: ClientCalls): WeKnoraClient {
  return {
    request: async () => ({ data: [] }),
    configuration: {
      models: { list: async () => [{ id: 'llm-1', name: 'gpt-x', type: 'KnowledgeQA', source: 'remote' }] },
    },
    dataSources: {
      list: async (kbId: string) => {
        calls.dataSources.push(kbId);
        return [{ id: 'ds-1', name: 'Feishu docs', type: 'feishu', status: 'completed' }];
      },
      types: async () => [],
    },
    knowledgeBases: {
      settings: {
        parserEngines: async () => ({ data: [{ Name: 'mineru', Description: 'MinerU self-hosted', Available: true }, { Name: 'builtin', Description: 'Built-in', Available: false }] }),
        storageBackends: async () => ({ data: [{ id: 'st-1', name: 'Main storage', provider: 's3', status: 'ready' }] }),
        vectorStores: async () => ({ data: [{ id: 'vs-1', name: 'Vectors', engine_type: 'pgvector', source: 'tenant', readonly: false }] }),
        activity: async (kbId: string, query?: Record<string, unknown>) => {
          calls.activityCalls.push({ knowledgeBaseId: kbId, query: query ?? {} });
          return { data: [{ id: 'a-1', action: 'kb.updated', outcome: 'success', created_at: '2026-09-15T00:00:00Z' }] };
        },
      },
    },
    identity: {
      organizations: {
        list: async () => {
          calls.shareList += 1;
          return { items: [], total: 0 };
        },
        knowledgeBaseShares: { list: async () => ({ items: [], total: 0 }) },
      },
    },
  } as unknown as WeKnoraClient;
}

async function loadKnowledgeSettingsOptionsRejectsIndependently(): Promise<void> {
  const failing = {
    knowledgeBases: {
      settings: {
        parserEngines: async () => { throw new Error('parser down'); },
        storageBackends: async () => ({ data: [{ id: 'st-1', name: 'Main storage', provider: 's3', status: 'ready' }] }),
        vectorStores: async () => { throw new Error('vectors down'); },
      },
    },
  } as unknown as WeKnoraClient;
  const options = await loadKnowledgeSettingsOptions(failing);
  assert.deepEqual(options.parserEngines, []);
  assert.deepEqual(options.storageBackends.map((backend) => backend.id), ['st-1']);
  assert.deepEqual(options.vectorStores, []);
  assert.match(options.error ?? '', /parser down/);
}

async function loadKnowledgeSettingsOptionsToleratesMissingMethods(): Promise<void> {
  const sparse = {
    knowledgeBases: {
      settings: {
        parserEngines: async () => ({ data: [{ Name: 'mineru', Description: 'MinerU self-hosted', Available: true }] }),
      },
    },
  } as unknown as WeKnoraClient;
  const options = await loadKnowledgeSettingsOptions(sparse);
  assert.deepEqual(options.parserEngines.map((engine) => engine.Name), ['mineru']);
  assert.deepEqual(options.storageBackends, []);
  assert.deepEqual(options.vectorStores, []);
  assert.ok(options.error, 'expected the missing catalogue endpoints to surface a load error instead of throwing');
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient): Promise<HTMLElement> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase, role: 'admin' }));
  });
  await act(async () => { await Promise.resolve(); });
  return container;
}

function clickSection(section: string): void {
  // R438 grouped nav: nav items carry data-section (Vue currentSection keys).
  const button = document.body.querySelector(`button[data-section="${section}"]`);
  assert.ok(button, `expected a ${section} section button; got: ${JSON.stringify([...document.body.querySelectorAll('button[data-section]')].map((candidate) => candidate.textContent))}`);
  button!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
}

test('options loader normalizes the live parser/vector/storage endpoints and degrades independently', async () => {
  const calls: ClientCalls = { dataSources: [], shareList: 0, activityCalls: [] };
  const options = await loadKnowledgeSettingsOptions(clientFor(calls));
  assert.deepEqual(options.parserEngines.map((engine) => engine.Name), ['mineru', 'builtin']);
  assert.deepEqual(options.storageBackends.map((backend) => backend.id), ['st-1']);
  assert.deepEqual(options.vectorStores.map((store) => store.id), ['vs-1']);
  assert.equal(options.error, null);
  await loadKnowledgeSettingsOptionsRejectsIndependently();
  await loadKnowledgeSettingsOptionsToleratesMissingMethods();
});

test('sections mount the Vue-equivalent inline surfaces: datasource page, share dialog, activity panel, and live parser select', async () => {
  const calls: ClientCalls = { dataSources: [], shareList: 0, activityCalls: [] };
  await renderPage(clientFor(calls));
  await act(async () => { clickSection('datasource'); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.deepEqual(calls.dataSources, ['kb-1'], 'expected DataSourcesPage to load through client.dataSources.list');
  assert.match(document.body.textContent ?? '', /Feishu docs/);
  await act(async () => { clickSection('share'); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.ok(calls.shareList >= 1, 'expected the inline share dialog to load organizations');
  await act(async () => { clickSection('activity'); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  assert.equal(calls.activityCalls.length, 1);
  assert.equal(calls.activityCalls[0]!.knowledgeBaseId, 'kb-1');
  await act(async () => { clickSection('parser'); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const select = document.body.querySelector('select');
  assert.ok(select, 'expected a parser engine select');
  assert.equal(select.disabled, false);
  const values = [...select.querySelectorAll('option')].map((option) => option.value);
  assert.equal(values.includes('mineru'), true);
  assert.equal(values.includes('builtin'), false, 'unavailable engines must be filtered out');
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
});

afterEach(() => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    return act(async () => { root.unmount(); }).then(() => {
      document.body.innerHTML = '';
    });
  }
  document.body.innerHTML = '';
  return undefined;
});
