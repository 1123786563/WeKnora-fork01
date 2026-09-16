import { groupChatReferences, type ChatReferenceGroup, type ChatReferenceItem } from '@weknora/domain/chat/references';
import { resolveChatCopy, type ChatCopyTable } from './chat-copy.ts';

export interface ReferenceListProps {
  references: readonly unknown[];
  activeId?: string | null;
  onActivate?(referenceId: string): void;
  copy?: ChatCopyTable;
}

export interface ReferenceSection extends ChatReferenceGroup {
  id: ChatReferenceGroup['kind'];
}

export function referenceSections(references: readonly unknown[] | null | undefined): ReferenceSection[] {
  return groupChatReferences(references).map((group) => ({ ...group, id: group.kind }));
}

function idsFor(item: ChatReferenceItem): string[] {
  return item.chunkIds?.length ? [...item.chunkIds] : item.chunkId ? [item.chunkId] : [];
}

function ReferenceItem({ item, activeId, onActivate, copy }: { item: ChatReferenceItem; activeId?: string | null; onActivate?: (referenceId: string) => void; copy: ChatCopyTable }) {
  const ids = idsFor(item);
  const active = Boolean(activeId && ids.includes(activeId));
  // Keep the panel bounded for large retrieved chunks; the shared model's
  // snippet is intentionally capped while the full content stays available to
  // a future explicit preview action.
  const summary = item.snippet ?? item.content;
  const activate = (id: string) => onActivate?.(id);

  return <article className={`wk-chat-reference wk-chat-reference--${item.kind}${active ? ' is-active' : ''}`}>
    {item.kind === 'web' && item.url ? <a href={item.url} target="_blank" rel="noopener noreferrer" className="wk-chat-reference-title">{item.title}</a> : <strong className="wk-chat-reference-title">{item.title}</strong>}
    {item.domain ? <small>{item.domain}</small> : null}
    {summary ? <p>{summary}</p> : null}
    {ids.length > 0 && item.kind !== 'web' ? <div className="wk-chat-reference-ids" aria-label={copy.referenceChunks}>{ids.map((id) => <button key={id} type="button" data-reference-id={id} aria-current={activeId === id ? 'true' : undefined} onClick={() => activate(id)}>{id}</button>)}</div> : null}
  </article>;
}

export function ReferenceList({ references, activeId = null, onActivate, copy = resolveChatCopy() }: ReferenceListProps) {
  const sections = referenceSections(references);
  if (sections.length === 0) return null;
  const sectionTitles: Record<ReferenceSection['id'], string> = { web: copy.referenceWebSources, document: copy.referenceDocuments, tool: copy.referenceToolResults };
  return <aside className="wk-chat-references" aria-label={copy.referencesTitle}>
    <h2>{copy.referencesTitle}</h2>
    {sections.map((section) => <section key={section.id} aria-label={sectionTitles[section.id]}>
      <h3>{sectionTitles[section.id]}</h3>
      <div>{section.items.map((item) => <ReferenceItem key={item.key} item={item} activeId={activeId} onActivate={onActivate} copy={copy} />)}</div>
    </section>)}
  </aside>;
}
