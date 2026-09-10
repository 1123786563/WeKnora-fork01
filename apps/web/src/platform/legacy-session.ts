import type { Credential } from '@weknora/api-client';

export interface LegacyPlatformSession {
  credential: Credential;
  tenantId: string | null;
}
export interface LegacyPlatformAdapter {
  read(): LegacyPlatformSession;
}

/** Migration-only seam for the Vue-era browser keys. The shared client never reads storage. */
export function createLegacyPlatformAdapter(storage: Pick<Storage, 'getItem'>): LegacyPlatformAdapter {
  return {
    read(): LegacyPlatformSession {
      const token = storage.getItem('weknora_token')?.trim() ?? '';
      const tenantId = storage.getItem('weknora_selected_tenant_id')?.trim() || null;
      if (!token) return { credential: { kind: 'anonymous' }, tenantId };

      if (/^embed\s/i.test(token)) {
        return { credential: { kind: 'embed', token }, tenantId };
      }
      return { credential: { kind: 'bearer', accessToken: token }, tenantId };
    },
  };
}

export function readLegacyPlatformSession(): LegacyPlatformSession {
  return createLegacyPlatformAdapter(window.localStorage).read();
}

export function authorizationHeader(credential: Credential): string | undefined {
  if (credential.kind === 'bearer') return `Bearer ${credential.accessToken}`;
  if (credential.kind === 'embed') return credential.token;
  return undefined;
}
