import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, supportedLocales } from '@weknora/i18n';
import {
  MOBILE_CAPABILITIES,
  capabilityAction,
  capabilityMessageKeys,
  capabilityModeLabel,
  capabilityModeMessageKey,
  mobileCapability,
  projectMobileCapability,
} from './capabilities.ts';

test('every mobile capability declares its execution mode, reason, and required roles', () => {
  assert.equal(MOBILE_CAPABILITIES.every((item) => item.reason.trim() && item.requiredRoles.length > 0), true);
  assert.deepEqual(
    Object.fromEntries(MOBILE_CAPABILITIES.map((item) => [item.key, item.mode])),
    {
      knowledge: 'native-write',
      chat: 'native-write',
      attachments: 'native-write',
      approvals: 'native-write',
      identity: 'native-write',
      'api-keys': 'native-write',
      'system-runtime': 'web-handoff',
      organizations: 'native-write',
      configuration: 'native-write',
      skills: 'native-read',
      'wiki-faq': 'web-handoff',
      sandbox: 'unsupported',
      'offline-writes': 'unsupported',
      'embed-admin': 'web-handoff',
    },
  );
  assert.deepEqual(mobileCapability('identity')?.requiredRoles, ['owner', 'admin']);
  assert.deepEqual(mobileCapability('api-keys')?.requiredRoles, ['owner']);
  assert.deepEqual(mobileCapability('organizations')?.requiredRoles, ['admin']);
  assert.deepEqual(mobileCapability('system-runtime')?.requiredRoles, ['system_admin']);
  assert.match(mobileCapability('identity')?.reason || '', /server-owned/);
});

test('management hub keeps the four native entry routes and never invents actions', () => {
  assert.deepEqual(capabilityAction(mobileCapability('configuration')!), { kind: 'route', route: '/management/configuration' });
  assert.deepEqual(capabilityAction(mobileCapability('identity')!), { kind: 'route', route: '/management/administration' });
  assert.deepEqual(capabilityAction(mobileCapability('api-keys')!), { kind: 'route', route: '/management/api-keys' });
  assert.deepEqual(capabilityAction(mobileCapability('organizations')!), { kind: 'route', route: '/management/organizations' });
  assert.deepEqual(capabilityAction(mobileCapability('wiki-faq')!), { kind: 'status', label: '转 Web' });
  assert.deepEqual(capabilityAction(mobileCapability('sandbox')!), { kind: 'status', label: '不支持' });
  assert.equal(capabilityModeLabel('native-read'), '只读');
});

test('mobile capability projection fails closed when the server disables a surface', () => {
  const projected = projectMobileCapability(mobileCapability('organizations')!, {
    organizations: { supported: false, reason: 'not_supported_in_lite' },
  });
  assert.equal(projected.reason, 'not_supported_in_lite');
  assert.equal(projected.mode, 'unsupported');
  assert.deepEqual(projected.requiredRoles, ['admin']);
});

test('management capability catalog has localized label, reason, and mode keys', () => {
  for (const locale of supportedLocales) {
    assert.notEqual(formatMessage(locale, 'mobileManagement.serverDisabled'), 'mobileManagement.serverDisabled', `${locale}:serverDisabled`);
    for (const capability of MOBILE_CAPABILITIES) {
      const keys = capabilityMessageKeys(capability.key);
      assert.ok(keys);
      assert.notEqual(formatMessage(locale, keys!.label), keys!.label, `${locale}:${capability.key}:label`);
      assert.notEqual(formatMessage(locale, keys!.reason), keys!.reason, `${locale}:${capability.key}:reason`);
    }
    for (const mode of ['native-write', 'native-read', 'web-handoff', 'unsupported'] as const) {
      const key = capabilityModeMessageKey(mode);
      assert.notEqual(formatMessage(locale, key), key, `${locale}:${mode}`);
    }
  }
});
