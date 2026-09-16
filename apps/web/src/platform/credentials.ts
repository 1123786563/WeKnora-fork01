import type { BearerCredential, Credential, CredentialAdapter } from '@weknora/api-client';
import { clearWeknoraUser } from '@weknora/domain/settings/local-preferences';
import { persistReactPlatformState, readReactPlatformState } from './legacy-session.ts';

const ACCESS_TOKEN_KEY = 'weknora_token';
const REFRESH_TOKEN_KEY = 'weknora_refresh_token';

type CredentialStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

function nonEmpty(value: string | null): string | undefined {
  const trimmed = value?.trim();
  return trimmed || undefined;
}

export function persistBrowserCredential(storage: CredentialStorage, credential: Credential): void {
  if (credential.kind === 'bearer') {
    storage.setItem(ACCESS_TOKEN_KEY, credential.accessToken);
    if (credential.refreshToken) storage.setItem(REFRESH_TOKEN_KEY, credential.refreshToken);
    else storage.removeItem(REFRESH_TOKEN_KEY);
  } else if (credential.kind === 'embed') {
    storage.setItem(ACCESS_TOKEN_KEY, credential.token);
    storage.removeItem(REFRESH_TOKEN_KEY);
  } else {
    storage.removeItem(ACCESS_TOKEN_KEY);
    storage.removeItem(REFRESH_TOKEN_KEY);
    // The anonymous write is the logout / session-invalidated path. Vue
    // stores/auth.ts logout removes weknora_user so per-user preference
    // namespaces (WeKnora_${userId}_*) resolve to "anon" afterwards and the
    // next account never inherits the previous one's preferences.
    clearWeknoraUser(storage);
  }

  const current = readReactPlatformState(storage) ?? {
    credential: { kind: 'anonymous' } as const,
    tenantId: null,
    preferences: {},
  };
  persistReactPlatformState(storage, { ...current, credential });
}

export function createBrowserCredentialAdapter(storage: CredentialStorage): CredentialAdapter {
  return {
    async read(): Promise<Credential> {
      const migrated = readReactPlatformState(storage);
      if (migrated) return migrated.credential;
      const accessToken = nonEmpty(storage.getItem(ACCESS_TOKEN_KEY));
      if (!accessToken) return { kind: 'anonymous' };
      if (/^embed\s/i.test(accessToken)) return { kind: 'embed', token: accessToken };
      const refreshToken = nonEmpty(storage.getItem(REFRESH_TOKEN_KEY));
      const credential: BearerCredential = { kind: 'bearer', accessToken, refreshToken };
      return credential;
    },
    async write(value: Credential): Promise<void> {
      persistBrowserCredential(storage, value);
    },
    async clear(): Promise<void> {
      persistBrowserCredential(storage, { kind: 'anonymous' });
    },
  };
}
