// Welcome-tour component behavior tests (jsdom + react-dom), following the
// established harness in apps/web/src/settings/ModelSettingsPanel.test.tsx.
// These live under apps/web because only this package resolves react-dom
// (same convention documented in packages/views/src/chat/tool-approval.test.tsx);
// the component itself lives in packages/views/src/guides/.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// NewUserGuide.tsx imports a .css file; teach the ESM loader to treat it as
// an empty module (same approach as global-command-palette-render.test.ts).
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { NewUserGuide } = await import('../../../../packages/views/src/guides/NewUserGuide.tsx');
const { GLOBAL_USER_GUIDE_KEY, OPEN_NEW_USER_GUIDE_EVENT } = await import('../../../../packages/views/src/guides/new-user-guide.ts');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

const GUIDE_PROPS = { locale: 'zh-CN', autoOpenDelayMs: 0, beforeDelayMs: 0, locateRetryDelayMs: 1 } as const;

async function mountGuide(extraProps: Record<string, unknown> = {}) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(NewUserGuide, { ...GUIDE_PROPS, ...extraProps }));
  });
  await settle(10);
  return container;
}

const overlay = () => document.querySelector('[data-testid="wk-new-user-guide"]');
const card = () => document.querySelector('[data-testid="wk-new-user-guide-card"]');
const stepTitle = () => card()?.querySelector('.wk-guide__title')?.textContent ?? '';
const stepLabel = () => card()?.querySelector('.wk-guide__step-label')?.textContent ?? '';
function buttonByText(root: Element, text: string): HTMLButtonElement | null {
  for (const button of root.querySelectorAll('button')) {
    if (button.textContent === text) return button as HTMLButtonElement;
  }
  return null;
}
async function click(button: HTMLButtonElement) {
  await act(async () => {
    button.click();
    await new Promise((resolve) => setTimeout(resolve, 5));
  });
}

test('(a) no modal renders once the localStorage done-key is set to 1', async () => {
  window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  await mountGuide();
  await settle(30);
  assert.equal(overlay(), null, 'guide overlay must not exist when the done-key is 1');
});

test('(b) with the key absent the guide auto-opens on step 1 with the Vue copy and 7 dots', async () => {
  await mountGuide();
  assert.ok(overlay(), 'expected the guide overlay');
  assert.match(overlay()?.getAttribute('role') ?? '', /dialog/);
  assert.equal(stepTitle(), '欢迎使用 WeKnora');
  assert.equal(stepLabel(), '1 / 7');
  assert.equal(card()?.querySelectorAll('.wk-guide__dot').length, 7);
  assert.ok(card()?.querySelector('.wk-guide__dot.is-active'));
  assert.equal(card()?.querySelectorAll('.wk-guide__btn').length, 1, 'step 1 shows next only');
  assert.ok(buttonByText(card()!, '下一步'), 'next button present');
  assert.ok(buttonByText(card()!, '跳过引导'), 'skip button present');
  assert.ok(card()?.className.includes('wk-guide__card--center'), 'target-less welcome step centers the card');
});

test('(c) 下一步 advances to step 2 (knowledge) with its title and counter', async () => {
  await mountGuide();
  const next = buttonByText(card()!, '下一步');
  assert.ok(next);
  await click(next!);
  // knowledge targets [data-guide="nav-knowledge-bases"] which does not exist
  // in the bare DOM, so the locator exhausts its retries and centers the card.
  await settle(60);
  assert.equal(stepTitle(), '创建你的知识库');
  assert.equal(stepLabel(), '2 / 7');
});

test('(d) 跳过引导 writes the key as 1 and unmounts the overlay', async () => {
  await mountGuide();
  const skip = buttonByText(card()!, '跳过引导');
  assert.ok(skip);
  await click(skip!);
  assert.equal(window.localStorage.getItem(GLOBAL_USER_GUIDE_KEY), '1');
  assert.equal(overlay(), null, 'overlay must unmount after skip');
});

test('(e) reaching the last step and pressing 完成 writes the key as 1', async () => {
  await mountGuide();
  // Walk forward until the 完成 button appears; optional steps auto-skip
  // because their targets do not exist in this bare DOM.
  let finalLabel = '';
  for (let guard = 0; guard < 24; guard += 1) {
    const done = buttonByText(card()!, '完成');
    if (done) {
      finalLabel = stepLabel();
      await click(done!);
      break;
    }
    const next = buttonByText(card()!, '下一步');
    if (!next) throw new Error('tour stuck without a next or done button');
    await click(next!);
    await settle(40);
  }
  assert.equal(finalLabel, '7 / 7');
  assert.equal(window.localStorage.getItem(GLOBAL_USER_GUIDE_KEY), '1');
  assert.equal(overlay(), null, 'overlay must unmount after completion');
});

test('(f) the weknora:open-new-user-guide event reopens a finished tour', async () => {
  window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  await mountGuide();
  assert.equal(overlay(), null);
  await act(async () => {
    window.dispatchEvent(new window.CustomEvent(OPEN_NEW_USER_GUIDE_EVENT));
    await new Promise((resolve) => setTimeout(resolve, 5));
  });
  await settle(20);
  assert.ok(overlay(), 'manual open event must show the tour even when done');
  assert.equal(stepTitle(), '欢迎使用 WeKnora');
});
