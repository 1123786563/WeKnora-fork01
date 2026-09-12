import assert from 'node:assert/strict';
import test from 'node:test';

import { writeClipboardText } from './message-list.tsx';

// Rendering assertions for MessageList (separators, timestamps, copy button,
// typing indicator, scroll-to-bottom) live in apps/web/src/chat/chat-page.test.tsx,
// the only package in the workspace with react-dom available for SSR.

test('writeClipboardText prefers the async clipboard API', async () => {
  const seen: string[] = [];
  await writeClipboardText('copied text', { writeText: async (t) => { seen.push(t); } });
  assert.deepEqual(seen, ['copied text']);
});
