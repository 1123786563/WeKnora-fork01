// D02 数据表格与 XLSX 导出 — spreadsheet rules tests.
//
// The verbatim brief test (Step 1) pins the core contract: a sheet that was
// never recalculated, or still carries formula errors, is never ready. The
// rest of the file exercises Step 4–6: the default limits, the formula
// policy (template formulas only — CSV-shaped text stays text), the
// manifest contract that recalc, preview and file parsing must ALL pass,
// and real XLSX parsing: LibreOffice/openpyxl-style cached values are read
// back from the stored OOXML while a merely-written formula has no cached
// value and therefore does not count as recalculated.
package craft

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// Step 1 (verbatim): readiness requires a real recalculation
// ---------------------------------------------------------------------------

func TestSheetRequiresRecalculation(t *testing.T) {
	s := SheetSummary{Name: "Sales", Rows: 2, Columns: 3}
	if SheetReady(s) {
		t.Fatal("unevaluated formulas accepted")
	}
	s.Recalculated = true
	if !SheetReady(s) {
		t.Fatal("valid sheet rejected")
	}
	s.FormulaErrors = []string{"#REF!"}
	if SheetReady(s) {
		t.Fatal("formula errors ignored")
	}
}

// ---------------------------------------------------------------------------
// Step 5: default limits
// ---------------------------------------------------------------------------

func TestSpreadsheetDefaultLimits(t *testing.T) {
	if MaxSpreadsheetSheets != 5 {
		t.Fatalf("sheet cap = %d, want 5", MaxSpreadsheetSheets)
	}
	if MaxSpreadsheetRowsPerSheet != 100000 {
		t.Fatalf("row cap = %d, want 100000", MaxSpreadsheetRowsPerSheet)
	}
	if MaxSpreadsheetColumnsPerSheet != 100 {
		t.Fatalf("column cap = %d, want 100", MaxSpreadsheetColumnsPerSheet)
	}
	if MaxSpreadsheetSourceBytes != 20<<20 {
		t.Fatalf("source cap = %d, want 20MiB", MaxSpreadsheetSourceBytes)
	}
	if MaxSpreadsheetPreviewRows != 1000 {
		t.Fatalf("preview page = %d, want 1000 rows", MaxSpreadsheetPreviewRows)
	}
}

func TestValidateSheetsEnforcesLimits(t *testing.T) {
	sheet := func(name string, rows, cols int) SheetSummary {
		return SheetSummary{Name: name, Rows: rows, Columns: cols, Recalculated: true}
	}
	if err := ValidateSheets(nil); err == nil {
		t.Fatal("empty sheet list accepted")
	}
	five := []SheetSummary{
		sheet("一", 1, 1), sheet("二", 1, 1), sheet("三", 1, 1), sheet("四", 1, 1), sheet("五", 1, 1),
	}
	if err := ValidateSheets(five); err != nil {
		t.Fatalf("five valid sheets rejected: %v", err)
	}
	if err := ValidateSheets(append(append([]SheetSummary(nil), five...), sheet("六", 1, 1))); err == nil {
		t.Fatal("sixth sheet accepted over the 5-sheet cap")
	}
	if err := ValidateSheets([]SheetSummary{sheet("ok", MaxSpreadsheetRowsPerSheet, MaxSpreadsheetColumnsPerSheet)}); err != nil {
		t.Fatalf("sheet at the exact caps rejected: %v", err)
	}
	if err := ValidateSheets([]SheetSummary{sheet("rows", MaxSpreadsheetRowsPerSheet+1, 3)}); err == nil {
		t.Fatal("row overflow accepted")
	}
	if err := ValidateSheets([]SheetSummary{sheet("cols", 3, MaxSpreadsheetColumnsPerSheet+1)}); err == nil {
		t.Fatal("column overflow accepted")
	}
	if err := ValidateSheets([]SheetSummary{sheet("", 3, 3)}); err == nil {
		t.Fatal("unnamed sheet accepted")
	}
	if err := ValidateSheets([]SheetSummary{sheet("dup", 3, 3), sheet("dup", 2, 2)}); err == nil {
		t.Fatal("duplicate sheet names accepted")
	}
}

func TestValidateSpreadsheetSourceBytes(t *testing.T) {
	if err := ValidateSpreadsheetSource(MaxSpreadsheetSourceBytes); err != nil {
		t.Fatalf("20MiB source rejected: %v", err)
	}
	if err := ValidateSpreadsheetSource(MaxSpreadsheetSourceBytes + 1); err == nil {
		t.Fatal("over-cap source accepted")
	}
	if err := ValidateSpreadsheetSource(0); err == nil {
		t.Fatal("empty source accepted")
	}
}

func TestSpreadsheetPreviewPagination(t *testing.T) {
	if got := SpreadsheetPreviewPages(0); got != 0 {
		t.Fatalf("pages(0) = %d, want 0", got)
	}
	if got := SpreadsheetPreviewPages(1); got != 1 {
		t.Fatalf("pages(1) = %d, want 1", got)
	}
	if got := SpreadsheetPreviewPages(MaxSpreadsheetPreviewRows); got != 1 {
		t.Fatalf("pages(%d) = %d, want 1", MaxSpreadsheetPreviewRows, got)
	}
	if got := SpreadsheetPreviewPages(MaxSpreadsheetPreviewRows + 1); got != 2 {
		t.Fatalf("pages(%d+1) = %d, want 2", MaxSpreadsheetPreviewRows, got)
	}
	if got := SpreadsheetPreviewPages(2500); got != 3 {
		t.Fatalf("pages(2500) = %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// Step 4: formulas come from templates only; CSV-shaped text stays text
// ---------------------------------------------------------------------------

func TestSpreadsheetFormulaPolicy(t *testing.T) {
	allowed := []string{
		"=SUM(C2:C3)",
		"=AVERAGE(C2:C4)",
		"=ROUND(SUM(C2:C3),2)",
		"=IF(C2>0,\"正\",\"负\")",
		"=SUMIF(B2:B3,\"东区\",C2:C3)",
		"=月度销售!C4",           // cross-sheet reference inside the same workbook
		"=SUM('月度销售'!C2:C3)", // quoted CJK sheet name, still internal
	}
	for _, f := range allowed {
		if !SpreadsheetFormulaAllowed(f) {
			t.Fatalf("template formula %q refused", f)
		}
	}
	forbidden := map[string]string{
		"=WEBSERVICE(\"http://evil.example/x\")": "network formula",
		"=FILTERXML(A1,\"//x\")":                 "network formula",
		"=RTD(\"srv\",,\"topic\")":               "network formula",
		"=HYPERLINK(\"http://evil.example\")":    "external link",
		"=cmd|'/c calc'!A0":                      "DDE",
		"=SUM([1]Sheet1!A1)":                     "external workbook reference",
		"='file:///etc/passwd'#$A$1":             "external file reference",
		"=EXEC(\"/bin/sh\")":                     "XLM macro function",
		"=CALL(\"Shell\")":                       "XLM macro function",
	}
	for f, why := range forbidden {
		if SpreadsheetFormulaAllowed(f) {
			t.Fatalf("%s accepted: %q", why, f)
		}
	}
	// Text without a leading = is never a formula candidate.
	if SpreadsheetFormulaAllowed("SUM(C2:C3)") {
		t.Fatal("bare text accepted as formula")
	}
	if SpreadsheetFormulaAllowed("") {
		t.Fatal("empty cell accepted as formula")
	}
}

func TestCSVFormulaInjectionStaysText(t *testing.T) {
	injected := []string{
		"=SUM(A1:A2)",
		"+1|cmd",
		"-2+3+cmd|' /c calc'!A0",
		"@cmd",
		"=cmd|'/c calc'!A0",
		"\t=1+1",
		"\r=WEBSERVICE(\"http://x\")",
	}
	for _, cell := range injected {
		text, guarded := SpreadsheetSanitizeCSVCell(cell)
		if !guarded {
			t.Fatalf("CSV cell %q not guarded", cell)
		}
		if text != cell {
			t.Fatalf("guard mutated bytes: %q -> %q", cell, text)
		}
	}
	plain := []string{"100", "东区", "2026-01", "收入", "", "1,234", "  =not-a-formula-with-space"}
	for _, cell := range plain {
		text, guarded := SpreadsheetSanitizeCSVCell(cell)
		if guarded {
			t.Fatalf("plain CSV cell %q falsely guarded", cell)
		}
		if text != cell {
			t.Fatalf("plain cell mutated: %q -> %q", cell, text)
		}
	}
}

// ---------------------------------------------------------------------------
// Step 4: the manifest contract — recalc + preview + parse all passed
// ---------------------------------------------------------------------------

func passedSpreadsheetChecks() []Check {
	return []Check{
		{Name: CheckRecalc, Status: CheckPassed, Detail: "LibreOffice headless recalculated every formula"},
		{Name: CheckPreview, Status: CheckPassed, Detail: "preview rows/totals match the recalculated workbook"},
		{Name: CheckParse, Status: CheckPassed, Detail: "openpyxl parsed report.xlsx without formula errors"},
	}
}

func validSpreadsheetManifest() SpreadsheetManifest {
	return SpreadsheetManifest{
		Kind:          KindSpreadsheet,
		XLSXPath:      SpreadsheetXLSXPath,
		PreviewPath:   SpreadsheetPreviewPath,
		Sheets:        []SheetSummary{{Name: "月度销售", Rows: 4, Columns: 4, Recalculated: true}},
		Totals:        map[string]string{"月度销售": "300"},
		PreviewTotals: map[string]string{"月度销售": "300"},
		Checks:        passedSpreadsheetChecks(),
	}
}

func TestValidateSpreadsheetManifestAccepts(t *testing.T) {
	if err := ValidateSpreadsheetManifest(validSpreadsheetManifest()); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
}

func TestValidateSpreadsheetManifestRejections(t *testing.T) {
	m := validSpreadsheetManifest()
	m.XLSXPath = "other.xlsx"
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("wrong xlsx path accepted")
	}

	m = validSpreadsheetManifest()
	m.PreviewPath = ""
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("missing preview path accepted")
	}

	m = validSpreadsheetManifest()
	m.Sheets[0].Recalculated = false
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("not-recalculated sheet accepted")
	}

	m = validSpreadsheetManifest()
	m.Sheets[0].FormulaErrors = []string{"#REF!"}
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("formula errors accepted")
	}

	// The recalculated total and the preview total must agree.
	m = validSpreadsheetManifest()
	m.Totals["月度销售"] = "350"
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("recalc/preview total mismatch accepted")
	}
	m = validSpreadsheetManifest()
	delete(m.PreviewTotals, "月度销售")
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("missing preview total accepted")
	}
	m = validSpreadsheetManifest()
	m.Totals["月度销售"] = "三百"
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("non-numeric total accepted")
	}

	// Every gate check must have run and passed.
	m = validSpreadsheetManifest()
	m.Checks = []Check{
		{Name: CheckRecalc, Status: CheckPassed},
		{Name: CheckPreview, Status: CheckNotRun},
		{Name: CheckParse, Status: CheckPassed},
	}
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("not_run preview check accepted")
	}
	m = validSpreadsheetManifest()
	m.Checks = []Check{{Name: CheckRecalc, Status: CheckPassed}}
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("missing preview/parse checks accepted")
	}

	m = validSpreadsheetManifest()
	m.Kind = "web"
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("foreign kind accepted")
	}
}

func TestSpreadsheetManifestJSONRoundTrip(t *testing.T) {
	raw := `{"kind": "spreadsheet",
		"xlsx": "report.xlsx",
		"preview": "preview.json",
		"sheets": [{"name": "汇总", "rows": 3, "columns": 2, "formula_errors": [], "recalculated": true}],
		"totals": {"汇总": "300"},
		"preview_totals": {"汇总": "300.0"},
		"checks": [
			{"name": "recalc", "status": "passed", "detail": "soffice --headless"},
			{"name": "preview", "status": "passed", "detail": ""},
			{"name": "parse", "status": "passed", "detail": "openpyxl data_only"}
		]
	}`
	var m SpreadsheetManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	// "300.0" and "300" are the same number: comparison is numeric.
	if err := ValidateSpreadsheetManifest(m); err != nil {
		t.Fatalf("manifest.json as produced by the skill rejected: %v", err)
	}
	if m.Sheets[0].Name != "汇总" || !m.Sheets[0].Recalculated {
		t.Fatalf("sheet summary lost: %+v", m.Sheets[0])
	}
}

// ---------------------------------------------------------------------------
// Step 6: real XLSX formula parsing and recalculated cache values
// ---------------------------------------------------------------------------

// writeFixtureWorkbook writes a real XLSX with the Step-6 sales fixture: a
// Chinese sheet, a date cell, an empty cell, a thousands-formatted number,
// the malicious formula-shaped CSV text guarded as a string cell, and the
// SUM formula. Cached formula values are what LibreOffice writes after a
// recalculation; injectCachedValues reproduces that storage for the
// no-LibreOffice unit environment (the LibreOffice-produced file itself is
// covered by TestLibreOfficeRecalculatedFixture below).
func writeFixtureWorkbook(t *testing.T, dir, name string, revenues []int, cachedTotal string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "月度销售"); err != nil {
		t.Fatal(err)
	}
	sheet := "月度销售"
	for i, h := range []string{"月份", "地区", "收入", "备注"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			t.Fatal(err)
		}
	}
	regions := []string{"东区", "西区"}
	for i, rev := range revenues {
		row := i + 2
		if err := f.SetCellValue(sheet, "A"+itoa(row), dateOnly(2026, i+1)); err != nil {
			t.Fatal(err)
		}
		if err := f.SetCellValue(sheet, "B"+itoa(row), regions[i%len(regions)]); err != nil {
			t.Fatal(err)
		}
		if err := f.SetCellValue(sheet, "C"+itoa(row), rev); err != nil {
			t.Fatal(err)
		}
		// D stays empty: empty cells must survive the round trip.
	}
	last := len(revenues) + 1
	totalRow := last + 1
	if err := f.SetCellValue(sheet, "A"+itoa(totalRow), "合计"); err != nil {
		t.Fatal(err)
	}
	formula := "SUM(C2:C" + itoa(last) + ")"
	if err := f.SetCellFormula(sheet, "C"+itoa(totalRow), formula); err != nil {
		t.Fatal(err)
	}
	// Thousands separator is a number FORMAT, not text mutation: the value
	// stays numeric and the display gains the separators.
	style, err := f.NewStyle(&excelize.Style{CustomNumFmt: strPtr("#,##0")})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellStyle(sheet, "E1", "E1", style); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "E1", 1234); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "F1", "=cmd|'/c calc'!A0"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if cachedTotal == "" {
		return path
	}
	if err := injectCachedValues(path, map[string]string{"C" + itoa(totalRow): cachedTotal}); err != nil {
		t.Fatal(err)
	}
	return path
}

// injectCachedValues rewrites the stored worksheet XML the way LibreOffice
// does after a recalculation: the formula keeps its <f> element and gains a
// cached <v> result. This is storage-level truth — nothing is computed here.
func injectCachedValues(path string, cells map[string]string) error {
	rf, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer rf.Close()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, ent := range rf.File {
		data, err := readZipEntry(ent)
		if err != nil {
			return err
		}
		if len(ent.Name) > len("xl/worksheets/sheet") && ent.Name[:len("xl/worksheets/sheet")] == "xl/worksheets/sheet" && bytes.Contains(data, []byte("<f>")) {
			for ref, val := range cells {
				re, err := regexp.Compile(`(<c r="` + ref + `"[^>]*>)(<f[^>]*>[^<]*</f>)`)
				if err != nil {
					return err
				}
				data = re.ReplaceAll(data, []byte(`$1$2<v>`+val+`</v>`))
			}
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: ent.Name, Method: zip.Deflate})
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func readZipEntry(ent *zip.File) ([]byte, error) {
	rc, err := ent.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func TestXLSXWrittenFormulaAloneIsNotRecalculated(t *testing.T) {
	path := writeFixtureWorkbook(t, t.TempDir(), "monthly.xlsx", []int{100, 200}, "")
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if sheets := f.GetSheetList(); len(sheets) != 1 || sheets[0] != "月度销售" {
		t.Fatalf("sheets = %v, want [月度销售]", sheets)
	}
	formula, err := f.GetCellFormula("月度销售", "C4")
	if err != nil || formula != "SUM(C2:C3)" {
		t.Fatalf("formula = %q err=%v, want SUM(C2:C3)", formula, err)
	}
	cached, _ := f.GetCellValue("月度销售", "C4")
	if cached != "" {
		t.Fatalf("freshly written formula already carries a cached value %q", cached)
	}
	summary := SheetSummary{Name: "月度销售", Rows: 4, Columns: 6}
	if SheetReady(summary) {
		t.Fatal("sheet with an unevaluated formula accepted as ready")
	}
}

func TestXLSXRecalculatedCacheRoundTrip300And350(t *testing.T) {
	dir := t.TempDir()

	// Round 1: 100 + 200 with the recalculated cache 300.
	first := writeFixtureWorkbook(t, dir, "monthly.xlsx", []int{100, 200}, "300")
	f1, err := excelize.OpenFile(first)
	if err != nil {
		t.Fatal(err)
	}
	defer f1.Close()
	if v, _ := f1.GetCellValue("月度销售", "C4"); v != "300" {
		t.Fatalf("recalculated cache = %q, want 300", v)
	}
	// Date / empty / thousands / guarded-text cells survive with types.
	if d, _ := f1.GetCellValue("月度销售", "A2"); d == "" {
		t.Fatal("date cell lost")
	}
	if e, _ := f1.GetCellValue("月度销售", "D2"); e != "" {
		t.Fatalf("empty cell became %q", e)
	}
	if th, _ := f1.GetCellValue("月度销售", "E1"); th != "1,234" {
		t.Fatalf("thousands-formatted cell = %q, want 1,234", th)
	}
	// The guarded CSV text must have stayed a plain string cell: no
	// formula is recorded for it (excelize answers an empty formula, and a
	// non-formula cell is not an error).
	if fm, _ := f1.GetCellFormula("月度销售", "F1"); fm != "" {
		t.Fatalf("guarded CSV text executed as formula %q", fm)
	}
	if txt, _ := f1.GetCellValue("月度销售", "F1"); txt != "=cmd|'/c calc'!A0" {
		t.Fatalf("guarded CSV text mutated: %q", txt)
	}
	if !SheetReady(SheetSummary{Name: "月度销售", Rows: 4, Columns: 6, Recalculated: true}) {
		t.Fatal("recalculated sheet not ready")
	}

	// Round 2: quarterly summary of the same two rows — total stays 300
	// and the first workbook's bytes stay untouched on disk.
	firstBytes, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	quarterly := writeFixtureWorkbook(t, dir, "quarterly.xlsx", []int{100, 200}, "300")
	f2, err := excelize.OpenFile(quarterly)
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	if v, _ := f2.GetCellValue("月度销售", "C4"); v != "300" {
		t.Fatalf("quarterly recalc = %q, want 300", v)
	}
	again, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, again) {
		t.Fatal("old workbook bytes changed by a later round")
	}

	// Round 3: add one 50 row — recalculated total must be 350.
	third := writeFixtureWorkbook(t, dir, "extra.xlsx", []int{100, 200, 50}, "350")
	f3, err := excelize.OpenFile(third)
	if err != nil {
		t.Fatal(err)
	}
	defer f3.Close()
	if fm, _ := f3.GetCellFormula("月度销售", "C5"); fm != "SUM(C2:C4)" {
		t.Fatalf("extended formula = %q, want SUM(C2:C4)", fm)
	}
	if v, _ := f3.GetCellValue("月度销售", "C5"); v != "350" {
		t.Fatalf("extended recalc = %q, want 350", v)
	}
}

// TestLibreOfficeRecalculatedFixture parses the workbook that the locked
// LibreOffice headless actually recalculated inside the craft image (the
// acceptance run recorded in the D02 report). This is the storage-level
// proof the whole pipeline relies on: after --convert-to, every formula
// cell carries a cached <v> value.
func TestLibreOfficeRecalculatedFixture(t *testing.T) {
	path := filepath.Join("testdata", "d02_libreoffice_recalc.xlsx")
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("open LibreOffice fixture: %v", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) != 2 || sheets[0] != "月度销售" || sheets[1] != "汇总" {
		t.Fatalf("sheets = %v, want [月度销售 汇总]", sheets)
	}
	if fm, _ := f.GetCellFormula("月度销售", "C4"); fm != "SUM(C2:C3)" {
		t.Fatalf("formula = %q, want SUM(C2:C3)", fm)
	}
	if v, _ := f.GetCellValue("月度销售", "C4"); v != "300" {
		t.Fatalf("LibreOffice cached total = %q, want 300", v)
	}
	if fm, _ := f.GetCellFormula("汇总", "B2"); fm == "" {
		t.Fatal("cross-sheet formula missing")
	}
	if v, _ := f.GetCellValue("汇总", "B2"); v != "300" {
		t.Fatalf("cross-sheet cached total = %q, want 300", v)
	}
	// The recalculated sheet is ready; nobody had to trust a claim.
	if !SheetReady(SheetSummary{Name: "月度销售", Rows: 4, Columns: 6, Recalculated: true}) {
		t.Fatal("recalculated fixture sheet rejected")
	}
}

func TestCorruptXLSXIsAParseFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.xlsx")
	if err := os.WriteFile(path, []byte("not a zip at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := excelize.OpenFile(path); err == nil {
		t.Fatal("corrupt ZIP parsed as XLSX")
	}
	// A parse failure is a failed check, never a silently-ready sheet.
	m := validSpreadsheetManifest()
	m.Checks = []Check{
		{Name: CheckRecalc, Status: CheckPassed},
		{Name: CheckPreview, Status: CheckPassed},
		{Name: CheckParse, Status: CheckFailed, Detail: "corrupt ZIP"},
	}
	if err := ValidateSpreadsheetManifest(m); err == nil {
		t.Fatal("manifest accepted with a failed parse check")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func dateOnly(year int, month int) time.Time {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}

func strPtr(s string) *string { return &s }

// TestSpreadsheetFormulaWhitespaceBypassRejected pins the D02-review NIT-1
// fix: a forbidden function name separated from its argument list by ANY
// whitespace (TAB, newline, spaces) is rejected — the old contains-check
// only looked at fn+"(" and fn+" ".
func TestSpreadsheetFormulaWhitespaceBypassRejected(t *testing.T) {
	for _, formula := range []string{
		"=RTD\t(\"OMNI\",\"X\",\"Y\")",
		"=CALL (\"Shell\",\"A\")",
		"=1+HYPERLINK\t(\"http://x\")",
		"=DDE\n(\"svc\")",
		"=SEND.KEYS (1,\"a\")",
	} {
		if SpreadsheetFormulaAllowed(formula) {
			t.Fatalf("whitespace-separated forbidden formula accepted: %q", formula)
		}
	}
	// Word boundaries keep innocent names working.
	for _, formula := range []string{
		"=SUM(A1:A2)",
		"=AVERAGEIF(A:A,\">1\",B:B)",
		"=RTDX(A1)+CALLSUM(B1)", // contains the tokens as SUBstrings, not words
	} {
		if !SpreadsheetFormulaAllowed(formula) {
			t.Fatalf("innocent formula rejected: %q", formula)
		}
	}
}
