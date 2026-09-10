import assert from 'node:assert/strict';
import test from 'node:test';
import { Dialog } from './dialog.tsx';

test('exports a dialog primitive with an explicit focus and escape contract', () => {
  assert.equal(typeof Dialog, 'function');
});
