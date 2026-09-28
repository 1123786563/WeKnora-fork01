// W05-family craft Slides view (D03): the workbench-side rendering of a
// slides version's SERVER-CONVERTED preview. The browser never parses the
// deck: the craft-slides skill's controlled pipeline already rendered the
// PPTX to a PDF and exported every page to pages/page-N.svg, and this
// component only projects preview.json — a thumbnail strip, the current
// page image, prev/next and page-number navigation (mouse AND keyboard
// ArrowLeft/ArrowRight), the fixed downloads of report.pptx / report.pdf
// for the selected immutable version, and a modify request that carries
// the EXPLICIT page number plus the current version id to the main agent.
// Props carry data and callbacks only; no global state, no localStorage
// (W05 contract).
import React from 'react';
import { useEffect, useState } from 'react';
import { Button } from './td.tsx';
import { craftStrings, type CraftLocale } from './presentation.ts';

/** Mirrors the skill's preview.json page entry (skills/craft-slides). */
export interface CraftSlidesPageView {
  index: number;
  title: string;
  image: string;
  notes: string;
  sources: string[];
}

/** Mirrors the skill's preview.json document shape. */
export interface CraftSlidesPreviewView {
  kind: 'slides';
  slide_count: number;
  pages: CraftSlidesPageView[];
  overflow_pages: number[];
}

/** The deliverable paths this view downloads (craft.SlidesPPTXPath/PDFPath). */
export const CRAFT_SLIDES_PPTX_PATH = 'report.pptx';
export const CRAFT_SLIDES_PDF_PATH = 'report.pdf';

/** Default page cap (craft.MaxSlidesPages). */
export const CRAFT_SLIDES_MAX_PAGES = 30;

/** Clamp a page selection into [0, pageCount-1]; non-positive counts have page 0. */
export function slidesClampPage(page: number, pageCount: number): number {
  if (pageCount <= 0) return 0;
  return Math.min(Math.max(page, 0), pageCount - 1);
}

// Slides strings ship with the view (RESTORE_STRINGS pattern): the generic
// craft strings stay in presentation.ts.
const SLIDES_STRINGS = {
  zh: {
    pages: '幻灯片',
    pageNumberOf: (current: number, total: number) => `第 ${current}/${total} 页`,
    prevPage: '上一页',
    nextPage: '下一页',
    thumbnails: '页面缩略图',
    downloadPptx: '下载 PPTX',
    downloadPdf: '下载 PDF',
    modifyPage: (page: number) => `修改第 ${page} 页`,
    sources: '本页来源',
    notes: '讲者备注',
    keyboardHint: '← → 键翻页',
    overflow: '大纲超出 30 页上限，溢出页待主 Agent 处理',
    loading: '正在加载演示稿预览…',
    failed: '演示稿预览加载失败',
    empty: '此版本还没有演示稿预览',
    retry: '重新加载预览',
    notRendered: '此页未完成渲染，仅可下载',
  },
  en: {
    pages: 'Slides',
    pageNumberOf: (current: number, total: number) => `Page ${current}/${total}`,
    prevPage: 'Previous page',
    nextPage: 'Next page',
    thumbnails: 'Page thumbnails',
    downloadPptx: 'Download PPTX',
    downloadPdf: 'Download PDF',
    modifyPage: (page: number) => `Modify page ${page}`,
    sources: 'Page sources',
    notes: 'Speaker notes',
    keyboardHint: 'Arrow keys to navigate',
    overflow: 'Outline exceeds the 30-page cap; overflow pages pending main-agent fix',
    loading: 'Loading slides preview…',
    failed: 'Slides preview failed to load',
    empty: 'No slides preview for this version yet',
    retry: 'Reload preview',
    notRendered: 'This page was not rendered; download only',
  },
} as const;

export interface CraftSlidesProps {
  locale: CraftLocale;
  /** The selected immutable version (null when nothing is published yet). */
  versionId: string | null;
  /** The fetched server-converted preview (null until it loads). */
  preview: CraftSlidesPreviewView | null;
  previewLoading: boolean;
  previewError: string | null;
  /** Re-fetches the preview of the selected version — preview only, never re-runs generation. */
  onFetchPreview(versionId: string): void;
  /**
   * Resolves one version-relative preview asset (pages/page-N.svg) into a
   * URL the assembly is authorized to load for THIS version. The asset
   * never loads through a raw path: preview resources stay under the
   * version's authorization (W02).
   */
  onResolvePageImage(versionId: string, path: string): string;
  /** Downloads one immutable member; the assembly owns auth + blob plumbing. */
  onDownload(versionId: string, path: string): void | Promise<void>;
  downloading: boolean;
  /**
   * Requests a modification of ONE explicit page of the CURRENT version —
   * the assembly forwards (page, versionId) to the main agent as the
   * user's change request.
   */
  onRequestChange(page: number, versionId: string): void;
}

export function CraftSlides(props: CraftSlidesProps) {
  const strings = craftStrings(props.locale);
  const s = props.locale === 'zh' ? SLIDES_STRINGS.zh : SLIDES_STRINGS.en;
  const [page, setPage] = useState(0);

  const pageCount = props.preview !== null ? props.preview.pages.length : 0;
  const current = slidesClampPage(page, pageCount);

  // Switching the version resets the page selection: the new preview is a
  // different document and must not inherit a stale position.
  useEffect(() => { setPage(0); }, [props.versionId, props.preview]);

  // Keyboard navigation: ArrowLeft/ArrowRight flip pages while the deck is
  // mounted. The listener is window-scoped (the preview pane may hold
  // focus inside an embedded viewer), and never swallows keys when a form
  // control has focus.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (pageCount <= 0) return;
      const target = event.target as HTMLElement | null;
      if (target !== null && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return;
      if (event.key === 'ArrowLeft') {
        event.preventDefault();
        setPage((now) => slidesClampPage(now - 1, pageCount));
      } else if (event.key === 'ArrowRight') {
        event.preventDefault();
        setPage((now) => slidesClampPage(now + 1, pageCount));
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [pageCount]);

  if (props.versionId === null) {
    return (
      <div className="wk-craft-preview-state" data-testid="craft-slides" data-state="empty" role="status">
        <p>{s.empty}</p>
      </div>
    );
  }
  if (props.preview === null) {
    return (
      <div className="wk-craft-preview-state" data-testid="craft-slides" data-state={props.previewError === null ? 'loading' : 'failed'} role="status">
        <p>{props.previewError !== null ? s.failed + (props.previewError !== '' ? ' — ' + props.previewError : '') : s.loading}</p>
        <Button type="button" onClick={() => props.onFetchPreview(props.versionId ?? '')}>{s.retry}</Button>
      </div>
    );
  }

  const entry = props.preview.pages[current] ?? null;
  const overflow = props.preview.overflow_pages;

  return (
    <div className="wk-craft-panel-body" data-testid="craft-slides" data-state="active">
      {overflow.length > 0 ? (
        <p className="wk-craft-hint" data-testid="craft-slides-overflow" role="alert">
          {s.overflow}（{overflow.join('、')}）
        </p>
      ) : null}
      <div className="wk-craft-actions">
        <Button
          type="button"
          data-testid="craft-slides-download-pptx"
          disabled={props.downloading}
          onClick={() => props.onDownload(props.versionId ?? '', CRAFT_SLIDES_PPTX_PATH)}
        >
          {props.downloading ? strings.craftDownloading : s.downloadPptx}
        </Button>
        <Button
          type="button"
          data-testid="craft-slides-download-pdf"
          disabled={props.downloading}
          onClick={() => props.onDownload(props.versionId ?? '', CRAFT_SLIDES_PDF_PATH)}
        >
          {props.downloading ? strings.craftDownloading : s.downloadPdf}
        </Button>
        {entry !== null ? (
          <Button
            type="button"
            data-testid="craft-slides-modify"
            data-page={entry.index}
            onClick={() => props.onRequestChange(entry.index, props.versionId ?? '')}
          >
            {s.modifyPage(entry.index)}
          </Button>
        ) : null}
      </div>
      {entry !== null ? (
        <>
          <figure className="wk-craft-slide-stage" data-testid="craft-slides-stage">
            <img
              className="wk-craft-slide-page"
              data-testid="craft-slides-page-image"
              src={props.onResolvePageImage(props.versionId ?? '', entry.image)}
              alt={entry.title}
            />
            <figcaption className="wk-craft-hint">
              {entry.title}
              {entry.sources.length > 0 ? ` · ${s.sources}: ${entry.sources.join(' ')}` : ''}
            </figcaption>
          </figure>
          {entry.notes !== '' ? (
            <p className="wk-craft-hint" data-testid="craft-slides-notes">{s.notes}: {entry.notes}</p>
          ) : null}
        </>
      ) : (
        <p className="wk-craft-hint">{s.notRendered}</p>
      )}
      <div className="wk-craft-thumbnails" role="listbox" aria-label={s.thumbnails} data-testid="craft-slides-thumbnails">
        {props.preview.pages.map((p, index) => (
          <button
            key={p.index}
            type="button"
            role="option"
            className="wk-craft-thumbnail"
            aria-selected={index === current}
            data-testid="craft-slides-thumb"
            data-page={p.index}
            onClick={() => setPage(slidesClampPage(index, pageCount))}
          >
            <img src={props.onResolvePageImage(props.versionId ?? '', p.image)} alt={`${p.index}. ${p.title}`} />
            <span>{p.index}</span>
          </button>
        ))}
      </div>
      {pageCount > 1 ? (
        <div className="wk-craft-actions">
          <Button type="button" disabled={current === 0} onClick={() => setPage((now) => slidesClampPage(now - 1, pageCount))}>{s.prevPage}</Button>
          <span className="wk-craft-hint" data-testid="craft-slides-page-number">{s.pageNumberOf(current + 1, pageCount)}</span>
          <Button type="button" disabled={current >= pageCount - 1} onClick={() => setPage((now) => slidesClampPage(now + 1, pageCount))}>{s.nextPage}</Button>
          <span className="wk-craft-hint">{s.keyboardHint}</span>
        </div>
      ) : null}
    </div>
  );
}
