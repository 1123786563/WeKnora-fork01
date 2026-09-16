import { describe, expect, it, vi } from 'vitest';
import { completeProductSessionCreation, navigateCreatedProductSession, resolveCreatedProductSessionNavigation } from './productSessionEntry';

const session = { id: 'session-1', metadata: { productSession: { spaceId: 'space-1', agentId: 'agent-1', targetId: 'target-1', workspaceRef: 'workspace-1', resourceUserId: 'user-1', resourceTenantId: 'tenant-1', runId: 'run-1' } } };

describe('new product session production adapter', () => {
    it('navigates with the complete persisted product payload after creation', () => {
        const navigate = vi.fn(() => true);
        expect(navigateCreatedProductSession({ sessionId: session.id, session, navigateProduct: navigate, onUnavailable: vi.fn() })).toBe(true);
        expect(navigate).toHaveBeenCalledWith(expect.objectContaining({ id: 'session-1', metadata: { productSession: expect.objectContaining({
            spaceId: 'space-1', agentId: 'agent-1', targetId: 'target-1', workspaceRef: 'workspace-1',
            resourceUserId: 'user-1', resourceTenantId: 'tenant-1', resourceSessionId: 'session-1', runId: 'run-1',
        }) } }));
    });

    it('fails closed without navigating when persisted metadata is missing', () => {
        const navigate = vi.fn(() => true);
        const unavailable = vi.fn();
        expect(resolveCreatedProductSessionNavigation({ id: session.id, metadata: {} })).toBeUndefined();
        expect(navigateCreatedProductSession({ sessionId: session.id, session: { id: session.id, metadata: {} }, navigateProduct: navigate, onUnavailable: unavailable })).toBe(false);
        expect(navigate).not.toHaveBeenCalled();
        expect(unavailable).toHaveBeenCalledOnce();
    });
});

it('runs refresh, reads durable metadata, then routes the created session', async () => {
    const events: string[] = [];
    const navigate = vi.fn(() => { events.push('router.push'); return true; });
    const result = await completeProductSessionCreation({
        sessionId: session.id,
        refreshSessions: async () => { events.push('refreshSessions'); },
        readSession: () => { events.push('storage.session'); return session; },
        navigateProduct: navigate,
        onUnavailable: vi.fn(),
    });
    expect(result).toBe(true);
    expect(events).toEqual(['refreshSessions', 'storage.session', 'router.push']);
    expect(navigate).toHaveBeenCalledWith(expect.objectContaining({ metadata: expect.objectContaining({ productSession: expect.objectContaining({ runId: 'run-1' }) }) }));
});
