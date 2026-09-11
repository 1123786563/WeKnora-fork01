import assert from 'node:assert/strict';
import test from 'node:test';
import { guardRoute, protectedPageForRoute, resolveRoute, routeRedirect, type RouteGuardContext } from './routes.tsx';

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
  assert.equal(routeRedirect('/creatChat?agentId=a'), '/platform/creatChat?agentId=a');
  assert.equal(resolveRoute('/platform').kind, 'platform');
  assert.deepEqual(resolveRoute('/register'), { kind: 'login', path: '/register', mode: 'register' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1'), { kind: 'knowledge-base', path: '/knowledgeBase/kb-1' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1/documents/doc-1'), { kind: 'knowledge-document', path: '/knowledgeBase/kb-1/documents/doc-1', knowledgeBaseId: 'kb-1', documentId: 'doc-1' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1/wiki'), { kind: 'knowledge-wiki', path: '/knowledgeBase/kb-1/wiki', knowledgeBaseId: 'kb-1' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1/faq'), { kind: 'knowledge-faq', path: '/knowledgeBase/kb-1/faq', knowledgeBaseId: 'kb-1' });
  assert.deepEqual(resolveRoute('/knowledgeBase/kb-1/settings'), { kind: 'knowledge-settings', path: '/knowledgeBase/kb-1/settings', knowledgeBaseId: 'kb-1' });
  assert.deepEqual(resolveRoute('/platform/knowledge-bases/kb-1/creatChat'), { kind: 'chat', path: '/platform/knowledge-bases/kb-1/creatChat', knowledgeBaseId: 'kb-1' });
  assert.deepEqual(resolveRoute('/platform/agents'), { kind: 'platform', path: '/platform/agents' });
  assert.equal(routeRedirect('/'), '/platform/knowledge-bases');
  assert.equal(routeRedirect('/platform/knowledge-search?query=hello'), '/platform/knowledge-bases?query=hello');
});

test('does not treat embed or missing capability paths as authenticated platform routes', () => {
  assert.equal(resolveRoute('/embed/channel-1').kind, 'embed');
  assert.equal(resolveRoute('/unknown').kind, 'not-found');
  assert.equal(resolveRoute('/platform/not-a-page').kind, 'not-found');
  assert.equal(resolveRoute('/platform/system/queue').kind, 'platform');
});

test('maps the canonical knowledge-base platform route to the list page', () => {
  assert.equal(protectedPageForRoute(resolveRoute('/platform/knowledge-bases')), 'knowledge-bases');
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
    to: '/platform/knowledge-bases?q=hello',
    reason: 'capability-unavailable',
  });
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
  assert.equal(routeRedirect('/join'), '/platform/organizations');
  assert.deepEqual(guardRoute('/platform/system/queue', { ...authenticated, isSystemAdmin: true }), {
    kind: 'redirect',
    to: '/platform/system',
    reason: 'capability-unavailable',
  });
});
