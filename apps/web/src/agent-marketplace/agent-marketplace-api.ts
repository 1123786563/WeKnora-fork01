import { createAgentVersionsApi } from '@weknora/api-client';
import type { WeKnoraClient } from '@weknora/api-client';
import { createTenantReleaseApi } from '@weknora/api-client';

/** Tenant Release Web workflow adapters over the authenticated shared client transport. */
export function createAgentMarketplaceApi(client: Pick<WeKnoraClient, 'request'>) {
  return {
    versions: createAgentVersionsApi(client.request),
    releases: createTenantReleaseApi(client.request),
  };
}

export type AgentMarketplaceApi = ReturnType<typeof createAgentMarketplaceApi>;
