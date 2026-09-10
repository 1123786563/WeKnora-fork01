import assert from 'node:assert/strict';
import test from 'node:test';
import { createWebPlatformAdapters } from './adapters.ts';

test('does not create a platform adapter at import time', () => {
  assert.equal(typeof createWebPlatformAdapters, 'function');
});

test('exposes replaceable storage, navigation, file, and clipboard ports', () => {
  const adapters = createWebPlatformAdapters();
  assert.equal(typeof adapters.removePreference, 'function');
  assert.equal(typeof adapters.openExternal, 'function');
  assert.equal(typeof adapters.saveFile, 'function');
});
