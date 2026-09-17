import { persistWeknoraUser } from '@weknora/domain/settings/local-preferences';

// R462 A3 adjudication: createAuthApi (the dead AuthPages-era HTTP adapter)
// was removed -- production code referenced it zero times; live login calls
// go through @weknora/api-client's createAuthApi. What survives here is the
// login-persistence half, still consumed alongside platform/credentials in
// the session chain tests.

export interface ParsedLogin {
  credential: { kind: 'bearer'; accessToken: string; refreshToken?: string };
  tenantId?: string | null;
  user?: Record<string, unknown>;
}

export function persistLogin(session: ParsedLogin, storage: Pick<Storage, 'setItem' | 'removeItem'> = window.localStorage): void {
  storage.setItem('weknora_token', session.credential.accessToken);
  if (session.credential.kind === 'bearer' && session.credential.refreshToken) storage.setItem('weknora_refresh_token', session.credential.refreshToken);
  else storage.removeItem('weknora_refresh_token');
  if (session.tenantId) storage.setItem('weknora_selected_tenant_id', session.tenantId);
  // Vue stores/auth.ts setUser parity: persist the login response's user so
  // per-user preference namespaces (WeKnora_${userId}_*) key off the account.
  // persistWeknoraUser resets the migration latch so the new identity adopts
  // legacy/anon preferences on its next preference read.
  if (session.user) persistWeknoraUser(storage, session.user);
}
