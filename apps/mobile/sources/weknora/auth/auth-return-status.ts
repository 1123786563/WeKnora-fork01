import { consumeAuthState, validateAuthReturn } from '@weknora/domain/mobile';

/**
 * Deep-link redirect the Go server returns to after the OIDC browser session.
 * Must stay in sync with the `scheme` in app.config.js and the auth-return route.
 */
export const AUTH_RETURN_REDIRECT = 'weknora://auth-return';

export type AuthReturnParams = Record<string, string | string[] | undefined>;

export type AuthReturnOutcome =
  | { status: 'verified'; state: string }
  | { status: 'missing_state' }
  | { status: 'unknown_state' }
  | { status: 'rejected' };

function firstParam(value: string | string[] | undefined): string | null {
  if (Array.isArray(value)) return typeof value[0] === 'string' ? value[0] : null;
  return typeof value === 'string' ? value : null;
}

/** Rebuild the callback URL exactly as the browser handed it to the app. */
export function buildCallbackUrl(params: AuthReturnParams, redirect: string): string | null {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    const first = firstParam(value);
    if (first !== null && first !== '') query.set(key, first);
  }
  const suffix = query.toString();
  return suffix ? `${redirect}?${suffix}` : redirect;
}

/**
 * Decide whether an incoming auth-return deep link may influence identity.
 * The state must have been issued by this app and not consumed before, the
 * link must target this app's redirect exactly, and no bearer token may ride
 * in the URL. Anything else is ignored without touching stored credentials.
 */
export function evaluateAuthReturn(params: AuthReturnParams, redirect: string): AuthReturnOutcome {
  const state = firstParam(params.state)?.trim() ?? '';
  if (!state) return { status: 'missing_state' };
  if (!consumeAuthState(state)) return { status: 'unknown_state' };
  const callback = buildCallbackUrl(params, redirect);
  if (!callback) return { status: 'rejected' };
  try {
    validateAuthReturn(state, callback, redirect);
  } catch {
    return { status: 'rejected' };
  }
  return { status: 'verified', state };
}
