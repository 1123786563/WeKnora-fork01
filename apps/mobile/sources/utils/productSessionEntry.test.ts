import { describe, expect, it, vi } from 'vitest';
import { navigateCreatedProductSession, resolveCreatedProductSessionNavigation } from './productSessionEntry';

const session = { id: 'session-1', metadata: { productSession: { spaceId: 'space-1', agentId: 'agent-1', targetId: 'target-1', workspaceRef: 'workspace-1', resourceUserId: 'user-1', resourceTenantId: 'tenant-1', runId: 'run-1' } } };

describe('new product session production adapter', () => {
    it('navigates with the complete persisted product payload after creation', () => {
        const navigate = vi.fn();
        expect(navigateCreatedProductSession({ sessionId: session.id, session, navigate, onUnavailable: vi.fn() })).toBe(true);
        expect(navigate).toHaveBeenCalledWith(session.id, expect.objectContaining({
            spaceId: 'space-1', agentId: 'agent-1', targetId: 'target-1', workspaceRef: 'workspace-1',
            resourceUserId: 'user-1', resourceTenantId: 'tenant-1', resourceSessionId: 'session-1', runId: 'run-1',
        }));
    });

    it('fails closed without navigating when persisted metadata is missing', () => {
        const navigate = vi.fn();
        const unavailable = vi.fn();
        expect(resolveCreatedProductSessionNavigation({ id: session.id, metadata: {} })).toBeUndefined();
        expect(navigateCreatedProductSession({ sessionId: session.id, session: { id: session.id, metadata: {} }, navigate, onUnavailable: unavailable })).toBe(false);
        expect(navigate).not.toHaveBeenCalled();
        expect(unavailable).toHaveBeenCalledOnce();
    });
});
