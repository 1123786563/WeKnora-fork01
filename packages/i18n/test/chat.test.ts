import assert from 'node:assert/strict';
import test from 'node:test';

import { chatMessages, formatMessage, messages, supportedLocales, type Locale } from '../src/index.ts';
import zhVue from '../../../frontend/src/i18n/locales/zh-CN.ts';
import enVue from '../../../frontend/src/i18n/locales/en-US.ts';
import jaVue from '../../../frontend/src/i18n/locales/ja-JP.ts';
import koVue from '../../../frontend/src/i18n/locales/ko-KR.ts';
import ruVue from '../../../frontend/src/i18n/locales/ru-RU.ts';

// Chat domain (packages/i18n/src/generated/chat.ts) — the web chat keys are a
// byte-exact port of the Vue chat copy (frontend/src/i18n/locales/*.ts) that
// the React chat rendering layer consumes (packages/views/src/chat/chat-copy.ts
// mirrors it locally). The mobileChat keys are native-client copy owned by the
// shared bundle; Vue does not expose that surface, so it is validated for
// locale completeness separately below.

const VUE_BASELINE: Record<Locale, Record<string, unknown>> = {
  'zh-CN': zhVue,
  'en-US': enVue,
  'ja-JP': jaVue,
  'ko-KR': koVue,
  'ru-RU': ruVue,
};

function vueGet(obj: Record<string, unknown>, path: string): unknown {
  let current: unknown = obj;
  for (const part of path.split('.')) {
    if (current == null || typeof current !== 'object') return undefined;
    current = (current as Record<string, unknown>)[part];
  }
  return current;
}

test('chat domain keys are consistent across all locales', () => {
  const localeKeySets = supportedLocales.map((locale) => Object.keys(messages[locale]).filter((key) => key.startsWith('chat.') || key.startsWith('chatHeader.') || key.startsWith('createChat.title') || key.startsWith('time.')));
  for (const keySet of localeKeySets) assert.ok(keySet.length >= 25, `chat key count too small: ${keySet.length}`);
  for (let i = 1; i < localeKeySets.length; i += 1) {
    assert.deepEqual([...localeKeySets[i]].sort(), [...localeKeySets[0]].sort(), `locale ${supportedLocales[i]} chat key set drift`);
  }
});

test('chatMessages carries exactly the generated key set per locale', () => {
  const reference = Object.keys(chatMessages['zh-CN']).sort();
  assert.ok(reference.length >= 30, `chatMessages key count too small: ${reference.length}`);
  for (const locale of supportedLocales) {
    assert.deepEqual(Object.keys(chatMessages[locale]).sort(), reference, `chatMessages key drift for ${locale}`);
  }
});

test('web chat values are byte-exact against the Vue locale baseline', () => {
  for (const locale of supportedLocales) {
    for (const [key, value] of Object.entries(chatMessages[locale]).filter(([key]) => !key.startsWith('mobileChat.'))) {
      const fromVue = vueGet(VUE_BASELINE[locale], key);
      assert.equal(typeof fromVue, 'string', `Vue baseline lacks ${key} for ${locale}`);
      assert.equal(value, fromVue, `value drift for ${key} (${locale})`);
    }
  }
});

test('mobile chat copy is complete and non-empty in every locale', () => {
  const mobileKeys = Object.keys(chatMessages['zh-CN']).filter((key) => key.startsWith('mobileChat.')).sort();
  assert.ok(mobileKeys.length >= 30, `mobileChat key count too small: ${mobileKeys.length}`);
  for (const locale of supportedLocales) {
    assert.deepEqual(
      Object.keys(chatMessages[locale]).filter((key) => key.startsWith('mobileChat.')).sort(),
      mobileKeys,
      `mobileChat key drift for ${locale}`,
    );
    for (const key of mobileKeys) {
      assert.equal(typeof chatMessages[locale][key], 'string', `${key} must be a string for ${locale}`);
      assert.ok(chatMessages[locale][key].trim().length > 0, `${key} must not be empty for ${locale}`);
    }
  }
});

test('chatMessages does not duplicate domains that already exist in the bundle', () => {
  for (const locale of supportedLocales) {
    for (const key of Object.keys(chatMessages[locale])) {
      assert.ok(!key.startsWith('menu.'), `menu.* lives in menu.ts, not chat.ts: ${key}`);
      assert.ok(!key.startsWith('agent.'), `agent.* lives in generated/agent.ts, not chat.ts: ${key}`);
      assert.ok(!key.startsWith('common.'), `common.* lives in the base table, not chat.ts: ${key}`);
    }
  }
});

test('chat copy resolves with interpolation through formatMessage', () => {
  assert.equal(formatMessage('zh-CN', 'chat.conversationTime.thisYear', { month: 3, day: 5, time: '14:00' }), '3月5日 14:00');
  assert.equal(formatMessage('en-US', 'chat.conversationTime.thisYear', { month: 3, day: 5, time: '14:00' }), '3/5 14:00');
  assert.equal(formatMessage('zh-CN', 'chat.conversationTime.otherYear', { year: 2024, month: 3, day: 5, time: '09:30' }), '2024年3月5日 09:30');
  assert.equal(formatMessage('ko-KR', 'chat.sandbox.start'), '터미널 시작');
  assert.equal(formatMessage('ru-RU', 'chatHeader.moreActions'), 'Другие действия с диалогом');
  assert.equal(formatMessage('ja-JP', 'createChat.title'), 'こんにちは、WeKnoraです。あなたのナレッジを、すぐそばに');
});
