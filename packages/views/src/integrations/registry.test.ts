import assert from 'node:assert/strict';
import test from 'node:test';

import { INTEGRATION_SECTIONS, integrationSection } from './registry.ts';

test('registers every external integration entry without an empty navigation target', () => {
  assert.deepEqual(INTEGRATION_SECTIONS.map((item) => item.key), ['im', 'embed', 'api', 'cli', 'chrome', 'claw']);
  for (const item of INTEGRATION_SECTIONS) {
    assert.ok(item.viewId.length > 0);
    assert.ok(item.operations.length > 0);
  }
});

test('keeps API ownership and embed/IM capabilities explicit', () => {
  assert.equal(integrationSection('api')?.minRole, 'owner');
  assert.equal(integrationSection('embed')?.apiDomain, 'channels');
  assert.equal(integrationSection('im')?.apiDomain, 'im');
  assert.equal(integrationSection('chrome')?.external, true);
  assert.ok(integrationSection('embed')?.operations.includes('manage'));
});
