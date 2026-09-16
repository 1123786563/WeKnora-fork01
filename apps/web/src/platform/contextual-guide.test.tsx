// Contextual-guide host/component behavior tests (jsdom + react-dom),
// following the harness in apps/web/src/platform/new-user-guide.test.tsx.
// These live under apps/web because only this package resolves react-dom
// (packages/views/src/chat/tool-approval.test.tsx convention); the components
// themselves live in packages/views/src/guides/.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

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
const { ContextualGuide, ContextualGuideHost } = await import('../../../../packages/views/src/guides/ContextualGuide.tsx');
const {
  CONTEXTUAL_GUIDE_PENDING_KEY,
  CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS,
  GLOBAL_USER_GUIDE_KEY,
  OPEN_CONTEXTUAL_GUIDE_EVENT,
  openContextualGuide,
} = await import('../../../../packages/views/src/guides/contextual-guides.ts');
import type { ContextualGuideTourId } from '../../../../packages/views/src/guides/contextual-guides.ts';

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.clear();
  dom.window.sessionStorage.clear();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

const GLOBAL_DONE = { [GLOBAL_USER_GUIDE_KEY]: '1' };

const FAST = {
  pollIntervalMs: 5,
  openDelayOverrideMs: 5,
  beforeDelayMs: 0,
  locateRetryDelayMs: 1,
} as const;

async function mountHost(extraProps: Record<string, unknown> = {}) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(ContextualGuideHost, { locale: 'zh-CN', ...FAST, ...extraProps }));
  });
  await settle(15);
  return container;
}

async function mountDirect(tour: ContextualGuideTourId, extraProps: Record<string, unknown> = {}, locale: 'zh-CN' | 'en-US' | 'ja-JP' | 'ko-KR' | 'ru-RU' = 'zh-CN') {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(ContextualGuide, { tour, locale, open: true, onDismiss: () => {}, ...extraProps }));
  });
  await settle(15);
  return container;
}

const overlay = () => document.querySelector('[data-testid="wk-contextual-guide"]');
const card = () => document.querySelector('[data-testid="wk-contextual-guide-card"]');
const stepTitle = () => card()?.querySelector('.wk-guide__title')?.textContent ?? '';
const stepLabel = () => card()?.querySelector('.wk-guide__step-label')?.textContent ?? '';
const buttonByText = (text: string): HTMLButtonElement | null => {
  for (const button of document.querySelectorAll('button')) {
    if (button.textContent === text) return button as HTMLButtonElement;
  }
  return null;
};
async function click(button: HTMLButtonElement) {
  await act(async () => {
    button.click();
    await new Promise((resolve) => setTimeout(resolve, 5));
  });
}

test('(dismissal) a finished tour never re-opens, even when triggered again', async () => {
  dom.window.localStorage.setItem(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS.kbList, '1');
  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  await mountHost();
  await act(async () => {
    openContextualGuide('kbList');
    await new Promise((resolve) => setTimeout(resolve, 15));
  });
  await settle(15);
  assert.equal(overlay(), null, 'done-key must block the trigger');
});

test('(gating) the host waits for the global welcome tour before opening (400ms poll semantics)', async () => {
  await mountHost();
  await act(async () => {
    openContextualGuide('kbList');
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  await settle(20);
  assert.equal(overlay(), null, 'global guide still pending — contextual guide stays closed');
  await act(async () => {
    dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
    await new Promise((resolve) => setTimeout(resolve, 30));
  });
  await settle(20);
  assert.ok(overlay(), 'poll must arm the open once the global guide finishes');
  assert.equal(stepTitle(), '创建第一个知识库');
});

test('(routing) openContextualGuide opens the requested tour with the Vue zh-CN copy and interact semantics', async () => {
  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  await mountHost();
  await act(async () => {
    openContextualGuide('agentList');
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  await settle(15);
  assert.ok(overlay());
  assert.equal(overlay()?.getAttribute('data-guide-tour'), 'agentList');
  assert.equal(stepTitle(), '创建你的智能体');
  assert.equal(stepLabel(), '1 / 1');
  assert.ok(card()?.querySelector('.wk-guide__interact-hint'), 'interact steps show the direct-click hint');
  assert.equal(card()?.querySelector('.wk-guide__interact-hint')?.textContent, '请直接点击高亮区域继续');
  assert.equal(buttonByText('下一步'), null, 'interact steps hide next/done (SpotlightGuide.vue:34)');
  assert.ok(buttonByText('跳过'));
});

test('(anchor spotlight) a fake anchor with a real box produces hole + ring + card placement', async () => {
  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  const anchor = document.createElement('button');
  anchor.setAttribute('data-guide', 'kb-list-create');
  anchor.style.width = '200px';
  anchor.style.height = '40px';
  document.body.append(anchor);
  // jsdom reports zero boxes; pin the geometry like a laid-out element.
  anchor.getBoundingClientRect = () => ({ x: 100, y: 200, width: 200, height: 40, top: 200, left: 100, right: 300, bottom: 240, toJSON: () => ({}) }) as DOMRect;
  await mountHost();
  await act(async () => {
    openContextualGuide('kbList');
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  await settle(15);
  const spot = document.querySelector('.wk-guide__spot') as HTMLElement | null;
  const ring = document.querySelector('.wk-guide__ring') as HTMLElement | null;
  assert.ok(spot, 'spotlight hole must render for a located anchor');
  assert.ok(ring, 'focus ring must render for a located anchor');
  assert.match(spot.style.left, /px/);
  assert.match(ring.style.width, /px/);
  const spotShadow = spot.style.boxShadow;
  assert.ok(spotShadow.includes('9999px'), 'spot dims the page via the 9999px shadow');
  assert.equal(card()?.className.includes('wk-guide__card--center'), false, 'anchored step does not center the card');
  assert.equal(document.querySelectorAll('.wk-guide__backdrop--hit').length, 4, 'four-piece backdrop renders');
});

test('(dismissal) skipping persists the tour key and cascades alsoCompleteTours', async () => {
  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  await mountHost();
  await act(async () => {
    openContextualGuide('kbCreate', { isFaq: true });
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  await settle(15);
  assert.ok(overlay());
  assert.equal(stepTitle(), '选择知识库类型');
  const skip = buttonByText('跳过');
  assert.ok(skip);
  await click(skip!);
  assert.equal(overlay(), null, 'overlay unmounts after skip');
  assert.equal(dom.window.localStorage.getItem(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS.kbCreate), '1');
  assert.equal(dom.window.localStorage.getItem(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS.kbList), '1', 'kbCreate completion retires kbList (Vue alsoCompleteTours)');
});

test('(routing) the pending sessionStorage hand-off opens the tour after a full-page navigation', async () => {
  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  dom.window.sessionStorage.setItem(CONTEXTUAL_GUIDE_PENDING_KEY, JSON.stringify({ tour: 'kbDetail', options: {} }));
  await mountHost();
  await settle(20);
  assert.ok(overlay(), 'destination-page host must consume the intent and open');
  assert.equal(stepTitle(), '知识库还是空的');
  assert.equal(dom.window.sessionStorage.getItem(CONTEXTUAL_GUIDE_PENDING_KEY), null, 'intent is consumed (one-shot)');
});

test('tenantModels agent variant renders the stepsAgent copy', async () => {
  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  await mountHost();
  await act(async () => {
    openContextualGuide('tenantModels', { variant: 'agent' });
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  await settle(15);
  assert.ok(overlay());
  assert.equal(stepTitle(), '需要先配置对话模型');
  const next = buttonByText('下一步');
  assert.ok(next);
  await click(next!);
  await settle(10);
  assert.equal(stepTitle(), '添加对话模型', 'second step comes from the stepsAgent block');
});

test('(direct mount) ContextualGuide walks kbDetail steps, marks dots, and reports dismissal', async () => {
  let dismissed = 0;
  await mountDirect('kbDetail', { onDismiss: () => { dismissed += 1; }, locateRetryDelayMs: 1, beforeDelayMs: 0 });
  assert.ok(overlay());
  assert.equal(stepTitle(), '知识库还是空的');
  assert.equal(stepLabel(), '1 / 3');
  assert.ok(card()?.className.includes('wk-guide__card--center'), 'intro step centers the card');
  await click(buttonByText('下一步')!);
  await settle(30);
  assert.equal(stepTitle(), '添加文档');
  assert.equal(stepLabel(), '2 / 3');
  await click(buttonByText('下一步')!);
  await settle(10);
  assert.equal(stepTitle(), '解析完成后即可使用');
  assert.equal(card()?.querySelectorAll('.wk-guide__dot.is-done').length, 2, 'traversed dots render as done');
  await click(buttonByText('知道了')!);
  assert.equal(dismissed, 1, 'done button reports dismissal to the parent');
});

test('(direct mount) optional chat kb step auto-skips when its anchor is missing', async () => {
  await mountDirect('chat', { locateRetryDelayMs: 1, beforeDelayMs: 0 });
  await settle(40);
  assert.equal(stepTitle(), '输入你的问题', 'optional kb step (missing anchor) is skipped, input follows');
  assert.equal(stepLabel(), '2 / 4');
});

test('Escape dismisses and reports to the host like the Vue keydown handler', async () => {
  let dismissed = 0;
  await mountDirect('kbList', { locateRetryDelayMs: 1, beforeDelayMs: 0, onDismiss: () => { dismissed += 1; } });
  assert.ok(overlay());
  await act(async () => {
    // React 18 attaches synthetic listeners at the root container; a native
    // bubbling keydown on the overlay triggers the component's onKeyDown.
    overlay()?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
    await new Promise((resolve) => setTimeout(resolve, 5));
  });
  assert.equal(dismissed, 1, 'Escape reports dismissal (SpotlightGuide.vue:5)');
});

// --- kbDetail host flows (documents page trigger + Vue KnowledgeBase.vue:339-345) ---

/** The toolbar upload anchor, laid out like a real button (jsdom boxes are zero). */
function addUploadAnchor() {
  const anchor = document.createElement('button');
  anchor.setAttribute('data-guide', 'kb-detail-add-doc');
  anchor.style.width = '28px';
  anchor.style.height = '28px';
  document.body.append(anchor);
  anchor.getBoundingClientRect = () => ({ x: 900, y: 200, width: 28, height: 28, top: 200, left: 900, right: 928, bottom: 228, toJSON: () => ({}) }) as DOMRect;
  return anchor;
}

test('(kbDetail) the tour opens after the 600ms catalog delay with the intro step centered', async () => {
  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  addUploadAnchor();
  await mountHost({ openDelayOverrideMs: undefined });
  await act(async () => {
    openContextualGuide('kbDetail');
    await new Promise((resolve) => setTimeout(resolve, 200));
  });
  assert.equal(overlay(), null, 'no overlay before the 600ms open delay (contextualGuides.ts:78)');
  await settle(700);
  assert.ok(overlay(), 'overlay appears after the catalog open delay');
  assert.equal(overlay()?.getAttribute('data-guide-tour'), 'kbDetail');
  assert.equal(stepTitle(), '知识库还是空的');
  assert.equal(stepLabel(), '1 / 3');
  assert.ok(card()?.className.includes('wk-guide__card--center'), 'intro step centers the card');
});

test('(kbDetail) next/prev walk the three steps and the upload anchor is spotlighted', async () => {
  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  addUploadAnchor();
  await mountHost();
  await act(async () => {
    openContextualGuide('kbDetail');
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  await settle(15);
  assert.equal(stepLabel(), '1 / 3');

  await click(buttonByText('下一步')!);
  await settle(30);
  assert.equal(stepTitle(), '添加文档');
  assert.equal(stepLabel(), '2 / 3');
  assert.ok(document.querySelector('.wk-guide__spot'), 'upload anchor gets the spotlight hole');
  assert.ok(document.querySelector('.wk-guide__ring'), 'brand ring renders around the hole');

  await click(buttonByText('上一步')!);
  await settle(30);
  assert.equal(stepTitle(), '知识库还是空的', 'prev returns to the intro step');
  assert.equal(stepLabel(), '1 / 3');
  assert.ok(card()?.className.includes('wk-guide__card--center'), 'centered again on the target-less step');
});

test('(kbDetail) skip, done and Escape each close the tour and persist the one-shot key', async () => {
  for (const [label, walk, close] of [
    ['skip', 0, '跳过'],
    ['done', 2, '知道了'],
  ] as const) {
    dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
    addUploadAnchor();
    await mountHost();
    await act(async () => {
      openContextualGuide('kbDetail');
      await new Promise((resolve) => setTimeout(resolve, 10));
    });
    await settle(15);
    for (let i = 0; i < walk; i += 1) {
      await click(buttonByText('下一步')!);
      await settle(20);
    }
    await click(buttonByText(close)!);
    assert.equal(overlay(), null, `${label} closes the tour`);
    assert.equal(dom.window.localStorage.getItem(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS.kbDetail), '1', `${label} writes weknora:contextual-guide-kb-detail:v1`);
    await act(async () => mountedRoot?.unmount());
    mountedRoot = undefined;
    document.body.replaceChildren();
    dom.window.localStorage.clear();
  }

  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  addUploadAnchor();
  await mountHost();
  await act(async () => {
    openContextualGuide('kbDetail');
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  await settle(15);
  await act(async () => {
    overlay()?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
    await new Promise((resolve) => setTimeout(resolve, 5));
  });
  assert.equal(overlay(), null, 'Escape closes the tour');
  assert.equal(dom.window.localStorage.getItem(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS.kbDetail), '1', 'Escape persists like Vue onFinish');
});

test('(kbDetail) a finished tour never re-opens when the documents page re-triggers', async () => {
  dom.window.localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1');
  dom.window.localStorage.setItem(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS.kbDetail, '1');
  addUploadAnchor();
  await mountHost();
  await act(async () => {
    openContextualGuide('kbDetail');
    await new Promise((resolve) => setTimeout(resolve, 15));
  });
  await settle(30);
  assert.equal(overlay(), null, 'the weknora:contextual-guide-kb-detail:v1 key blocks the trigger');
});
