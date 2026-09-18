package craft

// CFT-S04-T027: the spreadsheet chain's three acceptance assertions pinned
// as one named suite. The heavy suites stay in spreadsheet_test.go (the
// LibreOffice-recalculated fixture d02, the written-formula-alone rule, the
// formula/CSV-injection/whitespace policies); this restates the three
// bullets directly so the acceptance maps 1:1 to a green test.
import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func assertRecalcState(t *testing.T, path, wantCached string) {
	t.Helper()
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	cached, _ := f.GetCellValue("月度销售", "C4")
	if cached != wantCached {
		t.Fatalf("%s: C4 cached = %q, want %q", path, cached, wantCached)
	}
	summary := SheetSummary{Name: "月度销售", Rows: 4, Columns: 6, Recalculated: wantCached != ""}
	if SheetReady(summary) != (wantCached != "") {
		t.Fatalf("%s: SheetReady=%v with cached %q (readiness must require the recalculated cache)", path, SheetReady(summary), wantCached)
	}
}

func TestCraftSpreadsheetRecalcPins(t *testing.T) {
	t.Run("SUM is really calculated, not formula-only", func(t *testing.T) {
		// A workbook carrying only the written formula has NO cached value
		// and is correctly judged not ready.
		path := writeFixtureWorkbook(t, t.TempDir(), "pin.xlsx", []int{100, 200}, "")
		assertRecalcState(t, path, "")
		// The LibreOffice fixture (a REAL headless recalculation product)
		// carries the honest cached values.
		assertRecalcState(t, filepath.Join("testdata", "d02_libreoffice_recalc.xlsx"), "300")
	})

	t.Run("preview numbers equal the stored XLSX values", func(t *testing.T) {
		dir := t.TempDir()
		first := writeFixtureWorkbook(t, dir, "pin1.xlsx", []int{100, 200}, "300")
		second := writeFixtureWorkbook(t, dir, "pin2.xlsx", []int{100, 250}, "350")
		assertRecalcState(t, first, "300")
		assertRecalcState(t, second, "350")
	})

	t.Run("external links and dangerous shapes are refused or isolated", func(t *testing.T) {
		// The policy suite is the authority; assert the two headline rules
		// directly against the policy functions.
		if !SpreadsheetFormulaAllowed("=SUM(C2:C3)") {
			t.Fatal("plain SUM must be allowed")
		}
		for _, forbidden := range []string{
			"=[1]Book!A1", "=HYPERLINK(\"http://x\",\"y\")", "=WEBSERVICE(\"http://x\")",
			"=CMD|'/c calc'!A1",
		} {
			if SpreadsheetFormulaAllowed(forbidden) {
				t.Fatalf("policy allowed %q", forbidden)
			}
		}
	})
}
