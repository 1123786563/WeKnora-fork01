import assert from 'node:assert/strict';
import test from 'node:test';

import { formatMessage, messages, supportedLocales, type Locale } from '../src/index.ts';

// Ported exactly from frontend/src/i18n/locales/*.ts -> knowledgeList block.
// Every locale must carry the same key set so a missing translation falls
// through to the key name instead of a mixed-language page.
function knowledgeListKeys(locale: Locale): string[] {
  return Object.keys(messages[locale]).filter((key) => key.startsWith('knowledgeList.'));
}

test('every locale carries the knowledgeList keys', () => {
  for (const locale of supportedLocales) {
    const keys = knowledgeListKeys(locale);
    assert.ok(keys.length >= 50, `${locale} should port the knowledgeList block`);
    assert.ok(keys.includes('knowledgeList.create'), `${locale} missing create`);
    assert.ok(keys.includes('knowledgeList.delete.confirmMessage'), `${locale} missing delete.confirmMessage`);
    assert.ok(keys.includes('knowledgeList.empty.title'), `${locale} missing empty.title`);
  }
});

test('all locales share an identical knowledgeList key set', () => {
  const reference = knowledgeListKeys('en-US').sort();
  for (const locale of supportedLocales) {
    assert.deepEqual(knowledgeListKeys(locale).sort(), reference, `${locale} diverges from en-US keys`);
  }
});

test('delete confirmation message interpolates the knowledge base name', () => {
  assert.equal(
    formatMessage('en-US', 'knowledgeList.delete.confirmMessage', { name: 'Docs' }),
    'Are you sure you want to delete the knowledge base "Docs"? This action cannot be undone.',
  );
  assert.equal(
    formatMessage('zh-CN', 'knowledgeList.delete.confirmMessage', { name: '文档库' }),
    '确认要删除知识库"文档库"？删除后不可恢复',
  );
});

test('uninitialized banner and create label exist in all locales', () => {
  assert.equal(formatMessage('en-US', 'knowledgeList.create'), 'Create Knowledge Base');
  assert.ok(formatMessage('ja-JP', 'knowledgeList.uninitializedBanner').length > 0);
  assert.ok(formatMessage('ko-KR', 'knowledgeList.uninitializedBanner').length > 0);
  assert.ok(formatMessage('ru-RU', 'knowledgeList.uninitializedBanner').length > 0);
});
