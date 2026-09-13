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
  const submit = [...document.querySelectorAll('button')].find((n) => n.className.includes('submit-button')) as HTMLButtonElement;
  assert.ok(submit, 'expected the submit button');
  await act(async () => { submit.click(); await settle(30); });
  const toast = document.querySelector('.auth-toast');
  assert.ok(toast, 'expected a top toast after login failure');
  assert.match(toast.textContent || '', /Invalid email or password/);
  assert.equal(document.querySelector('.form-alert'), null, 'no inline banner in the form card');
});
