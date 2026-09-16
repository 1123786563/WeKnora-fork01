import { describe, expect, it } from 'vitest';
import {
    getSessionRouteFromNotificationData,
    getSessionRouteFromNotificationResponse,
} from './notificationRouting';

const productSession = { spaceId: 'space-1', agentId: 'agent-1', targetId: 'target-1', workspaceRef: 'workspace-1', resourceUserId: 'user-1', resourceTenantId: 'tenant-1', runId: 'run-1' };

describe('getSessionRouteFromNotificationData', () => {
    it('returns a session route when sessionId exists', () => {
        expect(getSessionRouteFromNotificationData({ sessionId: 'session-123', productSession })).toContain('/session/session-123?');
    });

    it('encodes session ids that contain spaces', () => {
        expect(getSessionRouteFromNotificationData({ sessionId: 'session 123', productSession })).toContain('/session/session%20123?');
    });

    it('returns null when sessionId is missing', () => {
        expect(getSessionRouteFromNotificationData({ kind: 'done' })).toBeNull();
    });

    it('returns null for empty session ids', () => {
        expect(getSessionRouteFromNotificationData({ sessionId: '   ' })).toBeNull();
    });

    it('uses a session url when present', () => {
        expect(getSessionRouteFromNotificationData({ url: '/session/session-123', productSession })).toContain('/session/session-123?');
    });
});

describe('getSessionRouteFromNotificationResponse', () => {
    it('reads the route from content data', () => {
        expect(getSessionRouteFromNotificationResponse({
            notification: {
                request: {
                    content: {
                        data: { sessionId: 'session-123', productSession }
                    }
                }
            }
        })).toContain('/session/session-123?');
    });

    it('returns null when content data is missing', () => {
        expect(getSessionRouteFromNotificationResponse({
            notification: {
                request: {
                    content: {}
                }
            }
        })).toBeNull();
    });
});
