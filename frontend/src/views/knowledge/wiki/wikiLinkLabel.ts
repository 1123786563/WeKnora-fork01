/**
 * Resolve the display label for a wiki link slug (backlink footer entries).
 *
 * The page payload carries `link_titles`, a slug→title map the backend
 * assembles in one batched query for every slug in in_links / out_links.
 * Backlinks frequently point at concept/entity pages the sidebar never
 * loaded, whose bare slug is a pinyin kebab-case string rather than a
 * readable title — so the map wins whenever it knows the slug, and the
 * caller's fallback (already-loaded pages, then the slug) takes over.
 */
export function wikiLinkLabel(
  linkTitles: Record<string, string> | undefined,
  slug: string,
  fallback: string,
): string {
  return linkTitles?.[slug] || fallback
}
