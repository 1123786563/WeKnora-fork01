import type { AuthSession } from '@weknora/api-client';

export type MobileOIDCCallback =
  | { kind: 'success'; session: AuthSession; state?: string }
  | { kind: 'error'; code: string; message: string; state?: string };

function record(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('payload must be an object');
  return value as Record<string, unknown>;
}

function requiredString(value: unknown, label: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${label} is required`);
  return value;
}

function decodeBase64URL(value: string): string {
  const padded = value.replaceAll('-', '+').replaceAll('_', '/') + '='.repeat((4 - (value.length % 4)) % 4);
  const binary = atob(padded);
  const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function paramsFromURL(raw: string): URLSearchParams {
  try { return new URL(raw).hash ? new URLSearchParams(new URL(raw).hash.slice(1)) : new URL(raw).searchParams; }
  catch { return new URLSearchParams(raw.startsWith('#') ? raw.slice(1) : raw); }
}

export function parseMobileOIDCCallback(raw: string): MobileOIDCCallback | null {
  const params = paramsFromURL(raw);
  const state = params.get('state') || undefined;
  const providerError = params.get('oidc_error');
  if (providerError) return { kind: 'error', code: providerError, message: params.get('oidc_error_description') || providerError, state };
  const encoded = params.get('oidc_result');
  if (!encoded) return null;
  try {
    const payload = record(JSON.parse(decodeBase64URL(encoded)));
    if (payload.success !== true) throw new Error('callback was not successful');
    return {
      kind: 'success',
      session: {
        token: requiredString(payload.token, 'access token'),
        refreshToken: requiredString(payload.refresh_token ?? payload.refreshToken, 'refresh token'),
        user: payload.user === undefined ? undefined : record(payload.user),
        tenant: payload.tenant === null ? null : payload.tenant === undefined ? undefined : record(payload.tenant),
        memberships: Array.isArray(payload.memberships) ? payload.memberships : undefined,
      },
      state,
    };
  } catch { return { kind: 'error', code: 'invalid_callback', message: 'The OIDC callback payload is invalid.', state }; }
}
