package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
)

var (
	// ErrSemanticScopeInvalid: the presented scope is not trustworthy
	// (bad signature, wrong audience, expired, tampered fields).
	ErrSemanticScopeInvalid = errors.New("semantic scope invalid")
	// ErrSemanticScopeChanged: authorization moved on (epoch bump or
	// deletion barrier) - the WHOLE result must be discarded, never
	// partially trimmed (spec section 5).
	ErrSemanticScopeChanged = errors.New("semantic scope changed")
)

// SemanticEpochBumper raises KB authorization epochs after visibility-
// changing writes (transfer flows). Implemented by the semantic control
// repository; nil-safe for callers that run without the control plane.
type SemanticEpochBumper interface {
	BumpKBSemanticEpochs(ctx context.Context, tenantID uint64, kbIDs ...string) error
}

// SemanticScopeConfig controls scope issuance.
type SemanticScopeConfig struct {
	Audience   string
	TTL        time.Duration
	IssuingKey []byte
	Now        func() time.Time
}

// ScopeSnapshot is the immutable authorization view resolved for one
// issued scope: which documents are visible, at which source revision,
// which revisions are denied, and whether stale-but-published versions
// may still be served.
type ScopeSnapshot struct {
	AllowedDocumentIDs    []string          `json:"allowed_document_ids"`
	MaxSourceRevision     map[string]uint64 `json:"max_source_revision"`
	AllowRetainedPrevious bool              `json:"allow_retained_previous"`
	DenyRevisions         map[string]uint64 `json:"deny_revisions"`
	ExpiresAt             time.Time         `json:"expires_at"`
}

// SemanticScopeService issues short-lived access scopes for the semantic
// service and re-validates them before delivery. The authorization truth
// (epochs, document revisions, denial barriers) lives in the business
// database; permission-service failures are errors, never silent passes.
type SemanticScopeService struct {
	repo   *apprepo.SemanticControlRepository
	config SemanticScopeConfig
}

func NewSemanticScopeService(repo *apprepo.SemanticControlRepository, config SemanticScopeConfig) *SemanticScopeService {
	if config.Audience == "" {
		config.Audience = "weknora-semantic"
	}
	if config.TTL <= 0 {
		config.TTL = 5 * time.Minute
	}
	// Upper bound: short-lived scopes are the spec's replay mitigation.
	if config.TTL > 15*time.Minute {
		config.TTL = 15 * time.Minute
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	// A short HMAC key is cryptographically indistinguishable from the
	// publicly-known empty key - DISABLE the service rather than silently
	// signing with a forgeable key (fail closed, never nil-and-continue).
	if 0 < len(config.IssuingKey) && len(config.IssuingKey) < minIssuingKeyBytes {
		return nil
	}
	return &SemanticScopeService{repo: repo, config: config}
}

// minIssuingKeyBytes is the minimum HMAC-SHA256 key length this service
// accepts; empty is allowed only to mean "no service" upstream.
const minIssuingKeyBytes = 32

// scopeToken is the HMAC-signed reference carried in ScopeRef.
type scopeToken struct {
	TenantID    uint64    `json:"tenant_id"`
	KBID        string    `json:"kb_id"`
	SubjectID   string    `json:"subject_id"`
	Purpose     string    `json:"purpose"`
	Epoch       uint64    `json:"epoch"`
	ExpiresAt   time.Time `json:"expires_at"`
	SnapshotSum string    `json:"snapshot_sum"`
}

func (s *SemanticScopeService) Issue(
	ctx context.Context, subjectID string, scope types.SemanticScopeKey, purpose string,
) (types.SemanticAccessScope, error) {
	if subjectID == "" {
		return types.SemanticAccessScope{}, fmt.Errorf("%w: empty subject", ErrSemanticScopeInvalid)
	}
	epoch, snapshot, err := s.snapshotState(ctx, scope)
	if err != nil {
		return types.SemanticAccessScope{}, err
	}
	// Whole-second expiry so the RFC3339 wire form round-trips exactly.
	expiresAt := s.config.Now().Add(s.config.TTL).Truncate(time.Second)
	token := scopeToken{
		TenantID: scope.TenantID, KBID: scope.KBID, SubjectID: subjectID,
		Purpose: purpose, Epoch: epoch, ExpiresAt: expiresAt,
		SnapshotSum: snapshotDigest(snapshot),
	}
	ref, err := s.sign(token)
	if err != nil {
		return types.SemanticAccessScope{}, err
	}
	return types.SemanticAccessScope{
		Scope:           scope,
		SubjectID:       subjectID,
		ScopeRef:        ref,
		ScopeHash:       token.SnapshotSum,
		PermissionEpoch: epoch,
		ExpiresAt:       expiresAt.Format(time.RFC3339),
		Audience:        s.config.Audience,
		Purpose:         types.SemanticAccessPurpose(purpose),
	}, nil
}

// Resolve verifies a scope reference and rebuilds the authorization
// snapshot from the CURRENT business state. A moved epoch fails with
// ErrSemanticScopeChanged: nothing may be served from stale authorization.
func (s *SemanticScopeService) Resolve(ctx context.Context, scopeRef string) (ScopeSnapshot, error) {
	token, err := s.verify(scopeRef)
	if err != nil {
		return ScopeSnapshot{}, err
	}
	if token.ExpiresAt.Before(s.config.Now()) {
		return ScopeSnapshot{}, fmt.Errorf("%w: scope expired", ErrSemanticScopeInvalid)
	}
	epoch, snapshot, err := s.snapshotState(ctx, types.SemanticScopeKey{TenantID: token.TenantID, KBID: token.KBID})
	if err != nil {
		return ScopeSnapshot{}, err
	}
	if epoch != token.Epoch {
		return ScopeSnapshot{}, fmt.Errorf("%w: epoch moved from %d to %d", ErrSemanticScopeChanged, token.Epoch, epoch)
	}
	// Live snapshot-hash recheck (plan constraint): a non-deleted revision
	// bump silently widens MaxSourceRevision without moving the epoch -
	// that drift must also invalidate the issued scope.
	if current := snapshotDigest(snapshot); current != token.SnapshotSum {
		return ScopeSnapshot{}, fmt.Errorf("%w: snapshot content changed since issuance", ErrSemanticScopeChanged)
	}
	snapshot.ExpiresAt = token.ExpiresAt
	return snapshot, nil
}

// ValidateDelivery is the FINAL authorization check before any semantic
// result is delivered: signature, audience, expiry, scope-hash and the
// current epoch are re-verified. Any change discards the WHOLE result.
func (s *SemanticScopeService) ValidateDelivery(ctx context.Context, scope types.SemanticAccessScope) error {
	token, err := s.verify(scope.ScopeRef)
	if err != nil {
		return err
	}
	if scope.Audience != s.config.Audience {
		return fmt.Errorf("%w: wrong audience %q", ErrSemanticScopeInvalid, scope.Audience)
	}
	expiresAt, err := time.Parse(time.RFC3339, scope.ExpiresAt)
	if err != nil {
		return fmt.Errorf("%w: unparsable expiry", ErrSemanticScopeInvalid)
	}
	if expiresAt.Before(s.config.Now()) {
		return fmt.Errorf("%w: scope expired", ErrSemanticScopeInvalid)
	}
	if scope.ScopeHash != token.SnapshotSum || scope.SubjectID != token.SubjectID ||
		scope.Scope.TenantID != token.TenantID || scope.Scope.KBID != token.KBID ||
		scope.PermissionEpoch != token.Epoch || string(scope.Purpose) != token.Purpose ||
		!token.ExpiresAt.Equal(expiresAt) {
		return fmt.Errorf("%w: scope fields do not match the signed reference", ErrSemanticScopeInvalid)
	}
	currentEpoch, err := s.repo.CurrentEpoch(ctx, token.TenantID, token.KBID)
	if err != nil {
		// Fail closed: an unreadable permission epoch is never a pass.
		return fmt.Errorf("semantic epoch read failed: %w", err)
	}
	if currentEpoch != token.Epoch {
		return fmt.Errorf("%w: epoch moved from %d to %d", ErrSemanticScopeChanged, token.Epoch, currentEpoch)
	}
	return nil
}

func (s *SemanticScopeService) snapshotState(
	ctx context.Context, scope types.SemanticScopeKey,
) (uint64, ScopeSnapshot, error) {
	epoch, err := s.repo.CurrentEpoch(ctx, scope.TenantID, scope.KBID)
	if err != nil {
		return 0, ScopeSnapshot{}, fmt.Errorf("semantic epoch read failed: %w", err)
	}
	states, err := s.repo.ListDocumentStates(ctx, scope.TenantID, scope.KBID)
	if err != nil {
		return 0, ScopeSnapshot{}, fmt.Errorf("semantic document states read failed: %w", err)
	}
	denials, err := s.repo.ListDenials(ctx, scope.TenantID, scope.KBID)
	if err != nil {
		return 0, ScopeSnapshot{}, fmt.Errorf("semantic denials read failed: %w", err)
	}
	snapshot := ScopeSnapshot{
		MaxSourceRevision:     make(map[string]uint64, len(states)),
		DenyRevisions:         denials,
		AllowRetainedPrevious: true,
	}
	for _, state := range states {
		if denied, ok := denials[state.DocumentID]; ok && denied >= state.Revision {
			continue // deletion barrier: never visible through this scope
		}
		if state.Deleted && state.Revision > 0 {
			// deleted flag on the row itself also blocks visibility
			continue
		}
		snapshot.AllowedDocumentIDs = append(snapshot.AllowedDocumentIDs, state.DocumentID)
		snapshot.MaxSourceRevision[state.DocumentID] = state.Revision
	}
	sort.Strings(snapshot.AllowedDocumentIDs)
	return epoch, snapshot, nil
}

func snapshotDigest(snapshot ScopeSnapshot) string {
	// Canonical digest over sorted visible ids + revisions + denials. Each
	// entry is length-prefixed so doc IDs containing "@" / "|" / "deny:"
	// can never collide with another entry's encoding.
	docs := append([]string(nil), snapshot.AllowedDocumentIDs...)
	sort.Strings(docs)
	denyDocs := make([]string, 0, len(snapshot.DenyRevisions))
	for doc := range snapshot.DenyRevisions {
		denyDocs = append(denyDocs, doc)
	}
	sort.Strings(denyDocs)
	buffer := &strings.Builder{}
	writeField := func(kind, doc string, revision uint64) {
		entry := fmt.Sprintf("%s%s@%d", kind, doc, revision)
		fmt.Fprintf(buffer, "%d:%s|", len(entry), entry)
	}
	for _, doc := range docs {
		writeField("allow:", doc, snapshot.MaxSourceRevision[doc])
	}
	for _, doc := range denyDocs {
		writeField("deny:", doc, snapshot.DenyRevisions[doc])
	}
	fmt.Fprintf(buffer, "retained:%v", snapshot.AllowRetainedPrevious)
	sum := sha256.Sum256([]byte(buffer.String()))
	return hex.EncodeToString(sum[:])
}

func (s *SemanticScopeService) sign(token scopeToken) (string, error) {
	if len(s.config.IssuingKey) < minIssuingKeyBytes {
		return "", fmt.Errorf("%w: issuing key too short to sign", ErrSemanticScopeInvalid)
	}
	payload, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.config.IssuingKey)
	mac.Write([]byte(encoded))
	return encoded + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func (s *SemanticScopeService) verify(ref string) (scopeToken, error) {
	var token scopeToken
	if len(s.config.IssuingKey) < minIssuingKeyBytes {
		return token, fmt.Errorf("%w: issuing key too short to verify", ErrSemanticScopeInvalid)
	}
	if ref == "" {
		return token, fmt.Errorf("%w: empty scope reference", ErrSemanticScopeInvalid)
	}
	encoded, signature, ok := strings.Cut(ref, ".")
	if !ok || encoded == "" || signature == "" {
		return token, fmt.Errorf("%w: malformed scope reference", ErrSemanticScopeInvalid)
	}
	mac := hmac.New(sha256.New, s.config.IssuingKey)
	mac.Write([]byte(encoded))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return token, fmt.Errorf("%w: bad scope signature", ErrSemanticScopeInvalid)
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return token, fmt.Errorf("%w: bad scope payload", ErrSemanticScopeInvalid)
	}
	if err := json.Unmarshal(payload, &token); err != nil {
		return token, fmt.Errorf("%w: bad scope json", ErrSemanticScopeInvalid)
	}
	// The HMAC key is issuer-bound: only this service's tokens verify.
	return token, nil
}
