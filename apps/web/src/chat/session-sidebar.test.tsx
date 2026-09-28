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
  // Vue menu.vue:113-131 t-skeleton rowCol [{width:100%,height:14px}] ×4；
  // DOM 复刻 tdesign skeleton 类名（样式由 tdesign.css + platform-shell.td.css 承载）。
  const skeletonBars = html.match(/t-skeleton__col/g)?.length ?? 0;
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
  assert.match(html, /session-list-loading/, 'bottom spinner row like Vue menu.vue:162-167');
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

test('SP13 share menu item follows the onShareSession capability switch', () => {
  // 能力开关惯例（isFeedbackAvailable 模式）：prop 缺省即隐藏菜单项。
  const base = {
    copy: resolveChatCopy('zh-CN'),
    sessions: [{ id: 's1', title: '产品周会纪要', is_pinned: false }],
    selectedSessionId: 's1',
    onSelect: () => undefined,
    onRename: (id: string, title?: string) => { void id; void title; },
  };
  const withoutShare = renderToStaticMarkup(createElement(SessionSidebarList, base));
  assert.doesNotMatch(withoutShare, /分享/, 'share menu item stays hidden without onShareSession');

  const shared = renderToStaticMarkup(createElement(SessionSidebarList, {
    ...base,
    onShareSession: (sessionId: string) => { void sessionId; },
  }));
  assert.match(shared, /分享/, 'share menu item renders with onShareSession');
  // 同一菜单形态：role=menu 内的 menuitem 按钮，携带会话 id 数据钩子。
  assert.match(shared, /data-share-session="s1"/);
});

test('isShareActionAvailable mirrors the isFeedbackAvailable capability gate', async () => {
  const { isShareActionAvailable } = await import('../../../../packages/views/src/chat/session-sidebar.tsx');
  assert.equal(isShareActionAvailable(), false);
  assert.equal(isShareActionAvailable(undefined), false);
  assert.equal(isShareActionAvailable(() => undefined), true);
});

test('share menu label resolves in every chat-copy locale', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const copy = resolveChatCopy(locale);
    assert.ok(copy.shareSession.trim().length > 0, `shareSession copy exists for ${locale}`);
  }
  assert.equal(resolveChatCopy('zh-CN').shareSession, '分享');
});
