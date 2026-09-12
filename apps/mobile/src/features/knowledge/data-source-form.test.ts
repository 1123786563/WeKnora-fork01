import assert from 'node:assert/strict';
import test from 'node:test';
import { buildNativeDataSourceInput, nativeDataSourceDraftFrom, parseNativeKeyValueLines, type NativeDataSourceDraft } from './data-source-form.ts';

const draft: NativeDataSourceDraft = {
  name: 'Docs', type: 'notion', schedule: '0 0 */6 * * *', mode: 'incremental', conflict: 'overwrite', deletions: true,
  credentialsText: 'token = secret', settingsText: 'workspace_id = ws-1',
};

test('native data-source form validates key-value fields and builds a knowledge-base input', () => {
  assert.deepEqual(parseNativeKeyValueLines('token = secret\ncount = 2'), { token: 'secret', count: '2' });
  assert.deepEqual(buildNativeDataSourceInput(draft, 'kb-1'), {
    name: 'Docs', type: 'notion', knowledge_base_id: 'kb-1', sync_schedule: '0 0 */6 * * *', sync_mode: 'incremental',
    conflict_strategy: 'overwrite', sync_deletions: true,
    config: { credentials: { token: 'secret' }, settings: { workspace_id: 'ws-1' } },
  });
});

test('editing a native data source never prefills server credentials', () => {
  const form = nativeDataSourceDraftFrom({
    id: 'source-1', knowledge_base_id: 'kb-1', name: 'Docs', type: 'notion', sync_schedule: '', sync_mode: 'full',
    conflict_strategy: 'skip', sync_deletions: false, config: { credentials: { token: 'server-secret' }, settings: { workspace_id: 'ws-1' } },
  });
  assert.equal(form.credentialsText, '');
  assert.equal(form.settingsText, 'workspace_id = ws-1');
});

test('native data-source form rejects blank names, types, and malformed lines before network writes', () => {
  assert.throws(() => buildNativeDataSourceInput({ ...draft, name: ' ' }, 'kb-1'), /name is required/);
  assert.throws(() => buildNativeDataSourceInput({ ...draft, type: '' }, 'kb-1'), /type is required/);
  assert.throws(() => parseNativeKeyValueLines('token'), /key=value/);
  assert.throws(() => parseNativeKeyValueLines('token = secret\ntoken = other'), /duplicated/);
});
