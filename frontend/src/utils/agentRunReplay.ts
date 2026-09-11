import { applyRunEvent, type ClientRunState, type RunEvent } from './agentRunState'

/** Run statuses after which no further events or decisions are possible. */
export const TERMINAL_RUN_STATUSES = ['succeeded', 'failed', 'canceled'] as const

export function isTerminalRunStatus(status: string | undefined | null): boolean {
  return TERMINAL_RUN_STATUSES.includes(status as (typeof TERMINAL_RUN_STATUSES)[number])
}

export interface RunReplayUpdate {
  /** Whether any consumed event advanced the replay state. */
  changed: boolean
  /** The rendered attempt changed; the caller must clear the streamed text before applying `delta`. */
  reset: boolean
  /** Text the UI has not rendered yet (the full attempt text right after a reset). */
  delta: string
  /** Last applied seq; use as the next replay cursor. */
  seq: number
  attemptId: string
}

const INITIAL_STATE: ClientRunState = { seq: 0, attemptId: '', text: '' }

/**
 * Folds durable run events for one run into render updates.
 *
 * Events go through applyRunEvent (seq dedupe + attempt replacement) while the
 * helper remembers what it already handed to the UI, so callers append each
 * piece of text exactly once — replaying an overlapping cursor after a
 * reconnect never duplicates half-streamed text, and an attempt replacement
 * is surfaced as reset + the new attempt's full text.
 */
export function createRunReplay() {
  let state: ClientRunState = { ...INITIAL_STATE }
  let renderedAttemptId = ''
  let renderedText = ''

  const consume = (events: RunEvent[]): RunReplayUpdate => {
    let changed = false
    for (const event of events) {
      // Bootstrap: applyRunEvent only folds events for the current attempt,
      // and a virgin state has none — adopt the first event's attempt so the
      // opening answer deltas are not dropped.
      if (!state.attemptId && event.attempt_id) {
        state = { ...state, attemptId: event.attempt_id }
        changed = true
      }
      const next = applyRunEvent(state, event)
      if (next === state) continue
      state = next
      changed = true
    }
    if (!changed) {
      return { changed: false, reset: false, delta: '', seq: state.seq, attemptId: state.attemptId }
    }
    const attemptChanged = state.attemptId !== renderedAttemptId
    if (attemptChanged || !state.text.startsWith(renderedText)) {
      // New attempt, or the attempt rewrote its own text (an `answer`
      // snapshot event): re-render from scratch with the accumulated text.
      renderedAttemptId = state.attemptId
      renderedText = state.text
      return { changed: true, reset: true, delta: state.text, seq: state.seq, attemptId: state.attemptId }
    }
    const delta = state.text.slice(renderedText.length)
    renderedText = state.text
    return { changed: true, reset: false, delta, seq: state.seq, attemptId: state.attemptId }
  }

  return { consume }
}
