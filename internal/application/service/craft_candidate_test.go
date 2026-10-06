package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/stretchr/testify/require"
)

type identifiedCandidateSource struct {
	*fakeSandboxSource
	runID      string
	generation string
}

func (s *identifiedCandidateSource) CraftArtifactRunID() string      { return s.runID }
func (s *identifiedCandidateSource) CraftArtifactGeneration() string { return s.generation }

type memoryCandidateStore struct {
	byRun map[string]craft.Candidate
	err   error
}

func (s *memoryCandidateStore) PutCandidate(_ context.Context, scope craft.Scope, in craft.Candidate) (craft.Candidate, error) {
	if s.err != nil {
		return craft.Candidate{}, s.err
	}
	if s.byRun == nil {
		s.byRun = map[string]craft.Candidate{}
	}
	if prior, ok := s.byRun[in.RunID]; ok {
		if prior.ID != in.ID {
			return craft.Candidate{}, craft.ErrConflict
		}
		return prior, nil
	}
	in.Scope = scope
	s.byRun[in.RunID] = in
	return in, nil
}

func (s *memoryCandidateStore) GetCandidate(_ context.Context, scope craft.Scope, id string) (craft.Candidate, error) {
	for _, candidate := range s.byRun {
		if candidate.ID == id && candidate.Scope == scope {
			return candidate, nil
		}
	}
	return craft.Candidate{}, craft.ErrNotFound
}

func candidateSource(runID, generation, content string) *identifiedCandidateSource {
	entry := sandbox.RemoteDirEntry{Name: "index.html", Path: craftTestOutputDir + "/index.html", Type: sandbox.RemoteEntryFile, Size: int64(len(content))}
	return &identifiedCandidateSource{
		fakeSandboxSource: craftSourceWith([]sandbox.RemoteDirEntry{entry}, map[string][]byte{entry.Path: []byte(content)}),
		runID:             runID, generation: generation,
	}
}

func TestCraftArtifactCollectCandidateUsesPerRunSourceAndStaysPrivate(t *testing.T) {
	files := newDirBackedFileService(t)
	versions := newMemVersionStore()
	candidates := &memoryCandidateStore{}
	svc := NewCraftArtifactServiceWithCandidates(
		candidateSource("run-a", "gen-a", "<h1>A</h1>"), files, versions, candidates, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	)
	legacyTask := craftArtifactTask("s1", "ws-1", "run-a")
	v1, err := svc.CollectForKind(context.Background(), legacyTask, craft.KindWeb)
	require.NoError(t, err)

	taskB := craftArtifactTask("s1", "ws-1", "run-b")
	sourceB := candidateSource("run-b", "gen-b", "<h1>B</h1>")
	first, err := svc.CollectCandidate(context.Background(), taskB, craft.KindWeb, sourceB, "gen-b")
	require.NoError(t, err)
	second, err := svc.CollectCandidate(context.Background(), taskB, craft.KindWeb, sourceB, "gen-b")
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "an exact candidate retry adopts its immutable row")
	require.Equal(t, "gen-b", first.Generation)
	require.Equal(t, []byte("<h1>B</h1>"), files.readRef(t, first.Files[0].Ref))
	require.Len(t, candidates.byRun, 1)

	list, err := versions.List(context.Background(), taskB.Scope)
	require.NoError(t, err)
	require.Equal(t, []craft.Version{v1}, list, "private candidates must not appear in version history")
	got, err := versions.Get(context.Background(), taskB.Scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, v1.Files, got.Files, "the prior default version remains intact")
}

func TestCraftArtifactCollectCandidateRejectsWrongRunSourceBeforeUpload(t *testing.T) {
	files := newDirBackedFileService(t)
	candidates := &memoryCandidateStore{}
	svc := NewCraftArtifactServiceWithCandidates(
		candidateSource("run-a", "gen-a", "<h1>A</h1>"), files, newMemVersionStore(), candidates, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	)
	_, err := svc.CollectCandidate(context.Background(), craftArtifactTask("s1", "ws-1", "run-b"), craft.KindWeb,
		candidateSource("run-a", "gen-a", "<h1>A</h1>"), "gen-b")
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Zero(t, files.saves)
	require.Empty(t, candidates.byRun)
}

func TestCraftArtifactCollectCandidateUploadFailureLeavesNoCandidate(t *testing.T) {
	files := newDirBackedFileService(t)
	files.failFrom = 1
	candidates := &memoryCandidateStore{}
	svc := NewCraftArtifactServiceWithCandidates(
		candidateSource("run-b", "gen-b", "<h1>B</h1>"), files, newMemVersionStore(), candidates, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	)
	_, err := svc.CollectCandidate(context.Background(), craftArtifactTask("s1", "ws-1", "run-b"), craft.KindWeb,
		candidateSource("run-b", "gen-b", "<h1>B</h1>"), "gen-b")
	require.Error(t, err)
	require.Empty(t, candidates.byRun)
}

func TestCraftArtifactCollectCandidateValidationFailureLeavesNoCandidate(t *testing.T) {
	files := newDirBackedFileService(t)
	candidates := &memoryCandidateStore{}
	malicious := "../escape.html"
	entry := sandbox.RemoteDirEntry{Name: malicious, Path: craftTestOutputDir + "/" + malicious, Type: sandbox.RemoteEntryFile, Size: 1}
	source := &identifiedCandidateSource{
		fakeSandboxSource: craftSourceWith([]sandbox.RemoteDirEntry{entry}, map[string][]byte{entry.Path: []byte("x")}),
		runID:             "run-b", generation: "gen-b",
	}
	svc := NewCraftArtifactServiceWithCandidates(
		source, files, newMemVersionStore(), candidates, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	)
	_, err := svc.CollectCandidate(context.Background(), craftArtifactTask("s1", "ws-1", "run-b"), craft.KindWeb, source, "gen-b")
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.Zero(t, files.saves)
	require.Empty(t, candidates.byRun)
}

// TestCraftArtifactWebHTMLScreenRejectsScriptShapes is the round-3 build-log
// trust regression: whatever a forged build-log.json claims, a staged web
// HTML member carrying script/event/egress shapes refuses the WHOLE round
// server-side before any byte is uploaded.
func TestCraftArtifactWebHTMLScreenRejectsScriptShapes(t *testing.T) {
	malformed := []string{
		`<h1>ok</h1><img src="a>b" onerror="alert(1)">`,                           // quoted ">" bypass of naive regexes
		"<h1>ok</h1><img/onerror=alert(1)>",                                       // "/" tag separator
		`<div style="&#92;75 rl&#40;&#92;2f&#92;2fevil&#46;example&#41;">x</div>`, // entity×CSS smuggle
		`<a href="https://evil.example">x</a>`,                                    // external URL
		`<iframe src="x"></iframe>`,                                               // embedding tag
	}
	for index, content := range malformed {
		files := newDirBackedFileService(t)
		candidates := &memoryCandidateStore{}
		svc := NewCraftArtifactServiceWithCandidates(
			candidateSource("run-s", "gen-s", "<h1>clean</h1>"), files, newMemVersionStore(), candidates, nil,
			CraftArtifactConfig{OutputDir: craftTestOutputDir},
		)
		_, err := svc.CollectCandidate(context.Background(), craftArtifactTask("s1", "ws-1", "run-s"), craft.KindWeb,
			candidateSource("run-s", "gen-s", content), "gen-s")
		require.ErrorIs(t, err, craft.ErrInvalidInput, "case %d must be refused: %s", index, content)
		require.Zero(t, files.saves, "case %d must not upload any object", index)
		require.Empty(t, candidates.byRun, "case %d must not stage a candidate", index)
	}

	// Clean HTML still stages (control).
	files := newDirBackedFileService(t)
	candidates := &memoryCandidateStore{}
	svc := NewCraftArtifactServiceWithCandidates(
		candidateSource("run-ok", "gen-ok", "<h1>clean</h1>"), files, newMemVersionStore(), candidates, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	)
	_, err := svc.CollectCandidate(context.Background(), craftArtifactTask("s1", "ws-1", "run-ok"), craft.KindWeb,
		candidateSource("run-ok", "gen-ok", "<!doctype html><html><body><h1>fine</h1><p>only = 3</p></body></html>"), "gen-ok")
	require.NoError(t, err)
	require.Len(t, candidates.byRun, 1)
}

// sealingWebBuildReceipts records the collector's seal invocations (the real
// latch lives in the repository; the container journeys prove it).
type sealingWebBuildReceipts struct {
	mu      sync.Mutex
	seals   [][4]string // workspaceID, runID, digest — scope asserted separately
	err     error
	lastCtx context.Context
}

func (s *sealingWebBuildReceipts) VerifyPromotionBuild(context.Context, craft.Scope, string, string, string) error {
	return nil
}

func (s *sealingWebBuildReceipts) SealCandidateManifest(_ context.Context, _ craft.Scope, workspaceID, runID, digest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seals = append(s.seals, [4]string{workspaceID, runID, digest})
	return s.err
}

// TestCraftArtifactCollectCandidateSealsManifestIntoBuildReceipt is the F08
// collector-seal fence: a successful candidate collection seals the server-
// computed manifest digest into the Run's build receipt; a refusing seal
// never fails the collection itself (the promotion fence holds the line).
func TestCraftArtifactCollectCandidateSealsManifestIntoBuildReceipt(t *testing.T) {
	ctx := context.Background()
	fence := &sealingWebBuildReceipts{}
	svc := NewCraftArtifactServiceWithCandidates(
		candidateSource("run-seal", "gen-seal", "<h1>seal</h1>"), newDirBackedFileService(t), newMemVersionStore(), &memoryCandidateStore{}, nil,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	).WithWebBuildReceipt(fence)

	task := craftArtifactTask("s1", "ws-seal", "run-seal")
	candidate, err := svc.CollectCandidate(ctx, task, craft.KindWeb, candidateSource("run-seal", "gen-seal", "<h1>seal</h1>"), "gen-seal")
	require.NoError(t, err)
	require.Len(t, fence.seals, 1, "collection must seal the candidate manifest digest")
	require.Equal(t, [4]string{"ws-seal", "run-seal", candidate.ManifestDigest}, fence.seals[0])

	fence.err = errors.New("sealed digest conflicts: output mutated")
	refusedTask := craftArtifactTask("s1", "ws-seal", "run-seal-next")
	refused, err := svc.CollectCandidate(ctx, refusedTask, craft.KindWeb, candidateSource("run-seal-next", "gen-next", "<h1>next</h1>"), "gen-next")
	require.NoError(t, err, "a refused seal never fails the collection; promotion refuses on digest mismatch")
	require.NotEqual(t, candidate.ManifestDigest, refused.ManifestDigest)
	require.Len(t, fence.seals, 2, "every successful collection attempts its seal")
}
