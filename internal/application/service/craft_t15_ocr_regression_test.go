package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// The OCR hardening round for the T15 promotion fence and the session's
// default-seat degradation.

// TestCraftT15StaleCandidateRidingCurrentRevisionIsRefused pins the fence's
// new identity binding: an OLD candidate whose callback claims the CURRENT
// head revision (with a DIFFERENT run's seal on the head) is refused —
// previously only the revision number was compared, so the stale files
// would silently promote and take the default seat.
func TestCraftT15StaleCandidateRidingCurrentRevisionIsRefused(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	const workspaceID = "ws-t15-ocr"

	filesSvc := newDirBackedFileService(t)
	versionStore := newMemVersionStore()
	candidateStore := &memoryCandidateStore{}
	draftHeads := &stubDraftHeadStore{heads: map[string]craft.DraftHead{}}
	probe := &scriptedPageProbe{}
	buildEvidence := func(context.Context, craft.Task) craft.ArtifactEvidence {
		return craft.ArtifactEvidence{ClaimedSuccess: true, BuildRan: true, BuildExitCode: 0}
	}
	svc := NewCraftArtifactServiceWithCandidates(
		craftSourceWith(nil, nil), filesSvc, versionStore, candidateStore, buildEvidence,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	).WithWebPromotion(draftHeads, probe).WithWebBuildReceipt(verifiedWebBuildReceipts{})

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

	// run-1 promotes normally at revision 1.
	run1 := stage("run-1", "<h1>v1</h1>", 1)
	probe.set(craft.WebCheckPassed, craft.WebCheckPassed)
	_, err := promote("run-1", run1, 1)
	require.NoError(t, err)

	// run-2 stages (revision 2, run-2's seal) but is NOT promoted.
	run2 := stage("run-2", "<h1>v2</h1>", 2)

	// run-3 stages and promotes at revision 3 — the head now carries
	// run-3's seal and manifest.
	run3 := stage("run-3", "<h1>v3</h1>", 3)
	_, err = promote("run-3", run3, 3)
	require.NoError(t, err)

	// THE ATTACK: run-2's stale callback claims the CURRENT head revision
	// (3). The revision number matches; only the head's SourceRunID/
	// ManifestDigest binding exposes the lie. The promotion must be refused.
	_, err = promote("run-2", run2, 3)
	require.ErrorIs(t, err, craft.ErrConflict, "a stale candidate riding the current revision must be refused by the identity binding")
	require.Contains(t, err.Error(), "sealed from run", "the refusal names the head's sealing run")

	// And the stale files never became a version or took the default seat.
	list, lerr := versionStore.List(ctx, scope)
	require.NoError(t, lerr)
	require.Len(t, list, 2, "only run-1 and run-3 published versions")
	defaultVersion, ok, derr := svc.SelectDefaultVersion(ctx, scope)
	require.NoError(t, derr)
	require.True(t, ok)
	require.NotEqual(t, run2.RunID, defaultVersion.RunID, "the stale run never took the seat")
}

// TestCraftT15EmptyHeadCannotPromoteUnsealedContent pins the state leg:
// an empty head (revision 0, capture not yet sealed) refuses promotion of
// not-yet-captured content even though the revision number matches.
func TestCraftT15EmptyHeadCannotPromoteUnsealedContent(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	const workspaceID = "ws-t15-empty"

	filesSvc := newDirBackedFileService(t)
	versionStore := newMemVersionStore()
	candidateStore := &memoryCandidateStore{}
	// Head exists at revision 0 but in the EMPTY state (capture pending).
	draftHeads := &stubDraftHeadStore{heads: map[string]craft.DraftHead{
		workspaceID: {WorkspaceID: workspaceID, Revision: 0, State: craft.DraftHeadEmpty},
	}}
	probe := &scriptedPageProbe{}
	buildEvidence := func(context.Context, craft.Task) craft.ArtifactEvidence {
		return craft.ArtifactEvidence{BuildRan: true, BuildExitCode: 0}
	}
	svc := NewCraftArtifactServiceWithCandidates(
		craftSourceWith(nil, nil), filesSvc, versionStore, candidateStore, buildEvidence,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	).WithWebPromotion(draftHeads, probe).WithWebBuildReceipt(verifiedWebBuildReceipts{})

	task := craftArtifactTask(scope.SessionID, workspaceID, "run-early")
	task.Scope = scope
	candidate, err := svc.CollectCandidate(ctx, task, craft.KindWeb, candidateSource("run-early", "gen-early", "<h1>early</h1>"), "gen-early")
	require.NoError(t, err)
	probe.set(craft.WebCheckPassed, craft.WebCheckPassed)
	_, err = svc.PromoteWebVersion(ctx, scope, craft.WebPromotionRequest{
		WorkspaceID: workspaceID, RunID: "run-early", CandidateID: candidate.ID, Revision: 0,
	})
	require.ErrorIs(t, err, craft.ErrConflict, "unsealed (empty-state) content cannot be promoted at revision 0")
	require.Contains(t, err.Error(), "sealed draft head", "the refusal names the state leg")
}

// TestCraftSessionViewSelectorErrorLeavesSeatEmpty pins the conservative
// degradation: a selector failure keeps the default seat EMPTY (never falls
// back to the unverified newest version) and the read still succeeds.
func TestCraftSessionViewSelectorErrorLeavesSeatEmpty(t *testing.T) {
	svc, sessionID := newSessionViewEnvWithSelector(t, craft.KindWeb, func(context.Context, craft.Scope) (craft.Version, bool, error) {
		return craft.Version{}, false, context.Canceled
	})
	view, err := svc.View(craftCtx(1, "u1", ""), craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID})
	require.NoError(t, err, "the read survives the selector failure")
	require.Nil(t, view.CurrentVersion, "the seat stays empty — an unverified latest version must not take it")
}
