import assert from 'node:assert/strict';
import test from 'node:test';

import {
  canViewSection,
  getVisibleSettingsTabs,
  normalizeSettingsTab,
  saveSettingsOnce,
  validateSettingsForm,
} from './state.ts';

test('normalizes legacy integration tab aliases to the Vue canonical section', () => {
  assert.equal(normalizeSettingsTab('integrations', 'cli'), 'integration-cli');
  assert.equal(normalizeSettingsTab('api'), 'integration-api');
  assert.equal(normalizeSettingsTab('integration-claw'), 'integration-claw');
  assert.equal(normalizeSettingsTab(undefined), 'general');
});

test('filters tabs by the same minimum roles as the settings navigation', () => {
  assert.equal(canViewSection('general', 'viewer'), true);
  assert.equal(canViewSection('websearch', 'viewer'), false);
  assert.equal(canViewSection('websearch', 'admin'), true);
  assert.equal(canViewSection('system-global', 'admin'), false);
  assert.equal(canViewSection('system-global', 'systemAdmin'), true);
  assert.deepEqual(getVisibleSettingsTabs('viewer'), [
    'general', 'models', 'system', 'userprofile', 'mymemory', 'envvars', 'tenant', 'members',
  ]);
  assert.deepEqual(getVisibleSettingsTabs('admin'), [
    'general', 'ollama', 'weknoracloud', 'models', 'websearch', 'chathistory', 'memory',
    'vectorstore', 'parser', 'storage', 'sandbox', 'skills', 'mcp', 'system', 'userprofile',
    'mymemory', 'envvars', 'tenant', 'members', 'integration-api', 'integration-cli',
    'integration-claw', 'integration-im',
  ]);
});

test('requires a name and provider before a settings form can submit', () => {
  assert.deepEqual(validateSettingsForm({ name: '', provider: '' }), {
    name: 'Name is required',
    provider: 'Provider is required',
  });
  assert.deepEqual(validateSettingsForm({ name: '  OpenAI ', provider: 'openai' }), {});
});

test('permission helper keeps read-only settings visible and blocks writes', () => {
  assert.equal(canViewSection('models', 'viewer'), true);
  assert.equal(canViewSection('models', 'contributor'), true);
  assert.equal(canViewSection('websearch', 'contributor'), false);
  assert.equal(canViewSection('models', 'admin'), true);
});

test('deduplicates concurrent saves and clears the gate after settlement', async () => {
  let calls = 0;
  const inFlight = { current: null as Promise<string> | null };
  let resolve!: (value: string) => void;
  const save = () => {
    calls += 1;
    return new Promise<string>((finish) => { resolve = finish; });
  };

  const first = saveSettingsOnce(save, inFlight);
  const second = saveSettingsOnce(save, inFlight);
  assert.equal(calls, 1);
  assert.equal(first, second);
  resolve('saved');
  assert.equal(await first, 'saved');
  assert.equal(inFlight.current, null);
});
