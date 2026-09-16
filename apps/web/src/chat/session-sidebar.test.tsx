import assert from 'node:assert/strict';
import test from 'node:test';
import * as React from 'react';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import { SessionSidebarList } from '../../../../packages/views/src/chat/session-sidebar.tsx';
import { resolveChatCopy } from '../../../../packages/views/src/chat/chat-copy.ts';

Object.assign(globalThis, { React });

test('session rows expose the Vue running-session status indicator', () => {
  const html = renderToStaticMarkup(createElement(SessionSidebarList, {
    copy: resolveChatCopy('zh-CN'),
    sessions: [{ id: 's1', title: '正在运行', is_pinned: false, running: true }],
    selectedSessionId: 's1',
    onSelect: () => undefined,
  }));

  assert.match(html, /role="status"/);
  assert.match(html, /会话进行中/);
  assert.match(html, /正在运行/);

  const idleHtml = renderToStaticMarkup(createElement(SessionSidebarList, {
    copy: resolveChatCopy('zh-CN'),
    sessions: [{ id: 's2', title: '已完成', is_pinned: false, running: false }],
    selectedSessionId: 's2',
    onSelect: () => undefined,
  }));
  assert.doesNotMatch(idleHtml, /会话进行中/);
  assert.match(idleHtml, /已完成/);
});

test('loading with an empty list renders Vue skeleton rows, not a text loading label', () => {
  // Vue menu.vue:113-131: four gradient skeleton rows while the first bucket
  // loads — never a visible "Loading..." text. The i18n copy stays as an
  // sr-only announcement for screen readers.
  const html = renderToStaticMarkup(createElement(SessionSidebarList, {
    copy: resolveChatCopy('zh-CN'),
    groups: [],
    selectedSessionId: null,
    loading: true,
    onSelect: () => undefined,
  }));
  assert.doesNotMatch(html, /Loading\.\.\./);
  assert.match(html, /加载中\.\.\./);
  const skeletonBars = html.match(/animate-pulse/g)?.length ?? 0;
  assert.equal(skeletonBars, 4, 'four skeleton rows like the Vue submenu boot state');
  assert.doesNotMatch(html, /暂无对话/, 'empty state stays hidden while loading');
});

test('loading a later page keeps the loaded rows visible with a bottom spinner', () => {
  // Vue menu.vue:162-167: rows stay mounted while a bucket continuation
  // streams in; only a small spinner appends below.
  const html = renderToStaticMarkup(createElement(SessionSidebarList, {
    copy: resolveChatCopy('zh-CN'),
    sessions: [{ id: 's1', title: '已加载的会话', is_pinned: false }],
    selectedSessionId: null,
    loading: true,
    onSelect: () => undefined,
  }));
  assert.match(html, /已加载的会话/);
  assert.match(html, /animate-spin/);
  assert.doesNotMatch(html, /Loading\.\.\./);
});

test('loading copy follows the passed chat copy table instead of navigator.language', () => {
  // The shell passes resolveChatCopy(locale); an en-US table renders the
  // English sr-only announcement, zh-CN the Chinese one — both without a
  // visible text label.
  const html = renderToStaticMarkup(createElement(SessionSidebarList, {
    copy: resolveChatCopy('en-US'),
    groups: [],
    selectedSessionId: null,
    loading: true,
    onSelect: () => undefined,
  }));
  assert.match(html, /Loading\.\.\./);
  // Loading copy renders only inside the sr-only announcement span (the
  // static markup cannot express visual hiding, so assert the placement).
  const occurrences = [...html.matchAll(/Loading\.\.\./g)];
  assert.equal(occurrences.length, 1);
  assert.match(html.slice(Math.max(0, (occurrences[0].index ?? 0) - 30), occurrences[0].index), /class="sr-only">$/, 'loading copy is announcement-only');
});
