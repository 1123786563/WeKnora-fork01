package appconnector

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	// ErrPublicationNotFound: no publication row for the (tenant, action)
	// pair — indistinguishable from a foreign tenant's row by design.
	ErrPublicationNotFound = errors.New("publication_not_found")
	// ErrPublicationConflict: invalid publication lifecycle transition or
	// a duplicate plan for one action.
	ErrPublicationConflict = errors.New("publication_conflict")
)

// Publication lifecycle states. planned is written at plan formation (the
// Action Plan record binding artifact version + external destination +
// expected external version); published/failed/unknown are settled from
// the ACTION row's authoritative terminal outcome — the receipt is a
// projection of the action, never the reverse.
const (
	PublicationPlanned   = "planned"
	PublicationPublished = "published"
	PublicationFailed    = "failed"
	PublicationUnknown   = "unknown"
)

// PublicationRow is the durable 外部发布 record (CONTEXT.md): the approved
// artifact version, the external target, the target's version, and the
// operation result as a receipt. ProgressJSON carries the NO-03 multi-step
// recovery checkpoint between dispatch and settlement.
type PublicationRow struct {
	TenantID  uint64 `gorm:"primaryKey;column:tenant_id"`
	ActionID  string `gorm:"primaryKey;column:action_id"`
	CreatedAt time.Time
	UpdatedAt time.Time

	ConnectionID string `gorm:"column:connection_id;not null"`
	Provider     string `gorm:"column:provider;not null"`    // "notion" (#48); feishu #49, confluence #50
	Mode         string `gorm:"column:mode;not null"`        // create | update
	Destination  string `gorm:"column:destination;not null"` // parent page id (create) / page id (update)
	// ExpectedVersion is the external current version READ BEFORE the plan
	// was formed (AC1 baseline; the update snapshot's expected_version).
	ExpectedVersion string `gorm:"column:expected_version;not null;default:''"`
	// The immutable internal artifact version this publish carries.
	ArtifactVersionID string `gorm:"column:artifact_version_id;not null;default:''"`
	ArtifactDigest    string `gorm:"column:artifact_digest;not null;default:''"`

	State           string `gorm:"column:state;not null"`
	ExternalID      string `gorm:"column:external_id;not null;default:''"`
	ExternalVersion string `gorm:"column:external_version;not null;default:''"`
	ReceiptJSON     string `gorm:"column:receipt_json;not null;default:''"`
	ProgressJSON    string `gorm:"column:progress_json;not null;default:''"`
}

func (PublicationRow) TableName() string { return "app_publications" }

// PublicationStore persists the publish plan records and their receipts.
type PublicationStore struct{ db *gorm.DB }

// NewPublicationStore builds a PublicationStore over a gorm DB.
func NewPublicationStore(db *gorm.DB) *PublicationStore { return &PublicationStore{db: db} }

// CreatePublication inserts one planned publication; a second plan for the
// same action is a conflict (the approval binding is never silently
// rewritten). The existence pre-check and the INSERT run in one
// transaction — gorm's driver-level duplicate-key translation is not
// enabled in this codebase, so the guarded insert is the portable form.
func (s *PublicationStore) CreatePublication(ctx context.Context, row PublicationRow) error {
	if row.TenantID == 0 || row.ActionID == "" || row.Provider == "" || row.Mode == "" || row.Destination == "" || row.State != PublicationPlanned {
		return ErrPublicationConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing PublicationRow
		err := tx.Where("tenant_id = ? AND action_id = ?", row.TenantID, row.ActionID).First(&existing).Error
		if err == nil {
			return ErrPublicationConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&row).Error
	})
}

// FindByAction loads the publication row scoped to the tenant.
func (s *PublicationStore) FindByAction(ctx context.Context, tenantID uint64, actionID string) (PublicationRow, error) {
	var row PublicationRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND action_id = ?", tenantID, actionID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PublicationRow{}, ErrPublicationNotFound
	}
	return row, err
}

// SaveProgress durably records the NO-03 checkpoint without touching the
// lifecycle columns.
func (s *PublicationStore) SaveProgress(ctx context.Context, tenantID uint64, actionID, progressJSON string) error {
	res := s.db.WithContext(ctx).Model(&PublicationRow{}).
		Where("tenant_id = ? AND action_id = ?", tenantID, actionID).
		Update("progress_json", progressJSON)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPublicationNotFound
	}
	return nil
}

func publicationTransitionAllowed(from, to string) bool {
	switch from {
	case PublicationPlanned:
		return to == PublicationPublished || to == PublicationFailed || to == PublicationUnknown
	case PublicationUnknown:
		return to == PublicationPublished || to == PublicationFailed
	default:
		return false
	}
}

// SettlePublication moves the receipt to a state derived from the ACTION
// row's authoritative outcome. Terminal states never move again; the
// published settle records the external id + the version the publish
// produced + the raw provider receipt payload.
func (s *PublicationStore) SettlePublication(ctx context.Context, tenantID uint64, actionID, state, externalID, externalVersion, receiptJSON string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row PublicationRow
		if err := tx.Where("tenant_id = ? AND action_id = ?", tenantID, actionID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPublicationNotFound
			}
			return err
		}
		if !publicationTransitionAllowed(row.State, state) {
			return ErrPublicationConflict
		}
		return tx.Model(&PublicationRow{}).
			Where("tenant_id = ? AND action_id = ?", tenantID, actionID).
			Updates(map[string]interface{}{
				"state": state, "external_id": externalID,
				"external_version": externalVersion, "receipt_json": receiptJSON,
			}).Error
	})
}

// LatestPublishedByDestination returns the most recent published receipt
// for one external destination — the authority rule for update plans: a
// page this tenant+connection published before is ours to update; anything
// else must fail closed.
func (s *PublicationStore) LatestPublishedByDestination(ctx context.Context, tenantID uint64, connectionID, externalID string) (PublicationRow, error) {
	var row PublicationRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND connection_id = ? AND external_id = ? AND state = ?", tenantID, connectionID, externalID, PublicationPublished).
		Order("updated_at DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PublicationRow{}, ErrPublicationNotFound
	}
	return row, err
}
