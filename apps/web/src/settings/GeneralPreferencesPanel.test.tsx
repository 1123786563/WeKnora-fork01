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

async function mountPanel(liteMode = false) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<GeneralPreferencesPanel liteMode={liteMode} />);
  });
  return container;
}

test('Lite mode exposes Vue auto-update switch and persists it without dropping other settings', async () => {
  dom.window.localStorage.setItem('WeKnora_settings', JSON.stringify({ selectedAgentId: 'agent-1', autoCheckUpdate: false }));
  const container = await mountPanel(true);
  const toggle = container.querySelector<HTMLButtonElement>('[role="switch"]');

  assert.ok(toggle, 'Lite GeneralSettings must expose the auto-update switch');
  assert.equal(toggle.getAttribute('aria-label'), '自动检查更新');
  assert.equal(toggle.getAttribute('aria-checked'), 'false');
  assert.ok(container.textContent?.includes('开启后自动检查并在后台下载最新版本安装包。'));

  await act(async () => toggle.click());

  assert.equal(toggle.getAttribute('aria-checked'), 'true');
  assert.deepEqual(JSON.parse(dom.window.localStorage.getItem('WeKnora_settings') ?? '{}'), {
    selectedAgentId: 'agent-1',
    autoCheckUpdate: true,
  });
});

test('non-Lite GeneralSettings does not expose the desktop-only auto-update control', async () => {
  const container = await mountPanel(false);
  assert.equal(container.querySelector('[role="switch"]'), null);
  assert.equal(container.textContent?.includes('自动检查更新'), false);
});

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
  // Pre-namespacing flat key: adopted into the active user's namespace by the
  // startup migration, exactly like Vue preferenceStorage (login required —
  // anon skips migration and keeps its own WeKnora_anon_* namespace).
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u1' }));
  dom.window.localStorage.setItem('weknora-font-size', 'large');
  const { migratePreferencesIntoUser, resetMigrationLatch } = await import('@weknora/domain/settings/local-preferences');
  resetMigrationLatch();
  migratePreferencesIntoUser(dom.window.localStorage);

  await mountPanel();

  assert.match(document.documentElement.style.getPropertyValue('--wk-font-sans'), /Georgia/);
  assert.match(document.documentElement.style.getPropertyValue('--wk-font-mono'), /Monaco/);
  assert.equal(document.documentElement.style.getPropertyValue('--wk-font-scale'), '1.125');
});

async function changeSelect(container: Element, index: number, value: string) {
  const select = container.querySelectorAll('select')[index]!;
  await act(async () => {
    select.value = value;
    select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
}

// Vue GeneralSettings.vue shows MessagePlugin.success(t('language.languageSaved'))
// after an accepted language change, with the message resolved in the NEW locale
// (locale.value is updated before the toast is created).
test('language change persists and shows the Vue success message in the new locale', async () => {
  const container = await mountPanel();

  await changeSelect(container, 0, 'en-US');

  assert.equal(dom.window.localStorage.getItem('locale'), 'en-US');
  assert.match(container.textContent ?? '', /Language settings saved/);
});

// Vue GeneralSettings.vue handlers (theme/font/size) show common.success after
// each accepted change; a rejected value rolls the form back silently.
test('theme change shows the Vue common.success feedback', async () => {
  const container = await mountPanel();

  await changeSelect(container, 1, 'dark');

  assert.equal(readLocalPreferences(dom.window.localStorage).theme, 'dark');
  assert.match(container.textContent ?? '', /成功/);
});

// The shared i18n tables carry every font option label (font.sans.* /
// font.mono.*) in all 5 locales, mirroring frontend/src/i18n/locales/*.ts.
// jsdom's UA resolves to the linux platform set; pin the localized labels so a
// locale that loses its font keys (falling back to the zh table) fails here.
test('font option labels follow the active locale (en-US Vue table)', async () => {
  dom.window.localStorage.setItem('locale', 'en-US');
  const container = await mountPanel();

  const sansOptions = Array.from(container.querySelectorAll('select')[2]!.options).map((option) => option.text);
  assert.equal(sansOptions.length, 4);
  assert.equal(sansOptions[0], 'System Default');
  assert.equal(sansOptions[sansOptions.length - 1], 'Generic Sans-Serif');
  const monoOptions = Array.from(container.querySelectorAll('select')[3]!.options).map((option) => option.text);
  assert.equal(monoOptions.length, 4);
  assert.equal(monoOptions[monoOptions.length - 1], 'Generic Monospace');
});

test('font option labels follow the active locale (ja-JP Vue table)', async () => {
  dom.window.localStorage.setItem('locale', 'ja-JP');
  const container = await mountPanel();

  const sansOptions = Array.from(container.querySelectorAll('select')[2]!.options).map((option) => option.text);
  assert.equal(sansOptions.length, 4);
  assert.equal(sansOptions[0], 'システムデフォルト');
  assert.equal(sansOptions[sansOptions.length - 1], '汎用サンセリフ');
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

// --- R441 A4: font selects read/write the per-user namespace (Vue
// preferenceStorage parity), not the pre-namespacing flat keys. ---

test('sans font change persists under WeKnora_{uid}_font_sans, never the flat key', async () => {
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u1' }));
  const container = await mountPanel();

  // jsdom resolves to the linux platform set; pick a visible sans key.
  await changeSelect(container, 2, 'noto-cjk');

  assert.equal(dom.window.localStorage.getItem('WeKnora_u1_font_sans'), 'noto-cjk');
  assert.equal(dom.window.localStorage.getItem('font_sans'), null);
});

test('mono font change persists under WeKnora_{uid}_font_mono, never the flat key', async () => {
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u1' }));
  const container = await mountPanel();

  await changeSelect(container, 3, 'dejavu-mono');

  assert.equal(dom.window.localStorage.getItem('WeKnora_u1_font_mono'), 'dejavu-mono');
  assert.equal(dom.window.localStorage.getItem('font_mono'), null);
});

test('panel adopts legacy flat font keys at mount and never reads them back for user u2', async () => {
  // u1 changed fonts while logged in; the flat keys are u1-era leftovers.
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u1' }));
  dom.window.localStorage.setItem('font_sans', 'georgia');
  dom.window.localStorage.setItem('font_mono', 'monaco');
  // Earlier tests in this process already consumed the module-level latch for
  // u1; reset it so this mount performs the adoption like a fresh login would.
  const { resetMigrationLatch } = await import('@weknora/domain/settings/local-preferences');
  resetMigrationLatch();
  await mountPanel();
  assert.equal(dom.window.localStorage.getItem('WeKnora_u1_font_sans'), 'georgia');
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();

  // u2 logs in on the same browser: u1's font choices must not surface, and
  // the flat leftovers must already have been consumed by the u1 migration.
  dom.window.localStorage.clear();
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u2' }));
  dom.window.localStorage.setItem('font_sans', 'georgia');
  dom.window.localStorage.setItem('font_mono', 'monaco');
  const second = await mountPanel();

  const selects = second.querySelectorAll('select');
  assert.equal((selects[2] as HTMLSelectElement).value, 'system', 'u2 must not inherit u1 font_sans');
  assert.equal((selects[3] as HTMLSelectElement).value, 'system', 'u2 must not inherit u1 font_mono');
});
