import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, isLocale, messages, supportedLocales } from '../src/index.ts';

test('keeps the five locales present in the current source inventory aligned', () => {
  assert.deepEqual([...supportedLocales].sort(), ['en-US', 'ja-JP', 'ko-KR', 'ru-RU', 'zh-CN'].sort());
  for (const locale of supportedLocales) assert.ok(messages[locale]['auth.login']);
});

test('formats interpolation and falls back to English/key without Vue runtime', () => {
  assert.equal(formatMessage('en-US', 'missing', { name: 'x' }), 'missing');
  // Vue baseline (frontend/src/i18n/locales/en-US.ts auth.login) is authoritative.
  assert.equal(formatMessage('en-US', 'auth.login'), 'Login');
  assert.equal(isLocale('fr-FR'), false);
});

test('keeps the React foundation keys and placeholders aligned across every locale', () => {
  const requiredKeys = ['auth.login', 'auth.email', 'auth.password', 'common.loading', 'common.retry', 'common.cancel', 'common.workspaceRequired', 'common.itemCount'];
  for (const locale of supportedLocales) {
    for (const key of requiredKeys) assert.ok(messages[locale][key], `${locale} is missing ${key}`);
  }
  assert.equal(formatMessage('en-US', 'common.itemCount', { count: 3 }), '3 items');
  assert.equal(formatMessage('zh-CN', 'common.itemCount', { count: 3 }), '3 项');
  const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
  for (const key of requiredKeys) {
    const expected = placeholders(messages['en-US'][key]);
    for (const locale of supportedLocales) assert.deepEqual(placeholders(messages[locale][key]), expected, `${locale} placeholder mismatch for ${key}`);
  }
});

// R455 A1: the knowledge-settings overview tiles (activity/datasource/share/
// graph) are a React-side addition with no Vue nav copy to port, so the
// kbSettings.summary.* keys are fresh translations carried by all five
// locales. The count guard pins the inventory size; the byte checks keep the
// en-US fallback copy identical to the pre-R455 hardcoded strings.
test('keeps the kbSettings.summary tile keys aligned across every locale', () => {
  const summaryKeys = [
    'kbSettings.summary.activity.countOne', 'kbSettings.summary.activity.countOther',
    'kbSettings.summary.activity.inspect', 'kbSettings.summary.activity.emptyLabel',
    'kbSettings.summary.activity.emptyDetail', 'kbSettings.summary.activity.sectionEmpty',
    'kbSettings.summary.datasource.countOne', 'kbSettings.summary.datasource.countOther',
    'kbSettings.summary.datasource.inspect', 'kbSettings.summary.datasource.emptyLabel',
    'kbSettings.summary.datasource.emptyDetail', 'kbSettings.summary.datasource.sectionEmpty',
    'kbSettings.summary.share.countOne', 'kbSettings.summary.share.countOther',
    'kbSettings.summary.share.managed', 'kbSettings.summary.share.emptyLabel',
    'kbSettings.summary.share.emptyDetail', 'kbSettings.summary.share.sectionEmpty',
    'kbSettings.summary.graph.enabledLabel', 'kbSettings.summary.graph.enabledDetail',
    'kbSettings.summary.graph.disabledLabel', 'kbSettings.summary.graph.disabledDetail',
  ];
  assert.equal(summaryKeys.length, 22);
  const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
  for (const locale of supportedLocales) {
    const localeSummaryKeys = Object.keys(messages[locale]).filter((key) => key.startsWith('kbSettings.summary.'));
    assert.equal(localeSummaryKeys.length, 22, `${locale} kbSettings.summary key count`);
    for (const key of summaryKeys) {
      assert.ok(messages[locale][key], `${locale} is missing ${key}`);
      assert.notEqual(messages[locale][key], key, `${locale} ${key} resolved to the raw key`);
    }
    for (const key of summaryKeys.filter((candidate) => candidate.endsWith('countOne') || candidate.endsWith('countOther'))) {
      assert.deepEqual(placeholders(messages[locale][key]), ['count'], `${locale} ${key} must interpolate {count}`);
    }
    for (const key of summaryKeys.filter((candidate) => !candidate.includes('count'))) {
      assert.deepEqual(placeholders(messages[locale][key]), [], `${locale} ${key} must not carry placeholders`);
    }
  }
  // en-US reuses the pre-R455 hardcoded English byte-for-byte.
  assert.equal(formatMessage('en-US', 'kbSettings.summary.datasource.emptyLabel'), 'No data sources');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.datasource.emptyDetail'), 'Add an external connector');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.share.emptyLabel'), 'Not shared');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.graph.enabledLabel'), 'Knowledge graph enabled');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.activity.sectionEmpty'), 'No recorded changes for this knowledge base.');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.datasource.countOther', { count: 2 }), '2 data sources');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.datasource.countOther', { count: 2 }), '2 个数据源');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.share.emptyLabel'), '未共享');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.graph.enabledLabel'), '知识图谱已启用');
});
