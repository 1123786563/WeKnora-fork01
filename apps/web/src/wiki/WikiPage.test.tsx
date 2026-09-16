import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

 (globalThis as typeof globalThis & { React: typeof React }).React = React;

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { WikiPage, wikiRevertConfirmation } = await import('./WikiPage.tsx');
const { wikiRevertCopy, wikiReaderEmptyState } = await import('./editor.ts');

test('Wiki revert feedback uses the Vue localized confirmation and result keys', () => {
  const translate = (key: string, values?: Record<string, string | number>) => `${key}:${values?.ver ?? ''}`;
  assert.equal(wikiRevertCopy(translate, 'confirm', 4), 'wikiBrowser.revertConfirm:4');
  assert.equal(wikiRevertCopy(translate, 'success', 4), 'wikiBrowser.revertSuccess:4');
  assert.equal(wikiRevertCopy(translate, 'failed', 4), 'wikiBrowser.revertFailed:');
});

test('Wiki shell follows Vue browser anatomy: sidebar search and reader editor are separate', () => {
  const client = { wiki: { list: async () => ({ pages: [], total: 0 }) } } as never;
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-secret', canContribute: true }));
  assert.match(html, /class="wk-wiki-layout[^"]*"/);
  assert.match(html, /class="wk-wiki-sidebar[^"]*"/);
  assert.match(html, /class="wk-wiki-search[^"]*"/);
  assert.match(html, /aria-label="页面操作"/);
  assert.match(html, /树形视图/);
  assert.match(html, /列表视图/);
  assert.match(html, /class="wk-wiki-editor[^"]*"/);
  assert.match(html, /class="wk-wiki-reader-empty[^\"]*"/);
  assert.match(html, /暂无 Wiki 页面/);
  assert.doesNotMatch(html, /class="wk-toolbar"/);
  assert.doesNotMatch(html, /kb-secret/, 'the Vue context eyebrow does not expose the raw KB id');
});

test('Wiki revert confirmation uses the localized Vue copy', () => {
  const translate = (key: string, values?: Record<string, string | number>) => `${key}:${values?.ver ?? ''}`;
  assert.equal(wikiRevertConfirmation(translate, 7), 'wikiBrowser.revertConfirm:7');
});

test('viewer mode hides Wiki folder mutation controls', () => {
  const client = { wiki: { list: async () => ({ pages: [], total: 0 }) } } as never;
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-1', canContribute: false }));
  assert.doesNotMatch(html, /目录操作/);
  assert.doesNotMatch(html, /重命名目录/);
  assert.doesNotMatch(html, /删除目录/);
});

test('viewer mode hides Wiki create and edit surfaces while preserving the read shell', () => {
  const client = { wiki: { list: async () => ({ pages: [], total: 0 }) } } as never;
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-1', canContribute: false }));
  assert.match(html, /class="wk-wiki-layout/);
  assert.doesNotMatch(html, /新建页面|新建 Wiki 页面|编辑页面|保存/);
  assert.doesNotMatch(html, /class="wk-wiki-editor/);
});

test('Wiki defaults to read-only when the contributor capability is omitted', () => {
  const client = { wiki: { list: async () => ({ pages: [], total: 0 }) } } as never;
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-1' }));
  assert.doesNotMatch(html, /class="wk-wiki-editor/);
  assert.doesNotMatch(html, /新建页面|新建 Wiki 页面/);
});

test('Wiki reader preserves the Vue hint when content exists but no page is selected', () => {
  const translate = (key: string) => key;
  assert.deepEqual(wikiReaderEmptyState(translate, true), {
    title: 'wikiBrowser.selectPageHint',
    description: undefined,
  });
  assert.deepEqual(wikiReaderEmptyState(translate, false), {
    title: 'wikiBrowser.emptyTitle',
    description: 'wikiBrowser.emptyDesc',
  });
});

test('contributor mode keeps Wiki create and editor surfaces available', () => {
  const client = { wiki: { list: async () => ({ pages: [], total: 0 }) } } as never;
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-1', canContribute: true }));
  assert.match(html, /class="wk-wiki-editor/);
  assert.match(html, /新建页面|新建 Wiki 页面/);
});

test('Vue page actions expose a contributor-only delete operation backed by the Wiki API', () => {
  const source = WikiPage.toString();
  assert.match(source, /deleteWikiPage\(/);
  assert.match(source, /client\.wiki\.remove\(knowledgeBaseId,selected\.slug\)/);
  assert.match(source, /wikiBrowser\.deletePageConfirm/);
});

// Vue WikiBrowser.vue `cancelEditPage`: the inline editor pairs Save with a
// Cancel button that exits edit mode and discards the draft.
test('Wiki editor exposes a cancel action that exits edit mode without saving', () => {
  const client = { wiki: { list: async () => ({ pages: [], total: 0 }) } } as never;
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-1', canContribute: true }));
  assert.match(html, /class="wk-wiki-editor[^"]*"/);
  const source = WikiPage.toString();
  assert.match(source, /cancelEdit\s*\(/);
  assert.match(source, /t\("common\.cancel"\)/);
});

// Vue WikiBrowser.vue `overwriteSavePage`: on a 409 conflict the editor offers
// "覆盖保存" (overwrite) in addition to reloading the latest version.
test('Wiki conflict state exposes the Vue overwrite action backed by the latest version', () => {
  const source = WikiPage.toString();
  assert.match(source, /saveState\?\.\s*status\s*===?\s*"conflict"/);
  assert.match(source, /wikiBrowser\.editConflictOverwrite/);
  assert.match(source, /overwriteWikiPage\s*\(/);
});
