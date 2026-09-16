export interface WailsAppBridge {
  GetAPIBaseURL?: () => string | Promise<string>;
  GetAPILanBaseURL?: () => string;
  GetDesktopListenPublicActive?: () => boolean;
  CheckForUpdates?: () => void;
  AutoCheckForUpdates?: () => void;
  GetPaseoURL?: () => string;
  GetPaseoAllowedOrigins?: () => string[];
  GetCredential?: (key: string) => string | null;
  DeleteCredential?: (key: string) => void;
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

export interface DesktopApiBaseUrlRetryOptions {
  attempts?: number;
  delayMs?: number;
}

/**
 * Vue's desktop integration waits for Wails' generated binding because the
 * WebView can expose `window.go` after the document starts evaluating. The
 * React renderer must resolve the same boundary before importing its client.
 */
export async function resolveDesktopApiBaseUrlWhenReady(
  readBridge: () => WailsAppBridge,
  readInjected: () => unknown,
  options: DesktopApiBaseUrlRetryOptions = {},
): Promise<string> {
  const attempts = Math.max(1, options.attempts ?? 40);
  const delayMs = Math.max(0, options.delayMs ?? 50);

  for (let attempt = 0; attempt < attempts; attempt += 1) {
    const injected = resolveDesktopApiBaseUrl(readInjected());
    if (injected) return injected;

    const bridged = await resolveDesktopApiBaseUrlFromBridge(readBridge());
    if (bridged) return bridged;

    if (attempt + 1 < attempts && delayMs > 0) {
      await new Promise<void>((resolve) => setTimeout(resolve, delayMs));
    }
  }

  return '';
}

export function isWailsWebView(value: unknown = typeof window === 'undefined' ? undefined : (window as Window & { runtime?: unknown }).runtime): boolean {
  return Boolean(value && typeof value === 'object');
}
