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

test('OIDC failure presents a visible localized toast', async () => {
  const client = fakeClient();
  (client.auth as Record<string, unknown>).oidcConfig = async () => ({ enabled: true });
  (client.auth as Record<string, unknown>).oidcUrl = async () => { throw new Error('OIDC unavailable'); };
  await mountLogin(client);
  const oidc = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').includes('OIDC')) as HTMLButtonElement;
  assert.ok(oidc, 'expected the OIDC button');
  await act(async () => { oidc.click(); await settle(10); });
  assert.match(document.querySelector('[data-testid="auth-toast"]')?.textContent ?? '', /OIDC unavailable/);
});

test('validation errors are associated with their inputs', async () => {
  await mountLogin(fakeClient());
  const submit = [...document.querySelectorAll('button')].find((node) => node.getAttribute('type') === 'submit') as HTMLButtonElement;
  await act(async () => { submit.click(); });
  const email = document.querySelector('input[autocomplete="email"]') as HTMLInputElement;
  const password = document.querySelector('input[autocomplete="current-password"]') as HTMLInputElement;
  assert.equal(email.getAttribute('aria-invalid'), 'true');
  assert.equal(email.getAttribute('aria-describedby'), 'auth-email-error');
  assert.equal(password.getAttribute('aria-describedby'), 'auth-password-error');
  assert.equal(document.querySelector('#auth-email-error')?.getAttribute('role'), 'alert');
});

test('language menu closes with Escape and exposes expanded state', async () => {
  await mountLogin(fakeClient());
  const trigger = document.querySelector('button[title="简体中文"]') as HTMLButtonElement;
  assert.equal(trigger.getAttribute('aria-expanded'), 'false');
  await act(async () => { trigger.click(); });
  assert.equal(trigger.getAttribute('aria-expanded'), 'true');
  await act(async () => { trigger.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); });
  assert.equal(document.querySelector('[role="menu"]'), null);
});

test('login card uses Vue box sizing and green outline for the create-account CTA', async () => {
  await mountLogin(fakeClient());
  const card = [...document.querySelectorAll('div')].find((node) => String(node.className).includes('bg-[rgba(255,255,255,0.97)]')) as HTMLDivElement | undefined;
  assert.match(card?.className ?? '', /\bbox-border\b/);
  const create = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').includes('创建账户')) as HTMLButtonElement | undefined;
  assert.match(create?.className ?? '', /border-\(--auth-brand\)/);
  assert.match(create?.className ?? '', /text-\(--auth-brand\)/);
});

test('login password control keeps Vue visibility toggle affordance', async () => {
  await mountLogin(fakeClient());
  const password = document.querySelector('input[autocomplete="current-password"]') as HTMLInputElement;
  const toggle = [...document.querySelectorAll('button')].find((node) => node.getAttribute('aria-label') === '密码') as HTMLButtonElement;
  assert.equal(password.type, 'password');
  assert.ok(toggle, 'expected the password visibility control');
  await act(async () => { toggle.click(); });
  assert.equal(password.type, 'text');
  await act(async () => { toggle.click(); });
  assert.equal(password.type, 'password');
});

test('login animated knowledge nodes preserve the Vue count and icon order', async () => {
  await mountLogin(fakeClient());
  const nodes = [...document.querySelectorAll('[aria-hidden="true"] > div')];
  assert.equal(nodes.length, 12, 'Vue renders twelve animated knowledge nodes');
  assert.ok(nodes[3]?.querySelector('ellipse'), 'Vue node 4 is the database icon');
  assert.equal(nodes[4]?.querySelector('circle')?.getAttribute('cx'), '11', 'Vue node 5 is the search icon');
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

test('login inputs keep the Vue inner-control geometry and transparent treatment', async () => {
  await mountLogin(fakeClient());
  const email = document.querySelector('input[autocomplete="email"]') as HTMLInputElement;
  assert.ok(email, 'expected the email input');
  assert.match(email.className, /\bh-6\b/);
  assert.match(email.className, /\brounded-none\b/);
  assert.match(email.className, /\bbg-transparent\b/);
  assert.match(email.className, /\bleading-6\b/);
  assert.equal(email.closest('.auth-input-shell')?.className.includes('h-10'), true);
  assert.equal(email.closest('.auth-input-shell')?.className.includes('px-3'), true);
  assert.match(email.className, /\bauth-input\b/);
});

test('login computed-style contract resets the native input padding and border', async () => {
  await mountLogin(fakeClient());
  const email = document.querySelector('input[autocomplete="email"]') as HTMLInputElement;
  assert.match(email.className, /\bp-0\b/);
  assert.ok(email.className.includes('text-[rgba(0,0,0,0.9)]'));
  assert.match(email.className, /\bauth-input\b/);
});

test('login heading and language control keep Vue computed typography', async () => {
  await mountLogin(fakeClient());
  const heading = document.querySelector('form[aria-label="Login form"]')?.parentElement?.querySelector('h2');
  assert.match(heading?.className ?? '', /leading-\[normal\]/);
  const language = document.querySelector('.language-switch > button');
  assert.match(language?.className ?? '', /\[font-family:var\(--auth-font\)\]/);
  assert.match(language?.className ?? '', /\bauth-language-control\b/);
  assert.equal(language?.querySelector('svg')?.className.baseVal ?? language?.querySelector('svg')?.getAttribute('class'), 'ml-0.5 shrink-0');
});

test('login page loads registration and OIDC config once without owning auto-setup', async () => {
  let registrationConfigCalls = 0;
  let oidcConfigCalls = 0;
  let autoSetupCalls = 0;
  const client = fakeClient();
  const auth = client.auth as Record<string, unknown>;
  auth.registrationConfig = async () => { registrationConfigCalls += 1; return { registrationMode: 'self_serve', complexPasswordEnabled: false }; };
  auth.oidcConfig = async () => { oidcConfigCalls += 1; return { enabled: false }; };
  auth.autoSetup = async () => { autoSetupCalls += 1; throw new Error('auto-setup belongs to bootstrap'); };
  await mountLogin(client);
  assert.equal(registrationConfigCalls, 1);
  assert.equal(oidcConfigCalls, 1);
  assert.equal(autoSetupCalls, 0);
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
