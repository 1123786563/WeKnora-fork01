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
