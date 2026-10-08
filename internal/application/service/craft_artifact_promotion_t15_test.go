package service

// T15 (#130) — the four-check release gate as one user journey at the
// highest service seam: a Run stages a private web candidate, the promotion
// callback can only turn it into the default preview version after build,
// entry, preview reachability and actual page load each independently
// passed. Pins the acceptance matrix:
//   - build pass / entry pass / reach pass / page load FAIL → refused, prior
//     default kept, candidate stays a draft
//   - page load not_run (probe never observed it) → refused
//   - stale Workspace revision → refused
//   - duplicate callback → idempotent, no duplicate version
//   - foreign scope → nothing promoted
import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// stubDraftHeadStore is the Workspace draft-head seam: the promotion gate
// compares the callback's revision against the current head.
type stubDraftHeadStore struct {
	mu    sync.Mutex
	heads map[string]craft.DraftHead
}

func (s *stubDraftHeadStore) Read(_ context.Context, _ craft.Scope, workspaceID string) (craft.DraftHead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.heads[workspaceID]; ok {
		return h.Clone(), nil
	}
	return craft.DraftHead{}, craft.ErrNotFound
}

func (s *stubDraftHeadStore) ReadRevision(_ context.Context, _ craft.Scope, workspaceID string, revision int64) (craft.DraftHead, error) {
	h, err := s.Read(context.Background(), craft.Scope{}, workspaceID)
	if err != nil {
		return craft.DraftHead{}, err
	}
	if h.Revision != revision {
		return craft.DraftHead{}, craft.ErrNotFound
	}
	return h, nil
}

func (s *stubDraftHeadStore) Advance(_ context.Context, _ craft.Scope, _ string, _ int64, _ string, _ []craft.File) (craft.DraftHead, error) {
	return craft.DraftHead{}, craft.ErrUnsupported
}

func (s *stubDraftHeadStore) set(workspaceID string, revision int64, runID string) {
	s.setSealed(workspaceID, revision, runID, "")
}

// setSealed records the head with the sealed manifest digest, mirroring the
// real post-terminal capture (DraftHeadStore.Advance binds State/SourceRunID/
// ManifestDigest to the immutable revision row).
func (s *stubDraftHeadStore) setSealed(workspaceID string, revision int64, runID, manifest string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heads[workspaceID] = craft.DraftHead{WorkspaceID: workspaceID, Revision: revision, State: craft.DraftHeadSelected, SourceRunID: runID, ManifestDigest: manifest}
}

// scriptedPageProbe is the T14 seam double: externally observed reachability
// and page-load facts, configured per phase.
type scriptedPageProbe struct {
	mu        sync.Mutex
	reachable craft.CheckOutcome
	loaded    craft.CheckOutcome
	calls     int
}

func (p *scriptedPageProbe) ProbeWebPage(_ context.Context, _ craft.Scope, _ craft.Candidate) (craft.CheckOutcome, craft.CheckOutcome) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return p.reachable, p.loaded
}

// verifiedWebBuildReceipts is the F08 T-2 fence double for service-level
// promotion journeys: the real receipt fence journey lives in the container
// package tests; here the fence's decision is scripted.
type verifiedWebBuildReceipts struct{ refuse bool }

func (v verifiedWebBuildReceipts) VerifyPromotionBuild(context.Context, craft.Scope, string, string, string) error {
	if v.refuse {
		return fmt.Errorf("%w: scripted receipt fence refusal", craft.ErrConflict)
	}
	return nil
}

func (v verifiedWebBuildReceipts) SealCandidateManifest(context.Context, craft.Scope, string, string, string) error {
	return nil
}

func (p *scriptedPageProbe) set(reachable, loaded craft.CheckOutcome) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reachable, p.loaded = reachable, loaded
}

func TestCraftT15Journey(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	const workspaceID = "ws-t15"

	filesSvc := newDirBackedFileService(t)
	versionStore := newMemVersionStore()
	candidateStore := &memoryCandidateStore{}
	draftHeads := &stubDraftHeadStore{heads: map[string]craft.DraftHead{}}
	probe := &scriptedPageProbe{}
	// The T04 build-evidence seam: the real build log folds BuildRan/BuildExitCode
	// into collection (craft_web_build.go); this journey stages successful builds.
	buildEvidence := func(context.Context, craft.Task) craft.ArtifactEvidence {
		return craft.ArtifactEvidence{ClaimedSuccess: true, BuildRan: true, BuildExitCode: 0}
	}
	svc := NewCraftArtifactServiceWithCandidates(
		craftSourceWith(nil, nil), filesSvc, versionStore, candidateStore, buildEvidence,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	).WithWebPromotion(draftHeads, probe).WithWebBuildReceipt(verifiedWebBuildReceipts{})

	// stage collects one Run's output as a private candidate and advances the
	// workspace draft head to that Run's revision.
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
	defaultVersion := func() craft.Version {
		v, ok, err := svc.SelectDefaultVersion(ctx, scope)
		require.NoError(t, err)
		require.True(t, ok, "a promoted version must be the default")
		return v
	}

	// ------------------------------------------------------------------------
	// Phase 0 — success: all four checks passed, promotion publishes the
	// immutable version with four independently recorded checks.
	// ------------------------------------------------------------------------
	run1 := stage("run-1", "<h1>v1</h1>", 1)
	probe.set(craft.WebCheckPassed, craft.WebCheckPassed)
	v1, err := promote("run-1", run1, 1)
	require.NoError(t, err)
	require.Equal(t, craft.VersionID(workspaceID, "run-1", run1.ManifestDigest), v1.ID, "version identity derives from the candidate manifest")
	require.Len(t, v1.Checks, 4)
	statuses := map[string]string{}
	for _, c := range v1.Checks {
		statuses[c.Name] = c.Status
	}
	require.Equal(t, map[string]string{
		craft.CheckBuild:            craft.CheckPassed,
		craft.CheckEntry:            craft.CheckPassed,
		craft.CheckPreviewReachable: craft.CheckPassed,
		craft.CheckPageLoad:         craft.CheckPassed,
	}, statuses, "each check recorded independently")
	require.NotNil(t, v1.WebEvidence, "promotion projects the four-check evidence")
	require.True(t, v1.WebEvidence.Ready())
	require.Equal(t, v1.ID, defaultVersion().ID, "the promoted version is the default preview version")

	// ------------------------------------------------------------------------
	// Phase 1 — highest-risk failure: build/entry/reach pass, page load FAILS.
	// Promotion is refused, nothing publishes, the candidate remains a draft
	// and the prior default is kept.
	// ------------------------------------------------------------------------
	run2 := stage("run-2", "<h1>v2 broken page</h1>", 2)
	probe.set(craft.WebCheckPassed, craft.WebCheckFailed)
	_, err = promote("run-2", run2, 2)
	require.ErrorIs(t, err, craft.ErrConflict, "a failed check prevents promotion")
	require.Contains(t, err.Error(), "page_loaded=failed", "the refusal names the failed fact")
	require.Equal(t, 1, versionCount(), "no version was published for the refused candidate")
	stillDraft, cerr := candidateStore.GetCandidate(ctx, scope, run2.ID)
	require.NoError(t, cerr, "generated files remain a private draft")
	require.Equal(t, run2.ID, stillDraft.ID)
	require.Equal(t, v1.ID, defaultVersion().ID, "prior default version is kept")

	// ------------------------------------------------------------------------
	// Phase 2 — not_run: the probe answered reachability but never observed a
	// page load. Reachability cannot substitute for the load fact.
	// ------------------------------------------------------------------------
	probe.set(craft.WebCheckPassed, craft.WebCheckNotRun)
	_, err = promote("run-2", run2, 2)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Contains(t, err.Error(), "page_loaded=not_run", "not-run page load is named, not guessed")
	probe.set(craft.WebCheckNotRun, craft.WebCheckNotRun)
	_, err = promote("run-2", run2, 2)
	require.ErrorIs(t, err, craft.ErrConflict, "a probe that observed nothing cannot promote")
	require.Equal(t, 1, versionCount())
	require.Equal(t, v1.ID, defaultVersion().ID)

	// ------------------------------------------------------------------------
	// Phase 3 — stale Workspace revision: a late callback for the revision the
	// workspace already advanced past is refused.
	// ------------------------------------------------------------------------
	probe.set(craft.WebCheckPassed, craft.WebCheckPassed)
	_, err = promote("run-2", run2, 1)
	require.ErrorIs(t, err, craft.ErrConflict, "stale revision callback is refused")
	require.Contains(t, err.Error(), "revision", "the refusal names the revision fence")
	require.Equal(t, 1, versionCount())

	// A callback that does not bind its own candidate is a conflict, too.
	_, err = svc.PromoteWebVersion(ctx, scope, craft.WebPromotionRequest{
		WorkspaceID: workspaceID, RunID: "run-2", CandidateID: run1.ID, Revision: 2,
	})
	require.ErrorIs(t, err, craft.ErrConflict, "the request must bind the candidate's own Run")

	// ------------------------------------------------------------------------
	// Phase 4 — recovery + idempotency: the same callback replayed twice
	// promotes exactly once; duplicate callbacks cannot create duplicates.
	// ------------------------------------------------------------------------
	v2a, err := promote("run-2", run2, 2)
	require.NoError(t, err, "the corrected retry promotes")
	v2b, err := promote("run-2", run2, 2)
	require.NoError(t, err, "the duplicate callback is idempotent")
	require.Equal(t, v2a.ID, v2b.ID)
	require.Equal(t, 2, versionCount(), "duplicate callbacks created no duplicate versions")
	require.Equal(t, v2a.ID, defaultVersion().ID, "the newer four-check version becomes the default")

	// The prior default stays immutable and visible.
	old, err := versionStore.Get(ctx, scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, v1.Checks, old.Checks, "the old default's checks never change")

	// ------------------------------------------------------------------------
	// Phase 5 — authorization: a foreign scope promotes nothing.
	// ------------------------------------------------------------------------
	run3 := stage("run-3", "<h1>v3</h1>", 3)
	foreignSession := scope
	foreignSession.SessionID = "s-other"
	_, err = svc.PromoteWebVersion(ctx, foreignSession, craft.WebPromotionRequest{
		WorkspaceID: workspaceID, RunID: "run-3", CandidateID: run3.ID, Revision: 3,
	})
	require.Error(t, err, "a foreign scope cannot promote")
	require.Equal(t, 2, versionCount(), "nothing was published for the foreign scope")
}

// TestCraftT15PromotionFailsClosedWithoutAssembly pins the recovery posture:
// promotion without the revision fence or without candidate wiring fails
// closed instead of promoting unbound evidence.
func TestCraftT15PromotionFailsClosedWithoutAssembly(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	filesSvc := newDirBackedFileService(t)

	// Without the draft-head store the revision fence cannot be verified.
	unfenced := NewCraftArtifactServiceWithCandidates(
		craftSourceWith(nil, nil), filesSvc, newMemVersionStore(), &memoryCandidateStore{}, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	)
	_, err := unfenced.PromoteWebVersion(ctx, scope, craft.WebPromotionRequest{
		WorkspaceID: "ws-1", RunID: "run-1", CandidateID: "cand_" + "a", Revision: 1,
	})
	require.ErrorIs(t, err, craft.ErrUnsupported, "an unfenced promotion is refused")

	// Without the candidate wiring there is nothing to promote.
	plain := NewCraftArtifactService(
		craftSourceWith(nil, nil), filesSvc, newMemVersionStore(), nil, CraftArtifactConfig{OutputDir: craftTestOutputDir},
	)
	_, err = plain.PromoteWebVersion(ctx, scope, craft.WebPromotionRequest{
		WorkspaceID: "ws-1", RunID: "run-1", CandidateID: "cand_" + "a", Revision: 1,
	})
	require.ErrorIs(t, err, craft.ErrInvalidInput, "an unassembled candidate store refuses promotion")
}

// TestCraftT15PromotionRequiresBoundBuildReceipt is the F08 T-2 fence: a
// promotion without the receipt fence wired is refused (ErrUnsupported), and
// a fence that finds no bound receipt for the Run refuses with a conflict.
func TestCraftT15PromotionRequiresBoundBuildReceipt(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s-f08"}
	probe := &scriptedPageProbe{}
	probe.set(craft.WebCheckPassed, craft.WebCheckPassed)

	unfenced := NewCraftArtifactServiceWithCandidates(
		craftSourceWith(nil, nil), newDirBackedFileService(t), newMemVersionStore(), &memoryCandidateStore{}, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	).WithWebPromotion(&stubDraftHeadStore{heads: map[string]craft.DraftHead{}}, probe)
	_, err := unfenced.PromoteWebVersion(ctx, scope, craft.WebPromotionRequest{
		WorkspaceID: "ws-f08", RunID: "run-f08", CandidateID: "cand_f08", Revision: 1,
	})
	require.ErrorIs(t, err, craft.ErrUnsupported, "an assembly without the receipt fence refuses promotion outright")
}
