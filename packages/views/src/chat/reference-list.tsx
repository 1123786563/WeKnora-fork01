import { groupChatReferences, type ChatReferenceGroup, type ChatReferenceItem } from '@weknora/domain/chat/references';

export interface ReferenceListProps {
  references: readonly unknown[];
  activeId?: string | null;
  onActivate?(referenceId: string): void;
}

export interface ReferenceSection extends ChatReferenceGroup {
  id: ChatReferenceGroup['kind'];
}

const SECTION_TITLES: Readonly<Record<ReferenceSection['id'], string>> = Object.freeze({
  web: 'Web sources',
  document: 'Documents',
  tool: 'Tool results',
});

export function referenceSections(references: readonly unknown[] | null | undefined): ReferenceSection[] {
  return groupChatReferences(references).map((group) => ({ ...group, id: group.kind }));
}

function idsFor(item: ChatReferenceItem): string[] {
  return item.chunkIds?.length ? [...item.chunkIds] : item.chunkId ? [item.chunkId] : [];
}

function ReferenceItem({ item, activeId, onActivate }: { item: ChatReferenceItem; activeId?: string | null; onActivate?: (referenceId: string) => void }) {
  const ids = idsFor(item);
  const active = Boolean(activeId && ids.includes(activeId));
  const summary = item.kind === 'document' ? item.content ?? item.snippet : item.snippet ?? item.content;
  const activate = (id: string) => onActivate?.(id);

  return <article className={`wk-chat-reference wk-chat-reference--${item.kind}${active ? ' is-active' : ''}`}>
    {item.kind === 'web' && item.url ? <a href={item.url} target="_blank" rel="noopener noreferrer" className="wk-chat-reference-title">{item.title}</a> : <strong className="wk-chat-reference-title">{item.title}</strong>}
    {item.domain ? <small>{item.domain}</small> : null}
    {summary ? <p>{summary}</p> : null}
    {ids.length > 0 && item.kind !== 'web' ? <div className="wk-chat-reference-ids" aria-label="Reference chunks">{ids.map((id) => <button key={id} type="button" data-reference-id={id} aria-current={activeId === id ? 'true' : undefined} onClick={() => activate(id)}>{id}</button>)}</div> : null}
  </article>;
}

export function ReferenceList({ references, activeId = null, onActivate }: ReferenceListProps) {
  const sections = referenceSections(references);
  if (sections.length === 0) return null;
  return <aside className="wk-chat-references" aria-label="Chat references">
    <h2>References</h2>
    {sections.map((section) => <section key={section.id} aria-label={SECTION_TITLES[section.id]}>
      <h3>{SECTION_TITLES[section.id]}</h3>
      <div>{section.items.map((item) => <ReferenceItem key={item.key} item={item} activeId={activeId} onActivate={onActivate} />)}</div>
    </section>)}
  </aside>;
}
