// C01 craft Sources: the knowledge material provenance panel.
//
// The panel lists the bounded material package the run was built from: one
// row per source with its stable citation ID, the durable source ref, the
// excerpt digest and size. Clicking a citation NEVER follows a stored URL:
// the component only knows the durable ref and hands it to the assembly,
// which re-resolves it through the EXISTING resource permission chain on
// every click — a share revoked after the run immediately yields the
// permission error instead of replaying a cached link.
import React from 'react';
import { Button } from './td.tsx';
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
}

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
  };
}

export function CraftSources(props: CraftSourcesProps) {
  const base = craftStrings(props.locale);
  const labels = sourceLabels(props.locale);

  return (
    <section className="wk-craft-panel-body" data-testid="craft-sources" aria-label={labels.heading}>
      <h3>{labels.heading}</h3>
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
              <th scope="col">{base.craftOpen}</th>
            </tr>
          </thead>
          <tbody>
            {props.sources.map((source) => {
              const revoked = props.revokedCitationIds?.includes(source.citationId) ?? false;
              return (
                <tr key={source.citationId} data-testid="craft-source-item" data-revoked={revoked}>
                  <td>
                    <code>{source.citationId}</code>
                    <div>{revoked ? labels.revoked : source.title}</div>
                  </td>
                  <td>{revoked ? <span className="wk-craft-muted">{labels.revoked}</span> : <code>{source.digest.slice(0, 12)}…</code>}</td>
                  <td>{revoked ? '—' : formatBytes(source.excerptBytes)}</td>
                  <td>
                    {source.tenantId}
                  </td>
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
    </section>
  );
}
