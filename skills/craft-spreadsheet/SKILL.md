---
name: craft-spreadsheet
description: Turn staged CSV material into a recalculated XLSX workbook (report.xlsx) with a server-converted preview (preview.json) and a verification manifest (manifest.json). openpyxl creates and parses; LibreOffice headless recalculates. Use when the user asks for a table, spreadsheet or Excel export from uploaded data.
version: 1
---

# Craft Spreadsheet（D02 数据表格与 XLSX 导出）

You are generating one deliverable round for a craft "craft" spreadsheet artwork.
Everything you write goes into the workspace "output/" directory. The round
succeeds only when "output/manifest.json" validates against
`internal/craft`'s ValidateSpreadsheetManifest — recalculation, preview
and file parsing must ALL have passed. Anything less is a failed round:
report it as failed, never ship a half-verified workbook.

## Outputs (exact names)

| file | meaning |
|---|---|
| `output/report.xlsx` | the deliverable workbook (plain .xlsx, no macros) |
| `output/preview.json` | server-converted preview data (sheets, rows, totals, typed cells) |
| `manifest.json` | sheet names, dimensions, recalculated totals, gate checks |

## Pipeline (in order, no shortcuts)

1. **Read the staged material** from `inputs/` (read-only). Refuse anything
   over 20 MiB or any CSV whose parsed sheet would exceed 5 sheets /
   100000 rows per sheet / 100 columns: write `manifest.json` with the
   offending check `failed` and stop.
2. **Guard every CSV cell** before it enters the workbook: a cell whose
   first byte is `=`, `+`, `-`, `@`, TAB or CR is stored as a literal
   STRING cell (`cell.data_type = "s"`), never executed as a formula.
   Guarded text keeps its exact bytes.
3. **Create the workbook with openpyxl** (image: python3-openpyxl 3.0.9).
   Formulas come ONLY from the templates below or from a formula the user
   explicitly asked for; never from cell text. Refuse formulas containing
   external workbook refs (`[``]`), DDE (`|`), URL schemes, or the
   functions WEBSERVICE / FILTERXML / RTD / HYPERLINK / EXEC / CALL /
   REGISTER / SEND.KEYS / DDE. Template set: `SUM`, `AVERAGE`, `COUNT`, `COUNTA`, `MAX`, `MIN`, `ROUND`, `IF`, `SUMIF`, `COUNTIF`, `AVERAGEIF` and same-workbook
   cross-sheet references like `=月度销售!C4`.
   Preserve cell types: dates as dates, money/numbers as numbers with
   `#,##0.00` style formats (the thousands separator is a number FORMAT,
   never text). Chinese sheet names are expected and supported.
4. **Recalculate with LibreOffice headless** (image:
   libreoffice-calc-nogui 7.4.7). Always seed an isolated profile first —
   without it a headless convert silently keeps stale caches:

   ```bash
   mkdir -p /tmp/lo-profile/user
   cat > /tmp/lo-profile/user/registrymodifications.xcu <<'XCU'
   <?xml version="1.0" encoding="UTF-8"?>
   <oor:items xmlns:oor="http://openoffice.org/2001/registry">
    <item oor:path="/org.openoffice.Office.Calc/Formula/Load"><prop oor:name="OOXMLRecalcMode" oor:op="fuse"><value>0</value></prop></item>
   </oor:items>
   XCU
   timeout -k 5 60 soffice --headless --norestore \
     -env:UserInstallation=file:///tmp/lo-profile \
     --convert-to xlsx --outdir "${OUT}" "${WORK}"
   ```

   The `timeout` budget is mandatory: a conversion killed by it (exit
   >= 124) records check `recalc: failed` and stops — never retry in a
   loop, never ship the un-recalculated file.
5. **Parse the recalculated file with openpyxl** (`load_workbook(..., data_only=True)`) and read
   the CACHED values. A formula cell whose cached value is missing means
   the recalculation did not happen: the sheet's `recalculated` stays false
   and the round fails. Collect formula errors (`#REF!`, `#DIV/0!`, `#VALUE!`, `#NAME?`, `#NUM!`) into
   `formula_errors` — any error entry fails the sheet.
6. **Write `output/preview.json`** from the recalculated values: one entry
   per sheet `{name, rows, columns, total, cells}` where `cells` carries
   the DISPLAY text plus a type tag (`number` / `date` / `currency` / `text` / `empty`). Cap preview data at
   1000 rows per page (`page_rows`, `page_count`). This file is the ONLY
   preview source — the browser never parses the workbook. When rendering
   any preview surface, cells are plain text (escaped), never HTML.
7. **Write `output/manifest.json`** exactly in this shape:

   ```json
   {
     "kind": "spreadsheet",
     "xlsx": "report.xlsx",
     "preview": "preview.json",
     "sheets": [{"name": "月度销售", "rows": 4, "columns": 4, "formula_errors": [], "recalculated": true}],
     "totals": {"月度销售": "300"},
     "preview_totals": {"月度销售": "300"},
     "checks": [
       {"name": "recalc", "status": "passed", "detail": "soffice --headless 7.4.7, OOXMLRecalcMode=0"},
       {"name": "preview", "status": "passed", "detail": "preview.json regenerated from recalculated cache"},
       {"name": "parse", "status": "passed", "detail": "openpyxl 3.0.9 data_only"}
     ]
   }
   ```

   `totals` are read from the recalculated cache; `preview_totals` are the
   separately so their agreement is checkable — recompute the preview from
   the recalculated workbook, never by hand.

## Failure states (write them, never fake a pass)

- over-limit input (20 MiB / 5 sheets / 100000 rows / 100 columns) →
  `checks: [{name: "parse", status: "failed", detail: "..."}]`, no report.xlsx claim;
- conversion killed by its budget → `recalc: failed`;
- corrupt workbook / unopenable ZIP → `parse: failed`;
- any formula error string → that sheet keeps it in `formula_errors` and
  the sheet is not ready.

## Not promised

Online Excel editing, macro support (.xlsm/VBA), external data links, DDE,
or formulas that reach the network. The deliverable is a plain recalculated
.xlsx plus a server-converted preview.
