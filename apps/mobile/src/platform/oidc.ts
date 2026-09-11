export const MOBILE_OIDC_REDIRECT = 'weknora://oidc';

export type MobileOIDCCallback =
  | { kind: 'code'; code: string; state: string }
  | { kind: 'error'; code: string; message: string; state?: string };

function callbackURL(raw: string): URL | null {
  try {
    const actual = new URL(raw);
    const expected = new URL(MOBILE_OIDC_REDIRECT);
    if (actual.protocol !== expected.protocol || actual.host !== expected.host || actual.pathname !== expected.pathname || actual.username || actual.password) return null;
    return actual;
  } catch {
    return null;
  }
}

export function parseMobileOIDCCallback(raw: string): MobileOIDCCallback | null {
  const url = callbackURL(raw);
  if (!url) return null;
  const params = url.searchParams;
  const state = params.get('state') || undefined;
  const hashParams = new URLSearchParams(url.hash.slice(1));
  const bearerKeys = ['access_token', 'id_token', 'token', 'refresh_token', 'oidc_result'];
  if (bearerKeys.some((key) => params.has(key) || hashParams.has(key))) {
    return { kind: 'error', code: 'invalid_callback', message: 'The OIDC callback payload is invalid.', state };
  }
  const providerError = params.get('oidc_error');
  if (providerError) return { kind: 'error', code: providerError, message: params.get('oidc_error_description') || providerError, state };
  const code = params.get('oidc_code');
  if (!code || !state) return params.has('oidc_code') ? { kind: 'error', code: 'invalid_callback', message: 'The OIDC callback payload is invalid.', state } : null;
  return { kind: 'code', code, state };
}

export function matchesMobileOIDCState(callback: MobileOIDCCallback, expectedState: string | null): boolean {
  return Boolean(expectedState && callback.state && callback.state === expectedState);
}
