import { isCapabilitySupported } from '@weknora/domain';

export type RouteMatch =
  | { kind: 'login'; path: '/login' | '/register'; mode?: 'login' | 'register' }
  | { kind: 'platform'; path: string }
  | { kind: 'knowledge-base'; path: string; knowledgeBaseId?: string }
  | { kind: 'chat'; path: string; knowledgeBaseId?: string }
  | { kind: 'knowledge-document'; path: string; knowledgeBaseId: string; documentId: string }
  | { kind: 'knowledge-wiki'; path: string; knowledgeBaseId: string }
  | { kind: 'knowledge-faq'; path: string; knowledgeBaseId: string }
  | { kind: 'knowledge-settings'; path: string; knowledgeBaseId: string }
  | { kind: 'join'; path: '/join' }
  | { kind: 'onboarding'; path: '/onboarding/workspace' }
  | { kind: 'embed'; path: string }
  | { kind: 'not-found'; path: string };

export function resolveRoute(pathname: string): RouteMatch {
  const path = pathname.split('?')[0] || '/';
  if (path === '/login') return { kind: 'login', path: '/login' };
  if (path === '/register') return { kind: 'login', path: '/register', mode: 'register' };
  if (path === '/join') return { kind: 'join', path: '/join' };
  if (path === '/onboarding/workspace') return { kind: 'onboarding', path };
  if (path.startsWith('/embed/')) return { kind: 'embed', path };
  const documentMatch = path.match(/^\/knowledgeBase\/([^/]+)\/documents\/([^/]+)$/);
  if (documentMatch) return { kind: 'knowledge-document', path, knowledgeBaseId: decodeURIComponent(documentMatch[1]!), documentId: decodeURIComponent(documentMatch[2]!) };
  const wikiMatch = path.match(/^\/knowledgeBase\/([^/]+)\/wiki$/);
  if (wikiMatch) return { kind: 'knowledge-wiki', path, knowledgeBaseId: decodeURIComponent(wikiMatch[1]!) };
  const faqMatch = path.match(/^\/knowledgeBase\/([^/]+)\/faq$/);
  if (faqMatch) return { kind: 'knowledge-faq', path, knowledgeBaseId: decodeURIComponent(faqMatch[1]!) };
  const settingsMatch = path.match(/^\/knowledgeBase\/([^/]+)\/settings$/);
  if (settingsMatch) return { kind: 'knowledge-settings', path, knowledgeBaseId: decodeURIComponent(settingsMatch[1]!) };
  if (path === '/knowledgeBase' || path.startsWith('/knowledgeBase/')) return { kind: 'knowledge-base', path };
  const platformKnowledgeChatMatch = path.match(/^\/platform\/knowledge-bases\/([^/]+)\/creatChat$/);
  if (platformKnowledgeChatMatch) return { kind: 'chat', path, knowledgeBaseId: decodeURIComponent(platformKnowledgeChatMatch[1]!) };
  const platformKnowledgeBaseMatch = path.match(/^\/platform\/knowledge-bases\/([^/]+)$/);
  if (platformKnowledgeBaseMatch) return { kind: 'knowledge-base', path, knowledgeBaseId: decodeURIComponent(platformKnowledgeBaseMatch[1]!) };
  if (path === '/platform' || path === '/platform/knowledge-bases' || path === '/platform/knowledge-search' || path === '/platform/agents' || path === '/platform/integrations' || path === '/platform/creatChat' || path === '/platform/organizations' || path === '/platform/settings' || path === '/platform/configuration' || path === '/platform/administration' || path === '/platform/system' || path === '/platform/dev/markdown' || path.startsWith('/platform/chat/') || path.startsWith('/platform/system/')) return { kind: 'platform', path };
  if (path === '/creatChat' || path.startsWith('/creatChat/')) return { kind: 'platform', path: `/platform/creatChat${path.slice('/creatChat'.length)}` };
  return { kind: 'not-found', path };
}

export function routeRedirect(pathname: string): string | undefined {
  const match = resolveRoute(pathname);
  if (match.path === '/') return '/platform/knowledge-bases';
  if (match.path === '/platform/knowledge-search') {
    const query = pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '';
    return `/platform/knowledge-bases${query}`;
  }
  if (match.kind === 'join') {
    const query = new URLSearchParams(pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '');
    const code = query.get('code')?.trim();
    return code ? `/platform/organizations?invite_code=${encodeURIComponent(code)}` : '/platform/organizations';
  }
  if (match.kind !== 'platform') return undefined;
  const query = pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '';
  return match.path !== pathname.split('?')[0] ? `${match.path}${query}` : undefined;
}

export function protectedPageForRoute(route: RouteMatch): 'knowledge-bases' | 'other' {
  return route.kind === 'platform' && route.path === '/platform/knowledge-bases' ? 'knowledge-bases' : 'other';
}

export interface RouteGuardContext {
  authenticated: boolean;
  tenantId: string | null;
  capabilities: Record<string, { supported: boolean; reason?: string }>;
  isSystemAdmin: boolean;
  liteMode?: boolean;
  edition?: string;
}

export type RouteGuardDecision =
  | { kind: 'allow' }
  | { kind: 'redirect'; to: string; reason: 'authentication-required' | 'workspace-required' | 'capability-unavailable' | 'system-admin-required' };

function protectedPath(path: string): boolean {
  return path === '/knowledgeBase' || path.startsWith('/knowledgeBase/') || path === '/platform' || path.startsWith('/platform/');
}

function capabilityForPath(path: string): string | undefined {
  if (path === '/platform/organizations' || path.startsWith('/platform/organizations/')) return 'organizations';
  if (path === '/platform/agents' || path.startsWith('/platform/agents/')) return 'agents';
  if (path.startsWith('/platform/integrations')) return 'integrations';
  return undefined;
}

export function guardRoute(pathname: string, context: RouteGuardContext): RouteGuardDecision {
  const path = pathname.split('?')[0] || '/';
  if (path.startsWith('/embed/')) return { kind: 'allow' };
  if (path === '/login' || path === '/register') return { kind: 'allow' };
  if (path === '/join') {
    if (!context.authenticated) return { kind: 'redirect', to: `/login?next=${encodeURIComponent(pathname)}`, reason: 'authentication-required' };
    return { kind: 'redirect', to: routeRedirect(pathname) ?? '/platform/organizations', reason: 'capability-unavailable' };
  }
  if (path === '/onboarding/workspace') {
    if (!context.authenticated) return { kind: 'redirect', to: '/login', reason: 'authentication-required' };
    return context.tenantId ? { kind: 'redirect', to: '/platform/knowledge-bases', reason: 'workspace-required' } : { kind: 'allow' };
  }
  if (path === '/') {
    if (!context.authenticated) return { kind: 'redirect', to: `/login?next=${encodeURIComponent(pathname)}`, reason: 'authentication-required' };
    if (!context.tenantId) return { kind: 'redirect', to: '/onboarding/workspace', reason: 'workspace-required' };
    return { kind: 'redirect', to: '/platform/knowledge-bases', reason: 'capability-unavailable' };
  }
  if (path === '/platform' || path === '/platform/knowledge-search') {
    return { kind: 'redirect', to: routeRedirect(pathname) ?? '/platform/knowledge-bases', reason: 'capability-unavailable' };
  }
  if (!protectedPath(path)) return { kind: 'allow' };
  if (!context.authenticated) return { kind: 'redirect', to: `/login?next=${encodeURIComponent(pathname)}`, reason: 'authentication-required' };
  if (!context.tenantId) return { kind: 'redirect', to: '/onboarding/workspace', reason: 'workspace-required' };
  if (path.startsWith('/platform/system') && !context.isSystemAdmin) return { kind: 'redirect', to: '/platform/settings', reason: 'system-admin-required' };
  if (path.startsWith('/platform/system/')) return { kind: 'redirect', to: '/platform/system', reason: 'capability-unavailable' };
  const capability = capabilityForPath(path);
  if (capability && !isCapabilitySupported(context.capabilities, capability, { liteMode: context.liteMode, edition: context.edition })) {
    return { kind: 'redirect', to: '/platform/knowledge-bases', reason: 'capability-unavailable' };
  }
  return { kind: 'allow' };
}
