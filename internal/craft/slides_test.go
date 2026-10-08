// D03 演示稿与分页预览 — slides rules tests.
//
// The verbatim brief test (Step 1) pins the core contract: a deck whose
// rendered page set does not cover EVERY slide is never ready — producing
// the PPTX ZIP alone is not visual acceptance. The rest of the file
// exercises Steps 4–7: the default limits (30-page cap, readable floor for
// font sizes), the artifact manifest contract (render/pages/sources checks
// all passed, citations traceable to the deck's source registry, overflow
// pages keep the kind closed), and real OOXML/PDF/page-image parsing of the
// fixtures the locked image toolchain actually produced (python-pptx deck,
// LibreOffice-rendered PDF, pdftocairo page SVG) plus the byte-stable
// single-page modification semantics the browser acceptance relies on.
package craft

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Step 1 (verbatim): every rendered page is required
// ---------------------------------------------------------------------------

func TestSlidesNeedEveryRenderedPage(t *testing.T) {
	m := SlideManifest{PPTXRef: "resource://ppt", SlideCount: 2, PageRefs: []string{"resource://p1"}}
	if SlidesReady(m) {
		t.Fatal("missing page accepted")
	}
	m.PageRefs = append(m.PageRefs, "resource://p2")
	if !SlidesReady(m) {
		t.Fatal("complete preview rejected")
	}
}

// ---------------------------------------------------------------------------
// Step 4: default limits
// ---------------------------------------------------------------------------

func TestSlidesDefaultLimits(t *testing.T) {
	if MaxSlidesPages != 30 {
		t.Fatalf("page cap = %d, want 30", MaxSlidesPages)
	}
	if MinSlidesFontPt != 12 {
		t.Fatalf("readable font floor = %d, want 12pt", MinSlidesFontPt)
	}
}

func TestSlidesReadyRejections(t *testing.T) {
	valid := func() SlideManifest {
		return SlideManifest{
			PPTXRef:    "resource://report.pptx",
			SlideCount: 2,
			PageRefs:   []string{"resource://pages/page-1.svg", "resource://pages/page-2.svg"},
		}
	}
	if !SlidesReady(valid()) {
		t.Fatal("complete deck rejected")
	}
	m := valid()
	m.PPTXRef = "report.pptx" // not a resource ref
	if SlidesReady(m) {
		t.Fatal("plain path accepted as pptx ref")
	}
	m = valid()
	m.SlideCount = 0
	if SlidesReady(m) {
		t.Fatal("zero-slide deck accepted")
	}
	m = valid()
	m.PageRefs[1] = "pages/page-2.svg"
	if SlidesReady(m) {
		t.Fatal("plain page path accepted")
	}
	m = valid()
	m.OverflowPages = []int{31, 32} // Step 7: overflow pages block readiness
	if SlidesReady(m) {
		t.Fatal("overflow pages accepted")
	}
}

func TestSlidesPagePaths(t *testing.T) {
	if got := SlidesPagePath(1); got != "pages/page-1.svg" {
		t.Fatalf("page path(1) = %q", got)
	}
	if got := SlidesPagePath(30); got != "pages/page-30.svg" {
		t.Fatalf("page path(30) = %q", got)
	}
	if got := SlidesPageRef(3); got != "resource://pages/page-3.svg" {
		t.Fatalf("page ref(3) = %q", got)
	}
}

// ---------------------------------------------------------------------------
// Step 4/5/7: the manifest contract — render + pages + sources all passed
// ---------------------------------------------------------------------------

func passedSlidesChecks() []Check {
	return []Check{
		{Name: CheckRender, Status: CheckPassed, Detail: "LibreOffice headless rendered report.pdf; pdftocairo exported every page"},
		{Name: CheckPages, Status: CheckPassed, Detail: "5/5 pages bounded, fonts >= 12pt, declared images present"},
		{Name: CheckSources, Status: CheckPassed, Detail: "every page citation resolves to the knowledge source registry"},
		{Name: "visual", Status: CheckNotRun, Detail: "aesthetics/visual hierarchy need human acceptance"},
	}
}

func validSlidesManifest() SlideManifest {
	return SlideManifest{
		Kind:        KindSlides,
		PPTXPath:    SlidesPPTXPath,
		PDFPath:     SlidesPDFPath,
		PreviewPath: SlidesPreviewPath,
		PPTXRef:     "resource://report.pptx",
		PageRefs: []string{
			"resource://pages/page-1.svg", "resource://pages/page-2.svg",
			"resource://pages/page-3.svg", "resource://pages/page-4.svg",
			"resource://pages/page-5.svg",
		},
		SlideCount: 5,
		Pages: []SlidePageSummary{
			{Index: 1, Title: "智绘云图客户方案", Notes: "开场", MinFontPt: 28, Images: 0},
			{Index: 2, Title: "客户背景", Notes: "备注", Sources: []string{"kc_customer"}, MinFontPt: 14},
			{Index: 3, Title: "核心方案", Notes: "来源 kc_sales kc_market", Sources: []string{"kc_sales", "kc_market"}, MinFontPt: 14, Images: 1},
			{Index: 4, Title: "实施计划", Notes: "备注", Sources: []string{"kc_plan"}, MinFontPt: 14},
			{Index: 5, Title: "来源", Notes: "来源页", Sources: []string{"kc_customer", "kc_sales", "kc_market", "kc_plan"}, MinFontPt: 12},
		},
		Sources: []SlideSource{
			{ID: "kc_customer", Title: "客户背景资料"},
			{ID: "kc_sales", Title: "销售数据摘要"},
			{ID: "kc_market", Title: "市场分析摘录"},
			{ID: "kc_plan", Title: "实施计划素材"},
		},
		Checks: passedSlidesChecks(),
	}
}

func TestValidateSlidesManifestAccepts(t *testing.T) {
	m := validSlidesManifest()
	if err := ValidateSlidesManifest(m); err != nil {
		t.Fatalf("valid slides manifest rejected: %v", err)
	}
	if !SlidesReady(slidesCoreOf(m)) {
		t.Fatal("manifest core fields not ready")
	}
}

func TestValidateSlidesManifestRejections(t *testing.T) {
	m := validSlidesManifest()
	m.Kind = "web"
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("foreign kind accepted")
	}

	m = validSlidesManifest()
	m.PPTXPath = "deck.pptx"
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("wrong pptx path accepted")
	}

	m = validSlidesManifest()
	m.PDFPath = ""
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("missing pdf path accepted")
	}

	m = validSlidesManifest()
	m.PPTXRef = "report.pptx"
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("plain pptx ref accepted")
	}

	// The rendered page set must cover every slide — the Step-1 rule, now
	// at manifest level: a missing page, a plain path and a count mismatch
	// all refuse.
	m = validSlidesManifest()
	m.PageRefs = m.PageRefs[:4]
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("missing rendered page accepted")
	}
	m = validSlidesManifest()
	m.PageRefs[2] = "pages/page-3.svg"
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("plain rendered-page path accepted")
	}
	m = validSlidesManifest()
	m.SlideCount = 6
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("slide count mismatch accepted")
	}

	m = validSlidesManifest()
	m.SlideCount = MaxSlidesPages + 1
	m.PageRefs = nil
	for i := 1; i <= m.SlideCount; i++ {
		m.PageRefs = append(m.PageRefs, SlidesPageRef(i))
	}
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("over-cap deck accepted")
	}

	// Step 7: overflow pages are recorded for the main agent to fix and
	// keep the kind closed — never silently dropped.
	m = validSlidesManifest()
	m.OverflowPages = []int{31}
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("overflow pages accepted")
	}

	// Page summaries must line up with the deck: indices 1..N exactly,
	// named, fonts readable.
	m = validSlidesManifest()
	m.Pages[2].MinFontPt = 9
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("unreadable font accepted")
	}
	m = validSlidesManifest()
	m.Pages[2].Index = 6
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("page index mismatch accepted")
	}
	m = validSlidesManifest()
	m.Pages[2].Title = "  "
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("blank page title accepted")
	}

	// Citations must resolve: an unknown source id on any page refuses the
	// manifest (page-level traceability, brief Step 5).
	m = validSlidesManifest()
	m.Pages[2].Sources = []string{"kc_ghost"}
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("unknown citation accepted")
	}
	m = validSlidesManifest()
	m.Sources = m.Sources[:1]
	if err := ValidateSlidesManifest(m); err == nil {
		t.Fatal("citation without registry entry accepted")
	}

	// Machine gate checks must all have run and passed; a visual check
	// stays honestly not_run without failing the machine gate.
	for _, name := range []string{CheckRender, CheckPages, CheckSources} {
		m = validSlidesManifest()
		m.Checks = []Check{{Name: name, Status: CheckFailed}}
		if err := ValidateSlidesManifest(m); err == nil {
			t.Fatalf("failed %s check accepted", name)
		}
		m = validSlidesManifest()
		m.Checks = []Check{{Name: name, Status: CheckNotRun}}
		if err := ValidateSlidesManifest(m); err == nil {
			t.Fatalf("not_run %s check accepted", name)
		}
	}
	m = validSlidesManifest()
	m.Checks = m.Checks[:3] // the not_run visual entry is not machine-gating
	if err := ValidateSlidesManifest(m); err != nil {
		t.Fatalf("manifest without the not_run visual entry rejected: %v", err)
	}
}

// slidesCoreOf projects the manifest onto the verbatim Step-1 struct view.
func slidesCoreOf(m SlideManifest) SlideManifest {
	return SlideManifest{PPTXRef: m.PPTXRef, PageRefs: m.PageRefs, SlideCount: m.SlideCount, OverflowPages: m.OverflowPages}
}

func TestSlidesManifestJSONRoundTrip(t *testing.T) {
	raw := `{"kind": "slides",
		"pptx": "report.pptx",
		"pdf": "report.pdf",
		"preview": "preview.json",
		"pptx_ref": "resource://report.pptx",
		"page_refs": ["resource://pages/page-1.svg", "resource://pages/page-2.svg"],
		"slide_count": 2,
		"overflow_pages": [],
		"pages": [
			{"index": 1, "title": "封面", "notes": "", "sources": [], "min_font_pt": 28, "images": 0},
			{"index": 2, "title": "来源", "notes": "kc_a", "sources": ["kc_a"], "min_font_pt": 12, "images": 0}
		],
		"sources": [{"id": "kc_a", "title": "素材A"}],
		"checks": [
			{"name": "render", "status": "passed", "detail": "soffice --headless + pdftocairo"},
			{"name": "pages", "status": "passed", "detail": ""},
			{"name": "sources", "status": "passed", "detail": ""},
			{"name": "visual", "status": "not_run", "detail": "human acceptance"}
		]
	}`
	var m SlideManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSlidesManifest(m); err != nil {
		t.Fatalf("manifest.json as produced by the skill rejected: %v", err)
	}
	if m.Pages[1].Notes != "kc_a" || m.Sources[0].ID != "kc_a" {
		t.Fatalf("manifest fields lost: %+v", m)
	}
	if !SlidesReady(slidesCoreOf(m)) {
		t.Fatal("round-tripped manifest core not ready")
	}
}

// ---------------------------------------------------------------------------
// Real artifact parsing (storage-level truth, same style as the D02 tests)
// ---------------------------------------------------------------------------

type slideFacts struct {
	Slide     string
	Text      string
	MinSizeCp int
	Pictures  int
	Charts    int
	HasNotes  bool
}

// slideXMLFacts opens a stored PPTX (a real ZIP) and extracts per-slide
// facts: the joined text runs, the smallest font size (centipoints), the
// picture count and whether the slide has stored notes.
func slideXMLFacts(t *testing.T, path string) []slideFacts {
	t.Helper()
	rf, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open pptx %s: %v", path, err)
	}
	defer rf.Close()
	notes := map[string]bool{}
	for _, ent := range rf.File {
		if strings.HasPrefix(ent.Name, "ppt/notesSlides/notesSlide") && strings.HasSuffix(ent.Name, ".xml") {
			notes[ent.Name] = true
		}
	}
	reNum := regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)
	var out []slideFacts
	for _, ent := range rf.File {
		m := reNum.FindStringSubmatch(ent.Name)
		if m == nil {
			continue
		}
		data := slidesReadZipEntry(t, ent)
		notesName := "ppt/notesSlides/notesSlide" + m[1] + ".xml"
		out = append(out, slideFacts{
			Slide:     ent.Name,
			Text:      slidesAllRuns(data),
			MinSizeCp: slidesMinFontSizeCp(data),
			Pictures:  strings.Count(string(data), "<p:pic>"),
			Charts:    strings.Count(string(data), "<p:graphicFrame>"),
			HasNotes:  notes[notesName],
		})
	}
	return out
}

func slidesReadZipEntry(t *testing.T, ent *zip.File) []byte {
	t.Helper()
	rc, err := ent.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func slidesAllRuns(data []byte) string {
	re := regexp.MustCompile(`<a:t>([^<]*)</a:t>`)
	var sb strings.Builder
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		sb.WriteString(m[1])
		sb.WriteString("\n")
	}
	return sb.String()
}

func slidesMinFontSizeCp(data []byte) int {
	re := regexp.MustCompile(`sz="(\d+)"`)
	best := 1 << 30
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		n := 0
		for _, c := range m[1] {
			n = n*10 + int(c-'0')
		}
		if n > 0 && n < best {
			best = n
		}
	}
	if best == 1<<30 {
		return 0
	}
	return best
}

// TestPythonPptxDeckFixture parses the deck the locked python-pptx actually
// wrote inside the craft image (the D03 acceptance run recorded in the
// report): 5 slides, Chinese titles, a chart image on slide 3, speaker
// notes carrying citation ids, and no run below the readable floor.
func TestPythonPptxDeckFixture(t *testing.T) {
	path := filepath.Join("testdata", "d03_python_pptx_deck.pptx")
	facts := slideXMLFacts(t, path)
	if len(facts) != 5 {
		t.Fatalf("slides = %d, want 5", len(facts))
	}
	titles := []string{"客户方案", "背景", "方案", "计划", "来源"}
	for i, f := range facts {
		if !strings.Contains(f.Text, titles[i]) {
			t.Fatalf("slide %d text %q misses %q", i+1, f.Text, titles[i])
		}
		if f.MinSizeCp == 0 || f.MinSizeCp < MinSlidesFontPt*100 {
			t.Fatalf("slide %d min font = %d centipoints, floor %d", i+1, f.MinSizeCp, MinSlidesFontPt*100)
		}
	}
	// The python-pptx deck carries a NATIVE chart (graphicFrame) on page 3;
	// the stdlib mock below embeds a picture instead — both are the page's
	// declared figure, and each fixture pins its own shape.
	if facts[2].Charts < 1 && facts[2].Pictures < 1 {
		t.Fatal("slide 3 chart missing in python-pptx fixture")
	}
	if !facts[2].HasNotes {
		t.Fatal("slide 3 speaker notes missing")
	}
	if !slidesNotesCite(t, path, 3) {
		t.Fatal("slide 3 speaker notes carry no citation id")
	}
}

func slidesNotesCite(t *testing.T, path string, slide int) bool {
	t.Helper()
	rf, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()
	for _, ent := range rf.File {
		if ent.Name == "ppt/notesSlides/notesSlide"+itoa(slide)+".xml" {
			return strings.Contains(string(slidesReadZipEntry(t, ent)), "kc_")
		}
	}
	return false
}

// TestLibreOfficeRenderedPDFFixture checks the PDF the locked LibreOffice
// Impress headless really produced from the python-pptx deck: a valid PDF
// whose page tree counts exactly the 5 slides (missing pages or a corrupt
// file must be a visible render failure, never a silent pass).
func TestLibreOfficeRenderedPDFFixture(t *testing.T) {
	path := filepath.Join("testdata", "d03_libreoffice_render.pdf")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte("%PDF-")) {
		t.Fatal("fixture is not a PDF")
	}
	if pages := slidesPDFPageCount(raw); pages != 5 {
		t.Fatalf("rendered pages = %d, want 5", pages)
	}
}

func slidesPDFPageCount(raw []byte) int {
	// LibreOffice writes the page tree so /Type /Page (without /s) stays
	// countable in the raw bytes; the count is asserted against the
	// producer run, so a producer change fails loudly here.
	re := regexp.MustCompile(`/Type\s*/Page[^s]`)
	return len(re.FindAll(raw, -1))
}

// TestSlidesPageImageFixture checks one page image the locked pdftocairo
// exported from the rendered PDF: a well-formed SVG carrying real glyph
// geometry (the visual preview is a rendered page, never a placeholder).
func TestSlidesPageImageFixture(t *testing.T) {
	path := filepath.Join("testdata", "d03_pdftocairo_page1.svg")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.HasPrefix(strings.TrimSpace(text), "<?xml") || !strings.Contains(text, "<svg") {
		t.Fatal("page image fixture is not an SVG")
	}
	if !strings.Contains(text, "</svg>") || len(raw) < 2000 {
		t.Fatal("page image fixture carries no rendered geometry")
	}
}

// TestSlidesSinglePageModificationIsolated pins the modification semantics
// the browser acceptance relies on: changing ONLY page 3 leaves every other
// slide's stored XML byte-identical, keeps the slide count and keeps the
// old deck's bytes untouched on disk.
func TestSlidesSinglePageModificationIsolated(t *testing.T) {
	dir := t.TempDir()
	first := writeMockDeck(t, dir, "deck1.pptx", "旧结论：分三期交付")
	second := writeMockDeck(t, dir, "deck2.pptx", "新结论：一次交付上线")

	f1 := slideXMLFacts(t, first)
	f2 := slideXMLFacts(t, second)
	if len(f1) != 5 || len(f2) != 5 {
		t.Fatalf("slide counts = %d/%d, want 5/5", len(f1), len(f2))
	}
	for i := range f1 {
		if f1[i].Slide != f2[i].Slide {
			t.Fatalf("slide order changed: %q vs %q", f1[i].Slide, f2[i].Slide)
		}
		changed := f1[i].Text != f2[i].Text
		if i == 2 && !changed {
			t.Fatal("page 3 did not change")
		}
		if i != 2 && changed {
			t.Fatalf("page %d changed unexpectedly: %q vs %q", i+1, f1[i].Text, f2[i].Text)
		}
	}
	// The first deck's bytes stay exactly as written after round two.
	before, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, mockDeckBytes("旧结论：分三期交付")) {
		t.Fatal("old deck bytes changed by a later round")
	}
	// The two decks differ ONLY in slide 3's part; every other stored part
	// is byte-equal.
	parts1, parts2 := mockDeckParts("旧结论：分三期交付"), mockDeckParts("新结论：一次交付上线")
	for name, data := range parts1 {
		if name == "ppt/slides/slide3.xml" {
			if bytes.Equal(data, parts2[name]) {
				t.Fatal("slide 3 identical across rounds")
			}
			continue
		}
		if !bytes.Equal(data, parts2[name]) {
			t.Fatalf("part %q changed across rounds", name)
		}
	}
}

// mockDeckParts builds the deterministic minimal 5-slide OOXML part set the
// mock acceptance generator produces (same storage shape as the e2e
// fixture): slide 3 carries the round-specific conclusion, every other part
// is round-independent, so byte-diffs localize the change to page 3.
func mockDeckParts(conclusion string) map[string][]byte {
	parts := map[string][]byte{}
	parts["[Content_Types].xml"] = []byte(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`)
	slideTexts := []string{
		"智绘云图客户方案",
		"客户背景",
		conclusion,
		"实施计划",
		"来源",
	}
	for i, text := range slideTexts {
		size := 1400
		if i == 0 {
			size = 3200
		}
		parts["ppt/slides/slide"+itoa(i+1)+".xml"] = []byte(
			`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
				"<p:cSld><p:spTree><p:nvGrpSpPr/><p:grpSpPr/><p:sp><p:txBody><a:p><a:r>" +
				`<a:rPr lang="zh-CN" sz="` + itoa(size) + `"/><a:t>` + text + "</a:t>" +
				"</a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>")
	}
	return parts
}

func mockDeckBytes(conclusion string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{
		"[Content_Types].xml",
		"ppt/slides/slide1.xml", "ppt/slides/slide2.xml", "ppt/slides/slide3.xml",
		"ppt/slides/slide4.xml", "ppt/slides/slide5.xml",
	} {
		info := &zip.FileHeader{Name: name, Method: zip.Deflate}
		info.SetModTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		w, err := zw.CreateHeader(info)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(mockDeckParts(conclusion)[name]); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func writeMockDeck(t *testing.T, dir, name, conclusion string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, mockDeckBytes(conclusion), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
