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
const { IntegrationsRoutePage } = await import('./IntegrationsRoutePage.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.removeItem('locale');
});

/** The preview modal defers the iframe mount by one tick (Vue nextTick parity). */
async function flushFrame() {
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
}

interface StubChannel { id: string; name?: string; default_locale?: string; [key: string]: unknown }

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function stubClient(input: { channels: StubChannel[]; sessionTokens: string[]; previewCalls: string[]; channelList?: () => Promise<StubChannel[]> }) {
  return {
    embed: {
      channels: {
        listAll: input.channelList ?? (async () => input.channels as never[]),
        get: async (id: string) => input.channels.find((channel) => channel.id === id) as never,
        previewSession: async (id: string) => {
          input.previewCalls.push(id);
          return { sessionToken: input.sessionTokens.shift() ?? '', expiresIn: 600 };
        },
      },
      im: { listAll: async () => [] },
    },
    configuration: { agents: { list: async () => [{ id: 'agent-1', name: '知识助手', config: {} }] } },
  } as unknown as Parameters<typeof IntegrationsRoutePage>[0]['client'];
}

test('integration channel panels keep the Vue loading state until the list request settles', async () => {
  const channelsGate = deferred<StubChannel[]>();
  const client = stubClient({ channels: [{ id: 'ch-1', name: '客服渠道' }], sessionTokens: [], previewCalls: [], channelList: () => channelsGate.promise });
  const container = await mountRoutePage(client, []);
  assert.match(container.textContent ?? '', /正在加载/);
  await act(async () => { channelsGate.resolve([]); await channelsGate.promise; });
  assert.doesNotMatch(container.textContent ?? '', /正在加载/);
});

async function mountRoutePage(client: Parameters<typeof IntegrationsRoutePage>[0]['client'], openedNewTabs: string[][], locale = 'zh-CN') {
  const container = document.createElement('div');
  document.body.append(container);
  dom.window.localStorage.setItem('locale', locale);
  (dom.window as unknown as { open: (...args: string[]) => unknown }).open = (...args: string[]) => { openedNewTabs.push(args); return null; };
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(IntegrationsRoutePage, { client, tenantId: null, activeTab: 'embed' }));
  });
  await act(async () => {});
  return container;
}

async function openDeployStepAndPreview(container: HTMLElement) {
  await act(async () => { (container.querySelector('article') as HTMLElement).click(); });
  await act(async () => {});
  const previewButton = Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-embed-code-panel button')).find((button) => /预览|Preview/.test(button.textContent ?? ''));
  assert.ok(previewButton, 'deploy step shows the Vue 预览 button');
  await act(async () => { previewButton!.click(); });
  await act(async () => {});
  await act(async () => {});
}

// Vue openPreviewForChannel (AgentEmbedChannelPanel.vue L993-1036) never opens
// a new tab: an unavailable preview session warns 预览暂时不可用 in place and
// the page stays mounted (Vue MessagePlugin.warning semantics — not the
// page-level error state, which would unmount every integrations panel).
test('empty preview session warns in place, keeps the page, never opens a new tab', async () => {
  const openedNewTabs: string[][] = [];
  const previewCalls: string[] = [];
  const client = stubClient({
    channels: [{ id: 'ch-1', name: '客服渠道', agent_id: 'agent-1', default_locale: 'en-US', allowed_origins: ['https://a.example.test'] }],
    sessionTokens: [''],
    previewCalls,
  });
  const container = await mountRoutePage(client, openedNewTabs);

  await openDeployStepAndPreview(container);

  assert.deepEqual(previewCalls, ['ch-1', 'ch-1'], 'views mint attempt + route-shell fallback attempt');
  assert.deepEqual(openedNewTabs, [], 'preview never falls back to a new tab (Vue L1007-1010)');
  // The preview drawer is the only aside[role="dialog"] with an aria-label
  // (replaces the former .wk-embed-preview-drawer class hook).
  assert.equal(container.querySelector('aside[role="dialog"][aria-label]'), null, 'no drawer without a token');
  assert.ok(container.querySelector('article'), 'channel list stays mounted (no page-level error wipe)');
  const alert = container.querySelector('[role="alert"]');
  assert.ok(alert, 'warning surfaced');
  assert.match(alert!.textContent!, /预览暂时不可用/);
});

test('preview warning follows the active locale', async () => {
  const client = stubClient({
    channels: [{ id: 'ch-1', name: 'Support', agent_id: 'agent-1', default_locale: 'en-US' }],
    sessionTokens: [''],
    previewCalls: [],
  });
  const container = await mountRoutePage(client, [], 'en-US');
  await openDeployStepAndPreview(container);
  assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /Preview is unavailable/);
});

// When the fallback mint succeeds the preview still stays in-page: the shell
// modal opens with the channel locale and a fresh refresh nonce (?r=1), the
// route wiring of Vue previewLocale/previewNonce (L1021-1022).
test('fallback with a fresh token opens the in-page modal with locale + nonce', async () => {
  const openedNewTabs: string[][] = [];
  const previewCalls: string[] = [];
  const client = stubClient({
    channels: [{ id: 'ch-1', name: '客服渠道', agent_id: 'agent-1', default_locale: 'en-US', allowed_origins: ['https://a.example.test'] }],
    sessionTokens: ['', 'tok_fresh'],
    previewCalls,
  });
  const container = await mountRoutePage(client, openedNewTabs);

  await openDeployStepAndPreview(container);

  assert.deepEqual(openedNewTabs, [], 'preview never opens a new tab');
  const dialog = container.querySelector('[role="dialog"]');
  assert.ok(dialog, 'shell preview modal mounted in-page');
  assert.equal(dialog!.getAttribute('aria-label'), '客服渠道');
  assert.equal(container.querySelector('[role="alert"]'), null, 'no warning on success');

  await flushFrame();
  const iframe = dialog!.querySelector('iframe');
  assert.ok(iframe, 'preview iframe mounted');
  assert.equal(iframe!.getAttribute('src'), 'https://weknora.test/embed/ch-1?locale=en-US&r=1#token=tok_fresh', 'shell modal carries locale + refresh nonce');
});
