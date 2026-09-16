import assert from 'node:assert/strict';
import test from 'node:test';

import { applyWikiSearch, saveWikiPage, validateWikiPageInput, wikiSaveState } from './editor.ts';

test('only applies the Wiki search query on submit and trims empty clears', () => {
  assert.deepEqual(applyWikiSearch('  architecture  '), { draft: '  architecture  ', keyword: 'architecture' });
  assert.deepEqual(applyWikiSearch('   '), { draft: '   ', keyword: '' });
});

test('preserves the server version on Wiki saves and reports conflicts separately', async () => {
  let captured: unknown;
  const result = await saveWikiPage({
    update: async (_kb, _slug, input) => { captured = input; throw Object.assign(new Error('stale version'), { status: 409 }); },
  }, 'kb-1', 'docs/start', { title: 'Start', content: 'New body', summary: 'Intro', version: 3 });
  assert.deepEqual(captured, { title: 'Start', content: 'New body', summary: 'Intro', version: 3 });
  assert.deepEqual(result, { status: 'conflict', message: 'This page changed elsewhere. Reload the latest version before saving.' });
});

test('rejects blank Wiki content before issuing a write', async () => {
  let called = false;
  const result = await saveWikiPage({ update: async () => { called = true; throw new Error('unexpected'); } }, 'kb-1', 'start', { title: ' ', content: 'body', version: 1 });
  assert.equal(called, false);
  assert.deepEqual(result, { status: 'error', message: 'Wiki title is required' });
});

test('uses caller-provided localized copy for validation and conflicts', async () => {
  const copy = { titleRequired: '请输入标题', contentRequired: '请输入正文', conflict: '页面已更新', saveFailed: '保存失败' };
  const blank = await saveWikiPage({ update: async () => { throw new Error('unexpected'); } }, 'kb-1', 'start', { title: '', content: 'body', version: 1 }, copy);
  assert.deepEqual(blank, { status: 'error', message: '请输入标题' });
  const conflict = await saveWikiPage({ update: async () => { throw Object.assign(new Error('stale'), { status: 409 }); } }, 'kb-1', 'start', { title: '标题', content: '正文', version: 1 }, copy);
  assert.deepEqual(conflict, { status: 'conflict', message: '页面已更新' });
});

test('validates Wiki create input before issuing a write', () => {
  const copy = { titleRequired: '请输入标题', contentRequired: '请输入正文', conflict: '页面已更新', saveFailed: '保存失败' };
  assert.equal(validateWikiPageInput({ title: '  ', content: '正文' }, copy), '请输入标题');
  assert.equal(validateWikiPageInput({ title: '标题', content: '  ' }, copy), '请输入正文');
  assert.equal(validateWikiPageInput({ title: '标题', content: '正文' }, copy), null);
});

test('keeps loading and error states explicit instead of returning an empty Wiki list', () => {
  assert.deepEqual(wikiSaveState(new Error('403 forbidden')), { status: 'error', message: '403 forbidden' });
});
