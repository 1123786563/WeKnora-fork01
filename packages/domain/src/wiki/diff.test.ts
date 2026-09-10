import assert from 'node:assert/strict';
import test from 'node:test';

import { diffWikiLines, diffWikiRevision } from './diff.ts';

test('preserves common lines and emits deletion before insertion', () => {
  assert.deepEqual(diffWikiLines('a\nold\nc', 'a\nnew\nc'), [
    { type: 'same', text: 'a' }, { type: 'del', text: 'old' }, { type: 'add', text: 'new' }, { type: 'same', text: 'c' },
  ]);
});

test('uses a coarse diff above the LCS limit without losing lines', () => {
  const oldText = Array.from({ length: 1501 }, (_, i) => `old-${i}`).join('\n');
  const newText = Array.from({ length: 1501 }, (_, i) => `new-${i}`).join('\n');
  const result = diffWikiLines(oldText, newText);
  assert.equal(result.filter((line) => line.type === 'del').length, 1501);
  assert.equal(result.filter((line) => line.type === 'add').length, 1501);
});

test('keeps revision fields in title-summary-content order', () => {
  const result = diffWikiRevision(
    { title: 'a', summary: 'old', content: 'body' },
    { title: 'b', summary: 'new', content: 'body' },
  );
  assert.deepEqual(result.map((section) => section.field), ['title', 'summary']);
});
