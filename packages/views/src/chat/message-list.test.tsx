import assert from 'node:assert/strict';
import test from 'node:test';

import { isBookmarkActionAvailable, writeClipboardText } from './message-list.tsx';

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
