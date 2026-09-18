import assert from 'node:assert/strict';
import test from 'node:test';

import { createChatSubmission, isSteerInjectShortcut, resolveSteerAttachmentWarning, resolveSteerInjectAction, shouldSubmitFromKeyboard, steerShortcutLabel, type ChatAttachmentView, type ChatSteerQueueChip } from './composer.tsx';

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

/*
 * R477-A2 — Vue Input-field.vue steer-path attachment gates (two warnings,
 * checked in this order before a steer dispatch):
 *   1. uploadedAttachments.some(status === 'uploading')
 *        → warning input.messages.steerAttachmentPending
 *   2. uploadedAttachments.length || uploadedImages.length
 *        → warning input.messages.steerHasAttachments
 * The React composer keeps one unified attachment list, so gate 2 is any
 * entry present; a still-uploading file always wins (gate 1) exactly like
 * the Vue some() check runs before the length check.
 */
test('resolveSteerAttachmentWarning mirrors the Vue two-key steer attachment gates', () => {
  const uploading: ChatAttachmentView[] = [{ id: 'local-1', name: 'spec.pdf', status: 'uploading' }];
  const settled: ChatAttachmentView[] = [{ id: 'att-1', name: 'guide.txt', status: 'ready', attachmentId: 'att-1' }];
  const failed: ChatAttachmentView[] = [{ id: 'local-2', name: 'broken.csv', status: 'failed', error: 'Upload failed' }];

  assert.equal(resolveSteerAttachmentWarning([]), null, 'no attachments → steer proceeds');
  assert.equal(resolveSteerAttachmentWarning(uploading), 'steerAttachmentPending', 'an in-flight upload → the pending warning');
  assert.equal(resolveSteerAttachmentWarning([...settled, ...uploading]), 'steerAttachmentPending', 'the uploading gate runs before the presence gate');
  assert.equal(resolveSteerAttachmentWarning(settled), 'steerHasAttachments', 'a settled attachment → the cannot-attach warning');
  assert.equal(resolveSteerAttachmentWarning(failed), 'steerHasAttachments', 'a failed attachment still blocks the steer path');
});
