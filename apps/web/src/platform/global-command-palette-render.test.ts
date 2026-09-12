import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

// GlobalCommandPalette.tsx imports a .css file; teach the ESM loader to treat
// it as an empty module (same approach as ConfigurationOperations.test.tsx).
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);

const { GlobalCommandPalette } = await import('./GlobalCommandPalette.tsx');

const noop = () => {};

test('renders nothing while closed', () => {
  const html = renderToStaticMarkup(React.createElement(GlobalCommandPalette, {
    open: false,
    initialQuery: '',
    recentQueries: [],
    locale: 'en-US',
    onClose: noop,
    onNavigate: noop,
    onSearch: noop,
    onClearRecent: noop,
  }));
  assert.equal(html, '');
});

test('empty query shows recent searches and the quick-action command catalogue', () => {
  const html = renderToStaticMarkup(React.createElement(GlobalCommandPalette, {
    open: true,
    initialQuery: '',
    recentQueries: ['prior search'],
    locale: 'en-US',
    onClose: noop,
    onNavigate: noop,
    onSearch: noop,
    onClearRecent: noop,
  }));
  assert.ok(html.includes('Recent'), 'expected Recent group label');
  assert.ok(html.includes('prior search'), 'expected the recent query to render');
  assert.ok(html.includes('Quick actions'), 'expected Quick actions group label');
  assert.ok(html.includes('New conversation'), 'expected the new-chat command label');
  assert.ok(html.includes('Open knowledge bases'));
  assert.ok(html.includes('Open agents'));
  assert.ok(html.includes('Open shared spaces'));
  assert.ok(html.includes('Open settings'));
});

test('a non-empty initial query filters to matching commands only, in the target locale', () => {
  const html = renderToStaticMarkup(React.createElement(GlobalCommandPalette, {
    open: true,
    initialQuery: 'agent',
    recentQueries: ['prior search'],
    locale: 'zh-CN',
    onClose: noop,
    onNavigate: noop,
    onSearch: noop,
    onClearRecent: noop,
  }));
  assert.ok(!html.includes('prior search'), 'recent group should be hidden once searching');
  assert.ok(html.includes('打开智能体'), 'expected the zh-CN agents command label');
  assert.ok(!html.includes('新建对话'), 'expected the new-chat command to be filtered out');
  assert.ok(html.includes('命令'), 'expected the zh-CN Commands group label while searching');
});

test('an unmatched query renders the localized empty state', () => {
  const html = renderToStaticMarkup(React.createElement(GlobalCommandPalette, {
    open: true,
    initialQuery: 'zzz-does-not-match-anything',
    recentQueries: [],
    locale: 'en-US',
    onClose: noop,
    onNavigate: noop,
    onSearch: noop,
    onClearRecent: noop,
  }));
  assert.ok(html.includes('No matches found'));
});
