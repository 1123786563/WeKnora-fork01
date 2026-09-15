import assert from 'node:assert/strict';
import test from 'node:test';

import { buildIntegrationPath, parseIntegrationRoute } from './route.ts';

test('maps the Vue integrations history URL to the canonical settings section', () => {
  assert.deepEqual(parseIntegrationRoute('/platform/integrations?tab=cli'), {
    tab: 'cli',
    agentId: null,
  });
  assert.deepEqual(parseIntegrationRoute('/platform/settings?section=integration-api'), {
    tab: 'api',
    agentId: null,
  });
});

test('preserves the optional agent filter and rejects unrelated routes', () => {
  assert.deepEqual(parseIntegrationRoute('/platform/integrations?tab=im&agentId=a-7'), {
    tab: 'im',
    agentId: 'a-7',
  });
  assert.equal(parseIntegrationRoute('/platform/agents'), null);
  assert.equal(parseIntegrationRoute('/platform/settings?section=general'), null);
});

test('builds canonical settings URLs for every Vue integration tab', () => {
  assert.equal(buildIntegrationPath('claw'), '/platform/settings?section=integration-claw');
  assert.equal(buildIntegrationPath('embed', 'agent-1'), '/platform/settings?section=integration-embed&agentId=agent-1');
});
