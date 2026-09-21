import { useEffect, useRef } from 'react';
import { useLinkingURL } from 'expo-linking';
import { router } from 'expo-router';
import { completeNativeOidcCallback, OIDC_REDIRECT_URI } from '../composition.ts';

type CompleteOidc = (callbackUrl: string) => Promise<unknown>;

function isRegisteredCallback(callbackUrl: string): boolean {
  try {
    const actual = new URL(callbackUrl);
    const registered = new URL(OIDC_REDIRECT_URI);
    return actual.protocol === registered.protocol
      && actual.host === registered.host
      && actual.pathname === registered.pathname;
  } catch {
    return false;
  }
}

/** Forwards the untouched native URL to Runtime, then reveals its resulting safe surface. */
export async function deliverOidcReturn(
  callbackUrl: string | null,
  complete: CompleteOidc = completeNativeOidcCallback,
  finish: () => void = () => { router.replace('/'); },
): Promise<boolean> {
  if (!callbackUrl || !isRegisteredCallback(callbackUrl)) return false;
  try {
    await complete(callbackUrl);
    return true;
  } finally {
    finish();
  }
}

/** Native OIDC return route for warm linking and process-restart delivery. */
export default function AuthReturn() {
  const callbackUrl = useLinkingURL();
  const delivered = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!callbackUrl || delivered.current === callbackUrl) return;
    delivered.current = callbackUrl;
    void deliverOidcReturn(callbackUrl).catch(() => {});
  }, [callbackUrl]);
  return null;
}
