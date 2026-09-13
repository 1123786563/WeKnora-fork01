// D01 报告文档与 DOCX 导出 — document rules.
//
// This file holds the stable document contracts of the craft package: the
// core readiness rule (a document round ships BOTH the editable Markdown
// source and the SAME-RUN DOCX export — one without the other is not a
// document round), the fixed deliverable paths, the citation policy (every
// citation is a C01 knowledge citation id with the kc_ prefix; invented
// sources never pass the gate), and the manifest contract that gates the
// document kind on 生成/修改/预览/导出 (generate/modify/preview/export) ALL
// passing. The heavy lifting (Markdown authoring, DOCX generation and the
// OOXML read-back) belongs to the craft-document skill inside the sandbox
// image: the Markdown is authored as the source, python-docx renders the
// DOCX from that same source in the same round, and only a DOCX whose
// OOXML was really opened and checked against the Markdown counts as an
// export. PDF is explicitly NOT promised.
package craft

import (
	"fmt"
	"regexp"
	"strings"
)

// KindDocument names the report-document artwork kind (request.go's closed
// kind set carries the same literal).
const KindDocument = "document"

// The version-relative artifact paths the craft-document skill must produce
// in one delegation's output directory. report.md is the editable source a
// later round modifies; report.docx is the SAME-RUN export of exactly that
// source; both enter the immutable version's files.
const (
	DocumentMarkdownPath = "report.md"
	DocumentDOCXPath     = "report.docx"
)

// Document gate check names (brief Step 7's four items: 生成/修改/预览/导出).
// generate proves the Markdown source was really authored (title, abstract,
// sections, a table and the sources section); modify proves the source is
// the continuation point — the numbers the sections carry are exactly what
// the DOCX reproduces, so a later round edits the Markdown and regenerates
// without drifting; preview proves the document view's data (the Markdown
// plus the manifest) is loadable and consistent; export proves the stored
// DOCX was opened as a real OOXML package whose paragraphs/tables match the
// Markdown's sections, numbers and citations.
const (
	CheckGenerate = "generate"
	CheckModify   = "modify"
	CheckExport   = "export"
)

// Default document limits. They bound what one delegation accepts before
// any generation work starts so an oversized outline fails fast with a
// failed status instead of exhausting the sandbox.
const (
	MaxDocumentHeadings    = 200
	MaxDocumentCitations   = 200
	MaxDocumentSourceBytes = 20 << 20 // 20 MiB, matching the per-file input cap
)

// documentCitationPattern is the exact shape of a C01 knowledge citation id
// (knowledge.go's KnowledgeCitationID): the kc_ prefix followed by the 24
// lowercase hex characters of its truncated digest. A citation that does
// not match was invented, never resolved from staged knowledge material.
var documentCitationPattern = regexp.MustCompile("^kc_[0-9a-f]{24}$")

// DocumentCitationAllowed reports whether one citation id is a real C01
// knowledge citation id. The document kind never accepts hand-made source
// references: every claim must trace to material the run actually staged.
func DocumentCitationAllowed(id string) bool {
	return documentCitationPattern.MatchString(id)
}

// DocumentManifest is the manifest.json the craft-document skill writes
// next to report.docx. The four core fields are the brief's verbatim
// contract: MarkdownRef points at the editable source, DOCXRef at the
// same-run export, Headings lists the document's section headings and
// CitationIDs the knowledge citation ids the document cites. The remaining
// fields carry the gate facts: the deliverable paths and the four machine
// checks. Markdown and DOCX belong to the SAME immutable version.
type DocumentManifest struct {
	// Core readiness contract (brief Interfaces, verbatim field set).
	MarkdownRef string   `json:"markdown_ref"`
	DOCXRef     string   `json:"docx_ref"`
	Headings    []string `json:"headings"`
	CitationIDs []string `json:"citation_ids"`

	// Gate facts.
	Kind         string  `json:"kind"`
	MarkdownPath string  `json:"markdown"`
	DOCXPath     string  `json:"docx"`
	Checks       []Check `json:"checks"`
}

// ValidateDocument is the brief's Step 3 verbatim core rule: a document
// round must carry a resource-ref Markdown source, a resource-ref DOCX
// export of the same round, and at least one heading. Anything less is
// ErrInvalidInput.
func ValidateDocument(m DocumentManifest) error {
	if !strings.HasPrefix(m.MarkdownRef, "resource://") || !strings.HasPrefix(m.DOCXRef, "resource://") || len(m.Headings) == 0 {
		return ErrInvalidInput
	}
	return nil
}

// ValidateDocumentManifest is the document kind's gate. Beyond the
// ValidateDocument core it enforces the artifact contract: the fixed
// deliverable paths, non-blank unique headings within the cap, citation
// ids that are ALL real C01 knowledge ids (kc_ + 24 hex — an invented
// source keeps the kind closed), and the four gate checks — generate,
// modify, preview, export — all passed. Only verified DOCX and Markdown
// are claimed; PDF is not part of this contract.
func ValidateDocumentManifest(m DocumentManifest) error {
	if m.Kind != KindDocument {
		return fmt.Errorf("%w: document manifest kind is %q", ErrInvalidInput, m.Kind)
	}
	if m.MarkdownPath != DocumentMarkdownPath {
		return fmt.Errorf("%w: document markdown path is %q, want %q", ErrInvalidInput, m.MarkdownPath, DocumentMarkdownPath)
	}
	if m.DOCXPath != DocumentDOCXPath {
		return fmt.Errorf("%w: document docx path is %q, want %q", ErrInvalidInput, m.DOCXPath, DocumentDOCXPath)
	}
	if err := ValidateDocument(m); err != nil {
		return fmt.Errorf("%w: document needs both the markdown source and the docx export: %v", ErrInvalidInput, err)
	}
	if len(m.Headings) > MaxDocumentHeadings {
		return fmt.Errorf("%w: %d headings exceed the %d-heading cap", ErrInvalidInput, len(m.Headings), MaxDocumentHeadings)
	}
	seenHeadings := make(map[string]struct{}, len(m.Headings))
	for _, h := range m.Headings {
		if strings.TrimSpace(h) == "" {
			return fmt.Errorf("%w: document manifest carries a blank heading", ErrInvalidInput)
		}
		if _, dup := seenHeadings[h]; dup {
			return fmt.Errorf("%w: duplicate heading %q", ErrInvalidInput, h)
		}
		seenHeadings[h] = struct{}{}
	}
	if len(m.CitationIDs) > MaxDocumentCitations {
		return fmt.Errorf("%w: %d citations exceed the %d-citation cap", ErrInvalidInput, len(m.CitationIDs), MaxDocumentCitations)
	}
	seenCitations := make(map[string]struct{}, len(m.CitationIDs))
	for _, id := range m.CitationIDs {
		if !DocumentCitationAllowed(id) {
			return fmt.Errorf("%w: citation %q is not a C01 knowledge citation id (kc_ + 24 hex); invented sources never pass", ErrInvalidInput, id)
		}
		if _, dup := seenCitations[id]; dup {
			return fmt.Errorf("%w: duplicate citation %q", ErrInvalidInput, id)
		}
		seenCitations[id] = struct{}{}
	}
	statuses := make(map[string]string, len(m.Checks))
	for _, c := range m.Checks {
		statuses[c.Name] = c.Status
	}
	for _, name := range []string{CheckGenerate, CheckModify, CheckPreview, CheckExport} {
		if statuses[name] != CheckPassed {
			return fmt.Errorf("%w: document gate check %q is %q, want %q", ErrInvalidInput, name, statuses[name], CheckPassed)
		}
	}
	return nil
}
