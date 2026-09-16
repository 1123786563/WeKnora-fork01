import { isExternalHttpUrl, isSafeDesktopDeepLink, normalizeDesktopLocation } from './navigation.ts';
import { hasDesktopFileBridge, type DesktopFileBridge } from './files.ts';
import { readWailsBridge, resolveDesktopApiBaseUrlWhenReady, type WailsAppBridge } from './wails.ts';

export const DESKTOP_WINDOW_DEFAULTS = {
  width: 1440,
  height: 900,
  minWidth: 1024,
  minHeight: 680,
} as const;

export interface DesktopWindowRuntime {
  WindowSetSize?: (width: number, height: number) => void;
  WindowSetMinSize?: (width: number, height: number) => void;
  BrowserOpenURL?: (url: string) => void;
}

export interface DesktopRuntimeAdapters {
  app: WailsAppBridge;
  runtime: DesktopWindowRuntime;
  fileBridge: DesktopFileBridge;
  normalizeLocation: () => string;
  openExternal: (url: string) => void;
}

type DesktopOpenTarget = Pick<Window, 'open'>;

/** Install before importing the shared renderer; Go's DomReady injection is intentionally later. */
export function installDesktopExternalUrlBridge(target: DesktopOpenTarget, runtime: DesktopWindowRuntime): () => void {
  const originalOpen = target.open;
  target.open = ((url?: string | URL, ...args: unknown[]) => {
    const value = typeof url === 'string' ? url : url?.toString() ?? '';
    if (isExternalHttpUrl(value) && runtime.BrowserOpenURL) {
      runtime.BrowserOpenURL(value);
      return null;
    }
    return originalOpen.call(target, value, args[0] as string | undefined, args[1] as string | undefined);
  }) as Window['open'];
  return () => { target.open = originalOpen; };
}

export function applyDesktopWindowDefaults(runtime: DesktopWindowRuntime): void {
  runtime.WindowSetMinSize?.(DESKTOP_WINDOW_DEFAULTS.minWidth, DESKTOP_WINDOW_DEFAULTS.minHeight);
  runtime.WindowSetSize?.(DESKTOP_WINDOW_DEFAULTS.width, DESKTOP_WINDOW_DEFAULTS.height);
}

export function createDesktopRuntimeAdapters(
  app: WailsAppBridge = {},
  runtime: DesktopWindowRuntime = {},
  fileBridge: DesktopFileBridge = {},
  location: Pick<Location, 'pathname' | 'search' | 'hash'> = { pathname: '/', search: '', hash: '' },
): DesktopRuntimeAdapters {
  return {
    app,
    runtime,
    fileBridge,
    normalizeLocation: () => normalizeDesktopLocation(location.pathname, location.search, location.hash),
    openExternal: (url) => {
      if (!isExternalHttpUrl(url)) return;
      if (runtime.BrowserOpenURL) runtime.BrowserOpenURL(url);
      else if (typeof window !== 'undefined') window.open(url, '_blank', 'noopener,noreferrer');
    },
  };
}

export async function installDesktopRuntime(): Promise<DesktopRuntimeAdapters> {
  const browserWindow = typeof window === 'undefined' ? undefined : window as Window & {
    runtime?: DesktopWindowRuntime;
    go?: { main?: { App?: WailsAppBridge } };
    __WEKNORA_API_BASE__?: string;
    __WEKNORA_DESKTOP__?: DesktopRuntimeAdapters;
  };
  const initialApp = readWailsBridge(browserWindow?.go?.main?.App);
  const runtime = browserWindow?.runtime ?? {};
  applyDesktopWindowDefaults(runtime);
  if (browserWindow) {
    const apiBaseUrl = await resolveDesktopApiBaseUrlWhenReady(
      () => readWailsBridge(browserWindow.go?.main?.App ?? initialApp),
      () => browserWindow.__WEKNORA_API_BASE__,
    );
    if (apiBaseUrl) browserWindow.__WEKNORA_API_BASE__ = apiBaseUrl;
    installDesktopExternalUrlBridge(browserWindow, runtime);
  }
  const app = readWailsBridge(browserWindow?.go?.main?.App ?? initialApp);
  const fileBridge = hasDesktopFileBridge(app) ? app : {};
  const adapters = createDesktopRuntimeAdapters(app, runtime, fileBridge, browserWindow?.location);
  if (browserWindow) {
    browserWindow.__WEKNORA_DESKTOP__ = adapters;
    const current = adapters.normalizeLocation();
    if (isSafeDesktopDeepLink(current) && current !== `${browserWindow.location.pathname}${browserWindow.location.search}${browserWindow.location.hash}`) {
      browserWindow.history.replaceState({}, browserWindow.document.title, current);
    }
  }
  return adapters;
}
