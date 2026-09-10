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
