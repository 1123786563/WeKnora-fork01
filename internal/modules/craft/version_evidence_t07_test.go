package craft

// T07 (#131) — version-pinned evidence domain rules. The evidence pinned to
// one version freezes the producing Run's RECORDED source facts (source ref,
// per-source digest, acquisition time); it validates fail-closed. Its
// integrity digest is a pure function of the canonical encoding (PinnedAt
// included): byte-for-byte integrity of what a store persists, never a
// replay-identity test — replay adoption compares the frozen facts.

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// t07Digest derives a well-formed SHA-256 hex digest from a seed so test
// facts satisfy the same shape production records carry.
func t07Digest(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// t07SourceRecord builds one recorded source fact with the exact shape T05
// persists for a Run.
func t07SourceRecord(id, digest string, acquired time.Time) KnowledgeSourceRecord {
	return KnowledgeSourceRecord{
		ID: id, Ref: KnowledgeRef("kb-a", "k-"+id, "c-"+id),
		Digest: digest, TenantID: 1, AcquiredAt: acquired, ExcerptBytes: 32,
	}
}

// t07RunRecord builds one published Run record carrying the given sources.
func t07RunRecord(scope Scope, runID string, sources ...KnowledgeSourceRecord) KnowledgeRecord {
	return KnowledgeRecord{
		Scope: scope, RunID: runID,
		RequestDigest:    t07Digest("req-" + runID),
		PackageDigest:    t07Digest("pkg-" + runID),
		PublicationState: KnowledgePublicationPublished,
		Sources:          sources,
	}
}

// TestCraftT07PinVersionEvidenceFreezesRunSourceFacts pins the success
// shape: pinning derives the version's evidence from the Run's immutable
// record — source ref, digest and acquisition time travel verbatim, the
// evidence-level acquisition time is the earliest source observation, and
// the digest of identical facts is stable.
func TestCraftT07PinVersionEvidenceFreezesRunSourceFacts(t *testing.T) {
	scope := Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	t1 := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	versionID := VersionID("ws-1", "run-1", t07Digest("manifest"))

	record := t07RunRecord(scope, "run-1",
		t07SourceRecord("kc_a", t07Digest("a"), t2),
		t07SourceRecord("kc_b", t07Digest("b"), t1),
	)
	pinnedAt := t2.Add(time.Minute)

	evidence, err := PinVersionEvidence(versionID, record, pinnedAt)
	require.NoError(t, err)
	require.Equal(t, versionID, evidence.VersionID)
	require.Equal(t, "run-1", evidence.RunID)
	require.Equal(t, record.RequestDigest, evidence.RequestDigest)
	require.Equal(t, record.PackageDigest, evidence.PackageDigest)
	require.Equal(t, t1, evidence.AcquiredAt, "the evidence acquisition time is the earliest source observation")
	require.Equal(t, pinnedAt, evidence.PinnedAt)
	require.False(t, evidence.Empty)
	require.Equal(t, record.Sources, evidence.Sources, "recorded source facts travel verbatim")

	digest, err := VersionEvidenceDigest(evidence)
	require.NoError(t, err)
	require.Len(t, digest, 64)

	// The identical pin derives the identical evidence identity again — a
	// crash-replayed promotion cannot produce drifting evidence.
	replayed, err := PinVersionEvidence(versionID, record, pinnedAt)
	require.NoError(t, err)
	replayDigest, err := VersionEvidenceDigest(replayed)
	require.NoError(t, err)
	require.Equal(t, digest, replayDigest)

	// Updated knowledge (a new Run's record with different source digests
	// and acquisition times) pins DIFFERENT evidence facts.
	later := t07RunRecord(scope, "run-2", t07SourceRecord("kc_a", t07Digest("a-updated"), t2.Add(time.Hour)))
	laterEvidence, err := PinVersionEvidence(VersionID("ws-1", "run-2", t07Digest("manifest-2")), later, pinnedAt.Add(time.Hour))
	require.NoError(t, err)
	laterDigest, err := VersionEvidenceDigest(laterEvidence)
	require.NoError(t, err)
	require.NotEqual(t, digest, laterDigest)
	require.NotEqual(t, evidence.Sources[0].Digest, laterEvidence.Sources[0].Digest)
}

// TestCraftT07ValidateVersionEvidenceFailsClosed pins the failure shape:
// evidence without a well-formed version identity, without its digests,
// with sources missing their digest or acquisition time, or whose Empty flag
// disagrees with its sources is refused before anything persists.
func TestCraftT07ValidateVersionEvidenceFailsClosed(t *testing.T) {
	scope := Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	acquired := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)
	pinnedAt := acquired.Add(time.Minute)
	versionID := VersionID("ws-1", "run-1", t07Digest("manifest"))

	valid := func() VersionEvidence {
		ev, err := PinVersionEvidence(versionID, t07RunRecord(scope, "run-1", t07SourceRecord("kc_a", t07Digest("a"), acquired)), pinnedAt)
		require.NoError(t, err)
		return ev
	}
	require.NoError(t, ValidateVersionEvidence(valid()))

	// A malformed version identity never pins.
	badID := valid()
	badID.VersionID = "not-a-version-id"
	require.ErrorIs(t, ValidateVersionEvidence(badID), ErrInvalidInput)

	// The producing Run and the package digests are required facts.
	noRun := valid()
	noRun.RunID = ""
	require.ErrorIs(t, ValidateVersionEvidence(noRun), ErrInvalidInput)
	noPackage := valid()
	noPackage.PackageDigest = ""
	require.ErrorIs(t, ValidateVersionEvidence(noPackage), ErrInvalidInput)
	noRequest := valid()
	noRequest.RequestDigest = ""
	require.ErrorIs(t, ValidateVersionEvidence(noRequest), ErrInvalidInput)

	// Each recorded source keeps its ref, digest and acquisition time.
	noDigest := valid()
	noDigest.Sources = append([]KnowledgeSourceRecord(nil), noDigest.Sources...)
	noDigest.Sources[0].Digest = "not-hex"
	require.ErrorIs(t, ValidateVersionEvidence(noDigest), ErrInvalidInput)
	noTime := valid()
	noTime.Sources = append([]KnowledgeSourceRecord(nil), noTime.Sources...)
	noTime.Sources[0].AcquiredAt = time.Time{}
	require.ErrorIs(t, ValidateVersionEvidence(noTime), ErrInvalidInput)
	noRef := valid()
	noRef.Sources = append([]KnowledgeSourceRecord(nil), noRef.Sources...)
	noRef.Sources[0].Ref = ""
	require.ErrorIs(t, ValidateVersionEvidence(noRef), ErrInvalidInput)

	// The Empty flag must agree with the recorded sources, and an empty
	// record carries no acquisition time to freeze.
	empty := valid()
	empty.Empty = true
	require.ErrorIs(t, ValidateVersionEvidence(empty), ErrInvalidInput)
	emptyRecord := KnowledgeRecord{
		Scope: scope, RunID: "run-empty", RequestDigest: t07Digest("req"), PackageDigest: t07Digest("pkg"),
		PublicationState: KnowledgePublicationPublished, Sources: nil, Empty: true,
	}
	emptyEvidence, err := PinVersionEvidence(VersionID("ws-1", "run-empty", t07Digest("m")), emptyRecord, pinnedAt)
	require.NoError(t, err)
	require.True(t, emptyEvidence.Empty)
	require.Zero(t, emptyEvidence.AcquiredAt, "an empty record has no acquisition time to fabricate")
	require.NoError(t, ValidateVersionEvidence(emptyEvidence))
}

// TestCraftT07EncodeVersionEvidenceOwnsDigest pins the encoding contract
// (T07 OCR fix): the digest is the SHA-256 of exactly the canonical bytes
// EncodeVersionEvidence returns, the digest helper delegates to the same
// encoding, and PinnedAt deliberately participates — a replayed promotion
// (fresh PinnedAt, identical frozen facts) derives a different digest, so
// replay adoption must compare facts, never digests.
func TestCraftT07EncodeVersionEvidenceOwnsDigest(t *testing.T) {
	scope := Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	t1 := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)
	versionID := VersionID("ws-1", "run-1", t07Digest("manifest"))
	record := t07RunRecord(scope, "run-1", t07SourceRecord("kc_a", t07Digest("a"), t1))
	ev, err := PinVersionEvidence(versionID, record, t1.Add(time.Minute))
	require.NoError(t, err)

	raw, digest, err := EncodeVersionEvidence(ev)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	require.Equal(t, hex.EncodeToString(sum[:]), digest, "the digest is the SHA-256 of exactly the returned canonical bytes")
	digestOnly, err := VersionEvidenceDigest(ev)
	require.NoError(t, err)
	require.Equal(t, digestOnly, digest, "VersionEvidenceDigest delegates to the same encoding")

	// A replay re-pins at a fresh clock reading: identical frozen facts,
	// different integrity digest — adoption compares facts (PinnedAt
	// excluded), never this digest.
	replay := ev
	replay.PinnedAt = ev.PinnedAt.Add(time.Hour)
	replayDigest, err := VersionEvidenceDigest(replay)
	require.NoError(t, err)
	require.NotEqual(t, digest, replayDigest, "PinnedAt deliberately participates in the integrity digest")
	require.Equal(t, ev.Sources, replay.Sources, "the frozen facts themselves are identical")
}
