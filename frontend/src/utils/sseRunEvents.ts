import type { RunEvent } from './agentRunState'

/**
 * The durable-run events endpoint always answers with an SSE body
 * (text/event-stream), even for one-shot replay requests (`once=1`). axios
 * hands the raw body back as a string, so replay consumers parse it here.
 *
 * Only run events survive the filter: objects carrying a numeric `seq` and a
 * `type`. Control frames the server interleaves (keepalive snapshots,
 * `run` views, `error` notices) are dropped — callers learn about failures
 * from the HTTP status or the run view instead.
 */
export function parseSseRunEvents(raw: unknown): RunEvent[] {
  const text = typeof raw === 'string' ? raw : ''
  if (!text) return []
  const events: RunEvent[] = []
  const lines = text.split('\n').map((line) => line.endsWith('\r') ? line.slice(0, -1) : line)
  for (const line of lines) {
    if (!line.startsWith('data:')) continue
    const payload = line.slice('data:'.length).trim()
    if (!payload) continue
    let parsed: unknown
    try {
      parsed = JSON.parse(payload)
    } catch {
      continue
    }
    if (!isRunEvent(parsed)) continue
    events.push(parsed)
  }
  return events
}

function isRunEvent(value: unknown): value is RunEvent {
  if (value === null || typeof value !== 'object') return false
  const candidate = value as Record<string, unknown>
  return typeof candidate.seq === 'number'
    && typeof candidate.type === 'string'
    && typeof candidate.attempt_id === 'string'
}
