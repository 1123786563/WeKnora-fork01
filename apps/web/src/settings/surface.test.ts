import assert from 'node:assert/strict';
import test from 'node:test';

import { settingsSectionMeta, settingsValueEntries } from './surface.ts';
import { SETTINGS_SECTIONS } from '@weknora/views';

test('gives every registered settings section a concrete inventory description', () => {
  assert.equal(settingsSectionMeta('general')?.title, 'General and preferences');
  assert.equal(settingsSectionMeta('weknoracloud')?.scope, 'tenant');
  assert.equal(SETTINGS_SECTIONS.every((section) => settingsSectionMeta(section.key)), true);
});

test('keeps settings operations explicit and does not display secret-shaped fields', () => {
  const section = settingsSectionMeta('storage');
  assert.deepEqual(section?.operations, ['read', 'save', 'test', 'delete', 'unavailable']);
  assert.deepEqual(settingsValueEntries({ name: 's3', api_key: 'hidden', configured: true, nested: { region: 'cn' } }), [
    ['name', 's3'],
    ['configured', 'true'],
    ['nested', '{"region":"cn"}'],
  ]);
});
