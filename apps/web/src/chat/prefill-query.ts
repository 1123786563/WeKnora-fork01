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
 *
 * R467-A2 — ?kbIds= knowledge-base scope preselect (Vue startChat kbIds).
 * Vue startChat(query, kbIds, fileIds) additionally calls
 * settingsStore.selectKnowledgeBases(kbIds) so the new conversation starts
 * scoped to those knowledge bases (Input-field renders them as selection
 * chips and sends knowledge_bases with the stream). Vue passes the scope
 * through the settings store (no URL parameter exists there); the React SPA
 * has no such global store, so the palette's scoped ask-AI stacks the scope
 * onto the R466 deep link as ?kbIds=kb-1,kb-2. Both prefill parameters share
 * the one-shot lifecycle: consumed on the new-chat entry, stripped together
 * via a single replaceState once the query is sent or cleared.
 */

/** Read the prefill query (?q=) from a search string; trimmed, '' when absent. */
export function readPrefillQuery(search: string): string {
  const value = new URLSearchParams(search).get('q');
  return value ? value.trim() : '';
}

/**
 * Read the preselected knowledge-base ids (?kbIds=) from a search string.
 * Accepts comma-separated and/or repeated values (`kbIds=a,b&kbIds=c`);
 * entries are trimmed, emptied entries dropped, duplicates removed.
 */
export function readPrefillKbIds(search: string): string[] {
  const params = new URLSearchParams(search);
  const ids = params.getAll('kbIds').flatMap((value) => value.split(','));
  return [...new Set(ids.map((id) => id.trim()).filter((id) => id.length > 0))];
}

/**
 * Strip the one-shot prefill parameters (?q= and ?kbIds=) from an href,
 * keeping sibling params and the hash. Returns the path+search+hash to
 * replaceState with, or null when there is nothing to strip.
 */
export function stripPrefillParamsFromHref(href: string): string | null {
  const url = new URL(href);
  if (!url.searchParams.has('q') && !url.searchParams.has('kbIds')) return null;
  url.searchParams.delete('q');
  url.searchParams.delete('kbIds');
  const search = url.searchParams.toString();
  return `${url.pathname}${search ? `?${search}` : ''}${url.hash}`;
}

/**
 * Remove the prefill parameters from the current URL without polluting the
 * history stack (replaceState). Injectable history for tests.
 * Returns the URL written back, or null when no prefill was present.
 */
export function clearPrefillParamsFromUrl(
  history: { replaceState(data: unknown, unused: string, url?: string | null): unknown },
  href: string,
): string | null {
  const next = stripPrefillParamsFromHref(href);
  if (next === null) return null;
  history.replaceState({}, '', next);
  return next;
}
