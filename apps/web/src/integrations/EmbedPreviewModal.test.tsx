import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// Interactive render harness (same pattern as embedWizardRender.test.tsx).
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/integrations' });
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
// Direct source import keeps this render test independent of any barrel.
const { EmbedPreviewModal } = await import('./EmbedPreviewModal.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

// Vue parity: AgentEmbedChannelPanel.vue deploy-step 预览 button (L317-320) ->
// openPreviewForChannel (L993-1035) mints a short-lived preview session
// (the previewSession port wired on IntegrationsRoutePage) and opens
// EmbedChannelPreview.vue (720px drawer modal, L2-18) whose iframe loads
// buildEmbedURL(channelId, sessionToken) with allow="clipboard-write".
test('deploy-step preview click opens the modal iframe over the previewSession URL', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);

  // Harness mirroring the route-page wiring: clicking 预览 mints the preview
  // session token, then opens the modal over the resulting URL.
  function PreviewHarness() {
    const [state, setState] = React.useState({ open: false, channelId: 'ch-1', token: '' });
    const openPreview = async () => {
      const token = await Promise.resolve('tok_preview');
      setState({ open: true, channelId: 'ch-1', token });
    };
    return React.createElement(React.Fragment, null,
      React.createElement('button', { type: 'button', onClick: () => { void openPreview(); } }, '预览'),
      React.createElement(EmbedPreviewModal, {
        open: state.open,
        channelId: state.channelId,
        token: state.token,
        title: '客服渠道',
        apiBaseUrl: 'https://weknora.test',
        locale: 'zh-CN',
        onClose: () => setState((current) => ({ ...current, open: false })),
      }),
    );
  }

  await act(async () => { mountedRoot?.render(React.createElement(PreviewHarness)); });
  assert.equal(container.querySelector('iframe'), null, 'no iframe before the preview click');

  const previewButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === '预览');
  assert.ok(previewButton, 'preview button rendered');
  await act(async () => { previewButton!.click(); });
  // The modal defers the iframe mount by one tick (Vue nextTick parity,
  // EmbedChannelPreview.vue watch visible L95-104).
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 5)); });

  const dialog = container.querySelector('[role="dialog"]');
  assert.ok(dialog, 'modal dialog rendered after the preview click');
  const iframe = dialog!.querySelector('iframe');
  assert.ok(iframe, 'modal renders the preview iframe');
  assert.equal(iframe!.getAttribute('src'), 'https://weknora.test/embed/ch-1?locale=zh-CN#token=tok_preview');
  assert.equal(iframe!.getAttribute('allow'), 'clipboard-write');

  // Vue EmbedChannelPreview drawer close surface (header ×, update:visible false).
  const closeButton = Array.from(container.querySelectorAll('button')).find((button) => button.title === '关闭');
  assert.ok(closeButton, 'close button rendered');
  await act(async () => { closeButton!.click(); });
  assert.equal(container.querySelector('[role="dialog"]'), null, 'close button unmounts the modal');
});
