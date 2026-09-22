// SP12 Task 8 — analytics 用量 tab（/platform/analytics?tab=usage）。
// The web suite runs under `node --import tsx --test`; these tests stay
// DOM-free (css imports resolve to an empty module via the resolve hook) and
// pin the pure helper the by-user table relies on: cost-descending, stable,
// non-mutating sort.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const { isUnattributedUsageRow, sortUsageRows } = await import('./AnalyticsPage.tsx');
import type { UsageByUserRow } from '@weknora/contracts';

// byUser rows are flat (one per user × model × window); only the fields the
// sort touches need to be realistic.
function row(user_id: string, model: string, cost_microcredits: number): UsageByUserRow {
  return {
    user_id, model, cost_microcredits,
    window_start: '2026-09-01T00:00:00Z',
    input_tokens: 0, output_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 0,
  };
}

test('sortUsageRows orders by cost_microcredits descending', () => {
  const sorted = sortUsageRows([row('u1', 'glm-4.7', 10), row('u2', 'glm-4.7', 900), row('u3', 'qwen', 250)]);
  assert.deepEqual(sorted.map((item) => item.user_id), ['u2', 'u3', 'u1']);
});

test('sortUsageRows is stable — equal-cost rows keep their input order', () => {
  // Array#sort stability is the contract the table's "same cost, server
  // order" presentation depends on (guaranteed since ES2019).
  const sorted = sortUsageRows([
    row('a', 'm1', 5), row('b', 'm1', 100), row('c', 'm2', 5), row('d', 'm3', 100), row('e', 'm1', 5),
  ]);
  assert.deepEqual(sorted.map((item) => item.user_id), ['b', 'd', 'a', 'c', 'e']);
});

test('sortUsageRows returns a new array and leaves the input untouched', () => {
  const input = [row('u1', 'm1', 1), row('u2', 'm2', 30), row('u3', 'm3', 7)];
  const snapshot = [...input];
  const sorted = sortUsageRows(input);
  assert.notEqual(sorted, input);
  assert.deepEqual(input, snapshot);
});

test('sortUsageRows handles empty and single-row pages', () => {
  assert.deepEqual(sortUsageRows([]), []);
  const only = row('solo', 'm1', 42);
  assert.deepEqual(sortUsageRows([only]), [only]);
});

// SP12 终审 — the craft fold attributes orphan facts to the empty user
// (craft_usage.go COALESCE fallback), so by-user pages can carry user_id:''
// rows; the table must show them under the System placeholder (not crash —
// the contract no longer rejects them either).
test('isUnattributedUsageRow flags only the empty user_id bucket', () => {
  assert.equal(isUnattributedUsageRow(row('', 'glm-4.7', 10)), true);
  assert.equal(isUnattributedUsageRow(row('u1', 'glm-4.7', 10)), false);
});

test('sortUsageRows keeps empty-user_id rows alongside attributed ones', () => {
  const sorted = sortUsageRows([row('u1', 'm1', 5), row('', 'm1', 100)]);
  assert.deepEqual(sorted.map((item) => item.user_id), ['', 'u1']);
});
