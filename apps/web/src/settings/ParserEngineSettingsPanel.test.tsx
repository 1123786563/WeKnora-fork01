import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import React from 'react';
import { act } from 'react';

const hooks = createRequire(import.meta.url)('node:module') as typeof import('node:module') & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
Object.assign(globalThis, { React, IS_REACT_ACT_ENVIRONMENT: true });
const { ParserEngineSettingsPanel } = await import('./ParserEngineSettingsPanel.tsx');

const ENGINES = [
  { Name: 'builtin', Available: true, Description: 'docreader' },
  { Name: 'simple', Available: true, Description: 'simple formats' },
  { Name: 'anydoc', Available: false, UnavailableReason: 'anydoc binary missing', Description: 'office docs' },
  { Name: 'mineru', Available: true, Description: 'mineru self-hosted', FileTypes: ['pdf'] },
];

function makeClient() {
  return {
    settings: {
      parser: {
        engines: async () => ({ items: ENGINES, docreader_addr: '10.0.0.8:18000', docreader_transport: 'http', connected: true }),
        check: async () => ({ items: ENGINES, connected: true }),
        config: {
          get: async () => ({ mineru_model: 'pipeline', mineru_parse_method: 'auto', mineru_language: 'ch' }),
          update: async (body: Record<string, unknown>) => body,
        },
      },
      weknoraCloud: { status: async () => ({ has_models: false, needs_reinit: false }) },
    },
  } as never as Parameters<typeof ParserEngineSettingsPanel>[0]['client'];
}

async function mountPanel(): Promise<{ container: HTMLElement; cleanup: () => Promise<void> }> {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings?section=parser' });
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  Object.assign(globalThis, { window: dom.window, document: dom.window.document });
  const { createRoot } = await import('react-dom/client');
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(React.createElement(ParserEngineSettingsPanel, { client: makeClient() }));
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  await act(async () => { await new Promise((resolve) => setImmediate(resolve)); });
  return {
    container,
    cleanup: async () => {
      await act(async () => { root.unmount(); });
      Object.assign(globalThis, { window: previousWindow, document: previousDocument });
      dom.window.close();
    },
  };
}

test('renders the Vue engine-card grid with localized names and availability', async () => {
  const { container, cleanup } = await mountPanel();
  try {
    const text = container.textContent ?? '';
    for (const name of ['builtin', 'simple', 'anydoc', 'mineru']) {
      assert.ok(container.querySelector(`[data-testid="parser-engine-card-${name}"]`), `${name} card renders`);
    }
    assert.match(text, /可用/);
    assert.match(text, /不可用/);
  } finally {
    await cleanup();
  }
});

test('mineru drawer exposes the Vue config controls (backend, parse method, language, vLLM)', async () => {
  const { container, cleanup } = await mountPanel();
  try {
    (container.querySelector('[data-testid="parser-engine-card-mineru"]') as HTMLButtonElement).click();
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    for (const testid of ['mineru-model', 'mineru-vllm-server-url', 'mineru-parse-method', 'mineru-language']) {
      assert.ok(container.querySelector(`[data-testid="${testid}"]`), `${testid} control renders in the drawer`);
    }
    assert.match(container.textContent ?? '', /公式识别/);
    assert.match(container.textContent ?? '', /表格识别/);
  } finally {
    await cleanup();
  }
});
