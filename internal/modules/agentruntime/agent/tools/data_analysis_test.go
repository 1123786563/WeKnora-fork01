package tools

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildExcelCreateTableSQL_NoSheets(t *testing.T) {
	got := buildExcelCreateTableSQL("tbl", "/tmp/data.xlsx", nil)
	want := `CREATE TABLE "tbl" AS SELECT * FROM read_xlsx('/tmp/data.xlsx', header=true, all_varchar=true)`
	if got != want {
		t.Fatalf("mismatch.\n got: %s\nwant: %s", got, want)
	}
}

func TestBuildExcelCreateTableSQL_SingleSheetTagsSource(t *testing.T) {
	got := buildExcelCreateTableSQL("tbl", "/tmp/data.xlsx", []string{"Sheet1"})

	// Must use read_xlsx (excel extension) with explicit sheet param.
	if !strings.Contains(got, "FROM read_xlsx('/tmp/data.xlsx', sheet = 'Sheet1', header=true, all_varchar=true)") {
		t.Fatalf("expected read_xlsx with sheet param, got: %s", got)
	}
	// Must tag the source sheet name via the synthetic column so downstream
	// SQL behaves consistently between single- and multi-sheet workbooks.
	if !strings.Contains(got, "'Sheet1' AS "+excelSheetNameColumn) {
		t.Fatalf("expected sheet-name column, got: %s", got)
	}
}

func TestBuildExcelCreateTableSQL_MultiSheetUsesUnionAllByName(t *testing.T) {
	got := buildExcelCreateTableSQL("tbl", "/tmp/data.xlsx", []string{"Sheet1", "Sheet2", "报表"})

	// Each sheet must appear as a SELECT reading that specific sheet, and
	// the __sheet_name column must carry its name for per-sheet filtering.
	for _, sheet := range []string{"Sheet1", "Sheet2", "报表"} {
		needleRead := "FROM read_xlsx('/tmp/data.xlsx', sheet = '" + sheet + "', header=true, all_varchar=true)"
		needleTag := "'" + sheet + "' AS " + excelSheetNameColumn
		if !strings.Contains(got, needleRead) {
			t.Fatalf("missing read_xlsx for sheet %q in:\n%s", sheet, got)
		}
		if !strings.Contains(got, needleTag) {
			t.Fatalf("missing __sheet_name tag for sheet %q in:\n%s", sheet, got)
		}
	}

	// Must combine with UNION ALL BY NAME so schema drift between sheets is
	// tolerated.
	if !strings.Contains(got, "UNION ALL BY NAME") {
		t.Fatalf("expected UNION ALL BY NAME in multi-sheet SQL, got:\n%s", got)
	}

	// Exactly N-1 UNIONs for N sheets.
	if strings.Count(got, "UNION ALL BY NAME") != 2 {
		t.Fatalf("expected 2 UNION ALL BY NAME separators, got %d in:\n%s",
			strings.Count(got, "UNION ALL BY NAME"), got)
	}
}

func TestBuildExcelCreateTableSQL_EscapesSingleQuotes(t *testing.T) {
	// Sheet name and file path both contain single quotes, which must be
	// doubled to produce a valid SQL literal.
	sheets := []string{"Jo's data"}
	got := buildExcelCreateTableSQL("tbl", "/tmp/O'Brien/data.xlsx", sheets)

	if !strings.Contains(got, "sheet = 'Jo''s data'") {
		t.Fatalf("sheet name was not escaped, got:\n%s", got)
	}
	if !strings.Contains(got, "read_xlsx('/tmp/O''Brien/data.xlsx'") {
		t.Fatalf("file path was not escaped, got:\n%s", got)
	}
	if !strings.Contains(got, "'Jo''s data' AS "+excelSheetNameColumn) {
		t.Fatalf("sheet-name literal was not escaped, got:\n%s", got)
	}
}

func TestSqlSingleQuoteEscape(t *testing.T) {
	cases := map[string]string{
		"":               "",
		"no_quote":       "no_quote",
		"a'b":            "a''b",
		"''":             "''''",
		"mix'ed'quote":   "mix''ed''quote",
		"中文 with 'quote": "中文 with ''quote",
	}
	for in, want := range cases {
		if got := sqlSingleQuoteEscape(in); got != want {
			t.Errorf("sqlSingleQuoteEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

// reconcileTestSchema is the schema used by the reconcile tests. The canonical
// column spellings are what model-written identifiers must be corrected to.
func reconcileTestSchema() *TableSchema {
	return &TableSchema{
		TableName: "k_doc",
		Columns: []ColumnInfo{
			{Name: "Order Status"},
			{Name: "Order Note"},
			{Name: "id"},
		},
	}
}

func TestReconcileSQLColumnsWithSchema(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantSQL string
		wantFix []string
	}{
		{
			name:    "case-only mismatch is corrected",
			sql:     `SELECT "orderstatus" FROM dataset`,
			wantSQL: `SELECT "Order Status" FROM dataset`,
			wantFix: []string{`"orderstatus" -> "Order Status"`},
		},
		{
			name:    "spacing mismatch is corrected",
			sql:     `SELECT "Order  Status" FROM dataset`,
			wantSQL: `SELECT "Order Status" FROM dataset`,
			wantFix: []string{`"Order  Status" -> "Order Status"`},
		},
		{
			name:    "full-width space is normalized",
			sql:     "SELECT \"Order　Status\" FROM dataset",
			wantSQL: `SELECT "Order Status" FROM dataset`,
			wantFix: []string{fmt.Sprintf("%q -> %q", "Order　Status", "Order Status")},
		},
		{
			name:    "exact identifier is left alone",
			sql:     `SELECT "Order Status" FROM dataset`,
			wantSQL: `SELECT "Order Status" FROM dataset`,
		},
		{
			name:    "unknown identifier is left alone",
			sql:     `SELECT "no_such_column" FROM dataset`,
			wantSQL: `SELECT "no_such_column" FROM dataset`,
		},
		{
			name:    "table alias and plain identifiers are untouched",
			sql:     `SELECT id, "ORDERNOTE" FROM dataset`,
			wantSQL: `SELECT id, "Order Note" FROM dataset`,
			wantFix: []string{`"ORDERNOTE" -> "Order Note"`},
		},
	}
	for _, tc := range cases {
		got, fixes := reconcileSQLColumnsWithSchema(tc.sql, reconcileTestSchema())
		if got != tc.wantSQL {
			t.Errorf("%s: sql = %s, want %s", tc.name, got, tc.wantSQL)
		}
		if fmt.Sprint(fixes) != fmt.Sprint(tc.wantFix) {
			t.Errorf("%s: fixes = %v, want %v", tc.name, fixes, tc.wantFix)
		}
	}
}

func TestReconcileSQLColumnsWithSchemaEmptySchemaPassthrough(t *testing.T) {
	sql := `SELECT "orderstatus" FROM dataset`
	for _, schema := range []*TableSchema{nil, {TableName: "k_doc"}} {
		got, fixes := reconcileSQLColumnsWithSchema(sql, schema)
		if got != sql || len(fixes) != 0 {
			t.Errorf("schema=%+v: got (%s, %v), want input unchanged with no fixes", schema, got, fixes)
		}
	}
}

// TestReconcileSQLColumnsWithSchemaNeverRewritesLiterals guards #3811: column
// reconciliation may only touch double-quoted identifiers OUTSIDE literals.
// Rewriting inside a literal silently changes query results (wrong rows match,
// projected values mutate) while execution still reports success.
func TestReconcileSQLColumnsWithSchemaNeverRewritesLiterals(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantSQL string
		wantFix []string
	}{
		{
			name:    "ticket repro: WHERE literal keeps its original text",
			sql:     `SELECT "orderstatus" AS status FROM dataset WHERE note = '"orderstatus"'`,
			wantSQL: `SELECT "Order Status" AS status FROM dataset WHERE note = '"orderstatus"'`,
			wantFix: []string{`"orderstatus" -> "Order Status"`},
		},
		{
			name:    "doubled-quote escape inside a single-quoted literal",
			sql:     `SELECT "ordernote" FROM dataset WHERE note = 'it''s "ordernote" ok'`,
			wantSQL: `SELECT "Order Note" FROM dataset WHERE note = 'it''s "ordernote" ok'`,
			wantFix: []string{`"ordernote" -> "Order Note"`},
		},
		{
			name:    "backslash-escaped quote keeps the literal open",
			sql:     `SELECT "ordernote" FROM dataset WHERE note = 'can\'t match "ordernote"'`,
			wantSQL: `SELECT "Order Note" FROM dataset WHERE note = 'can\'t match "ordernote"'`,
			wantFix: []string{`"ordernote" -> "Order Note"`},
		},
		{
			name:    "anonymous dollar-quoted literal",
			sql:     `SELECT $$"orderstatus"$$, "ordernote" FROM dataset`,
			wantSQL: `SELECT $$"orderstatus"$$, "Order Note" FROM dataset`,
			wantFix: []string{`"ordernote" -> "Order Note"`},
		},
		{
			name:    "named dollar-quote tag",
			sql:     `SELECT $tag$"orderstatus"$tag$ FROM dataset WHERE "ordernote" IS NOT NULL`,
			wantSQL: `SELECT $tag$"orderstatus"$tag$ FROM dataset WHERE "Order Note" IS NOT NULL`,
			wantFix: []string{`"ordernote" -> "Order Note"`},
		},
		{
			name:    "dollar-quoted literal nests other dollar quotes verbatim",
			sql:     `SELECT $a$"orderstatus" $b$"x"$b$ $a$, "ordernote" FROM dataset`,
			wantSQL: `SELECT $a$"orderstatus" $b$"x"$b$ $a$, "Order Note" FROM dataset`,
			wantFix: []string{`"ordernote" -> "Order Note"`},
		},
		{
			name:    "$1 parameter is not mistaken for a dollar quote",
			sql:     `SELECT "ordernote" FROM dataset WHERE note = $1`,
			wantSQL: `SELECT "Order Note" FROM dataset WHERE note = $1`,
			wantFix: []string{`"ordernote" -> "Order Note"`},
		},
		{
			name:    "projected string value is never rewritten",
			sql:     `SELECT '"orderstatus"' AS v FROM dataset`,
			wantSQL: `SELECT '"orderstatus"' AS v FROM dataset`,
		},
		{
			name:    "literal containing the exact canonical column name",
			sql:     `SELECT "orderstatus" FROM dataset WHERE note = 'Order Status'`,
			wantSQL: `SELECT "Order Status" FROM dataset WHERE note = 'Order Status'`,
			wantFix: []string{`"orderstatus" -> "Order Status"`},
		},
	}
	for _, tc := range cases {
		got, fixes := reconcileSQLColumnsWithSchema(tc.sql, reconcileTestSchema())
		if got != tc.wantSQL {
			t.Errorf("%s:\n got: %s\nwant: %s", tc.name, got, tc.wantSQL)
		}
		if fmt.Sprint(fixes) != fmt.Sprint(tc.wantFix) {
			t.Errorf("%s: fixes = %v, want %v", tc.name, fixes, tc.wantFix)
		}
	}
}
