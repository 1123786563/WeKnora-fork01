import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

const hooks = nodeModule as unknown as { registerHooks?: (h: unknown) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier: string, _c: unknown, next: (s: string, c: unknown) => unknown) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : next(specifier, _c) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom');
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { SandboxSettingsPanel, sandboxT } = await import('./SandboxSettingsPanel.tsx');

const t = sandboxT('zh-CN');
const cubeRecord = {
  id: 'sandbox-2', name: 'Cube cluster', description: '', sandbox_type: 'cube',
  config: { sandbox_type: 'cube', cube: { api_url: 'http://x', proxy_url: 'http://y', sandbox_domain: 'd', template_id: 't' } },
  created_at: '2030-01-01T00:00:00Z', updated_at: '2030-01-01T00:00:00Z',
} as never;

function deferred<T>() { let resolve!: (v: T) => void; const promise = new Promise<T>((n) => { resolve = n; }); return { promise, resolve }; }

test('probe: click with pending promise', async () => {
  const pending = deferred<{ id: string }>();
  const client = {
    request: async () => ({}),
    sessions: { get: () => pending.promise },
    sandboxConfigurations: {
      list: async () => ({ items: [cubeRecord], workspaceScriptsDisabled: false }),
      inventory: async () => ({ sandboxCount: 1, sessionIds: ['session-a'], agentNames: [] }),
    },
  };
  const container = document.createElement('div');
  document.body.append(container);
  let root: Root | undefined = createRoot(container);
  console.log('step-1 mount');
  await act(async () => { root?.render(React.createElement(SandboxSettingsPanel, { client, role: 'admin', initialData: { items: [cubeRecord], workspaceScriptsDisabled: false } as never })); });
  console.log('step-2 mounted, looking for button');
  const viewButton = Array.from(container.querySelectorAll('button')).find((b) => b.textContent === t('settings.sandbox.viewSandboxes'));
  assert.ok(viewButton, 'button exists');
  console.log('step-3 clicking');
  await act(async () => { viewButton!.click(); });
  console.log('step-4 clicked; inventory in DOM =', Boolean(container.querySelector('.wk-sandbox-inventory')));
  await act(async () => pending.resolve({ id: 'session-a' }));
  console.log('step-5 resolved; inventory in DOM =', Boolean(container.querySelector('.wk-sandbox-inventory')));
  await act(async () => root?.unmount());
  console.log('step-6 done');
});
