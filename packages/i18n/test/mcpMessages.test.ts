import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, mcpMessages, supportedLocales } from '../src/index.ts';

test('MCP messages have the same key set in every supported locale', () => {
  const expected = Object.keys(mcpMessages['zh-CN']).sort();
  for (const locale of supportedLocales) {
    assert.deepEqual(Object.keys(mcpMessages[locale]).sort(), expected, locale);
    for (const key of expected) assert.notEqual(formatMessage(locale, key), key, `${locale}:${key}`);
  }
});
