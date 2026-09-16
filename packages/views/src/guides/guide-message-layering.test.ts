// Layering tests for guides/steps.ts guideMessage (R007 guides closeout).
//
// packages/views cannot depend on the @weknora/i18n package name, so — like
// packages/views/src/integrations/messages.ts — the helper imports
// formatMessage relatively from the workspace source and resolves the shared
// bundle FIRST; the local byte-exact table (NEW_USER_GUIDE_MESSAGES) stays as
// the fallback. The shared newUserGuide.* block lives in
// packages/i18n/src/generated/newUserGuide.ts.
import assert from 'node:assert/strict';
import test from 'node:test';

import { formatMessage, messages, supportedLocales, type Locale } from '../../../i18n/src/index.ts';
import { guideMessage, NEW_USER_GUIDE_MESSAGES, type NewUserGuideLocale } from './steps.ts';

test('guideMessage prefers the shared i18n bundle over the local fallback table', () => {
  const locale: NewUserGuideLocale = 'zh-CN';
  const key = 'newUserGuide.skip';
  const original = messages[locale][key];
  try {
    messages[locale][key] = '共享包影子值';
    assert.equal(guideMessage(locale, key), '共享包影子值', 'shared bundle value must shadow the local table');
    assert.equal(guideMessage(locale, key, {}), '共享包影子值', 'values-path also resolves from the shared bundle');
  } finally {
    messages[locale][key] = original!;
  }
});

test('guideMessage falls back to the local table when the shared bundle lacks the key', () => {
  const locale: NewUserGuideLocale = 'en-US';
  const key = 'newUserGuide.steps.done.desc';
  const original = messages[locale][key];
  try {
    delete (messages[locale] as Record<string, string>)[key];
    assert.equal(guideMessage(locale, key), NEW_USER_GUIDE_MESSAGES[locale][key], 'local fallback serves the key');
  } finally {
    (messages[locale] as Record<string, string>)[key] = original!;
  }
});

test('guideMessage stays byte-equal to formatMessage for every key and locale', () => {
  for (const locale of supportedLocales) {
    for (const key of Object.keys(NEW_USER_GUIDE_MESSAGES[locale as NewUserGuideLocale]) as (keyof (typeof NEW_USER_GUIDE_MESSAGES)['zh-CN'])[]) {
      assert.equal(guideMessage(locale as NewUserGuideLocale, key), formatMessage(locale as Locale, key), `drift for ${key} (${locale})`);
    }
  }
  assert.equal(guideMessage('zh-CN', 'newUserGuide.stepOf', { current: 2, total: 7 }), '2 / 7');
  assert.equal(guideMessage('ru-RU', 'newUserGuide.reopen'), 'Обучение');
});

test('the views local table and the shared bundle stay byte-identical', () => {
  for (const locale of supportedLocales) {
    for (const [key, value] of Object.entries(NEW_USER_GUIDE_MESSAGES[locale as NewUserGuideLocale])) {
      assert.equal((messages[locale as Locale] as Record<string, string>)[key], value, `table drift for ${key} (${locale})`);
    }
  }
});
