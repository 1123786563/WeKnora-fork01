import assert from 'node:assert/strict';
import test from 'node:test';

import { SETTINGS_SECTIONS, roleAtLeast, settingsSection, settingsSectionsForRole } from './registry.ts';

test('registers every legacy T17 settings section exactly once', () => {
  const keys = SETTINGS_SECTIONS.filter((section) => section.ported || true).map((section) => section.key);
  assert.equal(new Set(keys).size, keys.length);
  for (const section of SETTINGS_SECTIONS) {
    assert.ok(section.viewId.length > 0);
    assert.ok(section.apiDomain.length > 0);
    assert.ok(section.operations.length > 0);
  }
});

test('keeps personal credentials and workspace secrets in separate access scopes', () => {
  assert.equal(settingsSection('envvars')?.scope, 'user');
  assert.equal(settingsSection('weknoracloud')?.scope, 'tenant');
  assert.equal(settingsSection('parser')?.minRole, 'admin');
  assert.equal(settingsSection('general')?.scope, 'local');
  assert.ok(settingsSection('parser')?.operations.includes('test'));
  assert.ok(!settingsSection('general')?.operations.includes('save'));
});

test('registers the parity audit must-fix sections with concrete scopes and roles', () => {
  // models/members are viewer-visible read-only pages in the Vue baseline
  // (frontend/src/config/settingsAccess.ts: models 'viewer', members 'viewer');
  // the panels gate their own write affordances.
  const expected = [
    ['models', 'tenant', 'viewer'],
    ['members', 'tenant', 'viewer'],
    ['mcp', 'tenant', 'admin'],
    ['sandbox', 'tenant', 'admin'],
    ['skills', 'tenant', 'admin'],
    ['system-global', 'platform', 'system-admin'],
    ['runtime-queues', 'platform', 'system-admin'],
    ['platform-api-keys', 'platform', 'system-admin'],
    ['system-audit-log', 'platform', 'system-admin'],
  ] as const;
  for (const [key, scope, minRole] of expected) {
    assert.equal(settingsSection(key)?.scope, scope, key);
    assert.equal(settingsSection(key)?.minRole, minRole, key);
  }
  for (const key of ['models', 'members', 'mcp', 'sandbox', 'skills', 'system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log']) {
    assert.equal(settingsSection(key)?.ported, false, key + ' must be flagged as partially ported');
  }
});

test('ranks roles so a viewer is denied admin and system-admin sections', () => {
  assert.equal(roleAtLeast('viewer', 'viewer'), true);
  assert.equal(roleAtLeast('viewer', 'admin'), false);
  assert.equal(roleAtLeast('admin', 'admin'), true);
  assert.equal(roleAtLeast('owner', 'admin'), true);
  assert.equal(roleAtLeast('owner', 'system-admin'), false);
  assert.equal(roleAtLeast('system-admin', 'system-admin'), true);
});

test('nav filtering by role hides admin-only and system-admin-only sections from a viewer', () => {
  const viewerKeys = settingsSectionsForRole('viewer').map((section) => section.key);
  assert.ok(viewerKeys.includes('general'));
  assert.ok(viewerKeys.includes('envvars'));
  assert.ok(!viewerKeys.includes('retrieval'));
  // Vue keeps models/members visible to viewers as read-only pages
  // (settingsAccess.ts: models 'viewer', members 'viewer').
  assert.ok(viewerKeys.includes('models'));
  assert.ok(viewerKeys.includes('members'));
  assert.ok(!viewerKeys.includes('system-global'));
  assert.ok(!viewerKeys.includes('platform-api-keys'));
  const adminKeys = settingsSectionsForRole('admin').map((section) => section.key);
  assert.ok(adminKeys.includes('retrieval'));
  assert.ok(!adminKeys.includes('system-global'));
  const systemKeys = settingsSectionsForRole('system-admin');
  assert.equal(systemKeys.length, SETTINGS_SECTIONS.length);
});

test('every section resolves through settingsSection and unknown keys return undefined', () => {
  assert.equal(settingsSection('nope'), undefined);
  assert.equal(settingsSection('system-audit-log')?.operations.includes('read'), true);
  assert.equal(settingsSection('system-audit-log')?.operations.includes('save'), false);
});
