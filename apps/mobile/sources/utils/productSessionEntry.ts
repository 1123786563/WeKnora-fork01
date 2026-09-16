import { createProductSessionNavigation, type ProductSessionNavigation } from './productSessionNavigation';

export function resolveCreatedProductSessionNavigation(session: unknown): ProductSessionNavigation | undefined {
    const candidate = (session as { id?: unknown; metadata?: { productSession?: Record<string, unknown> } } | null)?.metadata?.productSession;
    if (!candidate || typeof (session as { id?: unknown } | null)?.id !== 'string') return undefined;
    if (Object.values(candidate).some((value) => typeof value !== 'string' || value.trim() === '')) return undefined;
    try {
        return createProductSessionNavigation({ sessionId: (session as { id: string }).id, spaceId: candidate.spaceId as string, agentId: candidate.agentId as string, targetId: candidate.targetId as string, workspaceRef: candidate.workspaceRef as string, userId: candidate.resourceUserId as string, tenantId: candidate.resourceTenantId as string, runId: candidate.runId as string });
    } catch {
        return undefined;
    }
}

export function navigateCreatedProductSession(input: {
    sessionId: string;
    session: unknown;
    navigateProduct: (session: unknown) => boolean;
    onUnavailable: () => void;
}): boolean {
    const product = resolveCreatedProductSessionNavigation(input.session);
    if (!product) {
        input.onUnavailable();
        return false;
    }
    return input.navigateProduct({ ...(input.session as object), id: input.sessionId, metadata: { productSession: product } });
}
