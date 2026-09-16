import assert from 'node:assert/strict';
import test from 'node:test';

import { formatMessage, messages, newUserGuideMessages, supportedLocales, type Locale } from '../src/index.ts';
import zhVue from '../../../frontend/src/i18n/locales/zh-CN.ts';
import enVue from '../../../frontend/src/i18n/locales/en-US.ts';
import jaVue from '../../../frontend/src/i18n/locales/ja-JP.ts';
import koVue from '../../../frontend/src/i18n/locales/ko-KR.ts';
import ruVue from '../../../frontend/src/i18n/locales/ru-RU.ts';

// newUserGuide block (packages/i18n/src/generated/newUserGuide.ts) — byte-exact
// port of the Vue welcome-tour copy (frontend/src/i18n/locales/*.ts, newUserGuide
// block: stepOf/skip/prev/next/done/reopen + steps.{welcome,knowledge,agents,
// chat,settings,models,done}.{title,desc} = 20 keys x 5 locales). The views
// package keeps its own local table (packages/views cannot depend on
// @weknora/i18n; guides/steps.ts resolves the shared bundle first and falls
// back locally); this suite pins the shared bundle to the Vue baseline so both
// stay byte-identical.

const VUE_BASELINE: Record<Locale, Record<string, unknown>> = {
  'zh-CN': zhVue,
  'en-US': enVue,
  'ja-JP': jaVue,
  'ko-KR': koVue,
  'ru-RU': ruVue,
};

const EXPECTED_LEAF_KEYS = [
  'stepOf', 'skip', 'prev', 'next', 'done', 'reopen',
  ...['welcome', 'knowledge', 'agents', 'chat', 'settings', 'models', 'done'].flatMap((step) => [`steps.${step}.title`, `steps.${step}.desc`]),
].sort();

function vueGet(obj: Record<string, unknown>, path: string): unknown {
  let current: unknown = obj;
  for (const part of path.split('.')) {
    if (current == null || typeof current !== 'object') return undefined;
    current = (current as Record<string, unknown>)[part];
  }
  return current;
}

test('newUserGuide carries exactly the 20 Vue keys in every locale', () => {
  for (const locale of supportedLocales) {
    const keys = Object.keys(newUserGuideMessages[locale]).sort();
    assert.equal(keys.length, 20, `newUserGuide key count for ${locale}`);
    assert.deepEqual(keys, EXPECTED_LEAF_KEYS.map((leaf) => `newUserGuide.${leaf}`), `key set drift for ${locale}`);
  }
});

test('newUserGuide values are byte-exact against the Vue locale baseline', () => {
  for (const locale of supportedLocales) {
    for (const [key, value] of Object.entries(newUserGuideMessages[locale])) {
      const fromVue = vueGet(VUE_BASELINE[locale], key);
      assert.equal(typeof fromVue, 'string', `Vue baseline lacks ${key} for ${locale}`);
      assert.equal(value, fromVue, `value drift for ${key} (${locale})`);
    }
  }
});

test('newUserGuide keys are merged into the shared messages bundle with no locale drift', () => {
  const localeKeySets = supportedLocales.map((locale) => Object.keys(messages[locale]).filter((key) => key.startsWith('newUserGuide.')));
  for (let i = 1; i < localeKeySets.length; i += 1) {
    assert.deepEqual([...localeKeySets[i]].sort(), [...localeKeySets[0]].sort(), `merged bundle drift for ${supportedLocales[i]}`);
  }
});

test('newUserGuide copy resolves through formatMessage (incl. the reopen entry label)', () => {
  assert.equal(formatMessage('zh-CN', 'newUserGuide.reopen'), '新手引导');
  assert.equal(formatMessage('en-US', 'newUserGuide.reopen'), 'Product tour');
  assert.equal(formatMessage('ja-JP', 'newUserGuide.reopen'), 'プロダクトツアー');
  assert.equal(formatMessage('ko-KR', 'newUserGuide.reopen'), '사용 가이드');
  assert.equal(formatMessage('ru-RU', 'newUserGuide.reopen'), 'Обучение');
  assert.equal(formatMessage('zh-CN', 'newUserGuide.stepOf', { current: 1, total: 7 }), '1 / 7');
  assert.equal(formatMessage('ja-JP', 'newUserGuide.steps.welcome.title'), 'WeKnoraへようこそ');
});
