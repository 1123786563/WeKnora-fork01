package craft

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// t06Recorded builds the recorded-source id set one Run actually staged
// (knowledge.go's KnowledgeCitationID shape).
func t06Recorded(ids ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func t06Manifest(entries ...WebCitationEntry) WebCitationManifest {
	return WebCitationManifest{Schema: WebCitationSchema, Lang: "zh-CN", Entries: entries}
}

func TestWebCitationManifestBindsFactsToRecordedSourcesAndMarksInference(t *testing.T) {
	recorded := t06Recorded(KnowledgeCitationID("kb-a", "k-a", "c-a"))
	other := KnowledgeCitationID("kb-a", "k-a", "c-never-staged")

	require.NoError(t, ValidateWebCitationManifest(t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: KnowledgeCitationID("kb-a", "k-a", "c-a"), Claim: "销售额来自来源"},
		WebCitationEntry{Kind: WebCitationInference, Claim: "预计下季度增长"},
	), recorded))
	// A fact may cite the same recorded source twice (two claims, one source).
	require.NoError(t, ValidateWebCitationManifest(t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: KnowledgeCitationID("kb-a", "k-a", "c-a"), Claim: "first"},
		WebCitationEntry{Kind: WebCitationFact, CitationID: KnowledgeCitationID("kb-a", "k-a", "c-a"), Claim: "second"},
	), recorded))

	cases := []struct {
		name     string
		manifest WebCitationManifest
		errText  string
	}{
		{"fabricated id is shape-valid but never recorded", t06Manifest(
			WebCitationEntry{Kind: WebCitationFact, CitationID: other, Claim: "claim"}),
			"not among the Run"},
		{"malformed fact id", t06Manifest(
			WebCitationEntry{Kind: WebCitationFact, CitationID: "hand-made-1", Claim: "claim"}),
			"not a knowledge citation id"},
		{"fact without any id", t06Manifest(
			WebCitationEntry{Kind: WebCitationFact, Claim: "claim"}),
			"not a knowledge citation id"},
		{"inference presenting a source citation", t06Manifest(
			WebCitationEntry{Kind: WebCitationInference, CitationID: KnowledgeCitationID("kb-a", "k-a", "c-a"), Claim: "claim"}),
			"never present a source citation"},
		{"unknown kind", WebCitationManifest{Schema: WebCitationSchema, Entries: []WebCitationEntry{
			{Kind: WebCitationKind("rumor"), Claim: "claim"}}}, "unknown citation kind"},
		{"schema mismatch", WebCitationManifest{Schema: 2, Entries: []WebCitationEntry{
			{Kind: WebCitationInference, Claim: "claim"}}}, "schema"},
		{"blank claim", t06Manifest(WebCitationEntry{Kind: WebCitationInference, Claim: "  "}), "claim"},
		{"oversized claim", t06Manifest(WebCitationEntry{Kind: WebCitationInference, Claim: strings.Repeat("长", MaxWebCitationClaimBytes)}), "claim"},
		{"exact duplicate entry", t06Manifest(
			WebCitationEntry{Kind: WebCitationInference, Claim: "same"},
			WebCitationEntry{Kind: WebCitationInference, Claim: "same"}), "duplicate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWebCitationManifest(tc.manifest, recorded)
			require.ErrorIs(t, err, ErrInvalidInput)
			require.Contains(t, err.Error(), tc.errText)
		})
	}
	// Over the entry cap.
	entries := make([]WebCitationEntry, 0, MaxWebCitations+1)
	for i := 0; i <= MaxWebCitations; i++ {
		entries = append(entries, WebCitationEntry{Kind: WebCitationInference, Claim: "claim"})
	}
	require.ErrorIs(t, ValidateWebCitationManifest(WebCitationManifest{Schema: WebCitationSchema, Entries: entries}, recorded), ErrInvalidInput)

	// With no recorded set every fact fails closed.
	require.ErrorIs(t, ValidateWebCitationManifest(t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: KnowledgeCitationID("kb-a", "k-a", "c-a"), Claim: "claim"}),
		nil), ErrInvalidInput)
}

func TestDecodeWebCitationManifestIsStrict(t *testing.T) {
	good, err := json.Marshal(WebCitationManifest{Schema: WebCitationSchema, Entries: []WebCitationEntry{
		{Kind: WebCitationInference, Claim: "claim"}}})
	require.NoError(t, err)
	m, err := DecodeWebCitationManifest(good)
	require.NoError(t, err)
	require.Equal(t, WebCitationSchema, m.Schema)
	require.Len(t, m.Entries, 1)

	_, err = DecodeWebCitationManifest([]byte(`{"schema":1,"entries":[],"extra":true}`))
	require.ErrorIs(t, err, ErrInvalidInput, "unknown fields must be rejected, not silently dropped")
	_, err = DecodeWebCitationManifest([]byte(`not json`))
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = DecodeWebCitationManifest([]byte(`{"schema":1}`))
	require.ErrorIs(t, err, ErrInvalidInput, "entries must be present")
}

func TestRenderWebCitationViewIsOfflineSafeAndBindsMarkers(t *testing.T) {
	factID := KnowledgeCitationID("kb-a", "k-a", "c-a")
	m := t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: factID, Claim: "华东区销售额 <total> 1,200 万"},
		WebCitationEntry{Kind: WebCitationInference, Claim: "预计 Q4 增长"},
	)
	view, err := RenderWebCitationView(m)
	require.NoError(t, err)

	// The fact link and the explicit inference marker are both present.
	require.Contains(t, view, WebCitationFactMarkerAttr+"=\""+factID+"\"")
	require.Contains(t, view, WebCitationInferenceMarkerAttr+"=\"true\"")
	// Claims are escaped.
	require.Contains(t, view, "&lt;total&gt;")
	require.NotContains(t, view, "<total>")

	// The rendered view is admissible under the pinned offline toolchain's
	// denylist (docker/craft/web/build.py): no external or absolute
	// references, no scheme, no embedding/script/form tags, no inline event
	// handlers, no css fetch.
	for _, banned := range []string{"http", "//", "javascript:", "url(", "@import", "<script", "<iframe", "<form", "<object", "<embed", "<base", "<meta", " on"} {
		require.NotContains(t, view, banned, "rendered citation view must stay offline-safe")
	}
	// href attributes may only be in-page fragment anchors.
	for _, href := range strings.Split(view, "href=\"")[1:] {
		require.True(t, strings.HasPrefix(href, "#craft-cite-"), "citation href must be a fragment anchor, got %q", href)
	}

	// The canonical view agrees with its own manifest.
	require.NoError(t, ValidateWebCitationView(view, m))
	// Bilingual rendering.
	enView, err := RenderWebCitationView(WebCitationManifest{Schema: WebCitationSchema, Lang: "en", Entries: m.Entries})
	require.NoError(t, err)
	require.Contains(t, enView, "Model inference")
	require.Contains(t, view, "模型推断")
}

func TestValidateWebCitationViewBindsPageMarkersToManifest(t *testing.T) {
	factID := KnowledgeCitationID("kb-a", "k-a", "c-a")
	m := t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: factID, Claim: "fact claim"},
		WebCitationEntry{Kind: WebCitationInference, Claim: "inference claim"},
	)
	canonical, err := RenderWebCitationView(m)
	require.NoError(t, err)
	require.NoError(t, ValidateWebCitationView(canonical, m))

	cases := []struct {
		name     string
		html     string
		manifest WebCitationManifest
		errText  string
	}{
		{"page cites an id absent from the manifest", canonical + `<a data-craft-citation="` + KnowledgeCitationID("x", "y", "z") + `">x</a>`, m, "not declared"},
		{"manifest fact missing from the page", func() string {
			view, err := RenderWebCitationView(t06Manifest(
				WebCitationEntry{Kind: WebCitationInference, Claim: "inference claim"}))
			require.NoError(t, err)
			return view
		}(), m, "must appear"},
		{"inference entry missing its marker", `<p>` + WebCitationFactMarkerAttr + `="` + factID + `"</p>`, m, "must appear"},
		{"inference presented as source fact in one element", `<p ` + WebCitationInferenceMarkerAttr + `="true" ` + WebCitationFactMarkerAttr + `="` + factID + `">mixed</p>`, m, "never carry"},
		{"unbound page citation without any manifest", `<p ` + WebCitationFactMarkerAttr + `="` + factID + `">ghost</p>`,
			WebCitationManifest{Schema: WebCitationSchema, Entries: []WebCitationEntry{}}, "not declared"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWebCitationView(tc.html, tc.manifest)
			require.ErrorIs(t, err, ErrInvalidInput)
			require.Contains(t, err.Error(), tc.errText)
		})
	}
	// A page with neither manifest entries nor markers is admissible.
	require.NoError(t, ValidateWebCitationView(`<p>plain</p>`, WebCitationManifest{Schema: WebCitationSchema, Entries: []WebCitationEntry{}}))
}

func TestWebCitationPlaceholderIsNonLeaking(t *testing.T) {
	placeholder := WebCitationPlaceholder{
		CitationID: KnowledgeCitationID("kb-a", "k-a", "c-a"),
		Kind:       WebCitationFact,
		Status:     WebCitationStatusUnavailable,
	}
	raw, err := json.Marshal(placeholder)
	require.NoError(t, err)
	require.JSONEq(t, `{"citation_id":"`+placeholder.CitationID+`","kind":"fact","status":"unavailable"}`, string(raw))
	for _, leaking := range []string{"ref", "digest", "title", "excerpt", "url"} {
		require.NotContains(t, string(raw), leaking, "placeholder must not carry original-source material")
	}

	// An open result is exactly one of a durable ref or a placeholder.
	require.NoError(t, (WebCitationOpen{Ref: "craftkb://kb/a/knowledge/k/chunk/c"}).Validate())
	require.NoError(t, (WebCitationOpen{Placeholder: &placeholder}).Validate())
	require.ErrorIs(t, (WebCitationOpen{}).Validate(), ErrInvalidInput)
	require.ErrorIs(t, (WebCitationOpen{Ref: "craftkb://kb/a/knowledge/k/chunk/c", Placeholder: &placeholder}).Validate(), ErrInvalidInput)
}

func TestWebCitationsDigestIsStableForIdenticalManifests(t *testing.T) {
	m := t06Manifest(
		WebCitationEntry{Kind: WebCitationFact, CitationID: KnowledgeCitationID("kb-a", "k-a", "c-a"), Claim: "fact"},
		WebCitationEntry{Kind: WebCitationInference, Claim: "inference"},
	)
	d1, err := WebCitationsDigest(m)
	require.NoError(t, err)
	d2, err := WebCitationsDigest(m)
	require.NoError(t, err)
	require.Equal(t, d1, d2)
	changed := m
	changed.Entries = append([]WebCitationEntry(nil), m.Entries...)
	changed.Entries[1].Claim = "changed inference"
	d3, err := WebCitationsDigest(changed)
	require.NoError(t, err)
	require.NotEqual(t, d1, d3, "changed evidence facts must change the digest")
}
