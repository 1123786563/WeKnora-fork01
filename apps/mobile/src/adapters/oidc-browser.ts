import type { OidcBrowserPort } from '@weknora/mobile-core';

export interface BrowserPort {
  openAuthSessionAsync(url: string, redirectUri: string): Promise<{ type: string; url?: string }>;
}

function exactCallback(url: string, registeredRedirect: string): boolean {
  try {
    const actual = new URL(url);
    const expected = new URL(registeredRedirect);
    return actual.protocol === expected.protocol && actual.host === expected.host && actual.pathname === expected.pathname;
  } catch {
    return false;
  }
}

/** Opens an authorization session and accepts a redirect only for the registered app route. */
export function createOidcBrowser(registeredRedirect: string, browser: BrowserPort): OidcBrowserPort {
  if (!exactCallback(registeredRedirect, registeredRedirect)) throw new Error('OIDC_CALLBACK');
  return {
    async open(authorizationUrl: string): Promise<string> {
      const result = await browser.openAuthSessionAsync(authorizationUrl, registeredRedirect);
      if (result.type !== 'success' || typeof result.url !== 'string' || !exactCallback(result.url, registeredRedirect)) throw new Error('OIDC_CALLBACK');
      return result.url;
    },
  };
}

/** Loads Expo WebBrowser only in the native composition path. */
export function createNativeOidcBrowser(registeredRedirect: string): OidcBrowserPort {
  return createOidcBrowser(registeredRedirect, require('expo-web-browser') as BrowserPort);
}
