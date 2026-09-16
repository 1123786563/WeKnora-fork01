import assert from 'node:assert/strict';
import test from 'node:test';

import { buildIntegrationPath, parseIntegrationRoute } from './route.ts';
import { restoreApiPlaygroundFocus, resolveIntegrationsTab } from './IntegrationsRoutePage.tsx';
import { nextTab, visibleChannels } from './state.ts';

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

test('restores focus to the API playground trigger after the drawer closes', () => {
  let focusCalls = 0;
  const trigger = { focus: () => { focusCalls += 1; } } as unknown as HTMLElement;

  restoreApiPlaygroundFocus(trigger, true);
  assert.equal(focusCalls, 0, 'focus remains in the drawer while it is open');
  restoreApiPlaygroundFocus(trigger, false);
  assert.equal(focusCalls, 1, 'Vue SettingDrawer returns focus to its trigger on close');
});

test('keeps a clicked integration tab when the route is embedded with an active tab', () => {
  assert.equal(resolveIntegrationsTab({ activeTab: 'api', localTab: 'api', requestedTab: 'im' }), 'im');
  assert.equal(resolveIntegrationsTab({ activeTab: 'api', localTab: 'api' }), 'api');
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

test('drawer tabs use roving keyboard navigation and wrap', () => {
  const tabs = ['configuration', 'playground', 'embed'] as const;
  assert.equal(nextTab(tabs, 'configuration', 'ArrowRight'), 'playground');
  assert.equal(nextTab(tabs, 'configuration', 'ArrowLeft'), 'embed');
  assert.equal(nextTab(tabs, 'playground', 'End'), 'embed');
  assert.equal(nextTab(tabs, 'playground', 'Tab'), null);
});

test('channel filtering keeps IM Web and embed data separate', () => {
  const snapshot = { channels: [
    { id: 'im-1', name: 'IM', enabled: true, kind: 'im' as const, agentId: 'a' },
    { id: 'embed-1', name: 'Embed', enabled: true, kind: 'embed' as const, agentId: 'a' },
  ] };
  assert.deepEqual(visibleChannels(snapshot, 'embed', 'a').map((channel) => channel.id), ['embed-1']);
  assert.deepEqual(visibleChannels(snapshot, 'im', null).map((channel) => channel.id), ['im-1']);
});

test('integration page exposes permission and async state contracts', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'IntegrationsPage.tsx'), 'utf8');
  for (const marker of ['<Sheet open', 'role="tabpanel"', 'aria-selected', 'tabIndex={active === tab ? 0 : -1}', 'tabRefs.current', 'trigger.current?.focus()', 'Loading integration settings', 'Retry', 'runPlaygroundRequest', 'No API keys configured', 'canEdit', 'Unable to save']) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
});
