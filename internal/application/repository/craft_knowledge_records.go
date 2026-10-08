package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
)

// CraftKnowledgeRecordRow is the T05 immutable, Run-scoped persistence row.
// Its production table is allocated by the central migration owner.
type CraftKnowledgeRecordRow struct {
	TenantID   uint64    `gorm:"column:tenant_id;primaryKey"`
	SessionID  string    `gorm:"column:session_id;type:varchar(128);primaryKey"`
	RunID      string    `gorm:"column:run_id;type:varchar(128);primaryKey"`
	RecordJSON string    `gorm:"column:record_json;type:text;not null"`
	Digest     string    `gorm:"column:digest;type:char(64);not null"`
	AcquiredAt time.Time `gorm:"column:acquired_at;not null"`
}

func (CraftKnowledgeRecordRow) TableName() string { return "craft_knowledge_records" }

type CraftKnowledgeRecordRepository struct{ db *gorm.DB }

func NewCraftKnowledgeRecordRepository(db *gorm.DB) *CraftKnowledgeRecordRepository {
	return &CraftKnowledgeRecordRepository{db: db}
}

func (r *CraftKnowledgeRecordRepository) Save(ctx context.Context, record craft.KnowledgeRecord) error {
	if r == nil || r.db == nil {
		return craft.ErrForbidden
	}
	if record.Scope.TenantID == 0 || record.Scope.SessionID == "" || record.RunID == "" || record.RequestDigest == "" || record.PackageDigest == "" || record.PublicationState != craft.KnowledgePublicationPrepared {
		return craft.ErrInvalidInput
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	acquired := time.Now().UTC()
	for _, source := range record.Sources {
		if source.AcquiredAt.IsZero() {
			return craft.ErrInvalidInput
		}
		if source.AcquiredAt.Before(acquired) {
			acquired = source.AcquiredAt
		}
	}
	row := CraftKnowledgeRecordRow{TenantID: record.Scope.TenantID, SessionID: record.Scope.SessionID, RunID: record.RunID, RecordJSON: string(raw), Digest: digest, AcquiredAt: acquired}
	result := r.db.WithContext(ctx).Create(&row)
	if result.Error == nil {
		return nil
	}
	// A retry may repeat the exact immutable observation. Any different payload
	// for the same Run is a conflict, even if the DB reports a generic key error.
	var existing CraftKnowledgeRecordRow
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND session_id = ? AND run_id = ?", row.TenantID, row.SessionID, row.RunID).Take(&existing).Error; err != nil {
		return result.Error
	}
	if existing.Digest == digest && existing.RecordJSON == string(raw) {
		return nil
	}
	return fmt.Errorf("%w: source record for run %s changed", craft.ErrConflict, record.RunID)
}

// MarkPublished atomically transitions only the accepted package digest for a
// Run. The JSON and its integrity digest are updated in one compare-and-swap
// so a stale caller cannot publish a different source observation.
func (r *CraftKnowledgeRecordRepository) MarkPublished(ctx context.Context, scope craft.Scope, runID, packageDigest string) error {
	if r == nil || r.db == nil || scope.TenantID == 0 || scope.SessionID == "" || runID == "" || packageDigest == "" {
		return craft.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row CraftKnowledgeRecordRow
		query := tx.Where("tenant_id = ? AND session_id = ? AND run_id = ?", scope.TenantID, scope.SessionID, runID)
		if err := query.Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return craft.ErrNotFound
			}
			return err
		}
		rowDigest := sha256.Sum256([]byte(row.RecordJSON))
		if hex.EncodeToString(rowDigest[:]) != row.Digest {
			return fmt.Errorf("%w: source record digest mismatch", craft.ErrConflict)
		}
		var record craft.KnowledgeRecord
		if err := json.Unmarshal([]byte(row.RecordJSON), &record); err != nil {
			return fmt.Errorf("%w: corrupt source record: %v", craft.ErrConflict, err)
		}
		if record.Scope.TenantID != scope.TenantID || record.Scope.SessionID != scope.SessionID || record.RunID != runID {
			return craft.ErrForbidden
		}
		if record.PackageDigest != packageDigest {
			return fmt.Errorf("%w: accepted package digest changed for Run %s", craft.ErrConflict, runID)
		}
		if record.PublicationState == craft.KnowledgePublicationPublished {
			return nil
		}
		// Empty is the conservative representation for records written before
		// PublicationState existed. The caller may transition it only after it
		// has resumed and verified the accepted bytes and Publisher completed.
		if record.PublicationState != "" && record.PublicationState != craft.KnowledgePublicationPrepared {
			return fmt.Errorf("%w: unknown publication state %q", craft.ErrConflict, record.PublicationState)
		}
		record.PublicationState = craft.KnowledgePublicationPublished
		raw, err := json.Marshal(record)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		result := tx.Model(&CraftKnowledgeRecordRow{}).
			Where("tenant_id = ? AND session_id = ? AND run_id = ? AND record_json = ? AND digest = ?",
				scope.TenantID, scope.SessionID, runID, row.RecordJSON, row.Digest).
			Updates(map[string]interface{}{"record_json": string(raw), "digest": hex.EncodeToString(sum[:])})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("%w: source record changed while marking Run %s published", craft.ErrConflict, runID)
		}
		return nil
	})
}

func (r *CraftKnowledgeRecordRepository) Load(ctx context.Context, scope craft.Scope, runID string) (craft.KnowledgeRecord, error) {
	if r == nil || r.db == nil || scope.TenantID == 0 || scope.SessionID == "" || runID == "" {
		return craft.KnowledgeRecord{}, craft.ErrForbidden
	}
	var row CraftKnowledgeRecordRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND session_id = ? AND run_id = ?", scope.TenantID, scope.SessionID, runID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.KnowledgeRecord{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.KnowledgeRecord{}, err
	}
	sum := sha256.Sum256([]byte(row.RecordJSON))
	if hex.EncodeToString(sum[:]) != row.Digest {
		return craft.KnowledgeRecord{}, fmt.Errorf("%w: source record digest mismatch", craft.ErrConflict)
	}
	var record craft.KnowledgeRecord
	if err := json.Unmarshal([]byte(row.RecordJSON), &record); err != nil {
		return craft.KnowledgeRecord{}, fmt.Errorf("%w: corrupt source record: %v", craft.ErrConflict, err)
	}
	if record.Scope.TenantID != scope.TenantID || record.Scope.SessionID != scope.SessionID || record.RunID != runID {
		return craft.KnowledgeRecord{}, craft.ErrForbidden
	}
	return record, nil
}
