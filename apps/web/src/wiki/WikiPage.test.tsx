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
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-1' }));
  assert.match(html, /class="wk-wiki-layout"/);
  assert.match(html, /class="wk-wiki-sidebar"/);
  assert.match(html, /class="wk-wiki-search"/);
  assert.match(html, /aria-label="Wiki pages"/);
  assert.match(html, /class="wk-wiki-editor"/);
  assert.doesNotMatch(html, /class="wk-toolbar"/);
});

test('viewer mode hides Wiki folder mutation controls', () => {
  const client = { wiki: { list: async () => ({ pages: [], total: 0 }) } } as never;
  const html = renderToStaticMarkup(React.createElement(WikiPage, { client, knowledgeBaseId: 'kb-1', canContribute: false }));
  assert.doesNotMatch(html, /目录操作/);
  assert.doesNotMatch(html, /重命名目录/);
  assert.doesNotMatch(html, /删除目录/);
});
