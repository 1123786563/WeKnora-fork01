export function isExternalHttpUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch { return false; }
}

export function normalizeLegacyPath(pathname: string): string {
  if (pathname === '/creatChat' || pathname.startsWith('/creatChat/')) return `/platform/creatChat${pathname.slice('/creatChat'.length)}`;
  if (pathname === '/knowledgeBase' || pathname.startsWith('/knowledgeBase/')) return `/platform/knowledge-bases${pathname.slice('/knowledgeBase'.length)}`;
  return pathname || '/';
}

/** Keep Wails launches on the same canonical route contract as the browser. */
export function normalizeDesktopLocation(pathname: string, search = '', hash = ''): string {
  const normalized = normalizeLegacyPath(pathname.split('?')[0].split('#')[0] || '/');
  return `${normalized}${search || ''}${hash || ''}`;
}

/** Deep links are local app routes only; protocol URLs must never be routed in-app. */
export function isSafeDesktopDeepLink(value: string): boolean {
  return value.startsWith('/') && !value.startsWith('//') && !isExternalHttpUrl(value);
}
