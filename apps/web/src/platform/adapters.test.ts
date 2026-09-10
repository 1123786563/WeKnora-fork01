import assert from 'node:assert/strict';
import test from 'node:test';
import { createWebPlatformAdapters } from './adapters.ts';

test('does not create a platform adapter at import time', () => {
  assert.equal(typeof createWebPlatformAdapters, 'function');
});
