import assert from 'node:assert/strict';
import test from 'node:test';
import { isLocale, loadingLabel, supportedLocales } from '../src/runtime.ts';

test('bootstrap runtime exposes the complete locale set', () => {
  assert.deepEqual(supportedLocales, ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU']);
  for (const locale of supportedLocales) assert.equal(isLocale(locale), true);
  assert.equal(isLocale('fr-FR'), false);
});

test('bootstrap loading copy is localized without the full catalog', () => {
  assert.equal(loadingLabel('zh-CN'), '加载中…');
  assert.equal(loadingLabel('en-US'), 'Loading…');
  assert.equal(loadingLabel('ja-JP'), '読み込み中…');
  assert.equal(loadingLabel('ko-KR'), '로드 중…');
  assert.equal(loadingLabel('ru-RU'), 'Загрузка…');
});
