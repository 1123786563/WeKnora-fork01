// SP11 Task 9 fix round 1 — chart label keys resolve in every locale.
// AnalyticsPage composes recharts Line/Bar legend names from the wire data
// fields (queries/likes/dislikes, messages/unique_users), but i18n leaf keys
// follow the package's camelCase convention (analytics.uniqueUsers). The
// mapping lives in chart-series.ts and this test pins the contract: every
// labelKey the page actually consumes must resolve in all five locales
// without falling back to the raw key (the bug: unique_users used to be
// composed verbatim and rendered "analytics.unique_users" in the legend).
import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, supportedLocales } from '@weknora/i18n';
import { AGENT_SERIES, TREND_SERIES } from './chart-series.ts';

test('chart series label keys resolve in every locale without raw-key fallback', () => {
  const series = [...TREND_SERIES, ...AGENT_SERIES];
  assert.equal(series.length, 5);
  // The wire field unique_users must map onto the camelCase i18n leaf key.
  assert.deepEqual(
    AGENT_SERIES.map((entry) => [entry.dataKey, entry.labelKey]),
    [['messages', 'analytics.messages'], ['unique_users', 'analytics.uniqueUsers']],
  );
  for (const locale of supportedLocales) {
    for (const entry of series) {
      const resolved = formatMessage(locale, entry.labelKey);
      assert.notEqual(resolved, entry.labelKey, `${locale} ${entry.labelKey} fell back to the raw key`);
      assert.ok(resolved.length > 0, `${locale} ${entry.labelKey} resolved to an empty string`);
    }
  }
});
