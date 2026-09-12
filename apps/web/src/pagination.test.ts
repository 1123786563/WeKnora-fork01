import assert from 'node:assert/strict';
import test from 'node:test';

import { pagerState, slicePage } from './pagination.ts';

test('60 entries paginate: page 2 shows entries 51-60', () => {
  const entries = Array.from({ length: 60 }, (_value, index) => index + 1);
  const state = pagerState(entries.length, 2, 50);
  assert.equal(state.start, 51);
  assert.equal(state.end, 60);
  assert.equal(state.hasPrevious, true);
  assert.equal(state.hasNext, false);
  assert.deepEqual(slicePage(entries, 2, 50), entries.slice(50));
});

test('page 1 of 60 offers a next page; clamps out-of-range pages', () => {
  const state = pagerState(60, 1, 50);
  assert.equal(state.start, 1);
  assert.equal(state.end, 50);
  assert.equal(state.hasNext, true);
  assert.equal(pagerState(60, 9, 50).page, 2);
  assert.equal(pagerState(60, 0, 50).page, 1);
  assert.deepEqual(pagerState(0, 1, 50), { page: 1, pageSize: 50, total: 0, start: 0, end: 0, hasPrevious: false, hasNext: false });
});
