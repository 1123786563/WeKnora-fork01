import { get, post } from '../../utils/request'
import { parseSseRunEvents } from '../../utils/sseRunEvents'

export interface RunView {
  run_id: string
  status: string
  revision: number
  seq: number
  pending_id?: string
  wait_reason?: string
  engine_type?: string
}

export interface RunEvent {
  seq: number
  attempt_id: string
  type: string
  payload: Record<string, unknown>
}

export interface RunDecision {
  pending_id: string
  decision_id: string
  expected_revision: number
  action: 'retry' | 'provide_result' | 'terminate'
  reason?: string
  result?: Record<string, unknown>
}

export function getAgentRun(sessionId: string, runId: string): Promise<RunView> {
  return get(`/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}`)
}

// The events endpoint always answers with an SSE body (text/event-stream):
// `once=1` makes the request return once the buffered events are flushed,
// and the raw body is parsed into RunEvent[] here. A cursor the server no
// longer keeps rejects with HTTP 409 code=cursor_expired.
export async function getAgentRunEvents(sessionId: string, runId: string, after = 0, limit = 100): Promise<RunEvent[]> {
  const body = await get<unknown>(
    `/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/events?after=${after}&limit=${limit}&once=1`,
  )
  return parseSseRunEvents(body)
}

export function resolveAgentRunDecision(sessionId: string, runId: string, decision: RunDecision): Promise<RunView> {
  return post(`/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/decisions`, decision)
}

export function cancelAgentRun(sessionId: string, runId: string): Promise<RunView> {
  return post(`/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/cancel`, {})
}
