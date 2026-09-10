import assert from 'node:assert/strict';
import test from 'node:test';
import { INTEGRATION_SECTIONS } from './registry.ts';

test('external integration registry entries expose actionable guide urls', () => {
  for (const section of INTEGRATION_SECTIONS.filter((item) => item.external)) {
    assert.match(section.externalUrl ?? '', /^https:\/\//);
  }
});
