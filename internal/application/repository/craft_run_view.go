package repository

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
)

// CraftRunViewStore persists one immutable execution-view generation for each
// admitted Run. It records runtime identity only; it does not provide process
// or filesystem isolation.
type CraftRunViewStore struct {
	db *gorm.DB
}

var _ craft.RunViewStore = (*CraftRunViewStore)(nil)

// NewCraftRunViewStore constructs a RunView store backed by the migrated
// business database.
func NewCraftRunViewStore(db *gorm.DB) *CraftRunViewStore {
	return &CraftRunViewStore{db: db}
}

type craftRunViewRow struct {
	TenantID              uint64     `gorm:"column:tenant_id"`
	RunID                 string     `gorm:"column:run_id"`
	OwnerID               string     `gorm:"column:owner_id"`
	SessionID             string     `gorm:"column:session_id"`
	Generation            string     `gorm:"column:generation"`
	RuntimeID             string     `gorm:"column:runtime_id"`
	ContainerID           string     `gorm:"column:container_id"`
	OpenCodeSessionID     string     `gorm:"column:opencode_session_id"`
	State                 string     `gorm:"column:state"`
	SessionCreateIntentAt *time.Time `gorm:"column:session_create_intent_at"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (craftRunViewRow) TableName() string { return "craft_run_views" }

func (r craftRunViewRow) view() (craft.RunView, error) {
	view := craft.RunView{
		Key: craft.RunViewKey{
			TenantID: r.TenantID, OwnerID: r.OwnerID, SessionID: r.SessionID, RunID: r.RunID,
		},
		Generation: r.Generation,
		Runtime: craft.RunViewRuntime{
			RuntimeID: r.RuntimeID, ContainerID: r.ContainerID, OpenCodeSessionID: r.OpenCodeSessionID,
		},
		State: craft.RunViewState(r.State), SessionCreateIntentAt: r.SessionCreateIntentAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if err := craft.ValidateRunView(view); err != nil {
		return craft.RunView{}, fmt.Errorf("craft: stored RunView is invalid: %w", err)
	}
	return view, nil
}

type craftRunViewRunIdentity struct {
	OwnerID   string `gorm:"column:owner_id"`
	SessionID string `gorm:"column:session_id"`
}

func authorizeCraftRunView(db *gorm.DB, key craft.RunViewKey) (craftRunViewRunIdentity, error) {
	var run craftRunViewRunIdentity
	err := db.Table("agent_runs").Select("owner_id, session_id").
		Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craftRunViewRunIdentity{}, craft.ErrNotFound
	}
	if err != nil {
		return craftRunViewRunIdentity{}, err
	}
	// Session mismatch is deliberately indistinguishable from a missing Run;
	// only an owner mismatch inside the same session is reported forbidden.
	if run.SessionID != key.SessionID {
		return craftRunViewRunIdentity{}, craft.ErrNotFound
	}
	if run.OwnerID != key.OwnerID {
		return craftRunViewRunIdentity{}, craft.ErrForbidden
	}
	return run, nil
}

func loadCraftRunViewRow(db *gorm.DB, key craft.RunViewKey, run craftRunViewRunIdentity) (craftRunViewRow, error) {
	var row craftRunViewRow
	err := db.Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craftRunViewRow{}, craft.ErrNotFound
	}
	if err != nil {
		return craftRunViewRow{}, err
	}
	if row.OwnerID != run.OwnerID || row.SessionID != run.SessionID {
		return craftRunViewRow{}, fmt.Errorf("%w: stored RunView scope differs from admitted Run", craft.ErrConflict)
	}
	return row, nil
}

// Allocate creates or recovers the single view for an admitted Run. The
// INSERT...SELECT validates the authoritative owner/session in the same write
// statement; the primary key makes concurrent retries converge on one row.
func (s *CraftRunViewStore) Allocate(ctx context.Context, key craft.RunViewKey) (craft.RunView, error) {
	if err := craft.ValidateRunViewKey(key); err != nil {
		return craft.RunView{}, err
	}
	generation, err := newCraftRunViewGeneration()
	if err != nil {
		return craft.RunView{}, fmt.Errorf("craft: generate RunView generation: %w", err)
	}
	var result craft.RunView
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inserted := tx.Exec(`
			INSERT INTO craft_run_views
				(tenant_id, run_id, owner_id, session_id, generation, state, created_at, updated_at)
			SELECT admitted.tenant_id, admitted.run_id, admitted.owner_id, admitted.session_id, ?, 'allocating', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			FROM agent_runs AS admitted
			WHERE admitted.tenant_id = ? AND admitted.run_id = ? AND admitted.owner_id = ? AND admitted.session_id = ?
			ON CONFLICT (tenant_id, run_id) DO NOTHING`,
			generation, key.TenantID, key.RunID, key.OwnerID, key.SessionID,
		)
		if inserted.Error != nil {
			return inserted.Error
		}
		run, authErr := authorizeCraftRunView(tx, key)
		if authErr != nil {
			return authErr
		}
		row, loadErr := loadCraftRunViewRow(tx, key, run)
		if loadErr != nil {
			return loadErr
		}
		view, viewErr := row.view()
		if viewErr != nil {
			return viewErr
		}
		result = view
		return nil
	})
	return result, err
}

// Load recovers the row only after validating the complete key against the
// authoritative admitted Run identity.
func (s *CraftRunViewStore) Load(ctx context.Context, key craft.RunViewKey) (craft.RunView, error) {
	if err := craft.ValidateRunViewKey(key); err != nil {
		return craft.RunView{}, err
	}
	var result craft.RunView
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run, err := authorizeCraftRunView(tx, key)
		if err != nil {
			return err
		}
		row, err := loadCraftRunViewRow(tx, key, run)
		if err != nil {
			return err
		}
		view, err := row.view()
		if err != nil {
			return err
		}
		result = view
		return nil
	})
	return result, err
}

// BeginSessionCreate durably claims the one allowed external CreateSession
// request for this exact RunView generation. The claim is committed before a
// caller receives maySend=true. Any later call, including after a process
// restart or an unknown network result, returns the durable view with
// maySend=false and must be handled by reconciliation rather than resubmission.
func (s *CraftRunViewStore) BeginSessionCreate(ctx context.Context, key craft.RunViewKey, generation string) (craft.RunView, bool, error) {
	if err := craft.ValidateRunViewKey(key); err != nil {
		return craft.RunView{}, false, err
	}
	if generation == "" {
		return craft.RunView{}, false, fmt.Errorf("%w: empty RunView generation", craft.ErrInvalidInput)
	}
	var result craft.RunView
	var maySend bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updated := tx.Exec(`
			UPDATE craft_run_views
			SET session_create_intent_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			WHERE tenant_id = ? AND run_id = ? AND owner_id = ? AND session_id = ?
			  AND generation = ? AND state = 'allocating' AND session_create_intent_at IS NULL
			  AND EXISTS (
				SELECT 1 FROM agent_runs AS admitted
				WHERE admitted.tenant_id = craft_run_views.tenant_id AND admitted.run_id = craft_run_views.run_id
				  AND admitted.owner_id = ? AND admitted.session_id = ?
			  )`,
			key.TenantID, key.RunID, key.OwnerID, key.SessionID, generation, key.OwnerID, key.SessionID,
		)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 1 {
			maySend = true
		}

		// Load and authorize only after the atomic write attempt. This ordering
		// lets SQLite serialize competing UPDATEs rather than taking two read
		// snapshots that can deadlock when both are promoted to writers.
		run, err := authorizeCraftRunView(tx, key)
		if err != nil {
			return err
		}
		row, err := loadCraftRunViewRow(tx, key, run)
		if err != nil {
			return err
		}
		if row.Generation != generation {
			return fmt.Errorf("%w: RunView generation changed", craft.ErrConflict)
		}
		if row.State != string(craft.RunViewStateAllocating) && row.State != string(craft.RunViewStateBound) {
			return fmt.Errorf("%w: RunView cannot begin session creation from state %q", craft.ErrConflict, row.State)
		}
		if !maySend && row.SessionCreateIntentAt == nil && row.State != string(craft.RunViewStateBound) {
			return fmt.Errorf("%w: RunView create intent was not claimed", craft.ErrConflict)
		}
		result, err = row.view()
		return err
	})
	if err != nil {
		// A failed commit must never leak permission to issue the external POST.
		return craft.RunView{}, false, err
	}
	return result, maySend, nil
}

// BindRuntime compare-and-swaps a complete runtime identity onto an unresolved
// generation. Exact replay is idempotent; a lost or mismatched binding never
// silently creates or substitutes a runtime.
func (s *CraftRunViewStore) BindRuntime(ctx context.Context, key craft.RunViewKey, generation string, runtime craft.RunViewRuntime) (craft.RunView, error) {
	if err := craft.ValidateRunViewKey(key); err != nil {
		return craft.RunView{}, err
	}
	if generation == "" {
		return craft.RunView{}, fmt.Errorf("%w: empty RunView generation", craft.ErrInvalidInput)
	}
	if err := craft.ValidateRunViewRuntime(runtime); err != nil {
		return craft.RunView{}, err
	}
	var result craft.RunView
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updated := tx.Exec(`
			UPDATE craft_run_views
			SET runtime_id = ?, container_id = ?, opencode_session_id = ?, state = 'bound', updated_at = CURRENT_TIMESTAMP
			WHERE tenant_id = ? AND run_id = ? AND owner_id = ? AND session_id = ? AND generation = ? AND state = 'allocating'
			  AND session_create_intent_at IS NOT NULL
			  AND EXISTS (
				SELECT 1 FROM agent_runs AS admitted
				WHERE admitted.tenant_id = craft_run_views.tenant_id AND admitted.run_id = craft_run_views.run_id
				  AND admitted.owner_id = ? AND admitted.session_id = ?
			  )`,
			runtime.RuntimeID, runtime.ContainerID, runtime.OpenCodeSessionID,
			key.TenantID, key.RunID, key.OwnerID, key.SessionID, generation, key.OwnerID, key.SessionID,
		)
		if updated.Error != nil {
			return updated.Error
		}
		run, err := authorizeCraftRunView(tx, key)
		if err != nil {
			return err
		}
		row, err := loadCraftRunViewRow(tx, key, run)
		if err != nil {
			return err
		}
		if row.Generation != generation {
			return fmt.Errorf("%w: RunView generation changed", craft.ErrConflict)
		}
		view, err := row.view()
		if err != nil {
			return err
		}
		if updated.RowsAffected == 0 {
			if view.State != craft.RunViewStateBound || view.Runtime != runtime {
				return fmt.Errorf("%w: RunView runtime binding already differs", craft.ErrConflict)
			}
		}
		result = view
		return nil
	})
	return result, err
}

func newCraftRunViewGeneration() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "rv_" + base64.RawURLEncoding.EncodeToString(raw), nil
}
