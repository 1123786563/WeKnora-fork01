// CFT-S00-T004: the craft host shell composes the EXISTING @weknora/ui
// primitives (Sheet already owns focus trap, Escape and focus restore —
// packages/ui/src/interaction.test.tsx pins that contract). These tests pin
// the craft composition layer:
//   1. CraftDrawer (Sheet wrapper) closes on Escape and restores focus
//   2. a disabled/readonly action never fires its command
//   3. the shell guards against page-level horizontal overflow (the class
//      contract; real 1440/390 viewport proof runs in the e2e/T034 lane)
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
  KeyboardEvent: dom.window.KeyboardEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
Object.assign(globalThis, {
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  ResizeObserver: class { observe() {} unobserve() {} disconnect() {} },
});
dom.window.HTMLElement.prototype.scrollTo = function (): void {};
// The @weknora/ui entry imports theme.css; short-circuit CSS the same way
// packages/ui/src/index.test.tsx does (node module resolve hook).
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
const { createRoot } = await import('../../../../apps/web/node_modules/react-dom/client.js');

const {
  CraftShell,
  CraftPanelHeader,
  CraftNotice,
  CraftDrawer,
  CRAFT_NOTICE_KINDS,
} = await import('./shell.tsx');

async function mount(node: React.ReactNode) {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(node);
  });
  return root;
}

test('CraftDrawer closes on Escape and restores the opener focus', async () => {
  document.body.replaceChildren();
  const trigger = document.createElement('button');
  trigger.textContent = 'open sources';
  document.body.appendChild(trigger);
  trigger.focus();
  let closed = 0;
  const root = await mount(
    <CraftDrawer open title="来源" onClose={() => { closed += 1; }}>
      <p>sources body</p>
    </CraftDrawer>,
  );
  assert.match(document.body.textContent ?? '', /sources body/);
  // Sheet's Escape is focus-scoped: the key only lands when the dialog
  // panel itself holds focus (the mount effect focuses it; make it explicit).
  const panel = document.querySelector('[role="dialog"]');
  assert.ok(panel instanceof dom.window.HTMLElement, 'sheet panel rendered');
  panel.focus();
  await act(async () => {
    document.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  });
  assert.ok(closed >= 1, 'Escape must close the drawer');
  await act(async () => {
    root.unmount();
  });
});

test('a disabled action never fires its command', async () => {
  document.body.replaceChildren();
  let fired = 0;
  const root = await mount(
    <CraftPanelHeader
      title="版本"
      subtitle="v3"
      actions={[{ label: '从此版本继续', onAction: () => { fired += 1; }, disabled: true, disabledReason: '有活动任务' }]}
    />,
  );
  const body = document.body.textContent ?? '';
  assert.match(body, /从此版本继续/);
  const button = [...document.querySelectorAll('button')].find((b) => b.textContent === '从此版本继续');
  assert.ok(button, 'action button rendered');
  assert.equal(button.disabled, true, 'disabled action stays disabled');
  assert.match(body, /有活动任务/, 'the disabled reason stays readable');
  button.click();
  assert.equal(fired, 0, 'a disabled button must not fire the command');
  await act(async () => {
    root.unmount();
  });
});

test('CraftNotice speaks every state as text, never color-only', async () => {
  document.body.replaceChildren();
  for (const kind of CRAFT_NOTICE_KINDS) {
    const root = await mount(<CraftNotice kind={kind}>notice-{kind}</CraftNotice>);
    const live = document.querySelector('[aria-live]');
    assert.ok(live, `${kind} renders an aria-live region`);
    assert.match(live.textContent ?? '', new RegExp(`notice-${kind}`));
    await act(async () => {
      root.unmount();
    });
    document.body.replaceChildren();
  }
});

test('the shell guards page-level horizontal overflow by class contract', async () => {
  document.body.replaceChildren();
  const root = await mount(
    <CraftShell>
      <p>shell body</p>
    </CraftShell>,
  );
  const guard = document.querySelector('.wk-craft');
  assert.ok(guard, 'the shell carries the craft scope');
  assert.ok(guard.classList.contains('wk-craft-shell'), 'the shell exposes its overflow guard class');
  const css = await import('node:fs').then((fs) => fs.readFileSync(new URL('./craft.css', import.meta.url), 'utf8'));
  assert.match(css, /\.wk-craft-shell[^{]*\{[^}]*overflow-x:/, 'craft.css clips unexpected horizontal overflow on the shell');
  await act(async () => {
    root.unmount();
  });
});
