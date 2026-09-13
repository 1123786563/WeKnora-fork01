import assert from 'node:assert/strict';
import test from 'node:test';
import { buildDataSourceInput, parseCredentialLines } from './form.ts';

test('parses connector credentials from key-value lines and rejects malformed secrets', () => {
  assert.deepEqual(parseCredentialLines('app_id = demo\napp_secret = hidden\n'), { app_id: 'demo', app_secret: 'hidden' });
  assert.throws(() => parseCredentialLines('missing-separator'), /key=value/i);
});

test('builds an explicit connector payload without serializing a generic JSON editor', () => {
  assert.deepEqual(buildDataSourceInput({ name: 'Docs', type: 'notion', schedule: '0 0 */6 * * *', mode: 'incremental', conflict: 'overwrite', deletions: true, credentialsText: 'token = abc', settingsText: 'workspace_id = ws-1', resourceIds: ['space-1'] }), {
    name: 'Docs', type: 'notion', sync_schedule: '0 0 */6 * * *', sync_mode: 'incremental', conflict_strategy: 'overwrite', sync_deletions: true,
    config: { credentials: { token: 'abc' }, settings: { workspace_id: 'ws-1' }, resource_ids: ['space-1'] },
  });
});
