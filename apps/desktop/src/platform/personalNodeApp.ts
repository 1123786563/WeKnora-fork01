import type { DesktopPersonalNodeRuntimeInput } from './runtime.ts';
import type { DesktopCredentialBridge } from './credentials.ts';
import { createDesktopRegistrationClient } from './registrationClient.ts';

export interface DesktopPersonalNodeConfig { apiBaseURL: string; paseoURL: string; allowedOrigins: string[]; credentialKey?: string; accessCredentialKey?: string; }

/** Reads only deployment/Wails injected configuration; missing config disables the connector. */
export function createDesktopAppPersonalNode(app: DesktopCredentialBridge & { GetAPIBaseURL?: () => string | Promise<string>; GetPaseoURL?: () => string; GetPaseoAllowedOrigins?: () => string[] } = {}, supplied?: Partial<DesktopPersonalNodeConfig>): DesktopPersonalNodeRuntimeInput | undefined {
  const browserWindow = typeof window === 'undefined' ? undefined : window as Window & { __WEKNORA_PASEO_CONFIG__?: Partial<DesktopPersonalNodeConfig> };
  const injected = browserWindow?.__WEKNORA_PASEO_CONFIG__;
  const paseoURL = supplied?.paseoURL?.trim() || injected?.paseoURL?.trim() || app.GetPaseoURL?.().trim();
  const allowedOrigins = supplied?.allowedOrigins?.filter(Boolean) ?? injected?.allowedOrigins?.filter(Boolean) ?? app.GetPaseoAllowedOrigins?.() ?? [];
  const apiBaseURL = supplied?.apiBaseURL?.trim() || injected?.apiBaseURL?.trim();
  if (!apiBaseURL || !paseoURL || allowedOrigins.length === 0) return undefined;
  const accessKey = injected?.accessCredentialKey ?? 'weknora.desktop.access-token';
  const nodeKey = injected?.credentialKey ?? 'weknora.desktop.personal-node-credential';
  return {
    credentialBridge: app,
    credentialKey: nodeKey,
    registrationClient: createDesktopRegistrationClient({ baseURL: apiBaseURL, accessToken: () => app.readCredential?.(accessKey) ?? null }),
    transport: { baseURL: paseoURL, allowedOrigins },
  };
}
