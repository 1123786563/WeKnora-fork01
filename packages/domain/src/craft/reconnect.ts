// C03 craft reconnection rules (pure domain).
//
// replayAction is the single authority for what an incoming run-event seq
// means against the client cursor: duplicates replay as no-ops, the exact
// next seq applies, and anything further away is a REAL GAP — gaps are never
// bridged by guessing, they demand an authoritative snapshot reload (事件缺
// 口走权威重载).
//
// reconnectDelayMs pins the C03 Step 5 backoff schedule — 1/2/4/8/15s with
// relative jitter, hard-capped at 15s — so the controller only counts
// consecutive attempts and injects this function (tests inject zero delays).
/** Base backoff steps for consecutive reconnect attempts (0-based). */
export const RECONNECT_BACKOFF_STEPS_MS: readonly number[] = [1000, 2000, 4000, 8000, 15000];

/** Hard ceiling for any single reconnect delay, jitter included. */
export const RECONNECT_BACKOFF_CAP_MS = 15000;

/** Relative jitter fraction added on top of the base step (0 ≤ f < 1). */
const RECONNECT_JITTER_FRACTION = 0.2;

export function replayAction(last: number, incoming: number): 'drop' | 'apply' | 'reload' {
  if (incoming <= last) return 'drop';
  return incoming === last + 1 ? 'apply' : 'reload';
}

export function reconnectDelayMs(attempt: number, random: () => number = Math.random): number {
  const index = Math.max(0, Math.floor(attempt));
  const base = RECONNECT_BACKOFF_STEPS_MS[Math.min(index, RECONNECT_BACKOFF_STEPS_MS.length - 1)]!;
  const jitter = random() * base * RECONNECT_JITTER_FRACTION;
  return Math.min(base + jitter, RECONNECT_BACKOFF_CAP_MS);
}
