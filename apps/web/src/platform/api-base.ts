// Single source for the API base URL (main.tsx and pages that embed API
// surfaces inside the settings modal). Resolution order matches main.tsx:
// VITE_API_BASE_URL first, then the desktop shell injection.
export function resolveApiBaseUrl(): string {
  const env = (import.meta as ImportMeta & { env?: Record<string, string | undefined> }).env?.VITE_API_BASE_URL;
  if (typeof env === 'string' && env) return env;
  if (typeof window !== 'undefined') {
    const injected = (window as Window & { __WEKNORA_API_BASE__?: unknown }).__WEKNORA_API_BASE__;
    if (typeof injected === 'string' && injected) return injected;
  }
  return '';
}
