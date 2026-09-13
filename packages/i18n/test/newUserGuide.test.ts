import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, messages, supportedLocales } from '../src/index.ts';

// newUserGuide block (packages/i18n/src/generated/newUserGuide.ts) — byte-exact
// port of the Vue welcome-tour copy (frontend/src/i18n/locales/*.ts). The views
// package keeps its own local table (views cannot depend on @weknora/i18n);
// this suite pins the shared bundle to the same key set so both stay in sync.
test('newUserGuide keys are consistent across all locales and step keys resolve', () => {
  const localeKeySets = supportedLocales.map((locale) => Object.keys(messages[locale]).filter((key) => key.startsWith('newUserGuide.')));
  for (const keySet of localeKeySets) assert.ok(keySet.length >= 20, `newUserGuide key count too small: ${keySet.length}`);
  for (let i = 1; i < localeKeySets.length; i += 1) {
    assert.deepEqual([...localeKeySets[i]].sort(), [...localeKeySets[0]].sort(), `locale ${supportedLocales[i]} key set drift`);
  }
  const zh = formatMessage('zh-CN', 'newUserGuide.skip');
  assert.equal(zh, '跳过引导');
  const en = formatMessage('en-US', 'newUserGuide.skip');
  assert.equal(en, 'Skip');
  const stepOf = formatMessage('zh-CN', 'newUserGuide.stepOf', { current: 1, total: 7 });
  assert.equal(stepOf, '1 / 7');
});
