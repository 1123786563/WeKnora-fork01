// Poll backoff for the authorization-attempt watcher — verbatim port of
// frontend/src/views/apps/pollBackoff.ts (T15 QF-3 / T16 hardening QF-01):
// consecutive failures double the next poll delay, capped at
// POLL_MAX_INTERVAL_MS; a success resets the counter to zero. Pure logic so
// the growth curve is testable (pollBackoff.test.ts).
export const POLL_INTERVAL_MS = 3000
export const POLL_MAX_INTERVAL_MS = 30000

export const pollBackoffDelayMs = (failures: number): number => {
  const n = failures > 0 ? Math.floor(failures) : 0
  return Math.min(POLL_INTERVAL_MS * 2 ** n, POLL_MAX_INTERVAL_MS)
}
