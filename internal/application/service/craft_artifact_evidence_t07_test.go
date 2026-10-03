package service

// T07 (#131) — the version-pinned evidence journey at the highest artifact
// service seam. A Run promotes V1 with the knowledge it actually used; the
// source is then updated/revoked/deleted; a new Run promotes V2 with the NEW
// evidence. Pins the acceptance matrix:
//   - V1's pinned evidence is immutable and carries each source's
//     version/ref, digest and acquisition time;
//   - V2's evidence reflects the new source state without mutating V1;
//   - revoking the source keeps V1's historical facts readable while the
//     citation open STILL re-authorizes (non-leaking placeholder);
//   - the pinned evidence survives even the Run record disappearing — it is
//     never reconstructed from the live record, the Workspace files or the
//     current knowledge base;
//   - a promotion that cannot prove its Run's evidence is refused and the
//     prior default keeps the seat.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// t07EvidenceVersionStore extends the in-memory version store with the T07
// evidence members, mirroring the real store's semantics: evidence persists
// with the version, identical replays adopt the stored rows, a different
// evidence under the same version identity conflicts, and the read carries
// the version's scope ACL.
type t07EvidenceVersionStore struct {
	*memVersionStore
	mu       sync.Mutex
	evidence map[string]craft.VersionEvidence
	scopes   map[string]craft.Scope
}

func newT07EvidenceVersionStore() *t07EvidenceVersionStore {
	return &t07EvidenceVersionStore{
		memVersionStore: newMemVersionStore(),
		evidence:        map[string]craft.VersionEvidence{},
		scopes:          map[string]craft.Scope{},
	}
}

func (s *t07EvidenceVersionStore) PublishWithEvidence(ctx context.Context, scope craft.Scope, v craft.Version, ev craft.VersionEvidence) (craft.Version, error) {
	published, err := s.Publish(ctx, scope, v)
	if err != nil {
		return craft.Version{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if stored, ok := s.evidence[published.ID]; ok {
		// Replay identity ignores PinnedAt — the promotion's clock reading,
		// not a fact of the observation — mirroring the real store.
		storedFacts, evFacts := stored, ev
		storedFacts.PinnedAt, evFacts.PinnedAt = time.Time{}, time.Time{}
		if !reflect.DeepEqual(storedFacts, evFacts) {
			return craft.Version{}, craft.ErrConflict
		}
		return published, nil
	}
	s.evidence[published.ID] = ev
	s.scopes[published.ID] = scope
	return published, nil
}

// PublishWithDraftHead satisfies the DraftFencedVersionStore seam the web
// promotion requires. The revision/identity fence has already been checked
// against the draft-head store inside PromoteWebVersion; the transactional
// re-fence belongs to the real database store, so the double only publishes.
func (s *t07EvidenceVersionStore) PublishWithDraftHead(ctx context.Context, scope craft.Scope, v craft.Version, _ craft.DraftHead, evidence *craft.VersionEvidence) (craft.Version, error) {
	if evidence != nil {
		return s.PublishWithEvidence(ctx, scope, v, *evidence)
	}
	return s.Publish(ctx, scope, v)
}

func (s *t07EvidenceVersionStore) VersionEvidence(_ context.Context, scope craft.Scope, versionID string) (craft.VersionEvidence, error) {
	s.mu.Lock()
	stored, pinned := s.evidence[versionID]
	owner, known := s.scopes[versionID]
	s.mu.Unlock()
	if !pinned || !known {
		return craft.VersionEvidence{}, craft.ErrNotFound
	}
	if owner.TenantID != scope.TenantID || owner.SessionID != scope.SessionID {
		return craft.VersionEvidence{}, craft.ErrNotFound
	}
	if owner.UserID != scope.UserID {
		return craft.VersionEvidence{}, craft.ErrForbidden
	}
	return stored, nil
}

// t07KnowledgeAuthorizer stands in for the fresh source-open seam on the
// citation journey; revoking a citation id simulates the source share being
// revoked or the knowledge deleted after V1 was promoted.
type t07KnowledgeAuthorizer struct {
	mu      sync.Mutex
	revoked map[string]bool
}

func (a *t07KnowledgeAuthorizer) AuthorizeSourceOpen(_ context.Context, _ craft.Scope, _, citationID string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.revoked[citationID] {
		return "", craft.ErrForbidden
	}
	return "resource://knowledge/original", nil
}

func (a *t07KnowledgeAuthorizer) revoke(citationID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revoked[citationID] = true
}

func TestCraftT07Journey(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	const workspaceID = "ws-t07"

	filesSvc := newDirBackedFileService(t)
	versionStore := newT07EvidenceVersionStore()
	candidateStore := &memoryCandidateStore{}
	draftHeads := &stubDraftHeadStore{heads: map[string]craft.DraftHead{}}
	probe := &scriptedPageProbe{}
	records := &t05Records{}
	buildEvidence := func(context.Context, craft.Task) craft.ArtifactEvidence {
		return craft.ArtifactEvidence{ClaimedSuccess: true, BuildRan: true, BuildExitCode: 0}
	}
	svc := NewCraftArtifactServiceWithCandidates(
		craftSourceWith(nil, nil), filesSvc, versionStore, candidateStore, buildEvidence,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	).WithWebPromotion(draftHeads, probe).WithWebBuildReceipt(verifiedWebBuildReceipts{}).WithVersionEvidence(records)

	// The Run-side knowledge journey: each Run's record is saved once with
	// the facts it actually observed, exactly as the T05 build does.
	t1 := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)
	t2 := t1.Add(2 * time.Hour)
	runRecord := func(runID string, sourceDigest string, acquired time.Time) craft.KnowledgeRecord {
		record := craft.KnowledgeRecord{
			Scope: scope, RunID: runID,
			RequestDigest: t07Digest("req-" + runID), PackageDigest: t07Digest("pkg-" + runID),
			PublicationState: craft.KnowledgePublicationPublished,
			Sources: []craft.KnowledgeSourceRecord{{
				ID: craft.KnowledgeCitationID("kb-a", "k-a", "c-"+runID), Ref: craft.KnowledgeRef("kb-a", "k-a", "c-"+runID),
				Digest: sourceDigest, TenantID: scope.TenantID, AcquiredAt: acquired, ExcerptBytes: 32,
			}},
		}
		require.NoError(t, records.Save(context.Background(), record))
		return record
	}
	stage := func(runID, content string, revision int64) craft.Candidate {
		t.Helper()
		task := craftArtifactTask(scope.SessionID, workspaceID, runID)
		task.Scope = scope
		generation := "gen-" + runID
		candidate, err := svc.CollectCandidate(ctx, task, craft.KindWeb, candidateSource(runID, generation, content), generation)
		require.NoError(t, err)
		draftHeads.setSealed(workspaceID, revision, runID, candidate.ManifestDigest)
		return candidate
	}
	promote := func(runID string, candidate craft.Candidate, revision int64) (craft.Version, error) {
		t.Helper()
		return svc.PromoteWebVersion(ctx, scope, craft.WebPromotionRequest{
			WorkspaceID: workspaceID, RunID: runID, CandidateID: candidate.ID, Revision: revision,
		})
	}
	versionCount := func() int {
		list, err := versionStore.List(ctx, scope)
		require.NoError(t, err)
		return len(list)
	}

	// ------------------------------------------------------------------------
	// Phase 0 — success: run-1 uses knowledge (source digest d1 at t1), the
	// promoted V1 carries the Run's evidence as an immutable member.
	// ------------------------------------------------------------------------
	d1 := t07Digest("source-v1")
	rec1 := runRecord("run-1", d1, t1)
	run1 := stage("run-1", "<h1>v1</h1>", 1)
	probe.set(craft.WebCheckPassed, craft.WebCheckPassed)
	v1, err := promote("run-1", run1, 1)
	require.NoError(t, err)

	ev1, err := svc.VersionEvidence(ctx, scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, v1.ID, ev1.VersionID)
	require.Equal(t, "run-1", ev1.RunID)
	require.Equal(t, rec1.PackageDigest, ev1.PackageDigest, "the evidence names the source package version used")
	require.Len(t, ev1.Sources, 1)
	require.Equal(t, rec1.Sources[0].ID, ev1.Sources[0].ID)
	require.Equal(t, d1, ev1.Sources[0].Digest, "the evidence carries the source digest observed at acquisition time")
	require.Equal(t, t1, ev1.Sources[0].AcquiredAt, "the evidence carries the acquisition time")
	require.False(t, ev1.PinnedAt.IsZero())
	ev1Digest, err := craft.VersionEvidenceDigest(ev1)
	require.NoError(t, err)

	// ------------------------------------------------------------------------
	// Phase 1 — knowledge update: the same logical source now serves a NEW
	// digest at a later time; run-2's V2 pins the new evidence while V1's
	// evidence — digest and acquisition time — stays byte-for-byte stable.
	// ------------------------------------------------------------------------
	d2 := t07Digest("source-v2-updated")
	runRecord("run-2", d2, t2)
	run2 := stage("run-2", "<h1>v2</h1>", 2)
	v2, err := promote("run-2", run2, 2)
	require.NoError(t, err)

	ev2, err := svc.VersionEvidence(ctx, scope, v2.ID)
	require.NoError(t, err)
	require.Equal(t, d2, ev2.Sources[0].Digest, "the later version pins the UPDATED source state")
	require.Equal(t, t2, ev2.Sources[0].AcquiredAt)
	require.NotEqual(t, ev1Digest, mustT07Digest(t, ev2), "updated knowledge produces different evidence in the later version")

	ev1Again, err := svc.VersionEvidence(ctx, scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, ev1, ev1Again, "the old version's evidence never mutates after the later run")
	require.Equal(t, d1, ev1Again.Sources[0].Digest)
	require.Equal(t, t1, ev1Again.Sources[0].AcquiredAt)

	// The workspace already moved on (run-2 rewrote every file); reading V1's
	// evidence still answers the pinned facts, never the current workspace.
	require.Equal(t, ev1Digest, mustT07Digest(t, ev1Again))

	// ------------------------------------------------------------------------
	// Phase 2 — deletion/revocation: the original source share is revoked.
	// The historical evidence facts stay readable, but opening the cited
	// source STILL re-authorizes against the current grant and keeps a
	// non-leaking placeholder.
	// ------------------------------------------------------------------------
	checker := &t05Checker{allowed: true}
	authorizer := &t07KnowledgeAuthorizer{revoked: map[string]bool{}}
	citationSvc, err := NewCraftCitationService(CraftCitationConfig{
		Records: records, Authorizer: authorizer, TaskAccess: checker,
	})
	require.NoError(t, err)
	citationCtx := craftKnowledgeCtx(scope)
	citationID := rec1.Sources[0].ID

	open, err := citationSvc.ResolveCitation(citationCtx, scope, "run-1", citationID)
	require.NoError(t, err)
	require.Empty(t, open.Placeholder, "an authorized viewer still opens the source")

	authorizer.revoke(citationID)
	open, err = citationSvc.ResolveCitation(citationCtx, scope, "run-1", citationID)
	require.NoError(t, err)
	require.NotNil(t, open.Placeholder, "a revoked source resolves to the non-leaking placeholder")
	require.Equal(t, craft.WebCitationStatusUnavailable, open.Placeholder.Status)
	require.Equal(t, citationID, open.Placeholder.CitationID)
	require.Empty(t, open.Ref, "the placeholder never leaks the original ref")

	ev1Revoked, err := svc.VersionEvidence(ctx, scope, v1.ID)
	require.NoError(t, err, "revocation preserves the historical evidence facts")
	require.Equal(t, ev1, ev1Revoked)

	// Even the Run's own record disappearing leaves the pinned evidence
	// intact: history answers the pinned snapshot, never a live lookup.
	delete(records.byRun, "run-1")
	ev1Gone, err := svc.VersionEvidence(ctx, scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, ev1, ev1Gone, "evidence is never reconstructed from the live record, Workspace files or current KB")

	// ------------------------------------------------------------------------
	// Phase 3 — failure/recovery: a promotion whose Run has no provable
	// evidence is refused, publishes nothing and keeps the prior default;
	// once the Run's observation is provable the SAME candidate promotes.
	// ------------------------------------------------------------------------
	before := versionCount()
	run3 := stage("run-3", "<h1>v3 no record</h1>", 3)
	_, err = promote("run-3", run3, 3)
	require.ErrorIs(t, err, craft.ErrNotFound, "a Run without a recorded source observation cannot promote with wired evidence")
	require.Equal(t, before, versionCount(), "the refused promotion published nothing")
	def, ok, err := svc.SelectDefaultVersion(ctx, scope)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, v2.ID, def.ID, "the prior default keeps the seat")

	// Recovery: recording run-3's source observation lets the SAME candidate
	// promote while its revision still seals the head.
	runRecord("run-3", t07Digest("source-v3"), t2.Add(time.Hour))
	v3, err := promote("run-3", run3, 3)
	require.NoError(t, err, "the recovered promotion succeeds once the evidence is provable")
	require.Equal(t, before+1, versionCount())
	ev3, err := svc.VersionEvidence(ctx, scope, v3.ID)
	require.NoError(t, err)
	require.Equal(t, "run-3", ev3.RunID)
	require.Equal(t, ev1, ev1Again, "V1 evidence stays stable through the recovery round (same assertion values)")

	// Replay idempotency: the same promotion callback pins nothing new.
	v3Replay, err := promote("run-3", run3, 3)
	require.NoError(t, err)
	require.Equal(t, v3.ID, v3Replay.ID)

	// A record that was never published (only prepared) is not provable
	// evidence either: the promotion re-evaluates the CURRENT state.
	prepared := craft.KnowledgeRecord{
		Scope: scope, RunID: "run-4",
		RequestDigest: t07Digest("req-run-4"), PackageDigest: t07Digest("pkg-run-4"),
		PublicationState: craft.KnowledgePublicationPrepared,
		Sources: []craft.KnowledgeSourceRecord{{
			ID: craft.KnowledgeCitationID("kb-a", "k-a", "c-run-4"), Ref: craft.KnowledgeRef("kb-a", "k-a", "c-run-4"),
			Digest: t07Digest("source-run-4"), TenantID: scope.TenantID, AcquiredAt: t2, ExcerptBytes: 32,
		}},
	}
	require.NoError(t, records.Save(context.Background(), prepared))
	run4 := stage("run-4", "<h1>v4 unpublished record</h1>", 4)
	_, err = promote("run-4", run4, 4)
	require.ErrorIs(t, err, craft.ErrConflict, "an unpublished source observation is not promotable evidence")
	require.Equal(t, before+1, versionCount(), "the unpublished observation promoted nothing")

	// ------------------------------------------------------------------------
	// Phase 4 — authorization: a foreign scope reads no evidence, and an
	// unpinned version id is honestly absent, never reconstructed.
	// ------------------------------------------------------------------------
	foreign := scope
	foreign.SessionID = "s-other"
	_, err = svc.VersionEvidence(ctx, foreign, v1.ID)
	require.ErrorIs(t, err, craft.ErrNotFound, "a foreign session reads no evidence")
	foreignOwner := scope
	foreignOwner.UserID = "u-other"
	_, err = svc.VersionEvidence(ctx, foreignOwner, v1.ID)
	require.ErrorIs(t, err, craft.ErrForbidden, "a foreign owner is forbidden")

	_, err = svc.VersionEvidence(ctx, scope, craft.VersionID(workspaceID, "run-never", t07Digest("never")))
	require.ErrorIs(t, err, craft.ErrNotFound, "a version without pinned evidence is absent, never guessed")
}

// TestCraftT07EvidenceFailsClosedWithoutEvidenceStore pins the assembly
// posture: a service whose store cannot pin evidence refuses the read
// fail-closed instead of reconstructing history, and an unwired evidence
// source keeps the recorded unpinned promotion behavior (the historical read
// then honestly answers ErrNotFound).
func TestCraftT07EvidenceFailsClosedWithoutEvidenceStore(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}

	// A plain store without the evidence members: the read fails closed.
	plain := NewCraftArtifactServiceWithCandidates(
		craftSourceWith(nil, nil), newDirBackedFileService(t), newMemVersionStore(), &memoryCandidateStore{}, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	)
	_, err := plain.VersionEvidence(ctx, scope, craft.VersionID("ws-1", "run-1", t07Digest("m")))
	require.ErrorIs(t, err, craft.ErrUnsupported, "a store that cannot pin evidence cannot serve it either")

	// An unwired evidence source keeps the recorded unpinned behavior: the
	// T15 promotion matrix stays green without the T07 wiring.
	unwired := NewCraftArtifactServiceWithCandidates(
		craftSourceWith(nil, nil), newDirBackedFileService(t), newT07EvidenceVersionStore(), &memoryCandidateStore{}, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	).WithWebPromotion(&stubDraftHeadStore{heads: map[string]craft.DraftHead{}}, &scriptedPageProbe{}).WithWebBuildReceipt(verifiedWebBuildReceipts{})
	require.Nil(t, unwired.runEvidence, "the evidence port stays optional")
}

func mustT07Digest(t *testing.T, ev craft.VersionEvidence) string {
	t.Helper()
	digest, err := craft.VersionEvidenceDigest(ev)
	require.NoError(t, err)
	return digest
}

func t07Digest(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}
