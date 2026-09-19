import { strict as assert } from 'node:assert';
import { test } from 'node:test';
import { VUE_CREDENTIAL_FIELDS, VUE_CONNECTOR_GUIDES, buildDataSourceInput } from './form.ts';

test('confluence credential fields carry the edition-switched secret pair', () => {
  const fields = VUE_CREDENTIAL_FIELDS.confluence ?? [];
  const keys = fields.map((field) => field.key);
  assert.ok(keys.includes('edition'));
  assert.ok(keys.includes('base_url'));
  assert.ok(keys.includes('username'));
  // Both secret fields exist and are optional: which one is REQUIRED depends
  // on the edition — the backend's connection test reports the exact miss.
  const apiToken = fields.find((field) => field.key === 'api_token');
  const password = fields.find((field) => field.key === 'password');
  assert.ok(apiToken?.secret && apiToken.optional);
  assert.ok(password?.secret && password.optional);
});

test('dingtalk credential fields are the three required secrets', () => {
  const keys = (VUE_CREDENTIAL_FIELDS.dingtalk ?? []).map((field) => field.key);
  assert.deepEqual(keys.sort(), ['client_id', 'client_secret', 'operator_id']);
  assert.ok(VUE_CREDENTIAL_FIELDS.dingtalk?.every((field) => field.secret && !field.optional));
});

test('confluence credentials round-trip through the key=value protocol', () => {
  const input = buildDataSourceInput({
    name: 'cf', type: 'confluence', schedule: '0 0 */6 * * *', mode: 'incremental',
    conflict: 'overwrite', deletions: true,
    credentialsText: 'edition = cloud\nbase_url = https://x.atlassian.net\nusername = a@b.c\napi_token = t1',
    settingsText: '', resourceIds: [], authHeaders: [], gitlabProjects: [],
  });
  // DataSource.config is `unknown` in the api-client contract — the same
  // Record cast form.test.ts uses to read the serialized credentials.
  const credentials = (input.config as Record<string, unknown> | undefined)?.credentials as Record<string, unknown> | undefined;
  assert.equal(credentials?.edition, 'cloud');
});

test('both new connectors have setup guides', () => {
  assert.ok(VUE_CONNECTOR_GUIDES.confluence?.docUrl);
  assert.ok(VUE_CONNECTOR_GUIDES.dingtalk?.docUrl);
});
