import assert from 'node:assert/strict';
import test from 'node:test';
import { authNavigationTarget, guardRoute, nextPathAfterAuth, organizationInviteCode, protectedPageForRoute, resolveRoute, routeRedirect, type RouteGuardContext } from './routes.tsx';

const authenticated: RouteGuardContext = {
  authenticated: true,
  tenantId: 'tenant-1',
  capabilities: {},
  isSystemAdmin: false,
};

test('keeps legacy deep links and redirects the misspelled chat path compatibly', () => {
  assert.equal(resolveRoute('/knowledgeBase?id=kb-1').kind, 'knowledge-base');
  assert.equal(resolveRoute('/platform/settings?section=general').kind, 'platform');
  assert.equal(resolveRoute('/platform/integrations').kind, 'platform');
  assert.deepEqual(resolveRoute('/platform/chat'), { kind: 'platform', path: '/platform/creatChat' });
  assert.equal(routeRedirect('/platform/chat?agentId=a'), '/platform/creatChat?agentId=a');
  assert.equal(routeRedirect('/creatChat?agentId=a'), '/platform/creatChat?agentId=a');
  assert.equal(resolveRoute('/creatChat/unknown').kind, 'not-found');
  assert.deepEqual(guardRoute('/creatChat', { ...authenticated, authenticated: false, tenantId: null }), {
    kind: 'redirect',
    to: '/login?next=%2FcreatChat',
    reason: 'authentication-required',
  });
  assert.equal(resolveRoute('/platform').kind, 'platform');
  assert.equal(resolveRoute('/platform/tenant').kind, 'platform');
  assert.deepEqual(resolveRoute('/platform/system/queues'), { kind: 'platform', path: '/platform/system/queues' });
  assert.deepEqual(resolveRoute('/platform/system/settings'), { kind: 'platform', path: '/platform/system/settings' });
  assert.equal(resolveRoute('/platform/system/unknown').kind, 'not-found');
  assert.deepEqual(resolveRoute('/register'), { kind: 'login', path: '/register', mode: 'register' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1'), { kind: 'knowledge-base', path: '/knowledgeBase/kb-1', knowledgeBaseId: 'kb-1' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1?knowledge_id=doc-1'), { kind: 'knowledge-base', path: '/knowledgeBase/kb-1', knowledgeBaseId: 'kb-1', initialDocumentId: 'doc-1' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1/documents/doc-1'), { kind: 'knowledge-document', path: '/knowledgeBase/kb-1/documents/doc-1', knowledgeBaseId: 'kb-1', documentId: 'doc-1' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1/wiki'), { kind: 'knowledge-wiki', path: '/knowledgeBase/kb-1/wiki', knowledgeBaseId: 'kb-1' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1/faq'), { kind: 'knowledge-faq', path: '/knowledgeBase/kb-1/faq', knowledgeBaseId: 'kb-1' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1/settings'), { kind: 'knowledge-settings', path: '/knowledgeBase/kb-1/settings', knowledgeBaseId: 'kb-1' });
  assert.deepEqual(resolveRoute('/platform/knowledge-bases/kb-1/creatChat'), { kind: 'chat', path: '/platform/knowledge-bases/kb-1/creatChat', knowledgeBaseId: 'kb-1' });
  assert.deepEqual(resolveRoute('/platform/knowledge-bases/kb-1?tab=wiki&slug=docs/start'), { kind: 'knowledge-base', path: '/platform/knowledge-bases/kb-1', knowledgeBaseId: 'kb-1', tab: 'wiki', slug: 'docs/start' });
  assert.deepEqual(resolveRoute('/platform/knowledge-bases/kb-1?knowledge_id=doc-1'), { kind: 'knowledge-base', path: '/platform/knowledge-bases/kb-1', knowledgeBaseId: 'kb-1', initialDocumentId: 'doc-1' });
  assert.equal(resolveRoute('/knowledgeBase/%E0%A4%A').kind, 'not-found');
  assert.deepEqual(resolveRoute('/platform/agents'), { kind: 'platform', path: '/platform/agents' });
  assert.deepEqual(resolveRoute('/platform/apps'), { kind: 'apps', path: '/platform/apps', mode: 'catalog' });
  assert.deepEqual(resolveRoute('/platform/apps/connections'), { kind: 'apps', path: '/platform/apps/connections', mode: 'connections' });
  assert.deepEqual(resolveRoute('/platform/apps/authorization/attempt-1'), { kind: 'apps', path: '/platform/apps/authorization/attempt-1', mode: 'authorization', id: 'attempt-1' });
  assert.deepEqual(resolveRoute('/platform/apps/actions/action-1'), { kind: 'apps', path: '/platform/apps/actions/action-1', mode: 'action', id: 'action-1' });
  assert.equal(resolveRoute('/platform/apps/authorization/%E0%A4%A').kind, 'not-found');
  assert.equal(resolveRoute('/platform/apps/actions/%E0%A4%A').kind, 'not-found');
  assert.deepEqual(guardRoute('/platform/apps', { ...authenticated, authenticated: false, tenantId: null }), { kind: 'redirect', to: '/login?next=%2Fplatform%2Fapps', reason: 'authentication-required' });
  assert.equal(routeRedirect('/'), '/platform/knowledge-bases');
  assert.equal(routeRedirect('/platform/knowledge-search?query=hello'), '/platform/knowledge-bases?cmdk=');
  assert.equal(routeRedirect('/platform/knowledge-search'), '/platform/knowledge-bases?cmdk=');
  assert.equal(routeRedirect('/platform/knowledge-search?q=hello'), '/platform/knowledge-bases?cmdk=hello');
  assert.equal(routeRedirect('/platform/tenant'), '/platform/settings');
  assert.equal(routeRedirect('/platform/administration'), '/platform/settings?section=members');
  assert.equal(routeRedirect('/platform/system/queues'), '/platform/settings?section=runtime-queues');
  assert.equal(routeRedirect('/platform/system/admins'), '/platform/settings?section=system-global');
  assert.equal(resolveRoute('/platform/dev/markdown', { development: false }).kind, 'not-found');
});

test('does not treat embed or missing capability paths as authenticated platform routes', () => {
  assert.equal(resolveRoute('/embed/channel-1').kind, 'embed');
  assert.equal(resolveRoute('/unknown').kind, 'not-found');
  assert.equal(resolveRoute('/platform/not-a-page').kind, 'not-found');
  assert.equal(resolveRoute('/platform/system/queue').kind, 'not-found');
});

test('restores only safe auth next paths and prioritizes invite completion', () => {
  assert.equal(nextPathAfterAuth('?next=%2Fplatform%2Fapps%3Ftab%3Dconnections'), '/platform/apps?tab=connections');
  assert.equal(nextPathAfterAuth('?next=https%3A%2F%2Fevil.example'), '/platform/knowledge-bases');
  assert.equal(nextPathAfterAuth('?next=%2F%2Fevil.example'), '/platform/knowledge-bases');
  assert.equal(authNavigationTarget('?token=invite&next=%2Fplatform%2Fapps', true), '/platform/knowledge-bases');
  assert.equal(authNavigationTarget('?next=%2Fplatform%2Fapps', false), '/platform/apps');
});

test('maps the canonical knowledge-base platform route to the list page', () => {
  assert.equal(protectedPageForRoute(resolveRoute('/platform/knowledge-bases')), 'knowledge-bases');
  assert.equal(protectedPageForRoute(resolveRoute('/knowledgeBase')), 'knowledge-bases');
});

test('keeps the development markdown fixture public and dispatchable', () => {
  assert.equal(resolveRoute('/platform/dev/markdown').kind, 'not-found');
  assert.equal(guardRoute('/platform/dev/markdown', { ...authenticated, authenticated: false, tenantId: null, development: false }).kind, 'allow');
  assert.equal(protectedPageForRoute(resolveRoute('/platform/dev/markdown', { development: true })), 'markdown-test');
});

test('guards protected deep links and redirects no-tenant sessions to onboarding', () => {
  assert.deepEqual(guardRoute('/platform/knowledge-bases?tab=mine', { ...authenticated, authenticated: false, tenantId: null }), {
    kind: 'redirect',
    to: '/login?next=%2Fplatform%2Fknowledge-bases%3Ftab%3Dmine',
    reason: 'authentication-required',
  });
  assert.deepEqual(guardRoute('/platform/knowledge-bases', { ...authenticated, tenantId: null }), {
    kind: 'redirect',
    to: '/onboarding/workspace',
    reason: 'workspace-required',
  });
  assert.deepEqual(guardRoute('/', authenticated), {
    kind: 'redirect',
    to: '/platform/knowledge-bases',
    reason: 'capability-unavailable',
  });
  assert.deepEqual(guardRoute('/platform/knowledge-search?q=hello', authenticated), {
    kind: 'redirect',
    to: '/platform/knowledge-bases?cmdk=hello',
    reason: 'capability-unavailable',
  });
});

test('authenticates legacy platform redirects before choosing their destination', () => {
  const loggedOut = { ...authenticated, authenticated: false, tenantId: null };
  for (const path of ['/platform', '/platform/knowledge-search?q=hello', '/platform/tenant', '/platform/administration']) {
    assert.deepEqual(guardRoute(path, loggedOut), {
      kind: 'redirect',
      to: `/login?next=${encodeURIComponent(path)}`,
      reason: 'authentication-required',
    });
  }
  for (const path of ['/platform', '/platform/knowledge-search?q=hello', '/platform/tenant', '/platform/administration']) {
    assert.deepEqual(guardRoute(path, { ...authenticated, tenantId: null }), {
      kind: 'redirect',
      to: '/onboarding/workspace',
      reason: 'workspace-required',
    });
  }
});

test('lands non-system-admin system history links on the knowledge-base page', () => {
  for (const path of ['/platform/system', '/platform/system/settings', '/platform/system/admins', '/platform/system/queues']) {
    assert.deepEqual(guardRoute(path, authenticated), {
      kind: 'redirect',
      to: '/platform/knowledge-bases',
      reason: 'system-admin-required',
    });
  }
});

test('honors explicit capability and organization invite compatibility rules', () => {
  assert.deepEqual(guardRoute('/platform/organizations', {
    ...authenticated,
    capabilities: { organizations: { supported: false, reason: 'lite' } },
  }), {
    kind: 'redirect',
    to: '/platform/knowledge-bases',
    reason: 'capability-unavailable',
  });
  assert.equal(routeRedirect('/join?code=org-invite'), '/platform/organizations?invite_code=org-invite');
  assert.equal(organizationInviteCode('/platform/organizations?invite_code=org-invite'), 'org-invite');
  assert.equal(routeRedirect('/join'), '/platform/organizations');
  assert.equal(resolveRoute('/platform/system/queue').kind, 'not-found');
  assert.deepEqual(guardRoute('/platform/system/queues', { ...authenticated, isSystemAdmin: true }), {
    kind: 'redirect',
    to: '/platform/settings?section=runtime-queues',
    reason: 'capability-unavailable',
  });
  assert.deepEqual(guardRoute('/platform/system', { ...authenticated, isSystemAdmin: true }), {
    kind: 'redirect',
    to: '/platform/settings?section=system-global',
    reason: 'capability-unavailable',
  });
  assert.deepEqual(guardRoute('/platform/administration', authenticated), {
    kind: 'redirect',
    to: '/platform/settings?section=members',
    reason: 'capability-unavailable',
  });
});

test('normalizes Vue legacy integration URLs into settings without losing unrelated query values', () => {
  const cases = [
    ['', 'integration-im'],
    ['?tab=embed', 'integration-embed'],
    ['?section=integrations&tab=api', 'integration-api'],
    ['?section=claw&tab=embed', 'integration-claw'],
    ['?section=integration-cli&tab=api', 'integration-cli'],
    ['?tab=unknown', 'integration-im'],
    ['?section=general&tab=api', 'general'],
    ['?section=unknown&tab=api', 'unknown'],
  ] as const;
  for (const [search, section] of cases) {
    assert.equal(routeRedirect(`/platform/integrations${search}`), `/platform/settings?section=${section}`);
  }
  const destination = routeRedirect('/platform/integrations?section=integrations&tab=embed&agentId=agent-1&filter=a&filter=b');
  assert.ok(destination);
  const query = new URL(destination, 'https://example.test').searchParams;
  assert.equal(query.get('section'), 'integration-embed');
  assert.equal(query.has('tab'), false);
  assert.equal(query.get('agentId'), 'agent-1');
  assert.deepEqual(query.getAll('filter'), ['a', 'b']);
});

test('keeps query-driven settings and modal/preview entry points on their owning routes', () => {
  for (const path of [
    '/platform/settings?section=models&subsection=embedding',
    '/platform/settings?section=integration-embed&agentId=agent-1',
    '/platform/agents?edit=agent-1&section=tools&highlight=allowed_tools&sourceTenantId=10001',
  ]) {
    assert.equal(resolveRoute(path).kind, 'platform');
  }
  assert.deepEqual(resolveRoute('/platform/knowledge-bases/kb-1?tab=documents&knowledge_id=doc-1'), {
    kind: 'knowledge-base',
    path: '/platform/knowledge-bases/kb-1',
    knowledgeBaseId: 'kb-1',
    tab: 'documents',
    initialDocumentId: 'doc-1',
  });
});

test('guards legacy integration URLs before forwarding to the Vue settings destination', () => {
  const path = '/platform/integrations?tab=embed';
  assert.deepEqual(guardRoute(path, { ...authenticated, authenticated: false, tenantId: null }), {
    kind: 'redirect', to: `/login?next=${encodeURIComponent(path)}`, reason: 'authentication-required',
  });
  assert.deepEqual(guardRoute(path, { ...authenticated, tenantId: null }), {
    kind: 'redirect', to: '/onboarding/workspace', reason: 'workspace-required',
  });
  assert.deepEqual(guardRoute(path, authenticated), {
    kind: 'redirect', to: '/platform/settings?section=integration-embed', reason: 'capability-unavailable',
  });
  assert.deepEqual(guardRoute(path, { ...authenticated, capabilities: { integrations: { supported: false } } }), {
    kind: 'redirect', to: '/platform/settings?section=integration-embed', reason: 'capability-unavailable',
  });
});

test('requires authentication before rendering not-found pages under protected prefixes (S00 D1)', () => {
  const loggedOut = { ...authenticated, authenticated: false, tenantId: null };
  assert.deepEqual(guardRoute('/platform/not-a-page', loggedOut), {
    kind: 'redirect',
    to: '/login?next=%2Fplatform%2Fnot-a-page',
    reason: 'authentication-required',
  });
  assert.equal(guardRoute('/unknown', loggedOut).kind, 'allow');
});
