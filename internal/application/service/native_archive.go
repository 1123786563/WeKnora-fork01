package service

import (
	"context"
	"strings"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
)

const (
	nativeArchiveDefaultLimit = 100
	nativeArchiveMaxLimit     = 100
)

// NativeArchiveStore is the durable archive boundary. Its implementation must
// apply the authoritative scope to every lookup; the facade verifies it again
// before exposing a result to an HTTP caller.
type NativeArchiveStore interface {
	nativecontract.ArchiveReader
}

// NativeArchiveService makes the archive namespace read-only and rechecks the
// caller's current tenancy before every operation. It deliberately has no
// resume, continue, or mutation methods.
type NativeArchiveService struct {
	store  NativeArchiveStore
	scopes nativecontract.ScopeResolver
}

func NewNativeArchiveService(store NativeArchiveStore, scopes nativecontract.ScopeResolver) *NativeArchiveService {
	return &NativeArchiveService{store: store, scopes: scopes}
}

func (s *NativeArchiveService) List(ctx context.Context, scope nativecontract.Scope, query nativecontract.ArchiveQuery) (nativecontract.ArchivePage, error) {
	authoritative, err := s.recheck(ctx, scope)
	if err != nil {
		return nativecontract.ArchivePage{}, err
	}
	query, err = normalizeNativeArchiveQuery(query)
	if err != nil {
		return nativecontract.ArchivePage{}, err
	}
	page, err := s.store.List(ctx, authoritative, query)
	if err != nil {
		return nativecontract.ArchivePage{}, err
	}
	for _, record := range page.Records {
		if !nativeArchiveRecordMatchesQuery(record, query) {
			return nativecontract.ArchivePage{}, nativeArchiveFailure(nativecontract.ErrNotFound, "archive record was not found")
		}
	}
	return page, nil
}

func (s *NativeArchiveService) Read(ctx context.Context, scope nativecontract.Scope, id string) (nativecontract.ArchiveRecord, error) {
	authoritative, err := s.recheck(ctx, scope)
	if err != nil {
		return nativecontract.ArchiveRecord{}, err
	}
	if strings.TrimSpace(id) == "" {
		return nativecontract.ArchiveRecord{}, nativeArchiveFailure(nativecontract.ErrInvalid, "archive record id is required")
	}
	record, err := s.store.Read(ctx, authoritative, id)
	if err != nil {
		return nativecontract.ArchiveRecord{}, err
	}
	if strings.TrimSpace(record.ID) != id || strings.TrimSpace(record.SessionID) == "" || !nativeArchiveKind(record.Kind) {
		return nativecontract.ArchiveRecord{}, nativeArchiveFailure(nativecontract.ErrNotFound, "archive record was not found")
	}
	return record, nil
}

func (s *NativeArchiveService) ReadArtifact(ctx context.Context, scope nativecontract.Scope, id string) (nativecontract.ArtifactRef, error) {
	authoritative, err := s.recheck(ctx, scope)
	if err != nil {
		return nativecontract.ArtifactRef{}, err
	}
	if strings.TrimSpace(id) == "" {
		return nativecontract.ArtifactRef{}, nativeArchiveFailure(nativecontract.ErrInvalid, "archive artifact id is required")
	}
	artifact, err := s.store.ReadArtifact(ctx, authoritative, id)
	if err != nil {
		return nativecontract.ArtifactRef{}, err
	}
	if artifact.ID != id || strings.TrimSpace(artifact.MediaType) == "" || strings.TrimSpace(artifact.SHA256) == "" || artifact.SizeBytes < 0 {
		return nativecontract.ArtifactRef{}, nativeArchiveFailure(nativecontract.ErrNotFound, "archive artifact was not found")
	}
	return artifact, nil
}

func (s *NativeArchiveService) recheck(ctx context.Context, scope nativecontract.Scope) (nativecontract.Scope, error) {
	if s == nil || s.store == nil || s.scopes == nil || scope.TenantID == 0 || strings.TrimSpace(scope.SessionOwnerID) == "" {
		return nativecontract.Scope{}, nativeArchiveFailure(nativecontract.ErrForbidden, "archive authority is unavailable")
	}
	authoritative, err := s.scopes.Recheck(ctx, scope, nil)
	if err != nil || authoritative.TenantID != scope.TenantID || authoritative.SessionOwnerID != scope.SessionOwnerID {
		return nativecontract.Scope{}, nativeArchiveFailure(nativecontract.ErrForbidden, "archive authority was revoked")
	}
	return authoritative, nil
}

func normalizeNativeArchiveQuery(query nativecontract.ArchiveQuery) (nativecontract.ArchiveQuery, error) {
	query.SessionID = strings.TrimSpace(query.SessionID)
	query.Kind = strings.TrimSpace(query.Kind)
	query.Cursor = strings.TrimSpace(query.Cursor)
	if query.Kind != "" && !nativeArchiveKind(query.Kind) {
		return nativecontract.ArchiveQuery{}, nativeArchiveFailure(nativecontract.ErrInvalid, "archive record kind is invalid")
	}
	if query.Limit < 0 {
		return nativecontract.ArchiveQuery{}, nativeArchiveFailure(nativecontract.ErrInvalid, "archive limit is invalid")
	}
	if query.Limit == 0 {
		query.Limit = nativeArchiveDefaultLimit
	}
	if query.Limit > nativeArchiveMaxLimit {
		query.Limit = nativeArchiveMaxLimit
	}
	return query, nil
}

func nativeArchiveRecordMatchesQuery(record nativecontract.ArchiveRecord, query nativecontract.ArchiveQuery) bool {
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.SessionID) == "" || !nativeArchiveKind(record.Kind) {
		return false
	}
	return (query.SessionID == "" || record.SessionID == query.SessionID) && (query.Kind == "" || record.Kind == query.Kind)
}

func nativeArchiveKind(kind string) bool {
	switch kind {
	case "session", "message", "tool_call", "approval", "audit", "memory", "artifact":
		return true
	default:
		return false
	}
}

func nativeArchiveFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}

// ArchiveMutationError is the frozen protocol response for every historic
// mutation attempt. Archive readers intentionally never expose a write path.
func ArchiveMutationError() nativecontract.Failure {
	return nativecontract.Failure{Code: nativecontract.ErrArchiveReadOnly, Message: "历史记录仅供查询", Effect: nativecontract.EffectNotDispatched}
}

var _ nativecontract.ArchiveReader = (*NativeArchiveService)(nil)
