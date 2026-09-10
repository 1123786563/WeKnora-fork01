import { get, post } from '../../utils/request'

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

export function getAgentRunEvents(sessionId: string, runId: string, after = 0, limit = 100): Promise<RunEvent[]> {
  return get(`/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/events?after=${after}&limit=${limit}`)
}

export function resolveAgentRunDecision(sessionId: string, runId: string, decision: RunDecision): Promise<RunView> {
  return post(`/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/decisions`, decision)
}

export function cancelAgentRun(sessionId: string, runId: string): Promise<RunView> {
  return post(`/api/v1/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/cancel`, {})
}
