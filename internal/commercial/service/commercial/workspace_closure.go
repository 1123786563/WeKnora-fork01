package commercial

import (
	"context"
	"errors"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"

	"gorm.io/gorm"
)

// WorkspaceClosureService owns the #102 (Lago 30) closure flow. The order is
// deliberate: the tombstone lands FIRST (from that instant new commands,
// purchases and charge execution are refused whatever happens next), then
// the authority-side disposal — terminate subscriptions and wallets,
// de-identify the customer display data — runs through the frozen seam,
// then the local projection disposal caps the paid term at the closure
// instant so no later month is ever due. Financial records — orders,
// payments, refunds, credit notes, the audit trail — are never deleted or
// rewritten; the workspace stays queryable for compliance (US54).
type WorkspaceClosureService struct {
	closures *repocommercial.WorkspaceClosureStore
	subs     *repocommercial.SubscriptionStore
	platform domain.CommercialPlatform
}

// NewWorkspaceClosureService builds the service. A nil platform is legal
// (blocked-env): the tombstone and the local projection disposal still
// land — charging stops locally — while the authority disposal waits for a
// wired seam; the closure stays in state closing until then.
func NewWorkspaceClosureService(db *gorm.DB, platform domain.CommercialPlatform) (*WorkspaceClosureService, error) {
	if db == nil {
		return nil, errors.New("workspace closure service requires a database")
	}
	return &WorkspaceClosureService{
		closures: repocommercial.NewWorkspaceClosureStore(db),
		subs:     repocommercial.NewSubscriptionStore(db),
		platform: platform,
	}, nil
}

// CloseWorkspace runs the idempotent closure and answers the tombstone.
// The tombstone is unconditional; a platform failure on the disposal
// returns the error with the tombstone still in state closing — charging
// is ALREADY stopped locally, and a replay re-runs the disposal and
// converges (the caller that must not proceed on failure — the workspace
// deletion gate — refuses and retries).
func (s *WorkspaceClosureService) CloseWorkspace(ctx context.Context, tenantID uint64, actor, reason string) (repocommercial.WorkspaceClosureRow, error) {
	if s == nil || s.closures == nil {
		return repocommercial.WorkspaceClosureRow{}, errors.New("workspace closure service is not wired")
	}
	if tenantID == 0 {
		return repocommercial.WorkspaceClosureRow{}, errors.New("workspace closure requires an authenticated tenant scope")
	}
	extID := domain.ExternalCustomerID(tenantID)
	row, err := s.closures.RecordClosing(ctx, tenantID, extID)
	if err != nil {
		return row, err
	}
	if row.State == repocommercial.WorkspaceClosureStateClosed {
		return row, nil
	}
	var closedAt time.Time
	if s.platform != nil {
		receipt, err := s.platform.SubmitCommand(ctx, domain.Command{
			Kind:   domain.CommandKindCloseWorkspace,
			Key:    domain.CloseWorkspaceKey(extID),
			Actor:  actor,
			Reason: reason,
			Payload: domain.CloseWorkspacePayload{
				TenantID:           tenantID,
				ExternalCustomerID: extID,
				DisplayName:        domain.DeidentifiedDisplayName(extID),
			},
		})
		if err != nil {
			return row, err
		}
		closedAt = receipt.RecordedAt
	}
	if closedAt.IsZero() {
		closedAt = time.Now().UTC()
	}
	// Local projection disposal: the paid term is capped at the closure
	// instant, so the monthly grant walk never grants a month past closure.
	if err := s.subs.RecordWorkspaceClosure(ctx, tenantID, closedAt); err != nil {
		return row, err
	}
	if err := s.closures.MarkClosed(ctx, tenantID, closedAt); err != nil {
		return row, err
	}
	return s.closures.Get(ctx, tenantID)
}

// ClosureState answers the tombstone (ErrWorkspaceClosureNotFound when the
// workspace was never closed).
func (s *WorkspaceClosureService) ClosureState(ctx context.Context, tenantID uint64) (repocommercial.WorkspaceClosureRow, error) {
	if s == nil || s.closures == nil {
		return repocommercial.WorkspaceClosureRow{}, repocommercial.ErrWorkspaceClosureNotFound
	}
	return s.closures.Get(ctx, tenantID)
}

// WorkspaceClosed reports whether the closure tombstone exists — the one
// boolean every new-commercial-work entry point refuses on (#102 AC1).
func (s *WorkspaceClosureService) WorkspaceClosed(ctx context.Context, tenantID uint64) bool {
	if s == nil || s.closures == nil {
		return false
	}
	_, err := s.closures.Get(ctx, tenantID)
	return err == nil
}

// DisableNewScheduling lands ONLY the tombstone (#102 AC1's precondition,
// the O02 deletion-guard step): charging and new commercial work stop
// immediately; the disposal itself waits for CloseWorkspace.
func (s *WorkspaceClosureService) DisableNewScheduling(ctx context.Context, tenantID uint64) error {
	if s == nil || s.closures == nil {
		return errors.New("workspace closure service is not wired")
	}
	_, err := s.closures.RecordClosing(ctx, tenantID, domain.ExternalCustomerID(tenantID))
	return err
}

// WorkspaceClosureReason is the audit reason the closure command carries.
const WorkspaceClosureReason = "workspace_deletion"
