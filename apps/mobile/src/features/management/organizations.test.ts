import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, supportedLocales } from '@weknora/i18n';
import { canCreateOrganization, canManageOrganization, shareResourceId, shareResourceLabel, validateOrganizationDraft } from './organizations.ts';

test('mobile organization writes are limited to organization admins', () => {
  assert.equal(canManageOrganization({ my_role: 'admin' }), true);
  assert.equal(canManageOrganization({ my_role: 'viewer' }), false);
  assert.equal(canManageOrganization({ owner_id: 'owner-1' }), false);
});

test('mobile organization creation requires an owner or admin workspace role', () => {
  assert.equal(canCreateOrganization('owner'), true);
  assert.equal(canCreateOrganization('admin'), true);
  assert.equal(canCreateOrganization('contributor'), false);
  assert.equal(canCreateOrganization(undefined), false);
});

test('mobile organization creation validates the server-owned name boundary', () => {
  assert.deepEqual(validateOrganizationDraft(' ', 'description'), ['Name is required']);
  assert.deepEqual(validateOrganizationDraft('Research', 'description'), []);
});

test('mobile shared-resource rows preserve the server id required for a confirmed removal', () => {
  const knowledgeBase = { id: 'share-kb-1', knowledge_base_id: 'kb-1', knowledge_base_name: 'Support KB', permission: 'editor' };
  const agent = { id: 'share-agent-1', agent_id: 'agent-1', agent_name: 'Support agent' };

  assert.equal(shareResourceId(knowledgeBase, 'knowledge-base'), 'kb-1');
  assert.equal(shareResourceId(agent, 'agent'), 'agent-1');
  assert.equal(shareResourceLabel(knowledgeBase, 'knowledge-base'), 'Support KB');
  assert.equal(shareResourceLabel(agent, 'agent'), 'Support agent');
  assert.equal(shareResourceId({ id: 'missing-resource' }, 'agent'), null);
});


test('mobile shared-resource labels localize missing names in every supported locale', () => {
  const missing = { id: 'share-missing' };
  for (const locale of supportedLocales) {
    const format = (key: string) => formatMessage(locale, key);
    assert.equal(shareResourceLabel(missing, 'knowledge-base', format), formatMessage(locale, 'mobileOrganization.unnamedKnowledgeBase'), locale);
    assert.equal(shareResourceLabel(missing, 'agent', format), formatMessage(locale, 'mobileOrganization.unnamedAgent'), locale);
  }
  assert.equal(shareResourceLabel(missing, 'agent'), 'mobileOrganization.unnamedAgent');
});
