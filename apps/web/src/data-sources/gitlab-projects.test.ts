import assert from 'node:assert/strict';
import test from 'node:test';
import { buildDataSourceInput, dataSourceFormFrom, serializeGitLabProjects, type DataSourceFormValues } from './form.ts';
import type { DataSource } from '@weknora/api-client';

// Vue DataSourceEditorDialog syncGitLabProjectsToSettings: rows without a
// project_id are dropped; project_id/ref are trimmed (an empty ref survives as
// ''); paths splits on newlines OR commas, trims each entry and drops blanks.
test('serializes gitlab project rows to the Vue settings.projects shape', () => {
  assert.deepEqual(serializeGitLabProjects([
    { project_id: ' 12345 ', ref: ' main ', pathsText: 'docs\nsrc, lib ' },
    { project_id: 'group/project', ref: '', pathsText: '' },
    { project_id: '   ', ref: 'x', pathsText: 'y' },
  ]), [
    { project_id: '12345', ref: 'main', paths: ['docs', 'src', 'lib'] },
    { project_id: 'group/project', ref: '', paths: [] },
  ]);
});

// Vue buildConfigPayload runs syncGitLabProjectsToSettings on every save, so
// the persisted settings carry the structured projects array — byte-exact
// { project_id, ref, paths } objects, never a serialized text blob.
test('builds the gitlab payload with structured settings.projects matching Vue', () => {
  const input = buildDataSourceInput(gitlabForm({
    gitlabProjects: [
      { project_id: '12345', ref: 'main', pathsText: 'docs\nsrc' },
      { project_id: 'group/project', ref: '', pathsText: '' },
      { project_id: '', ref: 'drop-me', pathsText: '' },
    ],
  }));
  assert.deepEqual((input.config as Record<string, unknown>).settings, {
    projects: [
      { project_id: '12345', ref: 'main', paths: ['docs', 'src'] },
      { project_id: 'group/project', ref: '', paths: [] },
    ],
  });
});

// The Vue editor keeps projects as a structured draft (gitlabProjects ref)
// next to the generic key=value settings lines: scalar keys from settingsText
// survive, the structured projects array rides its own channel, and a stale
// hand-typed `projects = ...` line can never override the row editor.
test('settingsText and gitlab projects never pollute each other', () => {
  const source = { id: 'ds-1', knowledge_base_id: 'kb-1', name: 'Git', type: 'gitlab', config: { credentials: {}, settings: { verify_ssl: 'true', projects: [{ project_id: '12345', ref: 'main', paths: ['docs', 'src'] }] }, resource_ids: [] } } as DataSource;
  const hydrated = dataSourceFormFrom(source);
  assert.equal(hydrated.settingsText.includes('projects'), false, 'structured projects must not round-trip through settingsText');
  assert.equal(hydrated.settingsText.includes('verify_ssl = true'), true, 'scalar settings keys still hydrate into settingsText');
  assert.deepEqual(hydrated.gitlabProjects, [{ project_id: '12345', ref: 'main', pathsText: 'docs\nsrc' }], 'Vue joins stored paths back with newlines into pathsText');

  const input = buildDataSourceInput({ ...hydrated, credentialsText: 'base_url = https://gitlab.example.com\naccess_token = tk', settingsText: 'verify_ssl = true\nprojects = hack', gitlabProjects: [{ project_id: '12345', ref: 'main', pathsText: 'docs' }] });
  const settings = (input.config as Record<string, unknown>).settings as Record<string, unknown>;
  assert.deepEqual(settings.projects, [{ project_id: '12345', ref: 'main', paths: ['docs'] }], 'the structured rows own the projects key');
  assert.equal(settings.verify_ssl, 'true', 'unrelated settingsText keys survive untouched');
});

// Vue isGitLabConnector gates the whole projects channel: non-gitlab
// connectors never grow a settings.projects key even with stray row state.
test('non-gitlab payloads never grow a settings.projects key', () => {
  const input = buildDataSourceInput(gitlabForm({ type: 'notion', settingsText: 'workspace_id = ws-1', gitlabProjects: [{ project_id: '12345', ref: 'main', pathsText: 'docs' }] }));
  assert.deepEqual((input.config as Record<string, unknown>).settings, { workspace_id: 'ws-1' });
});

// Vue openEditor seeds exactly one empty row when creating a gitlab connector
// (addGitLabProject on def open); edits hydrate rows from the saved settings.
test('edit hydration tolerates malformed saved projects', () => {
  const source = { id: 'ds-2', knowledge_base_id: 'kb-1', name: 'Git', type: 'gitlab', config: { credentials: {}, settings: { projects: 'not-an-array' }, resource_ids: [] } } as DataSource;
  assert.deepEqual(dataSourceFormFrom(source).gitlabProjects, []);
  const none = { id: 'ds-3', knowledge_base_id: 'kb-1', name: 'Git', type: 'gitlab', config: { credentials: {}, settings: {}, resource_ids: [] } } as DataSource;
  assert.deepEqual(dataSourceFormFrom(none).gitlabProjects, []);
});

function gitlabForm(overrides: Partial<DataSourceFormValues>): DataSourceFormValues {
  return {
    name: 'GitLab docs',
    type: 'gitlab',
    schedule: '0 0 */6 * * *',
    mode: 'incremental',
    conflict: 'overwrite',
    deletions: true,
    credentialsText: 'base_url = https://gitlab.example.com\naccess_token = tk',
    settingsText: '',
    resourceIds: [],
    ...overrides,
  };
}
