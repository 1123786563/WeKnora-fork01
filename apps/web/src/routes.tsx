export type RouteMatch =
  | { kind: 'login'; path: '/login' }
  | { kind: 'platform'; path: string }
  | { kind: 'knowledge-base'; path: string }
  | { kind: 'join'; path: '/join' }
  | { kind: 'embed'; path: string }
  | { kind: 'not-found'; path: string };

export function resolveRoute(pathname: string): RouteMatch {
  const path = pathname.split('?')[0] || '/';
  if (path === '/login') return { kind: 'login', path: '/login' };
  if (path === '/join') return { kind: 'join', path: '/join' };
  if (path.startsWith('/embed/')) return { kind: 'embed', path };
  if (path === '/knowledgeBase' || path.startsWith('/knowledgeBase/')) return { kind: 'knowledge-base', path };
  if (path === '/platform' || path.startsWith('/platform/')) return { kind: 'platform', path };
  if (path === '/creatChat' || path.startsWith('/creatChat/')) return { kind: 'platform', path: `/platform/creatChat${path.slice('/creatChat'.length)}` };
  return { kind: 'not-found', path };
}

export function routeRedirect(pathname: string): string | undefined {
  const match = resolveRoute(pathname);
  if (match.kind !== 'platform') return undefined;
  const query = pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '';
  return match.path !== pathname.split('?')[0] ? `${match.path}${query}` : undefined;
}
