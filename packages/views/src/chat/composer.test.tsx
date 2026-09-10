import assert from 'node:assert/strict';
import test from 'node:test';

import { createChatSubmission } from './composer.tsx';

test('does not create a pending message from an empty draft', () => {
  assert.throws(() => createChatSubmission('   '), /message must not be empty/);
});

test('creates an explicit pending user message without pretending it was sent', () => {
  assert.deepEqual(createChatSubmission('  What changed?  '), {
    content: 'What changed?',
    status: 'pending',
  });
});
