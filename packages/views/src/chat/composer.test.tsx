import assert from 'node:assert/strict';
import test from 'node:test';

import { createChatSubmission, shouldSubmitFromKeyboard } from './composer.tsx';

test('does not create a pending message from an empty draft', () => {
  assert.throws(() => createChatSubmission('   '), /message must not be empty/);
});

test('creates an explicit pending user message without pretending it was sent', () => {
  assert.deepEqual(createChatSubmission('  What changed?  '), {
    content: 'What changed?',
    status: 'pending',
  });
});

test('matches Vue keyboard rules for modifiers and IME composition', () => {
  const enter = { key: 'Enter', keyCode: 13, isComposing: false, shiftKey: false, ctrlKey: false, altKey: false, metaKey: false };
  assert.equal(shouldSubmitFromKeyboard(enter, false), true);
  assert.equal(shouldSubmitFromKeyboard({ ...enter, shiftKey: true }, false), false);
  assert.equal(shouldSubmitFromKeyboard({ ...enter, ctrlKey: true }, false), false);
  assert.equal(shouldSubmitFromKeyboard({ ...enter, isComposing: true }, false), false);
  assert.equal(shouldSubmitFromKeyboard({ ...enter, altKey: true }, false), false);
  assert.equal(shouldSubmitFromKeyboard({ ...enter, altKey: true }, true), true);
  assert.equal(shouldSubmitFromKeyboard({ ...enter, metaKey: true }, false), true);
});
