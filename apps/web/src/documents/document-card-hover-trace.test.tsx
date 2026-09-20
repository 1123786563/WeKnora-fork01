// R490 B5 (R489 #8) — the "trace summary block" the R489 sweep reported as
// missing from the React document row menu actually lives in the Vue card
// HOVER popover (DocumentCardView.vue knowledge-card-hover-popover: the
// compact KnowledgeProcessingTimeline caption 总耗时 + 更新 + type + the
// 点击卡片查看全文与分段 hint), not inside DocumentActionMenu.vue. The React
// counterpart (DocumentCardGrid hover -> DocumentCardHoverPopover ->
// DocumentTraceCompact) already renders the same block. This test proves it
// end-to-end: hover a failed card, let the shared 300ms delay fire, and the
// popover shows the trace total duration, the updated time and the hint.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge/kb-1/documents' });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  MouseEvent: dom.window.MouseEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot } = await import('react-dom/client');
const { DocumentCardGrid } = await import('./KnowledgeDocumentsPage.tsx');
const { tagSurfaceT } = await import('./tags-locale.ts');
type TraceSummaryLike = import('./doc-row-menu.ts').TraceSummary;

const t = tagSurfaceT('zh-CN');

const failedDocument = {
  id: 'doc-failed',
  file_name: 'trace-summary-demo.md',
  title: 'trace-summary-demo.md',
  type: 'file',
  source: '',
  file_type: 'md',
  parse_status: 'failed',
  summary_status: '',
  updated_at: '2026-09-19T02:35:00Z',
  created_at: '2026-09-18T10:00:00Z',
} as never;

const traceSummary = {
  totalMs: 17,
  duration: '17ms',
  stageIndex: 2,
  stageTotal: 5,
  activeStage: undefined,
  steps: [
    { stage: 'upload', state: 'done' },
    { stage: 'docreader', state: 'failed' },
    { stage: 'chunker', state: 'pending' },
    { stage: 'vectorizer', state: 'pending' },
    { stage: 'notifier', state: 'pending' },
  ],
} as never;

test('document card hover popover renders the Vue trace summary block (总耗时 + 更新 + hint)', async () => {
  document.body.replaceChildren();
  const host = document.createElement('div');
  document.body.append(host);
  let root: { render: (node: React.ReactNode) => void; unmount: () => void } | null = null;
  act(() => { root = createRoot(host); });
  const current = root!;
  act(() => {
    current.render(React.createElement(DocumentCardGrid, {
      items: [failedDocument],
      folders: [],
      selected: new Set<string>(),
      batchMode: false,
      canContribute: false,
      canDownload: false,
      t,
      loadTrace: () => Promise.resolve(traceSummary as TraceSummaryLike),
      onOpen: () => {},
      onOpenFolder: () => {},
      onToggle: () => {},
      onTagEdit: () => {},
      onReparse: () => {},
      onCancelParse: () => {},
      onDownload: () => {},
      onEdit: () => {},
      onViewTrace: () => {},
      onMove: () => {},
      onBatchManage: () => {},
      onDelete: () => {},
    }));
  });

  const card = document.querySelector('.knowledge-card') as HTMLElement;
  assert.ok(card, 'document card rendered');
  // React's synthetic mouseenter rides the bubbling mouseover pair, so a
  // native dispatch here matches what a real pointer produces.
  act(() => {
    card.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
  });
  // Vue cardHoverShowDelay = 300 (DocumentCardView.vue:184); React matches.
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 340)); });
  // Flush the loadTrace promise resolution.
  await act(async () => { await Promise.resolve(); });

  const popover = document.querySelector('.knowledge-card-hover-popover');
  assert.ok(popover, 'hover popover mounted after the 300ms delay');
  const text = popover.textContent ?? '';
  assert.ok(text.includes('trace-summary-demo'), 'popover title (file name)');
  assert.ok(text.includes('总耗时：17ms'), 'compact trace total duration (knowledgeStages.totalDuration)');
  assert.ok(text.includes('更新：'), 'updated-at meta row (knowledgeBase.updatedAt)');
  assert.ok(text.includes('MD'), 'knowledge type chip');
  assert.ok(text.includes('点击卡片查看全文与分段'), 'footer hint (knowledgeBase.clickToViewFull)');
  act(() => current.unmount());
});
