import assert from 'node:assert/strict';
import test from 'node:test';

import { saveWikiPage, wikiSaveState } from './editor.ts';

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

test('keeps loading and error states explicit instead of returning an empty Wiki list', () => {
  assert.deepEqual(wikiSaveState(new Error('403 forbidden')), { status: 'error', message: '403 forbidden' });
});
