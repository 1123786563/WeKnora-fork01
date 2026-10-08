// T06 网页作品引用事实 — web citation contracts.
//
// Story 10: facts in the generated webpage must link to evidence while
// model inference stays distinguishable from source facts. This file holds
// the stable citation contracts of the craft package: the citation manifest
// one web Run stages beside its entry (bound to the Run's RECORDED actual
// sources, never to hand-made ids), the marker contract the generated page's
// citation view must honor, the canonical offline-safe citation view
// renderer, and the non-leaking placeholder a missing, revoked or
// inaccessible source keeps. The manifest carries only stable citation ids,
// kinds and bounded claim text — never the ref, digest, title or excerpt of
// a restricted original.
package craft

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// WebCitationsPath is the fixed output-relative location of the web citation
// manifest inside one Run's collected output. It enters the immutable
// artifact like every other collected file, so the manifest is stored with
// the version and pinned by the file manifest digest.
const WebCitationsPath = "citations.json"

// WebCitationSchema is the only accepted manifest schema.
const WebCitationSchema = 1

// Bounds of one citation manifest. The claim bound keeps one invented claim
// from smuggling an unrestricted payload into the stored artifact.
const (
	MaxWebCitations          = 200
	MaxWebCitationClaimBytes = 512
)

// Marker attributes of the generated page's citation view. The admission
// gate binds them to the manifest: a page may not display a citation the
// manifest does not declare, and an inference marker never rides the same
// element as a source citation.
const (
	WebCitationFactMarkerAttr      = "data-craft-citation"
	WebCitationInferenceMarkerAttr = "data-craft-inference"
)

// WebCitationStatusUnavailable is the only placeholder status: the source is
// missing, revoked or inaccessible and nothing about it is projected.
const WebCitationStatusUnavailable = "unavailable"

// WebCitationKind distinguishes a source fact from model inference.
type WebCitationKind string

const (
	// WebCitationFact marks a claim backed by one recorded source; its
	// CitationID is that source's stable citation id.
	WebCitationFact WebCitationKind = "fact"
	// WebCitationInference marks a model inference. It never carries a
	// citation id and is never presented as a source fact.
	WebCitationInference WebCitationKind = "inference"
)

func (k WebCitationKind) valid() bool {
	return k == WebCitationFact || k == WebCitationInference
}

// WebCitationEntry is one bounded entry of the citation manifest.
type WebCitationEntry struct {
	Kind       WebCitationKind `json:"kind"`
	CitationID string          `json:"citation_id,omitempty"`
	Claim      string          `json:"claim"`
}

// WebCitationManifest is the citation manifest one web Run stages beside its
// entry file. Entries carry only stable citation ids, kinds and bounded
// claim text; the durable refs, digests and excerpts of the underlying
// sources stay in the Run's knowledge record and are never embedded here.
type WebCitationManifest struct {
	Schema  int                `json:"schema"`
	Lang    string             `json:"lang,omitempty"`
	Entries []WebCitationEntry `json:"entries"`
}

// webCitationPattern is the exact shape of a C01 knowledge citation id
// (knowledge.go's KnowledgeCitationID), shared with the document gate's
// contract: the kc_ prefix followed by 24 lowercase hex characters.
var webCitationPattern = regexp.MustCompile("^kc_[0-9a-f]{24}$")

// WebCitationAllowed reports whether one citation id has the real C01
// knowledge citation id shape. Shape alone never proves the id was staged:
// the admission gate additionally requires it to be among the Run's recorded
// sources, so a fabricated but well-formed id still never passes.
func WebCitationAllowed(id string) bool {
	return webCitationPattern.MatchString(id)
}

// validateWebCitationShape enforces the manifest's closed structural
// contract: schema, entry cap, kinds, claim bounds, the kc_ shape of fact
// ids and the rule that an inference entry never carries a citation id.
func validateWebCitationShape(m WebCitationManifest) error {
	if m.Schema != WebCitationSchema {
		return fmt.Errorf("%w: web citation manifest schema is %d, want %d", ErrInvalidInput, m.Schema, WebCitationSchema)
	}
	if len(m.Entries) > MaxWebCitations {
		return fmt.Errorf("%w: %d citation entries exceed the %d-entry cap", ErrInvalidInput, len(m.Entries), MaxWebCitations)
	}
	seen := make(map[WebCitationEntry]struct{}, len(m.Entries))
	for _, entry := range m.Entries {
		if !entry.Kind.valid() {
			return fmt.Errorf("%w: unknown citation kind %q", ErrInvalidInput, entry.Kind)
		}
		if strings.TrimSpace(entry.Claim) == "" || !utf8.ValidString(entry.Claim) {
			return fmt.Errorf("%w: citation entry carries no readable claim", ErrInvalidInput)
		}
		if len(entry.Claim) > MaxWebCitationClaimBytes {
			return fmt.Errorf("%w: citation claim exceeds the %d-byte bound", ErrInvalidInput, MaxWebCitationClaimBytes)
		}
		switch entry.Kind {
		case WebCitationFact:
			if !WebCitationAllowed(entry.CitationID) {
				return fmt.Errorf("%w: citation %q is not a knowledge citation id (kc_ + 24 hex); invented sources never pass", ErrInvalidInput, entry.CitationID)
			}
		case WebCitationInference:
			if entry.CitationID != "" {
				return fmt.Errorf("%w: inference entry %q must never present a source citation", ErrInvalidInput, entry.Claim)
			}
		}
		if _, dup := seen[entry]; dup {
			return fmt.Errorf("%w: duplicate citation entry", ErrInvalidInput)
		}
		seen[entry] = struct{}{}
	}
	return nil
}

// ValidateWebCitationManifest is the binding rule of the citation manifest:
// beyond the closed shape, every fact entry's citation id must be among the
// Run's recorded actual sources. A nil recorded set fails every fact closed.
func ValidateWebCitationManifest(m WebCitationManifest, recorded map[string]struct{}) error {
	if err := validateWebCitationShape(m); err != nil {
		return err
	}
	for _, entry := range m.Entries {
		if entry.Kind != WebCitationFact {
			continue
		}
		if _, ok := recorded[entry.CitationID]; !ok {
			return fmt.Errorf("%w: citation %q is not among the Run's recorded sources; fabricated citations never pass", ErrInvalidInput, entry.CitationID)
		}
	}
	return nil
}

// DecodeWebCitationManifest strictly decodes the staged citations.json:
// unknown fields are rejected rather than silently dropped, entries must be
// present, and the closed shape must hold. Binding to the Run's recorded
// sources is checked separately by ValidateWebCitationManifest.
func DecodeWebCitationManifest(data []byte) (WebCitationManifest, error) {
	var m WebCitationManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return WebCitationManifest{}, fmt.Errorf("%w: web citation manifest decode: %v", ErrInvalidInput, err)
	}
	if decoder.More() {
		// Trailing content after the first JSON document ({...}garbage, or a
		// second shadow document) is refused: the manifest is untrusted
		// model output and strictness here is part of that contract.
		return WebCitationManifest{}, fmt.Errorf("%w: web citation manifest has trailing data", ErrInvalidInput)
	}
	// decoder.More() misses stray closing brackets ({...}}, {...]): decode
	// once more and demand the EXACT end of stream.
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return WebCitationManifest{}, fmt.Errorf("%w: web citation manifest has trailing data", ErrInvalidInput)
	}
	if m.Entries == nil {
		return WebCitationManifest{}, fmt.Errorf("%w: web citation manifest carries no entries field", ErrInvalidInput)
	}
	if err := validateWebCitationShape(m); err != nil {
		return WebCitationManifest{}, err
	}
	return m, nil
}

// WebCitationsDigest derives the stable evidence digest of one manifest from
// its entries. Changed evidence facts change the digest, so downstream
// version-pinned evidence and share decisions can bind to it.
func WebCitationsDigest(m WebCitationManifest) (string, error) {
	if err := validateWebCitationShape(m); err != nil {
		return "", err
	}
	raw, err := json.Marshal(m.Entries)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// webCitationMarkerAudit is the structured result of walking the entry HTML
// as real markup: citation markers and inference markers counted ONLY on
// element attributes. Comments, raw text inside script/style, and attribute
// values of unrelated elements can never satisfy a marker.
type webCitationMarkerAudit struct {
	citations   map[string]int // citation id → number of elements carrying it
	inferences  int            // elements carrying the inference marker
	mixedMarker bool           // one element carrying both markers
}

// auditWebCitationMarkers tokenizes the (model-generated, untrusted) entry
// HTML and reads the marker attributes from element start tags only. The
// previous text-level regexes matched the marker strings anywhere — a hidden
// comment could satisfy "declared facts must be rendered" and the inference
// count, and [^>]* chopping let a mixed-marker element slip through.
func auditWebCitationMarkers(entryHTML string) webCitationMarkerAudit {
	audit := webCitationMarkerAudit{citations: map[string]int{}}
	tokenizer := html.NewTokenizer(strings.NewReader(entryHTML))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return audit
		case html.StartTagToken, html.SelfClosingTagToken:
			hasCitation := false
			hasInference := false
			for _, attr := range tokenizer.Token().Attr {
				switch attr.Key {
				case WebCitationFactMarkerAttr:
					hasCitation = true
					audit.citations[attr.Val]++
				case WebCitationInferenceMarkerAttr:
					hasInference = true
				}
			}
			if hasInference {
				audit.inferences++
			}
			if hasCitation && hasInference {
				audit.mixedMarker = true
			}
		}
	}
}

// ValidateWebCitationView binds the rendered page to its manifest:
//
//   - every citation marker in the entry must be declared as a fact in the
//     manifest (an undeclared marker cannot resolve to any recorded source);
//   - every manifest fact must appear as a marker (declared evidence is
//     actually rendered);
//   - every inference entry must be rendered with the explicit inference
//     marker;
//   - no single element may carry both markers: inference is never
//     presented as a source fact.
//
// Markers are read structurally — from element attributes of a real HTML
// tokenization — so comments, script/style text and unrelated attribute
// payloads cannot fake marker presence.
func ValidateWebCitationView(entryHTML string, m WebCitationManifest) error {
	if err := validateWebCitationShape(m); err != nil {
		return err
	}
	audit := auditWebCitationMarkers(entryHTML)
	if audit.mixedMarker {
		return fmt.Errorf("%w: an inference element must never carry a source citation marker", ErrInvalidInput)
	}
	declared := make(map[string]struct{}, len(m.Entries))
	inferences := 0
	for _, entry := range m.Entries {
		if entry.Kind == WebCitationFact {
			declared[entry.CitationID] = struct{}{}
		} else {
			inferences++
		}
	}
	for id := range audit.citations {
		if _, ok := declared[id]; !ok {
			return fmt.Errorf("%w: citation %q is rendered in the page but not declared in the citation manifest", ErrInvalidInput, id)
		}
	}
	for id := range declared {
		if audit.citations[id] == 0 {
			return fmt.Errorf("%w: fact citation %q must appear in the page's citation view", ErrInvalidInput, id)
		}
	}
	if audit.inferences < inferences {
		return fmt.Errorf("%w: every inference entry must appear in the page's citation view with the explicit inference marker", ErrInvalidInput)
	}
	return nil
}

// webCitationLabels picks the view's fixed labels from the manifest's lang
// (zh default). All labels are fixed literals; claim text is the only
// variable and is always HTML-escaped.
type webCitationLabels struct {
	aria, heading, fact, inference string
}

func webCitationLabelsFor(lang string) webCitationLabels {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "en") {
		return webCitationLabels{
			aria:      "Citations and inference",
			heading:   "Citations and inference",
			fact:      "[Source fact]",
			inference: "[Model inference]",
		}
	}
	return webCitationLabels{
		aria:      "来源与推断标注",
		heading:   "来源与推断",
		fact:      "[事实来源]",
		inference: "[模型推断]",
	}
}

// RenderWebCitationView renders the canonical citation view fragment for one
// manifest: fact links as in-page fragment anchors carrying the citation
// marker, inference entries carrying the explicit inference marker. The
// fragment is offline-safe by construction — only fragment anchors, fixed
// local classes and escaped claim text, so it passes the pinned toolchain's
// denylist (docker/craft/web/build.py) unchanged. The manifest must be
// shape-valid; binding to recorded sources belongs to the admission gate.
func RenderWebCitationView(m WebCitationManifest) (string, error) {
	if err := validateWebCitationShape(m); err != nil {
		return "", err
	}
	labels := webCitationLabelsFor(m.Lang)
	var b strings.Builder
	b.WriteString(`<section class="craft-citations" aria-label="`)
	b.WriteString(labels.aria)
	b.WriteString(`"><h2>`)
	b.WriteString(labels.heading)
	b.WriteString(`</h2><ul class="craft-citation-list">`)
	for _, entry := range m.Entries {
		switch entry.Kind {
		case WebCitationFact:
			anchor := "craft-cite-" + entry.CitationID
			b.WriteString(`<li class="craft-citation-fact" id="`)
			b.WriteString(anchor)
			b.WriteString(`"><a class="craft-citation" href="#`)
			b.WriteString(anchor)
			b.WriteString(`" `)
			b.WriteString(WebCitationFactMarkerAttr)
			b.WriteString(`="`)
			b.WriteString(entry.CitationID)
			b.WriteString(`">`)
			b.WriteString(labels.fact)
			b.WriteString(` `)
			b.WriteString(stdhtml.EscapeString(entry.Claim))
			b.WriteString(`</a></li>`)
		case WebCitationInference:
			b.WriteString(`<li class="craft-citation-inference" `)
			b.WriteString(WebCitationInferenceMarkerAttr)
			b.WriteString(`="true">`)
			b.WriteString(labels.inference)
			b.WriteString(` `)
			b.WriteString(stdhtml.EscapeString(entry.Claim))
			b.WriteString(`</li>`)
		}
	}
	b.WriteString(`</ul></section>`)
	return b.String(), nil
}

// WebCitationPlaceholder is the non-leaking projection of one citation whose
// source is missing, revoked or inaccessible: it carries only the stable
// citation id, the marker kind and the unavailable status — never the ref,
// digest, title or excerpt of the original.
type WebCitationPlaceholder struct {
	CitationID string          `json:"citation_id"`
	Kind       WebCitationKind `json:"kind"`
	Status     string          `json:"status"`
}

// WebCitationOpen is the result of opening one citation: exactly one of a
// durable ref (freshly authorized through the existing permission chain) or
// a non-leaking placeholder.
type WebCitationOpen struct {
	Ref         string                  `json:"ref,omitempty"`
	Placeholder *WebCitationPlaceholder `json:"placeholder,omitempty"`
}

// Validate enforces the exactly-one-of contract.
func (o WebCitationOpen) Validate() error {
	if (o.Ref == "") == (o.Placeholder == nil) {
		return fmt.Errorf("%w: citation open must carry exactly one of ref or placeholder", ErrInvalidInput)
	}
	if o.Placeholder != nil {
		if !WebCitationAllowed(o.Placeholder.CitationID) || !o.Placeholder.Kind.valid() ||
			o.Placeholder.Status != WebCitationStatusUnavailable {
			return fmt.Errorf("%w: malformed citation placeholder", ErrInvalidInput)
		}
	}
	return nil
}
