export interface DesktopCredentialStorage {
  read(key: string): string | null;
  write(key: string, value: string): void;
  remove(key: string): void;
}

export interface DesktopCredentialBridge {
  readCredential?: (key: string) => string | null;
  writeCredential?: (key: string, value: string) => void;
  removeCredential?: (key: string) => void;
}

export function createDesktopCredentialStorage(
  storage: Storage | undefined = typeof localStorage === 'undefined' ? undefined : localStorage,
  bridge: DesktopCredentialBridge = {},
): DesktopCredentialStorage {
  return {
    read: (key) => bridge.readCredential?.(key) ?? storage?.getItem(key) ?? null,
    write: (key, value) => { bridge.writeCredential?.(key, value); storage?.setItem(key, value); },
    remove: (key) => { bridge.removeCredential?.(key); storage?.removeItem(key); },
  };
}
