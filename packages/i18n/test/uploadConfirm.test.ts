import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, messages, supportedLocales } from '../src/index.ts';

// uploadConfirm block (packages/i18n/src/generated/uploadConfirm.ts) — byte-exact
// port of the Vue upload-confirmation + graphSettings copy migrated from the
// former apps/web local table (apps/web/src/documents/upload-pipeline.ts).
// Every locale must carry the same key set so the dialog never falls through
// to a mixed-language page.
test('uploadConfirm keys are consistent across all locales and resolve', () => {
  const localeKeySets = supportedLocales.map((locale) => Object.keys(messages[locale]).filter((key) => key.startsWith('uploadConfirm.') || key.startsWith('graphSettings.')));
  for (const keySet of localeKeySets) assert.ok(keySet.length >= 90, `uploadConfirm key count too small: ${keySet.length}`);
  for (let i = 1; i < localeKeySets.length; i += 1) {
    assert.deepEqual([...localeKeySets[i]].sort(), [...localeKeySets[0]].sort(), `locale ${supportedLocales[i]} key set drift`);
  }
  assert.equal(formatMessage('zh-CN', 'uploadConfirm.title'), '上传文档确认');
  assert.equal(formatMessage('zh-CN', 'graphSettings.title'), '知识图谱配置');
  assert.equal(formatMessage('en-US', 'graphSettings.enableLabel'), 'Enable Entity-Relationship Extraction');
  assert.equal(formatMessage('zh-CN', 'uploadConfirm.confirm'), '确认上传并解析');
});
