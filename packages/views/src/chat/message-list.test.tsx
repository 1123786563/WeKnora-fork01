import assert from 'node:assert/strict';
import test from 'node:test';

import { assistantTimelineItems, isBookmarkActionAvailable, writeClipboardText } from './message-list.tsx';
import type { ChatMessage } from '@weknora/contracts';

// Rendering assertions for MessageList (separators, timestamps, copy button,
// typing indicator, scroll-to-bottom) live in the web chat integration suite;
// this package test keeps the host-owned bookmark contract local to the view.

test('writeClipboardText prefers the async clipboard API', async () => {
  const seen: string[] = [];
  await writeClipboardText('copied text', { writeText: async (t) => { seen.push(t); } });
  assert.deepEqual(seen, ['copied text']);
});

test('assistant bookmark action follows the host-owned Vue manual-editor capability', () => {
  assert.equal(isBookmarkActionAvailable(), false);
  assert.equal(isBookmarkActionAvailable(() => undefined), true);
});

/*
 * Vue main-face contract (R464): a plain chat turn's reasoning stream is never
 * rendered as a fold block. frontend/src/views/chat/components/botmsg.vue only
 * renders thinking through deepThink (`<think>` tags parsed out of content)
 * or the agent-mode AgentStreamDisplay timeline; the `thinking` field and the
 * persisted `agent_steps[].reasoning_content/thought` text that
 * assistantMessageExtras() collects are NOT displayed on the Vue main face,
 * so the simplified React timeline drops the reasoning item too.
 */
test('assistant timeline keeps Vue tool order and finish node without a reasoning item', () => {
  const message = {
    id: 'assistant-1',
    session_id: 'session-1',
    role: 'assistant',
    content: 'done',
    is_completed: true,
    thinking: 'plan',
    tool_calls: [
      { id: 'tool-1', name: 'search_docs', status: 'completed' },
      { id: 'tool-2', name: 'write_file', status: 'failed' },
    ],
  } as ChatMessage;

  assert.deepEqual(assistantTimelineItems(message), [
    { kind: 'tool', id: 'tool-1', name: 'search_docs', status: 'completed' },
    { kind: 'tool', id: 'tool-2', name: 'write_file', status: 'failed' },
    { kind: 'finish' },
  ]);
  assert.deepEqual(assistantTimelineItems({ ...message, is_completed: false }), [
    { kind: 'tool', id: 'tool-1', name: 'search_docs', status: 'completed' },
    { kind: 'tool', id: 'tool-2', name: 'write_file', status: 'failed' },
  ]);
});

test('reasoning-only assistant data renders no timeline at all like the Vue main face', () => {
  const message = {
    id: 'assistant-1',
    session_id: 'session-1',
    role: 'assistant',
    content: 'done',
    is_completed: true,
    thinking: 'plan',
    agent_steps: [{ iteration: 0, reasoning_content: 'plan the search', thought: 'more reasoning' }],
  } as unknown as ChatMessage;
  assert.deepEqual(assistantTimelineItems(message), []);
});
