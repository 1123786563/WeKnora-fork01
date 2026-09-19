// R06 stop entrance contract (Task 6): the workbench offers a verifiable stop
// while a main run is ACTIVE (the same run-active gate the composer uses — an
// idle or finished workbench must not offer a stop), the zh label is 停止
// (STOP_STRINGS), and stopPhase='stopping' shows the busy label disabled.
//
// Harness copied from the adjacent workbench tests: the controller/messageLog
// fakes from workbench-layout.test.ts, the JSDOM interactive mount pattern
// (createRoot + act) from assistant-runtime.test.tsx/shell.test.tsx.
import assert from 'node:assert/strict';
import test from 'node:test';
import { JSDOM } from 'jsdom';
import React, { act } from 'react';
import type { CraftWorkbenchController } from '@weknora/core/craft/controller';
import type { CraftWorkbenchProps } from './workbench.tsx';

const dom = new JSDOM('<!doctype html><html><body></body></html>');
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  Event: dom.window.Event,
  KeyboardEvent: dom.window.KeyboardEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// Borrow the observer/animation primitives the assistant-ui viewport hooks
// need (same set as assistant-runtime.test.tsx).
Object.assign(globalThis, {
  MutationObserver: dom.window.MutationObserver,
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(() => cb(Date.now()), 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? ((handle: number) => clearTimeout(handle)),
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  scrollIntoView: () => {},
  ResizeObserver: class { observe(): void {} unobserve(): void {} disconnect(): void {} },
});
// The workbench's narrow-layout probe reads window.matchMedia — JSDOM ships
// none, so provide the standard inert stand-in (never matches).
(dom.window as unknown as { matchMedia: (query: string) => MediaQueryList }).matchMedia = (query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
}) as MediaQueryList;
dom.window.HTMLElement.prototype.scrollTo = function (): void {};
// CSS short-circuit (workbench.tsx imports craft.css; @weknora/ui imports
// theme.css) — the same node module resolve hook as shell.test.tsx.
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & {
  registerHooks?: (h: { resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown }) => void;
};
if (hooks.registerHooks) {
  hooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier.endsWith('.css')
        ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
        : nextResolve(specifier, context),
  });
}
// react-dom resolves through the web app's install (views stays renderer-free).
const { createRoot } = await import('../../../../apps/web/node_modules/react-dom/client.js');
const { CraftWorkbench } = await import('./workbench.tsx');
const { createCraftMessageLog } = await import('./presentation.ts');

// The controller state shape the W04 reducer publishes (domain CraftState).
interface FakeState {
  generation: number;
  runId: string | null;
  seq: number;
  mainStatus: string;
  delegationStatus: string;
  versionId: string | null;
}

const RUNNING_STATE: FakeState = {
  generation: 1,
  runId: 'run-stop-1',
  seq: 3,
  mainStatus: 'running',
  delegationStatus: 'running',
  versionId: null,
};
const SUCCEEDED_STATE: FakeState = { ...RUNNING_STATE, seq: 9, mainStatus: 'succeeded' };

function fakeController(initial: FakeState | null): CraftWorkbenchController & { setState(next: FakeState | null): void } {
  let current = initial;
  return {
    load: async () => {},
    submit: async () => {},
    reconnect: async () => {},
    dispose: () => {},
    state: () => current,
    lastError: () => null,
    onChange: () => () => {},
    setState(next: FakeState | null) { current = next; },
  };
}

function baseProps(controller: CraftWorkbenchController, overrides: Partial<CraftWorkbenchProps> = {}): CraftWorkbenchProps {
  return {
    locale: 'zh',
    sessionId: 'ses-stop',
    title: '停止契约',
    kind: 'web',
    canWrite: true,
    controller,
    messageLog: createCraftMessageLog(),
    initialPrompt: null,
    resumedRun: false,
    pendingAttachments: [],
    enrichedPending: false,
    onPickAttachments: () => {},
    onRemoveAttachment: () => {},
    onEnrichedSend: async () => {},
    versions: [],
    sessionUpdatedAt: '2026-09-19T00:00:00Z',
    snapshotVersionId: null,
    onRefreshVersions: () => {},
    onIssuePreview: async () => { throw new Error('no preview in stop test'); },
    onDownload: () => {},
    onInteractionAction: () => {},
    onMintTerminalUrl: async () => 'wss://example.invalid/terminal',
    onBack: () => {},
    syncError: null,
    ...overrides,
  };
}

async function mount(node: React.ReactNode) {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(node);
  });
  return root;
}

function buttonByText(text: string): HTMLButtonElement | undefined {
  return [...document.querySelectorAll('button')].find((b) => b.textContent === text) as HTMLButtonElement | undefined;
}

test('stop button appears while a run is active and calls onStopRun once', async () => {
  document.body.replaceChildren();
  let calls = 0;
  const root = await mount(
    <CraftWorkbench
      {...baseProps(fakeController(RUNNING_STATE))}
      onStopRun={async () => { calls += 1; }}
    />,
  );
  const button = buttonByText('停止');
  assert.ok(button, 'the stop button renders with the zh label 停止 while the main run is active');
  await act(async () => {
    button.click();
  });
  assert.equal(calls, 1, 'onStopRun fires exactly once per click');
  await act(async () => {
    root.unmount();
  });
});

test('stop button hidden when no run is active', async () => {
  document.body.replaceChildren();
  // A finished run (terminal main status) AND a workbench with no run at all
  // (controller state null → mainStatus 'idle') both hide the entrance.
  for (const state of [SUCCEEDED_STATE, null]) {
    const root = await mount(
      <CraftWorkbench
        {...baseProps(fakeController(state))}
        onStopRun={async () => {}}
      />,
    );
    assert.equal(buttonByText('停止'), undefined, `no stop button for mainStatus ${state === null ? 'idle (state null)' : state.mainStatus}`);
    await act(async () => {
      root.unmount();
    });
    document.body.replaceChildren();
  }
});

test("stopPhase='stopping' shows the busy label and disables the button", async () => {
  document.body.replaceChildren();
  let calls = 0;
  const root = await mount(
    <CraftWorkbench
      {...baseProps(fakeController(RUNNING_STATE), { stopPhase: 'stopping' })}
      onStopRun={async () => { calls += 1; }}
    />,
  );
  const button = buttonByText('停止中…');
  assert.ok(button, 'the busy label 停止中… renders while stopping');
  assert.equal(button.disabled, true, 'the stop button is disabled while stopping');
  await act(async () => {
    button.click();
  });
  assert.equal(calls, 0, 'a disabled stop button must not fire onStopRun');
  await act(async () => {
    root.unmount();
  });
});
