export interface DesktopFileBridge {
  openPath?: (path: string) => Promise<void> | void;
  savePath?: (path: string) => Promise<void> | void;
}

export function hasDesktopFileBridge(value: unknown): value is DesktopFileBridge {
  if (!value || typeof value !== 'object') return false;
  const bridge = value as DesktopFileBridge;
  return typeof bridge.openPath === 'function' || typeof bridge.savePath === 'function';
}
