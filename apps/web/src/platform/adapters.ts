export interface WebPlatformAdapters {
  navigate(path: string): void;
  replace(path: string): void;
  readPreference(key: string): string | null;
  writePreference(key: string, value: string): void;
  removePreference(key: string): void;
  copyText(value: string): Promise<void>;
  openExternal(url: string): void;
  saveFile(content: Blob | string, filename: string): Promise<void>;
}

export function createWebPlatformAdapters(): WebPlatformAdapters {
  return {
    navigate: (path) => window.history.pushState({}, '', path),
    replace: (path) => window.history.replaceState({}, '', path),
    readPreference: (key) => window.localStorage.getItem(key),
    writePreference: (key, value) => window.localStorage.setItem(key, value),
    removePreference: (key) => window.localStorage.removeItem(key),
    copyText: async (value) => { await navigator.clipboard.writeText(value); },
    openExternal: (url) => { window.open(url, '_blank', 'noopener,noreferrer'); },
    saveFile: async (content, filename) => {
      const blob = content instanceof Blob ? content : new Blob([content], { type: 'application/octet-stream' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = filename;
      anchor.click();
      URL.revokeObjectURL(url);
    },
  };
}
