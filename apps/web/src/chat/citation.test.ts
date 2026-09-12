import assert from 'node:assert/strict';
import test from 'node:test';

import { externalCitationTarget } from './citation.ts';

test('keeps only absolute HTTP citations as external targets', () => {
  assert.equal(externalCitationTarget('https://example.com/a#source'), 'https://example.com/a#source');
  assert.equal(externalCitationTarget('http://example.com/a'), 'http://example.com/a');
  assert.equal(externalCitationTarget('chunk-1'), null);
  assert.equal(externalCitationTarget('javascript:alert(1)'), null);
  assert.equal(externalCitationTarget('//example.com/a'), null);
});
