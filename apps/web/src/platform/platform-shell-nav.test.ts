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

test('matches Vue TenantSelector visibility for ordinary multi-tenant members', () => {
  assert.equal(shouldShowTenantSwitcher({ canAccessAllTenants: false, collapsed: false, hasSwitchHandler: true }), false);
  assert.equal(shouldShowTenantSwitcher({ canAccessAllTenants: true, collapsed: false, hasSwitchHandler: true }), true);
  assert.equal(shouldShowTenantSwitcher({ canAccessAllTenants: true, collapsed: true, hasSwitchHandler: true }), false);
});
