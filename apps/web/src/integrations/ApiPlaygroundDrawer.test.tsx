import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// Interactive render harness (same pattern as imWizardRender.test.tsx).
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
  MouseEvent: dom.window.MouseEvent,
  KeyboardEvent: dom.window.KeyboardEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { formatMessage } = await import('../../../../packages/i18n/src/index.ts');
const { ApiPlaygroundDrawer, clampApiPlaygroundWidth } = await import('./ApiPlaygroundDrawer.tsx');
type ApiPlaygroundAgentOption = import('./ApiPlaygroundDrawer.tsx').ApiPlaygroundAgentOption;

const t = (key: string, values?: Record<string, string | number>) => formatMessage('zh-CN', key, values);
const encoder = new TextEncoder();

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

type FetchCall = { url: string; init: RequestInit };

function sseStream(chunks: string[], keepOpen = false) {
  return new ReadableStream<Uint8Array>({
    pull(controller) {
      if (chunks.length) controller.enqueue(encoder.encode(chunks.shift()!));
      else if (!keepOpen) controller.close();
    },
  });
}

/** Streams one frame then rejects the pending read when the signal aborts. */
function abortableSseStream(signal: AbortSignal) {
  return new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(encoder.encode('data: {"response_type":"answer","content":"部分","done":false}\n\n'));
      signal.addEventListener('abort', () => controller.error(signal.reason), { once: true });
    },
  });
}

function jsonResponse(payload: unknown, status = 200) {
  return { ok: status >= 200 && status < 300, status, text: async () => JSON.stringify(payload), body: null };
}

function mountDrawer(overrides: Record<string, unknown> = {}, onFetch?: (url: string, init: RequestInit) => unknown) {
  const calls: FetchCall[] = [];
  const fetchFn = async (url: string | URL, init: RequestInit = {}) => {
    calls.push({ url: String(url), init });
    if (!onFetch) throw new Error('unexpected fetch: ' + String(url));
    return onFetch(String(url), init);
  };
  const props = {
    open: true,
    onClose: () => undefined,
    apiKey: 'wk-key',
    mode: 'direct_header' as const,
    agents: [{ id: 'a1', name: '助手' }] as readonly ApiPlaygroundAgentOption[],
    apiBaseUrl: 'https://weknora.test',
    fetchFn: fetchFn as typeof fetch,
    t,
    ...overrides,
  };
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  void act(() => { mountedRoot?.render(React.createElement(ApiPlaygroundDrawer, props as never)); });
  // Vue SettingDrawer teleports to body; query the host body so the suite
  // remains valid when the React drawer uses the same portal boundary.
  return { container: document.body, calls };
}

async function waitFor(predicate: () => boolean, message = 'waitFor timeout') {
  const start = Date.now();
  for (;;) {
    if (predicate()) return;
    if (Date.now() - start > 2000) throw new Error(message);
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
  }
}

function runButton(container: HTMLElement) {
  return container.querySelector<HTMLButtonElement>('button[data-action="run"]');
}

// Vue parity: ApiIntegrationSettings.vue L339-460 — SettingDrawer with request
// config, masked request preview and the three result steps.

test('drawer renders the three Vue sections with Vue defaults in zh-CN', () => {
  const { container } = mountDrawer();
  const drawer = container.querySelector('aside[role="dialog"]');
  assert.ok(drawer, 'drawer aside rendered');
  assert.equal(drawer!.parentElement?.parentElement, document.body, 'Vue SettingDrawer teleport boundary is document.body');
  assert.equal(drawer!.getAttribute('aria-label'), 'API Playground');
  const titles = Array.from(container.querySelectorAll('h4')).map((node) => node.textContent);
  assert.deepEqual(titles, ['请求配置', '请求预览', '运行结果']);
  const query = container.querySelector<HTMLTextAreaElement>('.wk-api-playground-query');
  assert.equal(query!.value, 'hello', 'Vue default query');
  assert.equal(query!.rows, 2, 'Vue textarea autosize starts at minRows=2');
  const externalUser = container.querySelector<HTMLInputElement>('.wk-api-playground-external-user');
  assert.equal(externalUser!.value, 'user_123', 'Vue default external user');
  assert.equal(container.querySelector('.wk-api-playground-empty')?.textContent, '运行后将在这里显示 Session 响应、SSE 原始输出和提取出的回答。');
  assert.match(container.querySelector('pre')?.textContent ?? '', /<API_KEY>/);
  assert.ok(!Array.from(container.querySelectorAll('pre')).some((pre) => (pre.textContent ?? '').includes('wk-key')), 'preview masks the api key');
});

test('close button aborts and hands control back to the host', () => {
  let closed = false;
  const { container } = mountDrawer({ onClose: () => { closed = true; } });
  const close = container.querySelector<HTMLButtonElement>('.wk-api-playground-close');
  assert.ok(close, 'close affordance rendered');
  close!.focus();
  assert.equal(document.activeElement, close, 'close affordance can receive focus');
  void act(() => { close!.click(); });
  assert.equal(closed, true, 'close button closes the drawer');
  assert.notEqual(document.activeElement, close, 'Vue SettingDrawer blurs the active control before destroy-on-close');
});

test('run gating mirrors Vue: disabled with a reason and tenant mode locks the external user', () => {
  const { container } = mountDrawer({ apiKey: '', mode: 'tenant' });
  const run = runButton(container);
  assert.equal(run!.disabled, true, 'run disabled without an api key');
  assert.equal(container.querySelector('.wk-api-playground-disabled-reason')?.textContent, '当前空间没有 API Key。');
  const externalUser = container.querySelector<HTMLInputElement>('.wk-api-playground-external-user');
  assert.equal(externalUser!.disabled, true, 'Vue disables the external user input in tenant mode');
  assert.match(container.querySelector('.wk-api-playground-hint')?.textContent ?? '', /仅空间模式不会发送外部用户身份/);
});

test('agent selector exposes the Vue loading state while agents are resolving', () => {
  const { container } = mountDrawer({ agents: [], agentsLoading: true });
  const select = container.querySelector<HTMLInputElement>('[role="combobox"]');
  assert.equal(select?.getAttribute('aria-busy'), 'true');
  assert.equal(select?.placeholder, '加载中...');
});

test('signed_token preview masks both secrets behind the Vue placeholders', () => {
  const { container } = mountDrawer({ mode: 'signed_token' });
  const preview = container.querySelector('pre')?.textContent ?? '';
  assert.match(preview, /<JWT>/);
  assert.match(preview, /<API_KEY>/);
  assert.ok(!preview.includes('wk-key'));
});

test('full direct_header run streams the SSE answer with per-step statuses', async () => {
  const { container, calls } = mountDrawer({}, (url) => {
    if (url.endsWith('/api/v1/sessions')) return jsonResponse({ success: true, data: { id: 'sess-1' } });
    if (url.includes('/api/v1/agent-chat/')) {
      return { ok: true, status: 200, text: async () => '', body: sseStream([
        'data: {"response_type":"answer","content":"你好","done":false}\n\n',
        'data: {"response_type":"complete","content":"","done":true}\n\n',
      ]) };
    }
    throw new Error('unexpected url ' + url);
  });
  void act(() => { runButton(container)!.click(); });
  await waitFor(() => (container.querySelector('.wk-status')?.textContent ?? '').includes('测试完成'), 'run never finished');
  assert.equal(calls.length, 2, 'session then chat requests');
  const [session, chat] = calls as [FetchCall, FetchCall];
  assert.ok(session.url.endsWith('/api/v1/sessions'), 'Vue posts to /api/v1/sessions');
  assert.equal((session.init.body as string), '{}');
  const sessionHeaders = session.init.headers as Record<string, string>;
  assert.equal(sessionHeaders['X-API-Key'], 'wk-key');
  assert.equal(sessionHeaders['X-External-User-ID'], 'user_123');
  assert.equal(sessionHeaders.Accept, 'application/json');
  assert.ok(chat.url.endsWith('/api/v1/agent-chat/sess-1'), 'chat uses the created session id');
  const chatHeaders = chat.init.headers as Record<string, string>;
  assert.equal(chatHeaders.Accept, 'text/event-stream');
  assert.deepEqual(JSON.parse(chat.init.body as string), { query: 'hello', agent_enabled: true, agent_id: 'a1', channel: 'api' });
  const steps = Array.from(container.querySelectorAll('[data-step]'));
  assert.equal(steps.length, 3, 'session, chat and answer steps');
  const sessionStep = container.querySelector('[data-step="session"]')!;
  assert.equal(sessionStep.querySelector('[data-status]')!.getAttribute('data-status'), 'success');
  assert.match(sessionStep.querySelector('pre')!.textContent ?? '', /sess-1/);
  const chatStep = container.querySelector('[data-step="chat"]')!;
  assert.equal(chatStep.querySelector('[data-status]')!.getAttribute('data-status'), 'success');
  assert.match(chatStep.querySelector('pre')!.textContent ?? '', /"response_type":\s*"complete"/);
  assert.equal(container.querySelector('[data-step="answer"] pre')!.textContent, '你好');
});

test('stop during the SSE stream marks the steps stopped with the Vue message', async () => {
  const { container } = mountDrawer({}, (url, init) => {
    if (url.endsWith('/api/v1/sessions')) return jsonResponse({ success: true, data: { id: 'sess-1' } });
    return { ok: true, status: 200, text: async () => '', body: abortableSseStream(init.signal as AbortSignal) };
  });
  void act(() => { runButton(container)!.click(); });
  await waitFor(() => (container.querySelector('[data-step="chat"] pre')?.textContent ?? '').includes('部分'), 'stream never produced output');
  const stop = container.querySelector<HTMLButtonElement>('button[data-action="stop"]');
  assert.ok(stop, 'stop button visible while running');
  void act(() => { stop!.click(); });
  await waitFor(() => (container.querySelector('[role="alert"]')?.textContent ?? '') === '测试已停止', 'stop never settled');
  // Vue L1719-1720 only flips steps that are still 'running': the session step
  // already settled as success before the chat stream started.
  assert.equal(container.querySelector('[data-step="session"] [data-status]')!.getAttribute('data-status'), 'success');
  assert.equal(container.querySelector('[data-step="chat"] [data-status]')!.getAttribute('data-status'), 'stopped');
  await waitFor(() => !runButton(container)!.disabled, 'run re-enabled after stop');
});

test('terminal SSE error event fails the chat step with the provider message', async () => {
  const { container } = mountDrawer({}, (url) => {
    if (url.endsWith('/api/v1/sessions')) return jsonResponse({ success: true, data: { id: 'sess-1' } });
    return { ok: true, status: 200, text: async () => '', body: sseStream([
      'data: {"response_type":"error","content":"上游模型故障","done":true}\n\n',
    ], true) };
  });
  void act(() => { runButton(container)!.click(); });
  await waitFor(() => (container.querySelector('[role="alert"]')?.textContent ?? '') === '上游模型故障', 'terminal error never surfaced');
  assert.equal(container.querySelector('[data-step="chat"] [data-status]')!.getAttribute('data-status'), 'failed');
  assert.equal(runButton(container)!.disabled, false, 'run re-enabled after failure');
});

test('signed_token run mints the test token and shows the generated token step', async () => {
  const minted: string[] = [];
  const { container, calls } = mountDrawer({
    mode: 'signed_token',
    mintToken: async (externalUserId: string) => { minted.push(externalUserId); return { token: 'jwt-abc', headerName: 'X-External-User-Token' }; },
  }, (url) => {
    if (url.endsWith('/api/v1/sessions')) return jsonResponse({ success: true, data: { id: 'sess-1' } });
    return { ok: true, status: 200, text: async () => '', body: sseStream([
      'data: {"response_type":"answer","content":"ok","done":false}\n\n',
      'data: [DONE]\n\n',
    ]) };
  });
  void act(() => { runButton(container)!.click(); });
  await waitFor(() => (container.querySelector('.wk-status')?.textContent ?? '').includes('测试完成'), 'run never finished');
  assert.deepEqual(minted, ['user_123'], 'mint uses the trimmed external user');
  const sessionHeaders = (calls[0] as FetchCall).init.headers as Record<string, string>;
  assert.equal(sessionHeaders['X-External-User-Token'], 'jwt-abc', 'minted token rides the signed-token header');
  const tokenStep = container.querySelector('[data-step="token"]');
  assert.ok(tokenStep, 'generated token step rendered');
  assert.match(tokenStep!.querySelector('div span')!.textContent ?? '', /本次生成的测试 Token/);
  assert.equal(tokenStep!.querySelector('pre')!.textContent, 'jwt-abc');
});

test('agent select defaults to the builtin smart-reasoning agent and lists the builtin suffix', async () => {
  const { container } = mountDrawer({
    agents: [{ id: 'builtin-smart-reasoning', name: '深度推理', is_builtin: true }, { id: 'a1', name: '助手' }] as readonly ApiPlaygroundAgentOption[],
  });
  const select = container.querySelector<HTMLInputElement>('[role="combobox"]');
  assert.equal(select!.value, '深度推理 · 内置', 'Vue ensurePlaygroundAgent prefers the builtin');
  await act(async () => { select!.focus(); });
  const options = Array.from(container.querySelectorAll('[role="option"]')).map((option) => option.textContent);
  assert.ok(options.includes('深度推理 · 内置'), 'Vue agentOptionLabel builtin suffix');
  assert.ok(options.includes('助手'));
});

test('agent selector filters options, selects with Enter, and closes on Escape', async () => {
  const { container } = mountDrawer({
    agents: [{ id: 'a1', name: 'Alpha' }, { id: 'a2', name: 'Beta' }] as readonly ApiPlaygroundAgentOption[],
  });
  const input = container.querySelector<HTMLInputElement>('[role="combobox"]')!;
  await act(async () => {
    input.focus();
    Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set?.call(input, 'bet');
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
  assert.deepEqual(Array.from(container.querySelectorAll('[role="option"]')).map((option) => option.textContent), ['Beta']);
  await act(async () => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); });
  assert.equal(input.value, 'Beta');
  await act(async () => {
    input.focus();
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  });
  assert.equal(input.getAttribute('aria-expanded'), 'false');
});

test('drawer width follows Vue clamp rules and persists a dragged width', async () => {
  assert.equal(clampApiPlaygroundWidth(400, 1400), 560);
  assert.equal(clampApiPlaygroundWidth(1200, 1400), 960);
  assert.equal(clampApiPlaygroundWidth(900, 700), 700);
  window.localStorage.clear();
  const { container } = mountDrawer();
  const handle = container.querySelector<HTMLElement>('[role="separator"]');
  assert.ok(handle, 'resizable drawer exposes the Vue separator handle');
  assert.equal(handle!.style.right, '640px');
  await act(async () => { handle!.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, clientX: 1000 })); });
  await act(async () => { document.dispatchEvent(new MouseEvent('mousemove', { bubbles: true, clientX: 700 })); });
  await act(async () => { document.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: 700 })); });
  assert.equal(handle!.style.right, '940px');
  assert.equal(window.localStorage.getItem('setting-drawer:width:api-playground'), '940');
});
