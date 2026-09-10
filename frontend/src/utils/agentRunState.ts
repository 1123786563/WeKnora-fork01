export interface RunEvent {
  seq: number
  attempt_id: string
  type: string
  payload: Record<string, unknown>
}

export interface ClientRunState {
  seq: number
  attemptId: string
  text: string
}

/** Apply durable run events in sequence, making reconnect/replay idempotent. */
export function applyRunEvent(state: ClientRunState, event: RunEvent): ClientRunState {
  if (event.seq <= state.seq) return state
  if (event.type === 'attempt_replaced') {
    return { seq: event.seq, attemptId: event.attempt_id, text: '' }
  }
  if (event.attempt_id !== state.attemptId) return state
  if (event.type === 'answer_delta' || event.type === 'answer') {
    const value = event.payload.text
    return typeof value === 'string'
      ? { ...state, seq: event.seq, text: event.type === 'answer' ? value : state.text + value }
      : { ...state, seq: event.seq }
  }
  return { ...state, seq: event.seq }
}
