package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
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
