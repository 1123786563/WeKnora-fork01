// T17 (#136): the stop-outcome notice panel. The three durable stop outcomes
// of a Run (the frozen T00 CraftStopOutcome) must each announce themselves
// with their own sentence: the ACCEPTED stop is never announced as stopped
// (the executor may still be running), a confirmed stop is 已停止, and an
// unobservable abort outcome is 停止结果不明 — never a false confirmation.
// The notice speaks (text), never color alone (the CraftNotice contract).
import assert from 'node:assert/strict';
import test from 'node:test';
import { JSDOM } from 'jsdom';
import React, { act } from 'react';

const dom = new JSDOM('<!doctype html><html><body></body></html>');
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & {
  registerHooks?: (h: { resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown }) => void;
};
if (hooks.registerHooks) {
  hooks.registerHooks({
    resolve: (specifier, context, nextResolve) => specifier.endsWith('.css')
      ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
      : nextResolve(specifier, context),
  });
}
const { createRoot } = await import('../../../../apps/web/node_modules/react-dom/client.js');
const { CraftStopNotice } = await import('./status-notice.tsx');

async function renderText(node: React.ReactNode): Promise<string> {
  document.body.replaceChildren();
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(node);
  });
  const text = container.textContent ?? '';
  await act(async () => {
    root.unmount();
  });
  return text;
}

test('requested stop announces waiting for the executor, never stopped', async () => {
  const text = await renderText(<CraftStopNotice outcome="requested" />);
  assert.ok(text.includes('已请求停止'), `requested must announce the recorded request, got ${text}`);
  assert.ok(text.includes('等待执行器确认'), `requested must say the executor confirmation is pending, got ${text}`);
  assert.ok(!text.includes('已停止'), `an accepted stop must never read as stopped, got ${text}`);
});

test('confirmed stop announces stopped', async () => {
  const text = await renderText(<CraftStopNotice outcome="confirmed" />);
  assert.ok(text.includes('已停止'), `confirmed must announce the confirmed stop, got ${text}`);
  assert.ok(!text.includes('等待执行器确认'), `a confirmed stop is not pending, got ${text}`);
});

test('unknown abort outcome announces reconciliation, never a confirmation', async () => {
  const text = await renderText(<CraftStopNotice outcome="unknown" />);
  assert.ok(text.includes('停止结果不明'), `unknown must announce the unclear outcome, got ${text}`);
  assert.ok(!text.includes('已停止'), `unknown must never read as a confirmed stop, got ${text}`);
  assert.ok(!text.includes('等待执行器确认'), `unknown is not merely pending — it is a distinct durable outcome, got ${text}`);
});
