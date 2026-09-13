import assert from 'node:assert/strict';
import test from 'node:test';
import type { DataSourceResource } from '@weknora/api-client';
import { canManageDataSources, dataSourceCredentialFields, dataSourceStatusLabel, extractDriveFolderToken, filterSupportedDataSourceTypes, hasRunningSync, resourceCheckState, resourceSelectionMarker, safeDataSourceType, toggleDataSourceResourceSelection, validateDataSourceCredentials } from './data-sources.ts';

test('mobile data source inventory exposes safe status and type labels', () => {
  assert.equal(dataSourceStatusLabel({ status: 'active' }), 'active');
  assert.equal(dataSourceStatusLabel({ status: undefined }), 'unknown');
  assert.equal(safeDataSourceType({ type: 'notion' }), 'notion');
  assert.equal(safeDataSourceType({ type: '' }), 'unknown');
});

test('mobile data-source mutations fail closed for non-admin workspace roles', () => {
  assert.equal(canManageDataSources('owner'), true);
  assert.equal(canManageDataSources('admin'), true);
  assert.equal(canManageDataSources('contributor'), false);
  assert.equal(canManageDataSources('viewer'), false);
  assert.equal(canManageDataSources(undefined), false);
});

const resourceTree: DataSourceResource[] = [
  { external_id: 'root', name: 'Root', type: 'folder', has_children: true },
  { external_id: 'sibling', name: 'Sibling', type: 'page', parent_id: 'root' },
  { external_id: 'child', name: 'Child', type: 'page', parent_id: 'root' },
];

test('resource selection keeps a minimal cover and supports descendant uncheck', () => {
  assert.deepEqual(toggleDataSourceResourceSelection(resourceTree, [], 'root'), ['root']);
  assert.equal(resourceCheckState(resourceTree, ['root'], 'child'), 'checked');
  assert.deepEqual(toggleDataSourceResourceSelection(resourceTree, ['root'], 'child'), ['sibling']);
  assert.equal(resourceCheckState(resourceTree, ['sibling'], 'child'), 'unchecked');
});

test('resource selection exposes a distinct marker for partial trees', () => {
  assert.equal(resourceSelectionMarker('checked'), '✓ ');
  assert.equal(resourceSelectionMarker('indeterminate'), '− ');
  assert.equal(resourceSelectionMarker('unchecked'), '');
});

test('running sync state is derived from the server latest log', () => {
  assert.equal(hasRunningSync({ latest_sync_log: { id: 'log-1', status: 'running' } }), true);
  assert.equal(hasRunningSync({ latest_sync_log: { id: 'log-2', status: 'success' } }), false);
  assert.equal(hasRunningSync({}), false);
});

test('drive folder input accepts a bare token and Feishu/Lark folder URLs', () => {
  assert.equal(extractDriveFolderToken('  fldcn123  '), 'fldcn123');
  assert.equal(extractDriveFolderToken('https://example.feishu.cn/drive/folder/fldcn123?x=1'), 'fldcn123');
  assert.equal(extractDriveFolderToken('https://example.larksuite.com/drive/folder/fldus456'), 'fldus456');
  assert.equal(extractDriveFolderToken(''), '');
});

test('connector credential fields mirror Vue requirements', () => {
  assert.deepEqual(dataSourceCredentialFields('notion').map((field) => field.key), ['api_key']);
  assert.deepEqual(dataSourceCredentialFields('feishu').map((field) => field.labelKey), ['dataSource.field.appId', 'dataSource.field.appSecret', 'dataSource.field.baseUrl']);
  assert.deepEqual(validateDataSourceCredentials('feishu', { app_id: 'cli_x' }).map((field) => field.key), ['app_secret']);
  assert.deepEqual(validateDataSourceCredentials('gitlab', { base_url: 'https://gitlab.test' }).map((field) => field.labelKey), ['dataSource.gitlab.accessToken']);
  assert.deepEqual(validateDataSourceCredentials('rss', {}), []);
});

test('connector picker exposes only the Vue-supported connector definitions', () => {
  const types = ['feishu', 'lark', 'feishu_drive', 'lark_drive', 'notion', 'yuque', 'ima', 'rss', 'gitlab']
    .map((type) => ({ type }));
  assert.deepEqual(filterSupportedDataSourceTypes([...types, { type: 'unsupported' }]).map((item) => item.type), types.map((item) => item.type));
});
