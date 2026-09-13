// D02 数据表格与 XLSX 导出 — spreadsheet rules.
//
// This file holds the stable spreadsheet contracts of the craft package:
// the readiness rule (a sheet is only ready after a REAL recalculation,
// never because formulas were merely written), the default limits, the
// formula policy, the CSV-injection guard, and the manifest contract that
// gates the spreadsheet kind on recalc + preview + parse ALL passing. The
// heavy lifting (workbook creation, parsing, recalculation) belongs to the
// craft-spreadsheet skill inside the sandbox image: openpyxl creates and
// parses, LibreOffice headless recalculates, and only cached values that
// exist in the stored XLSX after that recalculation count.
package craft

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// KindSpreadsheet names the spreadsheet artwork kind (request.go's closed
// kind set carries the same literal).
const KindSpreadsheet = "spreadsheet"

// The version-relative artifact paths the craft-spreadsheet skill must
// produce in one delegation's output directory. report.xlsx is the
// deliverable; preview.json is the controlled server-converted preview
// data (never parsed in the browser from the workbook itself).
const (
	SpreadsheetXLSXPath    = "report.xlsx"
	SpreadsheetPreviewPath = "preview.json"
)

// Spreadsheet gate check names, extending the version checks (build,
// entry, preview) with the two facts only the spreadsheet pipeline can
// observe: LibreOffice actually recalculated, and openpyxl actually parsed
// the stored file.
const (
	CheckRecalc = "recalc"
	CheckParse  = "parse"
)

// Default spreadsheet limits (brief Step 5). They bound what one
// delegation accepts before any conversion work starts, so an oversized
// input fails fast with a failed status instead of exhausting the sandbox.
const (
	MaxSpreadsheetSheets          = 5
	MaxSpreadsheetRowsPerSheet    = 100000
	MaxSpreadsheetColumnsPerSheet = 100
	MaxSpreadsheetSourceBytes     = 20 << 20 // 20 MiB, matching the per-file input cap
	MaxSpreadsheetPreviewRows     = 1000
)

// SheetSummary is one worksheet's verification summary. Recalculated is
// true only when the recalculated cache values were actually read back
// from the stored workbook — writing a formula alone never sets it.
type SheetSummary struct {
	Name          string   `json:"name"`
	Rows          int      `json:"rows"`
	Columns       int      `json:"columns"`
	FormulaErrors []string `json:"formula_errors,omitempty"`
	Recalculated  bool     `json:"recalculated"`
}

// SheetReady reports whether one sheet may ship: named, non-empty, really
// recalculated, and free of formula errors.
func SheetReady(s SheetSummary) bool {
	return s.Name != "" && s.Rows > 0 && s.Columns > 0 && s.Recalculated && len(s.FormulaErrors) == 0
}

// ValidateSheets enforces the structural limits: at least one and at most
// MaxSpreadsheetSheets sheets, unique non-blank names, and per-sheet
// dimensions within the row/column caps. Readiness (SheetReady) is a
// separate fact — structure can be valid while recalculation is still
// pending.
func ValidateSheets(sheets []SheetSummary) error {
	if len(sheets) == 0 {
		return fmt.Errorf("%w: spreadsheet has no sheets", ErrInvalidInput)
	}
	if len(sheets) > MaxSpreadsheetSheets {
		return fmt.Errorf("%w: %d sheets exceed the %d-sheet cap", ErrInvalidInput, len(sheets), MaxSpreadsheetSheets)
	}
	seen := make(map[string]struct{}, len(sheets))
	for _, s := range sheets {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("%w: sheet with a blank name", ErrInvalidInput)
		}
		if _, dup := seen[s.Name]; dup {
			return fmt.Errorf("%w: duplicate sheet name %q", ErrInvalidInput, s.Name)
		}
		seen[s.Name] = struct{}{}
		if s.Rows <= 0 || s.Rows > MaxSpreadsheetRowsPerSheet {
			return fmt.Errorf("%w: sheet %q has %d rows, cap is %d", ErrInvalidInput, s.Name, s.Rows, MaxSpreadsheetRowsPerSheet)
		}
		if s.Columns <= 0 || s.Columns > MaxSpreadsheetColumnsPerSheet {
			return fmt.Errorf("%w: sheet %q has %d columns, cap is %d", ErrInvalidInput, s.Name, s.Columns, MaxSpreadsheetColumnsPerSheet)
		}
	}
	return nil
}

// ValidateSpreadsheetSource bounds one staged source (CSV upload or prior
// workbook) before any parsing or conversion begins.
func ValidateSpreadsheetSource(bytes int64) error {
	if bytes <= 0 {
		return fmt.Errorf("%w: spreadsheet source is empty", ErrInvalidInput)
	}
	if bytes > MaxSpreadsheetSourceBytes {
		return fmt.Errorf("%w: spreadsheet source is %d bytes, cap is %d", ErrInvalidInput, bytes, MaxSpreadsheetSourceBytes)
	}
	return nil
}

// SpreadsheetPreviewPages splits preview rows into pages of at most
// MaxSpreadsheetPreviewRows rows each. Non-positive row counts have no
// pages.
func SpreadsheetPreviewPages(rows int) int {
	if rows <= 0 {
		return 0
	}
	return (rows + MaxSpreadsheetPreviewRows - 1) / MaxSpreadsheetPreviewRows
}

// spreadsheetForbiddenFormulaTokens are formula constructs that never come
// from a template: external workbook references ("[1]Sheet!A1"), the DDE
// separator ("=cmd|'...'!A0") and URL schemes that reach outside the
// workbook.
var spreadsheetForbiddenFormulaTokens = []string{"[", "]", "|", "FILE://", "HTTP://", "HTTPS://", "FTP://"}

// spreadsheetForbiddenFormulaFuncs are functions that reach outside the
// workbook or execute things: network lookups, external links and the XLM
// macro legacy set. An .xlsx cannot carry XLM macros, but a formula naming
// those functions is still refused — defense in depth.
var spreadsheetForbiddenFormulaFuncs = []string{
	"WEBSERVICE", "FILTERXML", "RTD", "HYPERLINK",
	"EXEC", "CALL", "REGISTER", "SEND.KEYS", "DDE",
}

// spreadsheetForbiddenFormulaRe is the word-boundary form of the forbidden
// function set (D02 review NIT-1): a name match must be a WHOLE word
// followed by optional whitespace and an argument list, so "=RTD\t("
// or "=CALL (" can no longer slip past a naive fn+"(" / fn+" " contains
// check, and innocent names that merely CONTAIN a forbidden token (SPREAD,
// HYPERLINKS2-style typos aside) are not collateral damage.
var spreadsheetForbiddenFormulaRe = func() *regexp.Regexp {
	quoted := make([]string, 0, len(spreadsheetForbiddenFormulaFuncs))
	for _, fn := range spreadsheetForbiddenFormulaFuncs {
		quoted = append(quoted, regexp.QuoteMeta(fn))
	}
	return regexp.MustCompile("\\b(?:" + strings.Join(quoted, "|") + ")\\s*\\(")
}()

// SpreadsheetFormulaAllowed reports whether a formula may be written into
// the workbook: it must be formula-shaped (leading "=") and contain none
// of the forbidden external/DDE/network/macro constructs. This is the
// machine-enforced half of the policy "formulas only from allowed
// templates or explicit user goals"; choosing the template stays the
// skill's responsibility.
func SpreadsheetFormulaAllowed(formula string) bool {
	f := strings.TrimSpace(formula)
	if !strings.HasPrefix(f, "=") || len(f) < 2 {
		return false
	}
	upper := strings.ToUpper(f)
	for _, token := range spreadsheetForbiddenFormulaTokens {
		if strings.Contains(upper, token) {
			return false
		}
	}
	if spreadsheetForbiddenFormulaRe.MatchString(upper) {
		return false
	}
	return true
}

// SpreadsheetSanitizeCSVCell applies the CSV-injection guard to one raw
// CSV cell. A cell whose first byte makes spreadsheet applications parse
// it as a formula ("=", "+", "-", "@", TAB, CR) is flagged guarded: the
// caller must store the exact same bytes as a TEXT cell, never as a
// formula. Plain cells pass through untouched.
func SpreadsheetSanitizeCSVCell(cell string) (text string, guarded bool) {
	if cell == "" {
		return cell, false
	}
	switch cell[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return cell, true
	}
	return cell, false
}

// SpreadsheetManifest is the manifest.json the craft-spreadsheet skill
// writes next to report.xlsx. It records the sheet names and dimensions,
// the totals read from the RECALCULATED cache values, the same totals as
// rendered into the preview, and the three gate checks. Totals and
// PreviewTotals exist separately so their agreement is verifiable — the
// honest pipeline recomputes the preview from the recalculated workbook,
// so a mismatch means one of the two lied.
type SpreadsheetManifest struct {
	Kind          string            `json:"kind"`
	XLSXPath      string            `json:"xlsx"`
	PreviewPath   string            `json:"preview"`
	Sheets        []SheetSummary    `json:"sheets"`
	Totals        map[string]string `json:"totals"`
	PreviewTotals map[string]string `json:"preview_totals"`
	Checks        []Check           `json:"checks"`
}

// ValidateSpreadsheetManifest is the spreadsheet kind's gate: every sheet
// must be structurally valid AND ready (really recalculated, no formula
// errors), the recalculated totals must match the preview totals
// numerically, and all three gate checks — recalc, preview, parse — must
// have run and passed. Anything less keeps the kind closed.
func ValidateSpreadsheetManifest(m SpreadsheetManifest) error {
	if m.Kind != KindSpreadsheet {
		return fmt.Errorf("%w: spreadsheet manifest kind is %q", ErrInvalidInput, m.Kind)
	}
	if m.XLSXPath != SpreadsheetXLSXPath {
		return fmt.Errorf("%w: spreadsheet deliverable path is %q, want %q", ErrInvalidInput, m.XLSXPath, SpreadsheetXLSXPath)
	}
	if m.PreviewPath != SpreadsheetPreviewPath {
		return fmt.Errorf("%w: spreadsheet preview path is %q, want %q", ErrInvalidInput, m.PreviewPath, SpreadsheetPreviewPath)
	}
	if err := ValidateSheets(m.Sheets); err != nil {
		return err
	}
	if len(m.Totals) != len(m.Sheets) || len(m.PreviewTotals) != len(m.Sheets) {
		return fmt.Errorf("%w: totals must exist for exactly the %d sheets", ErrInvalidInput, len(m.Sheets))
	}
	for _, s := range m.Sheets {
		if !SheetReady(s) {
			return fmt.Errorf("%w: sheet %q is not ready (recalculated=%v, formula errors=%d)", ErrInvalidInput, s.Name, s.Recalculated, len(s.FormulaErrors))
		}
		total, err := parseFloatTotal(m.Totals[s.Name])
		if err != nil {
			return fmt.Errorf("%w: sheet %q recalculated total: %v", ErrInvalidInput, s.Name, err)
		}
		preview, err := parseFloatTotal(m.PreviewTotals[s.Name])
		if err != nil {
			return fmt.Errorf("%w: sheet %q preview total: %v", ErrInvalidInput, s.Name, err)
		}
		if total != preview {
			return fmt.Errorf("%w: sheet %q recalculated total %v differs from preview total %v", ErrInvalidInput, s.Name, total, preview)
		}
	}
	statuses := make(map[string]string, len(m.Checks))
	for _, c := range m.Checks {
		statuses[c.Name] = c.Status
	}
	for _, name := range []string{CheckRecalc, CheckPreview, CheckParse} {
		if statuses[name] != CheckPassed {
			return fmt.Errorf("%w: spreadsheet gate check %q is %q, want %q", ErrInvalidInput, name, statuses[name], CheckPassed)
		}
	}
	return nil
}

func parseFloatTotal(raw string) (float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, fmt.Errorf("empty total")
	}
	return strconv.ParseFloat(trimmed, 64)
}
