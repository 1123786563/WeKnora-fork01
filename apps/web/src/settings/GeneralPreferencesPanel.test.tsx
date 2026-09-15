import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
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
const { GeneralPreferencesPanel } = await import('./GeneralPreferencesPanel.tsx');
const { readLocalPreferences } = await import('@weknora/domain/settings/local-preferences');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.clear();
});

async function mountPanel() {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<GeneralPreferencesPanel />);
  });
  return container;
}

// Vue parity anchor: GeneralSettings.vue 字体大小 t-radio-group + the WeKnora
// global checked override (frontend/src/assets/theme/theme.css:120-141, solid
// brand bg + white text). The visual treatment lives in settings-wrapper.css
// (.wk-segmented); here we pin the interaction semantics the style hook rides on.
test('font size segmented group: selection moves aria-checked and persists', async () => {
  const container = await mountPanel();
  const radios = Array.from(container.querySelectorAll<HTMLButtonElement>('[role="radiogroup"] [role="radio"]'));
  assert.equal(radios.length, 3);

  // Vue default size is normal (useFont.ts) → middle button checked.
  assert.equal(radios[1]!.getAttribute('aria-checked'), 'true');
  assert.equal(radios[0]!.getAttribute('aria-checked'), 'false');
  assert.equal(radios[2]!.getAttribute('aria-checked'), 'false');
  assert.ok(radios[1]!.classList.contains('is-active'));

  await act(async () => {
    radios[2]!.click();
  });

  assert.equal(radios[2]!.getAttribute('aria-checked'), 'true');
  assert.equal(radios[1]!.getAttribute('aria-checked'), 'false');
  assert.ok(radios[2]!.classList.contains('is-active'));
  assert.ok(!radios[1]!.classList.contains('is-active'));

  // Persisted like Vue useFont.setFontSize (localStorage-backed) and the font
  // scale CSS variable applied immediately for live preview parity.
  assert.equal(readLocalPreferences(dom.window.localStorage).fontSize, 'large');
  assert.equal(document.documentElement.style.getPropertyValue('--wk-font-scale'), String(1.125));
});

test('general preferences keeps four labeled native selects (language/theme/mono/sans)', async () => {
  const container = await mountPanel();
  const selects = Array.from(container.querySelectorAll('select'));
  assert.equal(selects.length, 4);
  for (const select of selects) {
    assert.ok(select.getAttribute('aria-label'), 'select must keep an accessible name');
  }
});

test('applies persisted font preferences when the panel mounts', async () => {
  dom.window.localStorage.setItem('font_sans', 'georgia');
  dom.window.localStorage.setItem('font_mono', 'monaco');
  dom.window.localStorage.setItem('weknora-font-size', 'large');

  await mountPanel();

  assert.match(document.documentElement.style.getPropertyValue('--wk-font-sans'), /Georgia/);
  assert.match(document.documentElement.style.getPropertyValue('--wk-font-mono'), /Monaco/);
  assert.equal(document.documentElement.style.getPropertyValue('--wk-font-scale'), '1.125');
});

test('font previews use semantic surface and neutral border tokens', async () => {
  const container = await mountPanel();
  const previews = Array.from(container.querySelectorAll('[data-testid^="font-preview-"]'));

  assert.equal(previews.length, 2);
  for (const preview of previews) {
    assert.ok(preview.classList.contains('bg-surface'));
    assert.ok(preview.classList.contains('border-line-neutral'));
  }
});
