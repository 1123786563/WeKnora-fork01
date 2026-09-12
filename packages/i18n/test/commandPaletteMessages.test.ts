import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, messages, supportedLocales } from '../src/index.ts';
import { commandPaletteMessages } from '../src/generated/commandPalette.ts';

// GlobalCommandPalette (⌘K) copy must exist in every supported locale so the
// React port (apps/web/src/platform) has full parity with the Vue source
// (frontend/src/i18n/locales/*.ts commandPalette block) for the keys it uses.
const SAMPLE_KEYS = [
  'commandPalette.placeholder',
  'commandPalette.clearRecent',
  'commandPalette.group.recent',
  'commandPalette.group.quickActions',
  'commandPalette.group.commands',
  'commandPalette.quick.newChat',
  'commandPalette.quick.knowledgeBases',
  'commandPalette.quick.agents',
  'commandPalette.quick.organizations',
  'commandPalette.quick.settings',
  'commandPalette.empty.noResults',
  'commandPalette.hotkey.esc',
];

test('commandPalette message keys exist in every supported locale', () => {
  for (const locale of supportedLocales) {
    for (const key of SAMPLE_KEYS) {
      assert.notEqual(formatMessage(locale, key), key, `${locale} missing ${key}`);
    }
  }
});

test('commandPalette key sets are identical across locales', () => {
  const keySets = supportedLocales.map((locale) => Object.keys(commandPaletteMessages[locale]).sort());
  for (let i = 1; i < keySets.length; i++) {
    assert.deepEqual(keySets[i], keySets[0]);
  }
});

test('commandPalette messages are merged into the combined messages table', () => {
  for (const locale of supportedLocales) {
    for (const key of SAMPLE_KEYS) {
      assert.equal(messages[locale][key], commandPaletteMessages[locale][key]);
    }
  }
});

test('en-US quick action labels match the Vue baseline extraction', () => {
  assert.equal(formatMessage('en-US', 'commandPalette.quick.newChat'), 'New conversation');
  assert.equal(formatMessage('en-US', 'commandPalette.quick.knowledgeBases'), 'Open knowledge bases');
  assert.equal(formatMessage('zh-CN', 'commandPalette.quick.newChat'), '新建对话');
});
