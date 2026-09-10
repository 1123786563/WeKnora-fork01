import assert from 'node:assert/strict';
import test from 'node:test';
import { createWebScopeRuntime } from './scope-runtime.ts';

test('tenant switches abort the previous scope and produce a scoped query key', () => {
  const runtime = createWebScopeRuntime('https://api.test', 'user-1', 'tenant-a');
  const previous = runtime.current();
  const next = runtime.setTenant('tenant-b');
  assert.equal(previous.signal.aborted, true);
  assert.equal(runtime.key('knowledge-bases')[3], 'tenant-b');
  assert.equal(runtime.controller.isCurrent(next.scope), true);
});

test('logout invalidates the active generation so late data cannot be accepted', () => {
  const runtime = createWebScopeRuntime('https://api.test', 'user-1', 'tenant-a');
  const previous = runtime.current();
  runtime.controller.logout();
  assert.equal(previous.signal.aborted, true);
  assert.equal(runtime.controller.isCurrent(previous.scope), false);
});
