import assert from 'node:assert/strict';
import test from 'node:test';
import { createWorkspaceSelectionAdapter, parseMobileWorkspaces, toWorkspaceId } from './workspace.ts';

function store(initial: string | null = null) {
  let value = initial;
  return {
    async getItemAsync() { return value; },
    async setItemAsync(_key: string, next: string) { value = next; },
    async deleteItemAsync() { value = null; },
  };
}

test('normalizes server memberships without inventing malformed workspaces', () => {
  assert.deepEqual(parseMobileWorkspaces([
    { tenant_id: 7, tenant_name: 'Docs', role: 'admin' },
    { tenant_id: '8', tenant_name: 'Research', role: 'viewer' },
    { tenant_id: 0, tenant_name: 'invalid', role: 'admin' },
    { tenant_id: 'not-a-number', tenant_name: 'invalid', role: 'admin' },
  ]), [
    { id: 7, name: 'Docs', role: 'admin' },
    { id: 8, name: 'Research', role: 'viewer' },
  ]);
});

test('workspace switching accepts only positive safe integer ids', () => {
  assert.equal(toWorkspaceId(7), 7);
  assert.equal(toWorkspaceId('8'), 8);
  assert.equal(toWorkspaceId(''), null);
  assert.equal(toWorkspaceId('1.5'), null);
  assert.equal(toWorkspaceId(Number.MAX_SAFE_INTEGER + 1), null);
});

test('persists and clears the selected workspace id', async () => {
  const adapter = createWorkspaceSelectionAdapter(store('7'));
  assert.equal(await adapter.read(), 7);
  await adapter.write(8);
  assert.equal(await adapter.read(), 8);
  await adapter.write(null);
  assert.equal(await adapter.read(), null);
});
