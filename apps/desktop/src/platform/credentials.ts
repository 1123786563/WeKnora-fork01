export interface DesktopCredentialStorage {
  read(key: string): string | null;
  write(key: string, value: string): void;
  remove(key: string): void;
}

export function createDesktopCredentialStorage(storage: Storage | undefined = typeof localStorage === 'undefined' ? undefined : localStorage): DesktopCredentialStorage {
  return {
    read: (key) => storage?.getItem(key) ?? null,
    write: (key, value) => { storage?.setItem(key, value); },
    remove: (key) => { storage?.removeItem(key); },
  };
}
