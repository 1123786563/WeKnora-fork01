import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import React from 'react';

import { ChatComposer, resolveChatCopy, type ChatSteerQueueChip } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

/*
 * R473-A2 — Vue Input-field.vue steer queue strip (.steer-queue, ~2599): while
 * a steer-capable turn runs, every queued after-message renders as a chip
 * above the composer textarea with the waiting clock icon, the (truncated,
 * title-tooltipped) message text, and per-state actions:
 *   - failed  → retry button (input.steerRetry)
 *   - pending → loading marker, actions disabled (Vue steer-sending)
 *   - queued  → send-now/promote (input.steerQueueSendNow) + remove
 *               (common.remove)
 */

function renderQueue(queue: readonly ChatSteerQueueChip[]): string {
  return renderToStaticMarkup(React.createElement(ChatComposer, {
    draft: '',
    onDraftChange: () => undefined,
    onSubmit: () => undefined,
    streaming: true,
    canSteer: true,
    steerQueue: queue,
    onSteerPromote: () => undefined,
    onSteerRemove: () => undefined,
    onSteerRetry: () => undefined,
    copy: resolveChatCopy('zh-CN'),
  }));
}

test('queued steer chips render above the composer with waiting, send-now and remove affordances', () => {
  const html = renderQueue([{ steerId: 'steer-1', content: '回答后补充这一点', status: 'queued' }]);
  const strip = /<ul[^>]*wk-chat-steer-queue[^>]*role="list"[^>]*>/.exec(html);
  assert.ok(strip, 'steer queue strip with role=list must render');
  assert.match(html, /aria-label="当前回答结束后发送"/);
  assert.match(html, /data-steer-id="steer-1"/);
  // Full text stays reachable through the title attribute (Vue :title="item.content").
  assert.match(html, /title="回答后补充这一点"/);
  assert.match(html, /aria-label="补充当前任务"/);
  assert.match(html, /aria-label="移除"/);
});

test('a pending steer chip shows the loading marker instead of actions', () => {
  const html = renderQueue([{ steerId: 'steer-2', content: '排队中', status: 'pending' }]);
  assert.match(html, /data-steer-status="pending"/);
  assert.match(html, /aria-label="加载中/);
  assert.doesNotMatch(html, /aria-label="补充当前任务"/);
});

test('a failed steer chip swaps the actions for the retry button', () => {
  const html = renderQueue([{ steerId: 'steer-3', content: '失败了', status: 'failed' }]);
  assert.match(html, /data-steer-status="failed"/);
  assert.match(html, /aria-label="重试发送"/);
  assert.doesNotMatch(html, /aria-label="补充当前任务"/);
  assert.doesNotMatch(html, /aria-label="移除"/);
});

test('an empty steer queue renders no strip', () => {
  const html = renderQueue([]);
  assert.doesNotMatch(html, /wk-chat-steer-queue/);
});

test('the steer queue strip is localized in every shipped locale', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const copy = resolveChatCopy(locale);
    assert.ok(copy.steerQueueWaiting, `steerQueueWaiting missing for ${locale}`);
    assert.ok(copy.steerQueueSendNow, `steerQueueSendNow missing for ${locale}`);
    assert.ok(copy.steerRetry, `steerRetry missing for ${locale}`);
    assert.ok(copy.remove, `remove missing for ${locale}`);
  }
  assert.equal(resolveChatCopy('zh-CN').steerQueueWaiting, '当前回答结束后发送');
  assert.equal(resolveChatCopy('en-US').steerQueueWaiting, 'Will send after the current answer finishes');
  assert.equal(resolveChatCopy('ja-JP').steerQueueSendNow, '現在のタスクに追加');
  assert.equal(resolveChatCopy('ko-KR').steerRetry, '다시 보내기');
  assert.equal(resolveChatCopy('ru-RU').remove, 'Удалить');
});

/*
 * R474-A2 — Vue Input-field.vue ~2610: only the first promotable chip
 * advertises the ⌘Enter/Alt+Enter inject shortcut in its send-now tooltip
 * (steerShortcutLabel), same suffix style as the Vue t-tooltip content.
 */
test('the first promotable chip tooltip advertises the ⌘Enter/Alt+Enter shortcut', () => {
  const html = renderQueue([
    { steerId: 'steer-a', content: '第一条', status: 'pending' },
    { steerId: 'steer-b', content: '第二条', status: 'queued' },
    { steerId: 'steer-c', content: '第三条', status: 'queued' },
  ]);
  const tooltips = [...html.matchAll(/title="(补充当前任务[^"]*)"/g)].map((match) => match[1]);
  assert.equal(tooltips.length, 2, 'both queued chips render a send-now tooltip');
  assert.match(tooltips[0]!, / · (⌘ Enter|Alt\+Enter)$/, 'the first promotable chip carries the shortcut suffix');
  assert.doesNotMatch(tooltips[1]!, / · (⌘ Enter|Alt\+Enter)$/, 'later chips do not advertise the shortcut');
});
