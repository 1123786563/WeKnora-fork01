import assert from 'node:assert/strict';
import test from 'node:test';
import { createProductScope } from './product-session.ts';

test('switching tenant aborts previous request and rejects late response', () => {
  const scope = createProductScope({ origin: 'https://a.test', userId: 'u', tenantId: 'a' });
  const old = scope.capture();
  scope.switchTo({ origin: 'https://a.test', userId: 'u', tenantId: 'b' });
  assert.equal(old.signal.aborted, true);
  assert.equal(scope.accept(old.generation), false);
});

test('logout clears identity while preserving the current origin', () => {
  const scope = createProductScope({ origin: 'https://a.test', userId: 'u', tenantId: 'a' });
  scope.logout();
  const current = scope.capture();
  assert.equal(current.signal.aborted, false);
  assert.equal(scope.accept(current.generation), true);
  assert.deepEqual(scope.identity(), { origin: 'https://a.test', userId: null, tenantId: null });
});

test('a refresh write can be accepted only by the generation that started it', () => {
  const scope = createProductScope({ origin: 'https://a.test', userId: 'u', tenantId: 'a' });
  const refresh = scope.capture();
  scope.switchTo({ origin: 'https://a.test', userId: 'new-user', tenantId: 'new-tenant' });
  assert.equal(scope.accept(refresh.generation), false);
  assert.equal(scope.accept(scope.capture().generation), true);
});

test('scope transitions close client subscriptions without cancelling server work', async () => {
  const scope = createProductScope({ origin: 'https://a.test', userId: 'u', tenantId: 'a' });
  let closed = 0;
  scope.registerLifecycle(() => { closed += 1; });
  scope.switchTo({ origin: 'https://a.test', userId: 'u', tenantId: 'b' });
  await Promise.resolve();
  assert.equal(closed, 1);
});
