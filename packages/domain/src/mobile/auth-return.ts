const pendingStates = new Set<string>();

export function createAuthState(random: () => string = () => {
  const bytes = new Uint8Array(24);
  globalThis.crypto?.getRandomValues?.(bytes);
  return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('') || `${Date.now()}-${Math.random()}`;
}): string {
  const state = random().trim();
  if (!state) throw new Error('AUTH_STATE');
  pendingStates.add(state);
  return state;
}

/** Consume a pending OIDC state exactly once. */
export function consumeAuthState(state: string): boolean {
  const normalized = state.trim();
  if (!normalized || !pendingStates.has(normalized)) return false;
  pendingStates.delete(normalized);
  return true;
}

/**
 * Validate an OIDC browser-session return before handing it to the app.
 * The returned URL is safe to inspect for code/state; bearer tokens are
 * deliberately rejected from both query and fragment locations.
 */
export function validateAuthReturn(expectedState: string, callback: string, expectedRedirect: string): URL {
  let actual: URL;
  let expected: URL;
  try {
    actual = new URL(callback);
    expected = new URL(expectedRedirect);
  } catch {
    throw new Error('AUTH_RETURN');
  }
  const state = expectedState.trim();
  const tokenKeys = ['access_token', 'id_token', 'token', 'refresh_token'];
  const hasToken = tokenKeys.some((key) => actual.searchParams.has(key) || new URLSearchParams(actual.hash.replace(/^#/, '')).has(key));
  if (!state || actual.protocol !== expected.protocol || actual.host !== expected.host || actual.pathname !== expected.pathname ||
      actual.username || actual.password || actual.searchParams.get('state') !== state || hasToken) {
    throw new Error('AUTH_RETURN');
  }
  return actual;
}
