// FAQ B4 search test drawer: the toolbar 检索测试 button opens the 420px drawer,
// the close button closes it, and a real search round-trip renders ranked hits
// with the Vue three-state coverage (FAQEntryManager.vue:734-853, 2650-2680).
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => /\.(css|png|jpe?g|svg|gif|webp)$/.test(specifier)
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases/kb-1/faq' });
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
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { FAQPage } = await import('./FAQPage.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

function fakeClient(searchData: Array<Record<string, unknown>>, options: { searchError?: Error } = {}) {
  const searchCalls: Array<Record<string, unknown>> = [];
  return {
    client: {
      knowledgeBases: {
        settings: { get: async () => ({ id: 'kb-1', name: 'KB One', type: 'faq' }) },
        list: async () => [],
      },
      knowledge: {
        documents: { tags: async () => [] },
        faq: {
          list: async () => ({ data: [], total: 0, page: 1, page_size: 20 }),
          search: async (_kbId: string, input: Record<string, unknown>) => {
            searchCalls.push(input);
            if (options.searchError) throw options.searchError;
            return { success: true, data: searchData };
          },
        },
      },
      auth: { me: async () => ({ user: { id: 'u1', roles: [], memberships: [] }, membership: { role: 'owner' }, can_access_all_tenants: false }) },
    },
    searchCalls,
  };
}

async function mountPage(client: Record<string, unknown>) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(FAQPage, { client: client as never, knowledgeBaseId: 'kb-1' }));
  });
  await settle(20);
  return container;
}

function findTrigger(container: HTMLElement): HTMLButtonElement | undefined {
  return [...container.querySelectorAll('button')].find((node) => node.getAttribute('aria-label') === '检索测试') as HTMLButtonElement | undefined;
}

async function openDrawer(container: HTMLElement) {
  const trigger = findTrigger(container);
  if (!trigger) return false;
  await act(async () => { trigger.click(); await settle(5); });
  return Boolean(document.querySelector('.faq-search-drawer'));
}

/** Poll until the 280ms exit animation finishes and the drawer unmounts. */
async function waitForDrawerGone(timeoutMs = 3000): Promise<boolean> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 50)); });
    if (!document.querySelector('.faq-search-drawer')) return true;
  }
  return !document.querySelector('.faq-search-drawer');
}

async function typeQuery(value: string) {
  const input = document.querySelector('#faq-search-query') as HTMLInputElement | null;
  if (!input) return false;
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
    setter?.call(input, value);
    input.dispatchEvent(new window.Event('input', { bubbles: true }));
    await settle(5);
  });
  return true;
}

// Ledger Round N+8: collect outcomes while the flow settles, assert once at the
// end — an AssertionError at an await boundary hangs node:test.
test('toolbar search-test button opens the drawer and its close button closes it', async () => {
  const fake = fakeClient([]);
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};

  outcomes.triggerFound = Boolean(findTrigger(container));
  if (outcomes.triggerFound) {
    outcomes.drawerOpens = await openDrawer(container);
    const drawer = document.querySelector('.faq-search-drawer');
    const close = drawer?.querySelector('.faq-modal-close') as HTMLButtonElement | null;
    outcomes.closeFound = Boolean(close);
    if (close) {
      await act(async () => { close.click(); await settle(5); });
      outcomes.drawerCloses = await waitForDrawerGone();
    }
  }

  assert.ok(outcomes.triggerFound, 'expected the toolbar 检索测试 trigger');
  assert.equal(outcomes.drawerOpens, true, 'trigger should open the search drawer (not filter the list)');
  assert.ok(outcomes.closeFound, 'drawer should carry a close button');
  assert.equal(outcomes.drawerCloses, true, 'close button should close the drawer');
});

test('search drawer closes on Escape and backdrop click like the Vue drawer', async () => {
  const fake = fakeClient([]);
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};
  outcomes.opened = await openDrawer(container);
  if (outcomes.opened) {
    await act(async () => { document.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await settle(5); });
    outcomes.escapeClosed = await waitForDrawerGone();
  }
  outcomes.reopened = await openDrawer(container);
  if (outcomes.reopened) {
    const overlay = document.querySelector('.faq-search-drawer')?.parentElement as HTMLElement | null;
    if (overlay) await act(async () => { overlay.dispatchEvent(new window.MouseEvent('mousedown', { bubbles: true })); await settle(5); });
    outcomes.backdropClosed = await waitForDrawerGone();
  }
  assert.equal(outcomes.opened, true, 'drawer opened');
  assert.equal(outcomes.escapeClosed, true, 'Escape closes the drawer');
  assert.equal(outcomes.reopened, true, 'drawer reopens');
  assert.equal(outcomes.backdropClosed, true, 'backdrop click closes the drawer');
});

test('blank query warns without posting a search request', async () => {
  const fake = fakeClient([]);
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};

  outcomes.drawerOpens = await openDrawer(container);
  if (outcomes.drawerOpens) {
    const submit = document.querySelector('.faq-search-drawer .search-button') as HTMLButtonElement | null;
    outcomes.submitFound = Boolean(submit);
    if (submit) {
      await act(async () => { submit.click(); await settle(10); });
      outcomes.warningShown = (document.querySelector('main')?.textContent || '').includes('请输入要检索的问题');
      outcomes.noRequest = fake.searchCalls.length === 0;
    }
  }

  assert.equal(outcomes.drawerOpens, true, 'drawer open');
  assert.ok(outcomes.submitFound, 'submit button present');
  assert.ok(outcomes.warningShown, 'Vue queryPlaceholder warning surfaced');
  assert.ok(outcomes.noRequest, 'no faq.search call for a blank query');
});

test('search round-trip posts the trimmed form and renders ranked hits', async () => {
  const fake = fakeClient([
    { id: 2, standard_question: '如何扩容？', similar_questions: ['怎么扩容'], negative_questions: [], answers: ['加节点。'], is_enabled: true, is_recommended: false, score: 0.9123, matched_question: '如何扩容集群' },
    { id: 1, standard_question: '如何部署？', similar_questions: [], negative_questions: [], answers: [], is_enabled: true, is_recommended: false, score: 0.5 },
  ]);
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};

  outcomes.drawerOpens = await openDrawer(container);
  if (outcomes.drawerOpens) {
    outcomes.queryTyped = await typeQuery('  如何部署  ');
    const submit = document.querySelector('.faq-search-drawer .search-button') as HTMLButtonElement | null;
    if (submit) {
      await act(async () => { submit.click(); await settle(20); });
      const drawer = document.querySelector('.faq-search-drawer');
      outcomes.header = drawer?.querySelector('.results-header')?.textContent || '';
      outcomes.firstQuestion = drawer?.querySelector('.result-question')?.textContent || '';
      outcomes.scores = [...(drawer?.querySelectorAll('.score-tag') || [])].map((node) => node.textContent);
      outcomes.payload = fake.searchCalls[0];
    }
  }

  assert.ok(outcomes.queryTyped, 'query input present');
  assert.deepEqual(outcomes.payload, { query_text: '如何部署', vector_threshold: 0.7, match_count: 10 }, 'trimmed Vue-default payload posted');
  assert.ok(String(outcomes.header).includes('检索结果 (2)'), 'results header carries the count');
  assert.ok(String(outcomes.firstQuestion).includes('如何扩容？'), 'highest score first (Vue sort desc)');
  assert.deepEqual(outcomes.scores, ['0.912', '0.500'], '3-decimal score tags');
});

test('search failure surfaces the error in the drawer and clears previous hits', async () => {
  const fake = fakeClient([], { searchError: new Error('检索服务不可用') });
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};

  outcomes.drawerOpens = await openDrawer(container);
  if (outcomes.drawerOpens) {
    await typeQuery('如何部署');
    const submit = document.querySelector('.faq-search-drawer .search-button') as HTMLButtonElement | null;
    if (submit) {
      await act(async () => { submit.click(); await settle(20); });
      const drawer = document.querySelector('.faq-search-drawer');
      outcomes.alertShown = Boolean(drawer?.querySelector('.faq-editor-error [role="alert"]'));
      outcomes.errorText = drawer?.querySelector('.faq-editor-error')?.textContent || '';
      outcomes.noResultsBlock = Boolean(drawer?.querySelector('.no-results'));
      outcomes.busyCleared = !(submit as HTMLButtonElement).disabled;
    }
  }

  assert.equal(outcomes.drawerOpens, true, 'drawer open');
  assert.ok(outcomes.alertShown, 'error slot rendered inside the drawer');
  assert.ok(String(outcomes.errorText).includes('检索服务不可用'), 'error message text visible');
  assert.ok(outcomes.noResultsBlock, 'Vue clears the hits and shows noResults');
  assert.equal(outcomes.busyCleared, true, 'submit leaves the loading state');
});

test('expanding a hit reveals its answers and similar questions', async () => {
  const fake = fakeClient([
    { id: 7, standard_question: '如何部署？', similar_questions: ['docker?'], negative_questions: [], answers: ['使用 Docker。'], is_enabled: true, is_recommended: false, score: 0.9 },
  ]);
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};

  outcomes.drawerOpens = await openDrawer(container);
  if (outcomes.drawerOpens) {
    await typeQuery('如何部署');
    const submit = document.querySelector('.faq-search-drawer .search-button') as HTMLButtonElement | null;
    if (submit) {
      await act(async () => { submit.click(); await settle(20); });
      const collapsedBody = document.querySelector('.faq-search-drawer .result-body');
      outcomes.bodyHiddenWhileCollapsed = !collapsedBody;
      const header = document.querySelector('.faq-search-drawer .result-header') as HTMLButtonElement | null;
      outcomes.headerFound = Boolean(header);
      if (header) {
        await act(async () => { header.click(); await settle(5); });
        const drawer = document.querySelector('.faq-search-drawer');
        outcomes.expandedState = header.getAttribute('aria-expanded');
        const body = drawer?.querySelector('.result-body')?.textContent || '';
        outcomes.answerShown = body.includes('使用 Docker。');
        outcomes.similarShown = body.includes('docker?');
      }
    }
  }

  assert.equal(outcomes.drawerOpens, true, 'drawer open');
  assert.ok(outcomes.bodyHiddenWhileCollapsed, 'hit bodies default collapsed (Vue expanded=false)');
  assert.ok(outcomes.headerFound, 'result header toggler present');
  assert.equal(outcomes.expandedState, 'true', 'expander flips aria-expanded');
  assert.ok(outcomes.answerShown, 'answers section reveals');
  assert.ok(outcomes.similarShown, 'similar questions section reveals');
});

// R488 A3: Vue t-drawer motion semantics — the right panel transitions
// transform 0.28s cubic-bezier(0.38,0,0.24,1) from translateX(100%) and the
// overlay fades (tdesign.css:17144-17158). React must carry the same enter
// animation and keep the DOM mounted through the exit animation instead of
// vanishing instantly.
test('drawer enters with the Vue t-drawer slide/fade and unmounts only after the exit animation', async () => {
  const fake = fakeClient([]);
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};

  outcomes.opened = await openDrawer(container);
  if (outcomes.opened) {
    const overlay = document.querySelector('.faq-search-drawer')?.parentElement ?? null;
    const aside = document.querySelector('.faq-search-drawer');
    outcomes.overlayAnimatesIn = (overlay?.className || '').includes('faq-drawer-overlay-enter');
    outcomes.panelAnimatesIn = (aside?.className || '').includes('faq-drawer-panel-enter');
    const close = aside?.querySelector('.faq-modal-close') as HTMLButtonElement | null;
    outcomes.closeFound = Boolean(close);
    if (close) {
      await act(async () => { close.click(); await settle(10); });
      outcomes.stillMountedDuringExit = Boolean(document.querySelector('.faq-search-drawer'));
      outcomes.exitClasses = (document.querySelector('.faq-search-drawer')?.className || '').includes('faq-drawer-panel-exit');
      outcomes.unmountedAfterExit = await waitForDrawerGone();
    }
  }

  assert.equal(outcomes.opened, true, 'drawer open');
  assert.equal(outcomes.overlayAnimatesIn, true, 'overlay fades in like the t-drawer mask');
  assert.equal(outcomes.panelAnimatesIn, true, 'panel slides in from translateX(100%)');
  assert.ok(outcomes.closeFound, 'close button present');
  assert.equal(outcomes.stillMountedDuringExit, true, 'panel stays mounted while the exit animation runs');
  assert.equal(outcomes.exitClasses, true, 'exit state swaps the animation classes');
  assert.equal(outcomes.unmountedAfterExit, true, 'drawer unmounts after the exit animation completes');
});

// R488 A6 (K2 report): the Vue search drawer has no explicit #footer, so the
// TDesign t-drawer DEFAULT footer renders — for placement="right" the button
// row is [确认(primary), 取消(default)] (tdesign-vue-next drawer.mjs:222
// `placement === "right" ? confirmBtn : null, cancelBtn, ...`; useAction.mjs
// default themes primary/default, zh-CN locale 确认/取消). React must carry the
// same pinned footer button set.
test('search drawer carries the TDesign default footer: 确认 then 取消, pinned at the bottom', async () => {
  const fake = fakeClient([]);
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};

  outcomes.drawerOpens = await openDrawer(container);
  if (outcomes.drawerOpens) {
    const footer = document.querySelector('.faq-search-drawer .faq-editor-footer');
    outcomes.footerFound = Boolean(footer);
    if (footer) {
      const buttons = [...footer.querySelectorAll('button')].map((node) => node.textContent || '');
      outcomes.buttonLabels = buttons;
      // TDesign right placement order: confirm BEFORE cancel.
      outcomes.confirmFirst = buttons[0] === '确认';
      outcomes.cancelSecond = buttons[1] === '取消';
    }
  }

  assert.equal(outcomes.drawerOpens, true, 'drawer open');
  assert.ok(outcomes.footerFound, 'pinned footer rendered inside the search drawer');
  assert.deepEqual(outcomes.buttonLabels, ['确认', '取消'], 'TDesign right-placement default footer order [确认, 取消]');
  assert.equal(outcomes.confirmFirst, true, '确认 renders first (tdesign drawer.mjs right placement)');
  assert.equal(outcomes.cancelSecond, true, '取消 renders second');
});

test('footer 取消 closes the search drawer', async () => {
  const fake = fakeClient([]);
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};

  outcomes.drawerOpens = await openDrawer(container);
  if (outcomes.drawerOpens) {
    const cancel = [...document.querySelectorAll('.faq-search-drawer .faq-editor-footer button')]
      .find((node) => node.textContent === '取消') as HTMLButtonElement | undefined;
    outcomes.cancelFound = Boolean(cancel);
    if (cancel) {
      await act(async () => { cancel.click(); await settle(5); });
      outcomes.drawerClosed = await waitForDrawerGone();
    }
  }

  assert.equal(outcomes.drawerOpens, true, 'drawer open');
  assert.ok(outcomes.cancelFound, 'footer cancel button present');
  assert.equal(outcomes.drawerClosed, true, '取消 closes the drawer (TDesign cancelBtnAction closeDrawer)');
});

test('footer 确认 runs the search test: blank query warns, filled query posts', async () => {
  const fake = fakeClient([
    { id: 3, standard_question: '如何升级？', similar_questions: [], negative_questions: [], answers: ['下载新包。'], is_enabled: true, is_recommended: false, score: 0.88 },
  ]);
  const container = await mountPage(fake.client);
  const outcomes: Record<string, unknown> = {};

  outcomes.drawerOpens = await openDrawer(container);
  if (outcomes.drawerOpens) {
    const confirm = [...document.querySelectorAll('.faq-search-drawer .faq-editor-footer button')]
      .find((node) => node.textContent === '确认') as HTMLButtonElement | undefined;
    outcomes.confirmFound = Boolean(confirm);
    if (confirm) {
      // Blank query → the shared runSearchTest warning path, no request.
      await act(async () => { confirm.click(); await settle(10); });
      outcomes.blankWarning = (document.querySelector('.faq-search-drawer')?.textContent || '').includes('请输入要检索的问题');
      outcomes.noRequestWhenBlank = fake.searchCalls.length === 0;
      // Filled query → confirm posts the search request like 开始检索.
      await typeQuery('如何升级');
      await act(async () => { confirm.click(); await settle(20); });
      outcomes.requestSent = fake.searchCalls.length === 1;
      outcomes.payload = fake.searchCalls[0];
      outcomes.drawerStaysOpen = Boolean(document.querySelector('.faq-search-drawer'));
    }
  }

  assert.equal(outcomes.drawerOpens, true, 'drawer open');
  assert.ok(outcomes.confirmFound, 'footer confirm button present');
  assert.ok(outcomes.blankWarning, '确认 with a blank query surfaces the shared warning');
  assert.ok(outcomes.noRequestWhenBlank, 'no faq.search call while blank');
  assert.equal(outcomes.requestSent, true, '确认 posts the search request');
  assert.deepEqual(outcomes.payload, { query_text: '如何升级', vector_threshold: 0.7, match_count: 10 }, '确认 posts the same Vue-default payload as 开始检索');
  assert.equal(outcomes.drawerStaysOpen, true, 'drawer stays open so results stay readable');
});
