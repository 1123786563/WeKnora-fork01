package commercial

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// ErrWorkspaceClosureNotFound reports a workspace with no closure tombstone.
var ErrWorkspaceClosureNotFound = errors.New("workspace_closure_not_found")

// Workspace closure tombstone states (#102 / Lago 30). closing: the
// tombstone has landed (charging already refused) but the authority-side
// disposal has not been confirmed; closed: the disposal receipt landed.
const (
	WorkspaceClosureStateClosing = "closing"
	WorkspaceClosureStateClosed  = "closed"
)

// WorkspaceClosureRow is the durable closure tombstone (#102): from the
// moment it exists — state closing OR closed — the workspace refuses new
// commercial commands, purchases and charge execution, and the identity is
// never re-ensured (US54: Customer identity is never reused).
type WorkspaceClosureRow struct {
	TenantID           uint64     `gorm:"primaryKey;column:tenant_id"`
	ExternalCustomerID string     `gorm:"column:external_customer_id;uniqueIndex;not null"`
	State              string     `gorm:"column:state;not null"`
	ClosedAt           *time.Time `gorm:"column:closed_at"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null"`
}

func (WorkspaceClosureRow) TableName() string { return "commercial_workspace_closures" }

// WorkspaceClosureStore persists the closure tombstones.
type WorkspaceClosureStore struct{ db *gorm.DB }

// NewWorkspaceClosureStore builds the store and bootstraps the table with
// portable DDL (the NewQuotaGuard precedent — valid next to versioned
// migrations, safe when the table already exists).
func NewWorkspaceClosureStore(db *gorm.DB) *WorkspaceClosureStore {
	if db != nil {
		_ = db.Exec(`CREATE TABLE IF NOT EXISTS commercial_workspace_closures (
			tenant_id BIGINT PRIMARY KEY,
			external_customer_id TEXT NOT NULL UNIQUE,
			state TEXT NOT NULL,
			closed_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`).Error
	}
	return &WorkspaceClosureStore{db: db}
}

// RecordClosing lands the tombstone idempotently: exactly one row per
// workspace, the first caller wins, a replay reads the existing row back.
// The tombstone refuses new commercial work from the instant it lands,
// whatever happens next.
func (s *WorkspaceClosureStore) RecordClosing(ctx context.Context, tenantID uint64, externalCustomerID string) (WorkspaceClosureRow, error) {
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Exec(`INSERT INTO commercial_workspace_closures
		(tenant_id, external_customer_id, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT (tenant_id) DO NOTHING`,
		tenantID, externalCustomerID, WorkspaceClosureStateClosing, now, now).Error; err != nil {
		return WorkspaceClosureRow{}, err
	}
	return s.Get(ctx, tenantID)
}

// MarkClosed confirms the disposal receipt (idempotent: the first timestamp
// wins, a replay never rewrites the audit fact).
func (s *WorkspaceClosureStore) MarkClosed(ctx context.Context, tenantID uint64, at time.Time) error {
	return s.db.WithContext(ctx).Exec(`UPDATE commercial_workspace_closures
		SET state = ?, closed_at = COALESCE(closed_at, ?), updated_at = ? WHERE tenant_id = ?`,
		WorkspaceClosureStateClosed, at, time.Now().UTC(), tenantID).Error
}

// Get answers the tombstone; ErrWorkspaceClosureNotFound when none exists.
func (s *WorkspaceClosureStore) Get(ctx context.Context, tenantID uint64) (WorkspaceClosureRow, error) {
	var row WorkspaceClosureRow
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WorkspaceClosureRow{}, ErrWorkspaceClosureNotFound
	}
	return row, err
}
