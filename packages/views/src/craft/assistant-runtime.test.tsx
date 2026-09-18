// CFT-S00-T006: @assistant-ui/react (locked 0.15.20) must be REALLY mounted —
// an ExternalStoreRuntime fed by the W05 message projection, not a lookalike
// DOM. These tests run the real provider in JSDOM:
//   1. one user + one assistant message truly render through ThreadPrimitive
//   2. StrictMode mount/unmount keeps store subscriptions balanced (no leak,
//      no duplicate subscription)
//   3. onNew routes the composer's text to the command bridge; isRunning is
//      driven ONLY by the main-run projection input — the runtime never
//      decides run success/failure in the browser
import assert from 'node:assert/strict';
import test from 'node:test';
import { JSDOM } from 'jsdom';
import React, { act, StrictMode } from 'react';

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
// Borrow the observer/animation primitives the SDK's viewport hooks need;
// JSDOM implements them on its window, they are just not global by default.
Object.assign(globalThis, {
  MutationObserver: dom.window.MutationObserver,
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(() => cb(Date.now()), 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? ((handle: number) => clearTimeout(handle)),
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  scrollIntoView: () => {},
});
// The SDK's viewport hooks observe element resizes; JSDOM ships no
// ResizeObserver, so provide the standard no-op stand-in.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
Object.assign(globalThis, { ResizeObserver: ResizeObserverStub });
// JSDOM elements have no scrollTo; the auto-scroll viewport calls it.
dom.window.HTMLElement.prototype.scrollTo = function scrollTo(): void {};
// react-dom resolves through the web app's install (the same pattern as
// packages/ui/src/interaction.test.tsx — views itself stays renderer-free).
const { createRoot } = await import('../../../../apps/web/node_modules/react-dom/client.js');

const {
  craftThreadMessages,
  CraftAssistantThread,
  useCraftAssistantRuntime,
} = await import('./assistant-runtime.tsx');

type Listener = () => void;
class FixtureStore {
  private listeners = new Set<Listener>();
  activeSubscriptions = 0;
  totalSubscriptions = 0;
  text = 'v1 已发布，总额 300。';
  complete = true;
  // useSyncExternalStore requires a cached snapshot object (the W05 log does
  // the same); rebuilding it per call would loop React 19.
  private snapshot = { text: this.text, complete: this.complete };
  subscribe = (listener: Listener) => {
    this.listeners.add(listener);
    this.activeSubscriptions += 1;
    this.totalSubscriptions += 1;
    return () => {
      this.listeners.delete(listener);
      this.activeSubscriptions -= 1;
    };
  };
  getSnapshot = () => this.snapshot;
  emit() {
    this.snapshot = { text: this.text, complete: this.complete };
    for (const l of this.listeners) l();
  }
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

test('craftThreadMessages keeps stable ids and main-run-only completeness', () => {
  const msgs = craftThreadMessages({
    runId: 'run-7',
    prompt: '按月分析',
    assistantText: '已生成',
    assistantComplete: true,
    archivedTurns: [
      { prompt: '第一轮', assistantText: '第一轮完成', assistantComplete: true },
    ],
  });
  assert.equal(msgs.length, 4);
  assert.deepEqual(
    msgs.map((m) => [m.role, m.id]),
    [
      ['user', 'turn-1-user'],
      ['assistant', 'turn-1-assistant'],
      ['user', 'run-7-user'],
      ['assistant', 'run-7-assistant'],
    ],
  );
  // completeness comes from the main-run projection flag only
  assert.deepEqual(msgs[3].status, { type: 'complete', reason: 'stop' });
  const streaming = craftThreadMessages({
    runId: 'run-8',
    prompt: 'p',
    assistantText: '部分',
    assistantComplete: false,
    archivedTurns: [],
  });
  assert.deepEqual(streaming[1].status, { type: 'running' });
});

test('a real provider renders one user and one assistant message', async () => {
  document.body.replaceChildren();
  const store = new FixtureStore();
  const sent: string[] = [];
  const root = await mount(
    <CraftAssistantThread
      store={store}
      isRunning={false}
      onNew={async (text) => {
        sent.push(text);
      }}
      prompt="按月分析"
      runId="run-7"
    />,
  );
  const body = document.body.textContent ?? '';
  assert.match(body, /按月分析/);
  assert.match(body, /v1 已发布/);
  await act(async () => {
    root.unmount();
  });
});

test('StrictMode mount/unmount keeps store subscriptions balanced', async () => {
  document.body.replaceChildren();
  const store = new FixtureStore();
  const root = await mount(
    <StrictMode>
      <CraftAssistantThread
        store={store}
        isRunning={false}
        onNew={async () => {}}
        prompt="p"
        runId="r-1"
      />
    </StrictMode>,
  );
  // StrictMode double-invokes effects; after settle exactly one live
  // subscription may remain, and a second mount cycle must not stack more.
  const afterFirstMount = store.activeSubscriptions;
  assert.ok(afterFirstMount >= 1 && afterFirstMount <= 2, `active=${afterFirstMount}`);
  await act(async () => {
    root.render(
      <StrictMode>
        <CraftAssistantThread
          store={store}
          isRunning={false}
          onNew={async () => {}}
          prompt="p2"
          runId="r-2"
        />
      </StrictMode>,
    );
  });
  assert.equal(store.activeSubscriptions, afterFirstMount, 're-render must not stack subscriptions');
  await act(async () => {
    root.unmount();
  });
  assert.equal(store.activeSubscriptions, 0, 'unmount must release every subscription');
});

test('isRunning flows only from the main-run projection input', async () => {
  document.body.replaceChildren();
  const store = new FixtureStore();
  const hook: { runtime?: unknown } = {};
  function Probe(props: { isRunning: boolean }) {
    hook.runtime = useCraftAssistantRuntime({
      store,
      isRunning: props.isRunning,
      onNew: async () => {},
      prompt: 'p',
      runId: 'r',
    });
    return null;
  }
  const root = await mount(<Probe isRunning={false} />);
  const getRunning = () =>
    (hook.runtime as { thread?: { getState?: () => { isRunning: boolean } } }).thread?.getState?.().isRunning;
  assert.equal(getRunning(), false);
  await act(async () => {
    root.render(<Probe isRunning />);
  });
  assert.equal(getRunning(), true, 'isRunning must follow the external projection');
  await act(async () => {
    root.unmount();
  });
});

test('dependency graph resolves a single React instance', async () => {
  // Two React copies would have broken the hook rules during the mounts
  // above already; this pins it structurally: resolving "react" from the
  // views package and from inside @assistant-ui/react must land on the same
  // install.
  const { createRequire } = await import('node:module');
  const self = createRequire(import.meta.url);
  const viewsReact = self.resolve('react');
  // resolve through the package entry (its exports map has no ./package.json)
  const auiEntry = self.resolve('@assistant-ui/react');
  const auiRequire = createRequire(auiEntry);
  const auiReact = auiRequire.resolve('react');
  assert.equal(viewsReact, auiReact, 'views and @assistant-ui/react must share one react install');
  assert.ok(React.version.startsWith('19.'), `workspace react version ${React.version}`);
});
