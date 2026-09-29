import type { KBSurfaceKB } from "./permissions.ts";

/** Vue's detail route keeps a disabled Wiki capability on the documents tab.
 * KBL-R1：KB 详情统一 platform 族路径（library 族已在 router 层重定向）。 */
export function wikiEntryPath(knowledgeBaseId: string): string {
  return `/platform/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}`;
}

export function shouldOpenWiki(kb: KBSurfaceKB | null | undefined): boolean {
  return kb?.indexing_strategy?.wiki_enabled === true;
}
