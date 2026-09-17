// ─── R457 A3: host wiring helper for the Vue `open-source-doc` emit ───
//
// WikiBrowser.vue:607 clicks emit `open-source-doc` with the source ref's
// knowledge id; KnowledgeBase.vue:1431 `openSourceDoc(knowledgeId)` opens that
// document for viewing. The React host has no card-details drawer on the wiki
// surface — its established "view a document" affordance is the
// document-detail route (`kind: 'knowledge-document'`, served by
// KnowledgeDocumentDetailPage), the same target the documents surface's
// `onOpenDocument` navigates to.
//
// The WikiPage mount point (main.tsx `WikiEntry`) is owned elsewhere, so the
// host wires the prop in one line:
//
//   <WikiPage
//     ...
//     onOpenSourceDoc={createWikiSourceDocOpener({ knowledgeBaseId, navigate })}
//   />
//
// Without the prop the footer click stays a no-op, matching the Vue emit
// without a listener.

/** The host document-detail route for one knowledge/document id. */
export function wikiSourceDocPath(knowledgeBaseId: string, documentId: string): string {
  return `/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}/documents/${encodeURIComponent(documentId)}`;
}

/**
 * Vue `openSourceDoc` port: build the `onOpenSourceDoc` handler that opens a
 * clicked wiki source document in the host's document-detail route.
 */
export function createWikiSourceDocOpener({ knowledgeBaseId, navigate }: {
  knowledgeBaseId: string;
  navigate: (path: string) => void;
}): (documentId: string) => void {
  return (documentId: string) => {
    navigate(wikiSourceDocPath(knowledgeBaseId, documentId));
  };
}
