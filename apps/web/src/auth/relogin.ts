// Vue authRefresh.ts parity (S00 negpath batch 3, T-3): a failed token refresh
// must land the user on /login instead of leaving a silently poisoned session.
// Vue clears auth storage, then redirectToLogin() — which skips when already on
// /login or on an /embed page (embeds never refresh; they hold preview tokens).
export function shouldReloginAfterRefreshFailure(pathname: string): boolean {
  return pathname !== '/login' && !pathname.startsWith('/embed/');
}

export interface ReloginOptions {
  clearSession: () => void;
  pathname: string;
  assign: (url: string) => void;
}

export async function reloginAfterRefreshFailure(options: ReloginOptions): Promise<void> {
  options.clearSession();
  if (shouldReloginAfterRefreshFailure(options.pathname)) options.assign('/login');
}
