import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => void }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.png') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } as never : nextResolve(specifier, context) });

const { buildNavItems, shouldShowTenantSwitcher } = await import('./PlatformShell.tsx');

test('matches Vue menu: chat detail does not activate the New Chat item', () => {
  const items = buildNavItems((key) => key, {
    newChat: 'New Chat',
    agents: 'Agents',
    organizations: 'Organizations',
  });
  const newChat = items.find((item) => item.key === 'newChat');
  assert.ok(newChat);
  assert.equal(newChat.match('/platform/chat/session-1'), false);
  assert.equal(newChat.match('/platform/creatChat'), true);
});

test('M2: the rail carries the experts entry right after agents', () => {
  const items = buildNavItems((key) => key, {
    newChat: 'New Chat',
    knowledgeBases: 'Knowledge Bases',
    agents: 'Agents',
    experts: 'Expert Templates',
    market: 'Skills Market',
    organizations: 'Organizations',
    analytics: 'Analytics',
  });
  assert.deepEqual(items.map((item) => item.key), [
    'newChat', 'career', 'careerSearch', 'careerRules', 'knowledgeBases', 'agents', 'experts', 'market', 'organizations', 'analytics',
  ]);
  const experts = items.find((item) => item.key === 'experts');
  assert.ok(experts);
  assert.equal(experts.href, '/platform/experts');
  assert.equal(experts.label, 'Expert Templates');
  assert.ok(experts.icon);
  assert.equal(experts.match('/platform/experts'), true);
  assert.equal(experts.match('/platform/experts/news-trend'), true);
  assert.equal(experts.match('/platform/agents'), false);
});

// T11 one-shot search entry: an exact-match sibling under the career office.
// The career entry must not light up for /career/search (and vice versa).
test('T11: the rail carries the one-shot search entry next to the career office', () => {
  const items = buildNavItems((key) => key, {});
  const search = items.find((item) => item.key === 'careerSearch');
  const career = items.find((item) => item.key === 'career');
  assert.ok(search);
  assert.equal(search.href, '/platform/career/search');
  assert.equal(search.label, '找岗');
  assert.equal(search.match('/platform/career/search'), true);
  assert.equal(search.match('/platform/career'), false);
  assert.ok(career);
  assert.equal(career.match('/platform/career/search'), false);
});

// T13 recurring rule entry: an exact-match sibling right after the one-shot
// search; the neighbours must not light up for /career/rules (and vice
// versa).
test('T13: the rail carries the recurring rules entry after the one-shot search', () => {
  const items = buildNavItems((key) => key, {});
  const rules = items.find((item) => item.key === 'careerRules');
  const search = items.find((item) => item.key === 'careerSearch');
  const career = items.find((item) => item.key === 'career');
  assert.ok(rules);
  assert.equal(rules.href, '/platform/career/rules');
  assert.equal(rules.label, '持续找岗');
  assert.equal(rules.match('/platform/career/rules'), true);
  assert.equal(rules.match('/platform/career/search'), false);
  assert.equal(rules.match('/platform/career'), false);
  assert.ok(search);
  assert.equal(search.match('/platform/career/rules'), false);
  assert.ok(career);
  assert.equal(career.match('/platform/career/rules'), false);
});

test('M4: the rail carries the skills-market entry right after experts', () => {
  const items = buildNavItems((key) => key, {
    newChat: 'New Chat',
    knowledgeBases: 'Knowledge Bases',
    agents: 'Agents',
    experts: 'Expert Templates',
    market: 'Skills Market',
    organizations: 'Organizations',
    analytics: 'Analytics',
  });
  const market = items.find((item) => item.key === 'market');
  assert.ok(market);
  assert.equal(market.href, '/platform/market');
  assert.equal(market.label, 'Skills Market');
  assert.ok(market.icon);
  assert.equal(market.match('/platform/market'), true);
  assert.equal(market.match('/platform/market/some-skill'), true);
  assert.equal(market.match('/platform/experts'), false);
});

test('matches Vue TenantSelector visibility for ordinary multi-tenant members', () => {
  assert.equal(shouldShowTenantSwitcher({ canAccessAllTenants: false, collapsed: false, hasSwitchHandler: true }), false);
  assert.equal(shouldShowTenantSwitcher({ canAccessAllTenants: true, collapsed: false, hasSwitchHandler: true }), true);
  assert.equal(shouldShowTenantSwitcher({ canAccessAllTenants: true, collapsed: true, hasSwitchHandler: true }), false);
});
