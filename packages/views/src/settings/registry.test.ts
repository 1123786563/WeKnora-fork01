import assert from 'node:assert/strict';
import test from 'node:test';

import { SETTINGS_SECTIONS, settingsSection } from './registry.ts';

test('registers every legacy T17 settings section exactly once', () => {
  const keys = SETTINGS_SECTIONS.map((section) => section.key);
  assert.deepEqual(keys, [
    'general', 'tenant', 'userprofile', 'ollama', 'parser', 'retrieval', 'memory',
    'mymemory', 'envvars', 'storage', 'vectorstore', 'websearch', 'chathistory',
    'system', 'weknoracloud',
  ]);
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
