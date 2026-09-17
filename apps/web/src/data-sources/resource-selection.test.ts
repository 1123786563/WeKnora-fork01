import assert from 'node:assert/strict';
import test from 'node:test';
import { extractDriveFolderToken, isDriveConnector, resourceCheckStates, toggleResourceSelection } from './resource-selection.ts';

const resources = [
  { external_id: 'root', name: 'Root', type: 'folder', has_children: true },
  { external_id: 'a', name: 'A', type: 'page', parent_id: 'root' },
  { external_id: 'b', name: 'B', type: 'page', parent_id: 'root' },
] as const;

test('parent selection checks loaded descendants and child selection is indeterminate', () => {
  assert.equal(resourceCheckStates(resources, ['root']).get('a'), 'checked');
  assert.equal(resourceCheckStates(resources, ['a']).get('root'), 'indeterminate');
});

test('toggle selection stores a parent cover and removes its descendants when cleared', () => {
  assert.deepEqual(toggleResourceSelection(resources, [], 'root'), ['root']);
  assert.deepEqual(toggleResourceSelection(resources, ['root'], 'root'), []);
  assert.deepEqual(toggleResourceSelection(resources, ['root'], 'a'), ['b']);
});

// Vue extractDriveFolderToken: Drive connectors have no "list spaces" API, so
// the user supplies a root folder_token (bare token or a Feishu/Lark folder
// URL); matching is path-based and host-agnostic.
test('extractDriveFolderToken accepts bare tokens and Drive folder URLs', () => {
  assert.equal(extractDriveFolderToken('  fldcnAbc123  '), 'fldcnAbc123');
  assert.equal(extractDriveFolderToken('https://xxx.feishu.cn/drive/folder/fldcnURLToken?from=share'), 'fldcnURLToken');
  assert.equal(extractDriveFolderToken('https://xxx.larksuite.com/drive/folder/fldLark#/'), 'fldLark');
  assert.equal(extractDriveFolderToken('https://example.com/some/deep/path'), 'path', 'fallback: last URL path segment');
  assert.equal(extractDriveFolderToken(''), '');
  assert.equal(isDriveConnector('feishu_drive'), true);
  assert.equal(isDriveConnector('lark_drive'), true);
  assert.equal(isDriveConnector('notion'), false);
});
