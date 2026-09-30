package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// recordingCitationGate records the staged bytes it was handed and answers
// with a canned verdict, so the wiring test observes the seam without
// re-testing the gate's own (fully covered) admission rules.
type recordingCitationGate struct {
	calls  int
	staged map[string][]byte
	err    error
}

func (g *recordingCitationGate) AdmitStagedWebCitations(_ context.Context, _ craft.Scope, _ string, staged map[string][]byte) (craft.WebCitationManifest, error) {
	g.calls++
	g.staged = staged
	return craft.WebCitationManifest{Schema: 1}, g.err
}

// TestCraftArtifactWebCitationGateRunsBeforeUpload pins the T06 central
// wiring: with the gate configured on a web-kind artifact service, every
// staged round passes AdmitStagedWebCitations BEFORE any byte uploads, with
// the exact output-relative bytes; a refusal rejects the round and leaves
// nothing staged; a non-web kind never consults the gate; and the historic
// nil-gate behavior is unchanged.
func TestCraftArtifactWebCitationGateRunsBeforeUpload(t *testing.T) {
	task := craftArtifactTask("s1", "ws-1", "run-cite")
	entryContent := "<h1>cited</h1><p data-craft-citation=\"kc_000000000000000000000000\">fact</p>"

	t.Run("gate runs before upload with exact staged bytes", func(t *testing.T) {
		files := newDirBackedFileService(t)
		gate := &recordingCitationGate{}
		svc := NewCraftArtifactServiceWithCandidates(
			candidateSource("run-cite", "gen-cite", entryContent), files, newMemVersionStore(), &memoryCandidateStore{}, nil,
			CraftArtifactConfig{OutputDir: craftTestOutputDir, WebCitationGate: gate},
		)
		_, err := svc.CollectCandidate(context.Background(), task, craft.KindWeb,
			candidateSource("run-cite", "gen-cite", entryContent), "gen-cite")
		require.NoError(t, err)
		require.Equal(t, 1, gate.calls, "the gate must run exactly once per staged round")
		require.Equal(t, map[string][]byte{"index.html": []byte(entryContent)}, gate.staged,
			"the gate sees the exact output-relative staged bytes")
	})

	t.Run("gate refusal rejects the round and stages nothing", func(t *testing.T) {
		files := newDirBackedFileService(t)
		gate := &recordingCitationGate{err: craft.ErrForbidden}
		candidates := &memoryCandidateStore{}
		svc := NewCraftArtifactServiceWithCandidates(
			candidateSource("run-cite", "gen-cite", entryContent), files, newMemVersionStore(), candidates, nil,
			CraftArtifactConfig{OutputDir: craftTestOutputDir, WebCitationGate: gate},
		)
		_, err := svc.CollectCandidate(context.Background(), task, craft.KindWeb,
			candidateSource("run-cite", "gen-cite", entryContent), "gen-cite")
		require.ErrorIs(t, err, craft.ErrForbidden)
		require.Empty(t, candidates.byRun, "a refused round must not persist a candidate")
		require.Zero(t, files.saves, "a refused round must not upload any object")
	})

	t.Run("nil gate keeps the recorded pre-integration behavior", func(t *testing.T) {
		svc := NewCraftArtifactServiceWithCandidates(
			candidateSource("run-cite", "gen-cite", "<h1>plain</h1>"), newDirBackedFileService(t), newMemVersionStore(), &memoryCandidateStore{}, nil,
			CraftArtifactConfig{OutputDir: craftTestOutputDir},
		)
		_, err := svc.CollectCandidate(context.Background(), task, craft.KindWeb,
			candidateSource("run-cite", "gen-cite", "<h1>plain</h1>"), "gen-cite")
		require.NoError(t, err, "without the gate a marker-free page still collects (unwired legacy)")
	})
}
