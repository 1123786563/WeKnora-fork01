import { formatMessage, type Locale } from '@weknora/i18n';

export function canCreateKnowledgeBase(role: string | undefined): boolean {
  return role === "owner" || role === "admin" || role === "contributor";
}

export function knowledgeListLabel(locale: Locale, key: string, values: Record<string, string | number> = {}): string {
  return formatMessage(locale, key, values);
}

export function knowledgeBaseCountLabel(item: {
  type?: unknown;
  knowledge_count?: unknown;
  chunk_count?: unknown;
}, locale: Locale = 'en-US'): string {
  const kind = item.type === "faq"
    ? knowledgeListLabel(locale, 'common.typeFaq')
    : locale === 'en-US' ? 'Documents' : knowledgeListLabel(locale, 'common.typeDocument');
  const count = Number(
    item.type === "faq" ? item.chunk_count : item.knowledge_count || 0,
  );
  return `${kind} · ${knowledgeListLabel(locale, 'common.itemCount', { count })}`;
}
