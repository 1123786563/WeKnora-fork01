import '../test-tdom-harness.ts'; // jsdom 全局（tdesign 运行时；须首个 import）
// B5: Vue FAQTagTooltip parity (frontend/src/components/FAQTagTooltip.vue +
// FAQEntryManager.vue:303-354 card chips, :829-844 search-result chips).
// The bubble opens on hover AND click, carries the full tag content, follows
// the Vue placement math (8px gap, 8px viewport clamp, top→bottom flip).
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

const { createRoot } = await import('react-dom/client');
const { FaqTagTooltip, faqTooltipPosition, FAQPageView, FAQSearchResults } = await import('./FAQPage.tsx');
type FAQViewProps = import('./FAQPage.tsx').FAQPageViewProps;
type FAQResultsProps = import('./FAQPage.tsx').FAQSearchResultsProps;
const { createTranslator } = await import('../i18n.ts');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

async function mount(node: React.ReactElement) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => { mountedRoot?.render(node); });
  return container;
}

const rect = (top: number, left: number, width: number, height: number) => ({ top, left, width, height, bottom: top + height, right: left + width });

// --- Vue updatePosition math (FAQTagTooltip.vue:63-101) -----------------------------

test('faqTooltipPosition mirrors Vue top placement: centered, 8px above', () => {
  const pos = faqTooltipPosition('top', rect(100, 100, 60, 24), rect(0, 0, 200, 36), { width: 1024, height: 768 });
  assert.deepEqual(pos, { top: 56, left: 30, placement: 'top' });
});

test('faqTooltipPosition flips top to bottom when the space above is under 8px', () => {
  const pos = faqTooltipPosition('top', rect(20, 500, 60, 24), rect(0, 0, 200, 36), { width: 1024, height: 768 });
  assert.deepEqual(pos, { top: 52, left: 430, placement: 'bottom' });
});

test('faqTooltipPosition clamps the bubble to the viewport with 8px padding', () => {
  assert.equal(faqTooltipPosition('top', rect(300, 5, 20, 24), rect(0, 0, 300, 36), { width: 1024, height: 768 }).left, 8);
  assert.equal(faqTooltipPosition('top', rect(300, 1000, 20, 24), rect(0, 0, 300, 36), { width: 1024, height: 768 }).left, 1016 - 300);
  assert.equal(faqTooltipPosition('bottom', rect(700, 500, 20, 24), rect(0, 0, 200, 500), { width: 1024, height: 768 }).top, 760 - 500);
});

// --- component behavior (Vue mouseenter/mouseleave + task click requirement) -------

test('the bubble opens on hover with the full content and Vue classes, closes on leave', async () => {
  await mount(React.createElement(FaqTagTooltip, { content: '很长的相似问内容，需要气泡展示', type: 'similar' }, React.createElement('span', { className: 'question-tag' }, '很长的相似问内容，需要…')));
  assert.equal(document.querySelector('.faq-tag-tooltip'), null, 'bubble hidden before hover');
  const wrapper = document.querySelector('.faq-tag-wrapper') as HTMLElement;
  assert.ok(wrapper, 'wrapper mirrors the Vue faq-tag-wrapper');
  // jsdom rects are all zero (which would trigger the top→bottom flip); pin a
  // mid-viewport rect so the placement math is observable.
  Object.defineProperty(wrapper, 'getBoundingClientRect', { configurable: true, value: () => ({ top: 300, left: 100, width: 60, height: 24, bottom: 324, right: 160, x: 100, y: 300, toJSON: () => ({}) }) });
  await act(async () => { wrapper.dispatchEvent(new window.MouseEvent('mouseover', { bubbles: true })); });
  const bubble = document.querySelector('.faq-tag-tooltip') as HTMLElement | null;
  assert.ok(bubble, 'bubble teleported to body on hover');
  assert.ok(bubble.classList.contains('tooltip-similar'), 'type class (Vue tooltipClass)');
  assert.ok(bubble.classList.contains('placement-top'), 'default placement top');
  assert.equal(bubble.style.top, '292px', '300 − bubble height(0 in jsdom) − 8px gap');
  assert.equal(bubble.style.left, '130px', 'centred on the chip');
  assert.match(bubble.querySelector('.tooltip-content')?.textContent || '', /很长的相似问内容，需要气泡展示/, 'full content, pre-wrap');
  await act(async () => { wrapper.dispatchEvent(new window.MouseEvent('mouseout', { bubbles: true })); });
  assert.equal(document.querySelector('.faq-tag-tooltip'), null, 'bubble hidden after leave (Vue handleMouseLeave)');
});

// Vue FAQTagTooltip.vue:109-118 仅 mouseenter/mouseleave（无点击切换）——平移后同款。
test('the bubble follows hover like the Vue wrapper (no click toggle)', async () => {
  await mount(React.createElement(FaqTagTooltip, { content: '答案全文' }, React.createElement('span', { className: 'question-tag' }, '答案')));
  const wrapper = document.querySelector('.faq-tag-wrapper') as HTMLElement;
  await act(async () => { wrapper.dispatchEvent(new window.MouseEvent('mouseover', { bubbles: true })); });
  assert.ok(document.querySelector('.faq-tag-tooltip'), 'hover opens the bubble');
  await act(async () => { wrapper.dispatchEvent(new window.MouseEvent('mouseout', { bubbles: true })); });
  assert.equal(document.querySelector('.faq-tag-tooltip'), null, 'leave closes the bubble');
});

// --- wiring: card sections + search hits wrap their chips ---------------------------

const cardRows = [
  { id: 1, standard_question: '如何部署？', similar_questions: ['docker?'], negative_questions: ['什么是反例'], answers: ['使用 Docker。'], is_enabled: true, is_recommended: false },
] as never;

// Vue 折叠态（v-if）不渲染分区 chip，页脚 chip 是 t-dropdown+t-tag（无 tooltip 包裹）。
test('entry-card chips stay unmounted while collapsed and carry no native title', () => {
  const html = (require('react-dom/server') as typeof import('react-dom/server')).renderToStaticMarkup(
    React.createElement<FAQViewProps>(FAQPageView, { entries: cardRows, total: 1 }),
  );
  const wrappers = html.match(/faq-tag-wrapper/g) || [];
  assert.equal(wrappers.length, 0, 'collapsed sections unmount their chips (Vue v-if)');
  assert.ok(!html.includes('title="docker?"'), 'native title removed in favour of the bubble (Vue has none)');
  assert.ok(!html.includes('title="什么是反例"'), 'native title removed on negative chips');
  assert.ok(!html.includes('title="使用 Docker。"'), 'native title removed on answer chips');
  assert.ok(html.includes('faq-tag-chip'), 'footer chip renders (Vue t-dropdown + t-tag)');
});

test('footer chips render the resolved tag names, untagged fallback otherwise', async () => {
  await mount(React.createElement<FAQViewProps>(FAQPageView, {
    tags: [{ id: 'tag-1', seq_id: 5, name: '生产环境标签' }],
    entries: [
      { id: 1, standard_question: '如何部署？', similar_questions: [], negative_questions: [], answers: ['使用 Docker。'], tag_id: 5, is_enabled: true, is_recommended: false },
      { id: 2, standard_question: '如何回滚？', similar_questions: [], negative_questions: [], answers: ['回滚脚本。'], tag_id: null, is_enabled: true, is_recommended: false },
    ] as never,
    total: 2,
  }));
  const chips = document.querySelectorAll('.faq-card-tag .faq-tag-chip .tag-text');
  assert.equal(chips.length, 2, 'both footer chips render');
  assert.equal(chips[0]?.textContent, '生产环境标签', 'tagged chip shows the full tag name');
  assert.equal(chips[1]?.textContent, '无标签', 'untagged fallback (zh-CN)');
  for (const chip of document.querySelectorAll('.faq-tag-chip')) {
    assert.ok(!chip.hasAttribute('title'), 'no native title on the chip');
  }
});

test('search-result chips carry the Vue t-tooltip wrappers without native titles', () => {
  const hit = { id: 7, standard_question: 'Q', similar_questions: ['s1'], negative_questions: [], answers: ['a1'], is_enabled: true, is_recommended: false, score: 0.9 } as never;
  const html = (require('react-dom/server') as typeof import('react-dom/server')).renderToStaticMarkup(
    React.createElement<FAQResultsProps>(FAQSearchResults, { results: [hit], expandedIds: new Set([7]) }),
  );
  // Vue :829-844 t-tooltip（懒渲染）；chip 为 answer-tag/question-tag。
  // tdesign Tag 双端默认在 span 上渲染 title=内容（tag.mjs renderTitle）——库一致行为。
  assert.ok(html.includes('answer-tag'), 'answer chip class rendered');
  assert.ok(html.includes('question-tag'), 'similar chip class rendered');
  assert.ok(html.includes('title="s1"'), 'tdesign Tag native title matches vue-next renderTitle');
});

test('the summary translator keys stay resolvable for tooltip-adjacent copy', () => {
  const t = createTranslator('zh-CN');
  assert.ok(t('FAQ.import.downloadReasons').length > 0);
});
