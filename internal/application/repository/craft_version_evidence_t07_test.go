package repository

// T07 (#131) — version-pinned evidence persistence against the real store:
// the evidence lands in the SAME commit as the version it belongs to, is
// immutable across replays and later versions, answers strictly by Version
// ID (never reconstructed from Workspace or current knowledge state), and
// its integrity digest refuses tampered rows.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// t07EvidenceFor builds one valid evidence snapshot for the given version
// and run, with deterministic facts per run so later versions observably
// differ from earlier ones.
func t07EvidenceFor(t *testing.T, versionID, runID string, acquired time.Time) craft.VersionEvidence {
	t.Helper()
	scope := craftTestScope()
	record := craft.KnowledgeRecord{
		Scope: scope, RunID: runID,
		RequestDigest:    t07RepoDigest("req-" + runID),
		PackageDigest:    t07RepoDigest("pkg-" + runID),
		PublicationState: craft.KnowledgePublicationPublished,
		Sources: []craft.KnowledgeSourceRecord{{
			ID: "kc_" + runID, Ref: craft.KnowledgeRef("kb-a", "k-a", "c-"+runID),
			Digest: t07RepoDigest("source-" + runID), TenantID: scope.TenantID,
			AcquiredAt: acquired, ExcerptBytes: 32,
		}},
	}
	ev, err := craft.PinVersionEvidence(versionID, record, acquired.Add(time.Minute))
	require.NoError(t, err)
	return ev
}

func t07RepoDigest(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// TestCraftT07EvidencePersistsInSameCommitAsVersion pins the write contract:
// one PublishWithEvidence call leaves BOTH the version row and its evidence
// member, and the evidence read answers exactly the pinned facts.
func TestCraftT07EvidencePersistsInSameCommitAsVersion(t *testing.T) {
	db := openCraftDB(t)
	// The central migration (INT sqlite 000130) owns this schema on the
	// migrated chain openCraftDB provides; asserting it exists beats
	// re-AutoMigrating over it (GORM sqlite table-rebuild misparses the
	// migration FOREIGN KEY clause as a column).
	require.True(t, db.Migrator().HasTable(&craftVersionEvidenceRow{}), "central migration must create craft_version_evidence")
	store := &CraftVersionStore{db: db}
	ws := putCraftWorkspace(t, NewCraftStore(db))
	scope := craftTestScope()
	acquired := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)

	files := []craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")}
	digest := mustDigest(t, files)
	versionID := craft.VersionID(ws.ID, "run-1", digest)
	evidence := t07EvidenceFor(t, versionID, "run-1", acquired)

	published, err := store.PublishWithEvidence(context.Background(), scope, craft.Version{
		ID: versionID, WorkspaceID: ws.ID, RunID: "run-1", Kind: craft.KindWeb,
		Files: files, Checks: []craft.Check{{Name: craft.CheckEntry, Status: craft.CheckPassed}},
	}, evidence)
	require.NoError(t, err)
	require.Equal(t, versionID, published.ID)

	loaded, err := store.VersionEvidence(context.Background(), scope, versionID)
	require.NoError(t, err)
	require.Equal(t, evidence, loaded, "the pinned evidence answers verbatim")
	require.Equal(t, versionID, loaded.VersionID)
	require.Len(t, loaded.Sources, 1)
	require.Equal(t, t07RepoDigest("source-run-1"), loaded.Sources[0].Digest)
	require.Equal(t, acquired, loaded.Sources[0].AcquiredAt)
}

// TestCraftT07EvidenceImmutableAcrossReplaysAndLaterVersions pins the
// immutability contract: the identical replay adopts the stored rows, a
// DIFFERENT evidence under the same version identity is a conflict that
// changes nothing, and a later version's different evidence never mutates
// the earlier version's pinned facts.
func TestCraftT07EvidenceImmutableAcrossReplaysAndLaterVersions(t *testing.T) {
	db := openCraftDB(t)
	// The central migration (INT sqlite 000130) owns this schema on the
	// migrated chain openCraftDB provides; asserting it exists beats
	// re-AutoMigrating over it (GORM sqlite table-rebuild misparses the
	// migration FOREIGN KEY clause as a column).
	require.True(t, db.Migrator().HasTable(&craftVersionEvidenceRow{}), "central migration must create craft_version_evidence")
	store := &CraftVersionStore{db: db}
	ws := putCraftWorkspace(t, NewCraftStore(db))
	scope := craftTestScope()
	ctx := context.Background()
	acquired := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)

	v1Files := []craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")}
	v1ID := craft.VersionID(ws.ID, "run-1", mustDigest(t, v1Files))
	v1Evidence := t07EvidenceFor(t, v1ID, "run-1", acquired)
	v1 := craft.Version{ID: v1ID, WorkspaceID: ws.ID, RunID: "run-1", Kind: craft.KindWeb, Files: v1Files}

	_, err := store.PublishWithEvidence(ctx, scope, v1, v1Evidence)
	require.NoError(t, err)

	// The identical replay adopts the stored evidence.
	_, err = store.PublishWithEvidence(ctx, scope, v1, v1Evidence)
	require.NoError(t, err)

	// A different evidence under the same version identity is a conflict.
	forged := v1Evidence
	forged.PackageDigest = t07RepoDigest("forged")
	_, err = store.PublishWithEvidence(ctx, scope, v1, forged)
	require.ErrorIs(t, err, craft.ErrConflict)
	kept, err := store.VersionEvidence(ctx, scope, v1ID)
	require.NoError(t, err)
	require.Equal(t, v1Evidence, kept, "the refused evidence changed nothing")

	// A later version with different knowledge pins different evidence and
	// leaves the old version's facts untouched.
	v2Files := []craft.File{craftTestFile(t, "index.html", "<h1>v2 updated knowledge</h1>")}
	v2ID := craft.VersionID(ws.ID, "run-2", mustDigest(t, v2Files))
	v2Evidence := t07EvidenceFor(t, v2ID, "run-2", acquired.Add(time.Hour))
	_, err = store.PublishWithEvidence(ctx, scope, craft.Version{
		ID: v2ID, WorkspaceID: ws.ID, RunID: "run-2", Kind: craft.KindWeb, Files: v2Files,
	}, v2Evidence)
	require.NoError(t, err)

	v1After, err := store.VersionEvidence(ctx, scope, v1ID)
	require.NoError(t, err)
	require.Equal(t, v1Evidence, v1After, "an old version's evidence never changes after a later publish")
	require.NotEqual(t, v1Evidence.Sources[0].Digest, v2Evidence.Sources[0].Digest)
}

// TestCraftT07EvidenceReadNeverReconstructs pins the read contract: a
// version published without evidence answers ErrNotFound — history is never
// rebuilt from Workspace or knowledge state — a tampered row is refused by
// its integrity digest, and the read carries the version scope ACL.
func TestCraftT07EvidenceReadNeverReconstructs(t *testing.T) {
	db := openCraftDB(t)
	// The central migration (INT sqlite 000130) owns this schema on the
	// migrated chain openCraftDB provides; asserting it exists beats
	// re-AutoMigrating over it (GORM sqlite table-rebuild misparses the
	// migration FOREIGN KEY clause as a column).
	require.True(t, db.Migrator().HasTable(&craftVersionEvidenceRow{}), "central migration must create craft_version_evidence")
	store := &CraftVersionStore{db: db}
	ws := putCraftWorkspace(t, NewCraftStore(db))
	scope := craftTestScope()
	ctx := context.Background()
	acquired := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)

	// A version published through the plain Publish path has NO evidence
	// member; the read answers ErrNotFound instead of guessing.
	bare := publishCraftVersion(t, store, scope, ws.ID, "run-bare", []craft.File{craftTestFile(t, "index.html", "<h1>bare</h1>")})
	_, err := store.VersionEvidence(ctx, scope, bare.ID)
	require.ErrorIs(t, err, craft.ErrNotFound)

	// A pinned version whose stored bytes were tampered with is refused by
	// the integrity digest, never silently served.
	files := []craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")}
	pinnedID := craft.VersionID(ws.ID, "run-1", mustDigest(t, files))
	evidence := t07EvidenceFor(t, pinnedID, "run-1", acquired)
	_, err = store.PublishWithEvidence(ctx, scope, craft.Version{
		ID: pinnedID, WorkspaceID: ws.ID, RunID: "run-1", Kind: craft.KindWeb, Files: files,
	}, evidence)
	require.NoError(t, err)
	require.NoError(t, db.Model(&craftVersionEvidenceRow{}).
		Where("version_id = ?", pinnedID).
		Update("evidence_json", `{"version_id":"`+pinnedID+`","run_id":"run-1","sources":[]}`).Error)
	_, err = store.VersionEvidence(ctx, scope, pinnedID)
	// T08's round-3 OCR ruling replaced the ErrConflict mapping with the
	// ErrCorruptEvidence sentinel (digest mismatch is server-side corruption,
	// 500-class, not a client-retryable conflict) — the tamper refusal still
	// stands, under the reviewed contract.
	require.ErrorIs(t, err, craft.ErrCorruptEvidence)

	// The read carries the same scope ACL as the version itself: a foreign
	// tenant or session sees nothing, a foreign owner is forbidden.
	foreignTenant := scope
	foreignTenant.TenantID = 2
	_, err = store.VersionEvidence(ctx, foreignTenant, pinnedID)
	require.ErrorIs(t, err, craft.ErrNotFound)
	foreignSession := scope
	foreignSession.SessionID = "s-other"
	_, err = store.VersionEvidence(ctx, foreignSession, pinnedID)
	require.ErrorIs(t, err, craft.ErrNotFound)

	// A fabricated version id answers ErrNotFound.
	_, err = store.VersionEvidence(ctx, scope, craft.VersionID(ws.ID, "run-x", t07RepoDigest("never")))
	require.ErrorIs(t, err, craft.ErrNotFound)
}

// TestCraftT07EvidencelessVersionIsNeverRetroPinned pins the T07 OCR fix:
// a version published by an earlier, evidence-less route (a pre-T07
// deployment upgrade, or the evidence-less Publish paths still serving
// other collections) is never retro-pinned by a later PublishWithEvidence
// — replayed promotion callbacks included. Retro-pinning would reconstruct
// history under a fresh PinnedAt, so the write is refused like different
// content and the read keeps its honest ErrNotFound.
func TestCraftT07EvidencelessVersionIsNeverRetroPinned(t *testing.T) {
	db := openCraftDB(t)
	// The central migration (INT sqlite 000130) owns this schema on the
	// migrated chain openCraftDB provides; asserting it exists beats
	// re-AutoMigrating over it (GORM sqlite table-rebuild misparses the
	// migration FOREIGN KEY clause as a column).
	require.True(t, db.Migrator().HasTable(&craftVersionEvidenceRow{}), "central migration must create craft_version_evidence")
	store := &CraftVersionStore{db: db}
	ws := putCraftWorkspace(t, NewCraftStore(db))
	scope := craftTestScope()
	ctx := context.Background()
	acquired := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)

	files := []craft.File{craftTestFile(t, "index.html", "<h1>legacy</h1>")}
	vID := craft.VersionID(ws.ID, "run-legacy", mustDigest(t, files))
	v := craft.Version{ID: vID, WorkspaceID: ws.ID, RunID: "run-legacy", Kind: craft.KindWeb, Files: files}

	// The evidence-less route this store still serves publishes without
	// evidence — exactly the pre-T07 upgrade shape.
	_, err := store.Publish(ctx, scope, v)
	require.NoError(t, err)

	// A later (replayed) promotion callback carrying evidence must NOT
	// back-fill the historical row: it is refused like different content...
	_, err = store.PublishWithEvidence(ctx, scope, v, t07EvidenceFor(t, vID, "run-legacy", acquired))
	require.ErrorIs(t, err, craft.ErrConflict, "an evidence-less historical version is never retro-pinned")

	// ...and the read keeps its honest not-found — history is never
	// reconstructed under a fresh PinnedAt.
	_, err = store.VersionEvidence(ctx, scope, vID)
	require.ErrorIs(t, err, craft.ErrNotFound, "the evidence-less version stays evidence-less")
}
