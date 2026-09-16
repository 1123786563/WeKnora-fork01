import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import React from 'react';
import { act } from 'react';

import { appDigest, appRisk, appRows, appErrorMessage, installationState } from './model.ts';

const hooks = createRequire(import.meta.url)('node:module') as typeof import('node:module') & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
Object.assign(globalThis, { React, IS_REACT_ACT_ENVIRONMENT: true });
const { AppsPage } = await import('./AppsPages.tsx');

test('normalizes standard app list envelopes without dropping nested rows', () => {
  assert.deepEqual(appRows({ data: { items: [{ id: 'a-1' }] } }), [{ id: 'a-1' }]);
  assert.deepEqual(appRows({ data: { rows: [{ id: 'a-2' }] } }), [{ id: 'a-2' }]);
  assert.deepEqual(appRows({ items: [{ id: 'a-3' }] }), [{ id: 'a-3' }]);
  assert.deepEqual(appRows([{ id: 'a-4' }]), [{ id: 'a-4' }]);
});

test('preserves server errors and gives non-error failures a stable message', () => {
  assert.equal(appErrorMessage(new Error('provider unavailable')), 'provider unavailable');
  assert.equal(appErrorMessage('failure'), '应用页面加载失败');
});

test('matches the Vue AppsView semantic tags and digest preview', () => {
  assert.deepEqual(appRisk('read'), { label: '只读', tone: 'success' });
  assert.deepEqual(appRisk('write'), { label: '写入', tone: 'warning' });
  assert.deepEqual(appRisk('delete'), { label: '删除', tone: 'danger' });
  assert.deepEqual(appRisk('future'), { label: 'future', tone: 'neutral' });
  assert.equal(appDigest('1234567890123456'), '123456789012…');
  assert.deepEqual(installationState('active'), { label: '活跃', tone: 'success' });
  assert.deepEqual(installationState('disabled'), { label: '已停用', tone: 'neutral' });
  assert.deepEqual(installationState('paused'), { label: '状态：paused', tone: 'neutral' });
});

test('renders unknown action risk as an em dash even when the DTO contains delete', async () => {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/apps/actions/action-1' });
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  Object.assign(globalThis, { window: dom.window, document: dom.window.document });
  const { createRoot } = await import('react-dom/client');
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  const client = { request: async () => ({ action: { risk: 'delete', state: 'completed', content: '{}' } }) } as never;

  try {
    await act(async () => {
      root.render(React.createElement(AppsPage, { client, mode: 'action', id: 'action-1' }));
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    await act(async () => { await new Promise((resolve) => setImmediate(resolve)); });
    const riskLabel = Array.from(container.querySelectorAll('dt')).find((element) => element.textContent === '风险');
    assert.ok(riskLabel, 'renders the risk field');
    assert.equal(riskLabel.nextElementSibling?.textContent, '—');
  } finally {
    await act(async () => { root.unmount(); });
    Object.assign(globalThis, { window: previousWindow, document: previousDocument });
    dom.window.close();
  }
});
