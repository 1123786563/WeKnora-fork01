import type { KBSurfaceKB } from "./permissions.ts";

/** Vue's detail route keeps a disabled Wiki capability on the documents tab. */
export function wikiEntryPath(knowledgeBaseId: string): string {
  return `/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}`;
}

export function shouldOpenWiki(kb: KBSurfaceKB | null | undefined): boolean {
  return kb?.indexing_strategy?.wiki_enabled === true;
}
