import assert from 'node:assert/strict';
import test from 'node:test';
import { authNavigationTarget, guardRoute, nextPathAfterAuth, resolveRoute, type RouteGuardContext } from './routes.tsx';

const authenticated: RouteGuardContext = {
  authenticated: true,
  tenantId: 'tenant-1',
};

test('redirects unauthenticated apps deep links to login while retaining the query', () => {
  const path = '/platform/apps?tab=connections&filter=slack';
  assert.deepEqual(resolveRoute(path), { kind: 'apps-catalog', path: '/platform/apps' });
  assert.deepEqual(guardRoute(path, { ...authenticated, authenticated: false, tenantId: null }), {
    kind: 'redirect',
    to: `/login?next=${encodeURIComponent(path)}`,
    reason: 'authentication-required',
  });
});

test('resolves all Vue app connector modes and protects every mode', () => {
  assert.deepEqual(resolveRoute('/platform/apps'), { kind: 'apps-catalog', path: '/platform/apps' });
  assert.deepEqual(resolveRoute('/platform/apps/connections'), { kind: 'apps-connections', path: '/platform/apps/connections' });
  assert.deepEqual(resolveRoute('/platform/apps/authorization/attempt-1'), { kind: 'apps-authorization', path: '/platform/apps/authorization/attempt-1', id: 'attempt-1' });
  assert.deepEqual(resolveRoute('/platform/apps/actions/action-1'), { kind: 'apps-action', path: '/platform/apps/actions/action-1', id: 'action-1' });
  for (const path of ['/platform/apps/connections', '/platform/apps/authorization/attempt-1', '/platform/apps/actions/action-1']) {
    assert.deepEqual(guardRoute(path, { authenticated: false, tenantId: null }), {
      kind: 'redirect', to: `/login?next=${encodeURIComponent(path)}`, reason: 'authentication-required',
    });
  }
});

test('rejects malformed or empty app connector ids as unknown routes', () => {
  assert.equal(resolveRoute('/platform/apps/actions/%E0%A4%A').kind, 'not-found');
  assert.equal(resolveRoute('/platform/apps/authorization/').kind, 'not-found');
});

test('does not turn an unknown protected path into a successful page', () => {
  assert.equal(resolveRoute('/platform/apps/not-a-route').kind, 'not-found');
  assert.equal(guardRoute('/platform/apps/not-a-route', authenticated).kind, 'not-found');
});

test('protects legacy and canonical integration settings entries', () => {
  for (const path of ['/platform/integrations', '/platform/settings?section=integration-api']) {
    assert.equal(resolveRoute(path).kind, 'integration');
    assert.deepEqual(guardRoute(path, { authenticated: false, tenantId: null }), {
      kind: 'redirect',
      to: `/login?next=${encodeURIComponent(path)}`,
      reason: 'authentication-required',
    });
  }
});

test('retains the complete original URL in next and restores only safe absolute paths', () => {
  assert.equal(nextPathAfterAuth('?next=%2Fplatform%2Fapps%3Ftab%3Dconnections'), '/platform/apps?tab=connections');
  assert.equal(nextPathAfterAuth('?next=https%3A%2F%2Fevil.example'), '/platform/knowledge-bases');
  assert.equal(nextPathAfterAuth('?next=%2F%2Fevil.example'), '/platform/knowledge-bases');
});

test('register remains a public auth route with invite and next query strings', () => {
  assert.deepEqual(resolveRoute('/register?token=invite-1&next=%2Fplatform%2Fapps'), { kind: 'login', path: '/register' });
  assert.deepEqual(guardRoute('/register?token=invite-1&next=%2Fplatform%2Fapps', { authenticated: false, tenantId: null }), { kind: 'allow' });
});

test('invite completion takes precedence over next at the application entry', () => {
  assert.equal(authNavigationTarget('?token=invite-1&next=%2Fplatform%2Fapps', true), '/platform/knowledge-bases');
  assert.equal(authNavigationTarget('?next=%2Fplatform%2Fapps', false), '/platform/apps');
});
