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

test('assistant timeline preserves Vue step order and adds a finish node only after completion', () => {
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
    { kind: 'thinking', text: 'plan' },
    { kind: 'tool', id: 'tool-1', name: 'search_docs', status: 'completed' },
    { kind: 'tool', id: 'tool-2', name: 'write_file', status: 'failed' },
    { kind: 'finish' },
  ]);
  assert.deepEqual(assistantTimelineItems({ ...message, is_completed: false }), [
    { kind: 'thinking', text: 'plan' },
    { kind: 'tool', id: 'tool-1', name: 'search_docs', status: 'completed' },
    { kind: 'tool', id: 'tool-2', name: 'write_file', status: 'failed' },
  ]);
});
