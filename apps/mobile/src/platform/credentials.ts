import * as SecureStore from 'expo-secure-store';
import type { Credential, CredentialAdapter } from '@weknora/api-client';

const ACCESS_KEY = 'weknora.mobile.access-token';
const REFRESH_KEY = 'weknora.mobile.refresh-token';

export function createSecureCredentialAdapter(): CredentialAdapter {
  return {
    async read(): Promise<Credential> {
      const [accessToken, refreshToken] = await Promise.all([SecureStore.getItemAsync(ACCESS_KEY), SecureStore.getItemAsync(REFRESH_KEY)]);
      return accessToken ? { kind: 'bearer', accessToken, ...(refreshToken ? { refreshToken } : {}) } : { kind: 'anonymous' };
    },
    async write(value) {
      if (value.kind !== 'bearer') { await this.clear(); return; }
      await SecureStore.setItemAsync(ACCESS_KEY, value.accessToken);
      if (value.refreshToken) await SecureStore.setItemAsync(REFRESH_KEY, value.refreshToken);
      else await SecureStore.deleteItemAsync(REFRESH_KEY);
    },
    async clear() { await Promise.all([SecureStore.deleteItemAsync(ACCESS_KEY), SecureStore.deleteItemAsync(REFRESH_KEY)]); },
  };
}
