/**
 * Fork affordance + landing stash, ported from upstream views/chat/forkPoint.ts
 * and index.vue handleFork/stashForkLanding (L318-410). Pure helpers so the
 * chat view can show the right button state without a round trip, and so the
 * prefilled question survives navigation into the forked session.
 */

export interface ForkCandidateMessage {
  id?: unknown;
  role?: unknown;
  is_completed?: unknown;
}

export interface ForkAffordance {
  /** Whether the fork button should be offered on this message at all. */
  canFork: boolean;
}

const REFUSED: ForkAffordance = { canFork: false };

export function resolveForkAffordance(
  messages: readonly ForkCandidateMessage[],
  messageId: string,
): ForkAffordance {
  const index = messages.findIndex((m) => m.id === messageId);
  if (index < 0) {
    return REFUSED;
  }

  // A turn still streaming means the source session holds an active sandbox
  // lease. The backend would answer 409, so do not offer the button.
  if (messages.some((m) => m.role === 'assistant' && m.is_completed === false)) {
    return REFUSED;
  }

  const target = messages[index];
  if (target.role === 'assistant' || target.role === 'user') {
    return { canFork: true };
  }
  return REFUSED;
}

/** Prefill carried across the SPA navigation into the forked session. */
const FORK_PREFILL_KEY = 'weknora:fork-prefill';

export interface ForkLanding {
  sessionId: string;
  text: string;
}

/** sessionStorage can throw in private mode; landing still navigates. */
export function stashForkLanding(sessionId: string, text: string, storage: Pick<Storage, 'setItem'> = sessionStorage): void {
  try {
    storage.setItem(FORK_PREFILL_KEY, JSON.stringify({ sessionId, text }));
  } catch {
    // best-effort only
  }
}

/** Returns and clears the landing for sessionId, or null when none matches. */
export function takeForkLanding(sessionId: string, storage: Pick<Storage, 'getItem' | 'removeItem'> = sessionStorage): ForkLanding | null {
  let raw: string | null = null;
  try {
    raw = storage.getItem(FORK_PREFILL_KEY);
  } catch {
    return null;
  }
  if (!raw) return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!parsed || typeof parsed !== 'object') return null;
  const record = parsed as Record<string, unknown>;
  const landing: ForkLanding = {
    sessionId: String(record.sessionId || ''),
    text: String(record.text || ''),
  };
  if (!landing.sessionId || landing.sessionId !== sessionId) return null;
  try {
    storage.removeItem(FORK_PREFILL_KEY);
  } catch {
    // best-effort only
  }
  return landing;
}
