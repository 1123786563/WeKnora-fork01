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
  // A desktop bridge is the authoritative credential store. Never mirror
  // bearer material into WebView localStorage or fall back to a stale browser
  // value once the bridge is present.
  const hasBridge = Boolean(
    bridge.readCredential || bridge.writeCredential || bridge.removeCredential,
  );
  return {
    read: (key) => hasBridge ? bridge.readCredential?.(key) ?? null : storage?.getItem(key) ?? null,
    write: (key, value) => {
      if (hasBridge) bridge.writeCredential?.(key, value);
      else storage?.setItem(key, value);
    },
    remove: (key) => {
      if (hasBridge) bridge.removeCredential?.(key);
      else storage?.removeItem(key);
    },
  };
}
