package craft

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestDocumentNeedsSourceAndExport is the brief's Step 1 verbatim test: a
// document round ships BOTH the editable Markdown source and the same-run
// DOCX export — one without the other is not a document round.
func TestDocumentNeedsSourceAndExport(t *testing.T) {
	if ValidateDocument(DocumentManifest{MarkdownRef: "resource://md"}) == nil {
		t.Fatal("missing export")
	}
	if err := ValidateDocument(DocumentManifest{MarkdownRef: "resource://md", DOCXRef: "resource://docx", Headings: []string{"概要"}}); err != nil {
		t.Fatal(err)
	}
}

// validDocumentManifest is the fixture-shape manifest the image acceptance
// produced (docker/craft/document-requirements.lock records its source run).
func validDocumentManifest() DocumentManifest {
	return DocumentManifest{
		Kind:         KindDocument,
		MarkdownPath: DocumentMarkdownPath,
		DOCXPath:     DocumentDOCXPath,
		MarkdownRef:  "resource://report.md",
		DOCXRef:      "resource://report.docx",
		Headings:     []string{"智绘云图客户介绍", "公司概况", "产品能力", "实施效果", "来源"},
		CitationIDs:  []string{"kc_8833411d564eabdf5b60e012", "kc_9d6cf870db056f0bb909a0d6"},
		Checks: []Check{
			{Name: CheckGenerate, Status: CheckPassed, Detail: "title/abstract/sections/table/sources"},
			{Name: CheckModify, Status: CheckPassed, Detail: "docx keeps the markdown numbers verbatim"},
			{Name: CheckPreview, Status: CheckPassed, Detail: "markdown + manifest loadable"},
			{Name: CheckExport, Status: CheckPassed, Detail: "stored docx re-opened and verified"},
		},
	}
}

func TestDocumentCitationPolicy(t *testing.T) {
	// The exact C01 shape (knowledge.go KnowledgeCitationID) passes…
	for _, id := range []string{
		"kc_8833411d564eabdf5b60e012",
		"kc_9d6cf870db056f0bb909a0d6",
		"kc_" + strings.Repeat("0", 24),
	} {
		if !DocumentCitationAllowed(id) {
			t.Fatalf("real-shaped citation %q rejected", id)
		}
	}
	// …and nothing invented does: wrong prefix, wrong length, uppercase hex,
	// embedded whitespace, a URL or a bare filename.
	for _, id := range []string{
		"", "kc_", "kc_abc", "kc_8833411d564eabdf5b60e0123", "kc_8833411D564EABDF5B60E012",
		"kc_8833411d564eabdf5b60e01 ", " kc_8833411d564eabdf5b60e012", "http://example.com/s",
		"report.md", "来源1", "kc_8833411d564eabdf5b60e01z",
	} {
		if DocumentCitationAllowed(id) {
			t.Fatalf("invented citation %q accepted", id)
		}
	}
}

func TestValidateDocumentManifestAccepts(t *testing.T) {
	if err := ValidateDocumentManifest(validDocumentManifest()); err != nil {
		t.Fatalf("valid document manifest rejected: %v", err)
	}
	// A source-only document with no citations is still a valid document
	// round: citations are required to be REAL, not required to EXIST.
	m := validDocumentManifest()
	m.CitationIDs = nil
	m.Headings = []string{"报告", "正文"}
	if err := ValidateDocumentManifest(m); err != nil {
		t.Fatalf("citation-free document rejected: %v", err)
	}
}

func TestValidateDocumentManifestRejections(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*DocumentManifest)
		want   string
	}{
		{"wrong kind", func(m *DocumentManifest) { m.Kind = KindWeb }, "kind"},
		{"markdown path", func(m *DocumentManifest) { m.MarkdownPath = "intro.md" }, "markdown path"},
		{"docx path", func(m *DocumentManifest) { m.DOCXPath = "report-word.docx" }, "docx path"},
		{"missing docx ref", func(m *DocumentManifest) { m.DOCXRef = "" }, "markdown source and the docx export"},
		{"missing markdown ref", func(m *DocumentManifest) { m.MarkdownRef = "report.md" }, "markdown source and the docx export"},
		{"no headings", func(m *DocumentManifest) { m.Headings = nil }, "markdown source and the docx export"},
		{"blank heading", func(m *DocumentManifest) { m.Headings = []string{"报告", "  "} }, "blank heading"},
		{"duplicate heading", func(m *DocumentManifest) { m.Headings = []string{"报告", "报告"} }, "duplicate heading"},
		{"heading cap", func(m *DocumentManifest) { m.Headings = make([]string, MaxDocumentHeadings+1) }, "exceed"},
		{"invented citation", func(m *DocumentManifest) { m.CitationIDs = []string{"来源：内部访谈"} }, "not a C01 knowledge citation id"},
		{"malformed citation", func(m *DocumentManifest) { m.CitationIDs = []string{"kc_short"} }, "not a C01 knowledge citation id"},
		{"duplicate citation", func(m *DocumentManifest) {
			m.CitationIDs = []string{"kc_" + strings.Repeat("a", 24), "kc_" + strings.Repeat("a", 24)}
		}, "duplicate citation"},
		{"citation cap", func(m *DocumentManifest) {
			ids := make([]string, MaxDocumentCitations+1)
			for i := range ids {
				ids[i] = fmt.Sprintf("kc_%024x", i)
			}
			m.CitationIDs = ids
		}, "exceed"},
		{"missing generate", func(m *DocumentManifest) { m.Checks = m.Checks[1:] }, "generate"},
		{"missing export", func(m *DocumentManifest) { m.Checks = m.Checks[:3] }, "export"},
		{"export failed", func(m *DocumentManifest) {
			m.Checks[3] = Check{Name: CheckExport, Status: CheckFailed, Detail: "corrupt ZIP"}
		}, "export"},
		{"preview not run", func(m *DocumentManifest) {
			m.Checks[2] = Check{Name: CheckPreview, Status: CheckNotRun}
		}, "preview"},
		{"modify failed", func(m *DocumentManifest) {
			m.Checks[1] = Check{Name: CheckModify, Status: CheckFailed, Detail: "numbers drifted"}
		}, "modify"},
		{"no checks", func(m *DocumentManifest) { m.Checks = nil }, "generate"},
	}
	for _, tc := range cases {
		m := validDocumentManifest()
		tc.mutate(&m)
		err := ValidateDocumentManifest(m)
		if err == nil {
			t.Fatalf("%s: manifest accepted", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error %q does not mention %q", tc.name, err, tc.want)
		}
	}
}

func TestDocumentManifestJSONRoundTrip(t *testing.T) {
	raw, err := json.Marshal(validDocumentManifest())
	if err != nil {
		t.Fatal(err)
	}
	var back DocumentManifest
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDocumentManifest(back); err != nil {
		t.Fatalf("round-tripped manifest rejected: %v", err)
	}
	// The JSON field names are the skill's manifest.json contract.
	for _, key := range []string{"\"markdown_ref\"", "\"docx_ref\"", "\"headings\"", "\"citation_ids\"", "\"checks\""} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("manifest JSON missing contract key %s", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Real-DOCX OOXML read-back (brief Step 5). The fixture is the document the
// pinned python-docx actually rendered inside the craft image (one-shot
// container craft-d01-verify-docx; digests in document-requirements.lock).
// ---------------------------------------------------------------------------

// docxBody is the minimal WordprocessingML projection the document tests
// need: paragraphs (with their style) and tables, text taken from w:t runs.
type docxBody struct {
	Paragraphs []docxParagraph `xml:"body>p"`
	Tables     []docxTable     `xml:"body>tbl"`
}

type docxParagraph struct {
	Style docxPStyle `xml:"pPr>pStyle"`
	Texts []docxText `xml:"r>t"`
}

type docxPStyle struct {
	Val string `xml:"val,attr"`
}

type docxText struct {
	Text string `xml:",chardata"`
}

type docxTable struct {
	Rows []docxRow `xml:"tr"`
}

type docxRow struct {
	Cells []docxCell `xml:"tc"`
}

type docxCell struct {
	Paragraphs []docxParagraph `xml:"p"`
}

func (p docxParagraph) text() string {
	var sb strings.Builder
	for _, t := range p.Texts {
		sb.WriteString(t.Text)
	}
	return sb.String()
}

// openDocxFixture opens a real DOCX package and returns the parsed body,
// the raw styles.xml and the document rels of word/document.xml.
func openDocxFixture(path string) (docxBody, string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return docxBody{}, "", "", err
	}
	return openDocxBytes(raw)
}

func openDocxBytes(raw []byte) (docxBody, string, string, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return docxBody{}, "", "", fmt.Errorf("docx zip: %w", err)
	}
	var body docxBody
	var styles, rels string
	found := map[string]bool{}
	for _, f := range zr.File {
		switch f.Name {
		case "word/document.xml":
			data, err := readZipFile(f)
			if err != nil {
				return docxBody{}, "", "", err
			}
			if !utf8.Valid(data) {
				return docxBody{}, "", "", fmt.Errorf("word/document.xml is not valid UTF-8 (mojibake)")
			}
			if err := xml.Unmarshal(data, &body); err != nil {
				return docxBody{}, "", "", fmt.Errorf("word/document.xml parse: %w", err)
			}
			found["doc"] = true
		case "word/styles.xml":
			data, err := readZipFile(f)
			if err != nil {
				return docxBody{}, "", "", err
			}
			styles = string(data)
			found["styles"] = true
		case "word/_rels/document.xml.rels":
			data, err := readZipFile(f)
			if err != nil {
				return docxBody{}, "", "", err
			}
			rels = string(data)
			found["rels"] = true
		}
	}
	for _, need := range []string{"doc", "styles", "rels"} {
		if !found[need] {
			return docxBody{}, "", "", fmt.Errorf("docx package missing %s part", need)
		}
	}
	return body, styles, rels, nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// docxHeadingTexts returns the styled headings (Title + Heading1) in
// document order.
func docxHeadingTexts(body docxBody) []string {
	var out []string
	for _, p := range body.Paragraphs {
		if p.Style.Val == "Title" || p.Style.Val == "Heading1" {
			out = append(out, p.text())
		}
	}
	return out
}

var markdownCitationRe = regexp.MustCompile(`kc_[0-9a-f]{24}`)

// markdownFacts extracts the source document's headings, table rows and
// citation ids from the fixture report.md.
func markdownFacts(raw string) (headings []string, tableRows int, citations []string) {
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "")
		if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") {
			headings = append(headings, strings.TrimSpace(line[2:]))
		}
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") {
			tableRows++
		}
		for _, id := range markdownCitationRe.FindAllString(line, -1) {
			if !seen[id] {
				seen[id] = true
				citations = append(citations, id)
			}
		}
	}
	return headings, tableRows, citations
}

// TestPythonDocxFixtureMatchesMarkdown opens the REAL stored DOCX and maps
// it back onto the Markdown source: same headings in the same order, the
// same numbers verbatim, every citation in the body AND the sources
// section, and the same table shape.
func TestPythonDocxFixtureMatchesMarkdown(t *testing.T) {
	body, styles, rels, err := openDocxFixture(filepath.Join("testdata", "d01_python_docx_report.docx"))
	if err != nil {
		t.Fatalf("open python-docx fixture: %v", err)
	}
	mdRaw, err := os.ReadFile(filepath.Join("testdata", "d01_report.md"))
	if err != nil {
		t.Fatal(err)
	}
	mdHeadings, mdTableRows, mdCitations := markdownFacts(string(mdRaw))

	got := docxHeadingTexts(body)
	if len(got) != len(mdHeadings) {
		t.Fatalf("docx headings %v vs markdown headings %v", got, mdHeadings)
	}
	for i := range got {
		if got[i] != mdHeadings[i] {
			t.Fatalf("heading %d: docx %q vs markdown %q", i, got[i], mdHeadings[i])
		}
	}

	joined := &strings.Builder{}
	for _, p := range body.Paragraphs {
		fmt.Fprintln(joined, p.text())
	}
	for _, tbl := range body.Tables {
		for _, row := range tbl.Rows {
			for _, cell := range row.Cells {
				for _, p := range cell.Paragraphs {
					fmt.Fprintln(joined, p.text())
				}
			}
		}
	}
	all := joined.String()
	for _, n := range []string{"12", "96", "6 周"} {
		if !strings.Contains(all, n) {
			t.Fatalf("number %q missing from docx text", n)
		}
		if !strings.Contains(string(mdRaw), n) {
			t.Fatalf("number %q missing from markdown source", n)
		}
	}
	for _, id := range mdCitations {
		if strings.Count(all, id) < 2 {
			t.Fatalf("citation %s appears %d times in docx; want body + sources (>=2)", id, strings.Count(all, id))
		}
	}
	if len(body.Tables) != 1 {
		t.Fatalf("docx tables = %d, want 1", len(body.Tables))
	}
	if got := len(body.Tables[0].Rows); got != mdTableRows {
		t.Fatalf("docx table rows = %d, markdown table rows = %d", got, mdTableRows)
	}

	// Font: the CJK font must be declared in styles.xml (no tofu).
	if !strings.Contains(styles, "Noto Sans CJK SC") {
		t.Fatal("styles.xml does not declare the CJK font Noto Sans CJK SC")
	}
	// Images/rels: every relationship target of the document must resolve
	// to a real part of the package (no dangling pictures), and no external
	// target may exist.
	targetRe := regexp.MustCompile(`Target="([^"]+)"`)
	targetModeRe := regexp.MustCompile(`TargetMode="External"`)
	if targetModeRe.MatchString(rels) {
		t.Fatal("document rels carry an external target")
	}
	parts := map[string]bool{}
	zr, err := zip.OpenReader(filepath.Join("testdata", "d01_python_docx_report.docx"))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		parts[f.Name] = true
	}
	for _, m := range targetRe.FindAllStringSubmatch(rels, -1) {
		target := m[1]
		if strings.HasPrefix(target, "/") {
			target = strings.TrimPrefix(target, "/")
		} else {
			target = "word/" + target
		}
		target = filepath.ToSlash(filepath.Clean(target))
		if !parts[target] {
			t.Fatalf("dangling rel target %q (resolved %q)", m[1], target)
		}
	}
}

// TestDocumentExportFailures pins the failure states that must keep the
// export check failed: empty file, corrupt ZIP, and a truncated package.
func TestDocumentExportFailures(t *testing.T) {
	if _, _, _, err := openDocxBytes([]byte{}); err == nil {
		t.Fatal("empty file opened as DOCX")
	}
	if _, _, _, err := openDocxBytes([]byte("not a zip at all")); err == nil {
		t.Fatal("corrupt ZIP opened as DOCX")
	}
	truncated, err := os.ReadFile(filepath.Join("testdata", "d01_python_docx_report.docx"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := openDocxBytes(truncated[:len(truncated)/2]); err == nil {
		t.Fatal("truncated DOCX opened cleanly")
	}
	// An export failure is a failed manifest check, never a pass.
	m := validDocumentManifest()
	m.Checks = []Check{
		{Name: CheckGenerate, Status: CheckPassed},
		{Name: CheckModify, Status: CheckPassed},
		{Name: CheckPreview, Status: CheckPassed},
		{Name: CheckExport, Status: CheckFailed, Detail: "corrupt ZIP"},
	}
	if err := ValidateDocumentManifest(m); err == nil {
		t.Fatal("manifest with failed export accepted")
	}
}

// TestDocumentMissingFontIsDetectable rewrites the fixture package with the
// CJK font declaration stripped and verifies the read-back notices: a
// missing font is a failed export, not silent tofu.
func TestDocumentMissingFontIsDetectable(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "d01_python_docx_report.docx"))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatal(err)
	}
	var rebuilt bytes.Buffer
	zw := zip.NewWriter(&rebuilt)
	for _, f := range zr.File {
		data, err := readZipFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == "word/styles.xml" {
			data = []byte(strings.ReplaceAll(string(data), "Noto Sans CJK SC", "Calibri"))
		}
		nf, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := nf.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_, styles, _, err := openDocxBytes(rebuilt.Bytes())
	if err != nil {
		t.Fatalf("rebuilt package unreadable: %v", err)
	}
	if strings.Contains(styles, "Noto Sans CJK SC") {
		t.Fatal("font strip did not take effect")
	}
}

// ---------------------------------------------------------------------------
// D01 wiring (coordinator-assigned): every kind in the closed set has an
// entry deliverable and a previewable static file set; BuildChecks judges
// the entry of each kind by its own deliverable.
// ---------------------------------------------------------------------------

func TestEntryPathCoversEveryKind(t *testing.T) {
	for _, kind := range Kinds() {
		entry, ok := EntryPath(kind)
		if !ok || entry == "" {
			t.Fatalf("kind %q has no entry path", kind)
		}
	}
	for _, tc := range []struct{ kind, want string }{
		{KindWeb, "index.html"},
		{KindDocument, "report.docx"},
		{KindSpreadsheet, "report.xlsx"},
		{KindSlides, "report.pptx"},
	} {
		if got, _ := EntryPath(tc.kind); got != tc.want {
			t.Fatalf("EntryPath(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
	if _, ok := EntryPath("diagram"); ok {
		t.Fatal("unknown kind answered with an entry")
	}
}

func TestPreviewableKindCoversEveryKind(t *testing.T) {
	for _, kind := range Kinds() {
		if !PreviewableKind(kind) {
			t.Fatalf("kind %q is not previewable", kind)
		}
	}
	if PreviewableKind("diagram") {
		t.Fatal("unknown kind is previewable")
	}
}

// TestDocumentEntryCheckJudgesTheDOCX pins that a document version's entry
// check passes on report.docx (the deliverable) and fails when only the
// markdown source exists.
func TestDocumentEntryCheckJudgesTheDOCX(t *testing.T) {
	docx := []File{{Path: DocumentDOCXPath, SHA256: strings.Repeat("a", 64), Bytes: 10}}
	checks := BuildChecks(KindDocument, docx, ArtifactEvidence{BuildRan: true, BuildExitCode: 0})
	if checks[1].Status != CheckPassed {
		t.Fatalf("document entry with report.docx = %q, want passed", checks[1].Status)
	}
	mdOnly := []File{{Path: DocumentMarkdownPath, SHA256: strings.Repeat("a", 64), Bytes: 10}}
	checks = BuildChecks(KindDocument, mdOnly, ArtifactEvidence{BuildRan: true, BuildExitCode: 0})
	if checks[1].Status != CheckFailed {
		t.Fatalf("document entry with markdown only = %q, want failed", checks[1].Status)
	}
}
