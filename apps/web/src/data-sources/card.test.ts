import assert from 'node:assert/strict';
import test from 'node:test';
import { createTranslator } from '../i18n.ts';
import { humanizeCron, relativeTime, syncResultPills } from './card.ts';
import type { DataSource } from '@weknora/api-client';

const zh = createTranslator('zh-CN');
const en = createTranslator('en-US');

// Port of frontend/src/utils/cronHumanize.ts CRON_PRESET_MAP: known presets
// resolve to i18n keys whose values are byte-exact against the Vue baseline.
test('humanizeCron maps known cron presets to Vue-equal natural language', () => {
  assert.equal(humanizeCron('0 */30 * * * *', zh), '每 30 分钟');
  assert.equal(humanizeCron('0 0 * * * *', zh), '每小时');
  assert.equal(humanizeCron('0 0 */6 * * *', zh), '每 6 小时');
  assert.equal(humanizeCron('0 0 */12 * * *', zh), '每 12 小时');
  assert.equal(humanizeCron('0 0 2 * * *', zh), '每天');
  // Vue en-US scheduleHuman.1h is 'Hourly' on the card (schedule1h form label
  // is 'Every hour') — the card keys stay byte-exact with the card strings.
  assert.equal(humanizeCron('0 0 * * * *', en), 'Hourly');
  assert.equal(humanizeCron('0 0 */6 * * *', en), 'Every 6 hours');
});

test('humanizeCron falls back to the raw cron expression and empty to --', () => {
  assert.equal(humanizeCron('17 3 * * 1', zh), '17 3 * * 1');
  assert.equal(humanizeCron('', zh), '--');
  assert.equal(humanizeCron(undefined, zh), '--');
});

// Port of frontend/src/utils/cronHumanize.ts relativeTime buckets.
const NOW = Date.parse('2026-09-16T12:00:00Z');
const minutesAgo = (n: number) => new Date(NOW - n * 60000).toISOString();

test('relativeTime buckets match the Vue never/just/minutes/hours/days rules', () => {
  assert.equal(relativeTime(null, zh, NOW), '未同步');
  assert.equal(relativeTime('not-a-date', zh, NOW), '未同步');
  assert.equal(relativeTime(minutesAgo(0.5), zh, NOW), '刚刚', 'within 60s counts as just now');
  assert.equal(relativeTime(minutesAgo(3), zh, NOW), '3 分钟前');
  assert.equal(relativeTime(minutesAgo(120), zh, NOW), '2 小时前');
  assert.equal(relativeTime(minutesAgo(60 * 23.5), zh, NOW), '23 小时前', 'hours still win under 24h');
  assert.equal(relativeTime(minutesAgo(60 * 24 * 3), zh, NOW), '3 天前');
  assert.equal(relativeTime(minutesAgo(60 * 24 * 40), en, NOW), new Date(minutesAgo(60 * 24 * 40)).toLocaleDateString(), 'older than 30 days falls back to the locale date');
});

// Port of Vue DataSourceSettings.vue syncResultPills: only non-zero metrics
// become pills with +N / ~N / -N prefixes and labelled failed/skipped counts.
test('syncResultPills renders only non-zero metrics with Vue prefixes', () => {
  const source = (log: DataSource['latest_sync_log']): DataSource => ({ id: 'ds1', knowledge_base_id: 'kb', name: 'n', type: 'rss', latest_sync_log: log });
  assert.deepEqual(syncResultPills(source(undefined), zh), []);
  assert.deepEqual(syncResultPills(source({ id: 'l1', status: 'success' }), zh), []);
  const pills = syncResultPills(source({ id: 'l2', status: 'partial', items_created: 2, items_updated: 1, items_deleted: 3, items_failed: 4, items_skipped: 5 }), zh);
  assert.deepEqual(pills.map((pill) => pill.text), ['+2', '~1', '-3', '4 失败', '5 跳过']);
  assert.deepEqual(pills.map((pill) => pill.kind), ['created', 'updated', 'deleted', 'failed', 'skipped']);
});
