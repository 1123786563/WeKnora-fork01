// CFT-S01-T011 component contract: the versions drawer renders the
// view/restore separation — restore needs an explicit confirmation, and a
// version without a snapshot shows restore disabled WITH its reason while
// download stays enabled.
import { test } from 'node:test';
import assert from 'node:assert/strict';
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
const { CraftVersionsDrawer } = await import('./versions.tsx');

const selection = {
  viewVersionId: 'v2',
  baseVersionId: 'v1',
  workspaceRevision: 7,
  restorableVersionIds: ['v1'],
  versions: [
    { id: 'v1', published: true, fileCount: 2, hasSnapshot: true, citations: [] },
    { id: 'v2', published: true, fileCount: 3, hasSnapshot: false, citations: [] },
  ],
};
const rows = [
  { versionId: 'v1', runId: 'run-a', updatedAt: '2026-09-18T01:00:00Z', checksPassed: 2, checksTotal: 2 },
  { versionId: 'v2', runId: 'run-b', updatedAt: '2026-09-18T02:00:00Z', checksPassed: 1, checksTotal: 2 },
];

const markup = renderToStaticMarkup(React.createElement(CraftVersionsDrawer, {
  locale: 'zh',
  selection,
  rows,
  canWrite: true,
  hasActiveRun: false,
  onView: () => {},
  onDownload: () => {},
  onRestore: async () => {},
} as never));

test('v1 (snapshot) restores; v2 (no snapshot) keeps download, disables restore with a reason', () => {
  const v1 = markup.slice(markup.indexOf('data-version="v1"'), markup.indexOf('data-version="v2"'));
  const v2 = markup.slice(markup.indexOf('data-version="v2"'));
  // v1: restore enabled
  assert.match(v1, /从此版本继续/);
  assert.ok(!/disabled=""[^>]*>从此版本继续/.test(v1) || v1.indexOf('disabled') > v1.indexOf('从此版本继续'), 'v1 restore enabled');
  // v2: download enabled, restore disabled with the snapshot reason
  assert.match(v2, /no_complete_recovery_snapshot/, 'the block reason stays readable');
  const v2Buttons = v2.match(/<button[^>]*>(?:下载|从此版本继续)<\/button>/g) ?? [];
  const downloadBtn = v2Buttons.find((b) => b.includes('下载'));
  const restoreBtn = v2Buttons.find((b) => b.includes('从此版本继续'));
  assert.ok(downloadBtn && !downloadBtn.includes('disabled=""'), 'download stays available without a snapshot');
  assert.ok(restoreBtn && (restoreBtn.includes('disabled=""') || restoreBtn.includes('no_complete_recovery_snapshot')), 'restore disabled without a snapshot');
});

test('the restore confirmation dialog states the source and the no-overwrite contract', async () => {
  const { JSDOM: Dom } = await import('jsdom');
  const dom = new Dom('<!doctype html><html><body></body></html>');
  Object.assign(globalThis, {
    window: dom.window,
    document: dom.window.document,
    HTMLElement: dom.window.HTMLElement,
    Element: dom.window.Element,
    Node: dom.window.Node,
    Event: dom.window.Event,
    IS_REACT_ACT_ENVIRONMENT: true,
  });
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
  const { act } = await import('react');
  const { createRoot } = await import('../../../../apps/web/node_modules/react-dom/client.js');
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(React.createElement(CraftVersionsDrawer, {
      locale: 'zh',
      selection,
      rows,
      canWrite: true,
      hasActiveRun: false,
      onView: () => {},
      onDownload: () => {},
      onRestore: async () => {},
    } as never));
  });
  const restoreBtn = [...document.querySelectorAll('button')].find((b) => b.textContent === '从此版本继续' && !b.disabled);
  assert.ok(restoreBtn, 'v1 restore button enabled');
  await act(async () => {
    restoreBtn.click();
  });
  const dialog = document.querySelector('[role="dialog"]');
  assert.ok(dialog, 'the confirmation dialog opens');
  const text = document.body.textContent ?? '';
  assert.match(text, /确认恢复/);
  assert.match(text, /完整恢复快照重建工作区/);
  assert.match(text, /已发布的历史版本不会被覆盖/);
  assert.match(text, /下一轮交付才会发布新版本/);
  assert.match(text, /编辑基线/, 'the baseline row is labelled');
  await act(async () => {
    root.unmount();
  });
});
