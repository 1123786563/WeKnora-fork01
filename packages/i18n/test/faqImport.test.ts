import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, supportedLocales } from '../src/index.ts';
import { faqImportMessages } from '../src/generated/faqImport.ts';

// Vue faqManager.import block (FAQEntryManager import strip states) — the
// React FAQ import strip consumes exactly these four keys in five locales.
const EXPECTED: Record<string, Record<string, string>> = {
  'zh-CN': { importing: '导入中...', importDone: '导入完成', importFailed: '导入失败', waiting: '等待中...' },
  'en-US': { importing: 'Importing...', importDone: 'Import Complete', importFailed: 'Import Failed', waiting: 'Waiting...' },
  'ja-JP': { importing: 'インポート中...', importDone: 'インポート完了', importFailed: 'インポート失敗', waiting: '待機中...' },
  'ko-KR': { importing: '가져오기 중...', importDone: '가져오기 완료', importFailed: '가져오기 실패', waiting: '대기 중...' },
  'ru-RU': { importing: 'Импорт...', importDone: 'Импорт завершён', importFailed: 'Ошибка импорта', waiting: 'Ожидание...' },
};

test('faqManager.import.* keys are byte-exact in every locale and merged', () => {
  for (const locale of supportedLocales) {
    for (const [suffix, value] of Object.entries(EXPECTED[locale])) {
      const key = 'faqManager.import.' + suffix;
      assert.equal(faqImportMessages[locale][key], value, locale + ' ' + key + ' table drift');
      assert.equal(formatMessage(locale, key), value, locale + ' ' + key + ' merged drift');
    }
  }
});
