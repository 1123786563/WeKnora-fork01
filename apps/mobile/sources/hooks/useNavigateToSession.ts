import type { Router } from "expo-router"
import { useRouter } from "expo-router"
import { useCallback } from 'react';
import { Platform } from 'react-native';
import { storage } from '@/sync/storage';
import { sync } from '@/sync/sync';
import { trackSessionSwitched } from '@/track';
import { perfMark } from '@/utils/perfLog';
import { isRunningOnMac } from '@/utils/platform';

export interface ProductSessionNavigation { spaceId: string; agentId: string; targetId: string; workspaceRef: string; resourceUserId: string; resourceTenantId: string; }

function sessionHref(sessionId: string, selection?: ProductSessionNavigation): `/session/${string}` {
    const path = `/session/${encodeURIComponent(sessionId)}` as `/session/${string}`;
    if (!selection) return path;
    const query = new URLSearchParams(Object.entries(selection)).toString();
    return `${path}?${query}` as `/session/${string}`;
}

function productSelection(session: any): ProductSessionNavigation | undefined {
    const candidate = session?.metadata?.productSession as Partial<ProductSessionNavigation> | undefined;
    if (!candidate || Object.values(candidate).some((value) => typeof value !== 'string' || value.trim() === '')) return undefined;
    return candidate as ProductSessionNavigation;
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

export function navigateToSession(router: Router, sessionId: string) {
    perfMark(`session-open:${sessionId}`);
    const session = storage.getState().sessions[sessionId];
    if (session) {
        trackSessionSwitched(session);
    }

    const selection = productSelection(session);
    router.push(sessionHref(sessionId, selection));
}

export function useNavigateToSession() {
    const router = useRouter();
    return useCallback((sessionId: string) => {
        navigateToSession(router, sessionId);
    }, [router]);
}

/** Pressable owns tap cancellation, scrolling and long-press recognition. */
export function useSessionPressHandlers(sessionId: string) {
    const router = useRouter();
    const onPressIn = useCallback(() => prefetchSession(router, sessionId), [router, sessionId]);
    const onPress = useCallback(() => navigateToSession(router, sessionId), [router, sessionId]);
    return { onPressIn, onPress };
}
