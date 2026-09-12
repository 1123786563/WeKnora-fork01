/**
 * Bounded reconnect policy for the chat SSE stream.
 *
 * fetch-event-source retries automatically unless onerror throws, but its
 * default reconnect is immediate and unbounded. These helpers turn that into
 * a fixed retry budget with growing delays so a dead backend fails loudly
 * instead of hammering it forever.
 */

/** Delay before each reconnect attempt. Exhausted budget = no more retries. */
export const STREAM_RETRY_DELAYS_MS = [1000, 2000, 4000] as const

export function streamRetryDelayMs(retriesSoFar: number): number | null {
  if (!Number.isInteger(retriesSoFar) || retriesSoFar < 0) return null
  if (retriesSoFar >= STREAM_RETRY_DELAYS_MS.length) return null
  return STREAM_RETRY_DELAYS_MS[retriesSoFar]
}

/**
 * Whether a stream failure is worth reconnecting. Auth errors are handled by
 * the token-refresh wrapper, user aborts end the stream on purpose, and
 * client-error statuses (except 429) would fail identically again.
 */
export function isStreamRetryableError(error: unknown, signalAborted: boolean): boolean {
  if (signalAborted) return false
  if (error === null || error === undefined) return true // network drop without a status
  const status = streamErrorStatus(error)
  if (status === undefined) return true // no HTTP status attached: transport-level failure
  if (status === 429) return true
  return status >= 500
}

/**
 * HTTP status carried by an error thrown from onopen (`HTTP <status>`) or by
 * axios/fetch wrappers (`status` / `$httpStatus` fields).
 */
export function streamErrorStatus(error: unknown): number | undefined {
  if (error === null || typeof error !== 'object') return undefined
  const candidate = error as Record<string, unknown>
  if (typeof candidate.$httpStatus === 'number') return candidate.$httpStatus
  if (typeof candidate.status === 'number') return candidate.status
  if (typeof candidate.message === 'string') {
    const match = /\b(?:HTTP|status)\s+(\d{3})\b/i.exec(candidate.message)
    if (match) return Number(match[1])
  }
  return undefined
}
