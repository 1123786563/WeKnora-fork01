package container

// F08 T-4: nine-class journey coverage of the fixed web build chain —
// dispatch → receipt → evidence → promotion — at the highest stable
// interface (real repositories + the dispatch fixture's fake provider):
// success, replay idempotency, poisoned handle, unobservable container,
// unsealed receipt rejection, nil-evidence not_run, unwired promotion
// ErrUnsupported, budget-denied, stop-cancel. The truncated-transport
// journey carries the OutputComplete-truthful-both-ways proof.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/stretchr/testify/require"
)

// f08JourneySource is the Run-bound collector source double: one index.html
// member under the pinned output dir, bound to the Run's verified generation.
type f08JourneySource struct {
	runID      string
	generation string
	content    string
}

func (s f08JourneySource) ListSessionFiles(context.Context, string, string) ([]sandbox.RemoteDirEntry, error) {
	return []sandbox.RemoteDirEntry{{
		Name: "index.html", Path: craftLocalOutputDir + "/index.html",
		Type: sandbox.RemoteEntryFile, Size: int64(len(s.content)),
	}}, nil
}

func (s f08JourneySource) ReadSessionFile(_ context.Context, _, path string) ([]byte, error) {
	if strings.HasSuffix(path, "index.html") {
		return []byte(s.content), nil
	}
	return nil, craft.ErrNotFound
}

func (s f08JourneySource) CraftArtifactRunID() string      { return s.runID }
func (s f08JourneySource) CraftArtifactGeneration() string { return s.generation }

// f08PartialTransportProvider leaves the provider transport incomplete: the
// process still succeeds with start evidence, but the stream was not fully
// drained — the receipt must stay TransportComplete=false and the seal must
// keep OutputComplete=false.
type f08PartialTransportProvider struct{ f08WebBuildNormalProvider }

func (p *f08PartialTransportProvider) StartAttachedExecOnce(_ context.Context, receipt sandbox.DockerNormalExecReceipt, _ bool, _ []byte, sink sandbox.DockerNormalExecChunkSink) (sandbox.DockerNormalExecOutcome, error) {
	p.started++
	if err := sink.Seal(); err != nil {
		return sandbox.DockerNormalExecOutcome{}, err
	}
	code := 0
	return sandbox.DockerNormalExecOutcome{
		Receipt: receipt, Transport: sandbox.DockerNormalExecTransportPartial,
		Observation:   sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded, ExitCode: &code},
		StartEvidence: true,
	}, nil
}

// f08JourneyArtifacts assembles the promotion-side artifact service over the
// real durable stores; fence=true wires the REAL F08 receipt fence adapter.
// evidence supplies the collector's build-evidence source (nil leaves the
// build check not_run).
func f08JourneyArtifacts(t *testing.T, fixture *f08WebBuildDispatchFixture, fence bool, evidence service.ArtifactEvidenceSource) *service.CraftArtifactService {
	t.Helper()
	artifacts := service.NewCraftArtifactServiceWithCandidates(
		closedCraftArtifactSource{}, newCaptureWiringFiles(t, fixture.db), repository.NewCraftVersionStore(fixture.db),
		repository.NewCraftCandidateStore(fixture.db), evidence,
		service.CraftArtifactConfig{Kind: craft.KindWeb, OutputDir: craftLocalOutputDir},
	).WithWebPromotion(repository.NewCraftDraftHeadStore(fixture.db), &t20ProbeStub{reachable: craft.WebCheckPassed, loaded: craft.WebCheckPassed})
	if fence {
		artifacts = artifacts.WithWebBuildReceipt(craftWebBuildPromotionReceipts{receipts: fixture.receipts, pin: fixture.pin})
	}
	return artifacts
}

// f08JourneyCollect stages one candidate through the real collector lane
// (stage → screen → upload → PutCandidate → receipt seal).
func f08JourneyCollect(t *testing.T, artifacts *service.CraftArtifactService, fixture *f08WebBuildDispatchFixture, content string) craft.Candidate {
	t.Helper()
	candidate, err := artifacts.CollectCandidate(context.Background(), fixture.task, craft.KindWeb,
		f08JourneySource{runID: fixture.task.Fence.RunID, generation: "generation-1", content: content}, "generation-1")
	require.NoError(t, err)
	return candidate
}

// f08JourneyPutWorkspace creates the Run's workspace row (the candidate
// scope validation binds the candidate to it before collection).
func f08JourneyPutWorkspace(t *testing.T, fixture *f08WebBuildDispatchFixture) {
	t.Helper()
	_, err := repository.NewCraftStore(fixture.db).PutWorkspace(context.Background(), craft.Workspace{
		Scope: fixture.task.Scope, ID: fixture.task.WorkspaceID,
		SandboxID: "sbx-f08", Generation: "0", RuntimeDigest: "digest-f08",
	}, 0)
	require.NoError(t, err)
}

// f08JourneySelectHead selects the candidate's draft head, the promotion
// gate's revision fence.
func f08JourneySelectHead(t *testing.T, fixture *f08WebBuildDispatchFixture, candidate craft.Candidate) int64 {
	t.Helper()
	// The draft-head capture is a post-terminal pass: the Run is terminal by
	// the time the head is selected.
	require.NoError(t, fixture.db.Exec("UPDATE agent_runs SET status = 'succeeded' WHERE tenant_id = ? AND run_id = ?",
		fixture.task.Scope.TenantID, fixture.task.Fence.RunID).Error)
	head, err := repository.NewCraftDraftHeadStore(fixture.db).Advance(context.Background(), fixture.task.Scope, fixture.task.WorkspaceID, 0, fixture.task.Fence.RunID, candidate.Files)
	require.NoError(t, err)
	require.Equal(t, craft.DraftHeadSelected, head.State)
	return head.Revision
}

// TestCraftF08JourneySuccessPublishesSealedWebVersion: the whole chain —
// dispatch earns the terminal receipt, the evidence source trusts it, the
// collector seals the candidate manifest into the receipt (OutputComplete
// flips truthful), and the four-check gate publishes the version.
func TestCraftF08JourneySuccessPublishesSealedWebVersion(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	ctx := context.Background()
	receipt, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", receipt.ProcessState)
	require.False(t, receipt.OutputComplete, "a fresh record predates collection")

	f08JourneyPutWorkspace(t, fixture)

	// The collector's evidence source is the production one: the trusted
	// build fact comes from the receipt (T-2), folded with a REAL build log
	// from the shipped toolchain matching the pins.
	workspace := t.TempDir()
	inputDir, outputDir := filepath.Join(workspace, "input"), filepath.Join(workspace, "output")
	craftWebTestContent(t, inputDir)
	code, buildLog := runCraftWebBuild(t, craftWebToolchainAbsDir(t), inputDir, outputDir, fixture.pin.RuntimeDigest)
	require.Zero(t, code, "offline build must succeed:\n%s", buildLog)
	rawLog, err := os.ReadFile(filepath.Join(outputDir, "build-log.json"))
	require.NoError(t, err)
	previewOnly := func(context.Context, craft.Task) craft.ArtifactEvidence {
		return craft.ArtifactEvidence{PreviewRan: true, PreviewPassed: true}
	}
	readLog := func(context.Context, craft.Task) ([]byte, error) { return rawLog, nil }
	evidence := craftWebBuildEvidenceSource(previewOnly, readLog, fixture.pin, craftWebBuildReceiptExitCode(fixture.receipts, fixture.pin))

	artifacts := f08JourneyArtifacts(t, fixture, true, evidence)
	candidate := f08JourneyCollect(t, artifacts, fixture, "<h1>f08 journey</h1>")

	sealed, err := fixture.receipts.Read(ctx, receipt.Key())
	require.NoError(t, err)
	require.Equal(t, candidate.ManifestDigest, sealed.CandidateManifestSHA256, "the collector seals the server-computed candidate digest")
	require.True(t, sealed.OutputComplete, "a complete build's receipt flips truthful with the seal")

	revision := f08JourneySelectHead(t, fixture, candidate)
	version, err := artifacts.PromoteWebVersion(ctx, fixture.task.Scope, craft.WebPromotionRequest{
		WorkspaceID: fixture.task.WorkspaceID, RunID: fixture.task.Fence.RunID, CandidateID: candidate.ID, Revision: revision,
	})
	require.NoError(t, err)
	require.Equal(t, craft.VersionID(fixture.task.WorkspaceID, fixture.task.Fence.RunID, candidate.ManifestDigest), version.ID)
}

// TestCraftF08JourneyReplayIsIdempotent: a recorded receipt IS the answer;
// replays never touch the engine and cannot double-record.
func TestCraftF08JourneyReplayIsIdempotent(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	ctx := context.Background()
	first, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.NoError(t, err)
	created := fixture.normal.created
	claimed := len(fixture.authority.claims)

	replay, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	require.Equal(t, created, fixture.normal.created, "a recorded receipt replay never reaches the engine")
	require.Equal(t, claimed, len(fixture.authority.claims), "a replay claims no new effect")
}

// TestCraftF08JourneyPoisonedPersistedContainerIDNeverMints: a stale
// persisted Runtime.ContainerID must never become the minted handle.
func TestCraftF08JourneyPoisonedPersistedContainerIDNeverMints(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	const poisoned = "poisoned-passthrough"
	view := fixture.authority.view
	view.State = craft.RunViewStateBound
	view.Runtime = craft.RunViewRuntime{RuntimeID: "runtime-stale", ContainerID: poisoned, OpenCodeSessionID: "session-stale"}
	intentAt := time.Now().UTC()
	view.SessionCreateIntentAt = &intentAt
	fixture.authority.view = view
	fixture.store.view = view

	receipt, err := fixture.dispatcher.Dispatch(context.Background(), fixture.task, fixture.grantID)
	require.NoError(t, err)
	require.Equal(t, fixture.normal.capturedHandleID, receipt.ContainerID)
	require.NotEqual(t, poisoned, receipt.ContainerID)
}

// TestCraftF08JourneyUnobservableContainerRefusesBeforeEngine: a container
// that cannot be observed running refuses dispatch before any engine call
// and leaves no receipt behind.
func TestCraftF08JourneyUnobservableContainerRefusesBeforeEngine(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	fixture.provider.observeStateErr = errors.New("engine unobservable")

	_, err := fixture.dispatcher.Dispatch(context.Background(), fixture.task, fixture.grantID)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Zero(t, fixture.normal.created)
	var receipts int64
	require.NoError(t, fixture.db.Table("craft_web_build_receipts").Count(&receipts).Error)
	require.Zero(t, receipts)
}

// TestCraftF08JourneyUnsealedReceiptCannotPromote: a dispatched-but-uncollected
// Run — the receipt exists but the collector never sealed a manifest (the run
// stopped between build and collection) — is refused by the promotion fence.
func TestCraftF08JourneyUnsealedReceiptCannotPromote(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	ctx := context.Background()
	_, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.NoError(t, err)

	f08JourneyPutWorkspace(t, fixture)
	artifacts := f08JourneyArtifacts(t, fixture, true, nil)
	// The candidate is staged directly (the collector lane is what seals);
	// this is exactly the honest pre-collection state of the Run.
	candidate := f08JourneyDirectCandidate(t, fixture, "<h1>unsealed</h1>")
	revision := f08JourneySelectHead(t, fixture, candidate)

	_, err = artifacts.PromoteWebVersion(ctx, fixture.task.Scope, craft.WebPromotionRequest{
		WorkspaceID: fixture.task.WorkspaceID, RunID: fixture.task.Fence.RunID, CandidateID: candidate.ID, Revision: revision,
	})
	require.ErrorIs(t, err, craft.ErrConflict, "an unsealed receipt can never pass the promotion fence")
}

// TestCraftF08JourneyNilEvidenceStaysNotRun: without a bound receipt the
// build fact stays unobserved (not_run), even when a matching log exists.
func TestCraftF08JourneyNilEvidenceStaysNotRun(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	ctx := context.Background()
	readReceipt := craftWebBuildReceiptExitCode(fixture.receipts, fixture.pin)
	inner := func(context.Context, craft.Task) craft.ArtifactEvidence {
		return craft.ArtifactEvidence{PreviewRan: true, PreviewPassed: true}
	}
	readLog := func(context.Context, craft.Task) ([]byte, error) {
		return nil, errors.New("no build log")
	}

	// No dispatch happened for this run: the receipt source is nil-for-this-run.
	unobserved := craftWebBuildEvidenceSource(inner, readLog, fixture.pin, readReceipt)(ctx, fixture.task)
	require.False(t, unobserved.BuildRan, "without a bound receipt the build fact stays unobserved")
	require.True(t, unobserved.PreviewRan, "preview verdicts still flow")
}

// TestCraftF08JourneyUnwiredPromotionRefusesErrUnsupported: without the
// receipt fence assembled, promotion refuses ErrUnsupported (fail-closed).
func TestCraftF08JourneyUnwiredPromotionRefusesErrUnsupported(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	ctx := context.Background()
	f08JourneyPutWorkspace(t, fixture)
	artifacts := f08JourneyArtifacts(t, fixture, false, nil)
	candidate := f08JourneyCollect(t, artifacts, fixture, "<h1>unwired</h1>")
	revision := f08JourneySelectHead(t, fixture, candidate)

	_, err := artifacts.PromoteWebVersion(ctx, fixture.task.Scope, craft.WebPromotionRequest{
		WorkspaceID: fixture.task.WorkspaceID, RunID: fixture.task.Fence.RunID, CandidateID: candidate.ID, Revision: revision,
	})
	require.ErrorIs(t, err, craft.ErrUnsupported)
}

// TestCraftF08JourneyBudgetDeniedRefusesBeforeEngine: a revoked grant
// refuses the dispatch at the budget seam before any engine call.
func TestCraftF08JourneyBudgetDeniedRefusesBeforeEngine(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	require.NoError(t, fixture.db.Exec("UPDATE craft_budget_grants SET allowed = 0 WHERE grant_id = ?", fixture.grantID).Error)

	_, err := fixture.dispatcher.Dispatch(context.Background(), fixture.task, fixture.grantID)
	require.ErrorIs(t, err, craft.ErrGrantRevoked)
	require.Zero(t, fixture.normal.created, "a denied budget must never reach the engine")
	var receipts int64
	require.NoError(t, fixture.db.Table("craft_web_build_receipts").Count(&receipts).Error)
	require.Zero(t, receipts)
}

// TestCraftF08JourneyStopCancelLeavesNoReceipt: a canceled dispatch context
// returns the cancellation and records nothing.
func TestCraftF08JourneyStopCancelLeavesNoReceipt(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, fixture.normal.created)
	var receipts int64
	require.NoError(t, fixture.db.Table("craft_web_build_receipts").Count(&receipts).Error)
	require.Zero(t, receipts)
}

// TestCraftF08JourneyTruncatedTransportNeverSealsComplete: a build whose
// output stream was not fully drained keeps TransportComplete=false; the
// collector's seal still binds the manifest, but OutputComplete stays
// truthfully false — completeness never heals through the seal.
func TestCraftF08JourneyTruncatedTransportNeverSealsComplete(t *testing.T) {
		partial := &f08PartialTransportProvider{}
	fixture := newF08WebBuildDispatchFixtureWithEngine(t, partial, &partial.f08WebBuildNormalProvider)
	ctx := context.Background()
	receipt, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.Error(t, err, "an incomplete-transport exec cannot report a clean result")
	require.Equal(t, "succeeded", receipt.ProcessState, "the terminal process fact is still durable")
	require.False(t, receipt.TransportComplete)

	f08JourneyPutWorkspace(t, fixture)
	artifacts := f08JourneyArtifacts(t, fixture, true, nil)
	candidate := f08JourneyCollect(t, artifacts, fixture, "<h1>partial transport</h1>")
	sealed, err := fixture.receipts.Read(ctx, receipt.Key())
	require.NoError(t, err)
	require.Equal(t, candidate.ManifestDigest, sealed.CandidateManifestSHA256)
	require.False(t, sealed.OutputComplete, "a partial transport never seals complete")
}

// f08JourneyDirectCandidate stages a candidate through the store directly —
// bypassing the collector lane and therefore leaving the receipt unsealed.
func f08JourneyDirectCandidate(t *testing.T, fixture *f08WebBuildDispatchFixture, content string) craft.Candidate {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	files := []craft.File{{
		Path: "index.html", Ref: "resource://f08-journey/direct",
		SHA256: hex.EncodeToString(sum[:]), MIME: "text/html", Bytes: int64(len(content)),
	}}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	evidence := craft.ArtifactEvidence{ClaimedSuccess: true, BuildRan: true, BuildExitCode: 0}
	candidate := craft.Candidate{
		ID: craft.CandidateID(fixture.task.WorkspaceID, fixture.task.Fence.RunID, digest),
		Scope: fixture.task.Scope, WorkspaceID: fixture.task.WorkspaceID, RunID: fixture.task.Fence.RunID,
		Generation: "generation-1", Kind: craft.KindWeb, ManifestDigest: digest,
		Files: files, Evidence: evidence, Checks: craft.BuildChecks(craft.KindWeb, files, evidence),
	}
	_, err = repository.NewCraftCandidateStore(fixture.db).PutCandidate(context.Background(), fixture.task.Scope, candidate)
	require.NoError(t, err)
	return candidate
}
