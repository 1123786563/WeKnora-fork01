// W05-family craft Spreadsheet view (D02): the workbench-side rendering of
// a spreadsheet version's SERVER-CONVERTED preview. The browser never parses
// the workbook: the skill's controlled converter already produced
// preview.json from the RECALCULATED workbook, and this component only
// projects that JSON — sheet tabs, per-sheet dimensions, the computed
// total, paginated rows (at most 1000 per page) and the fixed download
// of report.xlsx for the selected immutable version. Props carry data and
// callbacks only; no global state, no localStorage (W05 contract).
import React from 'react';
import { useEffect, useMemo, useState } from 'react';
import { Button } from './td.tsx';
import { craftStrings, type CraftLocale } from './presentation.ts';

/** Mirrors the skill's preview.json sheet entry (skills/craft-spreadsheet). */
export interface CraftSpreadsheetCellView {
  text: string;
  type: 'number' | 'date' | 'currency' | 'text' | 'empty';
}

export interface CraftSpreadsheetSheetView {
  name: string;
  rows: number;
  columns: number;
  total: string;
  cells: CraftSpreadsheetCellView[][];
}

/** Mirrors the skill's preview.json document shape. */
export interface CraftSpreadsheetPreviewView {
  kind: 'spreadsheet';
  sheets: CraftSpreadsheetSheetView[];
  page_rows: number;
  page_count: number;
}

/** The deliverable path this view downloads (craft.SpreadsheetXLSXPath). */
export const CRAFT_SPREADSHEET_XLSX_PATH = 'report.xlsx';

/** Preview rows per page hard cap (craft.MaxSpreadsheetPreviewRows). */
export const CRAFT_SPREADSHEET_MAX_PREVIEW_ROWS = 1000;

/** Page count for a sheet of the given data rows under the preview cap. */
export function spreadsheetPreviewPageCount(rows: number, pageRows: number): number {
  if (rows <= 0 || pageRows <= 0) return 0;
  const capped = Math.min(pageRows, CRAFT_SPREADSHEET_MAX_PREVIEW_ROWS);
  return Math.ceil(rows / capped);
}

// Spreadsheet strings ship with the view (RESTORE_STRINGS pattern): the
// generic craft strings stay in presentation.ts.
const SPREADSHEET_STRINGS = {
  zh: {
    sheets: '工作表',
    rows: '行数',
    columns: '列数',
    total: '计算总额',
    recalculated: '已重算',
    download: '下载 XLSX',
    page: '页',
    prevPage: '上一页',
    nextPage: '下一页',
    loading: '正在加载表格预览…',
    failed: '表格预览加载失败',
    empty: '此版本还没有表格预览',
    retry: '重新加载预览',
    notRecalculated: '此表未完成重算，仅可下载',
  },
  en: {
    sheets: 'Sheets',
    rows: 'Rows',
    columns: 'Columns',
    total: 'Computed total',
    recalculated: 'Recalculated',
    download: 'Download XLSX',
    page: 'Page',
    prevPage: 'Previous page',
    nextPage: 'Next page',
    loading: 'Loading spreadsheet preview…',
    failed: 'Spreadsheet preview failed to load',
    empty: 'No spreadsheet preview for this version yet',
    retry: 'Reload preview',
    notRecalculated: 'This sheet was not recalculated; download only',
  },
} as const;

export interface CraftSpreadsheetProps {
  locale: CraftLocale;
  /** The selected immutable version (null when nothing is published yet). */
  versionId: string | null;
  /** The fetched server-converted preview (null until it loads). */
  preview: CraftSpreadsheetPreviewView | null;
  previewLoading: boolean;
  previewError: string | null;
  /** Re-fetches the preview of the selected version — preview only, never re-runs generation. */
  onFetchPreview(versionId: string): void;
  /** Downloads one immutable member; the assembly owns auth + blob plumbing. */
  onDownload(versionId: string, path: string): void | Promise<void>;
  downloading: boolean;
}

export function CraftSpreadsheet(props: CraftSpreadsheetProps) {
  const strings = craftStrings(props.locale);
  const s = props.locale === 'zh' ? SPREADSHEET_STRINGS.zh : SPREADSHEET_STRINGS.en;
  const [activeSheet, setActiveSheet] = useState(0);
  const [page, setPage] = useState(0);

  // Switching the version resets sheet/page selection: the new preview is a
  // different document and must not inherit stale positions.
  useEffect(() => { setActiveSheet(0); setPage(0); }, [props.versionId, props.preview]);

  const sheet = props.preview !== null ? props.preview.sheets[activeSheet] ?? null : null;
  const pageRows = props.preview !== null && props.preview.page_rows > 0
    ? Math.min(props.preview.page_rows, CRAFT_SPREADSHEET_MAX_PREVIEW_ROWS)
    : CRAFT_SPREADSHEET_MAX_PREVIEW_ROWS;
  const pageCount = sheet !== null ? spreadsheetPreviewPageCount(sheet.cells.length, pageRows) : 0;
  const visible = useMemo(() => {
    if (sheet === null) return [];
    return sheet.cells.slice(page * pageRows, (page + 1) * pageRows);
  }, [sheet, page, pageRows]);

  if (props.versionId === null) {
    return (
      <div className="wk-craft-preview-state" data-testid="craft-spreadsheet" data-state="empty" role="status">
        <p>{s.empty}</p>
      </div>
    );
  }
  if (props.preview === null) {
    return (
      <div className="wk-craft-preview-state" data-testid="craft-spreadsheet" data-state={props.previewError === null ? 'loading' : 'failed'} role="status">
        <p>{props.previewError !== null ? s.failed + (props.previewError !== '' ? ' — ' + props.previewError : '') : s.loading}</p>
        <Button type="button" onClick={() => props.onFetchPreview(props.versionId ?? '')}>{s.retry}</Button>
      </div>
    );
  }

  return (
    <div className="wk-craft-panel-body" data-testid="craft-spreadsheet" data-state="active">
      <div className="wk-craft-tabs" role="tablist" aria-label={s.sheets}>
        {props.preview.sheets.map((entry, index) => (
          <button
            key={entry.name}
            type="button"
            role="tab"
            className="wk-craft-tab"
            aria-selected={index === activeSheet}
            data-testid="craft-sheet-tab"
            onClick={() => { setActiveSheet(index); setPage(0); }}
          >
            {entry.name}
          </button>
        ))}
      </div>

      {sheet !== null ? (
        <>
          <p className="wk-craft-hint" data-testid="craft-sheet-meta">
            {s.rows} {sheet.rows} · {s.columns} {sheet.columns} · {s.total} <strong data-testid="craft-sheet-total">{sheet.total}</strong>
          </p>
          <div className="wk-craft-actions">
            <Button
              type="button"
              data-testid="craft-sheet-download"
              disabled={props.downloading}
              onClick={() => props.onDownload(props.versionId ?? '', CRAFT_SPREADSHEET_XLSX_PATH)}
            >
              {props.downloading ? strings.craftDownloading : s.download}
            </Button>
          </div>
          <div className="wk-craft-table-wrap">
            <table className="wk-craft-table" data-testid="craft-sheet-table">
              <tbody>
                {visible.map((row, rowIndex) => (
                  <tr key={rowIndex}>
                    {row.map((cell, columnIndex) => (
                      // Cells are PLAIN TEXT: React escapes content, and the
                      // data-type hook preserves the converted numeric/date/
                      // currency typing for styling (no script ever runs).
                      <td key={columnIndex} data-type={cell.type}>{cell.text}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {pageCount > 1 ? (
            <div className="wk-craft-actions">
              <Button type="button" disabled={page === 0} onClick={() => setPage((current) => Math.max(0, current - 1))}>{s.prevPage}</Button>
              <span className="wk-craft-hint">{s.page} {page + 1}/{pageCount}</span>
              <Button type="button" disabled={page >= pageCount - 1} onClick={() => setPage((current) => Math.min(pageCount - 1, current + 1))}>{s.nextPage}</Button>
            </div>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
