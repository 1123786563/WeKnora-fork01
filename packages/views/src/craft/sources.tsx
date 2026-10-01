// C01 craft Sources: the knowledge material provenance panel.
//
// The panel lists the bounded material package the run was built from: one
// row per source with its stable citation ID, the durable source ref, the
// excerpt digest and size. Clicking a citation NEVER follows a stored URL:
// the component only knows the durable ref and hands it to the assembly,
// which re-resolves it through the EXISTING resource permission chain on
// every click — a share revoked after the run immediately yields the
// permission error instead of replaying a cached link.
import React, { useMemo } from 'react';
import { Button } from 'tdesign-react';
import { craftStrings, formatBytes, type CraftLocale } from './presentation.ts';

/** One knowledge source row, mirroring the backend material manifest. */
export interface CraftSourceRow {
  /** Stable citation ID (kc_…) that delivered artifacts cite. */
  citationId: string;
  /** Durable, non-expiring source ref (craftkb://kb/…/chunk/…). */
  ref: string;
  /** Document title for display. */
  title: string;
  /** SHA-256 of the staged excerpt. */
  digest: string;
  /** Excerpt size in bytes. */
  excerptBytes: number;
  /** Owning library tenant (differs from the caller's tenant for shared libraries). */
  tenantId: number;
  /** Observation time of the material actually handed to this Run. */
  acquiredAt?: string;
}

export interface CraftKnowledgeBaseChoice {
  id: string;
  name: string;
}

/**
 * One citation entry of the delivered web artifact's citation manifest
 * (T06/#125). A fact carries the stable citation id of a source the Run
 * actually recorded; model inference never carries one and is never
 * presented as a source fact.
 */
export interface CraftCitationFact {
  kind: 'fact';
  citationId: string;
  claim: string;
}
export interface CraftCitationInference {
  kind: 'inference';
  claim: string;
}
export type CraftCitationEntry = CraftCitationFact | CraftCitationInference;

export interface CraftSourcesProps {
  locale: CraftLocale;
  /** The bounded source list of the run's knowledge package. */
  sources: CraftSourceRow[];
  /** Whether retrieval produced more than the caps kept. */
  truncated: boolean;
  /** Citation currently being resolved (for pending state only). */
  openingCitation: string | null;
  /**
   * Citations whose backing share was revoked (CFT-S01-T011): the row keeps
   * only the citable placeholder — no cached excerpt, no title replay.
   */
  revokedCitationIds?: readonly string[];
  /**
   * The delivered web artifact's citation manifest (T06/#125). Facts link to
   * their recorded source rows through the same durable-ref open seam;
   * inference entries carry the explicit distinct marker and open nothing.
   * Missing/revoked sources keep a non-leaking placeholder.
   */
  citations?: readonly CraftCitationEntry[];
  /** Server-projected libraries available to this viewer for a new Run. */
  knowledgeBases?: readonly CraftKnowledgeBaseChoice[];
  selectedKnowledgeBaseIds?: readonly string[];
  onSelectionChange?(ids: string[]): void;
  /**
   * Opens one source by its durable ref. The assembly resolves it through
   * the existing resource permission chain on every click; it must never
   * cache or pre-sign a URL into this component.
   */
  onOpenSource(citationId: string, ref: string): void;
}

interface SourceLabels {
  heading: string;
  empty: string;
  citation: string;
  digest: string;
  excerptBytes: string;
  tenant: string;
  open: string;
  opening: string;
  truncated: string;
  shared: string;
  revoked: string;
  revokedTitle: string;
  acquired: string;
  select: string;
  citationsHeading: string;
  factLabel: string;
  inferenceLabel: string;
  unavailable: string;
  unavailableTitle: string;
}

function sourceLabels(locale: CraftLocale): SourceLabels {
  if (locale === 'zh') {
    return {
      heading: '知识来源',
      empty: '本次运行没有知识材料。',
      citation: '引用',
      digest: '摘要指纹',
      excerptBytes: '摘录',
      tenant: '所属空间',
      open: '查看来源',
      opening: '打开中…',
      truncated: '检索命中超出材料上限，仅保留前 20 条 / 64KiB。',
      shared: '共享库',
      revoked: '已撤权',
      revokedTitle: '该来源的共享授权已被撤回，引用占位保留，原文不可再打开。',
      acquired: '获取时间',
      select: '选择知识库',
      citationsHeading: '引用与推断',
      factLabel: '[事实来源]',
      inferenceLabel: '[模型推断]',
      unavailable: '来源不可用',
      unavailableTitle: '该引用对应的来源缺失或已不可访问，占位保留，不泄露原文。',
    };
  }
  return {
    heading: 'Knowledge sources',
    empty: 'This run has no knowledge material.',
    citation: 'Citation',
    digest: 'Excerpt digest',
    excerptBytes: 'Excerpt',
    tenant: 'Owning tenant',
    open: 'Open source',
    opening: 'Opening…',
    truncated: 'Retrieval exceeded the material caps; only the first 20 sources / 64 KiB are kept.',
    shared: 'Shared library',
    revoked: 'Revoked',
    revokedTitle: 'The backing share was revoked; the citation placeholder stays, the excerpt cannot reopen.',
    acquired: 'Acquired',
    select: 'Select knowledge bases',
    citationsHeading: 'Citations and inference',
    factLabel: '[Source fact]',
    inferenceLabel: '[Model inference]',
    unavailable: 'Source unavailable',
    unavailableTitle: 'The cited source is missing or inaccessible; the placeholder stays without leaking the original.',
  };
}

// isCraftCitationFact narrows the citation union without a silent cast.
// NOTE: the parameter is deliberately typed { kind: string } so this is a
// RUNTIME predicate, not a compile-time guarantee — new kinds stay excluded
// from citedIds here; keep the RENDER branch (below) on the same predicate
// when adding kinds, or the two sites drift.
function isCraftCitationFact(entry: { kind: string }): entry is CraftCitationFact {
  return entry.kind === 'fact';
}

export function CraftSources(props: CraftSourcesProps) {
  const base = craftStrings(props.locale);
  const labels = sourceLabels(props.locale);
  const sourcesByCitation = useMemo(() => {
    const map = new Map(props.sources.map((source) => [source.citationId, source]));
    return map;
  }, [props.sources]);
  const citedIds = useMemo(
    () => new Set<string>((props.citations ?? []).filter(isCraftCitationFact).map((entry) => entry.citationId)),
    [props.citations],
  );

  return (
    <section className="wk-craft-panel-body" data-testid="craft-sources" aria-label={labels.heading}>
      <h3>{labels.heading}</h3>
      {props.knowledgeBases ? (
        <fieldset aria-label={labels.select}>
          <legend>{labels.select}</legend>
          {props.knowledgeBases.map((base) => (
            <label key={base.id}>
              <input type="checkbox" checked={props.selectedKnowledgeBaseIds?.includes(base.id) ?? false}
                disabled={!props.onSelectionChange}
                onChange={(event) => {
                  const selected = new Set(props.selectedKnowledgeBaseIds ?? []);
                  if (event.target.checked) selected.add(base.id); else selected.delete(base.id);
                  props.onSelectionChange?.([...selected]);
                }} />
              {base.name}
            </label>
          ))}
        </fieldset>
      ) : null}
      {props.sources.length === 0 ? (
        <p className="wk-craft-muted" data-testid="craft-sources-empty">{labels.empty}</p>
      ) : (
        <table className="wk-craft-table">
          <thead>
            <tr>
              <th scope="col">{labels.citation}</th>
              <th scope="col">{labels.digest}</th>
              <th scope="col">{labels.excerptBytes}</th>
              <th scope="col">{labels.tenant}</th>
              <th scope="col">{labels.acquired}</th>
              <th scope="col">{base.craftOpen}</th>
            </tr>
          </thead>
          <tbody>
            {props.sources.map((source) => {
              const revoked = props.revokedCitationIds?.includes(source.citationId) ?? false;
              const cited = citedIds.has(source.citationId);
              return (
                <tr key={source.citationId} data-testid="craft-source-item" data-revoked={revoked} data-cited={cited}>
                  <td>
                    <code>{source.citationId}</code>
                    <div>{revoked ? labels.revoked : source.title}</div>
                  </td>
                  <td>{revoked ? <span className="wk-craft-muted">{labels.revoked}</span> : <code>{source.digest.slice(0, 12)}…</code>}</td>
                  <td>{revoked ? '—' : formatBytes(source.excerptBytes)}</td>
                  <td>
                    {source.tenantId}
                  </td>
                  <td>{revoked ? '—' : source.acquiredAt ? <time dateTime={source.acquiredAt}>{source.acquiredAt}</time> : '—'}</td>
                  <td>
                    {revoked ? (
                      <Button type="button" disabled title={labels.revokedTitle}>{labels.revoked}</Button>
                    ) : (
                      <Button
                        type="button"
                        disabled={props.openingCitation === source.citationId}
                        onClick={() => props.onOpenSource(source.citationId, source.ref)}
                      >
                        {props.openingCitation === source.citationId ? labels.opening : labels.open}
                      </Button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
      {props.truncated ? (
        <p className="wk-craft-muted" data-testid="craft-sources-truncated" role="status">{labels.truncated}</p>
      ) : null}
      {props.citations && props.citations.length > 0 ? (
        <div className="wk-craft-citations" data-testid="craft-citations">
          <h4>{labels.citationsHeading}</h4>
          <ul className="wk-craft-citation-list">
            {props.citations.map((entry, index) => {
              if (!isCraftCitationFact(entry)) {
                return (
                  <li key={`inference-${index}`} className="wk-craft-citation-inference" data-craft-inference="true">
                    <span className="wk-craft-inference-label">{labels.inferenceLabel}</span> {entry.claim}
                  </li>
                );
              }
              const row = sourcesByCitation.get(entry.citationId);
              const revoked = props.revokedCitationIds?.includes(entry.citationId) ?? false;
              const openable = row !== undefined && !revoked;
              return (
                <li key={`fact-${entry.citationId}-${index}`} className="wk-craft-citation-fact" data-craft-citation={entry.citationId}>
                  <span className="wk-craft-fact-label">{labels.factLabel}</span> {entry.claim}{' '}
                  {openable && row ? (
                    <Button
                      type="button"
                      data-craft-citation-open={entry.citationId}
                      disabled={props.openingCitation === entry.citationId}
                      onClick={() => props.onOpenSource(entry.citationId, row.ref)}
                    >
                      {props.openingCitation === entry.citationId ? labels.opening : labels.open}
                    </Button>
                  ) : (
                    <span className="wk-craft-muted" data-craft-citation-unavailable={entry.citationId} title={labels.unavailableTitle}>
                      {labels.unavailable}
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}
    </section>
  );
}
