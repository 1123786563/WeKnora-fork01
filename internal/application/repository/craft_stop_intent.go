package repository

// T17 (#136): the durable member stop intent. One row per (tenant, session,
// run) holds the Run's stop journey state — requested before anything is
// aborted, confirmed only by the authoritative observation, unknown when the
// abort outcome could not be observed. A refresh replays the same durable
// answer and a confirmed stop is never downgraded (the pinned store
// contract the control service's idempotent replays rely on).
import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type craftStopIntentRow struct {
	TenantID  uint64                  `gorm:"primaryKey"`
	SessionID string                  `gorm:"primaryKey;column:session_id;type:varchar(128)"`
	RunID     string                  `gorm:"primaryKey;column:run_id;type:varchar(128)"`
	Status    craft.StopOutcomeStatus `gorm:"column:status;type:varchar(16);not null"`
	CreatedAt time.Time               `gorm:"column:created_at"`
	UpdatedAt time.Time               `gorm:"column:updated_at"`
}

func (craftStopIntentRow) TableName() string { return "craft_stop_intents" }

// CraftStopIntentStore is the production service.CraftStopIntentStore over
// the shared gorm database. GetStopIntent answers craft.ErrNotFound for a
// run without a stop journey — the sentinel the control service branches on.
type CraftStopIntentStore struct{ db *gorm.DB }

// NewCraftStopIntentStore assembles the store; a nil database refuses
// closed (the container keeps the nil seam instead).
func NewCraftStopIntentStore(db *gorm.DB) *CraftStopIntentStore {
	if db == nil {
		return nil
	}
	return &CraftStopIntentStore{db: db}
}

// PutStopIntent persists (or replays) one Run's stop intent. The write is
// idempotent by Run identity and a CONFIRMED stop is never downgraded: a
// replay that arrives after the confirmation keeps the confirmed fact and
// answers it (the caller's requested/unknown replays converge instead of
// erasing the terminal answer).
func (s *CraftStopIntentStore) PutStopIntent(ctx context.Context, scope craft.Scope, intent craft.StopIntent) (craft.StopIntent, error) {
	if s == nil || s.db == nil {
		return craft.StopIntent{}, errors.New("craft: stop intent store is not assembled")
	}
	if err := intent.Validate(); err != nil {
		return craft.StopIntent{}, err
	}
	if scope.TenantID == 0 || scope.SessionID == "" {
		return craft.StopIntent{}, fmt.Errorf("%w: stop intent requires the task scope", craft.ErrInvalidInput)
	}
	now := time.Now().UTC()
	effective := intent
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current craftStopIntentRow
		err := tx.Where("tenant_id = ? AND session_id = ? AND run_id = ?",
			scope.TenantID, scope.SessionID, intent.RunID).Take(&current).Error
		if err == nil && current.Status == craft.StopConfirmed && intent.Status != craft.StopConfirmed {
			// Never downgrade a confirmed stop.
			effective = craft.StopIntent{RunID: current.RunID, Status: current.Status}
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row := craftStopIntentRow{
			TenantID: scope.TenantID, SessionID: scope.SessionID, RunID: intent.RunID,
			Status: intent.Status, CreatedAt: now, UpdatedAt: now,
		}
		if err == nil {
			row.CreatedAt = current.CreatedAt
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "session_id"}, {Name: "run_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"status", "updated_at"}),
		}).Create(&row).Error
	})
	if err != nil {
		return craft.StopIntent{}, err
	}
	return effective, nil
}

// GetStopIntent reads one Run's durable stop intent; a run without a stop
// journey answers an error wrapping craft.ErrNotFound (never a fabricated
// status).
func (s *CraftStopIntentStore) GetStopIntent(ctx context.Context, scope craft.Scope, runID string) (craft.StopIntent, error) {
	if s == nil || s.db == nil {
		return craft.StopIntent{}, errors.New("craft: stop intent store is not assembled")
	}
	if scope.TenantID == 0 || scope.SessionID == "" || runID == "" {
		return craft.StopIntent{}, fmt.Errorf("%w: stop intent read requires the task scope and run id", craft.ErrInvalidInput)
	}
	var row craftStopIntentRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND session_id = ? AND run_id = ?",
		scope.TenantID, scope.SessionID, runID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.StopIntent{}, fmt.Errorf("%w: no stop intent for run %s", craft.ErrNotFound, runID)
	}
	if err != nil {
		return craft.StopIntent{}, err
	}
	return craft.StopIntent{RunID: row.RunID, Status: row.Status}, nil
}
