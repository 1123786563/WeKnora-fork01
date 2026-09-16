import type { Router } from "expo-router"
import { useRouter } from "expo-router"
import { useCallback } from 'react';
import { Platform } from 'react-native';
import { storage } from '@/sync/storage';
import { sync } from '@/sync/sync';
import { trackSessionSwitched } from '@/track';
import { perfMark } from '@/utils/perfLog';
import { isRunningOnMac } from '@/utils/platform';

export interface ProductSessionNavigation { spaceId: string; agentId: string; targetId: string; workspaceRef: string; resourceUserId: string; resourceTenantId: string; resourceSessionId: string; runId: string; }

/** Trusted producer used by product workbench/list/notification adapters. */
export function createProductSessionNavigation(input: {
    sessionId: string;
    spaceId: string;
    agentId: string;
    targetId: string;
    workspaceRef: string;
    userId: string;
    tenantId: string;
    runId: string;
}): ProductSessionNavigation {
    for (const [name, value] of Object.entries(input)) {
        if (name !== 'runId' && (typeof value !== 'string' || value.trim() === '')) throw new Error(`PRODUCT_SESSION_${name.toUpperCase()}_REQUIRED`);
    }
    if (input.runId.trim() === '') throw new Error('PRODUCT_SESSION_RUN_ID_REQUIRED');
    return { spaceId: input.spaceId, agentId: input.agentId, targetId: input.targetId, workspaceRef: input.workspaceRef, resourceUserId: input.userId, resourceTenantId: input.tenantId, resourceSessionId: input.sessionId, runId: input.runId };
}

function sessionHref(sessionId: string, selection?: ProductSessionNavigation): `/session/${string}` {
    const path = `/session/${encodeURIComponent(sessionId)}` as `/session/${string}`;
    if (!selection) return path;
    const query = new URLSearchParams(Object.entries(selection)).toString();
    return `${path}?${query}` as `/session/${string}`;
}

function productSelection(session: any): ProductSessionNavigation | undefined {
    const candidate = session?.metadata?.productSession as Partial<ProductSessionNavigation> | undefined;
    if (!candidate || Object.values(candidate).some((value) => typeof value !== 'string' || value.trim() === '')) return undefined;
    return createProductSessionNavigation({ sessionId: session.id, spaceId: candidate.spaceId!, agentId: candidate.agentId!, targetId: candidate.targetId!, workspaceRef: candidate.workspaceRef!, userId: candidate.resourceUserId!, tenantId: candidate.resourceTenantId!, runId: candidate.runId! });
}

export function prefetchSession(router: Router, sessionId: string) {
    // Native stack owns the off-screen instance. Web keeps its current
    // navigation behavior; mounting its file/sidebar tree is not a warmup.
    if (Platform.OS === 'web' || isRunningOnMac() || !storage.getState().sessions[sessionId]
        || storage.getState().currentViewingSessionId === sessionId) {
        return;
    }
    perfMark(`session-preload:${sessionId}`);
    sync.preloadSession(sessionId);
    try {
        router.prefetch(sessionHref(sessionId, productSelection(storage.getState().sessions[sessionId])));
    } catch (error) {
        // Preparation is optional; a failed hint must not break the press.
        console.warn('Unable to prefetch session screen', error);
    }
}

export function navigateToSession(router: Router, sessionId: string, product?: ProductSessionNavigation) {
    perfMark(`session-open:${sessionId}`);
    const session = storage.getState().sessions[sessionId];
    if (session) {
        trackSessionSwitched(session);
    }

    const selection = product ?? productSelection(session);
    router.push(sessionHref(sessionId, selection));
}

export function useNavigateToSession() {
    const router = useRouter();
    return useCallback((sessionId: string, product?: ProductSessionNavigation) => {
        navigateToSession(router, sessionId, product);
    }, [router]);
}

/** Pressable owns tap cancellation, scrolling and long-press recognition. */
export function useSessionPressHandlers(sessionId: string) {
    const router = useRouter();
    const onPressIn = useCallback(() => prefetchSession(router, sessionId), [router, sessionId]);
    const onPress = useCallback(() => navigateToSession(router, sessionId), [router, sessionId]);
    return { onPressIn, onPress };
}
