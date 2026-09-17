import assert from 'node:assert/strict';
import test from 'node:test';

import { splitLiveThinking } from './live-thinking.ts';

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
