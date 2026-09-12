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
// the expected ACTIVE GENERATION; a mismatch (concurrent promotion or
// moved generation) rejects with ErrPromotionRejected.
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
	if latest > state.NativeCheckpoint {
		return fmt.Errorf("%w: native at %d, source at %d", ErrNativeCatchupRequired, state.NativeCheckpoint, latest)
	}
	changed, err := s.repo.CompareAndSwapBackend(ctx, scope, "semantic", "native", state.ActiveGeneration)
	if err != nil {
		return err
	}
	if !changed {
		return fmt.Errorf("%w: rollback CAS failed", ErrPromotionRejected)
	}
	return nil
}
