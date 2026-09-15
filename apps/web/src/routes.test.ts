import assert from 'node:assert/strict';
import test from 'node:test';
import { guardRoute, resolveRoute, type RouteGuardContext } from './routes.tsx';

const authenticated: RouteGuardContext = {
  authenticated: true,
  tenantId: 'tenant-1',
};

test('redirects unauthenticated apps deep links to login while retaining the query', () => {
  const path = '/platform/apps?tab=connections&filter=slack';
  assert.deepEqual(resolveRoute(path), { kind: 'apps', path: '/platform/apps' });
  assert.deepEqual(guardRoute(path, { ...authenticated, authenticated: false, tenantId: null }), {
    kind: 'redirect',
    to: `/login?next=${encodeURIComponent(path)}`,
    reason: 'authentication-required',
  });
});

test('does not turn an unknown protected path into a successful page', () => {
  assert.equal(resolveRoute('/platform/apps/not-a-route').kind, 'not-found');
  assert.equal(guardRoute('/platform/apps/not-a-route', authenticated).kind, 'not-found');
});
