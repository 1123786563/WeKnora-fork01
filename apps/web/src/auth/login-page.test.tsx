// LoginPage toast-parity tests (jsdom + react-dom), following the harness in
// shell-sessions-header.test.tsx.
//
// Vue Login.vue presents auth outcomes with top-center MessagePlugin toasts
// (login failure: response.message || auth.loginError, Login.vue:695-697;
// register success: auth.registerSuccess, Login.vue:727/742) — never an
// inline banner inside the form card. These tests pin that presentation plus
// the TDesign 平移 DOM 契约（Login.vue 同构迁移，S4）。
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
  Node: dom.window.Node,
  MutationObserver: dom.window.MutationObserver,
  ResizeObserver: dom.window.ResizeObserver,
  Event: dom.window.Event,
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  IS_REACT_ACT_ENVIRONMENT: true,
});
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default locale */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { LoginPage } = await import('./LoginPage.tsx');
const { JoinPage } = await import('./JoinPage.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  // Language selection persists to localStorage; clear it so the next test
  // mounts with the default zh-CN locale.
  window.localStorage.clear();
  window.sessionStorage.clear();
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

/** tdesign-react Input 把未知属性（aria-*）透传到 .t-input__wrap 容器。 */
const inputWrap = (input: HTMLInputElement) => input.closest('.t-input__wrap') as HTMLElement;

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

// R445 A4: Vue Login.vue:636/647 — when the authorization-URL request fails
// without a backend message the fallback is auth.oidcLoginFailed, not the
// generic login-retry copy the React page used before this parity pass.
test('OIDC failure without a backend message falls back to the Vue auth.oidcLoginFailed copy', async () => {
  const client = fakeClient();
  (client.auth as Record<string, unknown>).oidcConfig = async () => ({ enabled: true });
  (client.auth as Record<string, unknown>).oidcUrl = async () => { throw 'network-down'; };
  await mountLogin(client);
  const oidc = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').includes('OIDC')) as HTMLButtonElement;
  assert.ok(oidc, 'expected the OIDC button');
  await act(async () => { oidc.click(); await settle(10); });
  assert.match(document.querySelector('[data-testid="auth-toast"]')?.textContent ?? '', /OIDC 登录失败/);
});

// R445 A4: Vue Login.vue:522-528 — selecting a language persists it and toasts
// language.languageSaved through MessagePlugin.success.
test('selecting a language toasts the Vue language.languageSaved confirmation', async () => {
  await mountLogin(fakeClient());
  const trigger = document.querySelector('.language-switch > button') as HTMLButtonElement;
  await act(async () => { trigger.click(); });
  const english = [...document.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent?.includes('English')) as HTMLElement;
  await act(async () => { english.click(); });
  const toast = document.querySelector('[data-testid="auth-toast"]');
  assert.ok(toast, 'expected a toast after switching language');
  assert.match(toast.textContent || '', /Language settings saved/);
});

test('validation errors are associated with their inputs', async () => {
  await mountLogin(fakeClient());
  const submit = [...document.querySelectorAll('button')].find((node) => node.getAttribute('type') === 'submit') as HTMLButtonElement;
  await act(async () => { submit.click(); });
  const email = document.querySelector('input[autocomplete="email"]') as HTMLInputElement;
  const password = document.querySelector('input[autocomplete="current-password"]') as HTMLInputElement;
  // tdesign-react Input 把 aria-* 放到 .t-input__wrap 容器（renderInput 固定
  // props，restProps 落容器——tdesign-react/es/input/Input.js:400-412）。
  assert.equal(inputWrap(email).getAttribute('aria-invalid'), 'true');
  assert.equal(inputWrap(email).getAttribute('aria-describedby'), 'auth-email-error');
  assert.equal(inputWrap(password).getAttribute('aria-describedby'), 'auth-password-error');
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

test('login card keeps the Vue form-card surface and create-account CTA classes', async () => {
  await mountLogin(fakeClient());
  const card = document.querySelector('.form-card') as HTMLDivElement | null;
  assert.ok(card, 'expected the .form-card surface (Vue Login.vue .form-card)');
  const create = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').includes('创建账户')) as HTMLButtonElement | undefined;
  assert.match(create?.className ?? '', /register-cta__button/);
  assert.match(create?.className ?? '', /t-button--variant-outline/);
});

test('login password control keeps the tdesign visibility toggle affordance', async () => {
  await mountLogin(fakeClient());
  const password = document.querySelector('input[autocomplete="current-password"]') as HTMLInputElement;
  const wrap = password.closest('.t-input') as HTMLElement;
  // tdesign Input 把 onClick 挂在 BrowseOff/Browse 图标 svg 上（Input.js:171-181），
  // suffix span 只是 renderIcon 的包装。
  const toggle = wrap.querySelector('.t-input__suffix-icon svg') as SVGElement | null;
  assert.equal(password.type, 'password');
  assert.ok(toggle, 'expected the tdesign browse-off visibility control');
  await act(async () => { toggle?.dispatchEvent(new window.MouseEvent('click', { bubbles: true })); });
  assert.equal(password.type, 'text');
  await act(async () => {
    wrap.querySelector('.t-input__suffix-icon svg')?.dispatchEvent(new window.MouseEvent('click', { bubbles: true }));
  });
  assert.equal(password.type, 'password');
});

test('login animated knowledge nodes preserve the Vue count and icon order', async () => {
  await mountLogin(fakeClient());
  const nodes = [...document.querySelectorAll('[aria-hidden="true"] > div')];
  assert.equal(nodes.length, 12, 'Vue renders twelve animated knowledge nodes');
  assert.ok(nodes[3]?.querySelector('ellipse'), 'Vue node 4 is the database icon');
  assert.equal(nodes[4]?.querySelector('circle')?.getAttribute('cx'), '11', 'Vue node 5 is the search icon');
});

test('invite registration uses the full Vue authentication shell', async () => {
  const previousPath = `${window.location.pathname}${window.location.search}`;
  window.history.replaceState({}, '', '/register?token=invite-token');
  const client = fakeClient();
  (client.auth as Record<string, unknown>).lookupInvitation = async () => ({ tenantId: 7, tenantName: '研发空间', role: 'member' });
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  try {
    await act(async () => { mountedRoot?.render(React.createElement(JoinPage, { client: client as never })); });
    await settle(20);
    // tdesign Form 不接受 aria-label（渲染固定 form 属性）；注册卡是页面上唯一
    // 的 form.t-form（登录/注册卡互斥渲染，Login.vue:179/260 同构）。
    assert.ok(document.querySelector('form.t-form'), 'invite registration should use the LoginPage register form');
    assert.equal(document.querySelector('main.wk-page'), null, 'invite registration should not use the legacy compact card');
  } finally {
    window.history.replaceState({}, '', previousPath);
  }
});

test('register form keeps Vue required markers on every required field', async () => {
  await mountLogin(fakeClient());
  const create = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').includes('创建账户')) as HTMLButtonElement;
  await act(async () => { create.click(); });
  const labels = [...document.querySelectorAll('form.t-form .t-form__label')].map((node) => ({
    required: node.classList.contains('t-form__label--required'),
    text: node.querySelector('label')?.textContent?.trim(),
  }));
  assert.equal(labels.length, 4);
  assert.deepEqual(labels.map((l) => [l.required, l.text]), [
    [true, '用户名'], [true, '邮箱'], [true, '密码'], [true, '确认密码'],
  ]);
});

test('register heading and return action keep the Vue visual contract', async () => {
  await mountLogin(fakeClient());
  const create = [...document.querySelectorAll('button')].find((node) => (node.textContent ?? '').includes('创建账户')) as HTMLButtonElement;
  await act(async () => { create.click(); });
  const heading = document.querySelector('form.t-form')?.closest('.form-card')?.querySelector('h2');
  assert.match(heading?.className ?? '', /form-title/);
  const back = [...document.querySelectorAll('a')].find((node) => (node.textContent ?? '').includes('返回登录'));
  assert.equal(back?.getAttribute('href'), '#');
  assert.match(back?.className ?? '', /link-button/);
});

test('login heading and field labels use the Vue typography contract', async () => {
  await mountLogin(fakeClient());
  const heading = document.querySelector('form.t-form')?.closest('.form-card')?.querySelector('h2');
  assert.match(heading?.className ?? '', /form-title/);
  const labels = [...document.querySelectorAll('form.t-form .t-form__label')];
  assert.equal(labels.length, 2);
  for (const label of labels) {
    assert.match(label.className, /t-form__label--top/);
    assert.match(label.className, /t-form__label--required/);
  }
});

test('login inputs keep the Vue t-input structure (wrap > t-size-l shell > inner)', async () => {
  await mountLogin(fakeClient());
  const email = document.querySelector('input[autocomplete="email"]') as HTMLInputElement;
  assert.ok(email, 'expected the email input');
  assert.match(email.className, /t-input__inner/);
  assert.match((email.parentElement as HTMLElement)?.className ?? '', /t-input\b/);
  assert.match((email.parentElement as HTMLElement)?.className ?? '', /t-size-l/);
  assert.ok(email.closest('.t-input__wrap'), 'expected the .t-input__wrap container');
  // t-form-item 结构：label + controls（Vue Login.vue t-form/t-form-item 同构）
  assert.ok(email.closest('.t-form__item')?.classList.contains('t-form-item__email'), 'expected the named form item');
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

test('carousel keeps all Vue slides mounted as a swiper fade stack', async () => {
  await mountLogin(fakeClient());
  const slides = [...document.querySelectorAll('.screenshot-swiper .swiper-slide')];
  assert.equal(slides.length, 4, 'expected all carousel slides to remain mounted');
  for (const slide of slides) assert.ok(slide.querySelector('img.slide-image'), 'each slide wraps the Vue .slide-image');
  const bullets = [...document.querySelectorAll('.screenshot-swiper .swiper-pagination-bullet')];
  assert.equal(bullets.length, 4, 'expected four swiper pagination bullets');

  const next = document.querySelector('.screenshot-swiper [aria-label="混合检索策略"]') as HTMLElement | null;
  assert.ok(next, 'expected a carousel pagination control');
  await act(async () => { next?.dispatchEvent(new window.MouseEvent('click', { bubbles: true })); });
  assert.match(slides[1]?.getAttribute('style') ?? '', /opacity:\s*1/);
  assert.match(slides[0]?.getAttribute('style') ?? '', /opacity:\s*0/);
  assert.ok(bullets[1]?.classList.contains('swiper-pagination-bullet-active'));
});

test('language menu options are keyboard-operable and close after selection', async () => {
  await mountLogin(fakeClient());
  const trigger = document.querySelector('button[title="简体中文"]') as HTMLButtonElement | null;
  assert.ok(trigger, 'expected the language menu trigger');
  await act(async () => { trigger?.click(); });
  const english = [...document.querySelectorAll('[role="menuitem"]')].find((node) => node.textContent?.includes('English')) as HTMLElement | undefined;
  assert.ok(english, 'expected semantic language menu options');
  await act(async () => { english?.click(); });
  assert.equal(document.querySelector('[role="menuitem"]'), null, 'menu should close after selection');
  assert.equal(document.querySelector('.language-switch > button')?.textContent?.includes('EN'), true);
});
