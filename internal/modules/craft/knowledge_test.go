package craft

import (
	"strings"
	"testing"
)

// TestSourcesKeepRefsWhenBounded is the brief's verbatim Step 1 test: a bundle
// bounded by bytes keeps the sources that fit with their refs intact and
// reports truncation for the rest.
func TestSourcesKeepRefsWhenBounded(t *testing.T) {
	b := BoundSources([]Source{{ID: "c1", Ref: "resource://a", Excerpt: "123"}, {ID: "c2", Ref: "resource://b", Excerpt: "456"}}, 3)
	if len(b.Sources) != 1 || b.Sources[0].Ref != "resource://a" || !b.Truncated {
		t.Fatalf("%+v", b)
	}
}

// TestBoundSourcesKeepsEverythingWhenUnderBudget verifies that a bundle within
// both caps is returned whole and not marked truncated.
func TestBoundSourcesKeepsEverythingWhenUnderBudget(t *testing.T) {
	sources := []Source{
		{ID: "kc1", Ref: "craftkb://kb/k/1", Excerpt: "alpha", Digest: "d1", TenantID: 7},
		{ID: "kc2", Ref: "craftkb://kb/k/2", Excerpt: "beta", Digest: "d2", TenantID: 9},
	}
	b := BoundSources(sources, 64)
	if b.Truncated || len(b.Sources) != 2 {
		t.Fatalf("bundle = %+v, want both sources without truncation", b)
	}
	for i, want := range sources {
		if b.Sources[i] != want {
			t.Fatalf("source %d = %+v, want %+v", i, b.Sources[i], want)
		}
	}
}

// TestBoundSourcesCapsAtTwentyEntries pins the per-bundle count cap: at most
// MaxKnowledgeSources entries survive even when every excerpt is tiny.
func TestBoundSourcesCapsAtTwentyEntries(t *testing.T) {
	sources := make([]Source, 0, MaxKnowledgeSources+5)
	for i := 0; i < MaxKnowledgeSources+5; i++ {
		sources = append(sources, Source{ID: "k", Ref: "craftkb://kb/k", Excerpt: "x"})
	}
	b := BoundSources(sources, 1<<20)
	if len(b.Sources) != MaxKnowledgeSources || !b.Truncated {
		t.Fatalf("len = %d, truncated = %v, want %d entries and truncation", len(b.Sources), b.Truncated, MaxKnowledgeSources)
	}
}

// TestBoundSourcesAccumulatesBytesAcrossSources pins that the byte budget is
// consumed by every kept source: an oversized source is skipped (marking
// truncation) while a later smaller source may still fit the remaining
// budget, exactly as the brief's BoundSources prescribes.
func TestBoundSourcesAccumulatesBytesAcrossSources(t *testing.T) {
	sources := []Source{
		{ID: "a", Ref: "r://a", Excerpt: "12"},
		{ID: "b", Ref: "r://b", Excerpt: "12"},
		{ID: "c", Ref: "r://c", Excerpt: "1"},
	}
	b := BoundSources(sources, 3)
	if len(b.Sources) != 2 || b.Sources[0].ID != "a" || b.Sources[1].ID != "c" || !b.Truncated {
		t.Fatalf("bundle = %+v, want sources a and c (b exceeds the shared budget) with truncation", b)
	}
}

// TestBoundSourcesReturnsEmptySliceForNoSources pins the empty-input shape:
// an empty non-nil slice and no truncation, so JSON manifests never carry null.
func TestBoundSourcesReturnsEmptySliceForNoSources(t *testing.T) {
	b := BoundSources(nil, 64)
	if b.Sources == nil || len(b.Sources) != 0 || b.Truncated {
		t.Fatalf("bundle = %+v, want empty non-nil sources without truncation", b)
	}
}

// TestExcerptOfBoundsAtRuneBoundary pins the controlled excerpt: bounded by
// bytes but never split mid-rune, and unchanged content passes through.
func TestExcerptOfBoundsAtRuneBoundary(t *testing.T) {
	if got := ExcerptOf("hello", 32); got != "hello" {
		t.Fatalf("ExcerptOf short = %q, want passthrough", got)
	}
	long := strings.Repeat("知", 64) // 3 bytes per rune
	got := ExcerptOf(long, 8)
	if !strings.HasSuffix(got, "�") {
		t.Fatalf("overshoot marker missing: %q", got)
	}
	if n := len(got); n > 8+3 {
		t.Fatalf("excerpt length %d exceeds byte cap plus one marker rune", n)
	}
}

// TestKnowledgeRefAndCitationIDStable pins the stable citation identity: the
// same chunk coordinates always produce the same ref and citation ID, and
// different coordinates never collide.
func TestKnowledgeRefAndCitationIDStable(t *testing.T) {
	ref := KnowledgeRef("kb-1", "k-1", "c-1")
	if ref == "" || KnowledgeRef("kb-1", "k-1", "c-1") != ref {
		t.Fatalf("KnowledgeRef not stable: %q", ref)
	}
	if KnowledgeRef("kb-2", "k-1", "c-1") == ref {
		t.Fatalf("KnowledgeRef collides across knowledge bases")
	}
	id := KnowledgeCitationID("kb-1", "k-1", "c-1")
	if id == "" || KnowledgeCitationID("kb-1", "k-1", "c-1") != id {
		t.Fatalf("KnowledgeCitationID not stable: %q", id)
	}
	if KnowledgeCitationID("kb-1", "k-1", "c-2") == id {
		t.Fatalf("KnowledgeCitationID collides across chunks")
	}
}
