export const POLL_INTERVAL_MS = 3000;
export const POLL_MAX_INTERVAL_MS = 30000;
const ACTIVE_STATUSES = new Set(['pending', 'authorizing', 'verifying']);

export function shouldContinueAuthorizationPolling(status: string, expiresAt: string | undefined, now: number): boolean {
  if (!ACTIVE_STATUSES.has(status)) return false;
  if (!expiresAt) return true;
  const expires = Date.parse(expiresAt);
  return Number.isNaN(expires) || expires > now;
}
export function nextAuthorizationPoll(failures: number): number {
  return Math.min(POLL_INTERVAL_MS * 2 ** Math.max(0, failures), POLL_MAX_INTERVAL_MS);
}
