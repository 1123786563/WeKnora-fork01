/**
 * The platform shell's session list navigates by route (Vue menu.vue rows
 * navigate to chat/:id). On global chat routes the shell dispatches this
 * event and the mounted ChatRoutePage performs the in-place switch; on other
 * pages the shell performs a full navigation instead.
 */
export const SHELL_SESSION_ROUTE_EVENT = 'weknora:session-route-change';

export function chatSessionIdFromPath(pathname: string): string | null {
  const match = /^\/platform\/chat\/([^/]+)$/.exec(pathname);
  if (!match) return null;
  try {
    const sessionId = decodeURIComponent(match[1]!);
    return sessionId.trim() ? sessionId : null;
  } catch {
    return null;
  }
}
