export interface WebPlatformAdapters {
  navigate(path: string): void;
  readPreference(key: string): string | null;
  writePreference(key: string, value: string): void;
  copyText(value: string): Promise<void>;
}

export function createWebPlatformAdapters(): WebPlatformAdapters {
  return {
    navigate: (path) => window.history.pushState({}, '', path),
    readPreference: (key) => window.localStorage.getItem(key),
    writePreference: (key, value) => window.localStorage.setItem(key, value),
    copyText: async (value) => { await navigator.clipboard.writeText(value); },
  };
}
