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
  'commandPalette.hotkey.select',
  'commandPalette.hotkey.enter',
  'commandPalette.hotkey.cmdNumber',
  'commandPalette.hotkey.cmdEnter',
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

test('hotkey hints are byte-exact against the Vue locale baseline', () => {
  // Vue GlobalCommandPalette.vue:142-150 footer (frontend/src/i18n/locales
  // commandPalette.hotkey block, all five locales).
  const expected: Record<string, Record<string, string>> = {
    'zh-CN': { select: '选择', enter: '打开', cmdNumber: '直接打开', cmdEnter: '发起对话', esc: '关闭' },
    'en-US': { select: 'Navigate', enter: 'Open', cmdNumber: 'Jump to', cmdEnter: 'Start chat', esc: 'Close' },
    'ja-JP': { select: '移動', enter: '開く', cmdNumber: '直接開く', cmdEnter: 'チャットを開始', esc: '閉じる' },
    'ko-KR': { select: '이동', enter: '열기', cmdNumber: '바로 이동', cmdEnter: '대화 시작', esc: '닫기' },
    'ru-RU': { select: 'Навигация', enter: 'Открыть', cmdNumber: 'Быстрый переход', cmdEnter: 'Начать диалог', esc: 'Закрыть' },
  };
  for (const locale of supportedLocales) {
    for (const [suffix, value] of Object.entries(expected[locale])) {
      assert.equal(formatMessage(locale, 'commandPalette.hotkey.' + suffix), value, locale + ' hotkey.' + suffix);
    }
  }
});

test('en-US quick action labels match the Vue baseline extraction', () => {
  assert.equal(formatMessage('en-US', 'commandPalette.quick.newChat'), 'New conversation');
  assert.equal(formatMessage('en-US', 'commandPalette.quick.knowledgeBases'), 'Open knowledge bases');
  assert.equal(formatMessage('zh-CN', 'commandPalette.quick.newChat'), '新建对话');
});
