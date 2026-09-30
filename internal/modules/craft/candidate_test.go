package craft

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCandidateValidateBindsRunGenerationAndManifest(t *testing.T) {
	scope := Scope{TenantID: 1, UserID: "owner", SessionID: "task"}
	bytes := []byte("<h1>candidate</h1>")
	sum := sha256.Sum256(bytes)
	files := []File{{Path: "index.html", Ref: "resource://candidate/index", SHA256: hex.EncodeToString(sum[:]), MIME: "text/html", Bytes: int64(len(bytes))}}
	digest, err := ManifestDigest(files)
	require.NoError(t, err)
	candidate := Candidate{
		Scope: scope, WorkspaceID: "workspace", RunID: "run-2", Generation: "rv_generation_2",
		Kind: KindWeb, ManifestDigest: digest, Files: files,
		Evidence: ArtifactEvidence{}, Checks: BuildChecks(KindWeb, files, ArtifactEvidence{}),
	}
	candidate.ID = CandidateID(candidate.WorkspaceID, candidate.RunID, candidate.ManifestDigest)
	require.NoError(t, candidate.Validate(scope))

	missingGeneration := candidate
	missingGeneration.Generation = ""
	require.ErrorIs(t, missingGeneration.Validate(scope), ErrInvalidInput)
	wrongScope := candidate
	wrongScope.Scope.UserID = "other"
	require.ErrorIs(t, wrongScope.Validate(scope), ErrForbidden)
	wrongDigest := candidate
	wrongDigest.ManifestDigest = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	require.ErrorIs(t, wrongDigest.Validate(scope), ErrConflict)
}
