export function canCreateKnowledgeBase(role: string | undefined): boolean {
  return role === "owner" || role === "admin" || role === "contributor";
}

export function knowledgeBaseCountLabel(item: {
  type?: unknown;
  knowledge_count?: unknown;
  chunk_count?: unknown;
}): string {
  const kind = item.type === "faq" ? "FAQ" : "Documents";
  const count = Number(
    item.type === "faq" ? item.chunk_count : item.knowledge_count || 0,
  );
  return `${kind} · ${count} items`;
}
