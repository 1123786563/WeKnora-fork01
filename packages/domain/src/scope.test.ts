import assert from 'node:assert/strict';
import test from 'node:test';

import { createScopeController } from './scope.ts';

test('switchScope creates a new generation and aborts the previous scope', () => {
  const controller = createScopeController();
  const first = controller.switchScope('https://one.example', 'user-1', 'tenant-1');
  const second = controller.switchScope('https://two.example', 'user-2', 'tenant-2');

  assert.equal(first.scope.generation + 1, second.scope.generation);
  assert.equal(first.signal.aborted, true);
  assert.equal(second.signal.aborted, false);
  assert.deepEqual(second.scope, {
    origin: 'https://two.example',
    userId: 'user-2',
    tenantId: 'tenant-2',
    generation: second.scope.generation,
  });
  assert.equal(controller.isCurrent(first.scope), false);
  assert.equal(controller.isCurrent(second.scope), true);
  assert.equal(controller.isCurrent(second.scope.generation), true);
});

test('invalidate and logout abort the active scope and reject its generation', () => {
  const controller = createScopeController();
  const active = controller.switchScope('https://example.test', 'user-1', 'tenant-1');

  controller.invalidate();
  const invalidated = controller.current();
  assert.equal(active.signal.aborted, true);
  assert.equal(controller.isCurrent(active.scope), false);
  assert.equal(controller.isCurrent(invalidated.scope.generation), true);

  controller.logout();
  const loggedOut = controller.current();
  assert.equal(invalidated.signal.aborted, true);
  assert.equal(controller.isCurrent(invalidated.scope), false);
  assert.equal(controller.isCurrent(loggedOut.scope), true);
});
