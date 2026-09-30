import assert from 'node:assert/strict';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';
import * as nodeModule from 'node:module';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings?section=memory' });
Object.assign(globalThis, {
  React, window: dom.window, document: dom.window.document,
  HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement, HTMLSelectElement: dom.window.HTMLSelectElement,
  Element: dom.window.Element, Node: dom.window.Node, SVGElement: dom.window.SVGElement,
  MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');
const { MemoryWorkspacePanel } = await import('./PersonalMemoryPanel.tsx');

let root: Root | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

test('memory workspace loads and saves extract_min_interval_seconds zero', async () => {
  const calls: Array<{ body: Record<string, unknown>; resolve: (value: Record<string, unknown>) => void }> = [];
  const client = {
    configuration: { models: { list: async () => [] } },
    settings: { memory: { workspace: { update: (body: Record<string, unknown>) => new Promise<Record<string, unknown>>((resolve) => calls.push({ body, resolve })) } } },
  } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(
    <MemoryWorkspacePanel client={client} initialConfig={{ enabled: true, write_mode: 'auto', extract_min_interval_seconds: 60 }} />,
  ));
  await act(async () => root?.render(
    <MemoryWorkspacePanel client={client} initialConfig={{ enabled: true, write_mode: 'auto', extract_min_interval_seconds: 0 }} />,
  ));

  const intervalRow = [...container.querySelectorAll<HTMLElement>('.setting-row')].find((row) => row.textContent?.includes('最小间隔'));
  assert.ok(intervalRow, 'automatic memory extraction exposes the minimum interval control');
  const intervalInput = intervalRow.querySelector<HTMLInputElement>('input');
  assert.ok(intervalInput, 'the interval control renders an input');
  assert.equal(intervalInput.value, '0', 'zero from the loaded configuration remains visible');

  const changeInterval = async (value: string) => act(async () => {
    Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set?.call(intervalInput, value);
    intervalInput.dispatchEvent(new window.Event('input', { bubbles: true }));
    intervalInput.dispatchEvent(new window.Event('change', { bubbles: true }));
  });
  await changeInterval('60');
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 550)); });
  assert.equal(calls.length, 1, 'the first interval generation starts saving');
  assert.equal(calls[0]?.body.extract_min_interval_seconds, 60);

  await changeInterval('0');
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 550)); });
  assert.equal(calls.length, 1, 'the zero edit waits for the active full-config update');
  await act(async () => { calls[0]!.resolve(calls[0]!.body); await Promise.resolve(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  assert.equal(intervalInput.value, '0', 'the older update response does not replace the newer zero draft');
  assert.equal(calls.length, 2, 'the latest interval generation is saved after the first request');
  assert.equal(calls[1]?.body.extract_min_interval_seconds, 0, 'zero is included in the saved full configuration');
  await act(async () => { calls[1]!.resolve(calls[1]!.body); await Promise.resolve(); });
});
