import assert from 'node:assert/strict';
import test from 'node:test';

import { formatMessage, messages, supportedLocales, type Locale } from '../src/index.ts';

// Ported verbatim from frontend/src/i18n/locales/*.ts: knowledgeBase.*,
// knowledgeEditor.*, wikiBrowser.* (hoisted out of knowledgeEditor) and
// faqManager.* ported as FAQ.*. Every locale must carry the same key set per
// domain; values are byte-exact against the Vue baseline.

function keysWithPrefix(locale: Locale, prefix: string): string[] {
  return Object.keys(messages[locale]).filter((key) => key.startsWith(prefix)).sort();
}

function assertDomainParity(prefix: string, minimum: number): string[] {
  const reference = keysWithPrefix('en-US', prefix);
  assert.ok(reference.length >= minimum, prefix + ' should port at least ' + minimum + ' keys, found ' + reference.length);
  for (const locale of supportedLocales) {
    assert.deepEqual(keysWithPrefix(locale, prefix), reference, locale + ' diverges from en-US for ' + prefix);
  }
  return reference;
}

test('knowledgeBase domain keys are present in every locale', () => {
  assertDomainParity('knowledgeBase.', 300);
});

test('knowledgeEditor domain keys are present in every locale', () => {
  assertDomainParity('knowledgeEditor.', 200);
});

test('wikiBrowser domain keys are hoisted and present in every locale', () => {
  const keys = assertDomainParity('wikiBrowser.', 100);
  for (const key of keys) assert.ok(!key.startsWith('knowledgeEditor.wikiBrowser.'), key);
});

test('FAQ domain keys are ported from the faqManager block', () => {
  assertDomainParity('FAQ.', 10);
});

test('React supplemental keys exist in every locale with placeholders aligned', () => {
  const reference = keysWithPrefix('en-US', 'knowledgeBase.documents.');
  assert.ok(reference.length >= 30);
  for (const locale of supportedLocales) {
    assert.deepEqual(keysWithPrefix(locale, 'knowledgeBase.documents.'), reference);
  }
  const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
  for (const key of reference) {
    const expected = placeholders(messages['en-US'][key]!);
    for (const locale of supportedLocales) assert.deepEqual(placeholders(messages[locale][key]!), expected, locale + ' placeholder mismatch for ' + key);
  }
});

// Documents-page render snapshot (zh-CN): every UI string on the documents
// page flows through formatMessage; the zh-CN value must not be raw English.
const documentsPageKeys = [
  'knowledgeBase.documents.title', 'knowledgeBase.documents.subtitle', 'knowledgeBase.documents.reload',
  'knowledgeBase.documents.tryAgain', 'knowledgeBase.documents.folders', 'knowledgeBase.documents.loadingFolders',
  'knowledgeBase.documents.search', 'knowledgeBase.documents.searchPlaceholder', 'knowledgeBase.documents.status',
  'knowledgeBase.documents.allStatuses', 'knowledgeBase.documents.tag', 'knowledgeBase.documents.allTags',
  'knowledgeBase.documents.selectedOnPage', 'knowledgeBase.documents.selectedTotal', 'knowledgeBase.documents.reparse',
  'knowledgeBase.documents.cancelParse', 'knowledgeBase.documents.move', 'knowledgeBase.documents.setTags',
  'knowledgeBase.documents.delete', 'knowledgeBase.documents.moveDestination', 'knowledgeBase.documents.moveRoot',
  'knowledgeBase.documents.moveConfirm', 'knowledgeBase.documents.moveCancel', 'knowledgeBase.documents.page',
  'knowledgeBase.documents.previous', 'knowledgeBase.documents.next', 'knowledgeBase.documents.source',
  'knowledgeBase.documents.sourceFile', 'knowledgeBase.documents.sourceManual', 'knowledgeBase.documents.file',
  'knowledgeBase.documents.url', 'knowledgeBase.documents.manualTitle', 'knowledgeBase.documents.manualContent',
  'knowledgeBase.documents.uploadFile', 'knowledgeBase.documents.importUrl', 'knowledgeBase.documents.createDocument',
  'knowledgeBase.documents.cancel', 'knowledgeBase.documents.statusUnknown', 'knowledgeBase.documents.loadingDocuments',
  'knowledgeBase.documents.noDocuments', 'knowledgeBase.documents.root',
  'knowledgeBase.documents.tabDocuments', 'knowledgeBase.documents.tabWiki', 'knowledgeBase.documents.tabGraph',
  'knowledgeBase.documents.detail', 'knowledgeBase.timeline.title', 'knowledgeBase.timeline.pending',
  'knowledgeBase.timeline.running', 'knowledgeBase.timeline.done', 'knowledgeBase.timeline.failed',
  'knowledgeBase.timeline.skipped',
  'knowledgeBase.detail.back', 'knowledgeBase.detail.eyebrow',
];

const allowedLatinValues = new Set(['knowledgeBase.documents.url', 'knowledgeBase.documents.sourceUrl', 'knowledgeBase.documents.tabWiki']);

test('documents page zh-CN render contains no raw-English literals', () => {
  for (const key of documentsPageKeys) {
    const value = formatMessage('zh-CN', key);
    assert.notEqual(value, key, 'missing zh-CN value for ' + key);
    if (allowedLatinValues.has(key)) continue;
    assert.ok(/[\u4e00-\u9fff]/.test(value), 'zh-CN value for ' + key + ' looks like raw English: ' + value);
  }
});

// R472-A1 (R470 遗留): a disabled multimodal stage closes as status
// 'skipped' with started_at/finished_at timestamps. Vue renders it through
// knowledgeStages.status.skipped (frontend/src/i18n/locales/*.ts); the React
// timeline key knowledgeBase.timeline.skipped carries the same copy
// byte-exact in every locale so the stage reads 已跳过 instead of 进行中.
test('knowledgeBase.timeline.skipped is carried byte-exact from Vue knowledgeStages.status.skipped in every locale', () => {
  const vueBaseline: Record<Locale, string> = {
    'zh-CN': '已跳过',
    'en-US': 'Skipped',
    'ja-JP': 'スキップ済み',
    'ko-KR': '건너뜀',
    'ru-RU': 'Пропущено',
  };
  for (const locale of supportedLocales) {
    assert.equal(formatMessage(locale, 'knowledgeBase.timeline.skipped'), vueBaseline[locale], locale + ' timeline.skipped diverges from the Vue baseline');
  }
});
