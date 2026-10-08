package craft

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const CandidateIDPrefix = "cand_"

// CandidateID is a private, deterministic collection identity. It deliberately
// lives outside VersionID/VersionStore so staging cannot publish a version.
func CandidateID(workspaceID, runID, manifestDigest string) string {
	raw, _ := json.Marshal([]string{workspaceID, runID, manifestDigest})
	sum := sha256.Sum256(raw)
	return CandidateIDPrefix + hex.EncodeToString(sum[:])
}

// Candidate is the immutable output snapshot staged by one verified Run.
// Unlike Version it is never visible through VersionStore or default preview.
type Candidate struct {
	ID, WorkspaceID, RunID, Generation, Kind, ManifestDigest string
	Scope                                                    Scope
	Files                                                    []File
	Checks                                                   []Check
	Evidence                                                 ArtifactEvidence
}

// Validate checks the persisted identity, manifest and evidence facts. The
// caller supplies the trusted scope; request input cannot widen a candidate.
func (c Candidate) Validate(scope Scope) error {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return fmt.Errorf("%w: incomplete candidate scope", ErrInvalidInput)
	}
	if c.Scope.TenantID != scope.TenantID || c.Scope.SessionID != scope.SessionID {
		return ErrNotFound
	}
	if c.Scope.UserID != scope.UserID {
		return ErrForbidden
	}
	if c.WorkspaceID == "" || c.RunID == "" || c.Kind == "" || !KnownKind(c.Kind) ||
		c.Generation == "" || len(c.Generation) > 128 || strings.ContainsAny(c.Generation, "\x00\r\n\t ") {
		return fmt.Errorf("%w: incomplete candidate identity", ErrInvalidInput)
	}
	for _, file := range c.Files {
		if strings.TrimSpace(file.Ref) == "" {
			return fmt.Errorf("%w: candidate file %q has no object reference", ErrInvalidInput, file.Path)
		}
	}
	digest, err := ManifestDigest(c.Files)
	if err != nil {
		return err
	}
	if digest != c.ManifestDigest || c.ID != CandidateID(c.WorkspaceID, c.RunID, digest) {
		return fmt.Errorf("%w: candidate identity differs from its manifest", ErrConflict)
	}
	expected := BuildChecks(c.Kind, c.Files, c.Evidence)
	if len(c.Checks) != len(expected) {
		return fmt.Errorf("%w: candidate checks differ from evidence", ErrConflict)
	}
	for i := range expected {
		if c.Checks[i] != expected[i] {
			return fmt.Errorf("%w: candidate checks differ from evidence", ErrConflict)
		}
	}
	return nil
}

// CandidateStore persists private candidates. Implementations authorize every
// read against the owning Task and never project rows into VersionStore.
type CandidateStore interface {
	PutCandidate(ctx context.Context, scope Scope, candidate Candidate) (Candidate, error)
	GetCandidate(ctx context.Context, scope Scope, id string) (Candidate, error)
}
