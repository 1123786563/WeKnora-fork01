import assert from 'node:assert/strict';
import test from 'node:test';
import { resourceCheckStates, toggleResourceSelection } from './resource-selection.ts';

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
