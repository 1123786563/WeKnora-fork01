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
  window.history.pushState = ((...args: Parameters<History['pushState']>) => { originalPushState(...args); notify(); }) as History['pushState'];
  window.history.replaceState = ((...args: Parameters<History['replaceState']>) => { originalReplaceState(...args); notify(); }) as History['replaceState'];
  window.addEventListener('popstate', onPopState);
  return () => {
    window.history.pushState = originalPushState;
    window.history.replaceState = originalReplaceState;
    window.removeEventListener('popstate', onPopState);
    lastNotifiedUrl = '';
    installed = false;
  };
}

export function navigate(path: string, mode: NavigationMode = 'push'): void {
  const method = mode === 'replace' ? window.history.replaceState : window.history.pushState;
  method.call(window.history, {}, document.title, path);
}
