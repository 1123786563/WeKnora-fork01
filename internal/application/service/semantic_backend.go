package service

import (
	"context"
	"errors"
	"fmt"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
)

var (
	// ErrNativeCatchupRequired: rollback refused because the native index
	// has not replayed document changes made while semantic was active.
	ErrNativeCatchupRequired = errors.New("native backend catch-up required before rollback")
	// ErrPromotionRejected: the CAS promotion precondition failed.
	ErrPromotionRejected = errors.New("semantic backend promotion rejected")
	// ErrRollbackRejected: the rollback CAS precondition failed (e.g. the
	// scope was never promoted to semantic).
	ErrRollbackRejected = errors.New("semantic backend rollback rejected")
	// ErrNotActiveSemantic: rollback attempted while semantic is not the
	// active backend (nothing to roll back).
	ErrNotActiveSemantic = errors.New("semantic backend is not active")
)

// BackendService is the desired/active backend state machine (W03):
// SetDesired only records INTENT (shadow build); Promote CAS-switches the
// active backend; Rollback requires the native index to have caught up to
// the current source revisions first - a stale native index NEVER serves.
type BackendService struct {
	repo *apprepo.SemanticControlRepository
}

func NewBackendService(repo *apprepo.SemanticControlRepository) *BackendService {
	return &BackendService{repo: repo}
}

// SetDesired records the desired backend. The ACTIVE backend stays
// unchanged: a shadow build may start, but live queries keep native.
func (s *BackendService) SetDesired(ctx context.Context, scope types.SemanticScopeKey, backend string) error {
	if backend != "native" && backend != "semantic" {
		return fmt.Errorf("unknown backend %q", backend)
	}
	return s.repo.SetDesiredBackend(ctx, scope, backend)
}

// Promote CAS-switches the active backend to semantic. The caller passes
// the expected ACTIVE GENERATION - a pure FENCING TOKEN against I03
// republish (the CAS never writes active_generation; only an I03 publish
// rotates it). A mismatch rejects with ErrPromotionRejected.
func (s *BackendService) Promote(ctx context.Context, scope types.SemanticScopeKey, expectedActiveGeneration string) error {
	changed, err := s.repo.CompareAndSwapBackend(ctx, scope, "native", "semantic", expectedActiveGeneration)
	if err != nil {
		return err
	}
	if !changed {
		return fmt.Errorf("%w: active generation moved from %q", ErrPromotionRejected, expectedActiveGeneration)
	}
	return nil
}

// Rollback switches back to native AFTER verifying the native index has
// caught up: every current document revision must be <= the native
// checkpoint. A stale native index refuses (ErrNativeCatchupRequired) -
// never a silent jump to outdated answers.
func (s *BackendService) Rollback(ctx context.Context, scope types.SemanticScopeKey) error {
	state, err := s.repo.BackendState(ctx, scope)
	if err != nil {
		return err
	}
	latest, err := s.repo.LatestDocumentRevision(ctx, scope)
	if err != nil {
		return err
	}
	if state.ActiveBackend != "semantic" {
		return fmt.Errorf("%w: active is %q, nothing to roll back", ErrNotActiveSemantic, state.ActiveBackend)
	}
	if latest > state.NativeCheckpoint {
		return fmt.Errorf("%w: native at %d, source at %d", ErrNativeCatchupRequired, state.NativeCheckpoint, latest)
	}
	changed, err := s.repo.CompareAndSwapBackend(ctx, scope, "semantic", "native", state.ActiveGeneration)
	if err != nil {
		return err
	}
	if !changed {
		return fmt.Errorf("%w: rollback CAS failed", ErrRollbackRejected)
	}
	// TOCTOU guard: a mutation landing between the catch-up read and this
	// CAS means native is transiently behind (same as normal indexing
	// lag; the barrier still holds for all pre-check revisions). Surface
	// it so operators can wait for the outbox consumer instead of a
	// silent stale window.
	if recheck, err := s.repo.LatestDocumentRevision(ctx, scope); err == nil && recheck > state.NativeCheckpoint {
		return fmt.Errorf("%w: rolled back with native at %d but source moved to %d (await outbox catch-up)", ErrRollbackRejected, state.NativeCheckpoint, recheck)
	}
	return nil
}
