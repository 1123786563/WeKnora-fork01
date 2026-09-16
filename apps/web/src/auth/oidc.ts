import type { AuthSession } from '@weknora/api-client';

export type OIDCCallbackResult =
  | { kind: 'success'; session: AuthSession }
  | { kind: 'error'; code: string; message: string };

function record(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('payload must be an object');
  return value as Record<string, unknown>;
}

function requiredString(value: unknown): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error('value must be a non-empty string');
  return value;
}

function decodeBase64URL(value: string): string {
  const padded = value.replaceAll('-', '+').replaceAll('_', '/') + '='.repeat((4 - (value.length % 4)) % 4);
  const binary = atob(padded);
  const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function parseSuccess(raw: string): AuthSession {
  const payload = record(JSON.parse(decodeBase64URL(raw)));
  if (payload.success !== true) throw new Error('callback was not successful');
  return {
    token: requiredString(payload.token),
    refreshToken: requiredString(payload.refresh_token ?? payload.refreshToken),
    user: payload.user === undefined ? undefined : record(payload.user),
    tenant: payload.tenant === null ? null : payload.tenant === undefined ? undefined : record(payload.tenant),
    memberships: Array.isArray(payload.memberships) ? payload.memberships : undefined,
  };
}

export function parseOIDCCallbackHash(hash: string): OIDCCallbackResult | null {
  const params = new URLSearchParams(hash.startsWith('#') ? hash.slice(1) : hash);
  const providerError = params.get('oidc_error');
  if (providerError) {
    return {
      kind: 'error',
      code: providerError,
      message: params.get('oidc_error_description') || providerError,
    };
  }
  const encoded = params.get('oidc_result');
  if (!encoded) return null;
  try {
    return { kind: 'success', session: parseSuccess(encoded) };
  } catch {
    return { kind: 'error', code: 'invalid_callback', message: 'The OIDC callback payload is invalid.' };
  }
}
