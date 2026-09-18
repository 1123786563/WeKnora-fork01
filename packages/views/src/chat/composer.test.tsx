import assert from 'node:assert/strict';
import test from 'node:test';

import { createChatSubmission, isSteerInjectShortcut, resolveSteerInjectAction, shouldSubmitFromKeyboard, steerShortcutLabel, type ChatSteerQueueChip } from './composer.tsx';

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

/*
 * R474-A2 — Vue Input-field.vue inject shortcut (frontend/src/utils/
 * chatSubmitShortcut.ts): ⌘Enter steers unconditionally, Alt+Enter only on a
 * steer-capable turn; shift/ctrl and IME composition never trigger it.
 */
test('matches the Vue inject shortcut modifiers', () => {
  const enter = { key: 'Enter', keyCode: 13, isComposing: false, shiftKey: false, ctrlKey: false, altKey: false, metaKey: false };
  assert.equal(isSteerInjectShortcut({ ...enter, metaKey: true }, false), true);
  assert.equal(isSteerInjectShortcut({ ...enter, metaKey: true }, true), true);
  assert.equal(isSteerInjectShortcut({ ...enter, altKey: true }, true), true);
  assert.equal(isSteerInjectShortcut({ ...enter, altKey: true }, false), false);
  assert.equal(isSteerInjectShortcut(enter, true), false);
  assert.equal(isSteerInjectShortcut({ ...enter, metaKey: true, shiftKey: true }, true), false);
  assert.equal(isSteerInjectShortcut({ ...enter, metaKey: true, ctrlKey: true }, true), false);
  assert.equal(isSteerInjectShortcut({ ...enter, metaKey: true, isComposing: true }, true), false);
});

/*
 * R474-A2 — Vue Input-field.vue injectCurrentInput + firstQueuedSteer: the
 * ⌘Enter/Alt+Enter inject shortcut sends the current draft when it has
 * content; with an empty draft it promotes the first queued after-message
 * (delivery 'after', not pending/promoting/failed).
 */
test('resolves the inject shortcut onto the draft or the first queued steer', () => {
  const chips: ChatSteerQueueChip[] = [
    { steerId: 's-pending', content: 'still posting', status: 'pending' },
    { steerId: 's-failed', content: 'failed retry', status: 'failed' },
    { steerId: 's-first', content: 'first queued', status: 'queued' },
    { steerId: 's-second', content: 'second queued', status: 'queued' },
  ];
  assert.deepEqual(
    resolveSteerInjectAction({ draft: '  typed now  ', steerQueue: chips }),
    { kind: 'submit' },
    'a non-empty draft wins over the queue (Vue createSession inject)',
  );
  assert.deepEqual(
    resolveSteerInjectAction({ draft: '   ', steerQueue: chips }),
    { kind: 'promote', steerId: 's-first' },
    'an empty draft promotes the first queued chip',
  );
  assert.deepEqual(
    resolveSteerInjectAction({ draft: '', steerQueue: [] }),
    undefined,
    'nothing to inject: empty draft and empty queue',
  );
  assert.deepEqual(
    resolveSteerInjectAction({
      draft: '',
      steerQueue: [
        { steerId: 's-pending', content: 'posting', status: 'pending' },
        { steerId: 's-failed', content: 'failed', status: 'failed' },
      ],
    }),
    undefined,
    'pending and failed chips are not promotable via the shortcut',
  );
});

test('the steer shortcut label renders SSR-safe like the Vue tooltip suffix', () => {
  assert.ok(/^(⌘ Enter|Alt\+Enter)$/.test(steerShortcutLabel()));
});
