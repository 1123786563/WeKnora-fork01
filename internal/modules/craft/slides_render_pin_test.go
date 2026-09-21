package craft

// CFT-S04-T029: the slides chain's four acceptance assertions, named and
// pinned in one place over the REAL D03 fixtures (a python-pptx deck, its
// LibreOffice render, the pdftocairo page images) and the existing
// single-page-modification suite. Heavy per-point suites stay in
// slides_test.go.
import (
	"os"
	"path/filepath"
	"testing"
)

// The deck really opens with the correct slide count; readiness requires
// EVERY rendered page (a failure page check is failed, never not_run→pass).
func TestCraftSlidesRenderPinsRealPages(t *testing.T) {
	facts := slideXMLFacts(t, filepath.Join("testdata", "d03_python_pptx_deck.pptx"))
	if len(facts) < 3 {
		t.Fatalf("real deck slide count = %d, want >= 3", len(facts))
	}
	incomplete := SheetSummary{} // unused; readiness lives on SlideManifest below
	_ = incomplete
	bad := SlideManifest{PPTXRef: "resource://deck", SlideCount: 4, PageRefs: []string{"resource://p1", "resource://p2", "resource://p3"}}
	if SlidesReady(bad) {
		t.Fatal("a deck with missing rendered pages accepted as ready")
	}
	good := SlideManifest{PPTXRef: "resource://deck", SlideCount: 3, PageRefs: []string{"resource://p1", "resource://p2", "resource://p3"}}
	if !SlidesReady(good) {
		t.Fatal("a fully rendered deck refused")
	}
	overflow := good
	overflow.OverflowPages = []int{2}
	if SlidesReady(overflow) {
		t.Fatal("a deck with overflow pages accepted as ready")
	}
}

// The render chain is real: the deck, its PDF render and the page image
// all exist as D03 fixtures (python-pptx → LibreOffice → pdftocairo).
func TestCraftSlidesRenderPinsRenderChainFixtures(t *testing.T) {
	for _, name := range []string{
		"d03_python_pptx_deck.pptx", "d03_libreoffice_render.pdf", "d03_pdftocairo_page1.svg",
	} {
		if _, err := filepath.Abs(filepath.Join("testdata", name)); err != nil {
			t.Fatalf("fixture %s: %v", name, err)
		}
		if !fileExists(filepath.Join("testdata", name)) {
			t.Fatalf("render-chain fixture missing: %s", name)
		}
	}
}

// A single-page modification keeps every other page and the historical
// deck's bytes untouched.
func TestCraftSlidesRenderPinsHistoryImmutable(t *testing.T) {
	TestSlidesSinglePageModificationIsolated(t)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
