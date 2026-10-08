package repository

// Wrap-up OCR F34: evidence identity must be clock-representation safe ALL
// the way down — a bare != on the Sources' AcquiredAt would flag the same
// instant serialized in a different zone offset as a phantom conflict and
// break idempotent replay adoption.
import (
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

func TestSameCraftVersionEvidenceIsZoneOffsetSafe(t *testing.T) {
	instant := time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC)
	sameInstantOtherZone := instant.In(time.FixedZone("CST", 8*60*60))

	a := craft.VersionEvidence{
		VersionID: "ver-1", RunID: "run-1", RequestDigest: "d1", PackageDigest: "p1",
		AcquiredAt: instant,
		Sources: []craft.KnowledgeSourceRecord{
			{ID: "k1", Ref: "craftkb://x", Digest: "sha256:a", TenantID: 1, AcquiredAt: instant, ExcerptBytes: 8},
		},
	}
	b := craft.VersionEvidence{
		VersionID: "ver-1", RunID: "run-1", RequestDigest: "d1", PackageDigest: "p1",
		AcquiredAt: sameInstantOtherZone,
		Sources: []craft.KnowledgeSourceRecord{
			{ID: "k1", Ref: "craftkb://x", Digest: "sha256:a", TenantID: 1, AcquiredAt: sameInstantOtherZone, ExcerptBytes: 8},
		},
	}
	require.True(t, sameCraftVersionEvidence(a, b),
		"the same instant in a different zone offset is the SAME evidence — idempotent replay must adopt it")

	// A genuinely different source timestamp still conflicts.
	c := b
	c.Sources[0].AcquiredAt = instant.Add(time.Second)
	require.False(t, sameCraftVersionEvidence(a, c))
}
