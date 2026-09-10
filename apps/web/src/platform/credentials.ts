import type { BearerCredential, Credential, CredentialAdapter } from '@weknora/api-client';

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
    return;
  }
  if (credential.kind === 'embed') {
    storage.setItem(ACCESS_TOKEN_KEY, credential.token);
    storage.removeItem(REFRESH_TOKEN_KEY);
    return;
  }
  storage.removeItem(ACCESS_TOKEN_KEY);
  storage.removeItem(REFRESH_TOKEN_KEY);
}

export function createBrowserCredentialAdapter(storage: CredentialStorage): CredentialAdapter {
  return {
    async read(): Promise<Credential> {
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
