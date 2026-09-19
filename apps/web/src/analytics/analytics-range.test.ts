// SP11 Task 9 — analytics date-range helpers (pure functions, node:test).
// The web suite runs under `node --import tsx --test`, so these tests stay
// DOM-free and pin the three behaviours the dashboard relies on: the default
// 30-day UTC window, the inverted/invalid fallback, and the 366-day cap.
import assert from 'node:assert/strict';
import test from 'node:test';
import { clampAnalyticsRange, defaultAnalyticsRange, toISODate } from './analytics-range.ts';

test('defaultAnalyticsRange spans 30 days ending at the current UTC day', () => {
  // A mid-day timestamp must still clamp to the UTC day boundary (the
  // analytics endpoints bucket by UTC date, so a partial day would skew the
  // last column of every chart).
  const now = new Date('2026-09-19T15:42:11Z');
  const range = defaultAnalyticsRange(now);
  assert.equal(range.endTime, '2026-09-19');
  assert.equal(range.startTime, '2026-08-20');
  assert.equal(toISODate(new Date('2026-01-02T00:00:00Z')), '2026-01-02');
});

test('clampAnalyticsRange falls back to the default window when bounds are inverted or invalid', () => {
  const now = new Date('2026-09-19T23:59:59Z');
  // Inverted: startTime after endTime.
  assert.deepEqual(clampAnalyticsRange('2026-09-19', '2026-08-20', now), defaultAnalyticsRange(now));
  // Malformed dates must not reach the query string either.
  assert.deepEqual(clampAnalyticsRange('nonsense', '2026-08-20', now), defaultAnalyticsRange(now));
  assert.deepEqual(clampAnalyticsRange('2026-08-20', '', now), defaultAnalyticsRange(now));
});

test('clampAnalyticsRange truncates spans longer than 366 days, keeping endTime anchored', () => {
  const clamped = clampAnalyticsRange('2020-01-01', '2026-09-19');
  assert.equal(clamped.endTime, '2026-09-19');
  // 366 days before 2026-09-19 (2026 is not a leap year).
  assert.equal(clamped.startTime, '2025-09-18');
  // A legal span passes through untouched.
  assert.deepEqual(clampAnalyticsRange('2026-08-20', '2026-09-19'), { startTime: '2026-08-20', endTime: '2026-09-19' });
});
