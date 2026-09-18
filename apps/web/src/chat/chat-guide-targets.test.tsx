import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import React from 'react';

import { ChatComposer, SessionSidebarList, resolveChatCopy } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

/*
 * R469-A2 — Vue creatChat.vue mounts <ContextualGuide tour="chat"> on the new
 * chat entry; the tour spotlights three composer anchors (frontend/src/
 * components/Input-field.vue):
 *   data-guide="chat-input"      → rich-input-container (line 2623)
 *   data-guide="chat-kb-mention" → @ knowledge-scope button (line 2769)
 *   data-guide="chat-send"       → send button (line 2851)
 * The React host/catalog/trigger already exist (guides/ContextualGuide.tsx +
 * ChatRoutePage openContextualGuide('chat')); the composer must carry the same
 * anchors so the spotlight resolves real controls instead of falling back to
 * centered cards.
 */

const composerProps = {
  draft: '',
  onDraftChange: () => undefined,
  onSubmit: () => undefined,
  copy: resolveChatCopy('zh-CN'),
};

test('composer carries the three Vue chat-tour spotlight anchors', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, composerProps));
  assert.match(html, /data-guide="chat-input"/);
  assert.match(html, /data-guide="chat-kb-mention"/);
  assert.match(html, /data-guide="chat-send"/);
});

test('chat-send anchor marks the submit button, not the streaming stop control', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, {
    ...composerProps,
    disabled: true,
    streaming: true,
    onStop: () => undefined,
  }));
  // Vue swaps send for stop while streaming (Input-field.vue:2851 vs stop
  // branch); the tour anchor must not survive onto the stop control.
  assert.doesNotMatch(html, /data-guide="chat-send"/);
  assert.match(html, /class="wk-chat-stop/);
});

test('new-chat entry still arms the chat contextual tour (Vue creatChat.vue:51)', () => {
  const routeSource = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  assert.match(routeSource, /openContextualGuide\('chat'\)/);
});

/*
 * R468 P1 (second half): the Vue "近7天" time-group header (menu.vue
 * timeline_header) is a plain label — no click navigation. The React grouped
 * list renders the same semantics; keep the header non-interactive so a click
 * on the group label can never navigate or select anything.
 */
test('session group headers stay non-interactive labels like the Vue timeline_header', () => {
  const html = renderToStaticMarkup(React.createElement(SessionSidebarList, {
    groups: [{ key: 'last7', label: '近7天', items: [{ id: 'session-1', title: 'Chat', is_pinned: false }] }],
    selectedSessionId: null,
    onSelect: () => undefined,
  }));
  assert.match(html, /<h3[^>]*>近7天<\/h3>/);
  const header = html.match(/<h3[^>]*>/)?.[0] ?? '';
  assert.doesNotMatch(header, /onclick/i);
  assert.doesNotMatch(header, /tabindex/i);
  assert.doesNotMatch(header, /role="button"/);
});
