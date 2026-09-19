import { integrationSettingsQuery } from '../../../packages/views/src/integrations/settings-route.ts';
import { isCapabilitySupported } from '@weknora/domain';

export type RouteMatch =
  | { kind: 'login'; path: '/login' | '/register'; mode?: 'login' | 'register' }
  | { kind: 'platform'; path: string }
  | { kind: 'knowledge-base'; path: string; knowledgeBaseId?: string; tab?: 'documents' | 'wiki' | 'graph'; slug?: string; initialDocumentId?: string }
  | { kind: 'chat'; path: string; knowledgeBaseId?: string }
  | { kind: 'knowledge-document'; path: string; knowledgeBaseId: string; documentId: string }
  | { kind: 'knowledge-wiki'; path: string; knowledgeBaseId: string }
  | { kind: 'knowledge-faq'; path: string; knowledgeBaseId: string }
  | { kind: 'knowledge-settings'; path: string; knowledgeBaseId: string }
  | { kind: 'join'; path: '/join' }
  | { kind: 'onboarding'; path: '/onboarding/workspace' }
  | { kind: 'apps'; path: string; mode: 'catalog' | 'connections' | 'authorization' | 'action'; id?: string }
  | { kind: 'craft'; path: string }
  | { kind: 'embed'; path: string }
  | { kind: 'not-found'; path: string };

export function resolveRoute(pathname: string, options: { development?: boolean } = {}): RouteMatch {
  const path = pathname.split('?')[0] || '/';
  const development = options.development ?? false;
  const query = new URLSearchParams(pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '');
  const knowledgeBaseView = (): { tab?: 'documents' | 'wiki' | 'graph'; slug?: string } => {
    const requestedTab = query.get('tab');
    const tab = requestedTab === 'wiki' || requestedTab === 'graph' || requestedTab === 'documents' ? requestedTab : undefined;
    const slug = query.get('slug')?.trim() || undefined;
    return { ...(tab ? { tab } : {}), ...(slug ? { slug } : {}) };
  };
  const knowledgeBaseDocumentEntry = (): { initialDocumentId?: string } => {
    const initialDocumentId = query.get('knowledge_id')?.trim() || undefined;
    return initialDocumentId ? { initialDocumentId } : {};
  };
  const decodeSegment = (value: string): string | undefined => {
    try { return decodeURIComponent(value); } catch { return undefined; }
  };
  if (path === '/login') return { kind: 'login', path: '/login' };
  if (path === '/register') return { kind: 'login', path: '/register', mode: 'register' };
  if (path === '/join') return { kind: 'join', path: '/join' };
  if (path === '/onboarding/workspace') return { kind: 'onboarding', path };
  if (path === '/platform/apps') return { kind: 'apps', path, mode: 'catalog' };
  if (path === '/platform/apps/connections') return { kind: 'apps', path, mode: 'connections' };
  const appAuthorization = path.match(/^\/platform\/apps\/authorization\/([^/]+)$/);
  if (appAuthorization) {
    const id = decodeSegment(appAuthorization[1]!);
    return id === undefined ? { kind: 'not-found', path } : { kind: 'apps', path, mode: 'authorization', id };
  }
  const appAction = path.match(/^\/platform\/apps\/actions\/([^/]+)$/);
  if (appAction) {
    const id = decodeSegment(appAction[1]!);
    return id === undefined ? { kind: 'not-found', path } : { kind: 'apps', path, mode: 'action', id };
  }
  if (path === '/craft' || path.startsWith('/craft/')) return { kind: 'craft', path };
  if (path.startsWith('/embed/')) return { kind: 'embed', path };
  const documentMatch = path.match(/^\/knowledgeBase\/([^/]+)\/documents\/([^/]+)$/);
  if (documentMatch) {
    const knowledgeBaseId = decodeSegment(documentMatch[1]!);
    const documentId = decodeSegment(documentMatch[2]!);
    return knowledgeBaseId === undefined || documentId === undefined
      ? { kind: 'not-found', path }
      : { kind: 'knowledge-document', path, knowledgeBaseId, documentId };
  }
  const wikiMatch = path.match(/^\/knowledgeBase\/([^/]+)\/wiki$/);
  if (wikiMatch) { const knowledgeBaseId = decodeSegment(wikiMatch[1]!); return knowledgeBaseId === undefined ? { kind: 'not-found', path } : { kind: 'knowledge-wiki', path, knowledgeBaseId }; }
  const faqMatch = path.match(/^\/knowledgeBase\/([^/]+)\/faq$/);
  if (faqMatch) { const knowledgeBaseId = decodeSegment(faqMatch[1]!); return knowledgeBaseId === undefined ? { kind: 'not-found', path } : { kind: 'knowledge-faq', path, knowledgeBaseId }; }
  const settingsMatch = path.match(/^\/knowledgeBase\/([^/]+)\/settings$/);
  if (settingsMatch) { const knowledgeBaseId = decodeSegment(settingsMatch[1]!); return knowledgeBaseId === undefined ? { kind: 'not-found', path } : { kind: 'knowledge-settings', path, knowledgeBaseId }; }
  const knowledgeBaseMatch = path.match(/^\/knowledgeBase\/([^/]+)$/);
  if (knowledgeBaseMatch) { const knowledgeBaseId = decodeSegment(knowledgeBaseMatch[1]!); return knowledgeBaseId === undefined ? { kind: 'not-found', path } : { kind: 'knowledge-base', path, knowledgeBaseId, ...knowledgeBaseView(), ...knowledgeBaseDocumentEntry() }; }
  if (path === '/knowledgeBase') return { kind: 'knowledge-base', path };
  const platformKnowledgeChatMatch = path.match(/^\/platform\/knowledge-bases\/([^/]+)\/creatChat$/);
  if (platformKnowledgeChatMatch) { const knowledgeBaseId = decodeSegment(platformKnowledgeChatMatch[1]!); return knowledgeBaseId === undefined ? { kind: 'not-found', path } : { kind: 'chat', path, knowledgeBaseId }; }
  const platformKnowledgeBaseMatch = path.match(/^\/platform\/knowledge-bases\/([^/]+)$/);
  if (platformKnowledgeBaseMatch) { const knowledgeBaseId = decodeSegment(platformKnowledgeBaseMatch[1]!); return knowledgeBaseId === undefined ? { kind: 'not-found', path } : { kind: 'knowledge-base', path, knowledgeBaseId, ...knowledgeBaseView(), ...knowledgeBaseDocumentEntry() }; }
  if (path === '/platform' || path === '/platform/knowledge-bases' || path === '/platform/knowledge-search' || path === '/platform/agents' || path === '/platform/integrations' || path === '/platform/creatChat' || path === '/platform/tenant' || path === '/platform/organizations' || path === '/platform/analytics' || path === '/platform/settings' || path === '/platform/configuration' || path === '/platform/administration' || path === '/platform/system' || path === '/platform/system/settings' || path === '/platform/system/admins' || path === '/platform/system/queues' || (development && path === '/platform/dev/markdown') || path.startsWith('/platform/chat/')) return { kind: 'platform', path };
  // Preserve the historical flat chat entry point while converging on the
  // Vue-compatible creatChat route used by the current shell.
  if (path === '/platform/chat') return { kind: 'platform', path: '/platform/creatChat' };
  if (path === '/creatChat') return { kind: 'platform', path: '/platform/creatChat' };
  return { kind: 'not-found', path };
}

export function routeRedirect(pathname: string): string | undefined {
  const match = resolveRoute(pathname);
  if (match.path === '/') return '/platform/knowledge-bases';
  if (match.path === '/platform/knowledge-search') {
    const query = new URLSearchParams(pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '');
    return `/platform/knowledge-bases?cmdk=${encodeURIComponent(query.get('q') ?? '')}`;
  }
  if (match.kind === 'join') {
    const query = new URLSearchParams(pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '');
    const code = query.get('code')?.trim();
    return code ? `/platform/organizations?invite_code=${encodeURIComponent(code)}` : '/platform/organizations';
  }
  if (match.path === '/platform/integrations') return `/platform/settings?${integrationSettingsQuery(pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '', true)}`;
  if (match.path === '/platform/tenant') return '/platform/settings';
  if (match.path === '/platform/administration') return '/platform/settings?section=members';
  if (match.path === '/platform/system' || match.path === '/platform/system/settings' || match.path === '/platform/system/admins') return '/platform/settings?section=system-global';
  if (match.path === '/platform/system/queues') return '/platform/settings?section=runtime-queues';
  if (match.kind !== 'platform') return undefined;
  const query = pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '';
  return match.path !== pathname.split('?')[0] ? `${match.path}${query}` : undefined;
}

export function protectedPageForRoute(route: RouteMatch): 'knowledge-bases' | 'markdown-test' | 'other' {
  if (route.kind === 'knowledge-base' && !route.knowledgeBaseId) return 'knowledge-bases';
  if (route.kind === 'platform' && route.path === '/platform/knowledge-bases') return 'knowledge-bases';
  if (route.kind === 'platform' && route.path === '/platform/dev/markdown') return 'markdown-test';
  return 'other';
}

export function organizationInviteCode(pathname: string): string | undefined {
  const code = new URLSearchParams(pathname.includes('?') ? pathname.slice(pathname.indexOf('?')) : '').get('invite_code')?.trim();
  return code || undefined;
}

export function authNavigationTarget(): string {
  // Vue parity: Login.vue persistLoginResponse always lands on the fixed
  // workspace home (hasValidTenant ? '/platform/knowledge-bases' :
  // '/onboarding/workspace') and never consumes a ?next return URL. The
  // onboarding split is owned by guardRoute, so the auth target is constant.
  return '/platform/knowledge-bases';
}

/**
 * Settings owns same-path query/subsection history through its popstate
 * listener. A full reload is only needed when browser history changes the
 * page path and the entry point must select a different route tree.
 */
export function shouldReloadOnPopState(previousPathname: string, currentPathname: string): boolean {
  return previousPathname !== currentPathname;
}

export interface RouteGuardContext {
  authenticated: boolean;
  tenantId: string | null;
  capabilities: Record<string, { supported: boolean; reason?: string }>;
  isSystemAdmin: boolean;
  liteMode?: boolean;
  edition?: string;
  development?: boolean;
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
  // Login redirects carry no return URL (Vue parity — router/index.ts:368
  // `next('/login')`): after authentication the app always lands on the
  // default home, or onboarding when no tenant exists yet.
  const rawPath = pathname.split('?')[0] || '/';
  const resolved = resolveRoute(pathname, { development: context.development });
  const path = resolved.kind === 'platform' && resolved.path === '/platform/creatChat' ? resolved.path : rawPath;
  if (path.startsWith('/embed/')) return { kind: 'allow' };
  if (path === '/login' || path === '/register') return { kind: 'allow' };
  if (path === '/craft' || path.startsWith('/craft/')) return { kind: 'allow' };
  if (path === '/join') {
    if (!context.authenticated) return { kind: 'redirect', to: '/login', reason: 'authentication-required' };
    return { kind: 'redirect', to: routeRedirect(pathname) ?? '/platform/organizations', reason: 'capability-unavailable' };
  }
  if (path === '/onboarding/workspace') {
    if (!context.authenticated) return { kind: 'redirect', to: '/login', reason: 'authentication-required' };
    return context.tenantId ? { kind: 'redirect', to: '/platform/knowledge-bases', reason: 'workspace-required' } : { kind: 'allow' };
  }
  if (path === '/') {
    if (!context.authenticated) return { kind: 'redirect', to: '/login', reason: 'authentication-required' };
    if (!context.tenantId) return { kind: 'redirect', to: '/onboarding/workspace', reason: 'workspace-required' };
    return { kind: 'redirect', to: '/platform/knowledge-bases', reason: 'capability-unavailable' };
  }
  if (path === '/platform/dev/markdown') return { kind: 'allow' };
  if (resolved.kind === 'not-found') {
    if (protectedPath(path) && !context.authenticated) return { kind: 'redirect', to: '/login', reason: 'authentication-required' };
    return { kind: 'allow' };
  }
  if (!protectedPath(path)) return { kind: 'allow' };
  if (!context.authenticated) return { kind: 'redirect', to: '/login', reason: 'authentication-required' };
  if (!context.tenantId) return { kind: 'redirect', to: '/onboarding/workspace', reason: 'workspace-required' };
  if (path.startsWith('/platform/system') && !context.isSystemAdmin) return { kind: 'redirect', to: '/platform/knowledge-bases', reason: 'system-admin-required' };
  if (path === '/platform' || path === '/platform/knowledge-search' || path === '/platform/tenant' || path === '/platform/administration') {
    return { kind: 'redirect', to: routeRedirect(pathname) ?? '/platform/knowledge-bases', reason: 'capability-unavailable' };
  }
  if (path === '/platform/system' || path === '/platform/system/settings' || path === '/platform/system/admins' || path === '/platform/system/queues') return { kind: 'redirect', to: routeRedirect(pathname) ?? '/platform/settings', reason: 'capability-unavailable' };
  if (path === '/platform/integrations') return { kind: 'redirect', to: routeRedirect(pathname)!, reason: 'capability-unavailable' };
  const capability = capabilityForPath(path);
  if (capability && !isCapabilitySupported(context.capabilities, capability, { liteMode: context.liteMode, edition: context.edition })) {
    return { kind: 'redirect', to: '/platform/knowledge-bases', reason: 'capability-unavailable' };
  }
  return { kind: 'allow' };
}
