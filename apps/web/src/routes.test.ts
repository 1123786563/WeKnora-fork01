import assert from 'node:assert/strict';
import test from 'node:test';
import { resolveRoute, routeRedirect } from './routes.tsx';

test('keeps legacy deep links and redirects the misspelled chat path compatibly', () => {
  assert.equal(resolveRoute('/knowledgeBase?id=kb-1').kind, 'knowledge-base');
  assert.equal(resolveRoute('/platform/settings?section=general').kind, 'platform');
  assert.equal(routeRedirect('/creatChat?agentId=a'), '/platform/creatChat?agentId=a');
  assert.equal(resolveRoute('/platform').kind, 'platform');
});

test('does not treat embed or missing capability paths as authenticated platform routes', () => {
  assert.equal(resolveRoute('/embed/channel-1').kind, 'embed');
  assert.equal(resolveRoute('/unknown').kind, 'not-found');
});
