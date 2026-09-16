import { createDesktopAppPersonalNode, type DesktopPersonalNodeConfig } from './personalNodeApp.ts';
import type { DesktopCredentialBridge } from './credentials.ts';
import { resolveDesktopApiBaseUrlWhenReady, type WailsAppBridge } from './wails.ts';

type DesktopAppBridge = WailsAppBridge & DesktopCredentialBridge & {
  GetPaseoURL?: () => string | Promise<string>;
  GetPaseoAllowedOrigins?: () => string[] | Promise<string[]>;
};

/**
 * Resolve every Wails-owned value before constructing the connector. Wails
 * exposes generated methods asynchronously, so composing first and installing
 * the runtime second loses the personal-node adapter on a normal launch.
 */
export async function resolveDesktopPersonalNode(
  app: DesktopAppBridge,
  readInjected: () => unknown = () => typeof window === 'undefined' ? undefined : (window as Window & { __WEKNORA_PASEO_CONFIG__?: Partial<DesktopPersonalNodeConfig> }).__WEKNORA_PASEO_CONFIG__,
  options: { attempts?: number; delayMs?: number } = {},
) {
  const apiBaseURL = await resolveDesktopApiBaseUrlWhenReady(
    () => app,
    () => typeof window === 'undefined' ? undefined : (window as Window & { __WEKNORA_API_BASE__?: unknown }).__WEKNORA_API_BASE__,
    options,
  );
  if (!apiBaseURL) return undefined;

  const injected = readInjected() as Partial<DesktopPersonalNodeConfig> | undefined;
  const paseoURL = injected?.paseoURL?.trim() || await Promise.resolve(app.GetPaseoURL?.() ?? '');
  const allowedOrigins = injected?.allowedOrigins?.filter(Boolean) ?? await Promise.resolve(app.GetPaseoAllowedOrigins?.() ?? []);
  if (!paseoURL || allowedOrigins.length === 0) return undefined;

  // Wails generated credential methods return Promises. Read once before the
  // synchronous connector composition and retain only an opaque in-memory
  // value; revoke still calls DeleteCredential through the bridge.
  const accessKey = injected?.accessCredentialKey ?? 'weknora.desktop.access-token';
  const nodeKey = injected?.credentialKey ?? 'weknora.desktop.personal-node-credential';
  const credentialValues = new Map<string, string>();
  if (typeof app.GetCredential === 'function') {
    for (const key of [accessKey, nodeKey]) {
      try {
        const value = (await Promise.resolve(app.GetCredential(key)))?.trim();
        if (value) credentialValues.set(key, value);
      } catch {
        // Missing native credentials fail closed below.
      }
    }
  }
  if (!credentialValues.has(accessKey) || !credentialValues.has(nodeKey)) return undefined;

  const credentialBridge: DesktopCredentialBridge = {
    readCredential: (key) => credentialValues.get(key) ?? null,
    removeCredential: (key) => {
      credentialValues.delete(key);
      void Promise.resolve(app.DeleteCredential?.(key));
    },
  };
  return createDesktopAppPersonalNode(credentialBridge, {
    apiBaseURL,
    paseoURL,
    allowedOrigins,
    credentialKey: nodeKey,
    accessCredentialKey: accessKey,
  });
}
