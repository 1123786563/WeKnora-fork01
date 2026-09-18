// CFT-S01-T010: the workbench conversation column now renders through the
// REAL assistant-ui thread. Layout contract tests (SSR over the full
// CraftWorkbench with stubbed controller/log):
//   1. the composer stays OUTSIDE the scrolling viewport — its position never
//      fights the thread's independent scroll
//   2. narrow switching HIDES panes (data-narrow-hidden) instead of unmounting
//      them — drafts and run subscriptions survive panel switches
//   3. long titles and wide action rows wrap instead of overflowing at 390px
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & {
  registerHooks?: (h: { resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown }) => void;
};
if (hooks.registerHooks) {
  hooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier.endsWith('.css')
        ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
        : nextResolve(specifier, context),
  });
}
const React = await import('react');
const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
const { CraftWorkbench } = await import('./workbench.tsx');
const { createCraftMessageLog } = await import('./presentation.ts');

const controller = {
  load: async () => {},
  submit: async () => {},
  reconnect: async () => {},
  dispose: () => {},
  state: () => null,
  lastError: () => null,
  onChange: () => () => {},
};

const longTitle = '超长标题'.repeat(18) + '——验证任何宽度下都不撑破布局的极长作品名称';

const markup = renderToStaticMarkup(React.createElement(CraftWorkbench, {
  locale: 'zh',
  sessionId: 'ses-layout',
  title: longTitle,
  kind: 'web',
  canWrite: true,
  controller,
  messageLog: createCraftMessageLog(),
  initialPrompt: null,
  resumedRun: false,
  pendingAttachments: [],
  enrichedPending: false,
  onPickAttachments: () => {},
  onRemoveAttachment: () => {},
  onEnrichedSend: async () => {},
  versions: [],
  sessionUpdatedAt: '2026-09-18T00:00:00Z',
  snapshotVersionId: null,
  onRefreshVersions: () => {},
  onIssuePreview: async () => { throw new Error('no preview in layout test'); },
  onDownload: () => {},
} as never));

const css = readFileSync(new URL('./craft.css', import.meta.url), 'utf8');

test('the thread viewport scrolls independently of the composer', () => {
  assert.match(markup, /wk-craft-thread-viewport/, 'the assistant-ui viewport renders');
  assert.match(markup, /<form class="wk-craft-composer"/, 'the host composer renders (draft machine + anchors)');
  // composer must NOT be nested inside the scrolling viewport
  const viewportStart = markup.indexOf('wk-craft-thread-viewport');
  const viewportEnd = markup.indexOf('</div>', markup.lastIndexOf('wk-craft-thread', viewportStart));
  const composerAt = markup.indexOf('<form class="wk-craft-composer"');
  assert.ok(composerAt > viewportEnd || composerAt < viewportStart, 'composer sits outside the viewport subtree');
  assert.match(css, /\.wk-craft-thread-viewport\s*\{[^}]*overflow-y:\s*auto/, 'viewport owns the vertical scroll');
  assert.match(css, /\.wk-craft-thread-supplement\s*\{[^}]*overflow-y:\s*auto/, 'supplements scroll in their own box');
  // the e2e anchors survive the swap
  for (const anchor of ['craft-prompt', 'craft-send', 'craft-upload', 'craft-main-status']) {
    assert.match(markup, new RegExp(`data-testid="${anchor}"`), `${anchor} anchor preserved`);
  }
});

test('narrow switching hides panes instead of unmounting them', () => {
  // both panes render in the same tree; the narrow switch only flips the
  // data-narrow-hidden attribute — React keeps the subtree (draft state,
  // subscriptions) mounted under CSS display:none
  const panes = markup.match(/<section[^>]*wk-craft-pane[^>]*>/g) ?? [];
  assert.ok(panes.length >= 2, 'conversation and artifact panes both exist');
  assert.match(css, /\.wk-craft-pane\[data-narrow-hidden='true'\]\s*\{\s*display:\s*none/, 'hiding is CSS-only');
  assert.doesNotMatch(css, /\.wk-craft-pane\[data-narrow-hidden='true'\][^}]*visibility/);
});

test('long titles and action rows wrap instead of overflowing', () => {
  assert.ok(markup.includes(longTitle), 'the long title renders verbatim');
  assert.match(css, /\.wk-craft-title\s*\{[^}]*overflow-wrap:\s*anywhere/, 'titles wrap anywhere');
  assert.match(css, /\.wk-craft-actions\s*\{[^}]*flex-wrap:\s*wrap/, 'action rows wrap');
  assert.match(css, /\.wk-craft-page\s*\{[^}]*overflow-x:\s*clip/, 'the page clips unexpected horizontal overflow');
  assert.match(css, /\.wk-craft-thread \.wk-craft-msg\s*\{[^}]*overflow-wrap:\s*anywhere/, 'message text never overflows');
});
