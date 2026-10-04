import assert from 'node:assert/strict';
import test from 'node:test';
import React, { act } from 'react';
import { createScopeController } from '@weknora/domain/scope';
import { createRequire } from 'node:module';

const hooks = createRequire(import.meta.url)('node:module') as typeof import('node:module') & {
  registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void;
};
hooks.registerHooks?.({ resolve: (specifier, context, nextResolve) =>
  specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context),
});
Object.assign(globalThis, { React, IS_REACT_ACT_ENVIRONMENT: true });
const { CraftRoutes } = await import('./routes.tsx');
const { JSDOM } = await import('jsdom');

// The T20 consent assembly check: both consent panels ride the workbench
// aside, bound to the SELECTED published version, decide through the
// api-client seams with the exact digest, and re-project the answer.

const version = (id: string) => ({ id, workspace_id: 'workspace-task-1', run_id: 'run-1', kind: 'web', files: [], checks: [], web_evidence: null });

function workspace(sessionId: string) {
  return {
    session_id: sessionId, workspace_id: `workspace-${sessionId}`, kind: 'web', title: `Task ${sessionId}`, engine_type: 'trpc',
    workspace: { id: `workspace-${sessionId}`, session_id: sessionId, user_id: 'owner', sandbox_id: 'sandbox', generation: '1', runtime_digest: 'digest', revision: 1 },
    active_run_id: null, pending_id: null, last_seq: 0, active_run: null, current_version: version('v-1'),
  };
}

const sharePrivate = { version_id: 'v-1', restricted: true, evidence_digest: 'e'.repeat(64), status: 'private', decision: null };
const shareConsented = (decision: string) => ({
  version_id: 'v-1', restricted: true, evidence_digest: 'e'.repeat(64), status: decision === 'approved' ? 'consented' : 'declined',
  decision: { version_id: 'v-1', evidence_digest: 'e'.repeat(64), owner_id: 'owner', decision }, expires_at: '2026-10-05T00:00:00Z',
});

const exportAwaiting = {
  version_id: 'v-1', manifest_digest: 'm'.repeat(64), state: 'awaiting', restricted_derived: ['index.html'],
  files: [{ path: 'index.html', sha256: 'f'.repeat(64), restricted: false, origins: [{ kind: 'knowledge', ref: 'craftkb://kb-1/doc-1', sha256: 'o'.repeat(64), restricted: true }] }],
  decision: null,
};
const exportDecided = (decision: string) => ({ ...exportAwaiting, state: decision === 'approved' ? 'consented' : 'declined', decision: { version_id: 'v-1', manifest_digest: 'm'.repeat(64), owner_id: 'owner', decision } });

async function harness(userId = 'owner') {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/craft/task-1' });
  const previous = { window: globalThis.window, document: globalThis.document, HTMLElement: globalThis.HTMLElement, Event: globalThis.Event, ResizeObserver: globalThis.ResizeObserver, MutationObserver: globalThis.MutationObserver, requestAnimationFrame: globalThis.requestAnimationFrame, cancelAnimationFrame: globalThis.cancelAnimationFrame };
  const previousNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator');
  const raf = (_callback: FrameRequestCallback) => 0;
  const caf = (_handle: number) => {};
  class TestResizeObserver { observe() {} unobserve() {} disconnect() {} }
  Object.assign(dom.window, { matchMedia: () => ({ matches: false, media: '', onchange: null, addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {}, dispatchEvent: () => true }), requestAnimationFrame: raf, cancelAnimationFrame: caf, ResizeObserver: TestResizeObserver, scrollTo() {} });
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Event: dom.window.Event, requestAnimationFrame: raf, cancelAnimationFrame: caf, ResizeObserver: TestResizeObserver, MutationObserver: dom.window.MutationObserver });
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
  const { createRoot } = await import('react-dom/client');
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const state: { share: unknown; exportConsent: unknown } = { share: { ...sharePrivate }, exportConsent: { ...exportAwaiting } };
  const client = {
    request: async (input: { method: string; path: string; body?: unknown }) => {
      calls.push(input);
      if (input.path === '/api/v1/auth/me') return { data: { user: { id: userId } } };
      if (input.path.endsWith('/craft/access') && input.method === 'GET') return { success: true, data: [{ user_id: 'owner', role: 'owner' }, { user_id: 'viewer', role: 'viewer' }] };
      if (input.path.endsWith('/craft/versions') && input.method === 'GET') return { success: true, data: [version('v-1')], next_cursor: null };
      if (/^\/api\/v1\/sessions\/[^/]+\/craft$/.test(input.path)) return { success: true, data: workspace(input.path.split('/')[4]!) };
      if (input.path.endsWith('/versions/v-1/share') && input.method === 'GET') return { ...state.share as object };
      if (input.path.endsWith('/versions/v-1/share/decision')) { state.share = shareConsented((input.body as { decision: string }).decision); return { ...state.share as object }; }
      if (input.path.endsWith('/versions/v-1/share/revocation')) { state.share = { ...sharePrivate }; return { ...state.share as object }; }
      if (input.path.endsWith('/versions/v-1/export/consent') && input.method === 'GET') return { ...state.exportConsent as object };
      if (input.path.endsWith('/versions/v-1/export/consent/decision')) { state.exportConsent = exportDecided((input.body as { decision: string }).decision); return { ...state.exportConsent as object }; }
      throw new Error(`Unexpected ${input.method} ${input.path}`);
    },
    knowledgeBases: { list: async () => [], documents: { list: async () => ({ data: [] }) } },
  };
  const fetchImpl = async () => new Response(JSON.stringify({ data: [] }), { status: 200, headers: { 'content-type': 'application/json' } });
  Object.assign(globalThis, { fetch: fetchImpl });
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  const props = {
    client: client as never,
    scopeController: createScopeController({ origin: 'web', userId, tenantId: 'tenant-1' }),
    session: { credential: { kind: 'bearer' as const, accessToken: 'token' }, tenantId: 'tenant-1' },
    apiBaseUrl: '',
  };
  return {
    dom, container, root, calls, state,
    async render() { await act(async () => { root.render(<CraftRoutes {...props} />); await Promise.resolve(); }); },
    async settle() { await act(async () => { for (let i = 0; i < 10; i++) await Promise.resolve(); }); },
    async cleanup() {
      await act(async () => root.unmount());
      Object.assign(globalThis, previous);
      if (previousNavigator) Object.defineProperty(globalThis, 'navigator', previousNavigator);
      dom.window.close();
    },
  };
}

test('T20 consent assembly: both consent panels ride the aside and decide with the exact digest', async () => {
  const view = await harness();
  try {
    await view.render();
    await view.settle();
    const share = view.container.querySelector('section.wk-craft-share');
    assert.ok(share, 'the restricted share panel is mounted on the selected version');
    assert.equal(share.getAttribute('data-status'), 'private');
    const exportPanel = view.container.querySelector('section.wk-craft-export');
    assert.ok(exportPanel, 'the restricted export consent panel is mounted on the selected version');
    assert.equal(exportPanel.getAttribute('data-status'), 'awaiting');

    const confirm = share.querySelector('button.wk-craft-share-confirm') as HTMLButtonElement;
    assert.ok(confirm, 'the owner sees the confirm-share control');
    await act(async () => { confirm.click(); await Promise.resolve(); });
    await view.settle();
    const shareDecision = view.calls.find((call) => call.method === 'POST' && call.path.endsWith('/share/decision'));
    assert.deepEqual(shareDecision?.body, { decision: 'approved', evidence_digest: 'e'.repeat(64) }, 'the decision binds the loaded evidence digest');
    assert.equal(view.container.querySelector('section.wk-craft-share')?.getAttribute('data-status'), 'consented', 'the panel re-projects the server answer');

    const revoke = view.container.querySelector('button.wk-craft-share-revoke') as HTMLButtonElement;
    assert.ok(revoke, 'a consented owner can revoke');
    await act(async () => { revoke.click(); await Promise.resolve(); });
    await view.settle();
    assert.ok(view.calls.some((call) => call.method === 'POST' && call.path.endsWith('/share/revocation')));
    assert.equal(view.container.querySelector('section.wk-craft-share')?.getAttribute('data-status'), 'private', 'revocation returns the panel to private');

    const approve = exportPanel.querySelector('button.wk-craft-export-approve') as HTMLButtonElement;
    assert.ok(approve, 'the owner sees the export-approve control');
    await act(async () => { approve.click(); await Promise.resolve(); });
    await view.settle();
    const exportDecision = view.calls.find((call) => call.method === 'POST' && call.path.endsWith('/export/consent/decision'));
    assert.deepEqual(exportDecision?.body, { decision: 'approved', manifest_digest: 'm'.repeat(64) }, 'the export decision binds the loaded manifest digest');
    assert.equal(view.container.querySelector('section.wk-craft-export')?.getAttribute('data-status'), 'consented');
  } finally { await view.cleanup(); }
});

test('T20 consent assembly: unrestricted versions render no consent panel', async () => {
  const view = await harness();
  view.state.share = { version_id: 'v-1', restricted: false, evidence_digest: 'e'.repeat(64), status: 'private', decision: null };
  view.state.exportConsent = { version_id: 'v-1', manifest_digest: 'm'.repeat(64), state: 'none', restricted_derived: [], files: [], decision: null };
  try {
    await view.render();
    await view.settle();
    assert.equal(view.container.querySelector('section.wk-craft-share'), null, 'an unrestricted version hides the share panel');
    assert.equal(view.container.querySelector('section.wk-craft-export'), null, 'a none-state version hides the export panel');
    assert.ok(!view.calls.some((call) => call.path.includes('/share/decision') || call.path.includes('/export/consent/decision')));
  } finally { await view.cleanup(); }
});
