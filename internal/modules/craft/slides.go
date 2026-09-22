// D03 演示稿与分页预览 — slides rules.
//
// This file holds the stable slide-deck contracts of the craft package:
// the readiness rule (a deck is only ready when EVERY slide has a rendered
// page — producing the PPTX ZIP alone is never visual acceptance), the
// default limits (30-page cap, 12pt readable floor), the page-image naming
// contract, and the manifest contract that gates the slides kind on
// render + pages + sources ALL passing. The heavy lifting (deck creation,
// PDF rendering, page-image export) belongs to the craft-slides skill
// inside the sandbox image: python-pptx writes the deck, LibreOffice
// headless renders the PDF, pdftocairo exports every page; a page that was
// never rendered keeps the kind closed, exactly like an un-recalculated
// formula did for spreadsheets.
package craft

import (
	"fmt"
	"strings"
)

// KindSlides names the slide-deck artwork kind (request.go's closed kind
// set carries the same literal).
const KindSlides = "slides"

// The version-relative artifact paths the craft-slides skill must produce
// in one delegation's output directory. report.pptx is the deliverable,
// report.pdf the print-fidelity render, pages/page-N.svg the per-page
// preview images and preview.json the controlled server-converted preview
// data (the browser never parses the deck).
const (
	SlidesPPTXPath    = "report.pptx"
	SlidesPDFPath     = "report.pdf"
	SlidesPagesDir    = "pages"
	SlidesPreviewPath = "preview.json"
)

// Slides gate check names, extending the version checks (build, entry,
// preview) with the facts only the slides pipeline can observe: the deck
// was really rendered to PDF + page images, every page passed the
// count/boundary/font/image checks, and every citation resolves to a real
// knowledge source. A fourth check "visual" stays honestly not_run for
// human acceptance — it never gates the machine pass.
const (
	CheckRender  = "render"
	CheckPages   = "pages"
	CheckSources = "sources"
)

// Default slides limits (brief Step 4). MaxSlidesPages bounds one deck
// before any rendering work starts, so an oversized outline fails fast
// with a recorded OverflowPages list instead of exhausting the sandbox;
// MinSlidesFontPt is the readable floor the rendered pages are checked
// against (smaller body text is an unreadable deck, not a pass).
const (
	MaxSlidesPages  = 30
	MinSlidesFontPt = 12
)

// SlidesPagePath returns the version-relative path of page index's
// rendered page image (1-based).
func SlidesPagePath(index int) string {
	return fmt.Sprintf("%s/page-%d.svg", SlidesPagesDir, index)
}

// SlidesPageRef returns the resource ref of page index's rendered image —
// the form SlideManifest.PageRefs carries.
func SlidesPageRef(index int) string {
	return "resource://" + SlidesPagePath(index)
}

// SlideManifest is the manifest.json the craft-slides skill writes next to
// report.pptx. The four core fields are the brief's verbatim contract:
// PPTXRef points at the deck, PageRefs at every rendered page image,
// SlideCount counts the deck's slides and OverflowPages lists the pages
// beyond the cap that were NOT rendered (recorded so the main agent can
// fix the outline; their presence keeps the kind closed). The remaining
// fields carry the gate facts: the deliverable paths, the per-page
// summaries, the deck's source registry and the machine checks. PPTX
// source file, rendered PDF and per-page previews all belong to the SAME
// immutable version.
type SlideManifest struct {
	// Core readiness contract (brief Interfaces, verbatim field set).
	PPTXRef       string   `json:"pptx_ref"`
	PageRefs      []string `json:"page_refs"`
	SlideCount    int      `json:"slide_count"`
	OverflowPages []int    `json:"overflow_pages"`

	// Gate facts.
	Kind        string             `json:"kind"`
	PPTXPath    string             `json:"pptx"`
	PDFPath     string             `json:"pdf"`
	PreviewPath string             `json:"preview"`
	Pages       []SlidePageSummary `json:"pages"`
	Sources     []SlideSource      `json:"sources"`
	Checks      []Check            `json:"checks"`
}

// SlidePageSummary is one slide's verification summary. Sources carries the
// knowledge citation ids this slide's claims came from (speaker notes or
// the trailing sources slide must repeat them), MinFontPt the smallest
// font size on the page and Images the pictures the page declares (a
// declared chart that did not land in the deck is a failed pages check,
// never a silent gap).
type SlidePageSummary struct {
	Index     int      `json:"index"`
	Title     string   `json:"title"`
	Notes     string   `json:"notes"`
	Sources   []string `json:"sources"`
	MinFontPt int      `json:"min_font_pt"`
	Images    int      `json:"images"`
}

// SlideSource is one entry of the deck's source registry: the knowledge
// citation id (kc_…, C01) and its human title. Pages cite these ids; the
// trailing sources slide lists them so a reader can trace every claim.
type SlideSource struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SlidesReady reports whether a deck may ship: the PPTX and EVERY page
// image are resource refs, the rendered page set covers exactly the slide
// count, at least one slide exists and nothing overflowed the cap. A deck
// whose pages were not all rendered (or that carries overflow pages the
// main agent still has to fix) is not ready — the PPTX ZIP alone is never
// visual acceptance.
func SlidesReady(m SlideManifest) bool {
	if !strings.HasPrefix(m.PPTXRef, "resource://") || m.SlideCount < 1 || len(m.PageRefs) != m.SlideCount || len(m.OverflowPages) > 0 {
		return false
	}
	for _, p := range m.PageRefs {
		if !strings.HasPrefix(p, "resource://") {
			return false
		}
	}
	return true
}

// ValidateSlidesManifest is the slides kind's gate. Beyond the SlidesReady
// core it enforces the artifact contract: the fixed deliverable paths, the
// page cap, page summaries that line up with the deck (indices 1..N,
// named, fonts at or above the readable floor), citations that resolve to
// the deck's source registry, and the three machine checks — render,
// pages, sources — all passed. A "visual" check may honestly stay not_run:
// aesthetics are human acceptance, and this gate never fakes them.
func ValidateSlidesManifest(m SlideManifest) error {
	if m.Kind != KindSlides {
		return fmt.Errorf("%w: slides manifest kind is %q", ErrInvalidInput, m.Kind)
	}
	if m.PPTXPath != SlidesPPTXPath {
		return fmt.Errorf("%w: slides deliverable path is %q, want %q", ErrInvalidInput, m.PPTXPath, SlidesPPTXPath)
	}
	if m.PDFPath != SlidesPDFPath {
		return fmt.Errorf("%w: slides render path is %q, want %q", ErrInvalidInput, m.PDFPath, SlidesPDFPath)
	}
	if m.PreviewPath != SlidesPreviewPath {
		return fmt.Errorf("%w: slides preview path is %q, want %q", ErrInvalidInput, m.PreviewPath, SlidesPreviewPath)
	}
	if !SlidesReady(m) {
		if len(m.OverflowPages) > 0 {
			return fmt.Errorf("%w: %d pages overflow the %d-page cap (pages %v): split or trim the outline and re-run", ErrInvalidInput, len(m.OverflowPages), MaxSlidesPages, m.OverflowPages)
		}
		return fmt.Errorf("%w: rendered pages do not cover the deck (%d refs for %d slides)", ErrInvalidInput, len(m.PageRefs), m.SlideCount)
	}
	if m.SlideCount > MaxSlidesPages {
		return fmt.Errorf("%w: %d slides exceed the %d-page cap", ErrInvalidInput, m.SlideCount, MaxSlidesPages)
	}
	// Every rendered page must be the canonical page path of its position.
	for i, ref := range m.PageRefs {
		if ref != SlidesPageRef(i+1) {
			return fmt.Errorf("%w: page ref %d is %q, want %q", ErrInvalidInput, i+1, ref, SlidesPageRef(i+1))
		}
	}
	if len(m.Pages) != m.SlideCount {
		return fmt.Errorf("%w: %d page summaries for %d slides", ErrInvalidInput, len(m.Pages), m.SlideCount)
	}
	ids := make(map[string]struct{}, len(m.Sources))
	for _, s := range m.Sources {
		if strings.TrimSpace(s.ID) == "" {
			return fmt.Errorf("%w: source registry entry with a blank id", ErrInvalidInput)
		}
		if _, dup := ids[s.ID]; dup {
			return fmt.Errorf("%w: duplicate source id %q", ErrInvalidInput, s.ID)
		}
		ids[s.ID] = struct{}{}
	}
	for i, p := range m.Pages {
		if p.Index != i+1 {
			return fmt.Errorf("%w: page summary %d has index %d", ErrInvalidInput, i+1, p.Index)
		}
		if strings.TrimSpace(p.Title) == "" {
			return fmt.Errorf("%w: page %d has a blank title", ErrInvalidInput, p.Index)
		}
		if p.MinFontPt < MinSlidesFontPt {
			return fmt.Errorf("%w: page %d smallest font is %dpt, floor is %dpt", ErrInvalidInput, p.Index, p.MinFontPt, MinSlidesFontPt)
		}
		for _, c := range p.Sources {
			if _, ok := ids[c]; !ok {
				return fmt.Errorf("%w: page %d cites unknown source %q", ErrInvalidInput, p.Index, c)
			}
		}
	}
	statuses := make(map[string]string, len(m.Checks))
	for _, c := range m.Checks {
		statuses[c.Name] = c.Status
	}
	for _, name := range []string{CheckRender, CheckPages, CheckSources} {
		if statuses[name] != CheckPassed {
			return fmt.Errorf("%w: slides gate check %q is %q, want %q", ErrInvalidInput, name, statuses[name], CheckPassed)
		}
	}
	return nil
}
