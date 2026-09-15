export interface WailsAppBridge {
  GetAPIBaseURL?: () => string | Promise<string>;
  GetAPILanBaseURL?: () => string;
  GetDesktopListenPublicActive?: () => boolean;
  CheckForUpdates?: () => void;
  AutoCheckForUpdates?: () => void;
}

function globalBridge(value: unknown): WailsAppBridge {
  if (!value || typeof value !== 'object') return {};
  return value as WailsAppBridge;
}

export function readWailsBridge(value: unknown = typeof window === 'undefined' ? undefined : (window as Window & { go?: { main?: { App?: WailsAppBridge } } }).go?.main?.App): WailsAppBridge {
  return globalBridge(value);
}

export function resolveDesktopApiBaseUrl(value: unknown = typeof window === 'undefined' ? undefined : (window as Window & { __WEKNORA_API_BASE__?: unknown }).__WEKNORA_API_BASE__): string {
  const raw = typeof value === 'string' ? value.trim() : '';
  if (!raw) return '';
  try {
    const url = new URL(raw);
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return '';
    return url.toString().replace(/\/$/, '');
  } catch { return ''; }
}

/** Resolve the Go method directly so the shared renderer never boots against the WebView origin. */
export async function resolveDesktopApiBaseUrlFromBridge(app: WailsAppBridge): Promise<string> {
  if (typeof app.GetAPIBaseURL !== 'function') return '';
  try {
    return resolveDesktopApiBaseUrl(await app.GetAPIBaseURL());
  } catch {
    return '';
  }
}

export function isWailsWebView(value: unknown = typeof window === 'undefined' ? undefined : (window as Window & { runtime?: unknown }).runtime): boolean {
  return Boolean(value && typeof value === 'object');
}
