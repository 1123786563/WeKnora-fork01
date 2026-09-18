export interface ProductSessionNavigation { spaceId: string; agentId: string; targetId: string; workspaceRef: string; resourceUserId: string; resourceTenantId: string; resourceSessionId: string; runId: string; }

export function createProductSessionNavigation(input: { sessionId: string; spaceId: string; agentId: string; targetId: string; workspaceRef: string; userId: string; tenantId: string; runId: string }): ProductSessionNavigation {
    for (const [name, value] of Object.entries(input)) {
        if (name !== 'runId' && (typeof value !== 'string' || value.trim() === '')) throw new Error(`PRODUCT_SESSION_${name.toUpperCase()}_REQUIRED`);
    }
    if (input.runId.trim() === '') throw new Error('PRODUCT_SESSION_RUN_ID_REQUIRED');
    return { spaceId: input.spaceId, agentId: input.agentId, targetId: input.targetId, workspaceRef: input.workspaceRef, resourceUserId: input.userId, resourceTenantId: input.tenantId, resourceSessionId: input.sessionId, runId: input.runId };
}
