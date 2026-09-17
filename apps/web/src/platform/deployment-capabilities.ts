// R465-A2 — deployment-capability gating for the command palette, ported
// from frontend/src/config/deploymentCapabilities.ts
// (isDeploymentCapabilitySupported) plus the palette-specific access shape
// consumed by visibleCommands() in command-palette.ts.
//
// Semantics (Vue is authoritative):
//   • Fail-open: a missing key, a failed probe, or an old backend that never
//     returns the entry keeps the UI entry VISIBLE — only an explicit
//     `supported: false` hides it.
//   • `organizations` is additionally forced off on lite deployments
//     (liteMode flag OR edition 'lite'); no other key is lite-gated.
//   • The palette's open-organizations quick action also requires an admin
//     tenant role (Vue menu.ts gates the sidebar entry the same way).
//
// The data comes from GET /api/v1/system/capabilities
// (client.administration.capabilities()); this module stays DOM-free so the
// semantics are unit-testable in isolation.

/** Deployment capabilities as returned by /api/v1/system/capabilities. */
export interface PaletteDeploymentCapabilities {
  edition: string;
  capabilities: Record<string, { supported: boolean; reason?: string }>;
}

/** Capability keys the palette consumes. */
export type PaletteCapabilityKey = 'agents' | 'organizations';

export function isPaletteCapabilitySupported(
  deployment: PaletteDeploymentCapabilities | null,
  key: PaletteCapabilityKey,
  options?: { liteMode?: boolean; edition?: string },
): boolean {
  if (!deployment) return true; // probe failed / not loaded yet → fail-open
  if (key === 'organizations') {
    const isLite =
      options?.liteMode === true
      || options?.edition?.trim().toLowerCase() === 'lite'
      || deployment.edition.trim().toLowerCase() === 'lite';
    if (isLite) return false;
  }
  return deployment.capabilities[key]?.supported !== false;
}

/**
 * Quick-action access flags for visibleCommands(). `isAdmin` mirrors Vue's
 * authStore.hasRole('admin') check on the organizations entry.
 */
export function paletteAccessFromCapabilities(
  deployment: PaletteDeploymentCapabilities | null,
  context: { liteMode: boolean; isAdmin: boolean },
): { canOpenAgents: boolean; canOpenOrganizations: boolean } {
  return {
    canOpenAgents: isPaletteCapabilitySupported(deployment, 'agents'),
    canOpenOrganizations:
      context.isAdmin
      && isPaletteCapabilitySupported(deployment, 'organizations', { liteMode: context.liteMode, edition: deployment?.edition ?? '' }),
  };
}

/** Load capabilities for the palette; a failed probe fail-opens as null. */
export async function loadPaletteDeploymentCapabilities(
  fetcher: () => Promise<PaletteDeploymentCapabilities>,
): Promise<PaletteDeploymentCapabilities | null> {
  try {
    return await fetcher();
  } catch {
    return null;
  }
}
