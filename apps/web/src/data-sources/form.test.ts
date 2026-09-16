import assert from 'node:assert/strict';
import test from 'node:test';
import { buildDataSourceInput, credentialsRequiredForValidation, firstMissingRequiredCredential, parseCredentialLines } from './form.ts';

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

// Vue DataSourceEditorDialog.validateStep1Fields: before the connection test and
// submit, every non-optional connector credential field must be present; the
// first missing one is reported by its localized label
// (MessagePlugin.warning(`${t(labelKey)} ${t('datasource.isRequired')}`)).
test('flags the first missing required credential field before submit (Vue validateStep1Fields)', () => {
  assert.equal(firstMissingRequiredCredential('feishu', 'app_id = cli_x'), 'dataSource.field.appSecret');
  assert.equal(firstMissingRequiredCredential('feishu', 'app_id = cli_x\napp_secret = s'), null, 'base_url is optional in Vue');
  assert.equal(firstMissingRequiredCredential('notion', ''), 'dataSource.field.integrationToken');
  assert.equal(firstMissingRequiredCredential('gitlab', 'base_url = https://gitlab.example.com'), 'dataSource.field.apiToken');
});

test('skips credential field validation for connectors without a Vue field map', () => {
  assert.equal(firstMissingRequiredCredential('rss', ''), null, 'rss uses feed-url settings, not credential fields');
  assert.equal(firstMissingRequiredCredential('', ''), null);
});

// Vue DataSourceEditorDialog credentialsRequired: the per-field validation walk
// only runs when credentials are actually required — an edit of an already
// configured connector without typed replacements skips it (the stored
// credentials travel on the request as-is).
test('runs the required-credential walk only when Vue credentialsRequired holds', () => {
  assert.equal(credentialsRequiredForValidation({ isEdit: false, credentialsConfigured: false, replacementTyped: false }), true, 'create always validates');
  assert.equal(credentialsRequiredForValidation({ isEdit: true, credentialsConfigured: true, replacementTyped: false }), false, 'edit of a configured connector without replacements skips validation');
  assert.equal(credentialsRequiredForValidation({ isEdit: true, credentialsConfigured: true, replacementTyped: true }), true, 'typed replacement re-enables validation');
  assert.equal(credentialsRequiredForValidation({ isEdit: true, credentialsConfigured: false, replacementTyped: false }), true, 'unconfigured connector validates');
  assert.equal(credentialsRequiredForValidation({ isEdit: true, credentialsConfigured: false, replacementTyped: true }), true);
});
