import { test } from 'node:test';
import assert from 'node:assert/strict';
import { formatMessage, messages, supportedLocales } from '../src/index.ts';

// Representative keys ported from the Vue settings surface (general/retrieval/
// tenant/user profile). Full parity is asserted per-section as panels land.
// general.allSettings (R448-A2) guards the user-menu catch-all entry copy —
// its presence across all five locales keeps the key-set guard meaningful.
const SAMPLE_KEYS = ['general.title', 'general.allSettings', 'settings.retrieval.title', 'tenant.name', 'user.password.oldPassword', 'settings.title'];

test('settings messages are present across locales for sampled keys that exist', () => {
  let checked = 0;
  for (const locale of supportedLocales) {
    for (const key of SAMPLE_KEYS) {
      if (messages[locale][key] === undefined) continue;
      assert.notEqual(formatMessage(locale, key), key);
      checked++;
    }
  }
  assert.ok(checked > 0, 'no sampled settings keys present');
});

test('settings key sets are identical across locales', () => {
  const keySets = supportedLocales.map((locale) => Object.keys(messages[locale]).sort());
  for (let i = 1; i < keySets.length; i++) assert.deepEqual(keySets[i], keySets[0]);
});
