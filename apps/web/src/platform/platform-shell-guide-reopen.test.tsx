// Platform shell user-menu guide-reopen tests (jsdom + react-dom), following
// the harness in apps/web/src/platform/shell-session-list.test.tsx.
//
// Slice: R007 guides closeout. Vue parity baseline —
//   frontend/src/components/UserMenu.vue:45-50 (dropdown-guide-btn,
//   aria-label/tooltip $t('newUserGuide.reopen'), help-circle icon)
//   frontend/src/components/UserMenu.vue:501-504 (reopenGuide: close menu +
//   openNewUserGuide() — dispatches weknora:open-new-user-guide)
//   frontend/src/config/contextualGuides.ts:6-8 (openNewUserGuide dispatches
//   the event; NewUserGuide.vue opens on it even when
//   weknora:new-user-guide-done:v1 === '1', i.e. the replay overrides the
//   done-key gate — the key itself stays untouched until finish/skip).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// PlatformShell imports .css files; teach the ESM loader to treat them as
// empty modules (same approach as new-user-guide.test.tsx).
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.png')
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
// zh-CN like the acceptance environment; resolveLocale reads navigator.language.
// Shadow the jsdom navigator's language (prototype getter) before react-dom loads.
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default locale */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { PlatformShell } = await import('./PlatformShell.tsx');
const { formatMessage, supportedLocales } = await import('@weknora/i18n');
import type { Locale } from '@weknora/i18n';

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
  try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep last locale */ }
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

function fakeClient(role?: string): Record<string, unknown> {
  return {
    auth: { me: async () => ({ user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '' }, tenant: { id: 'tenant-1', name: 'Parity' }, memberships: role ? [{ tenant_id: 'tenant-1', role }] : [] }) },
    sessions: {
      list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }),
    },
  };
}

// The done-key is preset to '1': the tour counts as finished, so only the
// user-menu reopen entry may bring it back (auto-open must stay off).
async function mountShell(locale = 'zh-CN') {
  try { Object.defineProperty(jsdomNavigator, 'language', { value: locale, configurable: true }); } catch { /* keep default locale */ }
  window.localStorage.setItem('weknora:new-user-guide-done:v1', '1');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(
      PlatformShell,
      { client: fakeClient() as never, onLogout: () => undefined, children: React.createElement('div', null, 'page') },
    ));
  });
  await settle(20);
  return container;
}

const userButton = () => document.querySelector<HTMLButtonElement>('[data-guide="user-menu"]');
// The dropdown class became utilities; scope the user menu by the user
// button's wrapper and the reopen entry by its data-testid.
const dropdown = () => userButton()?.parentElement?.querySelector('[role="menu"]') ?? null;
const reopenItem = () => document.querySelector<HTMLButtonElement>('[data-testid="plat-shell-guide-reopen"]');
const guideOverlay = () => document.querySelector('[data-testid="wk-new-user-guide"]');
const guideTitle = () => guideOverlay()?.querySelector('.wk-guide__title')?.textContent ?? '';
const guideStepLabel = () => guideOverlay()?.querySelector('.wk-guide__step-label')?.textContent ?? '';

async function openUserMenu() {
  assert.ok(userButton(), 'user button (data-guide="user-menu") must exist');
  await act(async () => { userButton()!.click(); await new Promise((resolve) => setTimeout(resolve, 5)); });
  assert.ok(dropdown(), 'dropdown must open after clicking the user button');
}

test('admin user menu exposes the Vue management shortcuts for members, models and skills', async () => {
  window.localStorage.setItem('weknora:new-user-guide-done:v1', '1');
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(PlatformShell, {
      client: fakeClient('admin') as never,
      onLogout: () => undefined,
      children: React.createElement('div', null, 'page'),
    }));
  });
  await settle(20);
  await openUserMenu();
  const links = [...document.querySelectorAll<HTMLAnchorElement>('[role="menu"] a')];
  assert.deepEqual(links.map((link) => link.getAttribute('href')), [
    '/platform/settings?section=userprofile',
    '/platform/settings?section=tenant',
    '/platform/settings?section=members',
    '/platform/settings?section=models',
    '/platform/settings?section=skills',
  ]);
});

test('(a) with the tour finished, the user menu offers a reopen entry labelled newUserGuide.reopen', async () => {
  await mountShell();
  await openUserMenu();
  const item = reopenItem();
  assert.ok(item, 'reopen entry missing from the user menu');
  assert.equal(item.getAttribute('role'), 'menuitem');
  const expected = formatMessage('zh-CN', 'newUserGuide.reopen');
  assert.equal(expected, '新手引导', 'zh-CN Vue copy for newUserGuide.reopen');
  assert.equal(item.getAttribute('aria-label'), expected, 'aria-label mirrors the Vue $t(newUserGuide.reopen)');
  assert.equal(item.textContent, expected, 'visible label mirrors the same key');
  assert.ok(item.querySelector('svg'), 'help-circle icon present (Vue t-icon name="help-circle")');
});

test('(b) clicking the entry closes the menu and replays the tour even though the done-key is 1', async () => {
  await mountShell();
  await openUserMenu();
  await act(async () => { reopenItem()!.click(); await new Promise((resolve) => setTimeout(resolve, 10)); });
  assert.equal(dropdown(), null, 'menu closes like Vue reopenGuide (menuVisible = false)');
  assert.ok(guideOverlay(), 'tour reopens despite weknora:new-user-guide-done:v1 === "1"');
  assert.equal(guideTitle(), '欢迎使用 WeKnora', 'replay starts at the welcome step');
  assert.equal(guideStepLabel(), '1 / 7');
  // Vue parity: openNewUserGuide only dispatches the event — the stored key
  // is not rewritten on reopen (finish/skip write it again).
  assert.equal(window.localStorage.getItem('weknora:new-user-guide-done:v1'), '1');
});

test('(c) the entry label follows the shared bundle in all five locales', async () => {
  for (const locale of supportedLocales) {
    await mountShell(locale as Locale);
    await openUserMenu();
    const item = reopenItem();
    assert.ok(item, `reopen entry missing for locale ${locale}`);
    const expected = formatMessage(locale as Locale, 'newUserGuide.reopen');
    assert.ok(expected.length > 0 && expected !== 'newUserGuide.reopen', `shared key must resolve for ${locale}`);
    assert.equal(item.getAttribute('aria-label'), expected, `locale ${locale} label drift`);
    await act(async () => mountedRoot?.unmount());
    mountedRoot = undefined;
    document.body.replaceChildren();
    window.localStorage.clear();
  }
});
