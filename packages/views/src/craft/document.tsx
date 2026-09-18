// W05-family craft Document view (D01): the workbench-side rendering of a
// document version's EDITABLE Markdown source with its same-run DOCX export.
// The browser never executes document content: report.md is projected by a
// SAFE Markdown renderer whose only output is React elements built from
// PLAIN strings (React escapes every byte; raw HTML lines are dropped, never
// parsed, never injected), and the DOCX is downloaded as an ATTACHMENT
// through the immutable-version files route — the browser must not open or
// execute embedded DOCX content. Citations in the text (C01 kc_ ids) render
// as clickable buttons that resolve through the knowledge permission chain
// on every click. Props carry data and callbacks only; no global state, no
// localStorage (W05 contract).
import React from 'react';
import { useMemo } from 'react';
import { Button } from '@weknora/ui';
import { craftStrings, type CraftLocale } from './presentation.ts';

/** The editable source path this view renders (craft.DocumentMarkdownPath). */
export const CRAFT_DOCUMENT_MD_PATH = 'report.md';
/** The export path this view downloads as an attachment (craft.DocumentDOCXPath). */
export const CRAFT_DOCUMENT_DOCX_PATH = 'report.docx';

/** A citation id is exactly a C01 knowledge id: kc_ + 24 lowercase hex. */
export const DOCUMENT_CITATION_PATTERN = /kc_[0-9a-f]{24}/g;

/** One inline segment of a rendered line: plain text or a clickable citation. */
export type CraftDocumentSegment = { kind: 'text'; text: string } | { kind: 'citation'; id: string };

/** One block of the safe Markdown projection. Only these shapes exist —
 * there is deliberately no html block, so nothing can smuggle markup. */
/** One table cell = the cell's inline segments; a row is its cells. */
export type CraftDocumentTableCell = CraftDocumentSegment[];
export type CraftDocumentTableRow = CraftDocumentTableCell[];

export type CraftDocumentBlock =
  | { type: 'h1' | 'h2' | 'h3' | 'p'; segments: CraftDocumentSegment[] }
  | { type: 'li'; segments: CraftDocumentSegment[] }
  | { type: 'table'; rows: CraftDocumentTableRow[] };

export interface CraftDocumentProjection {
  blocks: CraftDocumentBlock[];
  citations: string[];
  htmlDropped: number;
}

/** Strips inline Markdown emphasis/links to PLAIN text. The URL of a link
 * is dropped (never becomes an anchor): the document's only clickable
 * affordances are the kc_ citations, which route through the knowledge
 * permission chain, not through model-supplied URLs. */
export function documentInlineText(raw: string): string {
  return raw
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/\*([^*]+)\*/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/~~([^~]+)~~/g, '$1');
}

/** Splits one line of plain text into text/citation segments. */
export function documentSegments(text: string): CraftDocumentSegment[] {
  const segments: CraftDocumentSegment[] = [];
  let last = 0;
  for (const match of text.matchAll(DOCUMENT_CITATION_PATTERN)) {
    const at = match.index ?? 0;
    if (at > last) segments.push({ kind: 'text', text: text.slice(last, at) });
    segments.push({ kind: 'citation', id: match[0] });
    last = at + match[0].length;
  }
  if (last < text.length) segments.push({ kind: 'text', text: text.slice(last) });
  return segments;
}

const tableSeparator = /^\|?\s*:?-{2,}/;

/** Projects the Markdown source into safe render blocks. Line-based, no
 * HTML parsing: a line whose trimmed content starts with '<' is raw HTML
 * and is DROPPED (counted in htmlDropped) — the view never renders it. */
export function parseDocumentMarkdown(md: string): CraftDocumentProjection {
  const blocks: CraftDocumentBlock[] = [];
  const citations: string[] = [];
  const seen = new Set<string>();
  let htmlDropped = 0;
  let pendingTable: CraftDocumentTableRow[] | null = null;
  const flushTable = (): void => {
    if (pendingTable !== null && pendingTable.length > 0) blocks.push({ type: 'table', rows: pendingTable });
    pendingTable = null;
  };
  for (const rawLine of md.split('\n')) {
    const line = rawLine.trimEnd();
    const trimmed = line.trim();
    if (trimmed === '') { flushTable(); continue; }
    if (trimmed.startsWith('<')) { htmlDropped += 1; flushTable(); continue; }
    if (trimmed.startsWith('# ') || trimmed.startsWith('## ') || trimmed.startsWith('### ')) {
      flushTable();
      const level = trimmed.startsWith('### ') ? 'h3' : trimmed.startsWith('## ') ? 'h2' : 'h1';
      blocks.push({ type: level, segments: documentSegments(documentInlineText(trimmed.replace(/^#+\s*/, ''))) });
      continue;
    }
    if (trimmed.startsWith('|')) {
      if (tableSeparator.test(trimmed)) continue;
      const cells = trimmed.replace(/^\|/, '').replace(/\|$/, '').split('|').map((cell) => documentSegments(documentInlineText(cell.trim())));
      pendingTable = pendingTable ?? [];
      pendingTable.push(cells);
      continue;
    }
    flushTable();
    if (trimmed.startsWith('- ') || trimmed.startsWith('* ')) {
      blocks.push({ type: 'li', segments: documentSegments(documentInlineText(trimmed.slice(2))) });
      continue;
    }
    blocks.push({ type: 'p', segments: documentSegments(documentInlineText(trimmed)) });
  }
  flushTable();
  for (const match of md.matchAll(DOCUMENT_CITATION_PATTERN)) {
    if (!seen.has(match[0])) { seen.add(match[0]); citations.push(match[0]); }
  }
  return { blocks, citations, htmlDropped };
}

// Document strings ship with the view (RESTORE_STRINGS pattern): the
// generic craft strings stay in presentation.ts.
const DOCUMENT_STRINGS = {
  zh: {
    download: '下载 DOCX',
    source: '可编辑源文件',
    citations: '引用来源',
    citationOpen: '查看来源',
    citationMissing: '引用来源不可见（无权限或已删除）',
    htmlDropped: '已忽略 N 行原始 HTML（安全渲染，仅纯文本）',
    loading: '正在加载文档…',
    failed: '文档加载失败',
    empty: '此版本还没有文档',
    retry: '重新加载文档',
  },
  en: {
    download: 'Download DOCX',
    source: 'Editable source',
    citations: 'Cited sources',
    citationOpen: 'Open source',
    citationMissing: 'Cited source is not visible (no permission or deleted)',
    htmlDropped: 'Dropped N raw-HTML line(s) (safe renderer, plain text only)',
    loading: 'Loading document…',
    failed: 'Document failed to load',
    empty: 'No document for this version yet',
    retry: 'Reload document',
  },
} as const;

export interface CraftDocumentProps {
  locale: CraftLocale;
  /** The selected immutable version (null when nothing is published yet). */
  versionId: string | null;
  /** The fetched report.md source (null until it loads). */
  markdown: string | null;
  markdownLoading: boolean;
  markdownError: string | null;
  /** The manifest's citation ids (used for the sources panel order). */
  manifestCitations: string[];
  /** Re-fetches the document of the selected version — preview only, never re-runs generation. */
  onFetchDocument(versionId: string): void;
  /** Downloads one immutable member AS AN ATTACHMENT; the assembly owns auth + blob plumbing. */
  onDownload(versionId: string, path: string): void | Promise<void>;
  downloading: boolean;
  /** Opens one citation id through the knowledge permission chain (never a cached URL). */
  onOpenCitation(citationId: string): void;
}

export function CraftDocument(props: CraftDocumentProps) {
  const strings = craftStrings(props.locale);
  const s = props.locale === 'zh' ? DOCUMENT_STRINGS.zh : DOCUMENT_STRINGS.en;
  const projection = useMemo(
    () => (props.markdown !== null ? parseDocumentMarkdown(props.markdown) : null),
    [props.markdown],
  );
  const citations = useMemo(() => {
    if (projection === null) return [];
    const ordered = props.manifestCitations.filter((id) => projection.citations.includes(id));
    for (const id of projection.citations) if (!ordered.includes(id)) ordered.push(id);
    return ordered;
  }, [projection, props.manifestCitations]);

  const openCitation = (citationId: string): void => {
    props.onOpenCitation(citationId);
  };

  if (props.versionId === null) {
    return (
      <div className="wk-craft-preview-state" data-testid="craft-document" data-state="empty" role="status">
        <p>{s.empty}</p>
      </div>
    );
  }
  if (projection === null) {
    return (
      <div className="wk-craft-preview-state" data-testid="craft-document" data-state={props.markdownError === null ? 'loading' : 'failed'} role="status">
        <p>{props.markdownError !== null ? s.failed + (props.markdownError !== '' ? ' — ' + props.markdownError : '') : s.loading}</p>
        <Button type="button" onClick={() => props.onFetchDocument(props.versionId ?? '')}>{s.retry}</Button>
      </div>
    );
  }

  const renderSegments = (segments: CraftDocumentSegment[], keyPrefix: string): React.JSX.Element[] =>
    segments.map((segment, index) => {
      const key = keyPrefix + '-' + index;
      if (segment.kind === 'citation') {
        return (
          <button
            key={key}
            type="button"
            className="wk-craft-citation"
            data-testid="craft-document-citation"
            data-citation={segment.id}
            title={s.citationOpen}
            onClick={() => openCitation(segment.id)}
          >
            {segment.id}
          </button>
        );
      }
      return <span key={key}>{segment.text}</span>;
    });

  return (
    <div className="wk-craft-panel-body" data-testid="craft-document" data-state="active">
      <div className="wk-craft-actions">
        <Button
          type="button"
          data-testid="craft-document-download"
          disabled={props.downloading}
          onClick={() => props.onDownload(props.versionId ?? '', CRAFT_DOCUMENT_DOCX_PATH)}
        >
          {props.downloading ? strings.craftDownloading : s.download}
        </Button>
        <span className="wk-craft-hint">{s.source}: {CRAFT_DOCUMENT_MD_PATH} · {CRAFT_DOCUMENT_DOCX_PATH}</span>
      </div>
      {projection.htmlDropped > 0 ? (
        <p className="wk-craft-hint" data-testid="craft-document-html-dropped">
          {s.htmlDropped.replace('N', String(projection.htmlDropped))}
        </p>
      ) : null}
      <article className="wk-craft-document" data-testid="craft-document-body">
        {(() => {
          // Consecutive list blocks render inside ONE ul; every other block
          // renders on its own. Only these element types ever exist.
          const out: React.JSX.Element[] = [];
          let list: React.JSX.Element[] = [];
          const flush = (): void => {
            if (list.length > 0) {
              out.push(<ul key={'ul-' + out.length} className="wk-craft-doc-list">{list}</ul>);
              list = [];
            }
          };
          projection.blocks.forEach((block, index) => {
            const key = 'block-' + index;
            switch (block.type) {
              case 'li':
                list.push(<li key={key}>{renderSegments(block.segments, key)}</li>);
                return;
              case 'h1':
                flush();
                out.push(<h2 key={key}>{renderSegments(block.segments, key)}</h2>);
                return;
              case 'h2':
                flush();
                out.push(<h3 key={key} data-testid="craft-document-heading">{renderSegments(block.segments, key)}</h3>);
                return;
              case 'h3':
                flush();
                out.push(<h4 key={key} data-testid="craft-document-heading">{renderSegments(block.segments, key)}</h4>);
                return;
              case 'p':
                flush();
                out.push(<p key={key}>{renderSegments(block.segments, key)}</p>);
                return;
              case 'table':
                flush();
                out.push(
                  <div key={key} className="wk-craft-table-wrap">
                    <table className="wk-craft-table" data-testid="craft-document-table">
                      <tbody>
                        {block.rows.map((row, rowIndex) => (
                          <tr key={rowIndex}>
                            {row.map((cell, columnIndex) => (
                              <td key={columnIndex}>{renderSegments(cell, key + '-' + rowIndex + '-' + columnIndex)}</td>
                            ))}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>,
                );
                return;
            }
          });
          flush();
          return out;
        })()}
      </article>
      {citations.length > 0 ? (
        <div className="wk-craft-card" data-testid="craft-document-sources">
          <h3>{s.citations}</h3>
          <ul className="wk-craft-list">
            {citations.map((id) => (
              <li key={id}>
                <button type="button" className="wk-craft-citation" data-testid="craft-document-source" data-citation={id} onClick={() => openCitation(id)}>
                  {id}
                </button>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}