import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, messages, supportedLocales } from '../src/index.ts';

test('data-source log messages exist in every supported locale', () => {
  const keys = Object.keys(messages['en-US']).filter((key) => key.startsWith('dataSource.')).sort();
  assert.equal(keys.length, 19);
  for (const locale of supportedLocales) {
    assert.deepEqual(Object.keys(messages[locale]).filter((key) => key.startsWith('dataSource.')).sort(), keys, `${locale} data-source messages diverge`);
    assert.notEqual(formatMessage(locale, 'dataSource.syncHistory'), 'dataSource.syncHistory');
    assert.notEqual(formatMessage(locale, 'dataSource.status.running'), 'dataSource.status.running');
  }
});
