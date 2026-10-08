package craft

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The OCR hardening round: ValidateWebCitationView must read markers from
// real element attributes (structured tokenization), never from raw text.
// Hidden comments, script/style payloads and regex-chopping tricks must not
// satisfy any of the four binding rules.

func TestWebCitationViewIgnoresHiddenCommentMarkers(t *testing.T) {
	factID := KnowledgeCitationID("kb-a", "k-a", "c-a")
	m := t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: factID, Claim: "fact"},
		WebCitationEntry{Kind: WebCitationInference, Claim: "inference"},
	)
	// The fact marker and the inference marker appear ONLY inside an HTML
	// comment: under the old text-level regexes this page passed all four
	// rules while rendering nothing.
	commentOnly := `<p>plain page</p><!-- data-craft-citation="` + factID + `" --><!-- data-craft-inference="true" -->`
	err := ValidateWebCitationView(commentOnly, m)
	require.ErrorContains(t, err, "must appear", "a comment-only marker must not count as rendered evidence")

	// The same strings inside a script payload are equally inert.
	scriptOnly := `<p>x</p><script>var s = 'data-craft-citation="` + factID + `" data-craft-inference="true"';</script>`
	err = ValidateWebCitationView(scriptOnly, m)
	require.ErrorContains(t, err, "must appear", "script text must not satisfy marker presence")
}

func TestWebCitationViewCountsOnlyElementAttributes(t *testing.T) {
	factID := KnowledgeCitationID("kb-a", "k-a", "c-a")
	m := t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: factID, Claim: "fact"},
		WebCitationEntry{Kind: WebCitationInference, Claim: "inference"},
	)
	// One real marker of each kind on real elements passes…
	real := `<p ` + WebCitationFactMarkerAttr + `="` + factID + `">fact</p>` +
		`<p ` + WebCitationInferenceMarkerAttr + `="true">guess</p>`
	require.NoError(t, ValidateWebCitationView(real, m))

	// …while text that merely mentions the attribute name does not count.
	mentioned := `<p>` + WebCitationFactMarkerAttr + `="` + factID + `" is an attribute name</p>` +
		`<p>data-craft-inference appears here as text</p>`
	err := ValidateWebCitationView(mentioned, m)
	require.ErrorContains(t, err, "must appear", "attribute names in text nodes are not markers")
}

func TestWebCitationViewMixedMarkerDetectionSurvivesTricks(t *testing.T) {
	factID := KnowledgeCitationID("kb-a", "k-a", "c-a")
	m := t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: factID, Claim: "fact"},
		WebCitationEntry{Kind: WebCitationInference, Claim: "inference"},
	)
	// A single element carrying both markers through an attribute value
	// containing '>' — the old [^>]* regex chopped here and let it pass —
	// plus the single-quoted inference form the old pattern never matched.
	tricky := `<p data-note="a > b" ` + WebCitationFactMarkerAttr + `="` + factID + `" data-craft-inference='true'>mixed</p>` +
		`<p ` + WebCitationInferenceMarkerAttr + `>second inference</p>`
	err := ValidateWebCitationView(tricky, m)
	require.ErrorContains(t, err, "never carry", "a mixed-marker element must be refused regardless of quoting or intervening '>'")

	// Single-quoted inference marker still COUNTS as a real inference
	// marker when no citation marker shares the element.
	singleQuoted := `<p ` + WebCitationFactMarkerAttr + `="` + factID + `">fact</p>` +
		`<p data-craft-inference='true'>guess</p>`
	require.NoError(t, ValidateWebCitationView(singleQuoted, m),
		"the tokenizer must honor single-quoted attributes for counting, not only for refusal")
}

func TestWebCitationViewStillBindsCanonicalRender(t *testing.T) {
	// Guard against over-tightening: the canonical renderer's output keeps
	// validating (double-quoted attributes, one marker per element).
	m := t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: KnowledgeCitationID("kb-a", "k-a", "c-a"), Claim: "fact"},
		WebCitationEntry{Kind: WebCitationInference, Claim: "inference"},
	)
	view, err := RenderWebCitationView(m)
	require.NoError(t, err)
	require.NoError(t, ValidateWebCitationView(view, m))
}
