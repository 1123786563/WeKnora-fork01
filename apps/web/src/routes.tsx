export type RouteMatch =
  | { kind: 'login'; path: '/login' | '/register' }
  | { kind: 'onboarding'; path: '/onboarding/workspace' }
  | { kind: 'apps-catalog'; path: '/platform/apps' }
  | { kind: 'apps-connections'; path: '/platform/apps/connections' }
  | { kind: 'apps-authorization'; path: string; id: string }
  | { kind: 'apps-action'; path: string; id: string }
  | { kind: 'platform'; path: string }
  | { kind: 'craft'; path: string }
  | { kind: 'integration'; path: string }
  | { kind: 'not-found'; path: string };

function pathnameOf(pathname: string): string {
  return pathname.split('?')[0] || '/';
}

function routeId(value: string): string {
  try { return decodeURIComponent(value); } catch { return ''; }
}

export function resolveRoute(pathname: string): RouteMatch {
  const path = pathnameOf(pathname);
  if (path === '/login' || path === '/register') return { kind: 'login', path: path as '/login' | '/register' };
  if (path === '/onboarding/workspace') return { kind: 'onboarding', path };
  if (path === '/platform/apps') return { kind: 'apps-catalog', path };
  if (path === '/platform/apps/connections') return { kind: 'apps-connections', path };
  const authorizationPrefix = '/platform/apps/authorization/';
  if (path.startsWith(authorizationPrefix) && path.length > authorizationPrefix.length && routeId(path.slice(authorizationPrefix.length))) return { kind: 'apps-authorization', path, id: routeId(path.slice(authorizationPrefix.length)) };
  const actionPrefix = '/platform/apps/actions/';
  if (path.startsWith(actionPrefix) && path.length > actionPrefix.length && routeId(path.slice(actionPrefix.length))) return { kind: 'apps-action', path, id: routeId(path.slice(actionPrefix.length)) };
  if (path === '/craft' || path.startsWith('/craft/')) return { kind: 'craft', path };
  if (path === '/platform/integrations' || path === '/platform/settings') return { kind: 'integration', path };
  if (path === '/' || path === '/platform' || path === '/platform/knowledge-bases') return { kind: 'platform', path };
  return { kind: 'not-found', path };
}

export interface RouteGuardContext {
  authenticated: boolean;
  tenantId: string | null;
}

export type RouteGuardDecision =
  | { kind: 'allow' }
  | { kind: 'not-found' }
  | { kind: 'redirect'; to: string; reason: 'authentication-required' | 'workspace-required' };

function isProtectedRoute(route: RouteMatch): boolean {
  return route.kind === 'platform' || route.kind.startsWith('apps-');
}

/** Vue-compatible auth gate. The complete original URL is retained in `next`. */
export function guardRoute(pathname: string, context: RouteGuardContext): RouteGuardDecision {
  const route = resolveRoute(pathname);
  if (route.kind === 'login' || route.kind === 'craft') return { kind: 'allow' };
  if (route.kind === 'not-found') return pathnameOf(pathname).startsWith('/platform/') && !context.authenticated
    ? { kind: 'redirect', to: `/login?next=${encodeURIComponent(pathname)}`, reason: 'authentication-required' }
    : { kind: 'not-found' };
  if (route.kind === 'onboarding') {
    if (!context.authenticated) return { kind: 'redirect', to: '/login', reason: 'authentication-required' };
    return context.tenantId ? { kind: 'redirect', to: '/platform/knowledge-bases', reason: 'workspace-required' } : { kind: 'allow' };
  }
  if (route.kind === 'integration' || isProtectedRoute(route)) {
    if (!context.authenticated) return { kind: 'redirect', to: `/login?next=${encodeURIComponent(pathname)}`, reason: 'authentication-required' };
    if (!context.tenantId) return { kind: 'redirect', to: '/onboarding/workspace', reason: 'workspace-required' };
  }
  return { kind: 'allow' };
}

export function nextPathAfterAuth(search: string): string {
  const next = new URLSearchParams(search).get('next');
  return next && next.startsWith('/') && !next.startsWith('//') ? next : '/platform/knowledge-bases';
}

export function authNavigationTarget(search: string, invited: boolean): string {
  if (invited) return '/platform/knowledge-bases';
  return nextPathAfterAuth(search);
}
