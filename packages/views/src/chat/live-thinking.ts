/*
 * Vue main-face streaming thinking split (frontend/src/composables/
 * useChatStreamHandler.ts processStreamChunk): the non-agent chat face drives
 * its deepThink indicator from `<think>`/`</think>` tags inside the
 * accumulated answer content — not from SSE `thinking` events. An open
 * `<think>` keeps the block live with the answer held back; the last
 * `</think>` folds the block and only the post-tag text stays visible.
 */
export interface LiveThinkingState {
  /** Vue `showThink`: a think block exists (live or completed). */
  showThink: boolean;
  /** Vue `thinking`: true while the block is still streaming. */
  thinking: boolean;
  /** Text inside the think block (first `<think>` tag stripped, Vue parity). */
  thinkContent: string;
  /** Visible answer: empty while thinking, the post-tag text once closed. */
  answer: string;
}

const THINK_OPEN = '<think>';
const THINK_CLOSE = '</think>';

const NO_THINK: LiveThinkingState = { showThink: false, thinking: false, thinkContent: '', answer: '' };

export function splitLiveThinking(content: string): LiveThinkingState {
  if (!content.includes(THINK_OPEN)) {
    return { ...NO_THINK, answer: content };
  }
  if (!content.includes(THINK_CLOSE)) {
    return {
      showThink: true,
      thinking: true,
      thinkContent: content.replace(THINK_OPEN, '').trim(),
      answer: '',
    };
  }
  const closeIndex = content.lastIndexOf(THINK_CLOSE);
  return {
    showThink: true,
    thinking: false,
    thinkContent: content.substring(0, closeIndex).replace(THINK_OPEN, ''),
    answer: content.substring(closeIndex + THINK_CLOSE.length).trim(),
  };
}
