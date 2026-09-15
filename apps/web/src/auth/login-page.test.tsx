// LoginPage toast-parity tests (jsdom + react-dom), following the harness in
// shell-sessions-header.test.tsx.
//
// Vue Login.vue presents auth outcomes with top-center MessagePlugin toasts
// (login failure: response.message || auth.loginError, Login.vue:695-697;
// register success: auth.registerSuccess, Login.vue:727/742) — never an
// inline banner inside the form card. The React page used an inline red
// banner; these tests pin the toast presentation.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : /\.(png|jpe?g|svg|gif|webp)$/.test(specifier)
  ? { shortCircuit: true, url: 'data:text/javascript,export default \"logo.png\"' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/login' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default locale */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { LoginPage } = await import('./LoginPage.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

function fakeClient(loginImpl?: () => Promise<unknown>, registerImpl?: () => Promise<unknown>): Record<string, unknown> {
  return {
    auth: {
      login: loginImpl ?? (async () => ({ user: {}, token: 't' })),
      register: registerImpl ?? (async () => ({ success: true })),
      registrationConfig: async () => ({ registration_mode: 'self_serve', complex_password: false }),
      oidcConfig: async () => ({ enabled: false }),
      lookupInvitation: async () => { throw new Error('no invitation'); },
    },
  };
}

async function mountLogin(client: Record<string, unknown>) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(LoginPage, { client: client as never, initialMode: 'login' }));
  });
  await settle(10);
  return container;
}

test('login failure presents the backend message as a top toast, not an inline banner', async () => {
  await mountLogin(fakeClient(async () => { throw new Error('Invalid email or password'); }));
  const email = document.querySelector('input[autocomplete="email"]') as HTMLInputElement;
  const password = document.querySelector('input[autocomplete="current-password"]') as HTMLInputElement;
  assert.ok(email && password, 'expected the login form fields');
  // React controlled inputs dedupe plain .value writes — go through the
  // native setter so onChange actually fires.
  const setNativeValue = (input: HTMLInputElement, value: string) => {
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!.set!;
    setter.call(input, value);
    input.dispatchEvent(new window.Event('input', { bubbles: true }));
  };
  await act(async () => {
    setNativeValue(email, 'parity-test@local.dev');
    setNativeValue(password, 'WrongPassword!');
  });
  const submit = [...document.querySelectorAll('button')].find((n) => n.getAttribute('type') === 'submit') as HTMLButtonElement;
  assert.ok(submit, 'expected the submit button');
  await act(async () => { submit.click(); await settle(30); });
  const toast = document.querySelector('[data-testid="auth-toast"]');
  assert.ok(toast, 'expected a top toast after login failure');
  assert.match(toast.textContent || '', /Invalid email or password/);
  assert.equal(document.querySelector('.form-alert'), null, 'no inline banner in the form card');
});

test('login card uses Vue box sizing and green outline for the create-account CTA', async () => {
  await mountLogin(fakeClient());
  const card = [...document.querySelectorAll('div')].find((node) => String(node.className).includes('bg-[rgba(255,255,255,0.97)]')) as HTMLDivElement | undefined;
  assert.match(card?.className ?? '', /\bbox-border\b/);
  const create = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').includes('创建账户')) as HTMLButtonElement | undefined;
  assert.match(create?.className ?? '', /border-\(--auth-brand\)/);
  assert.match(create?.className ?? '', /text-\(--auth-brand\)/);
});

test('register form keeps Vue required markers on every required field', async () => {
  await mountLogin(fakeClient());
  const create = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').includes('创建账户')) as HTMLButtonElement;
  await act(async () => { create.click(); });
  const labels = [...document.querySelectorAll('form[aria-label="Register form"] label > span:first-child')].map((node) => node.textContent?.trim());
  assert.deepEqual(labels.slice(0, 4), ['*用户名', '*邮箱', '*密码', '*确认密码']);
});

test('register heading and return action keep the Vue visual contract', async () => {
  await mountLogin(fakeClient());
  const create = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').includes('创建账户')) as HTMLButtonElement;
  await act(async () => { create.click(); });
  const heading = document.querySelector('form[aria-label="Register form"]')?.parentElement?.querySelector('h2');
  assert.match(heading?.className ?? '', /text-2xl/);
  assert.match(heading?.className ?? '', /font-semibold/);
  const back = [...document.querySelectorAll('a')].find((node) => (node.textContent ?? '').includes('返回登录'));
  assert.equal(back?.getAttribute('href'), '#');
  assert.match(back?.className ?? '', /font-medium/);
  assert.match(back?.className ?? '', /hover:underline/);
});

test('login heading and field labels use the Vue typography contract', async () => {
  await mountLogin(fakeClient());
  const heading = document.querySelector('form[aria-label="Login form"]')?.parentElement?.querySelector('h2');
  assert.match(heading?.className ?? '', /text-2xl/);
  assert.match(heading?.className ?? '', /font-semibold/);
  const labels = [...document.querySelectorAll('form[aria-label="Login form"] label > span:first-child')];
  assert.equal(labels.length, 2);
  for (const label of labels) assert.match(label.className, /text-sm/);
});

test('carousel keeps all Vue slides mounted for a fade transition', async () => {
  await mountLogin(fakeClient());
  const slides = [...document.querySelectorAll('img[alt]')].filter((node) =>
    node.closest('.rounded-2xl') && node.closest('.rounded-2xl')?.querySelector('button[aria-label]'),
  );
  assert.equal(slides.length, 4, 'expected all carousel slides to remain mounted');
  const slide = slides[0]?.parentElement;
  assert.match(slide?.className ?? '', /col-start-1/);
  assert.match(slide?.className ?? '', /row-start-1/);
  assert.match(slide?.className ?? '', /transition-opacity/);
  assert.match(slide?.className ?? '', /duration-\[800ms\]/);

  const next = document.querySelector('button[aria-label="混合检索策略"]') as HTMLButtonElement | null;
  assert.ok(next, 'expected a carousel pagination control');
  await act(async () => { next?.click(); });
  assert.match(slides[1]?.parentElement?.className ?? '', /opacity-100/);
  assert.match(slides[0]?.parentElement?.className ?? '', /opacity-0/);
});

test('language menu options are keyboard-operable and close after selection', async () => {
  await mountLogin(fakeClient());
  const trigger = document.querySelector('button[title="简体中文"]') as HTMLButtonElement | null;
  assert.ok(trigger, 'expected the language menu trigger');
  await act(async () => { trigger?.click(); });
  const english = [...document.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent?.includes('English')) as HTMLButtonElement | undefined;
  assert.ok(english, 'expected semantic language menu options');
  assert.equal(english?.tagName, 'BUTTON');
  await act(async () => { english?.click(); });
  assert.equal(document.querySelector('[role="menuitem"]'), null, 'menu should close after selection');
  assert.equal(document.querySelector('.language-switch > button')?.textContent?.includes('EN'), true);
});
