/**
 * Web-search readiness gates, ported from upstream
 * frontend/src/utils/agentWebSearch.ts (Vue baseline, read-only).
 *
 * R484 D15 — the chat composer's globe toggle follows the Vue showWebSearchButton
 * / isWebSearchConfigured computeds: a tenant is ready when a default engine
 * exists; an agent is ready when it enables web search and its engine resolves
 * (agent override > tenant default), or the source workspace already vouches
 * for it (shared agents carry web_search_ready).
 */

export interface WebSearchProviderLike {
  id: string;
  is_default?: boolean;
}

export interface AgentWebSearchConfigLike {
  web_search_enabled?: boolean;
  web_search_provider_id?: string;
}

/** Resolve the engine an agent would actually use (backend agent > tenant default). */
export function resolveAgentWebSearchProviderId(
  config: AgentWebSearchConfigLike | undefined,
  providers: readonly WebSearchProviderLike[],
): string | null {
  const explicitId = config?.web_search_provider_id?.trim();
  if (explicitId) {
    return providers.some((provider) => provider.id === explicitId) ? explicitId : null;
  }
  const defaultProvider = providers.find((provider) => provider.is_default);
  return defaultProvider?.id ?? null;
}

export function isAgentWebSearchEnabled(config: AgentWebSearchConfigLike | undefined): boolean {
  return config?.web_search_enabled === true;
}

/** Agent web search is on AND a usable engine resolves. */
export function isAgentWebSearchReady(
  config: AgentWebSearchConfigLike | undefined,
  providers: readonly WebSearchProviderLike[],
  sourceWorkspaceReady?: boolean,
): boolean {
  if (!isAgentWebSearchEnabled(config)) return false;
  if (sourceWorkspaceReady !== undefined) return sourceWorkspaceReady;
  return resolveAgentWebSearchProviderId(config, providers) !== null;
}

/** Tenant-level default engine availability (no agent constraint). */
export function isTenantWebSearchReady(providers: readonly WebSearchProviderLike[]): boolean {
  return providers.some((provider) => provider.is_default);
}
