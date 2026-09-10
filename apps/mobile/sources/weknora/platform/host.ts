import * as React from 'react';

export interface MobileHost {
  backend: 'weknora';
  origin: string;
}

export function createMobileHost(origin: string): MobileHost {
  if (!origin.trim()) throw new Error('SERVER_REQUIRED');
  let url: URL;
  try { url = new URL(origin); } catch { throw new Error('INVALID_SERVER'); }
  if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new Error('INVALID_SERVER');
  }
  return { backend: 'weknora', origin: url.origin };
}

type MobileHostContextValue = {
  host: MobileHost | null;
  setMobileHost: (host: MobileHost) => void;
};
const MobileHostContext = React.createContext<MobileHostContextValue | null>(null);

export function MobileHostProvider({
  initialHost,
  children,
}: React.PropsWithChildren<{ initialHost: MobileHost | null }>) {
  const [host, setMobileHost] = React.useState<MobileHost | null>(initialHost);
  const value = React.useMemo(() => ({ host, setMobileHost }), [host]);
  return React.createElement(MobileHostContext.Provider, { value }, children);
}

export function useMobileHost(): MobileHost | null {
  return React.useContext(MobileHostContext)?.host ?? null;
}

export function useSetMobileHost(): (host: MobileHost) => void {
  const context = React.useContext(MobileHostContext);
  if (!context) throw new Error('MOBILE_HOST_PROVIDER_REQUIRED');
  return context.setMobileHost;
}
