import assert from 'node:assert/strict';
import test from 'node:test';

import { formatMessage, messages, supportedLocales, type Locale } from '../src/index.ts';

// Ported verbatim from frontend/src/i18n/locales/*.ts (organization, agent,
// integrations blocks) and frontend/src/i18n/embed.ts (chat visitor subset as
// embed.*). Every locale must carry the same key set per domain.

function keysWithPrefix(locale: Locale, prefix: string): string[] {
  return Object.keys(messages[locale]).filter((key) => key.startsWith(prefix)).sort();
}

function assertDomainParity(prefix: string, minimum: number) {
  const reference = keysWithPrefix('en-US', prefix);
  assert.ok(reference.length >= minimum, prefix + ' should port at least ' + minimum + ' keys, found ' + reference.length);
  for (const locale of supportedLocales) {
    assert.deepEqual(keysWithPrefix(locale, prefix), reference, locale + ' diverges from en-US for ' + prefix);
  }
  return reference;
}

test('organization domain keys are present in every locale', () => {
  const keys = assertDomainParity('organization.', 50);
  for (const key of ['organization.invite.primaryJoin', 'organization.invite.submitRequest', 'organization.invite.needApproval', 'organization.invite.noApproval', 'organization.invite.alreadyMember', 'organization.invite.requestRole', 'organization.invite.applicationNote', 'organization.role.viewer']) {
    assert.ok(keys.includes(key), 'missing organization key: ' + key);
  }
});

test('agent domain keys are present in every locale', () => {
  const keys = assertDomainParity('agent.', 20);
  for (const key of ['agent.sections.builtin', 'agent.sections.mine', 'agent.sections.sharedEditable', 'agent.sections.sharedReadonly']) {
    assert.ok(keys.includes(key), 'missing agent key: ' + key);
  }
});

test('integrations domain keys are present in every locale', () => {
  const keys = assertDomainParity('integrations.api.', 30);
  for (const key of ['integrations.api.apiKeys', 'integrations.api.createApiKey', 'integrations.api.apiKeyName', 'integrations.api.apiKeyAccessMode', 'integrations.api.copy', 'integrations.api.actions', 'integrations.api.noApiKeys']) {
    assert.ok(keys.includes(key), 'missing integrations key: ' + key);
  }
});

test('embed visitor keys are present in every locale', () => {
  const keys = assertDomainParity('embed.', 5);
  for (const key of ['embed.send', 'embed.inputPlaceholder', 'embed.newChat', 'embed.loading', 'embed.noMessages']) {
    assert.ok(keys.includes(key), 'missing embed key: ' + key);
  }
});

test('ported values stay byte-exact against the Vue baseline', () => {
  assert.equal(formatMessage('en-US', 'organization.title'), 'Shared Spaces');
  assert.equal(formatMessage('en-US', 'organization.invite.primaryJoin'), 'Join');
  assert.equal(formatMessage('en-US', 'organization.invite.submitRequest'), 'Request to Join');
  assert.equal(formatMessage('zh-CN', 'organization.invite.primaryJoin'), '加入');
  assert.equal(formatMessage('en-US', 'agent.sections.builtin'), 'Built-in');
  assert.equal(formatMessage('en-US', 'integrations.api.apiKeys'), 'API Keys');
  assert.equal(formatMessage('en-US', 'embed.send'), 'Send');
  assert.equal(formatMessage('en-US', 'embed.inputPlaceholder'), 'Enter your message...');
});
