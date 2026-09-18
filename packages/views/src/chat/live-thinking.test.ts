import assert from 'node:assert/strict';
import test from 'node:test';

import { splitHistoryThinking, splitLiveThinking } from './live-thinking.ts';

/*
 * Vue contract (frontend/src/composables/useChatStreamHandler.ts processStreamChunk):
 * the non-agent streaming face parses the accumulated answer content for
 * `<think>`/`</think>` tags. An open `<think>` keeps the deepThink block in its
 * live "thinking" state with the answer held back; a closed block folds to the
 * completed state and only the post-tag text renders as the answer.
 */
test('an open <think> tag keeps the live block thinking with the answer held back', () => {
  assert.deepEqual(splitLiveThinking('<think>first step\nsecond step'), {
    showThink: true,
    thinking: true,
    thinkContent: 'first step\nsecond step',
    answer: '',
  });
});

test('a closed think block folds and streams only the post-tag answer', () => {
  assert.deepEqual(splitLiveThinking('<think>reasoning</think>the answer'), {
    showThink: true,
    thinking: false,
    thinkContent: 'reasoning',
    answer: 'the answer',
  });
});

test('an empty but closed think block still shows the folded deep-think header', () => {
  assert.deepEqual(splitLiveThinking('<think></think>answer text'), {
    showThink: true,
    thinking: false,
    thinkContent: '',
    answer: 'answer text',
  });
});

test('content without think tags renders as the plain answer', () => {
  assert.deepEqual(splitLiveThinking('plain answer'), {
    showThink: false,
    thinking: false,
    thinkContent: '',
    answer: 'plain answer',
  });
  assert.deepEqual(splitLiveThinking(''), {
    showThink: false,
    thinking: false,
    thinkContent: '',
    answer: '',
  });
});

test('the last close tag wins when a message repeats think blocks', () => {
  assert.deepEqual(splitLiveThinking('<think>one</think>mid<think>two</think>final'), {
    showThink: true,
    thinking: false,
    thinkContent: 'one</think>mid<think>two',
    answer: 'final',
  });
});

/*
 * Vue history contract (frontend/src/composables/useChatStreamHandler.ts
 * handleMsgList restore branch): a persisted assistant answer that contains
 * `<think>…</think>` is split once at load time — showThink=true with
 * thinking=false (deepThink.vue mounts folded under 「已深度思考」), thinkContent
 * is the pre-close text minus the first open tag (trimmed), and only the
 * post-tag remainder stays as the rendered content.
 */
test('history split: a closed think block loads folded with the tag stripped', () => {
  assert.deepEqual(splitHistoryThinking('<think>reasoning</think>the answer'), {
    showThink: true,
    thinking: false,
    thinkContent: 'reasoning',
    answer: 'the answer',
  });
});

test('history split mirrors the Vue restore trim (content trimmed, thinkContent trimmed)', () => {
  // Vue: content.trim() then substring; thinkContent .trim(); the answer only
  // loses the whole-content outer trim, not the spacing after the close tag.
  assert.deepEqual(splitHistoryThinking('  <think> reasoning </think>  answer '), {
    showThink: true,
    thinking: false,
    thinkContent: 'reasoning',
    answer: '  answer',
  });
});

test('history split: an unclosed think block restores the live thinking state', () => {
  // A crashed/interrupted persisted turn keeps thinking=true and hides the
  // answer exactly like the Vue restore branch.
  assert.deepEqual(splitHistoryThinking('<think>partial reasoning'), {
    showThink: true,
    thinking: true,
    thinkContent: 'partial reasoning',
    answer: '',
  });
});

test('history split: plain persisted answers pass through untouched', () => {
  assert.deepEqual(splitHistoryThinking('plain answer'), {
    showThink: false,
    thinking: false,
    thinkContent: '',
    answer: 'plain answer',
  });
});

test('history split: the last close tag wins like the Vue lastIndexOf restore', () => {
  assert.deepEqual(splitHistoryThinking('<think>one</think>mid<think>two</think>final'), {
    showThink: true,
    thinking: false,
    thinkContent: 'one</think>mid<think>two',
    answer: 'final',
  });
});
