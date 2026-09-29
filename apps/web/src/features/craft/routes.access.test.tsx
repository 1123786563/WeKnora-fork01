import assert from 'node:assert/strict';
import test from 'node:test';
import React, { act } from 'react';
import { File as NodeFile } from 'node:buffer';
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

const workspace = (sessionId: string, activeRun: unknown = null) => ({
  session_id: sessionId,
  workspace_id: `workspace-${sessionId}`,
  kind: 'web',
  title: `Task ${sessionId}`,
  engine_type: 'trpc',
  workspace: { id: `workspace-${sessionId}`, session_id: sessionId, user_id: 'owner', sandbox_id: 'sandbox', generation: '1', runtime_digest: 'digest', revision: 1 },
  active_run_id: activeRun === null ? null : (activeRun as { run_id: string }).run_id,
  pending_id: null,
  last_seq: 0,
  active_run: activeRun,
  current_version: null,
});

function member(user_id: string, role: 'owner' | 'collaborator' | 'viewer') { return { user_id, role }; }

type FakeRequest = (input: { method: string; path: string; body?: unknown }) => Promise<unknown>;

async function harness(initialMembers: Array<{ user_id: string; role: string }>, userId = 'owner', narrow = false) {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/craft/task-1' });
  const previous = { window: globalThis.window, document: globalThis.document, HTMLElement: globalThis.HTMLElement, Event: globalThis.Event, fetch: globalThis.fetch, ResizeObserver: globalThis.ResizeObserver, MutationObserver: globalThis.MutationObserver, requestAnimationFrame: globalThis.requestAnimationFrame, cancelAnimationFrame: globalThis.cancelAnimationFrame };
  const previousNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator');
  const raf = (_callback: FrameRequestCallback) => 0;
  const caf = (_handle: number) => {};
  class TestResizeObserver { observe() {} unobserve() {} disconnect() {} }
  Object.assign(dom.window, { matchMedia: () => ({ matches: narrow, media: '', onchange: null, addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {}, dispatchEvent: () => true }), requestAnimationFrame: raf, cancelAnimationFrame: caf, ResizeObserver: TestResizeObserver, scrollTo() {} });
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Event: dom.window.Event, requestAnimationFrame: raf, cancelAnimationFrame: caf, ResizeObserver: TestResizeObserver, MutationObserver: dom.window.MutationObserver });
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
  const { createRoot } = await import('react-dom/client');
  let members = [...initialMembers];
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  let inputCount = 0;
  let decisionHandler: FakeRequest = async (input) => {
    const body = input.body as { ref: string; action: string };
    return { success: true, data: { ref: body.ref, action: body.action } };
  };
  let submitHandler: FakeRequest = async (input) => ({ success: true, data: {
    run_id: `run-${calls.filter((call) => call.path.endsWith('/craft/runs')).length}`,
    session_id: input.path.split('/')[4]!, status: 'queued', wait_reason: '', revision: 1, epoch: 1, seq: 0, pending_id: null, budget_pause: null,
  } });
  let activeRun: unknown = null;
  let budgetPause: unknown = { success: true, data: { run_id: 'run-budget', reason: 'budget_exhausted', limit: 10, used: 10 }, can_extend: false };
  let extensionHandler: FakeRequest = async () => ({ success: true });
  const client: { request: FakeRequest; knowledgeBases: { list(): Promise<unknown[]>; documents: { list(): Promise<{ data: unknown[] }> } } } = {
    request: async (input) => {
      calls.push(input);
      if (input.path === '/api/v1/auth/me') return { data: { user: { id: userId } } };
      if (input.path.endsWith('/craft/access') && input.method === 'GET') return { success: true, data: members };
      if (input.path.endsWith('/craft/access') && input.method === 'POST') {
        const body = input.body as { user_id: string; role: 'collaborator' | 'viewer' };
        members = [...members.filter((item) => item.user_id !== body.user_id), member(body.user_id, body.role)];
        return { success: true };
      }
      if (input.path.endsWith('/craft/inputs/decision') && input.method === 'POST') return decisionHandler(input);
      if (input.path.endsWith('/budget/pause') && input.method === 'GET') return budgetPause;
      if (input.path.endsWith('/budget/extend') && input.method === 'POST') return extensionHandler(input);
      if (input.path.endsWith('/craft/inputs') && input.method === 'POST') {
        inputCount += 1;
        return { success: true, data: {
          ref: `opaque://input-${inputCount}`, name: 'mystery.bin', sha256: 'a'.repeat(64), bytes: 8, citation_id: `citation-${inputCount}`,
          recognition: { accepted: true, understood: false, reason: 'unrecognized_format' },
        } };
      }
      if (input.path.endsWith('/craft/runs') && input.method === 'POST') return submitHandler(input);
      if (input.path.endsWith('/craft/access/revoke')) {
        members = members.filter((item) => item.user_id !== (input.body as { user_id: string }).user_id);
        return { success: true };
      }
      if (input.path.endsWith('/craft/versions')) return { success: true, data: [], next_cursor: null };
      if (input.path === '/api/v1/craft/sessions') return { success: true, data: [{ session_id: 'task-1', workspace_id: 'workspace-task-1', kind: 'web', title: 'Task one', engine_type: 'trpc', updated_at: '2026-09-23T00:00:00Z' }], next_cursor: null };
      if (/^\/api\/v1\/sessions\/[^/]+\/craft$/.test(input.path)) {
        const sessionId = input.path.split('/')[4]!;
        return { success: true, data: workspace(sessionId, activeRun) };
      }
      throw new Error(`Unexpected ${input.method} ${input.path}`);
    },
    knowledgeBases: { list: async () => [], documents: { list: async () => ({ data: [] }) } },
  };
  const fetchCalls: string[] = [];
  let uploadFails = false;
  const fetchImpl = async (input: RequestInfo | URL, init?: RequestInit) => {
    fetchCalls.push(String(input));
    if (String(input).endsWith('/attachments') && init?.method === 'POST' && uploadFails) return new Response('upload failed', { status: 500 });
    if (String(input).endsWith('/attachments') && init?.method === 'POST') return new Response(JSON.stringify({ data: { id: 'uploaded-1' } }), { status: 200, headers: { 'content-type': 'application/json' } });
    if (String(input).endsWith('/attachments/uploaded-1')) return new Response(JSON.stringify({ data: { status: 'ready' } }), { status: 200, headers: { 'content-type': 'application/json' } });
    return new Response(JSON.stringify({ data: [] }), { status: 200, headers: { 'content-type': 'application/json' } });
  };
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
    dom, container, root, props, client, calls, fetchCalls,
    setDecisionHandler(handler: FakeRequest) { decisionHandler = handler; },
    setSubmitHandler(handler: FakeRequest) { submitHandler = handler; },
    setBudgetPause(value: unknown) { budgetPause = value; },
    setActiveRun(value: unknown) { activeRun = value; },
    setExtensionHandler(handler: FakeRequest) { extensionHandler = handler; },
    setUploadFails(value: boolean) { uploadFails = value; },
    async render() { await act(async () => { root.render(<CraftRoutes {...props} />); await Promise.resolve(); }); },
    async settle() { await act(async () => { for (let i = 0; i < 8; i++) await Promise.resolve(); }); },
    async cleanup() {
      await act(async () => root.unmount());
      Object.assign(globalThis, previous);
      if (previousNavigator) Object.defineProperty(globalThis, 'navigator', previousNavigator);
      dom.window.close();
    },
  };
}

async function startUnknownInputSend(view: Awaited<ReturnType<typeof harness>>) {
  const english = [...view.container.querySelectorAll('button')].find((button) => button.textContent === 'English');
  assert.ok(english, 'locale switch is available');
  await act(async () => { english.click(); });
  await view.settle();
  const upload = view.container.querySelector('[data-testid="craft-upload"]') as HTMLInputElement;
  assert.ok(upload, 'workbench upload input is mounted');
  Object.defineProperty(upload, 'files', { configurable: true, value: [new NodeFile(['opaque payload'], 'mystery.bin')] });
  await act(async () => { upload.dispatchEvent(new view.dom.window.Event('change', { bubbles: true })); });
  const prompt = view.container.querySelector('[data-testid="craft-prompt"]') as HTMLTextAreaElement;
  const setter = Object.getOwnPropertyDescriptor(view.dom.window.HTMLTextAreaElement.prototype, 'value')?.set;
  setter?.call(prompt, 'Review this input');
  await act(async () => { prompt.dispatchEvent(new view.dom.window.Event('input', { bubbles: true })); });
  await act(async () => { (view.container.querySelector('[data-testid="craft-send"]') as HTMLButtonElement).click(); });
  await view.settle();
  return view.container.querySelector('[aria-label="Unrecognized input materials"]');
}

test('Craft workbench mounts server-backed Task access, refreshes after grant and revoke', async () => {
  const view = await harness([member('owner', 'owner'), member('viewer', 'viewer')]);
  try {
    await view.render();
    await view.settle();
    const access = view.container.querySelector('[aria-label="Task access"]');
    assert.ok(access, 'the Craft workbench exposes Task access');
    assert.match(access.textContent ?? '', /viewer/);
    const input = access.querySelector('input') as HTMLInputElement;
    assert.ok(input);
    const setter = Object.getOwnPropertyDescriptor(view.dom.window.HTMLInputElement.prototype, 'value')?.set;
    setter?.call(input, 'collaborator');
    await act(async () => { input.dispatchEvent(new view.dom.window.Event('input', { bubbles: true })); });
    await act(async () => { (access.querySelector('select') as HTMLSelectElement).value = 'collaborator'; (access.querySelector('select') as HTMLSelectElement).dispatchEvent(new view.dom.window.Event('change', { bubbles: true })); });
    await act(async () => { (access.querySelector('form') as HTMLFormElement).dispatchEvent(new view.dom.window.Event('submit', { bubbles: true, cancelable: true })); await Promise.resolve(); });
    await view.settle();
    assert.match(access.textContent ?? '', /collaborator: collaborator/);
    assert.ok(view.calls.some((call) => call.method === 'POST' && call.path.endsWith('/craft/access')));
    const revoke = [...access.querySelectorAll('button')].find((button) => button.getAttribute('aria-label')?.includes('viewer')) as HTMLButtonElement;
    assert.ok(revoke);
    await act(async () => { revoke.click(); await Promise.resolve(); });
    await view.settle();
    assert.doesNotMatch(access.textContent ?? '', /viewer: viewer/);
    assert.ok(view.calls.filter((call) => call.method === 'GET' && call.path.endsWith('/craft/access')).length >= 3, 'mount, grant, and revoke each refetch authoritative membership');
  } finally { await view.cleanup(); }
});

test('server-authorized collaborator can request extension, and retry reuses its key while in flight is disabled', async () => {
  const view = await harness([member('owner', 'owner'), member('collaborator', 'collaborator')], 'collaborator');
  view.setActiveRun({ run_id: 'run-budget', session_id: 'task-1', status: 'waiting_user', wait_reason: 'budget_exhausted', revision: 1, epoch: 1, seq: 0, pending_id: null, budget_pause: null });
  const action = { key: 'server-intent-run-budget', extra_calls: 17, extra_credits: 2345678 };
  view.setBudgetPause({ success: true, data: { run_id: 'run-budget', reason: 'budget_exhausted', limit: 10, used: 10, extension_action: action }, can_extend: true });
  const extensionBodies: Array<Record<string, unknown>> = [];
  let rejectFirst!: (error: Error) => void;
  let resolveSecond!: (value: unknown) => void;
  let attempts = 0;
  view.setExtensionHandler(async (input) => {
    extensionBodies.push(input.body as Record<string, unknown>);
    attempts += 1;
    if (attempts === 1) return new Promise((_, reject) => { rejectFirst = reject; });
    return new Promise((resolve) => { resolveSecond = resolve; });
  });
  try {
    await view.render();
    await view.settle();
    const pause = view.container.querySelector('[data-testid="craft-budget-pause"]');
    assert.ok(pause, 'budget pause notice is rendered');
    const extend = [...pause.querySelectorAll('button')].find((button) => button.textContent === '申请增加预算') as HTMLButtonElement;
    assert.ok(extend, 'a non-owner collaborator receives the server-authorized action');
    assert.equal(extend.disabled, false);

    await act(async () => { extend.click(); });
    await view.settle();
    assert.equal(extensionBodies.length, 1);
    assert.deepEqual(extensionBodies[0], action, 'first request sends the server-owned key and quantum unchanged');
    assert.equal(extend.disabled, true, 'the action is disabled while the request is in flight');
    await act(async () => { rejectFirst(new Error('temporary connection failure')); });
    await view.settle();
    assert.equal(extend.disabled, false, 'a failed transient request can be retried');

    await act(async () => { extend.click(); });
    await view.settle();
    assert.equal(extensionBodies.length, 2);
    assert.equal(extensionBodies[1]?.key, extensionBodies[0]?.key, 'retry replays the same logical extension key');
    assert.deepEqual(extensionBodies[1], action, 'retry replays the exact server-owned tuple');
    assert.equal(extend.disabled, true, 'the retry is also guarded while in flight');
    await act(async () => { resolveSecond({ success: true }); });
    await view.settle();
    assert.ok(view.calls.some((call) => call.path.endsWith('/budget/extend')));
  } finally { await view.cleanup(); }
});

test('ambiguous extension retry reuses the server intent after CraftRoutes is recreated', async () => {
  const members = [member('owner', 'owner'), member('collaborator', 'collaborator')];
  const activeRun = { run_id: 'run-budget', session_id: 'task-1', status: 'waiting_user', wait_reason: 'budget_exhausted', revision: 1, epoch: 1, seq: 0, pending_id: null, budget_pause: null };
  const action = { key: 'durable-server-intent', extra_calls: 10, extra_credits: 10000000 };
  const first = await harness(members, 'collaborator');
  first.setActiveRun(activeRun);
  first.setBudgetPause({ success: true, data: { run_id: 'run-budget', reason: 'budget_exhausted', limit: 10, used: 10, extension_action: action }, can_extend: true });
  let firstKey = '';
  first.setExtensionHandler(async (input) => {
    firstKey = String((input.body as Record<string, unknown>).key);
    throw new Error('response lost after server may have resumed the Run');
  });
  try {
    await first.render();
    await first.settle();
    const extend = [...first.container.querySelectorAll('[data-testid="craft-budget-pause"] button')][0] as HTMLButtonElement;
    await act(async () => { extend.click(); });
    await first.settle();
  } finally { await first.cleanup(); }

  const second = await harness(members, 'collaborator');
  second.setActiveRun(activeRun);
  second.setBudgetPause({ success: true, data: { run_id: 'run-budget', reason: 'budget_exhausted', limit: 10, used: 10, extension_action: action }, can_extend: true });
  let retryKey = '';
  second.setExtensionHandler(async (input) => {
    retryKey = String((input.body as Record<string, unknown>).key);
    return { success: true };
  });
  try {
    await second.render();
    await second.settle();
    const extend = [...second.container.querySelectorAll('[data-testid="craft-budget-pause"] button')][0] as HTMLButtonElement;
    await act(async () => { extend.click(); });
    await second.settle();
    assert.ok(second.calls.some((call) => call.method === 'GET' && call.path.endsWith('/budget/pause')), 'remount refetches the server-owned pending action');
    assert.equal(retryKey, firstKey, 'new component instance replays the unresolved extension key');
  } finally { await second.cleanup(); }

  const laterPause = await harness(members, 'collaborator');
  laterPause.setActiveRun(activeRun);
  const nextAction = { key: 'next-server-intent', extra_calls: 12, extra_credits: 3456789 };
  laterPause.setBudgetPause({ success: true, data: { run_id: 'run-budget', reason: 'budget_exhausted', limit: 20, used: 20, extension_action: nextAction }, can_extend: true });
  let laterKey = '';
  laterPause.setExtensionHandler(async (input) => {
    laterKey = String((input.body as Record<string, unknown>).key);
    return { success: true };
  });
  try {
    await laterPause.render();
    await laterPause.settle();
    const extend = [...laterPause.container.querySelectorAll('[data-testid="craft-budget-pause"] button')][0] as HTMLButtonElement;
    await act(async () => { extend.click(); });
    await laterPause.settle();
    assert.equal(laterKey, nextAction.key, 'a later pause uses the new server-owned extension key');
  } finally { await laterPause.cleanup(); }
});

test('missing server extension action keeps the budget notice non-actionable', async () => {
  const view = await harness([member('owner', 'owner')]);
  view.setActiveRun({ run_id: 'run-budget', session_id: 'task-1', status: 'waiting_user', wait_reason: 'budget_exhausted', revision: 1, epoch: 1, seq: 0, pending_id: null, budget_pause: null });
  view.setBudgetPause({ success: true, data: { run_id: 'run-budget', reason: 'budget_exhausted', limit: 10, used: 10, extension_action: null }, can_extend: true });
  try {
    await view.render();
    await view.settle();
    const pause = view.container.querySelector('[data-testid="craft-budget-pause"]');
    assert.ok(pause);
    assert.equal(pause.querySelector('button'), null, 'can_extend cannot invent a client action when the server returned no intent');
  } finally { await view.cleanup(); }
});

test('Craft workbench keeps Viewer role read only and fails closed when access cannot be loaded', async () => {
  const viewer = await harness([member('owner', 'owner'), member('viewer', 'viewer')], 'viewer');
  try {
    await viewer.render();
    await viewer.settle();
    const access = viewer.container.querySelector('[aria-label="Task access"]');
    assert.ok(access);
    assert.doesNotMatch(access.textContent ?? '', /Add member|Revoke/);
  } finally { await viewer.cleanup(); }

  const denied = await harness([member('owner', 'owner')]);
  const originalRequest = denied.client.request;
  denied.client.request = async (input) => input.path.endsWith('/craft/access')
    ? { success: false, error: { code: 'FORBIDDEN', message: 'Forbidden' } }
    : originalRequest(input);
  try {
    await denied.render();
    await denied.settle();
    assert.equal(denied.container.querySelector('[aria-label="Task access"]'), null);
    assert.match(denied.container.textContent ?? '', /access.*unavailable|forbidden/i);
    assert.doesNotMatch(denied.container.textContent ?? '', /Add member/);
  } finally { await denied.cleanup(); }

  const collaborator = await harness([member('owner', 'owner'), member('collaborator', 'collaborator')], 'collaborator');
  try {
    await collaborator.render();
    await collaborator.settle();
    const access = collaborator.container.querySelector('[aria-label="Task access"]');
    assert.ok(access);
    assert.doesNotMatch(access.textContent ?? '', /Add member|Revoke/);
  } finally { await collaborator.cleanup(); }

  const unidentified = await harness([member('owner', 'owner')], 'user-not-listed');
  try {
    await unidentified.render();
    await unidentified.settle();
    assert.match(unidentified.container.textContent ?? '', /role could not be confirmed/i);
    assert.doesNotMatch(unidentified.container.textContent ?? '', /Add member|Revoke/);
  } finally { await unidentified.cleanup(); }
});

test('stale access response from a prior Task cannot replace current route membership', async () => {
  const view = await harness([member('owner', 'owner')]);
  let resolveOld!: (value: unknown) => void;
  const request = view.client.request;
  view.client.request = async (input) => {
    if (input.method === 'GET' && input.path.endsWith('/sessions/task-1/craft/access')) {
      return new Promise<unknown>((resolve) => { resolveOld = resolve; });
    }
    if (input.method === 'GET' && input.path.endsWith('/sessions/task-2/craft/access')) {
      return { success: true, data: [member('owner', 'owner'), member('task-2-member', 'viewer')] };
    }
    return request(input);
  };
  try {
    await view.render();
    await view.settle();
    assert.match(view.container.textContent ?? '', /Loading Task access/);
    await act(async () => {
      view.dom.window.history.pushState(null, '', '/craft/task-2');
      view.dom.window.dispatchEvent(new view.dom.window.PopStateEvent('popstate'));
    });
    await view.settle();
    const access = view.container.querySelector('[aria-label="Task access"]');
    assert.ok(access);
    assert.match(access.textContent ?? '', /task-2-member/);
    await act(async () => resolveOld({ success: true, data: [member('owner', 'owner'), member('stale-task-1-member', 'viewer')] }));
    await view.settle();
    assert.match(access.textContent ?? '', /task-2-member/);
    assert.doesNotMatch(view.container.textContent ?? '', /stale-task-1-member/);
  } finally { await view.cleanup(); }
});

test('an accepted but unrecognized input pauses Run until the exact continue decision is acknowledged', async () => {
  const view = await harness([member('owner', 'owner')], 'owner', true);
  try {
    await view.render();
    await view.settle();
    const panel = await startUnknownInputSend(view);
    assert.ok(panel, 'unrecognized material is disclosed in the workbench');
    assert.match(panel.textContent ?? '', /Accepted, but content was not understood/);
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/runs')).length, 0, 'no Run is submitted while the decision is pending');
    await act(async () => { [...panel.querySelectorAll('button')].find((button) => button.textContent === 'Continue')!.click(); });
    await view.settle();
    assert.deepEqual(view.calls.filter((call) => call.path.endsWith('/craft/inputs/decision')).map((call) => call.body), [
      { ref: 'opaque://input-1', action: 'continue' },
    ]);
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/runs')).length, 1, 'one acknowledged continue admits exactly one Run');
  } finally { await view.cleanup(); }
});

test('Cancel is server acknowledged and submits no Run', async () => {
  const view = await harness([member('owner', 'owner')], 'owner', true);
  try {
    await view.render();
    await view.settle();
    const panel = await startUnknownInputSend(view);
    assert.ok(panel);
    await act(async () => { [...panel.querySelectorAll('button')].find((button) => button.textContent === 'Cancel')!.click(); });
    await view.settle();
    assert.deepEqual(view.calls.filter((call) => call.path.endsWith('/craft/inputs/decision')).map((call) => call.body), [
      { ref: 'opaque://input-1', action: 'cancel' },
    ]);
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/runs')).length, 0);
    assert.match(view.container.textContent ?? '', /cancel/i);
  } finally { await view.cleanup(); }
});

test('failed decision keeps the send blocked and offers a successful retry', async () => {
  const view = await harness([member('owner', 'owner')], 'owner', true);
  let attempts = 0;
  view.setDecisionHandler(async (input) => {
    attempts += 1;
    if (attempts === 1) return { success: false, error: { code: 'TEMPORARY', message: 'decision unavailable' } };
    const body = input.body as { ref: string; action: string };
    return { success: true, data: body };
  });
  try {
    await view.render();
    await view.settle();
    const panel = await startUnknownInputSend(view);
    assert.ok(panel);
    const continueButton = [...panel.querySelectorAll('button')].find((button) => button.textContent === 'Continue') as HTMLButtonElement;
    await act(async () => { continueButton.click(); });
    await view.settle();
    assert.match(panel.textContent ?? '', /Could not record decision/);
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/runs')).length, 0);
    await act(async () => { continueButton.click(); });
    await view.settle();
    assert.equal(attempts, 2);
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/runs')).length, 1);
  } finally { await view.cleanup(); }
});

test('upload failure submits no Run and retry still requires input decision', async () => {
  const view = await harness([member('owner', 'owner')], 'owner', true);
  view.setUploadFails(true);
  try {
    await view.render();
    await view.settle();
    assert.equal(await startUnknownInputSend(view), null);
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/runs')).length, 0);
    assert.match(view.container.textContent ?? '', /attachment upload failed/i);
    view.setUploadFails(false);
    await act(async () => { (view.container.querySelector('[data-testid="craft-send"]') as HTMLButtonElement).click(); });
    await view.settle();
    assert.ok(view.container.querySelector('[aria-label="Unrecognized input materials"]'));
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/runs')).length, 0);
  } finally { await view.cleanup(); }
});

test('a pending decision from the previous Task cannot authorize a Run after route switch', async () => {
  const view = await harness([member('owner', 'owner')], 'owner', true);
  let resolveDecision!: (value: unknown) => void;
  view.setDecisionHandler(async () => new Promise<unknown>((resolve) => { resolveDecision = resolve; }));
  try {
    await view.render();
    await view.settle();
    const panel = await startUnknownInputSend(view);
    assert.ok(panel);
    await act(async () => { [...panel.querySelectorAll('button')].find((button) => button.textContent === 'Continue')!.click(); });
    await view.settle();
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/runs')).length, 0);
    await act(async () => {
      view.dom.window.history.pushState(null, '', '/craft/task-2');
      view.dom.window.dispatchEvent(new view.dom.window.PopStateEvent('popstate'));
    });
    await view.settle();
    resolveDecision({ success: true, data: { ref: 'opaque://input-1', action: 'continue' } });
    await view.settle();
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/runs')).length, 0);
    assert.equal(view.container.querySelector('[aria-label="Unrecognized input materials"]'), null);
  } finally { await view.cleanup(); }
});

test('a lost Run response retries the exact accepted body without reuploading or asking again', async () => {
  const view = await harness([member('owner', 'owner')], 'owner', true);
  const bodies: Array<Record<string, unknown>> = [];
  let admittedRequestId: string | null = null;
  let attempts = 0;
  view.setSubmitHandler(async (input) => {
    const body = input.body as Record<string, unknown>;
    bodies.push(structuredClone(body));
    attempts += 1;
    if (attempts === 1) {
      // The server accepted K and created a Run, but the response was lost.
      admittedRequestId = String(body.request_id);
      throw new Error('connection dropped after admission');
    }
    assert.equal(body.request_id, admittedRequestId, 'the replay uses the admitted idempotency key');
    assert.deepEqual(body, bodies[0], 'the replay sends the byte-equivalent command body');
    return { success: true, data: {
      run_id: 'run-retry', session_id: 'task-1', status: 'queued', wait_reason: '', revision: 1, epoch: 1, seq: 0, pending_id: null, budget_pause: null,
    } };
  });
  try {
    await view.render();
    await view.settle();
    const firstPanel = await startUnknownInputSend(view);
    assert.ok(firstPanel);
    await act(async () => { [...firstPanel.querySelectorAll('button')].find((button) => button.textContent === 'Continue')!.click(); });
    await view.settle();
    assert.match(view.container.textContent ?? '', /connection dropped after admission/);
    assert.equal(bodies.length, 1);
    assert.deepEqual(bodies[0]?.input_refs, ['opaque://input-1'], 'first submission uses canonical ref A');

    const prompt = view.container.querySelector('[data-testid="craft-prompt"]') as HTMLTextAreaElement;
    const setter = Object.getOwnPropertyDescriptor(view.dom.window.HTMLTextAreaElement.prototype, 'value')?.set;
    setter?.call(prompt, 'A different prompt');
    await act(async () => { prompt.dispatchEvent(new view.dom.window.Event('input', { bubbles: true })); });
    await act(async () => { (view.container.querySelector('[data-testid="craft-send"]') as HTMLButtonElement).click(); });
    await view.settle();
    assert.equal(bodies.length, 1, 'a changed prompt cannot reuse the unresolved key');
    assert.match(view.container.textContent ?? '', /previous Run outcome is unresolved/i);

    setter?.call(prompt, 'Review this input');
    await act(async () => { prompt.dispatchEvent(new view.dom.window.Event('input', { bubbles: true })); });

    await act(async () => { (view.container.querySelector('[data-testid="craft-send"]') as HTMLButtonElement).click(); });
    await view.settle();
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/inputs')).length, 1, 'retry does not upload a new ref B');
    assert.equal(view.calls.filter((call) => call.path.endsWith('/craft/inputs/decision')).length, 1, 'retry does not ask for a second decision');
    assert.equal(bodies.length, 2, 'retry resolves the ambiguous submit via idempotent replay');
    assert.deepEqual(bodies[1], bodies[0]);
  } finally { await view.cleanup(); }
});

test('an ambiguous submit memo is cleared when the active Task changes', async () => {
  const view = await harness([member('owner', 'owner')], 'owner', true);
  const bodies: Array<{ request_id: string; input_refs: string[] }> = [];
  let attempts = 0;
  view.setSubmitHandler(async (input) => {
    const body = input.body as { request_id: string; input_refs: string[] };
    bodies.push(structuredClone(body));
    attempts += 1;
    if (attempts === 1) throw new Error('connection dropped after admission');
    return { success: true, data: {
      run_id: 'run-task-2', session_id: 'task-2', status: 'queued', wait_reason: '', revision: 1, epoch: 1, seq: 0, pending_id: null, budget_pause: null,
    } };
  });
  try {
    await view.render();
    await view.settle();
    const firstPanel = await startUnknownInputSend(view);
    assert.ok(firstPanel);
    await act(async () => { [...firstPanel.querySelectorAll('button')].find((button) => button.textContent === 'Continue')!.click(); });
    await view.settle();
    assert.equal(bodies.length, 1);

    await act(async () => {
      view.dom.window.history.pushState(null, '', '/craft/task-2');
      view.dom.window.dispatchEvent(new view.dom.window.PopStateEvent('popstate'));
    });
    await view.settle();
    const nextPrompt = view.container.querySelector('[data-testid="craft-prompt"]') as HTMLTextAreaElement;
    const setter = Object.getOwnPropertyDescriptor(view.dom.window.HTMLTextAreaElement.prototype, 'value')?.set;
    setter?.call(nextPrompt, 'Review this input');
    await act(async () => { nextPrompt.dispatchEvent(new view.dom.window.Event('input', { bubbles: true })); });
    await act(async () => { (view.container.querySelector('[data-testid="craft-send"]') as HTMLButtonElement).click(); });
    await view.settle();
    const secondPanel = view.container.querySelector('[aria-label="Unrecognized input materials"]');
    assert.ok(secondPanel, 'new Task prepares its own input and decision');
    assert.match(secondPanel.textContent ?? '', /opaque:\/\/input-2/);
    await act(async () => { [...secondPanel.querySelectorAll('button')].find((button) => button.textContent === 'Continue')!.click(); });
    await view.settle();

    assert.equal(bodies.length, 2);
    assert.notEqual(bodies[1]?.request_id, bodies[0]?.request_id, 'a distinct Task gets a distinct idempotency key');
    assert.deepEqual(bodies[1]?.input_refs, ['opaque://input-2']);
    assert.ok(view.calls.some((call) => call.path === '/api/v1/sessions/task-2/craft/runs'));
  } finally { await view.cleanup(); }
});
