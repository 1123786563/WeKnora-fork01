import { resolveMobileApiBaseUrl } from './transport.ts';

const SERVER_URL_KEY = 'weknora.mobile.server-url';

export interface ServerAddressStore {
  getItemAsync(key: string): Promise<string | null>;
  setItemAsync(key: string, value: string): Promise<void>;
  deleteItemAsync(key: string): Promise<void>;
}

export interface ServerAddressAdapter {
  read(): Promise<string>;
  write(value: string): Promise<string>;
}

export function createServerAddressAdapter(store: ServerAddressStore): ServerAddressAdapter {
  return {
    async read() {
      const saved = await store.getItemAsync(SERVER_URL_KEY);
      return resolveMobileApiBaseUrl(saved ?? '');
    },
    async write(value) {
      const raw = value.trim();
      if (!raw) {
        await store.deleteItemAsync(SERVER_URL_KEY);
        return '';
      }
      const normalized = resolveMobileApiBaseUrl(raw);
      if (!normalized) throw new Error('Server address must use HTTP(S)');
      await store.setItemAsync(SERVER_URL_KEY, normalized);
      return normalized;
    },
  };
}
