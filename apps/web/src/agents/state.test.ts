import assert from 'node:assert/strict';
import test from 'node:test';
import { canEditAgent, canManageAgent, classifyAgent, initialEditorState, type Agent, type AgentPermissions } from './state.ts';

const own: Agent = { id: 'a-1', name: 'Mine', created_by: 'u-1', is_builtin: false };
const builtin: Agent = { id: 'builtin-quick-answer', name: 'Quick', is_builtin: true };
const shared: Agent = { id: 'a-2', name: 'Shared', created_by: 'u-2', is_builtin: false, permission: 'editor', source_tenant_id: 't-2' };

test('matches Vue ownership and admin mutation rules', () => {
  const contributor: AgentPermissions = { userId: 'u-1', roles: ['contributor'] };
  assert.equal(canManageAgent(own, contributor), true);
  assert.equal(canManageAgent(builtin, contributor), false);
  assert.equal(canManageAgent(shared, contributor), false);
  assert.equal(canManageAgent(builtin, { userId: 'u-9', roles: ['admin'] }), true);
  assert.equal(canManageAgent(own, { userId: 'u-9', roles: ['viewer'] }), false);
});

test('shared edit permission is distinct from owner management', () => {
  assert.equal(canEditAgent(shared), true);
  assert.equal(canEditAgent({ ...shared, permission: 'viewer' }), false);
  assert.equal(canEditAgent(own), false);
});

test('classifies list sections without leaking shared agents into mine', () => {
  assert.equal(classifyAgent(own, 'u-1'), 'mine');
  assert.equal(classifyAgent({ ...own, created_by: 'u-2' }, 'u-1'), 'tenant-others');
  assert.equal(classifyAgent(shared, 'u-1'), 'shared-editable');
  assert.equal(classifyAgent({ ...shared, permission: 'viewer' }, 'u-1'), 'shared-readonly');
  assert.equal(classifyAgent(builtin, 'u-1'), 'builtin');
});

test('editor starts clean and preserves the requested section', () => {
  assert.deepEqual(initialEditorState(null, 'tools'), { mode: 'create', section: 'tools', saving: false, error: null });
  assert.deepEqual(initialEditorState(own, 'model'), { mode: 'edit', section: 'model', saving: false, error: null });
});
