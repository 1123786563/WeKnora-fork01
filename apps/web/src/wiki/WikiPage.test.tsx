import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

 (globalThis as typeof globalThis & { React: typeof React }).React = React;

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { WikiPage } = await import('./WikiPage.tsx');

test('Wiki shell follows Vue browser anatomy: sidebar search and reader editor are separate', () => {
  const client = { wiki: { list: async () => ({ pages: [], total: 0 }) } } as never;
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-1', canContribute: true }));
  assert.match(html, /class="wk-wiki-layout[^"]*"/);
  assert.match(html, /class="wk-wiki-sidebar[^"]*"/);
  assert.match(html, /class="wk-wiki-search[^"]*"/);
  assert.match(html, /aria-label="页面操作"/);
  assert.match(html, /树形视图/);
  assert.match(html, /列表视图/);
  assert.match(html, /class="wk-wiki-editor[^"]*"/);
  assert.doesNotMatch(html, /class="wk-toolbar"/);
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

test('contributor mode keeps Wiki create and editor surfaces available', () => {
  const client = { wiki: { list: async () => ({ pages: [], total: 0 }) } } as never;
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-1', canContribute: true }));
  assert.match(html, /class="wk-wiki-editor/);
  assert.match(html, /新建页面|新建 Wiki 页面/);
});
