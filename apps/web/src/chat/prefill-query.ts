/*
 * R466-A2 — ?q= prefill consumption (Vue menuStore.prefillQuery equivalent).
 *
 * Vue contract (frontend/src/stores/menu.ts + Input-field.vue onMounted):
 * the "问 AI" entries call useStartChat.startChat → menuStore.setPrefillQuery(q)
 * + router.push('/platform/creatChat'); the composer consumes it exactly once
 * (consumePrefillQuery): fill query.value, focus the textarea in nextTick,
 * NEVER auto-send. The React SPA carries the same one-shot seed as the
 * /platform/creatChat?q=… deep link (R465 palette onAskAi fallback), so the
 * URL parameter is consumed on the new-chat entry and stripped — via
 * replaceState only, never pushing a history entry — once the query is sent
 * or cleared.
 */

/** Read the prefill query (?q=) from a search string; trimmed, '' when absent. */
export function readPrefillQuery(search: string): string {
  const value = new URLSearchParams(search).get('q');
  return value ? value.trim() : '';
}

/**
 * Strip ?q= from an href, keeping sibling params and the hash.
 * Returns the path+search+hash to replaceState with, or null when there is
 * nothing to strip (no q parameter).
 */
export function stripPrefillQueryFromHref(href: string): string | null {
  const url = new URL(href);
  if (!url.searchParams.has('q')) return null;
  url.searchParams.delete('q');
  const search = url.searchParams.toString();
  return `${url.pathname}${search ? `?${search}` : ''}${url.hash}`;
}

/**
 * Remove the prefill query from the current URL without polluting the
 * history stack (replaceState). Injectable history for tests.
 * Returns the URL written back, or null when no prefill was present.
 */
export function clearPrefillQueryFromUrl(
  history: { replaceState(data: unknown, unused: string, url?: string | null): unknown },
  href: string,
): string | null {
  const next = stripPrefillQueryFromHref(href);
  if (next === null) return null;
  history.replaceState({}, '', next);
  return next;
}
