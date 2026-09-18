import { createProductSessionNavigation } from './productSessionNavigation';
function getObjectValue(value: unknown, key: string): unknown {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
        return null;
    }
    return (value as Record<string, unknown>)[key];
}

function parseJson(value: string): unknown {
    try {
        return JSON.parse(value);
    } catch {
        return value;
    }
}

function normalizeNotificationData(data: unknown): unknown {
    if (typeof data === 'string') {
        return parseJson(data);
    }
    return data;
}

function getSessionIdFromUrl(url: string): string | null {
    const trimmedUrl = url.trim();
    if (!trimmedUrl) {
        return null;
    }

    const match = trimmedUrl.match(/(?:^|\/)session\/([^/?#]+)/);
    if (!match) {
        return null;
    }

    const encodedSessionId = match[1];
    const sessionId = (() => {
        try {
            return decodeURIComponent(encodedSessionId);
        } catch {
            return encodedSessionId;
        }
    })();

    const trimmedSessionId = sessionId.trim();
    if (!trimmedSessionId) {
        return null;
    }

    return trimmedSessionId;
}

function getProductRoute(sessionId: string, data: Record<string, unknown>): `/session/${string}` | null {
    const product = getObjectValue(data, 'productSession');
    if (!product || typeof product !== 'object' || Array.isArray(product)) return null;
    const value = product as Record<string, unknown>;
    try {
        const navigation = createProductSessionNavigation({
            sessionId,
            spaceId: value.spaceId as string,
            agentId: value.agentId as string,
            targetId: value.targetId as string,
            workspaceRef: value.workspaceRef as string,
            userId: value.resourceUserId as string,
            tenantId: value.resourceTenantId as string,
            runId: value.runId as string,
        });
        return `/session/${encodeURIComponent(sessionId)}?${new URLSearchParams(Object.entries(navigation)).toString()}` as `/session/${string}`;
    } catch {
        return null;
    }
}

export function getSessionRouteFromNotificationData(data: unknown): `/session/${string}` | null {
    const normalizedData = normalizeNotificationData(data);
    if (!normalizedData || typeof normalizedData !== 'object' || Array.isArray(normalizedData)) {
        return null;
    }

    const url = getObjectValue(normalizedData, 'url');
    if (typeof url === 'string') {
        const sessionId = getSessionIdFromUrl(url);
        if (sessionId) return getProductRoute(sessionId, normalizedData as Record<string, unknown>);
    }

    const sessionId = getObjectValue(normalizedData, 'sessionId');
    if (typeof sessionId !== 'string') {
        return null;
    }

    const trimmedSessionId = sessionId.trim();
    if (!trimmedSessionId) {
        return null;
    }

    return getProductRoute(trimmedSessionId, normalizedData as Record<string, unknown>);
}

export function getSessionRouteFromNotificationResponse(response: unknown): `/session/${string}` | null {
    const contentData = getObjectValue(getObjectValue(getObjectValue(response, 'notification'), 'request'), 'content');
    return getSessionRouteFromNotificationData(getObjectValue(contentData, 'data'));
}
