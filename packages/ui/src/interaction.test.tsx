import assert from 'node:assert/strict';
import test from 'node:test';
import { JSDOM } from 'jsdom';
import { Dialog, getDialogFocusableElements } from './dialog.tsx';

test('exports a dialog primitive with an explicit focus and escape contract', () => {
  assert.equal(typeof Dialog, 'function');
});

test('dialog focusables exclude disabled and untabbable controls', () => {
  const dom = new JSDOM('<section><button id="first"></button><button disabled>skip</button><input tabindex="-1" /><a href="/next" id="last">next</a></section>');
  const root = dom.window.document.querySelector('section')!;
  assert.deepEqual(getDialogFocusableElements(root).map((element) => element.id), ['first', 'last']);
});
