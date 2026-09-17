import assert from 'node:assert/strict';
import test from 'node:test';
import { buildDataSourceInput, credentialStepKind, credentialStepReducer, credentialsRequiredForValidation, dataSourceFormFrom, firstMissingRequiredCredential, initialCredentialStepState, parseCredentialLines, serializeAuthHeaders, VUE_CREDENTIAL_FIELDS, VUE_SETTINGS_FIELDS } from './form.ts';
import type { DataSource } from '@weknora/api-client';

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
  assert.equal(firstMissingRequiredCredential('gitlab', 'base_url = https://gitlab.example.com'), 'dataSource.gitlab.accessToken', 'gitlab uses its own Vue label keys');
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

// Vue DataSourceEditorDialog step-1 credential branches: create always shows
// the inputs; a configured edit shows the "configured" faux row until the user
// opts in to Replace; an unconfigured edit shows the "not configured" row whose
// Configure action reveals the inputs (degenerate replace mode).
test('derives the Vue credential step branch (configured / unconfigured / inputs)', () => {
  const kind = (isEdit: boolean, credentialsConfigured: boolean, replaceMode: boolean) => credentialStepKind({ isEdit, credentialsConfigured, replaceMode });
  assert.equal(kind(false, false, false), 'inputs', 'create always renders inputs');
  assert.equal(kind(true, true, false), 'configured');
  assert.equal(kind(true, true, true), 'inputs', 'replace mode reveals editable inputs');
  assert.equal(kind(true, false, false), 'unconfigured');
  assert.equal(kind(true, false, true), 'inputs');
});

// Vue enterReplaceCredentials / cancelReplaceCredentials / request- and
// cancelRemoveCredentials / confirmRemoveCredentials / commitCredentialsIfNeeded
// as a pure reducer. cancel-replace discards anything typed; remove-confirmed
// falls back to the unconfigured row; replace-committed collapses back to the
// configured row with the new credentials stored server-side.
test('Reducer reproduces the Vue replace/remove credential state machine', () => {
  const start = initialCredentialStepState(true);
  assert.deepEqual(start, { credentialsConfigured: true, replaceMode: false, pendingRemove: false });
  assert.deepEqual(credentialStepReducer(start, 'enter-replace'), { credentialsConfigured: true, replaceMode: true, pendingRemove: false }, 'entering replace cancels a pending remove prompt');
  assert.deepEqual(credentialStepReducer({ ...start, replaceMode: true }, 'cancel-replace'), { credentialsConfigured: true, replaceMode: false, pendingRemove: false });
  assert.deepEqual(credentialStepReducer(start, 'request-remove'), { credentialsConfigured: true, replaceMode: false, pendingRemove: true });
  assert.deepEqual(credentialStepReducer({ ...start, pendingRemove: true }, 'cancel-remove'), { credentialsConfigured: true, replaceMode: false, pendingRemove: false });
  assert.deepEqual(credentialStepReducer({ ...start, pendingRemove: true, replaceMode: true }, 'remove-confirmed'), { credentialsConfigured: false, replaceMode: false, pendingRemove: false }, 'after removal the row falls back to unconfigured');
  assert.deepEqual(credentialStepReducer({ ...start, replaceMode: true }, 'replace-committed'), { credentialsConfigured: true, replaceMode: false, pendingRemove: false }, 'a committed replacement collapses back to the configured row');
});

// Vue connectorDefs is the source of truth for labels, placeholders and hints.
// The React field map must stay byte-compatible: lark reuses the Feishu
// base_url placeholder, gitlab carries its own label keys, base_url fields
// render the shared baseUrlHint, and the rss feed_urls setting keeps its hint.
test('field map stays byte-compatible with the Vue connectorDefs', () => {
  const baseUrl = (type: string) => VUE_CREDENTIAL_FIELDS[type]!.find((field) => field.key === 'base_url');
  assert.equal(baseUrl('lark')!.placeholder, 'https://open.feishu.cn');
  for (const type of ['feishu', 'lark', 'feishu_drive', 'lark_drive', 'yuque', 'ima']) {
    assert.equal(baseUrl(type)!.hint, 'dataSource.field.baseUrlHint', `${type} base_url carries the shared hint`);
  }
  const gitlab = Object.fromEntries(VUE_CREDENTIAL_FIELDS.gitlab!.map((field) => [field.key, field]));
  assert.equal(gitlab.base_url!.label, 'dataSource.gitlab.baseUrl');
  assert.equal(gitlab.access_token!.label, 'dataSource.gitlab.accessToken');
  assert.equal(VUE_SETTINGS_FIELDS.rss!.find((field) => field.key === 'feed_urls')!.hint, 'dataSource.field.feedUrlsHint');
});

// Vue DataSourceEditorDialog connectorDefs: the rss connector's only credential
// field is auth_headers with fieldType 'custom_headers', edited as key-value
// rows and serialized by serializeAuthHeaders ("Key: Value" lines, empty-key
// rows dropped, keys trimmed) into config.credentials.auth_headers.
test('serializes rss auth header rows the way Vue serializeAuthHeaders does', () => {
  assert.equal(serializeAuthHeaders([{ key: ' Authorization ', value: 'Bearer x' }, { key: '   ', value: 'dropped' }, { key: 'X-Trace', value: '1' }]), 'Authorization: Bearer x\nX-Trace: 1');
  assert.equal(serializeAuthHeaders([]), '');
  assert.equal(serializeAuthHeaders([{ key: 'k', value: '' }]), 'k: ');
});

// Vue syncRssAuthHeadersToCredentials writes the serialized rows into
// form.config.credentials.auth_headers for rss only; other connectors never
// grow an auth_headers key and empty rows leave credentials untouched.
test('rss header rows flow into config.credentials.auth_headers on build', () => {
  const base = { name: 'Feeds', schedule: '0 0 */6 * * *', mode: 'incremental' as const, conflict: 'overwrite' as const, deletions: true, settingsText: '', resourceIds: [] };
  const rss = buildDataSourceInput({ ...base, type: 'rss', credentialsText: '', authHeaders: [{ key: 'Authorization', value: 'Bearer x' }] });
  assert.deepEqual((rss.config as Record<string, unknown>).credentials, { auth_headers: 'Authorization: Bearer x' });
  const rssEmpty = buildDataSourceInput({ ...base, type: 'rss', credentialsText: '', authHeaders: [] });
  assert.deepEqual((rssEmpty.config as Record<string, unknown>).credentials, {});
  const notion = buildDataSourceInput({ ...base, type: 'notion', credentialsText: '', authHeaders: [{ key: 'Authorization', value: 'Bearer x' }] });
  assert.deepEqual((notion.config as Record<string, unknown>).credentials, {}, 'non-rss connectors never leak header rows into credentials');
});

// Edit hydration mirrors Vue: rssAuthHeaders resets to [] when the dialog
// opens, so stored auth_headers stay server-side unless the user opts in to
// Replace and retypes rows.
test('dataSourceFormFrom opens the rss header editor with empty rows', () => {
  const source = { id: 'ds-1', knowledge_base_id: 'kb-1', name: 'Feeds', type: 'rss', config: { credentials: { auth_headers: 'Authorization: Bearer x' }, settings: {}, resource_ids: [] } } as DataSource;
  assert.deepEqual(dataSourceFormFrom(source).authHeaders, []);
  const plain = { id: 'ds-2', knowledge_base_id: 'kb-1', name: 'Notion', type: 'notion' } as DataSource;
  assert.deepEqual(dataSourceFormFrom(plain).authHeaders, []);
});
