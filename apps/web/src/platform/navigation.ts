export type NavigationMode = 'push' | 'replace';
type NavigationListener = (url: string) => void;
const listeners = new Set<NavigationListener>();
let installed = false;
let lastNotifiedUrl = '';

function currentUrl(): string {
  return `${window.location.pathname}${window.location.search}${window.location.hash}`;
}

function notify(): void {
  const url = currentUrl();
  if (url === lastNotifiedUrl) return;
  lastNotifiedUrl = url;
  for (const listener of listeners) listener(url);
}

export function subscribeNavigation(listener: NavigationListener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function installNavigationObserver(): () => void {
  if (installed) return () => undefined;
  installed = true;
  lastNotifiedUrl = currentUrl();
  const originalPushState = window.history.pushState.bind(window.history);
  const originalReplaceState = window.history.replaceState.bind(window.history);
  const onPopState = () => notify();
  const onDocumentClick = (event: MouseEvent) => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const target = event.target;
    if (!(target instanceof Element)) return;
    const anchor = target.closest('a[href]');
    if (!(anchor instanceof HTMLAnchorElement) || anchor.target === '_blank' || anchor.hasAttribute('download')) return;
    const rawHref = anchor.getAttribute('href');
    // Preserve in-place controls such as the auth page's `href="#"` toggle.
    if (!rawHref || rawHref === '#' || rawHref.startsWith('javascript:')) return;
    const url = new URL(anchor.href, window.location.href);
    if (url.origin !== window.location.origin || url.pathname.startsWith('/api/')) return;
    event.preventDefault();
    navigate(`${url.pathname}${url.search}${url.hash}`);
  };
  window.history.pushState = ((...args: Parameters<History['pushState']>) => { originalPushState(...args); notify(); }) as History['pushState'];
  window.history.replaceState = ((...args: Parameters<History['replaceState']>) => { originalReplaceState(...args); notify(); }) as History['replaceState'];
  window.addEventListener('popstate', onPopState);
  // Capture before React or a nested menu can stop propagation without
  // preventing the browser's default document navigation.
  document.addEventListener('click', onDocumentClick, true);
  return () => {
    window.history.pushState = originalPushState;
    window.history.replaceState = originalReplaceState;
    window.removeEventListener('popstate', onPopState);
    document.removeEventListener('click', onDocumentClick, true);
    lastNotifiedUrl = '';
    installed = false;
  };
}

export function navigate(path: string, mode: NavigationMode = 'push'): void {
  const method = mode === 'replace' ? window.history.replaceState : window.history.pushState;
  method.call(window.history, {}, document.title, path);
}
