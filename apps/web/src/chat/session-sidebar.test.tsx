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
