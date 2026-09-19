import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

// @weknora/ui pulls in theme.css; node:test needs the same short-circuit as
// the other settings panel tests (CloudSettingsPanel.test.tsx).
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

import type { UsageRow } from '@weknora/contracts';
const { aggregateByModel, usageTotals } = await import('./UsagePanel.tsx');

function row(overrides: Partial<UsageRow> & Pick<UsageRow, 'model'>): UsageRow {
  return {
    window_start: '2026-09-01T00:00:00Z',
    input_tokens: 0,
    output_tokens: 0,
    cache_read_tokens: 0,
    cache_write_tokens: 0,
    cost_microcredits: 0,
    ...overrides,
  };
}

test('aggregateByModel sums windows of the same model and merges cache read+write', () => {
  const items: UsageRow[] = [
    row({ model: 'gpt-test', window_start: '2026-09-01T00:00:00Z', input_tokens: 100, output_tokens: 40, cache_read_tokens: 10, cache_write_tokens: 5, cost_microcredits: 700 }),
    row({ model: 'gpt-test', window_start: '2026-09-02T00:00:00Z', input_tokens: 50, output_tokens: 60, cache_read_tokens: 20, cache_write_tokens: 15, cost_microcredits: 300 }),
    row({ model: 'bge-m3', window_start: '2026-09-01T00:00:00Z', input_tokens: 7, output_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 3, cost_microcredits: 1 }),
  ];
  assert.deepEqual(aggregateByModel(items), [
    { model: 'gpt-test', input: 150, output: 100, cache: 50, cost: 1000 },
    { model: 'bge-m3', input: 7, output: 0, cache: 3, cost: 1 },
  ]);
});

test('aggregateByModel keeps first-appearance model order', () => {
  const items: UsageRow[] = [
    row({ model: 'zeta' }),
    row({ model: 'alpha' }),
    row({ model: 'zeta', input_tokens: 1 }),
  ];
  assert.deepEqual(aggregateByModel(items).map((item) => item.model), ['zeta', 'alpha']);
  assert.equal(aggregateByModel(items)[0]!.input, 1);
});

test('aggregateByModel returns an empty list for empty input', () => {
  assert.deepEqual(aggregateByModel([]), []);
});

test('usageTotals sums every aggregate column into the footer row', () => {
  const rows = aggregateByModel([
    row({ model: 'gpt-test', input_tokens: 100, output_tokens: 40, cache_read_tokens: 10, cache_write_tokens: 5, cost_microcredits: 700 }),
    row({ model: 'bge-m3', input_tokens: 7, cache_write_tokens: 3, cost_microcredits: 1 }),
  ]);
  assert.deepEqual(usageTotals(rows), { model: '', input: 107, output: 40, cache: 18, cost: 701 });
});

test('usageTotals zeroes out when no models were used', () => {
  assert.deepEqual(usageTotals([]), { model: '', input: 0, output: 0, cache: 0, cost: 0 });
});
