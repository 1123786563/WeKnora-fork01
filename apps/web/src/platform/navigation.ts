export type NavigationMode = 'push' | 'replace';
type NavigationListener = (url: string) => void;
const listeners = new Set<NavigationListener>();
let installed = false;
let lastNotifiedUrl = '';

// Craft and the chat page own internal URL state machines (parse window.location
// + their own popstate listeners + in-memory session state that must survive a
// same-page transition). Their URL mutations stay invisible to the router:
// re-matching them would remount the page and reload session data on every
// internal switch.
const ROUTER_EXCLUDED_PREFIXES = ['/craft', '/platform/chat', '/platform/creatChat'];

function isRouterExcluded(url: string): boolean {
  const path = url.split('?')[0]!.split('#')[0]!;
  return ROUTER_EXCLUDED_PREFIXES.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
}

function currentUrl(): string {
  return `${window.location.pathname}${window.location.search}${window.location.hash}`;
}

function notify(): void {
  const url = currentUrl();
  if (url === lastNotifiedUrl) return;
  lastNotifiedUrl = url;
  for (const listener of listeners) listener(url);
}

/**
 * Tells the router a URL mutation happened outside of its own navigation
 * calls (pages writing search params, guard SPA replaces). The router's
 * Transitioner subscribes to popstate, so re-dispatching the event makes it
 * re-read window.location and re-render the matching routes — the same
 * notify → re-render contract the pre-router app used.
 */
function syncRouter(): void {
  if (typeof PopStateEvent !== 'function') return;
  window.dispatchEvent(new PopStateEvent('popstate'));
}

/** TanStack history writes carry __TSR state; skipping the router sync for
 * them avoids a redundant second load after its own navigation. */
function isTanStackHistoryWrite(state: unknown): boolean {
  return state !== null && typeof state === 'object' && ('__TSR_index' in (state as Record<string, unknown>) || '__TSR_key' in (state as Record<string, unknown>));
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
  window.history.pushState = ((...args: Parameters<History['pushState']>) => {
    originalPushState(...args);
    notify();
    if (!isTanStackHistoryWrite(args[0]) && !isRouterExcluded(typeof args[2] === 'string' ? args[2] : currentUrl())) syncRouter();
  }) as History['pushState'];
  window.history.replaceState = ((...args: Parameters<History['replaceState']>) => {
    originalReplaceState(...args);
    notify();
    if (!isTanStackHistoryWrite(args[0]) && !isRouterExcluded(typeof args[2] === 'string' ? args[2] : currentUrl())) syncRouter();
  }) as History['replaceState'];
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
  notify();
}
